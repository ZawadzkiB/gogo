# Report — CLI process-orchestrator (Slice 1)

**Feature:** `cli-orchestrator` · **Status:** awaiting-uat · **Version:** 0.13.0
**Pipeline:** plan → implement (in-context, warm) → review ×2 → test → report

---

## What shipped

A **second orchestrator** for gogo: the Go CLI subcommand **`gogo run [<slug>]`** that drives the
②→③→④(→⑤) loop by spawning each phase as its own `claude -p` session — the **developer session
kept warm across fix rounds via `--resume`** (never re-reads the codebase), while **review and
test spawn fresh** (fresh eyes). It **coexists** with the in-chat `/gogo:go`: both drive the *same*
phase skills and the *same* typed contracts (`state.md`, `*/issues.json`, `*/result.json`), so no
phase logic is duplicated. This is Slice 1 (the walking skeleton) of roadmap #11; the core
warm-continuity premise was proven by a `claude -p --resume` spike before any code was written.

## Planned vs. shipped

| Plan | Shipped |
|---|---|
| FR1 `gogo run` + acceptance gate | ✅ `cli/run.go` + `RunnableStatus` (same gate as `/gogo:go`) |
| FR2 session-aware launch primitives | ✅ `launch.PhaseArgs` + `launch.RunPhase` (injection-safe argv; waits on `-p` exit) |
| FR3 warm developer via `--resume` | ✅ one stable dev UUID, `--session-id` first then `--resume` every fix round |
| FR4 fresh review/test sessions | ✅ a new UUID per round, never `--resume` |
| FR5 exit-based phase detection | ✅ `RunPhase` blocks on process exit, then reads typed outputs |
| FR6 one routing rule, no drift | ✅ `contract.Route` — **corrected to be track-aware** (see decisions) |
| FR7 bounded loop + cost ceiling | ✅ total-per-feature round bound + cost ceiling, both **gate** (never abort); cost pre-flighted |
| FR8 gate via existing attach path | ✅ `gateDecision`/`gateBudget`; `--attach` launches an interactive `/gogo:resume` |
| FR9 CLI-owned session registry | ✅ `.gogo/resources/cli/sessions/<slug>.json` (never mutates pipeline state) |
| FR11 `--in-session` implement flag | ✅ `commands/implement.md` + `skills/gogo-implement` (so `--resume` continues the real worker) |
| FR10/FR12/FR13 | **deferred** to Slices 2–3 (agent-type abstraction, cost surfacing, board wiring) — as planned |

## Implementation (the pieces)

- **`cli/internal/orchestrator/`** (new) — `orchestrator.go` (the loop, gates, bounds, UUID gen),
  `registry.go` (the CLI-owned session registry). A `PhaseRunner` interface makes the loop testable
  with an injected fake (no real spawns); `ClaudeRunner` is the production impl.
- **`cli/internal/launch/launch.go`** — added `PhaseArgs`/`RunResult`/`RunPhase` (foreground
  wait-for-exit `claude -p`, distinct from the board's backgrounded `Launch`) + `ResumeIntent`.
- **`cli/internal/contract/`** — `result.go` (`ReadResult`), `route.go` (`Route`, track-aware).
- **`cli/run.go`** + `cli/main.go` — the `gogo run` dispatch, help, acceptance gate.
- **plugin side (FR11)** — `commands/implement.md` + `skills/gogo-implement/SKILL.md` document the
  `--in-session` selector.
- **docs + version** — README (the process-orchestrator bullet), `docs/cli-contract.md` (the registry
  as CLI-owned, not contract), version bumped to 0.13.0 (plugin.json + `cli/main.go` mirror).

## Decisions + reasons

- **D1–D6** (accepted at plan time, all as recommended): new `gogo run` subcommand; wait-on-exit
  phase detection; pause+notify gate (opt-in `--attach`); `--in-session` flag; registry under
  `.gogo/resources/`; ~3-round bound + cost ceiling that gate.
- **REV-001 (routing semantics, resolved deterministically).** Review flagged that a single
  `Route` matched `gogo-test` §④ (routes on any open/new) but diverged from `gogo-review` §④
  (routes only on blockers/majors, **batches minors**). Tagged needs-user-decision, but the accepted
  plan's **constraint 3 already declares the skills canonical** — so `Route` was made **track-aware**
  to match both skills exactly, and FR6's imprecise "count > 0" text was corrected. **Open question
  for the user:** if `gogo run` should be *deliberately stricter* than `/gogo:go` on minors, this is
  the one place to flip; otherwise it now matches the in-chat flow (the "no drift" intent).

## Review & test outcomes

- **Review:** round 1 → **CHANGES** (3 major, 2 minor). Real bugs caught: a **false-green** path
  (`is_error`/no-output silently advancing), a **money-sink re-gate** (a re-run spending a paid
  session before re-gating), and the **routing drift** above. All fixed warm, in-context. Round 2 →
  **APPROVE**, all 6 findings (incl. a follow-on cost-accounting minor, REV-006) verified/fixed, **0 open**.
- **Test:** **all green, done-bar met.** Go suite (`gofmt`/`vet`/`go test -race`) clean across 9
  packages; CLI guards exercised (help / bad-slug / acceptance-gate refusal / no-`.gogo` / no-`claude`);
  and the **stub-`claude` e2e dry run** drove the *real* binary through happy-path, warm-`--resume`,
  and `is_error`-halt scenarios. A new hermetic `cli/run_e2e_test.go` captures them (it crosses the
  real `exec.Command("claude", …)` boundary the fake-runner unit tests never reach).

## Diagrams (as-built)

`report/flow.mmd` (the Go loop), `report/shared-core.mmd` (two orchestrators, one core),
`report/sequence.mmd` (warm resume vs fresh eyes), `report/class.mmd` (the shipped Go structure);
`report/before/` holds the plan-time baseline. View with `/gogo:view cli-orchestrator` or
`gogo view cli-orchestrator:report --web`.

## Audit trail

Full detail in `.gogo/work/feature-cli-orchestrator/`: `plan.md` (as-built FRs), `decisions.md`
(D1–D6 resolved), `review/issues.json` + `review-01/02.md`, `test/issues.json` + `test-01.md`,
`events.jsonl`.

## What's next

- **UAT (you):** verify, then `/gogo:done cli-orchestrator` to ship — or give feedback to loop back.
- **Later slices:** FR10 agent-type abstraction (gemini/codex/opencode seam), FR12 cost/telemetry on
  the board + `gogo status`, FR13 board wiring. Optional follow-ups a future pass could pick up:
  per-finding round bound (REV-005 is currently total-per-feature, documented).
