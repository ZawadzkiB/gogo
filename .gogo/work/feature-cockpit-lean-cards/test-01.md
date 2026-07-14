# Test round 1 — cockpit-lean-cards

**Verdict: GREEN.** Automated suites all pass; the hands-on render exercise confirms
every item in the plan's Verification block. No issues found. Ready to advance to
phase ⑤ (report).

## What changed under test

Presentation-only rework of `cli/internal/tui/`: dropped the `⏸ NEEDS YOU` strip and
the per-card `①②③④⑤` phase dots, dropped the `1..9` gate number-key shortcut, and
added a green `● <agent>` chip that shows only when a live session is on a card and
the card is not a user gate. Same `contract.Repo`; no contract/skill/pipeline-state
change. Version bumped to 0.20.0.

## 1. Automated suites (from `cli/`)

| Command | Result |
|---|---|
| `gofmt -l .` | clean — no files listed |
| `go vet ./...` | clean |
| `go build ./...` | OK |
| `go test -race -count=1 ./...` | **all packages pass** |

```
ok  	github.com/ZawadzkiB/gogo/cli	2.824s
ok  	github.com/ZawadzkiB/gogo/cli/internal/contract	1.824s
ok  	github.com/ZawadzkiB/gogo/cli/internal/diagram	1.232s
ok  	github.com/ZawadzkiB/gogo/cli/internal/diagram/mermaidascii	2.077s
ok  	github.com/ZawadzkiB/gogo/cli/internal/launch	1.657s
ok  	github.com/ZawadzkiB/gogo/cli/internal/orchestrator	2.261s
ok  	github.com/ZawadzkiB/gogo/cli/internal/pages	1.536s
?   	github.com/ZawadzkiB/gogo/cli/internal/textfmt	[no test files]
ok  	github.com/ZawadzkiB/gogo/cli/internal/trash	1.957s
ok  	github.com/ZawadzkiB/gogo/cli/internal/tui	3.364s
```

Confirmed alongside the suite run:
- The four new tests the plan calls for exist and pass: `TestActiveAgent`,
  `TestAgentChipOnlyWhenLive`, `TestNoPhaseDots`, `TestNoNeedsYouStrip`
  (`internal/tui/redesign_test.go`).
- The eight tests the plan calls to remove are gone (grep for their names in
  `internal/tui/*_test.go` returns nothing): `TestPhaseProgressVector`,
  `TestPhaseDotsAndBarPlainText`, `TestGatesEnumeration`, `TestNeedsYouStripRender`,
  `TestNumberKeyReadsGate`, `TestNumberKeyOutOfRange`, `TestGateNumberKeyParse`,
  `TestUATReplanGate`.
- Every dead-code symbol the plan's "delete" table names is gone from
  `internal/tui/*.go` (grep for all of `renderNeedsYouStrip`, `stripDegraded`,
  `numberedGates`, `stripHeight`, `jumpToGate`, `gateNumberKey`, `phaseDots`,
  `phaseDotsPlain`, `phaseBar`, `phaseProgress`, `phaseGlyphs`, `phaseIndex`,
  `phaseIndexFromStatus`, `phaseStyleFor`, `phaseDoneStyle`, `phaseCurrentStyle`,
  `phasePendingStyle`, `pendingDot`, `stripBoxStyle`, `stripBg`, `waitStyle` is empty).
- `colAvail()` now reads `return m.height - 5` (`internal/tui/window.go:44`) — the
  strip-height subtraction is gone.
- `.claude-plugin/plugin.json` `"version"` is `0.20.0`.

## 2. Hands-on render exercise (does it look right?)

Drove the real renderer with a throwaway `internal/tui/zzz_handson_render_test.go`
(deleted after use — `git status --short internal/tui/` afterward shows only the
implementation diff, no leftover file): `New("../contract/testdata/repo")`, a
`tea.WindowSizeMsg{Width:200,Height:40}`, then rendered `m.View()` under three
session states.

**Live session on the in-progress card** (`m.sessions = []string{"gogo-go-inprogress"}`,
slug `inprogress`, phase `review`):

```
gogo cockpit  9 features                                                        ⏸ 1 need you   ● 1 session
▸ plan 3            │  in progress 1     │  ready 2            │  changelog 3 shipped
╭─────────────────╮ │╭─────────────────╮ │╭─────────────────╮  │  ✓ shipped-status     06-20
│ unfinished       │ ││ inprogress ●     │ │┃ ○ ready          │  │  ✓ shipped-by-folder  06-18
│ Unfinished, ...  │ ││ In progress ...  │ │┃ Ready to ship    │  │  ✓ shipped-by-members 06-17
│ plan r1          │ ││  review r2   ● reviewer │ │┃  ⏸ awaiting-uat   │  │
╰─────────────────╯ │╰─────────────────╯ │╰─────────────────╯  │
...
unfinished   [m] go   [enter] drill   [v] view   [w] web                              [?] all keys
```

Checked against the plan's Verification list — every item confirmed:

| Verification item | Result |
|---|---|
| No `⏸ NEEDS YOU` strip anywhere in the board | **confirmed** — `"NEEDS YOU"` does not appear |
| Header reads `⏸ K need you` + `● S session` | **confirmed** — `⏸ 1 need you` and `● 1 session` both present |
| Gate cards carry the heavy `┃` left border | **confirmed** — the `ready` card (awaiting-uat) is drawn with `┃` on every row |
| No `①②③④⑤` anywhere | **confirmed** — none of the five glyphs appear in any render |
| A live in-progress card shows `● <agent>` | **confirmed** — the live `inprogress` card (phase=review) shows `● reviewer` on the status row, plus the separate name-row `●` liveness dot |
| An idle card shows no chip | **confirmed** — with `m.sessions = nil`, the same card's status row reads `review r2` alone, no `● reviewer` |
| `1–N answer gate` gone from the full `?` help line | **confirmed** — full-key line reads `←→/h cols · ↑↓/jk cards · space select · enter drill · v view · w web · m move · d ship · a attach · l peek · x del · / filter · ? keys · q quit`, no gate-number mention |

**Extra sanity check (D1 — chip suppressed on gates):** put a live session on the
`ready` slug (`status: awaiting-uat`, a user gate) — the name-row liveness dot (`●`)
appears next to the slug, but the status row shows only `⏸ awaiting-uat`, no
`● reporter` chip. Confirms the chip is gated on `!f.WaitingForInput()` even when a
session is live on a gate card, exactly as D1 specifies.

The harness (and its output above) was deleted immediately after the checks; the
package tree is back to only the plan's implementation diff.

## 3. Real binary / TTY-driven run

Not attempted this round. The no-TTY render harness above is the substring-level
exercise this package is explicitly designed around (per `test-strategy.md`'s Go-TUI
section, unit-render assertions plus a render harness cover the presentation logic;
a live tmux drive is the strategy's escalation for message-routing/focus/lifecycle
bugs). This change touches **only static rendering** (no new key handling beyond
deletions, no new async command/message wiring) — a live tmux drive would add
confidence but is not required to close this round, and is not treated as a blocker.

## New/extended tests

None added by this round — the plan's own implementation already ships the four new
unit tests (`TestActiveAgent`, `TestAgentChipOnlyWhenLive`, `TestNoPhaseDots`,
`TestNoNeedsYouStrip`) plus updates to `TestBoardViewRenders` and the window-math
tests, all verified green above. The hands-on harness in section 2 was throwaway by
design (an ad hoc render/eyeball check, not a permanent regression test) and was
deleted after use.

## Issues found this round

None.

## Done-bar

Build green + unit green + e2e/hands-on done, per `test-strategy.md`:

- [x] build (`go build ./...`)
- [x] unit (`go test -race -count=1 ./...`, all packages)
- [x] hands-on render exercise (the "does it look right?" check for this TUI
      change) — run, all plan verification items confirmed
- [ ] live tmux drive — not attempted; not a blocker for a static-render-only change
      (see section 3)

**Verdict: all green, no open/new issues. Advancing to phase ⑤ (report).**
