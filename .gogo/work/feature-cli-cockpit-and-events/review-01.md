# Review round 01 — feature `cli-cockpit-and-events` (Stage A only)

Scope: Stage A of the frozen consumer contract — `templates/contracts/events.schema.json`,
`docs/cli-contract.md`, `templates/contracts/README.md`, the 7 skill emission edits
(gogo, gogo-plan, gogo-implement, gogo-review, gogo-test, gogo-knowledge, gogo-done),
and the enumeration syncs (README table, skills/gogo list, state.template.md).
Stage B (the Go CLI, plugin.json bump, architecture.md `cli/` entry) is out of scope.

Diff base: `47d872f`. plugin.json verified untouched (still `0.9.0`); no `cli/` dir — consistent with Stage A.

## Verdict: **CHANGES** — 2 open majors.

Severity counts: **0 blocker · 2 major · 4 minor · 1 nit** (7 findings, all new this round).

## Findings

### REV-001 · major · P1 · new — phase-started/phase-done double emission
Both the orchestrator (`skills/gogo/SKILL.md:220-222`) and the phase skills
(`gogo-plan`, `gogo-implement`, `gogo-knowledge`) emit `phase-started` for
plan/implement/report, and both emit `phase-done/report`. Through `/gogo:go` each of
these lands in `events.jsonl` TWICE with different `ts`. The emitter table
(`docs/cli-contract.md:175-181`) documents both sides; §5 Reader rules warn only about
gaps, never duplicates. Ownership is internally contradictory — the orchestrator's own
composition note lists the phase-skill events *without* `phase-started`. This defeats the
timeline-accuracy that is the feature's whole point.
**Fix (AGENT-FIXABLE):** pick one owner. Recommended — skills own `phase-started`/`phase-done`
(needed for standalone `/gogo:implement`), orchestrator drops its copies and keeps only the
gate events; update `skills/gogo/SKILL.md:220-225` + the cli-contract emitter table. Or, if
intentional, add an explicit dedup rule to §5.

### REV-002 · major · P1 · new — `events.jsonl` missing from docs/architecture.md file tree
The work-folder tree at `docs/architecture.md:151-163` enumerates the per-feature file set
but omits `events.jsonl` (README + skills/gogo + state.template were synced; architecture
slipped). This is exactly the 0.8.0 REV-001 doc-sync class `code-review-standards.md` #1
warns about.
**Fix (AGENT-FIXABLE):** add an `events.jsonl` line to the tree. It is a Stage A artifact —
it must land with (or before) the Stage B architecture sweep and must not slip.

### REV-003 · minor · P2 · new — gate events can emit invalid `phase:"knowledge"`
Orchestrator gate events emit `"phase":"<resume phase>"` (`skills/gogo/SKILL.md:236-243`).
state.md's fifth-phase token is `knowledge`, which is NOT in the events `phase` enum
(`events.schema.json:32`). A gate in the report phase would produce a schema-invalid line.
**Fix (AGENT-FIXABLE):** instruct the gate emission to map `knowledge`→`report` (mirror the
note already in gogo-knowledge).

### REV-004 · minor · P2 · new — `ts` under-pinned for the Go parser
`ts` is specified only as "ISO-8601" (`events.schema.json:10-13`), a superset of RFC3339.
An LLM-emitted non-RFC3339 value would fail `time.Parse(time.RFC3339,...)` and be silently
skipped.
**Fix (AGENT-FIXABLE):** pin to RFC3339 — add `"format":"date-time"`, say RFC3339 in the
description and cli-contract §5, keep the `...Z` example.

### REV-005 · minor · P2 · new — docs/contracts.md schema catalog omits events.schema.json
`templates/contracts/README.md` added the events row; `docs/contracts.md:22-27` did not, yet
cli-contract cross-links contracts.md as "the same schemas." (Minor — events is telemetry,
not a validated hand-off, so exclusion is a judgement call.)
**Fix (AGENT-FIXABLE):** add an events row to contracts.md (marked best-effort/non-gated), or
soften the cross-link.

### REV-006 · nit · P3 · new — forward-compat promise vs `additionalProperties:false`
The stability statement promises additive compat ("reader must ignore what it does not
recognize"), but the schema is `additionalProperties:false`, so strict validation of a future
line would reject it. Holds only because Go's `encoding/json` ignores unknowns.
**Fix (AGENT-FIXABLE):** one clarifying sentence — additionalProperties:false is producer
self-validation; forward-compat relies on the consumer ignoring unknown fields.

### REV-007 · minor · P3 · new — §6 overclaims `members[]` presence on changelog manifests
`docs/cli-contract.md:200` presents `members[]` as present ("`[<slug>]` for a single entry"),
but all 5 existing on-disk changelog manifests omit it (they predate the field). Stage B tests
run against these real entries. The §3 folder-slug fallback still classifies them correctly, so
this is a doc-completeness gap.
**Fix (AGENT-FIXABLE):** add a symmetric "older entries may lack members[]" caveat to §6
(§6 already caveats the parallel diagrams.html case).

## Answers to the orchestrator's explicit questions

- **Double-emission:** Confirmed real. `phase-started/plan`, `phase-started/implement`,
  `phase-started/report`, and `phase-done/report` are each emitted by BOTH the orchestrator and
  the corresponding phase skill, and the cli-contract emitter table bakes both in. Ownership is
  ambiguous and self-contradictory → REV-001 (major).
- **report vs knowledge mismatch:** The naming split is **adequately documented** for a parser
  author — the schema `phase` description, cli-contract §2, and §5 all state the
  `knowledge`(state.md) ↔ `report`(events) mapping. No schema/state.md alignment is required.
  The one place the mapping can leak is the orchestrator's gate events → REV-003 (minor).
- **Implementable-as-spec by a Go author without reading the skills?** **Almost.** The layout,
  state.md grammar (incl. HTML-comment block, `stage:`/`completed:` lines — verified against
  real features), the verbatim classifier table, schema pointers (all resolve), changelog shape,
  and stability statement are all present and accurate. The gaps that would trip a Go author are
  REV-001 (duplicate events, undocumented), REV-004 (ts not pinned to RFC3339), and REV-007
  (members[] absent on the real test entries). Fixing those makes it a clean standalone spec.
