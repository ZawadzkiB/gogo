# Decisions — feature `pipeline-commands`

Forks that shape the design. Each has gogo's recommendation; tell me which to
change and I'll revise the plan before acceptance.

## D1 — Issues list format
- **Question:** what is the issues-list *contract* — JSON or markdown?
- **Options:**
  - A. **JSON-first** (`issues.json` is the contract) + auto-rendered `*-NN.md` human view.
  - B. Markdown-first (`review-NN.md`), parse fields out when needed.
- **gogo recommends:** **A** — you asked for typed fields; only JSON is machine-validatable. Markdown stays as the readable snapshot.
- **Status:** RESOLVED — recommended option accepted (user, 2026-06-24)

## D2 — Living list vs per-round files
- **Question:** one issues list updated in place, or a new file each round?
- **Options:**
  - A. **One living `issues.json` per track** (review, test), statuses move open→fixed→verified/new; `*-NN.md` = per-round snapshot for audit.
  - B. Immutable `review-01/02…json` per round; orchestrator diffs them.
- **gogo recommends:** **A** — matches "update the list with what was fixed"; simpler to reason about; snapshots preserve history.
- **Status:** RESOLVED — recommended option accepted (user, 2026-06-24)

## D3 — Validation mechanism (under the no-deps bar)
- **Question:** how do we validate non-deterministic LLM artifacts portably?
- **Options:**
  - A. **In-plugin JSON schemas + two-tier check**: structural via `jq`/schema *if present*, else agent validates against the schema; semantic checks always run. Lives in a new `gogo-contracts` skill.
  - B. Hard-require `jq` + a schema validator.
  - C. Agent-only validation, no schemas.
- **gogo recommends:** **A** — keeps the portability contract; still catches bad hand-offs. B breaks no-deps; C isn't reliably checkable.
- **Status:** RESOLVED — recommended option accepted (user, 2026-06-24)

## D4 — Where chaining state lives
- **Question:** how does the orchestrator chain deterministically?
- **Options:**
  - A. **Per-run `result.json`** (phase, status, inputs, outputs, validated flags) + feature-level `pipeline.json` index; `state.md` stays human-facing.
  - B. Cram it all into `state.md`.
- **gogo recommends:** **A** — machine state separate from the human state file; keeps `state.md` readable.
- **Status:** RESOLVED — recommended option accepted (user, 2026-06-24)

## D5 — Command granularity
- **Question:** how many commands?
- **Options:**
  - A. **Three idempotent workers** (`implement`/`review`/`test`) + `report` + orchestrator `go`. "implement-from-review" = `implement --issues`; "review-after-fixes" = re-run `review`.
  - B. Literally five commands mirroring the five sub-steps you listed.
- **gogo recommends:** **A** — same capability, less surface + duplication; idempotent commands compose.
- **Status:** RESOLVED — recommended option accepted (user, 2026-06-24)

## D6 — Chart kinds emitted by `implement`
- **Question:** which as-built diagrams does implement produce (and review/test consume)?
- **Options:**
  - A. **flow + sequence + class + activity/state (when relevant)** — skip any that carry no signal.
  - B. A fixed mandatory set (always all four).
- **gogo recommends:** **A** — per the diagram-subject rules; avoids trivial/misleading charts. Confirm if you want all four always.
- **Status:** RESOLVED — recommended option accepted (user, 2026-06-24)

## D7 — Delivery phasing
- **Question:** one big change or staged?
- **Options:**
  - A. **Two stages** — Stage A: contracts + validation + issues-JSON + standalone `review`/`implement`/`test`/`report`. Stage B: rewire `go` chaining + `pipeline.json`/`result.json`.
  - B. All at once.
- **gogo recommends:** **A** — Stage A is independently testable and useful; Stage B builds on a proven contract layer.
- **Status:** RESOLVED — recommended option accepted (user, 2026-06-24)
