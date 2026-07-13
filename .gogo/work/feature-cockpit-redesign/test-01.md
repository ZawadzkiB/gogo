# Test round 1 — cockpit-redesign (phase ④)

Feature: restyle the terminal cockpit TUI (`cli/internal/tui/`) into the
Claude-Design **1b + 1c** mockup. The acceptance signal per plan.md is a
**live-TUI side-by-side against the mockup**, not a token/palette diff — so
this round is almost entirely hands-on rendering + a live tmux drive, not new
unit tests (the redesign already ships `redesign_test.go` pinning every FR at
the unit level; reviewed and confirmed correct in review-01.md).

## 1. Automated suite (gate)

From `cli/`:
- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go test -race ./...` — **green**, all packages (contract, diagram,
  diagram/mermaidascii, launch, orchestrator, pages, trash, tui). Redesign
  coverage confirmed present and passing: `redesign_test.go`
  (`TestPhaseProgressVector`, `TestPhaseDotsAndBarPlainText`, `TestPillLabel`,
  `TestStripeAccentGatesOnly`, `TestGateCardStripeGlyph`,
  `TestGatesEnumeration`, `TestNeedsYouStripRender`,
  `TestHeaderAttentionSummary`, `TestContextualFooterChips`,
  `TestAllKeysToggle`, `TestNumberKeyReadsGate`, `TestNumberKeyOutOfRange`,
  `TestGateNumberKeyParse`), plus the updated `tui_test.go` (`TestBoardViewRenders`,
  `TestSessionIndicatorOnCard`), `waiting_test.go` (`TestWaitingCardCue`,
  `TestBadgeAwaitingPlanAcceptance` unchanged, `TestColumnSeparatorRendered`
  unchanged), and `window_test.go` (`TestChangelogCollapsedList`,
  `TestChangelogOverflowBrowseHint`, repointed card-windowing tests). Ran
  `go clean -testcache` first to rule out a stale cache.
- Version bump confirmed in both `.claude-plugin/plugin.json` (`0.18.0`) and
  `cli/main.go` (`const Version = "0.18.0"`), and `gogo --version` on a built
  binary prints `gogo 0.18.0`.

## 2. Hands-on: deterministic render harness

Wrote a temporary `internal/tui/zz_render_harness_test.go` (deleted after this
round) that calls `New(root)` → `Update(tea.WindowSizeMsg{...})` → `View()`
and `t.Logf`s the actual rendered board — no TTY needed (lipgloss emits plain
text without one), so this is the real `Model.View()` output, not a mock.

### 2a. Real repo (`/Users/bartlomiej.zawadzki/repos/gogo`, 17 features, cockpit-redesign itself live/in-progress)

At 120×40 and 160×45 (no gates currently open on this repo → K=0):

```
gogo cockpit  17 features                                                                           ● 2 session
▸ plan 0                   │  in progress 1            │  ready 0                  │  changelog 16 shipped
                           │                           │                           │
(none)                     │╭───────────────────────╮  │(none)                     │✓ cockpit-cards-an… 07-12
                           ││ cockpit-redesign ●    │  │                           │✓ immediate-kill-a… 07-12
                           ││ Restyle the terminal  │  │                           │✓ persistent-sessi… 07-11
                           ││ c…                    │  │                           │✓ unattended-ops-i… 07-11
                           ││  running   ①②③④⑤      │  │                           │✓ cli-orchestrator  07-10
                           │╰───────────────────────╯  │                           │... (11 more rows)
sessions: gogo-go-cockpit-redesign · gogo-go-flowchart-render-legibility
no card focused                                                                                   [?] all keys
```

Confirms FR-1 (no red pill when K=0, `● 2 session` shown), FR-2 (`plan 0` /
`in progress 1` / `changelog 16 shipped`, no `(N)` form), FR-3 (`running`
pill), FR-4 (`①②③④⑤` dots), FR-6 (collapsed changelog rows `✓ slug… MM-DD`,
no boxes), FR-7 (`no card focused` + right-aligned `[?] all keys` when the
focused column is empty).

At **120×20** (forces changelog overflow): last visible row followed by
`  ↓ 2 more · enter to browse` — the exact FR-6 overflow text.

Moved focus right into "in progress" (`tea.KeyRight`) to reach the live
`cockpit-redesign` card — footer became:
```
● cockpit-redesign   [m] go   [enter] drill   [v] view   [l] peek   [a] attach   [w] web                              [?] all keys
```
Confirms FR-7's "a live card leads with green ●" (glyph-level here; color
confirmed separately via the live tmux drive, §4) and that peek/attach chips
only appear for a live session.

### 2b. Fixture repo (`../contract/testdata/repo` — has the one built-in awaiting-uat gate)

At 140×40, the needs-you strip renders with exactly one uat-gate row:
```
gogo cockpit  9 features                                                                                 ⏸ 1 need you   ● 2 session
╭─────────────────────────────────────────────────────────────────... 
│ ⏸ NEEDS YOU (1)
│  uat gate   ready
│ report done, awaiting your verification
│ [1] read report · [d] ship   ▓▓▓ ▓▓▓ ▓▓▓ ▓▓▓ ▓▓▓
╰─────────────────────────────────────────────────────────────────...
```
and the `ready` card ALSO still renders in its column below, with the `┃`
gate-stripe and `⏸ awaiting-uat` pill — confirming **D3** (strip is a
shortcut, not a move — both places show the gate simultaneously).

At **100×14** (short terminal): the strip **degrades to a one-line summary**
(`⏸ NEEDS YOU (1) — press 1–1 to answer · ? for keys`) and the board still
renders fully within the given height with `↓ N more` windowing — confirming
**D3**'s graceful degradation, never an overflow.

### 2c. Purpose-built multi-gate workspace (all three gate types + multi-gate strip)

Built a temporary `.gogo/work/` (scratchpad-only, deleted after this round)
with four features: `plan-gate-demo` (awaiting-plan-acceptance),
`decision-gate-demo` (waiting-for-user, mid-phase), `uat-gate-demo`
(awaiting-uat, with a real `report/report.md`), and `flowing-demo`
(implementing, no gate — the negative case).

At **150×45 (tall, full strip)**:
```
gogo cockpit  4 features                                             ⏸ 3 need you   ● 2 session
╭───────────────────────────────────────────────────────────────────────╮
│ ⏸ NEEDS YOU (3)
│  plan gate   plan-gate-demo
│ plan ready for your acceptance
│ [1] read plan · [m] accept   ▓▓▓ ░░░ ░░░ ░░░ ░░░
│ ─────────────────────────────────────────────────────────────────────
│  decision gate   decision-gate-demo
│ parked on a decision — resume with your answer
│ [2] read state · [m] resume   ▓▓▓ ▓▓▓ ▓▓▓ ░░░ ░░░
│ ─────────────────────────────────────────────────────────────────────
│  uat gate   uat-gate-demo
│ report done, awaiting your verification
│ [3] read report · [d] ship   ▓▓▓ ▓▓▓ ▓▓▓ ▓▓▓ ▓▓▓
╰───────────────────────────────────────────────────────────────────────╯
▸ plan 1               │  in progress 2        │  ready 1              │  changelog 0 shipped
┃ plan-gate-demo       │┃ decision-gate-demo    │┃ ○ uat-gate-demo      │(none)
┃ Demo plan gate       │┃ Demo decision gate    │┃ Demo uat gate        │
┃ ⏸ accept plan ①②③④⑤  │┃ ⏸ decision ①②③④⑤      │┃ ⏸ awaiting-uat ①②③④⑤ │
                        │╭──────────────────────╮│                      │
                        ││ flowing-demo          ││                      │
                        ││  implement r2 ①②③④⑤   ││                      │
plan-gate-demo   [m] accept   [enter] drill   [v] view   [w] web                          [?] all keys
```
This single render confirms, in one shot: **all three gate kinds** in the
strip (plan/decision/uat pills, each correctly colored per FR-8), **the
segmented bar** (FR-9) reflecting each gate's actual phase (plan-gate at
plan → 1/5 filled; decision-gate at review → 3/5; uat-gate at report → 5/5
filled), **the `┃` left stripe** (FR-5) on all three gate cards (independent
of which is focused) and its **absence** on `flowing-demo` (plain `│`
border), and **phase dots** (FR-4) on every card.

At **150×14 (short)**: strip collapses to `⏸ NEEDS YOU (3) — press 1–3 to
answer · ? for keys`; the board (all 4 cards across 3 non-empty columns)
still renders within 14 rows with `↓ 1 more` windowing on "in progress" —
**no overflow**.

### 2d. Interactive keys (FR-10) driven against the multi-gate workspace

Sent real `tea.KeyMsg`s through `Update`:
- `"2"` → focused jumped from `plan` col/row0 to `in progress` col/row0
  (`decision-gate-demo`) and opened its drill/file-list via `quickView`
  (in-progress has no default artifact, so it correctly stays on the file
  list rather than crashing) — footer after `esc` reads
  `decision-gate-demo   [m] go   ...`, confirming the jump landed on the right
  card.
- `"3"` (from the fresh board) → jumped to `ready` col/row0
  (`uat-gate-demo`) and opened `report/report.md` in the async markdown
  viewer (`⠋ rendering report/report.md…`) — confirms `gateFor`'s `read`
  label ("read report") maps to the actual artifact quickView opens.
- `"?"` toggled the full pre-redesign key list on/off (verified again live,
  §4).
- `"9"` (out of range, only 3 gates) → status line
  `no gate 9 — 3 need you (1–3)`, no crash, no move — matches
  `jumpToGate`'s out-of-range branch exactly.

## 3. Live TTY drive (tmux) — the real acceptance test

`go build -o /tmp/gogo-cockpit-test .` succeeded; `--version` printed
`gogo 0.18.0`. Both `tmux` and `script` are present on this host, so per
test-strategy.md's "Go TUI... live tmux driving for integration" this is
**not** the tmux-absent fallback — the interactive path was driven for real.

Launched detached: `tmux new-session -d -s gogo-test-tui-$$ -x 220 -y 50
"cd <multi-gate-workspace> && /tmp/gogo-cockpit-test"`. `capture-pane -p`
reproduced **exactly** the same structure as the deterministic harness (§2c),
confirming the harness renders are faithful to a real terminal launch (not an
artifact of the test-only code path).

**Real color confirmed** via `capture-pane -e` (raw ANSI): decoded the escape
sequences and every one matches its design-spec token exactly:
- `⏸ K need you` — fg `38;2;255;107;107` (`#ff6b6b`, waitAccent) on bg
  `48;2;42;23;25` (`#2a1719`, redTint) — the mockup's red-on-tinted pill.
- `● S session` — fg `38;2;87;217;119` (`#57d977`, sessionDot) — matches.
- Needs-you strip background — `48;2;23;27;36` (`#171b24`, **exactly** the
  design token's "strip card bg").
- `plan-gate-demo` slug — bold fg `38;2;230;233;239` (`#e6e9ef`, titleText).
- "plan ready for your acceptance" — fg `38;2;183;189;201` (`#b7bdc9`,
  secondaryText) — matches the mockup's secondary body color.
- `[1] read plan · [m] accept` — fg `38;2;154;160;170` (`#9aa0aa`, dimText).
- Segmented bar — current-phase segment bold amber `38;2;230;161;73`
  (`#e6a14a`), pending segments `38;2;73;80;96` (≈`#4a5060`, pendingDot).

This is the **side-by-side-against-the-mockup** evidence the plan calls the
real acceptance signal — not just substrings, but the actual rendered colors
matching the distilled design tokens.

Drove keys live via `tmux send-keys`:
- `"2"` → pane showed the `decision-gate-demo` drill/file-list panel — same
  jump as the harness.
- `Escape` (waited ~1.6s for curses `ESCDELAY`) → back to the board, focus
  visibly moved to "in progress" (`▸` marker shifted column).
- `"?"` → pane showed the full key list
  (`←→/h cols · ↑↓/jk cards · space select · enter drill · v view · w web ·
  m move · d ship · a attach · l peek · x del · 1–N answer gate · / filter ·
  ? keys · q quit`), then `"?"` again → back to the contextual footer.
- `"9"` → status line `no gate 9 — 3 need you (1–3)`, exactly matching the
  harness.

**Cleanup**: killed the tmux session (`tmux kill-session`), removed
`/tmp/gogo-cockpit-test`, deleted the scratch `.gogo/` multi-gate workspace,
and deleted the temporary `zz_render_harness_test.go` — confirmed
`git status --short internal/tui/` shows only the implementer's real changes
afterward, and re-ran the full gate (`gofmt`/`vet`/`test -race`, all green)
with the harness removed.

## 4. FR-by-FR verdict (against design-spec.md + plan.md)

| FR | What was checked | Result |
|----|----|----|
| FR-1 header attention summary | pill hidden at K=0, shown + correctly colored at K>0; session count | ✅ match |
| FR-2 column header restyle | `title N` (no parens), dim count, `▸` focus marker | ✅ match |
| FR-3 status pills | `⏸ accept plan` / `review r2` / `implement r1` / `⏸ awaiting-uat` / `running` / `⏸ decision`, correct tint per state | ✅ match |
| FR-4 phase dots | `①②③④⑤` on every card, correct done/current/pending split verified via the segmented-bar analog (same vector) | ✅ match |
| FR-5 left accent stripe | `┃` heavy border, red on plan/decision gates, purple on uat, present independent of focus, absent on `flowing-demo` | ✅ match |
| FR-6 collapsed changelog | `✓ slug… MM-DD` rows, no card boxes, `↓ N more · enter to browse` overflow | ✅ match (exact text) |
| FR-7 contextual footer | focused card's key-chips, green `●` lead for a live card, `[?] all keys` right-aligned, `?` reveals the full list | ✅ match |
| FR-8 needs-you strip | `⏸ NEEDS YOU (N)`, one row-group per gate-type pill + one-liner + `[n] read… · answer`, gates ALSO stay in their columns (D3) | ✅ match |
| FR-9 segmented bars | 5-segment `▓`/`░` bar on the same `phaseProgress` vector, rendered on the strip's gate rows | ✅ match |
| FR-10 number keys + `?` | `1..N` jump-and-quickView per gate (verified all 3 gate kinds), out-of-range hint, `?` toggle — all driven live | ✅ match |
| D3 short-terminal degradation | strip → one-line summary below the threshold; board never overflows at h=14/h=20 (real repo, fixture, multi-gate) | ✅ match |

## 5. Findings

| id | sev | pri | status | title |
|----|-----|-----|--------|-------|
| TEST-001 | nit | P3 | new | `update.go:212` doc-comment on `jumpToGate` still says `[g] resume` — stale since REV-001 renamed the actual key to `[m] resume` |

### TEST-001 (nit, AGENT-FIXABLE)
The rendered strip and the live TUI both correctly say `[m] resume` (REV-001
verified fixed in code and confirmed live). Only the **comment** above
`jumpToGate()` in `update.go` (line ~212) still reads "...the board move keys
— [m] accept / [d] ship / [g] resume — act on it)..." — a documentation
inconsistency, not a behavioural bug. See `test/issues.json` for the proposed
fix.

## Verdict

**GREEN, with one trivial nit.** Build/vet/test gate is green. Every FR-1
through FR-10 and D1/D2/D3 element was exercised hands-on and matches the
1b/1c mockup — verified twice: once via a deterministic render harness across
the real repo, the existing fixture, and a purpose-built multi-gate
workspace, and once via a **live tmux drive of the actual built binary**,
including raw-ANSI confirmation that the rendered colors match the design
tokens exactly. No hands-on check was blocked (tmux, script, and Node were
all available; the Go build succeeded). The one finding (TEST-001) is a
doc-comment nit with no user-visible effect — it does not, on its own, change
the done-bar verdict on visual fidelity, but per the living-issues-list
convention it is recorded as `new`/`open` for the router to fold into the
next implement round (or accept as `wontfix`, at the orchestrator/user's
call, given its triviality).
