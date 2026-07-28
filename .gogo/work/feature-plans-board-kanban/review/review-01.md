# Review 01 — plans-board-kanban (0.26.0)

Fresh-eyes review of the uncommitted working tree for the plans-board kanban
(Phases A/B/C, FR1-FR6). Build/vet/`go test -race`/gofmt confirmed green by the
orchestrator; this pass is logic / correctness / contract-fidelity only.

## Verdict: **APPROVE** — no open blockers or majors

3 findings, all minor/nit. None blocks the ship; each is agent-fixable and can be
picked up as polish (REV-001 and REV-002 ideally before the changelog/report phase).

## Scope reviewed
`cli/internal/plans/plans.go`, `cli/plan.go`, `cli/internal/tui/{model,update,view,window,plans_tab,pickup}.go`,
`cli/main.go`, `.claude-plugin/plugin.json`, README/SKILL doc-sync, and the new
`pickup_test.go` / `changelog_test.go` plus the adapted `plans_tab_test.go` /
`plan_test.go` / `tabs_test.go` / `version_test.go`.

## What holds up (verified against the plan + standards)

- **Spawn truly re-sequenced off `ready` onto `go`.** TUI `planMarkReady` (draft→ready)
  calls `plans.MarkReady` only, no launcher; `planGo` (ready→active) owns the fan-out.
  CLI mirror split: `planReady` marks-ready-only even for a targeted plan;
  `planGo` fans out. Both idempotent — TUI via `unspawnedTargets` (member OR board
  feature), CLI via `planHasMember` + `planFeatureSpawned`. Member recorded /
  status flipped **only on a successful launch** (REV-005 preserved; no phantom
  member on launch failure). Tests: `TestPlansTabDraftMoveMarksReadyNoSpawn`,
  `TestPlansTabGoSpawnsPerTarget`, `…GoSkipsAlreadySpawned`, `…GoLaunchErrorRecordsNoMember`,
  and the CLI `TestCmdPlanGo…` set.
- **Cross-project skip isolation (REV-001) preserved.** `finishPlanSpawn` resolves
  the skip via `m.sourceByName` (focused project only); `TestPlansTabAcceptSpawnSkipScopedToFocusedProject`
  still pins it. The pickup path resolves skip/cap by the feature's own `Root`
  through `capWatchSources()`, so it is root-scoped, not first-path-match.
- **Auto-pickup fire-once + cap two-branch correct.** Fire-once key recorded
  synchronously in `autoPickupCmds` (re-entrancy-safe against a second reload);
  cap-blocked candidates are NOT recorded, so a freed slot auto-fires on a later
  reload; cap counted per-source via `CapForSource`/`ActiveWorkCount`/`CapExceeded`
  scoped to `f.Root`; pointer-receiver map mutation survives back to the returned
  Model (Update's `m` is addressable → `(&m).autoPickupCmds()` → `return m`). The
  seven BDD cases (a-g) are pinned in `pickup_test.go`, incl. `--skip-acceptance`
  carry, non-skip skip, cap cue on both cards, live-session dedupe, and the
  waiting-for-user real-gate guard.
- **Kanban cursor/windowing safe.** `focusedPlan` guards the empty column;
  `rebuildPlans` re-clamps; `reflowPlanColumns` reuses the shared `scrollWindow`/
  `fitEnd` (which always keeps `end >= start+1`), so no index-out-of-range on empty/
  done/clamped columns. `done` column stays populated — `MarkDone` keeps the plan
  in-store as `done` (`TestPlansTabKanbanColumns`, `TestMarkDoneWritesChangelogEntry`).
- **Changelog write-scope safe + deterministic.** `ChangelogDir` = `projects.Dir/.gogo/changelog`
  (home-only, mirrors `plans.Dir`); `writeChangelogEntry` is LLM-free / launch-free,
  `loadProjectRepo` and `memberFeature` are nil-safe (fall back to `source:slug`).
  A changelog-write failure returns `(p, err)` after the flip has persisted — surfaced
  by both callers (`finishPlanDone`, CLI `planDone`); a re-run short-circuits on the
  already-`done` guard (by-design, plan-stated).
- **Invariants intact.** Writes confined to `~/.gogo/`; every spawn/pickup is a
  launched `claude -p` (never a CLI state-flip); heap-stable `*formBinding` untouched;
  `A`/`n`/`c`/`+`/`x` carried unchanged; version bumped to 0.26.0 in BOTH `plugin.json`
  and `cli/main.go`; enumeration sync done across README, `skills/gogo-cli/SKILL.md`,
  `cli/main.go` printHelp, and the scaffold/version tests.

## Findings

| id | sev | pri | title | fix |
|----|-----|-----|-------|-----|
| REV-001 | minor | P2 | Auto-pickup records the fire-once key before the launch resolves → a launcher error silently strands the member (no retry, no cue) | AGENT-FIXABLE |
| REV-002 | minor | P2 | `reloadMsg`→`autoPickupCmds` wiring untested end-to-end; `TestReloadNoAutoPickupWithoutClaude` carries a doc-comment for a `TestAutoPickupReloadWiring` that does not exist | AGENT-FIXABLE |
| REV-003 | nit | P3 | Plan's "no em-dash" invariant not honored (48 added lines use `—`, consistent with existing house style) | AGENT-FIXABLE |

### REV-001 (minor) — fire-once recorded before the launch resolves
`pickup.go` `autoPickupCmds` sets `autoPickedUp[key]=true` (line 94) before the async
`autoPickupLaunch` runs. Correct for re-entrancy, but if the launcher errors
(pickup.go:110) the member is stranded: no auto-retry this lifetime AND no cue (the
"trigger manually" cue only shows for cap-blocked, not fired-and-failed). Diverges
from `finishPlanSpawn`'s record-on-success discipline. Fix: on launch error, return a
typed msg carrying the failed `featureKey` and delete it from `autoPickedUp` in
`Update` so the next reload retries.

### REV-002 (minor) — missing wiring test + stale doc-comment
No test drives `reloadMsg{}` through `Update` and asserts a pickup fired; all pickup
tests call `autoPickupCmds()` directly. `pickup_test.go:191-193` documents a
`TestAutoPickupReloadWiring` that "drives a reloadMsg through Update … and asserts a
launch fired" but sits on `TestReloadNoAutoPickupWithoutClaude`, which asserts the
opposite (0 cmds, no claude). Fix the comment and add the real integration test.

### REV-003 (nit) — em-dash invariant
48 added lines use `—` (comments, status strings, test messages), matching the
codebase's existing pervasive em-dash style but contradicting the plan's stated
"no em-dash" invariant and the owner's global rule. Cosmetic; either sweep `—`→` - `
or drop the invariant from future plans.

## Notes (not findings)
- The `done` column grows unbounded over time (MarkDone never prunes) — this is the
  explicit FR5 "kanban 4th-column parity" intent, not a defect.
- `renderCard`/`renderPlanCard` call `autoPickupBlocked` per card per render (O(cards ×
  (sources+features))) — pure Go, small boards, well within the CLI read-path bar.
