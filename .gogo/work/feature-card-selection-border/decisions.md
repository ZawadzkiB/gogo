# Decisions — feature `card-selection-border`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

<!-- Template for each decision — copy and fill:

## D1 — <short title>
- **Phase:** <plan | implement | review | test>
- **Question:** <the fork, stated plainly>
- **Options:**
  - A. <option> — <trade-off>
  - B. <option> — <trade-off>
- **gogo recommends:** <A / B> — <one-line why>
- **Status:** OPEN        # OPEN | RESOLVED

### RESOLVED (user, <YYYY-MM-DD>)
<the decision, in the user's terms>
-->

## D1 — What exactly marks the focused card, once the fill is gone
- **Phase:** plan
- **Question:** With the fg/bg fill removed, the cursor card needs a marker that is
  unmistakable but touches nothing inside the card. Colour alone is flattened by a
  no-colour terminal and by TTY-less `go test`, so what carries the cue?
- **Options:**
  - A. **Double-line border** (`╔ ═ ║`) in the column accent, rounded for everything else —
    zero content width, visible without colour, assertable in `View()`; slightly heavier
    ink on screen, and one focused card per column will look "stronger" than the rest.
  - B. **Colour-only border** — keep the rounded set everywhere and just recolour the
    focused card's border. Quietest visually and the smallest diff; but invisible on a
    no-colour terminal (`NO_COLOR`, `TERM=dumb`, piped output) and untestable, since
    `go test` has no TTY and lipgloss strips every colour.
  - C. **`▸` cursor inside the card** (plus an accent border) — matches the changelog /
    sessions-panel idiom, but it *is* content: it eats 2 columns of the focused card's slug
    budget, so the name row shifts as the cursor moves.
  - D. **Thick border** (`┏ ━ ┃`) — rejected outright, not offered: its sides are the exact
    `┃` glyph the gate stripe uses, so every focused card would read as a user gate (and it
    breaks `TestGateCardStripeGlyph`).
- **gogo recommends:** **A** — it is literally "only a border", costs no content width, and
  is the only option that keeps the focus cue alive on a colourless terminal (the standing
  Diagnosability bar) while staying assertable in tests. If it reads too heavy in your
  terminal, B is a one-line downgrade afterwards.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-02)
**A** — double-line border in the column accent. Accepted with the recommendation at the
plan-acceptance gate.

## D2 — Do the changelog + config-tab ROWS come along?
- **Phase:** plan
- **Question:** The same repaint-on-focus treatment (`changelogFocusStyle`, a full-row
  fg/bg bar with the dots rendered plain) is used by the collapsed changelog rows
  (`view.go:406-439`) and the config tab's project/source rows (`config_tab.go:342,375`).
  They are rows, not cards, so they cannot carry a border. Fix them in this change or not?
- **Options:**
  - A. **Include them (FR6).** Drop the fill; focus stays marked by the `▸ ` cursor those
    rows *already* render, plus an accent on the focused label. Consistent with the
    sessions panel, the drill file list and the plans list, which already mark focus with
    `▸` and no fill. Also removes the last `plain bool` parameters
    (`originDots`, `projectOriginDots`) and lets `changelogFocusStyle` go. Costs a slightly
    larger diff (two more files + `colors_test.go` call sites) and a somewhat quieter
    selection bar in those lists.
  - B. **Cards only.** Smallest possible diff, exactly the surface you complained about.
    But the changelog row and the config rows keep losing their origin/session dot colours
    on focus — the same defect, one keystroke away.
- **gogo recommends:** **A** — it is the same defect class, the marker it falls back to
  already exists in those very rows, and it leaves no "second style of selection" behind.
  Choosing B costs nothing structurally: FR6 is written so it can be dropped without
  touching FR1-FR5.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-02)
**A** — include the changelog + config-tab rows (FR6 stays in scope). Accepted with the
recommendation at the plan-acceptance gate.
