---
title: Contracts
nav_order: 6
---

# Contracts — the pipeline's type system

Because the phase workers are LLMs (non-deterministic), the only thing that
guarantees a correct hand-off from one phase to the next is an explicit, checkable
**shape** for the data that crosses the boundary. gogo ships those shapes as JSON
Schemas in `templates/contracts/`, and every phase **validates its inputs**
before working and **validates its outputs** before hand-off. Source of truth:
`templates/contracts/*` and `skills/gogo-contracts/SKILL.md`.

`plan.md` is a **prose** contract — its "validation" is the human acceptance gate
(`state.md` status `plan-accepted`), not a JSON schema.

## The typed artifacts

All live in the feature folder `.gogo/plans/feature-<slug>/`.

| Schema | Artifact | Produced by | Consumed by |
|---|---|---|---|
| `issues-list.schema.json` | `review/issues.json`, `test/issues.json` | review ③, test ④ | implement ② |
| `charts-manifest.schema.json` | `charts/manifest.json` | implement ② | review ③, test ④ |
| `phase-result.schema.json` | `<phase>/result.json` (per run) | every standalone command | orchestrator `go` |
| `pipeline.schema.json` | `pipeline.json` (feature-level index) | every standalone command | orchestrator `go` |

### `issues-list.schema.json` — the living issues list

An object `{ slug, track, round, updated?, issues[] }` where each issue carries
**exactly**:

```
id, title, description, proposed_solution,
severity ∈ {blocker, major, minor, nit},
priority ∈ {P0, P1, P2, P3},
status   ∈ {open, fixed, verified, wontfix, new},
origin   ∈ {review, test},
found_in_round,
fixed_in_round?, fix_summary?    (required once status = fixed)
```

There is **one living file per track**. `review/issues.json` and
`test/issues.json` are updated **in place** across rounds — statuses move
`new`/`open` -> `fixed` (implement) -> `verified` (a later review/test), and new
findings are appended. Ids are stable and never reused. The matching `review-NN.md`
/ `test-NN.md` is the rendered human **snapshot** of one round (the audit trail),
not the contract. The consumer is `implement` ② via `--issues <path>`, which fixes
the `open`/`new` issues and writes back `status: fixed`, `fix_summary`,
`fixed_in_round`.

### `charts-manifest.schema.json` — the as-built diagram index

`{ slug, updated?, note?, diagrams[] }`, each diagram `{ kind ∈ {flow, sequence,
class, activity}, file (a .mmd under charts/), title }`. Implement ② emits the
as-built set; only the kinds that carry signal appear (diagram the **product**,
never the task list). A pure-process change has an empty `diagrams` array and a
`note`. Review ③ and test ④ consume it to reason about the change.

### `phase-result.schema.json` — the per-run record

`{ slug, phase, status ∈ {ok, blocked, waiting-for-user}, round?, inputs[],
outputs[], validated_in, validated_out, open_issues?, summary }`. Each standalone
command writes one after it finishes, recording whether its validate-in and
validate-out gates passed.

### `pipeline.schema.json` — the feature-level index

`{ slug, updated?, phases{ plan?, implement?, review?, test?, report? } }`, each
phase entry `{ status, valid, round?, open_issues?, artifacts[] }` — a glanceable
index of what each phase last produced and whether it is contract-valid.
`state.md` stays the human-facing phase/status file alongside it.

## The validate gate

The shared `gogo-contracts` skill runs at every boundary in **two tiers**, and is
**portable — no new required dependency**:

- **Tier 1 (structural)** — parse the JSON and, *if and only if* a validator is
  already installed, schema-check it. Uses `jq` / `python3` / `check-jsonschema` /
  `ajv` only when present, never installs anything; if nothing is present, the
  agent checks the document against the schema field-by-field.
- **Tier 2 (semantic)** — the agent **always** runs these: right `slug` (matches
  the feature folder), real paths (every referenced path exists, repo-relative,
  nothing outside `.gogo/`), unique stable ids, valid enums, and required fields
  present (including conditional ones — a `fixed` issue has `fixed_in_round` +
  `fix_summary`).

**validate-in** runs before any work: each required input must exist and pass both
tiers, or the phase **STOPs** with a precise contract error and never works on bad
input. **validate-out** runs before hand-off: the produced artifact must pass; on
failure the phase repairs once, and if it still fails it writes a `result.json`
with `status: blocked`, `validated_out: false` and does not hand off. A contract
error names the artifact, the schema, and the exact check, for example:

> Contract error (validate-in): `test/issues.json` — issue `TEST-002` has
> `severity: "high"`, not in {blocker, major, minor, nit}
> (`issues-list.schema.json`). Fix the producer or the file before re-running.
