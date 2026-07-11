# Persistent-session orchestrator - the CLI as a session-lifecycle manager

**Shipped:** 2026-07-12 · **Version:** 0.15.0 · **Member:** `persistent-session-orchestrator`

The `gogo` CLI stopped **re-implementing the pipeline loop**. `gogo go <slug>` / `gogo plan <slug>` now **launch-or-`--resume` ONE persistent `claude -p` session** that runs the existing skill end-to-end (implement in-context + review/test as nested `Task` subagents + report), and the CLI became a pure **session-lifecycle manager**: an owner lock, a session registry, exit classification, and reaping. This is the direct follow-up to Slice 1 (`gogo run`, 0.13.0): it **deletes** the second copy of the pipeline loop that lived in Go - so the routing rule now lives in exactly one place, the skill - and lands the two fixes a live incident demanded: a **one-owner-per-work-item lock** and a **session registry + kill-at-ship + `gogo sweep`** orphan reaper.

## Key outcomes

- **One routing rule, in the skill.** The Go per-phase loop (`orchestrator.Run()` spawning fresh dev/review/test sessions and re-routing via `contract.Route`) is gone; `contract.Route` + `contract.PhaseResult` + their tests are **deleted** (D3). No more two-copies-can-drift.
- **`gogo go` / `gogo plan`** are the primary verbs (matching the skills they launch); **`gogo run`** stays a one-version **deprecated alias** (D2). Each builds a `Session` and calls `LaunchOrResume`: reap-if-terminal → acquire the owner lock → resolve fresh-`--session-id` vs warm-`--resume` from the registry → run the persistent session → **classify the exit by reading `state.md`** (awaiting-uat / waiting-for-user / `is_error`) → book telemetry + release the lock.
- **One-owner lock** (D1) - a lockfile recording PID + uuid + tmux + host + started-at, whose liveness is cross-checked against **both** signal-0 **and** a matching live `gogo-*` tmux session (exact match, never substring). Live owner → refuse-by-default (or `--takeover` to seize + reap the prior); both dead → stale, silently reclaimed. The create is atomic (`O_CREATE|O_EXCL`), and a live **untracked** board session (no lockfile) is refused too - the exact board-launched racer the lock exists to catch.
- **Session registry + reaper** - a per-leg (`go` | `plan`) persistent session tracked under `.gogo/resources/cli/sessions/<slug>.json` (uuid, tmux name, PID, lifecycle status, cost/turns telemetry); missing/garbled/legacy → degrades to a fresh run, never a crash. `gogo sweep` reaps orphaned `gogo-*` sessions (with `--dry-run` + a TTL backstop), and the `--attach` path never sets `remain-on-exit`, so it leaves no lingering pane by construction (D4/D5).

## Decisions (D1–D6 accepted as recommended at plan time; D7 resolved at test)

- **D1 → C** - lock = lockfile **+** PID/tmux liveness cross-check (a pure file or a pure scan each miss a case).
- **D2 → A** - rename `gogo run` → `gogo go` (+ add `gogo plan`); keep `run` as a deprecated alias.
- **D3 → A** - delete `contract.Route` + `route_test.go` + the Go loop; the skill is the single router.
- **D4 → C** - headless `-p` default + `--attach` option (spike-proven, cost-lean, race-free exit; `--attach` keeps live-answer ergonomics off the leak-prone default).
- **D5 → A** - `gogo sweep` + opportunistic reap (keeps `/gogo:done` + skills untouched, FR10).
- **D6 → A** - refuse-by-default on a live-owner collision, `--takeover` to seize.
- **D7 → light real smoke** (test gate) - the change under test is the CLI session-manager, so a real `gogo plan` leg exercises exactly the new code against a live model, cheaper than a full pipeline.

## Review & test verdict

**Review: APPROVE (2 rounds).** Round 1 found no blockers/majors and 4 agent-fixable findings, all fixed in-context; round 2 **verified every one** with no regressions - REV-001 (a write-scope `validSlug` guard so a `..`/`/` slug can't escape `.gogo/resources/`), REV-002 (reap by slug, not the pre-collision base name), REV-003 (atomic `O_EXCL` acquire + closing the untracked-board-session gap the review didn't flag), REV-004 (deleted the dead `contract/result.go`).

**Test: GREEN.** `gofmt`/`go vet`/`go test -race` clean, **149/149** across all packages - including the hermetic **stub-claude e2e** (`TestGoE2EStubClaude`, which crosses the real `exec.Command("claude", …)` boundary: first-launch argv, warm `--resume`, `is_error` halt, `run`-alias) and the full lock/registry/reap/sweep suite. The true end-to-end (**TEST-001 / D7**) was **verified by a live smoke**: `gogo plan psorch-smoke` drove a real `claude -p "/gogo:plan …"` - fresh `--session-id`, real exec + JSON-envelope parse, the real skill parked at `awaiting-plan-acceptance`, the CLI classified the exit correctly (exit 2 + resume hint), the registry booked a `plan` session with live telemetry (cost $4.69, 11 turns, 745s), and the lock was released.

## Scope note

Deliberately deferred (noted in the plan, not built here): the gate/state CLI commands (`gogo accept`/`adjust`/`done`), board drill-in surfacing the recorded session telemetry, multi-model (gemini/codex) behind an agent-type seam, changing the board's own `Launch`/`remain-on-exit` (the reaper covers its orphans meanwhile), and a `/gogo:done` reap hook for *immediate* kill-at-ship. None blocking. Knowledge: `.gogo/knowledge/project-knowledge.md`'s CLI section was updated to the 0.15.0 persistent-session model; no `Mode: proxy` source was touched.

## Diagrams (as-built)

- `persistent-session-orchestrator-flow.mmd` - `gogo go/plan` control flow: lock guard (incl. the untracked board racer) → launch-or-resume the persistent `claude -p` → classify exit → reap/sweep.
- `persistent-session-orchestrator-sequence.mmd` - two orchestrators over one skill: the CLI session manager spawns `claude -p "/gogo:go"`; the skill runs implement in-context + `Task` review/test + report.
- `persistent-session-orchestrator-activity.mmd` - the session lifecycle: none → running → parked/awaiting-uat (resume) → shipped/orphaned → reaped.
- `persistent-session-orchestrator-class.mmd` - the shipped structure: `Session` manager + `Registry`/`PersistentSession`, `Lock`/`Owner`, `Sweeper`, and the injectable `SessionRunner`/`AttachFn`/`LivenessFn`/`Lister` seams.
- `before/persistent-session-orchestrator-flow.mmd` - the plan-time baseline (the deleted per-phase Go loop + the board-launch `remain-on-exit` orphan leak) for before/after compare.

## Audit trail

Full detail lives in [`.gogo/work/feature-persistent-session-orchestrator/`](../../work/feature-persistent-session-orchestrator/): `plan.md` (as-built FRs), `decisions.md` (D1–D7 resolved), `review/issues.json` + `review-01/02.md`, `test/issues.json` + `test-01.md`, `report/report.md` (the full as-built report), and `events.jsonl`.
