# Test round 01 — feature `board-session-picker`

- **Tester:** gogo-tester (phase ④)
- **Date:** 2026-07-15
- **Scope:** FR-1 (changelog live-session dot), FR-2 (attach picker), FR-3 (kill
  picker), the `cli/main.go` version bump. This is a terminal bubbletea TUI — the
  bundled Playwright MCP does not apply (test-strategy.md §0.9.0/0.10.0); the two
  mandatory layers are pure unit tests + a live tmux drive.

## Verdict: GREEN — done-bar met

Build + unit gates green, full suite green, and all three surfaces were driven
live to completion in a real tmux pane (not just picker render — one real kill
was executed and observed). No hands-on check was blocked. No issues found;
`test/issues.json` round 1 has zero open/new findings.

## Level 1 — pure unit (`cd cli && gofmt -l . && go vet ./... && go test -race ./... -count=1`)

| Gate | Result |
| --- | --- |
| `gofmt -l .` | clean, no files listed |
| `go vet ./...` | clean |
| `go test -race ./... -count=1` | **all 9 packages ok**: `cli`, `internal/contract`, `internal/diagram`, `internal/diagram/mermaidascii`, `internal/launch`, `internal/orchestrator`, `internal/pages`, `internal/trash`, `internal/tui` |

New/targeted tests confirmed passing (verbose run):
- `TestChangelogLiveSessionDot` — PASS
- `TestDrillAttachPicker` — PASS, including subtests:
  - `opens_a_picker_listing_each_session,_excludes_the_sibling`
  - `selecting_a_session_attaches_exactly_it`
  - `cancel_attaches_nothing_and_restores_the_drill`
  - `board-origin_cancel_restores_the_board_and_keeps_the_ready-ship_selection` (the REV-001 fix)
- `TestDrillKillPicker` — PASS, including subtests:
  - `picker_lists_each_session_+_all_+_Cancel,_excludes_the_sibling`
  - `selecting_one_session_kills_exactly_it_once`
  - `all_N_kills_each_session_once`
  - `Cancel_kills_nothing_and_stays_on_the_drill`
- Regression: `TestDrillAttachWiring`, `TestDrillKillWiring` (all 3 subtests),
  `TestDrillDegradesNoSessions`, `TestFormSingleConfirmLaunches`,
  `TestFormMergedReleaseLaunches`, `TestChangelogFocusCursor` — all PASS.

`go build -o /tmp/gogo-e2e ./` succeeded; `gogo --version` → `0.20.0`, matching
`.claude-plugin/plugin.json`.

## Level 2 — live TUI drive via tmux (hands-on)

tmux 3.7b present on host. Built the binary, created throwaway fixture sessions
against **real, already-shipped** changelog slugs (chosen to not collide with
any of this host's genuinely-live sessions, which were enumerated first and
left untouched):

- `gogo-done-skill-extraction` — single dot exercise (`.gogo/changelog/2026-06-29-skill-extraction`).
- `gogo-go-docs-and-verified-discovery` + `gogo-plan-docs-and-verified-discovery`
  — ≥2-session exercise (`.gogo/changelog/2026-06-30-docs-and-verified-discovery`).

Launched `tmux new-session -d -s gogo-e2e -x 220 -y 50 '/tmp/gogo-e2e'` from the
repo root and drove it with `send-keys` + `capture-pane`:

1. **Header + FR-1 dot.** Header read `● 11 session` (7 real host sessions + the
   3 fixtures + the current pipeline's own live `board-session-picker` card, seen
   correctly separately). The collapsed changelog column showed a green `●`
   prefix (`✓ ● <slug>`) on exactly the rows with a live session —
   `cockpit-lean-cards`, `cockpit-redesign` (pre-existing real sessions),
   `docs-and-verified-discovery`, `skill-extraction` (the fixtures) — and plain
   `✓ <slug>` on every idle row. `MM-DD` stayed right-aligned on dotted rows.
2. **Drill.** Navigated to the changelog column, moved the cursor onto
   `docs-and-verified-discovery`, pressed `enter` → drilled in; the card showed
   both fixture sessions as `● untracked live`.
3. **FR-2 attach picker.** Pressed `a` over the 2-session card → rendered
   exactly per plan: title `Attach which session for docs-and-verified-discovery?`,
   one row per session name, plus `Cancel`. Pressed `esc` → status line read
   `cancelled`, both sessions still listed live (no attach fired), back on the
   drill.
4. **FR-3 kill picker.** Pressed `K` over the same card → rendered
   `Kill which session for docs-and-verified-discovery?`, one row per session,
   an `all 2 sessions` row, and `Cancel` — exactly per plan. Selected the first
   session and pressed `enter` (a **real completion**, not just a render check,
   since these were disposable fixtures): status read `killed 1 session`, the
   drill's session list dropped to just the sibling
   (`gogo-plan-docs-and-verified-discovery`), and `tmux list-sessions` confirmed
   `gogo-go-docs-and-verified-discovery` was actually gone while the sibling
   (TEST-005 exact-match, never substring) was untouched. Backed out to the
   board — header count correctly dropped to `● 10 session`.
5. **Cleanup.** Killed the two remaining fixture sessions
   (`gogo-done-skill-extraction`, `gogo-plan-docs-and-verified-discovery`) and
   the `gogo-e2e` driver session. `tmux list-sessions` afterward showed exactly
   the same 7 sessions present before the drive — no stray sessions left, no
   real session touched.

No hands-on check was blocked — tmux, the build, and a real fixture-bearing
`.gogo/` tree were all available, and every plan-listed acceptance signal (dot,
attach picker, kill picker with "all N") was driven to completion.

## New/extended tests

None added this round — `TestChangelogLiveSessionDot`, `TestDrillAttachPicker`
(with the board-origin + selection-preservation subtests), and
`TestDrillKillPicker` already exist from implement r1/r2 and cover FR-1/FR-2/FR-3
at the pure-unit level exactly as the plan's Tests section specifies; the live
tmux drive in this round exercised the same surfaces end-to-end and found no gap
warranting a new test.

## Issues this round

None. `test/issues.json` round 1: 0 open/new findings.

## Done-bar check (test-strategy.md)

- [x] Build + unit + e2e green (`gofmt`, `go vet`, `go test -race ./...`)
- [x] Hands-on drive completed at every relevant level (live tmux, all 3 surfaces)
- [x] No blocked hands-on checks
- [x] No open/new issues

**Verdict: all-green — ready to advance to ⑤ report.**
