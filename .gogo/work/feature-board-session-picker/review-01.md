# Review round 01 — feature `board-session-picker`

- **Reviewer:** gogo-reviewer (phase ③, fresh eyes)
- **Date:** 2026-07-14
- **Scope reviewed:** board-session-picker only (the changelog live-session dot, the
  attach picker, the kill picker, and the `0.20.0` version bump). The overlapping
  `cockpit-lean-cards` / `changelog-cursor` hunks in the same working tree were
  deliberately NOT reviewed.
- **Files:** `cli/internal/tui/model.go`, `cli/internal/tui/update.go`,
  `cli/internal/tui/view.go` (`changelogRow`/`renderChangelogColumn`), `cli/main.go`,
  `cli/internal/tui/card_test.go`.

## Verdict: APPROVE

No open blockers or majors. One optional nit (test-coverage completeness,
agent-fixable). The change is correct, scoped, and gate-green; it does not need a
user decision.

## Gates (re-verified this round)

| Gate | Result |
| --- | --- |
| `gofmt -l .` | clean (no files listed) |
| `go vet ./...` | clean |
| `go test -race ./...` | green (all packages) |
| New tests fresh run | `TestChangelogLiveSessionDot`, `TestDrillAttachPicker`, `TestDrillKillPicker` PASS; `TestDrillKillWiring`/`TestDrillAttachWiring`/`TestDrillDegradesNoSessions` regression-green |
| `.claude-plugin/plugin.json` vs `cli/main.go` | both `0.20.0` — version bump present and matched |

## What I checked and confirmed

- **TEST-001 (heap-stable form target).** `formBinding.selected` is a new field on the
  existing pointer-behind binding. Both pickers bind `.Value(&m.binding.selected)` on a
  freshly heap-allocated `&formBinding{}` created inside pointer-receiver
  `startAttachPicker`/`startKillPicker`, whose mutations persist on the value model
  returned by `attachFeature`/`killDrill`. Completion is driven through the existing
  `updateForm`, which forwards **every** `tea.Msg` to the live form; the `keyPress`
  pump test exercises the async `NextField→nextGroup→StateCompleted` chain and passes.
- **TEST-005 (exact session→slug attribution).** The dot (`hasLiveSession`→
  `liveSessionFor`), the attach picker and the kill picker all resolve sessions via
  `liveSessionsFor`→`launch.SessionMatchesSlug` (exact boundary match, never substring).
  Both picker tests inject a substring sibling `gogo-go-inprogressX` for slug
  `inprogress` and assert it is neither offered nor targeted; the single kill/attach
  targets the picked **exact** option-value string. No cross-attribution.
- **Sentinel safety.** `killAll`/`killCancel`/`attachCancel` are non-empty, leading-space
  ASCII values that can never equal a real `gogo-*` session (`launch.ListSessions`
  filters to the `gogo-` prefix). The `selected == ""` discriminator that selects the
  single-session Confirm path is therefore never ambiguous — no extra field needed, and
  no stale carryover (binding is re-created per open).
- **Injection / write-scope safety.** Attach spawns `exec.Command("tmux",
  launch.AttachArgs(session)...)` — a single argv, no shell; kill goes through the
  `m.killer` seam. No slug or session name reaches a shell. Killing a tmux session is
  not a pipeline-state write, so the CLI remains a deterministic, LLM-free reader.
- **Error handling.** `finishKill` counts and surfaces failures (`killed N, M failed`) —
  no silent swallow. Zero-session attach/kill degrade to a clear status hint.
- **Rendered-not-just-set (standard #8).** The `attaching <session>` status is set on the
  returned model whose mode is a real `View()` path (board/drill), and the pickers render
  as full huh forms; nothing is set-but-unrendered.
- **Decisions honored.** D1 — the collapsed `✓ slug … MM-DD` list is kept; a green `●` is
  prefixed (`✓ ● slug`) only on live rows, with the rune-width math adjusted so `MM-DD`
  stays right-aligned. D2 — single-session UX is untouched (direct attach; single
  `huh.NewConfirm` for kill); pickers appear only at ≥2, and kill offers one / "all N" /
  Cancel exactly as specified.
- **Plan fidelity.** FR-1/FR-2/FR-3 implemented as planned; diff is presentation/
  interaction-only in `cli/internal/tui/` over the same `contract.Repo`; no contract /
  classifier / launch / skill / pipeline-state change crept in.

## Findings

| id | sev | pri | status | title |
| --- | --- | --- | --- | --- |
| REV-001 | nit | P3 | new | Attach picker: board-origin (≥2) branch and the ready-ship selection-preservation assertion are untested |

### REV-001 — nit — AGENT-FIXABLE
The attach picker's DRILL origin is well tested, but the BOARD origin
(`pickerFromDrill=false` → cancel restores `modeBoard`) is a real, reachable branch
with no test, and the plan's FR-2 test spec called for asserting that an attach-cancel
preserves the ready-ship multi-selection (`formPreservesSelection` now includes
`pendingAttach`) — the implemented cancel subtest asserts only mode+status. The product
code is correct on both counts; this is a test-completeness gap only. Suggested fix: add
a board-origin attach-picker subtest (open + esc → back to `modeBoard`) and extend an
attach-cancel subtest to pre-select a ready slug and assert it survives the cancel. No
product change.

## Notes for the next round

- REV-001 is optional and does not block the approve. If implement addresses it, it is a
  pure test addition in `card_test.go`.
