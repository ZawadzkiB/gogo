# gogo pipeline contracts

The pipeline's **type system**. Because the phase workers are LLMs
(non-deterministic), the only thing that guarantees a correct hand-off from one
command to the next is an explicit, checkable **shape** for the data that crosses
the boundary. These JSON Schemas are those shapes.

Each standalone command (`/gogo:implement`, `/gogo:review`, `/gogo:test`,
`/gogo:report`) **validates its inputs** against these contracts before doing any
work, and **validates its outputs** against them before hand-off. The shared
procedure lives in the **`gogo-contracts`** skill (`skills/gogo-contracts/SKILL.md`),
which every phase skill calls. Validation is **two-tier and portable**: structural
via `jq` or a JSON-schema validator *if present*, else the agent validates against
the schema; semantic checks (right slug, real paths, unique ids, valid enums)
always run. No new required dependency.

## The schemas

| Schema | Artifact (per feature folder) | Produced by | Consumed by |
|---|---|---|---|
| `issues-list.schema.json` | `review/issues.json`, `test/issues.json` | review ③, test ④ | implement ② |
| `charts-manifest.schema.json` | `charts/manifest.json` | implement ② | review ③, test ④ |
| `phase-result.schema.json` | `<phase>/result.json` (per run) | every standalone command | orchestrator `go` (Stage B) |
| `pipeline.schema.json` | `pipeline.json` (feature-level index) | every standalone command | orchestrator `go` (Stage B) |

> The feature folder is `.gogo/plans/feature-<slug>/`. Paths in the table are
> relative to it.

### `issues-list.schema.json` — the living issues list

The core contract (plan FR4/FR5). An object `{ slug, track, round, updated?,
issues[] }` where each issue carries **exactly**:

```
id, title, description, proposed_solution,
severity ∈ {blocker, major, minor, nit},
priority ∈ {P0, P1, P2, P3},
status   ∈ {open, fixed, verified, wontfix, new},
origin   ∈ {review, test},
found_in_round,
fixed_in_round?, fix_summary?    (required once status = fixed)
```

- **One living file per track.** `review/issues.json` and `test/issues.json` are
  updated **in place** across rounds — statuses move `new`/`open` → `fixed`
  (implement) → `verified` (a later review/test), and new findings are appended.
  Ids are stable and never reused. The matching `review-NN.md` / `test-NN.md` is
  the rendered human **snapshot** of one round (audit trail), not the contract.
- **Producers:** `review` ③ and `test` ④. **Consumer:** `implement` ② via
  `--issues <path>`, which fixes `open`/`new` issues and writes back `status:
  fixed`, `fix_summary`, `fixed_in_round`.

### `charts-manifest.schema.json` — the as-built diagram index

`{ slug, updated?, note?, diagrams[] }`, each diagram `{ kind ∈ {flow, sequence,
class, activity}, file (a .mmd under charts/), title }`. Implement ② emits the
as-built set; only the kinds that carry signal appear (per the diagram-subject
rules in `gogo-mermaid` — diagram the **product**, never the task list). A
pure-process change has an empty `diagrams` array and a `note`. Review ③ and test
④ consume it to reason about the change.

### `phase-result.schema.json` — the per-run record

`{ slug, phase, status ∈ {ok, blocked, waiting-for-user}, round?, inputs[],
outputs[], validated_in, validated_out, open_issues?, summary }`. Each standalone
command writes one after it finishes. **Stage A** emits this as an output
artifact; the orchestrator **consuming** it to chain phases is **Stage B**.

### `pipeline.schema.json` — the feature-level index

`{ slug, updated?, phases{ plan?, implement?, review?, test?, report? } }`, each
phase entry `{ status, valid, round?, open_issues?, artifacts[] }`. A glanceable
index of what each phase last produced and whether it is contract-valid. **Stage
A** may emit it; the orchestrator **consuming** it to drive the loop is **Stage
B**. `state.md` stays the human-facing phase/status file in both stages.

## Validating by hand

Structural check, if a tool is present (both optional):

```bash
# parse-only (catches malformed JSON) — jq or python3, whichever exists
jq . review/issues.json >/dev/null            # or: python3 -m json.tool review/issues.json
```

For full schema validation, use any JSON-schema validator you already have (e.g.
`check-jsonschema`, `ajv`); none is required. When no validator is present, the
agent validates the document field-by-field against the schema and always runs
the semantic checks. See `skills/gogo-contracts/SKILL.md` for the exact steps.
