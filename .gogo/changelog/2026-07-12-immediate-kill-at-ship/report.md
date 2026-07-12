# Immediate kill-at-ship — 0.17.0

- **shipped:** 2026-07-12
- **member:** `immediate-kill-at-ship`
- **audit trail:** [.gogo/work/feature-immediate-kill-at-ship/](../../work/feature-immediate-kill-at-ship/) (full plan, decisions, review/test rounds, per-file changes)

## What shipped

`/gogo:done` now **reaps the tmux session(s) that drove a feature — immediately, and only its own** — the moment it ships. A just-shipped card stops showing a phantom "● session running" badge, and nobody has to run `gogo sweep` by hand anymore. In the same slice the board's interactive launcher **drops `remain-on-exit`**, so a finished board session's pane closes on its own instead of lingering as a dead corpse.

This completes the deferred **D5=B** refinement of the v0.15.0 kill-at-ship slice, which had only reaped on the *next* manual sweep or launch. The change is three scoped Go/skill edits plus housekeeping, all reusing the existing v0.15.0 reaper (`Sweeper`, `SessionMatchesSlug`, `KillSession`) — no new command, no new dependency, one additive CLI arg.

## Key outcomes

- **Immediate ship-reap.** `skills/gogo-done` gained a best-effort, classifier-safe reap step that runs after the members are marked terminal: `gogo sweep <member-slug>...` kills the driving `gogo-go`/`gogo-plan` session(s) and nothing else. State flip precedes the reap (load-bearing order).
- **Targeted, not whole-board.** A new `Sweeper.Only []string` seam restricts the session scan **and** the lock/registry cleanup to the named slugs (exact `SessionMatchesSlug` parse, no substring). Plain `gogo sweep` (no slug) stays the manual whole-board cleanup. Command surface: `gogo sweep [--dry-run] [<slug>...]`.
- **Self-guard.** A new `Sweeper.Self` seam (resolved by `launch.CurrentSession()` via `tmux display-message -p '#S'`) makes `Sweep()` skip its own hosting session **before** any reap rule — a `/gogo:done` running inside a board-launched `gogo-done-<slug>` session can never kill its own host mid-flight.
- **No more dead panes.** `launch.Launch()` no longer sets `remain-on-exit on`, so a board pane closes when claude exits — matching `LaunchPersistent` and the headless `-p` path. A parked gate still keeps claude (and the pane) alive.
- **Best-effort, never fails a ship.** Missing `gogo` on PATH → silent skip; the standalone sweep / next-launch reap remains the backstop.

## Decisions (one-liners)

- **D1 — reap mechanism:** chose plain `gogo sweep` at plan for minimalism, then **changed to targeted `gogo sweep <slug>` at review** — see D4.
- **D2 — `remain-on-exit` fate:** **drop it** — the badge is truthful immediately, matches the `-p`/`--attach` paths, and it only ever kept a *dead* pane.
- **D3 — best-effort reap:** **silent skip, never fail a ship** — the core loop needs no external deps.
- **D4 — ship-reap scope (raised at review):** **targeted** — a ship should touch only its own card's session, never the whole board. This fixes REV-002 and pulled the previously out-of-scope "targeted sweep" into scope.

## Review / test verdict

**Review APPROVE** (2 rounds) and **test GREEN** (1 round, all hands-on run live on a safe scratch-tmux harness): REV-002 (a plain ship-sweep could truncate another feature's concurrent `/gogo:done`) was **verified fixed** by the targeted `Only` filter; REV-001 (the ship's own host session lingers until quit — inherent to self-reaping) accepted as **works-as-designed**; REV-003 (stale command-surface docs) **fixed**. The Go race suite passes including the two new unit tests (`TestSweepSparesSelf`, `TestSweepTargetedOnlyNamedSlug`).

## Files changed (as-built)

| File | Change |
|---|---|
| `cli/internal/orchestrator/sweep.go` | `Sweeper.Self` (self-guard) + `Sweeper.Only` (targeted filter) seams; `matchesOnly()`/`inScope()`; scoped registry/lock cleanup |
| `cli/go.go` | `cmdSweep` parses `gogo sweep [--dry-run] [<slug>...]`, wires `Self`+`Only`; help updated |
| `cli/internal/launch/launch.go` | new `CurrentSession()`; dropped `remain-on-exit on` in `Launch()` |
| `cli/internal/orchestrator/orchestrator_test.go` | `TestSweepSparesSelf` + `TestSweepTargetedOnlyNamedSlug` |
| `skills/gogo-done/SKILL.md` | best-effort targeted ship-reap step + Degradation note |
| `docs/cli-contract.md` | additive `### Changed in 0.17.0` block + updated command-surface line |
| `README.md`, `skills/gogo-cli/SKILL.md` | sweep usage strings synced to `[<slug>...]` |
| `.claude-plugin/plugin.json`, `cli/main.go` | paired version bump → **0.17.0** |

## Known limitations (carried forward)

- **REV-001 (cosmetic):** the ship's own `gogo-done-<slug>` host session lingers until the user quits it (the pane then closes by construction) or a later sweep reaps the now-terminal feature. A future board-side, badge-only suppression could hide "running" on such a card — out of this feature's scope.
- **TEST-002 (deferred):** a couple of `events.jsonl` lines from earlier phases carry non-chronological placeholder timestamps — a telemetry-hygiene gap unrelated to this feature's code.

## Diagrams

- **flow** (`immediate-kill-at-ship-flow.mmd`) — the targeted ship→reap control flow (mark shipped → best-effort `gogo sweep <slug>...` → self-guard → `matchesOnly` targeted filter → terminal/orphan check → reap). A **before/after** pair exists for this kind.
- **activity** (`immediate-kill-at-ship-activity.mmd`) — the session lifecycle state machine, with the two shipped edges (immediate reap-at-ship; pane closes on exit instead of leaking).
- **class** (`immediate-kill-at-ship-class.mmd`) — the code structure: the `Sweeper.Self`/`Only` seams, `cmdSweep`'s wiring, and the `launch.CurrentSession`/`Launch` changes.
