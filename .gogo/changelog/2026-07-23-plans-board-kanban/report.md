# plans-board-kanban - 0.26.0 (2026-07-23)

The cockpit's **plans tab became a 4-column kanban** (drafts · ready · active · done) that
mirrors the work board, driven by an **all-manual single `m` move**. The 0.25.0 auto-spawn
that used to fire when a plan hit `ready` was **re-sequenced onto a new `go` step**, so
marking a plan ready is now a pure bookkeeping move and the fan-out only happens when the
user says go. Alongside that, **`plan done` writes a deterministic project changelog** (and
the plan stays in-store as `done` so the 4th column has something to show), the plan detail
now lists **work items with their live status**, and a work item spawned into a
skip-configured source **auto-runs on the next board reload** when its per-source slot is
free, showing a visible "trigger manually" cue instead of silently waiting when the repo is
at cap.

Review **APPROVE**, test **PASS**.

## What changed

- **Plans tab is a kanban.** `viewPlansBoard` / `renderPlanColumn` / `renderPlanCard` reuse
  the work board's column chrome, separators, and card box styles; the flat cursor was
  replaced by per-column state (`planCols [4][]plans.Plan` plus column/card index and
  offset), with `done` promoted to a visible 4th column and windowing reusing the shared
  `scrollWindow`/`fitEnd`.
- **One move key.** `planMove` advances the focused plan one column right, branching on its
  persisted status; the old `r` (accept+spawn) and `D` (accept UAT) folded into `m`. Arrows
  or `hjkl` navigate, `enter` opens detail.
- **Spawn re-sequenced.** `planMarkReady` (draft to ready, no spawn) split from `planGo`
  (ready to active, the fan-out). The CLI gained a `plan go` verb beside a now
  mark-ready-only `plan ready`, preserving idempotency, member-on-success-only, and
  cross-project skip isolation.
- **Work items in plan detail.** Each row shows the source dot, name, the spawned feature's
  slug, and its live status pill, read through the existing contract reader, never a source
  write.
- **Deterministic project changelog.** `MarkDone` writes an LLM-free
  `ChangelogDir(project)/<date>-<id>/entry.md` with best-effort member enrichment (bare
  `source:slug` fallback), under `~/.gogo/` only.
- **Reload-driven auto-pickup.** `autoPickupCmds` launches `claude -p /gogo:go <slug>` for
  plan members in a `planAcceptanceSkip` source sitting at `plan-accepted` with no live
  session, gated by the source's integer concurrency cap, counted per source root, fired
  once per composite key. At cap it does not fire and surfaces a transient "trigger
  manually" cue on both the work-board and plan cards.

## Decisions

- **D1=B, all FR1-6 in one build** rather than a sliced cut; organized as build phases
  A/B/C but shipped together.
- **D2=A, a single board-style `m` move.** The transition handlers are identical either
  way, so it came down to board consistency.
- **D3=A, deterministic CLI changelog**, matching the CLI's deterministic-writer role; the
  plan stays in-store as `done` so the kanban still shows it.
- **D4=B, reload auto-fire, cap-gated.** Auto-pickup only removes the initial keypress:
  skip-configured sources only, only into a free slot, fire-once, with a visible cue rather
  than a silent wait. A genuine mid-pipeline gate still parks the session at
  `waiting-for-user`.
- **No generic `Board[T]`.** The kanban shares the work board's chrome but the card bodies
  and move semantics genuinely differ, so only a plan-typed partition plus a small card
  renderer were added.

## Review + test verdict

Review **APPROVE** in one round: 0 blocker/major; REV-001 (fire-once stranded the key on a
launch error, now un-records and retries), REV-002 (missing reload-wiring test, added), and
REV-003 (em-dashes in user strings) all fixed. Test **PASS** in one round: 407 tests green
under `-race`, plus a real-binary CLI e2e confirming `plan ready` marks ready with zero
launches, `plan go` fans out once per target with `--correlation` and per-source
`--skip-acceptance`, bad input exits non-zero, and `plan done` wrote the changelog at the
right path while source repos' `.gogo/` stayed untouched; a live tmux drive confirmed the
4-column render and an `m` move. TEST-001 (a stray em-dash in `planGo` stderr) fixed.

Invariants held: the CLI writes only `~/.gogo/`, never a source's `.gogo/`; every
spawn/pickup is a launched `claude -p`, never a CLI state-flip; sessions stay attributed by
exact slug match. Additive over 0.25.1.

## Follow-ups

- A headless/scheduled auto-pickup runner (`gogo plan pickup` / cron). The reload path
  only fires from an open cockpit, but the reconcile is factored pure for a future caller.
- A launched-synthesis project changelog (the D3 option B) if richer prose is wanted.
- Tune the `gogo-project-plan` analyst quality by driving `A` (carried from 0.25.x).

---

Full audit trail (plan, adjustments, decisions, review and test rounds, per-file changes):
[`.gogo/work/feature-plans-board-kanban/`](../../work/feature-plans-board-kanban/)
