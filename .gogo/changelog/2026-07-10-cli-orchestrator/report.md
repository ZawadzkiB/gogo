# CLI process-orchestrator - `gogo run` (Slice 1)

**Shipped:** 2026-07-10 · **Version:** 0.13.0 · **Member:** `cli-orchestrator`

gogo gained a **second orchestrator**: the Go CLI subcommand **`gogo run [<slug>]`**, which drives
the ②→③→④(→⑤) pipeline loop by spawning each phase as its own `claude -p` batch session. The
**developer session is kept warm across fix rounds** (created once with `--session-id`, then
`--resume` every round, so it never re-reads the codebase), while **review and test spawn fresh each
round** to preserve fresh eyes. It **coexists** with the in-chat `/gogo:go`: both drive the *same*
phase skills and the *same* typed contracts (`state.md`, `*/issues.json`, `*/result.json`), so there
is one pipeline, two front-ends, and no duplicated phase logic. This is Slice 1, the walking
skeleton of roadmap #11; the warm-continuity premise was proven by a `claude -p --resume` spike
before any code was written.

**Key outcomes**
- `gogo run [<slug>]` behind the same acceptance gate as `/gogo:go` (`RunnableStatus`), with a
  bounded fix loop (round cap + cost ceiling) that **gates** rather than aborts, so the human always
  decides at the ceiling.
- Warm developer continuity (one stable UUID) vs. fresh review/test UUIDs, with phase completion
  detected by **process exit** (no polling), then typed outputs read back.
- One shared routing rule (`contract.Route`) made **track-aware** so `gogo run` matches the in-chat
  flow exactly (the "no drift" intent), plus a CLI-owned session registry under
  `.gogo/resources/cli/sessions/` that never mutates pipeline state.
- The plugin side gained the `--in-session` implement path (FR11) so `--resume` continues the *real*
  worker instead of spawning an inner `gogo-developer` Task.

**Decisions (D1-D6, all accepted as recommended at plan time)**
- D1 → new `gogo run <slug>` subcommand (board wiring deferred to Slice 3), keeping the board a fast,
  LLM-free selector.
- D2 → wait on the `claude -p` process exit for phase completion; no polling.
- D3 → decision gate = pause + notify + print the attach/resume command (opt-in `--attach`
  auto-attaches).
- D4 → a documented `--in-session` flag on `/gogo:implement` (landed first, FR11).
- D5 → session registry at `.gogo/resources/cli/sessions/<slug>.json` (CLI-owned bookkeeping).
- D6 → ~3-round bound + per-feature cost ceiling, both env-configurable, both gate (never silently
  abort).
- Mid-build fork (REV-001): review found the single `Route` matched `gogo-test` §④ but diverged from
  `gogo-review` §④ (which batches minors). Resolved deterministically per the plan's "skills are
  canonical" constraint: `Route` was made track-aware to match both. One open lever noted for the
  user: flip it if `gogo run` should be *deliberately stricter* than `/gogo:go` on minors.

**Review & test verdict:** review round 1 → CHANGES (caught a false-green advance, a money-sink
re-gate, and the routing drift, all fixed warm), round 2 → **APPROVE, 0 open**; test **all green**
(Go `gofmt`/`vet`/`go test -race` clean across 9 packages, CLI guards exercised, and a hermetic
stub-`claude` e2e dry run driving the real binary through happy-path, warm-`--resume`, and
`is_error`-halt).

**Scope note:** FR10 (agent-type abstraction), FR12 (cost/telemetry surfacing), and FR13 (board
wiring) are deliberately deferred to Slices 2-3, as planned.

## Diagrams (as-built)
- `cli-orchestrator-flow.mmd` - the Go orchestrator loop (warm dev resume, fresh review/test,
  `issues.json` routing).
- `cli-orchestrator-shared-core.mmd` - two orchestrators over one shared core (skills + typed
  contracts + the one routing rule).
- `cli-orchestrator-sequence.mmd` - warm developer resume across fix rounds vs. fresh-eyes
  review/test.
- `cli-orchestrator-class.mmd` - the as-built Go structure (orchestrator package +
  `launch.RunPhase` + `contract.Route`).
- `before/cli-orchestrator-flow.mmd` - the plan-time baseline (before/after compare).

## Audit trail
Full detail lives in [`.gogo/work/feature-cli-orchestrator/`](../../work/feature-cli-orchestrator/):
`plan.md` (as-built FRs), `decisions.md` (D1-D6 resolved), `review/issues.json` + `review-01/02.md`,
`test/issues.json` + `test-01.md`, and `events.jsonl`.
