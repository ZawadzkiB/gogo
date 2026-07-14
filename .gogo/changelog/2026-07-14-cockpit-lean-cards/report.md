# cockpit-lean-cards - leaner cockpit cards: drop the needs-you strip + phase dots, add a live-only agent chip

- **shipped:** 2026-07-14 · CLI **0.19.0 -> 0.20.0**
- **members:** `cockpit-lean-cards` (single feature)
- **full audit trail:** [.gogo/work/feature-cockpit-lean-cards/](../../work/feature-cockpit-lean-cards/)

## What shipped

The gogo terminal **cockpit board** (`cli/internal/tui/`) is now leaner and more legible.
Each card answers two questions at a glance: **what state** the ticket is in (the status
pill) and **who is on it right now** - a green `● <agent>` chip that appears **only** when
a live session is on a **non-gate** card. The heavy `┃` left border becomes the single
**"act now"** cue. This walks back three elements the 0.18.0 redesign had added: the
`⏸ NEEDS YOU` inbox strip, the per-card `①②③④⑤` phase dots, and the `1..9` gate
number-key shortcut. The change is **presentation-only**, rendered over the **same
`contract.Repo`** the board already reads - no contract, classifier, skill, or
pipeline-state change; the CLI stays a deterministic, LLM-free reader that never mutates
pipeline state. Ships as **0.20.0**.

## Key outcomes

- **Row 3 is now `status pill [+ agent chip]`.** `renderCard` computes the chip once
  (`agent := activeAgent(f)`, gated on `hasSession && !f.WaitingForInput()`) and renders it
  green on an unfocused card, plain on a focused one (the focus frame carries a single
  fg+bg fill, so a colored chip would punch a hole); the chip's rune width is reserved from
  the pill's truncation budget, and the pill takes the full width when there is no chip.
- **`activeAgent(f)` is a pure phase->agent map** - `plan->analyst`, `implement->developer`,
  `review->reviewer`, `test->tester`, `knowledge|report->reporter`, `done`/unknown -> `""`,
  with a status fallback when `f.Phase` momentarily lags telemetry. `reporter` is a display
  label only (no `gogo-reporter` agent file was added).
- **The header count is honest.** `⏸ K need you` now reads a new `needsYouCount()`
  (`WaitingForInput()` across all four columns) - byte-for-byte the old `len(gates())`,
  minus the strip machinery.
- **Dead code fully swept.** The strip renderer + its helpers, the phase-progress vector and
  its renderers, the gate enumerator, the number-key path, and their now-unused styles were
  all removed, with the plan's three flagged divergences (`waitStyle`, `TestUATReplanGate`,
  `TestBoardViewRenders`) reconciled - nothing kept is now dead.
- **Fully unit-pinned.** Every touched element stays substring-assertable (no TTY under
  `go test`); 4 new tests replaced the 8 removed ones, `go test -race` green across all 9
  packages.

## Decisions (one-liners)

- **D1 - agent chip only when live:** render the green `● <agent>` chip **only** when
  `hasLiveSession(slug) && !WaitingForInput()` - the chip means "an agent is on this *right
  now*," a distinct signal from the status pill; gate cards (parked on the user) show the
  pill alone. (Pre-confirmed with the user during planning.)
- **D2 - remove the `1..9` gate number-key:** deleted `jumpToGate`, `gateNumberKey`, the
  number-key branch, and its `1-N answer gate` help text - the shortcut only ever made sense
  as the strip's answering surface; the left-border cue + arrow navigation replace it.
  (Pre-confirmed with the user during planning.)

No decisions arose during implement / review / test - the run was clean end to end.

## Review / test verdict

**Review round 1 - CLEAN / APPROVE, 0 findings** (chip gating vs D1, focused-card width
reservation, `needsYouCount()` equivalence, and dead-code completeness all verified).
**Test round 1 - GREEN, 0 findings** (`gofmt`/`go vet`/`go build` clean, `go test -race`
green across 9 packages; hands-on no-TTY render confirmed no strip, no dots, the live-only
chip, and the retired number-key help). A real-binary/TTY tmux drive was deferred as a
non-blocker for a static-render-only change.

## Follow-ups & known limitations

- **TTY drive deferred (non-blocker):** no real-binary tmux/TTY drive this round - the
  change is static-render-only with no new key or async wiring.
- **Independent in-tree change rides along:** the pre-existing, uncommitted changelog
  focus-cursor change (the collapsed-changelog `▸` selection bar) is out of scope for this
  feature but ships in the same 0.20.0 working tree.

## Diagrams

- `cockpit-lean-cards-flow.mmd` - the as-built lean board render flow: no strip, no phase
  dots; row 3 = status pill + live-only agent chip; the heavy `┃` left border is the sole
  act-now cue. The plan-time as-is baseline sits in `before/` for side-by-side compare.

This is a presentation-only render change - **no new types** (no class diagram), **no new
runtime interaction** (no sequence), and **no new status transitions** (no activity
diagram), so `flow` is the only kind that carries signal.

---

*The full audit trail - every review/test round, the per-file change table, and the
decisions detail - lives in [.gogo/work/feature-cockpit-lean-cards/](../../work/feature-cockpit-lean-cards/).*
