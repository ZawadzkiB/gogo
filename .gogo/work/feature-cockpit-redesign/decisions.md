# Decisions — feature `cockpit-redesign`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

## D1 — Scope / sequencing: one feature, or 1b first then 1c
- **Phase:** plan
- **Question:** Ship the redesign as ONE feature (1b + 1c together), or as TWO
  slices — **1b (refined board, deltas 1-7)** first, then **1c (needs-you strip +
  number keys + bars, deltas 8-10)**?
- **Options:**
  - A. **1b first, then 1c** — a fast, self-contained *visible* change that
    rebuilds trust after the "nothing changed" attempt; 1c builds on it. Two
    version bumps (0.18.0 then 0.19.0).
  - B. **One feature (1b + 1c together)** — a single release (0.18.0), but a bigger
    unaccepted diff and a slower first visible result.
- **gogo recommends:** **A** — the prior attempt produced no visible change, so a
  fast, obviously-different 1b board is the priority; 1c layers on cleanly.
- **Status:** RESOLVED → B (user, 2026-07-12; one feature, 1b + 1c together, ships 0.18.0)

## D2 — Phase-progress representation: dots `①②③④⑤`, segmented bars, or both
- **Phase:** plan
- **Question:** FR-4 (1b) specifies per-card **dots `①②③④⑤`**; FR-9 (1c) specifies
  **5-segment bars**. Do we keep both, or pick one?
- **Options:**
  - A. **Both, one shared model** — a single `phaseProgress(f) [5]phaseState`
    vector; **dots** are the dense-board default (1b), **bars** are 1c's fuller
    flavor where space allows. Two thin renderers, one source of truth.
  - B. **Dots only** — simpler, but drops 1c's bar element from the mockup.
  - C. **Bars only** — richer per card, but heavier on the dense 4-column board.
- **gogo recommends:** **A** — dots as the default per-card element, bars as 1c's
  flavor; both render the same vector, so keeping both is nearly free and matches
  the mockup.
- **Status:** RESOLVED → A (user, 2026-07-12; both dots + bars via one shared phaseProgress vector)

## D3 — Needs-you strip duplicates gate cards + short-terminal degradation
- **Phase:** plan
- **Question:** The 1c needs-you strip pulls each gate into an inbox at the top,
  but every gate ALSO stays in its column below (a shortcut, not a move). Confirm
  that's intended, and choose how the strip degrades when the terminal is short
  (strip + board must both fit).
- **Options:**
  - A. **Strip duplicates (shortcut), degrade gracefully** — gate stays in its
    column AND appears in the strip; when the terminal is too short, collapse the
    strip to a single summary line (or window it), reusing the `window.go` height
    budget so the board never overflows.
  - B. **Strip replaces (move gates out of columns)** — no duplication, but breaks
    the spatial model and the existing column navigation/tests.
- **gogo recommends:** **A** — the mockup treats the strip as an answer-first
  shortcut; keeping gates in their columns preserves navigation and the existing
  tests, and the height cost is bounded by graceful degradation.
- **Status:** RESOLVED → A (user, 2026-07-12; strip is a shortcut — gates stay in columns — degrade on short terminals)

---

## RESOLVED (user, 2026-07-12) — plan-acceptance gate

Plan accepted. Build the FULL mockup as one feature.

- **D1 → B** — ship **1b + 1c together** as one feature (single release **0.18.0**), not split.
- **D2 → A** — keep **both** the phase dots `①②③④⑤` (1b) and the segmented bars (1c), driven by
  one shared `phaseProgress(f) [5]phaseState` vector.
- **D3 → A** — the needs-you strip is a **shortcut** (each gate ALSO stays in its column); the
  strip **degrades gracefully** on short terminals (collapse to a summary line / window it,
  reusing the `window.go` height budget) so the board never overflows.

`state.md` → `plan-accepted`. Next: `/gogo:go cockpit-redesign` builds ②→⑤ (implement in-context,
fresh review/test), stopping at the UAT gate. Acceptance test = live TUI side-by-side vs the mockup.
