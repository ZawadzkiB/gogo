# Review — cockpit-lean-cards · round 01

- **Track:** review (phase ③) · **Round:** 1 · **Date:** 2026-07-14
- **Scope:** working-tree diff under `cli/internal/tui/` + `.claude-plugin/plugin.json`
  (the lean-cards change only — the in-tree collapsed-changelog focus-cursor hunks are
  a separate, out-of-scope change per `plan.md` and were **not** reviewed).
- **Reviewer:** fresh eyes (did not write the code).

## Verdict: APPROVE

No open blockers or majors. No verified issues raised. Presentation-only change,
faithful to the accepted plan and its two settled decisions (D1, D2); build gates green.

## Findings

_None._ Zero verified issues this round.

## What was checked (and passed)

**Build gates (re-run, not trusted)**
- `gofmt -l .` clean · `go vet ./...` clean · `go build ./...` clean · `go test -race -count=1 ./...` green (all packages).
- The four new tests pass: `TestActiveAgent`, `TestAgentChipOnlyWhenLive`, `TestNoPhaseDots`, `TestNoNeedsYouStrip`; the retuned `TestBoardViewRenders` / window tests pass.

**Correctness — `activeAgent` (model.go:326)**
- Phase→agent map matches FR-6 exactly, incl. `knowledge`/`report` → `reporter` (agrees with `contract.EventsPhase`, contract.go:107). `done`/`""`/unknown → `""` (no chip).
- Empty-phase status fallback (`implementing`/`reviewing`/`testing`) matches the plan; phase takes precedence, no conflict. Verified against `TestActiveAgent`'s table.

**Chip gating (view.go:311)** — `hasSession && !f.WaitingForInput()` matches D1. `WaitingForInput()` (contract.go:95) is the union of the three user-gate statuses, so all gate cards are correctly excluded. In practice the pill under a chip is always a short non-gate label, so no card-width overflow. Green treatment: unfocused uses `sessionStyle` (green); focused renders plain because the frame carries one fg+bg fill (matches the plan and the name-row dot tactic).

**Focused-card width reservation (view.go:335)** — `truncate(pillLabel(f), width-len([]rune(chip))) + chip` keeps the badge line ≤ `width` runes when the pill is long, and shorter when it fits; negative `max` is clamped safely inside `truncate` (no panic). Consistent with the removed `phaseDotsPlain` reservation it replaces.

**`needsYouCount` / header (model.go:454, view.go:75)** — counts `WaitingForInput()` across all four columns, byte-for-byte equivalent to the old `len(m.gates())` it replaces; header text (`⏸ K need you`) unchanged. Column 3 (changelog) contributes 0 as before.

**Dead-code completeness** — grep across `internal/` finds **no** dangling reference to any deleted symbol (`phase*`, `gate*`, `strip*`, `waitStyle`, `pendingDot`, `jumpToGate`, `gateNumberKey`, `numberedGates`). Nothing kept is now dead: `waitingMarker`, `isUATReplan`/`uatRound`, `sessionStyle`, `secondaryStyle`/`faintStyle`, the pill styles all remain in use (verified). The plan's three verified divergences (`waitStyle` dies, `TestUATReplanGate` removed, `TestBoardViewRenders` updated) are all reflected.

**`colAvail` retune (window.go:43)** — `return m.height - 5` matches the plan; the window tests re-tune correctly (20→13 and 9→7 both preserve the intended `colAvail`).

**Conventions & scope** — ASCII except the intentional glyphs (`●`, `⏸`, `✓`); diff minimal and scoped to the plan; version bumped `0.19.0 → 0.20.0` (behavioural change). Presentation-only: no `contract` / classifier / skill / pipeline-state change. No docs/README/skill described the removed strip / phase dots / number-key shortcut, so there is no cross-file enumeration drift to sync (the `①②③④⑤` in `docs/` denote pipeline stages, not the per-card dots).

## Route

Clean round → advance to **④ test**.
