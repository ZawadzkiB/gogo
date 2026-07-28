# Report — `plans-board-kanban` (0.26.0)

**The cockpit's plans tab is now a 4-column KANBAN (drafts · ready · active · done) that
mirrors the work board, with an all-manual lifecycle driven by a single `m` move — and the
0.25.0 auto-spawn that fired on `ready` is re-sequenced onto a new `go` step.** Accepting a
plan's project-UAT now also writes a deterministic project changelog, and a work item spawned
into a skip-configured source auto-runs on the next board reload when a slot is free. Ships
**0.26.0**. Review **APPROVE**, test **PASS**.

## Run status

Plan accepted 2026-07-23 (D1=B all FR1-6 in one build, D2=A single `m` move, D3=A deterministic
changelog, D4=B reload auto-fire with the per-source cap two-branch). Built in-context across
Phases A/B/C. Review 1 round APPROVE (0 blocker/major; 3 findings REV-001/002/003 all fixed).
Test 1 round PASS (407 tests `-race` green + real-binary CLI e2e + live TUI drive; 1 nit TEST-001
fixed). Gate: `gofmt`/`go vet`/`go test -race ./...` green; `gogo --version` → 0.26.0.

## Planned vs shipped

| FR | Shipped |
|---|---|
| **FR1** — plans tab is a 4-column kanban | `cli/internal/tui/plans_tab.go` `viewPlansBoard`/`renderPlanColumn`/`planColumnHeader`/`renderPlanCard` reuse the work board's column width + separators + per-column card box styles; `model.go` replaced the flat `planIdx`/`groupedPlans` cursor with `planCols [4][]plans.Plan` + `planColIdx`/`planCardIdx [4]int`/`planColOffset [4]int` (`rebuildPlans` partitions by status, `done` now a visible 4th column); `window.go` `reflowPlanColumns`/`planCardHeights` reuse the shared `scrollWindow`/`fitEnd`. |
| **FR2** — all moves manual via one board-style `m` | `planMove` advances the focused plan one column right, resolving by status; the old `r` (accept+spawn) and `D` (accept UAT) folded into `m`. `←→/h l` columns · `↑↓/j k` cards · `enter` detail. |
| **FR3** — re-sequence spawn off `ready` onto `go` | `planMarkReady` (draft→ready, **no spawn**) + `planGo` (ready→active, the moved fan-out). CLI: `cli/plan.go` split `planReady` (mark-ready-only) + added `planGo`; `go` registered in `isPlanStoreVerb`/`cmdPlanStore`/`planStoreHelp`. Idempotency (member-OR-feature skip), member-on-success-only (REV-005), and cross-project skip isolation (REV-001 of 0.25.0) preserved. |
| **FR4** — plan detail lists work items + status | `viewPlanDetail` heading reframed to `WORK ITEMS`; each row shows the source dot + name + the spawned feature's slug + its live status pill (read via the existing `spawnedFeature` contract reader; never a source write). |
| **FR5** — active→done archives + writes a project changelog (D3=A) | `cli/internal/plans/plans.go` `MarkDone` now also writes a deterministic, LLM-free `ChangelogDir(project)/<date>-<id>/entry.md` (`writeChangelogEntry` + `loadProjectRepo` for best-effort member enrichment, bare `source:slug` fallback). The plan STAYS in the store as `done` (kanban 4th-column parity). Writes only under `~/.gogo/`. |
| **FR6** — reload-driven auto-pickup (D4=B) | `cli/internal/tui/pickup.go` — `autoPickupCmds` (called from the `reloadMsg` handler in `update.go`) launches `claude -p /gogo:go <slug>` for each plan member in a `planAcceptanceSkip` source at `plan-accepted`, no live session, **under its source's integer `ConcurrentWorkItems` cap** (counted per-source-root via `orchestrator.CapForSource`/`ActiveWorkCount`/`CapExceeded`), not already fired (`autoPickedUp` set, composite `featureKey`). At cap → NOT fired + a **"trigger manually"** cue on both the work-board card (`view.go renderCard`) and the plan card (`planPickupCue`), transient. Launch reuses `intentFor(ActionGo)` (carries the source's SkipParams) — byte-for-byte a manual go; a failed launch un-records the key so the next reload retries (REV-001). |
| version + docs | `0.26.0` in `plugin.json` + `cli/main.go`; help text, `README.md`, `skills/gogo-cli/SKILL.md` synced; enum-sync + version-mirror + no-unsafe-rm guards green. |

## Implementation notes

- **Reuse, not a parallel board.** The kanban shares the work board's column chrome + card box
  styles; only a plan-typed partition (`rebuildPlans`) + a small `renderPlanCard` were added
  (a generic `Board[T]` was rejected — the card bodies + move semantics genuinely differ).
- **The move is one dispatch.** `planMove(p)` branches on the plan's persisted status, so the
  kanban (focused plan) and the plan detail both move a plan identically. Each transition keeps
  its own huh confirm + the existing `updateForm` routing — no new form plumbing.
- **Auto-pickup fire-once is re-entrancy-safe.** The key is recorded synchronously in
  `autoPickupCmds` (so a concurrent reload before the session appears can't double-launch); a
  cap-skip is deliberately NOT recorded (a freed slot re-fires later); a failed launch
  un-records via `autoPickupResultMsg{ok:false}` so it retries. Cap is the source's integer,
  counted per-source-root, never global.

## Decisions (+ reasons)

- **D1=B — all FR1-6 in one build.** The user chose the whole vision over the sliced cut;
  organized as build-order Phases A/B/C but shipped together.
- **D2=A — a single board-style `m` move.** Board consistency; the transition handlers are
  identical either way, so it was a keymap choice. `r`/`D` folded into `m`.
- **D3=A — deterministic CLI changelog.** Matches the CLI's deterministic-writer role (no
  launched synthesis); the plan stays in-store as `done` so the kanban still shows it.
- **D4=B — reload auto-fire, cap-gated.** The user's "picked automatically" intent, made safe:
  only skip-configured sources, only into a free slot (respecting the per-source integer cap),
  fire-once, and a visible "trigger manually" cue (not a silent wait) when the repo is busy —
  which auto-clears and fires when a slot frees. A genuine mid-pipeline gate still parks the
  session at `waiting-for-user`; auto-pickup only removes the initial keypress.

## Review + test outcomes

- **Review:** APPROVE. The five high-risk areas held — auto-pickup fire-once/cap correctness,
  the spawn re-sequence, the kanban cursor/windowing, the changelog invariants, and the
  `~/.gogo/`-only / launched-`claude -p` invariants. REV-001 (fire-once strands on launch
  error → now retries), REV-002 (no reload-wiring test → added), REV-003 (em-dash in user
  strings → converted) all fixed.
- **Test:** PASS. `go test -race ./...` (407 tests) green; the real `/tmp/gogo` binary confirmed
  `plan ready` marks-ready-with-zero-launches even with targets, `plan go` fans out once per
  target with `--correlation` + per-source `--skip-acceptance`, the bad-input paths exit
  non-zero, and `plan done` wrote the changelog `entry.md` at the right path while the source
  repos' `.gogo/` stayed untouched. A live tmux drive confirmed the 4-column kanban render + a
  live `m` move. TEST-001 (one remaining em-dash in `planGo`'s stderr) fixed.

## Invariants held

CLI writes ONLY `~/.gogo/` (the plan file + members/status + the changelog entry), NEVER a
source's `.gogo/`; every spawn/pickup is a launched `claude -p` (the skill writes the source's
`.gogo/work/`), never a CLI state-flip; heap-stable `*formBinding` (TEST-001) untouched;
sessions attributed by exact `SessionMatchesSlug`. Additive over 0.25.1.

## Follow-ups

- A **headless/scheduled** auto-pickup runner (`gogo plan pickup` / cron) — FR6 fires only from
  the open cockpit's reload loop; the reconcile is factored pure so a future caller can reuse it.
- A launched-synthesis project changelog (D3 option B) if a richer prose entry is wanted later.
- Tune the `gogo-project-plan` analyst quality by driving `A` (carried from 0.25.x).

## TL;DR

The plans tab is now a **4-column kanban** with an **all-manual `m` move**; **spawn moved off
`ready` onto `go`**; **`plan done` writes a deterministic project changelog** (plan stays `done`
in-store); and **skip-source work items auto-run on reload into a free slot**, showing a
**"trigger manually"** cue when the repo is at its per-source cap. Ships **0.26.0**. Review
APPROVE, test PASS. Full audit: `.gogo/work/feature-plans-board-kanban/`.
