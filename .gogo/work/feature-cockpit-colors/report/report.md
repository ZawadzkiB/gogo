# Report — `cockpit-colors` (0.22.0)

**A colored, origin-at-a-glance cockpit + the home-dir board fix.** A presentation +
additive-config follow-up to 0.21.0: default colors auto-assigned per **project** and per
**source**, rendered everywhere (board · chips · plans · plan-detail · **changelog** · config)
to match the "Gogo Cockpit" design; plus the fix for `gogo` in the home dir opening an empty
board. Ships **0.22.0**. Review APPROVE, test PASS.

## Run status

Plan accepted 2026-07-19 (D1–D5 = A). Implement 1 round. Review 1 round APPROVE (0 blockers/
majors/minors; 3 P3 nits accepted). Test 1 round PASS (no new issues; FR1 confirmed e2e via the
real binary in tmux). Gate green: `gofmt`/`go vet`/`go test -race ./...`; `gogo --version` → 0.22.0.

## Planned vs shipped

| FR | Shipped |
|---|---|
| FR1 home-dir bug | `chooseBoard(root, rootFound, listProjects, initialized, dataHome)` + `sameDir`; a found root whose `.gogo == projects.Home()` falls through to the global cockpit. `runBoard` passes `projects.Home()`. Pure test + e2e-confirmed. |
| FR2 default colors | `projects/palette.go` (8 swatches, pure strings — no lipgloss); `AssignColor` (deterministic round-robin, skip-taken, wrap); `Project.Color` added (additive, schema 1); assigned in `project add` (project + source #1) + `source add`; re-add preserves. |
| FR3 combine | `originDots(projectColor, sourceColor)` → `●P ●S` on multi-project surfaces (config switcher); single-project stays one source dot. |
| FR4 render + changelog | `tui/palette.go colorFor(hex,idx)` (swatch→adaptive / arbitrary-hex→direct / blank→index-fallback, NEVER blank); source dots on `sourceTag`/`viewSourceChips`/`planSourceDots`/plan-detail; **changelog** leading source dot + relocated trailing session dot (D3); config project/source dots + editable `label color`. |
| FR5 design fidelity | Matches TURN 3 (3a board+chips+changelog, 3b config) + TURN 4 (4a/4b dots). |
| version | `plugin.json` + `main.go` → 0.22.0; no new verb → enum-sync untouched. |

## Decisions (D1–D5 = A)

D1 inject `dataHome` into `chooseBoard` (pure-testable). · D2 persist swatch Dark hex, render
adaptive-on-match / direct-hex / index-fallback (never blank). · D2.1 the 8-swatch palette
(design teal/pink/blue verbatim; alert-red avoided so a dot never reads as "needs you"). · D3
changelog leading dot = source, live-session cue relocated to a trailing green dot (single-repo
byte-for-byte). · D4 persisted editable `Project.Color`. · D5 two dots `●P ●S`.

## Review + test outcomes

- **Review:** APPROVE. 3 P3 nits (accepted): REV-001 taken-colors logic duplicated across 3 sites;
  REV-002 legacy colorless source's fallback swatch is position-indexed (can shift on reorder,
  cosmetic/legacy-only); REV-003 config switcher `●P ●S` row not width-truncated (can wrap for a
  long name at ~width 50). All within plan latitude; candidate fast-follows.
- **Test:** PASS, no new issues. FR1 e2e (home/child → global cockpit; real repo → own board);
  color assign/preserve/wrap over ~22 CLI calls; full dot structure vs the design; garbage/hex/
  extreme-index edges degrade gracefully; `A`-draft persistence confirmed on disk before launch.

## Invariants held

CLI writes only `~/.gogo/…`; a blank/garbage color never crashes and never renders grey/blank;
single-repo board + changelog byte-for-byte; palette is a lipgloss-free shared source of truth (no
import cycle); read path LLM-free.

## Follow-ups (accepted nits)

- REV-001 extract a shared `takenColors` helper. · REV-002 stabilize the legacy fallback color
  (hash by name, not index). · REV-003 truncate the config switcher `●P ●S` row.
- The **unified all-projects board** (design 3a) remains the separate deferred feature (this only
  added the color MODEL to support it).

## TL;DR

Colors per project + per source, auto-assigned + editable, on every surface incl. the changelog
(the origin gap), matching the design; `gogo` in the home dir now opens the global cockpit.
Ships 0.22.0. Review APPROVE, test PASS. Full audit: `.gogo/work/feature-cockpit-colors/`.
