# Report — feature `pipeline-commands`

- **feature:** Composable, validatable pipeline commands (typed phase commands + validation gates)
- **status:** done (Stage A)
- **completed:** 2026-06-24
- **branch / commits:** main — **uncommitted working tree** (nothing committed yet)

## Summary
Each gogo phase is now a standalone, idempotent command that behaves like a typed
function: it declares the documents it consumes, **validates inputs**, works, and
**validates outputs** before hand-off. A new **contract layer** (JSON Schemas +
the `gogo-contracts` skill) makes the data that crosses phase boundaries
machine-checkable, so a bad LLM hand-off is caught instead of propagated. This
shipped as **Stage A** (per decision D7); Stage B — rewiring `/gogo:go` to chain
on `result.json`/`pipeline.json` — is deferred to a follow-up.

## Planned vs shipped
Shipped as planned for Stage A. Notable specifics:
- **Added** four standalone commands (`/gogo:implement`, `/gogo:review`,
  `/gogo:test`, `/gogo:report`) + the `gogo-contracts` skill + four JSON-Schema
  contracts.
- **Issues list** is JSON-first (D1) and lives at `review/issues.json` /
  `test/issues.json` (one living list per track, D2), with `review-NN.md` /
  `test-NN.md` as the rendered human snapshot.
- **Deferred to Stage B** (D7): `commands/go.md` and the `gogo` orchestrator's
  loop logic are unchanged except vocabulary/enumeration edits. Standalone
  commands *emit* `result.json`; *consuming* it to drive the loop is Stage B.
- **Honest as-built note:** this feature was itself driven through the **legacy**
  pipeline — review/test produced markdown `review-NN.md`/`test-NN.md`, not the new
  `issues.json`, because the new commands only activate once the plugin is
  installed/reloaded. The new JSON contract takes effect for the *next* feature.

## Changes (as-built)
| File | Change | Note |
|---|---|---|
| `templates/contracts/issues-list.schema.json` | added | FR4 fields + enums; `fixed ⇒ fixed_in_round + fix_summary` conditional (allOf/if/then) |
| `templates/contracts/phase-result.schema.json` | added | per-run `result.json` shape |
| `templates/contracts/pipeline.schema.json` | added | feature-level artifact index |
| `templates/contracts/charts-manifest.schema.json` | added | as-built diagram index; `kind ∈ {flow,sequence,class,activity}` |
| `templates/contracts/README.md` | added | documents each shape + producer/consumer |
| `skills/gogo-contracts/SKILL.md` | added | schema registry + two-tier validate-in/out (jq/python/validator if present, else agent) |
| `commands/implement.md` `review.md` `test.md` `report.md` | added | thin standalone entry points; `implement` documents `--issues <path>` |
| `skills/gogo-implement/SKILL.md` | modified | `--issues` fix mode + fix-backs; emits as-built charts + `charts/manifest.json`; validate gates |
| `skills/gogo-review/SKILL.md` | modified | emits living `review/issues.json` + renders `review-NN.md`; validate gates |
| `skills/gogo-test/SKILL.md` | modified | emits living `test/issues.json` + `test-NN.md`; loop-back contract; validate gates |
| `skills/gogo-knowledge/SKILL.md` | modified | standalone `report`; re-writes/validates `charts/manifest.json`; writes `report/result.json` |
| `skills/gogo/SKILL.md` | modified | feature-folder artifact list + phase bodies name the JSON contracts (no orchestrator rewiring — Stage B) |
| `skills/gogo-mermaid/SKILL.md` | modified | chart-kind vocabulary aligned to the schema enum (`class`/`activity`) |
| `templates/report.template.md` | modified | diagram-kind wording aligned |
| `templates/state.template.md` | modified | feature-folder file list adds `*/issues.json`, `charts/manifest.json` |
| `README.md` | modified | Commands section adds the 4 standalone commands + contract-layer note; "What gets created" adds issues lists + contracts |
| `.claude-plugin/plugin.json` | modified | version 0.1.4 → 0.2.0 |

## Decisions
D1 JSON-first issues list · D2 one living list per track · D3 in-plugin schemas +
two-tier portable validation · D4 `result.json`+`pipeline.json` (state.md stays
human) · D5 three idempotent workers + `report` + `go` · D6 chart kinds
flow/sequence/class/activity when they carry signal · D7 two-stage delivery
(Stage A now). All resolved as recommended — see [decisions.md](./decisions.md).

## Review outcome
Two rounds. Round 1 → **CHANGES** (1 major + 3 minor + 1 nit); the major (REV-001)
was real contract drift — chart-kind prose (`actions`/`structure`) disagreeing
with the schema enum, which would make a producer fail its own validate-out gate.
Round 2 → **APPROVE**, all five resolved, no new drift. See
[review-01.md](./review-01.md), [review-02.md](./review-02.md).

## Test outcome
**GREEN.** The contract layer was exercised empirically with `jsonschema`
(Draft7Validator). All negative cases for the issues-list contract were correctly
rejected — bad `severity`/`priority` enums, the `fixed ⇒ fixed_in_round +
fix_summary` conditional, a missing required field; duplicate-id is caught by the
semantic (Tier-2) check by design. Command files are well-formed and thin; the
no-tool portability fallback holds; diagrams inline correctly. Live command
behaviour was **not** tested (requires installing the plugin) — acceptable for
Stage A. See [test-01.md](./test-01.md).

## Diagrams
Open [charts/diagrams.html](./charts/diagrams.html) (offline). Indexed in
[charts/manifest.json](./charts/manifest.json):
- **flow** — `pipeline.mmd`: typed artifacts through validate-in/out, looping on issues.
- **activity** — `issue-lifecycle.mmd`: an issue's status lifecycle in the living `issues.json`.
- **sequence** — `handoff.mmd`: a validated review→implement hand-off via `gogo-contracts`.

## Knowledge updates
- `.gogo/knowledge/project-knowledge.md` — architecture now records the contract
  layer (`templates/contracts/` + `gogo-contracts`) and the standalone phase
  commands; glossary gains the typed-artifact / validate-gate terms.
- No upstreaming suggestions (the README is gogo's own and was already updated as
  part of the change).

## Follow-ups & known limitations
- **Stage B** — rewire `/gogo:go` + the `gogo` orchestrator to chain on
  `result.json`/`pipeline.json` and loop on issues-list emptiness; emit/consume
  `pipeline.json`. (Tracked; the contract layer it builds on is now proven.)
- **Release** — bump-and-publish: commit, push, `marketplace update` + `install`
  to activate the new commands; only then can the new JSON pipeline be dogfooded live.
- **Migration** — existing features carry markdown `review-NN.md`/`test-NN.md`;
  the JSON contract applies to features built after install.
