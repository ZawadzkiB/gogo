# Cockpit redesign — distilled design spec (source of truth for implement)

Fetched from Claude Design via DesignSync on 2026-07-12.
- **Project:** `83feef99-0ae8-4229-ad4e-d50563a4a75e` ("Dashboard redesign planning")
- **File:** `Gogo Cockpit.dc.html` (re-fetch with `DesignSync get_file` for the raw HTML)
- **Targets:** variant **1b (refined TUI)** + variant **1c (needs-you strip)**.
- **Out of scope:** 1a (faithful recreation of the *current* board — reference only) and
  **1d (phone companion)** — that is a separate web app that "consumes .gogo data", NOT this
  terminal cockpit.

The whole design is terminal-shaped and **lipgloss-buildable** (1b is explicitly labelled so).
Everything below is a monospace TUI over the SAME `contract.Repo` the board already reads.

---

## Design tokens (dark) — ALREADY present in styles.go, verify only

| Token | Hex | styles.go name | Status |
|---|---|---|---|
| bg | `#0b0d12` | (terminal bg) | n/a |
| card bg | `#12151c` | — | n/a (border-only cards today) |
| card bg (focus/selected) | `#222834` | `focusBg` | ✓ present |
| strip card bg | `#171b24` | — | NEW (needs-you strip) |
| subtle border | `#3a3f4b` | `subtleBorder` | ✓ present |
| faint border/rule | `#262b36` | — | NEW (column rules, header underline) |
| plan blue | `#7aa8ff` | `columnAccent[0]` | ✓ present |
| in-progress amber | `#e6a14a` | `columnAccent[1]` | ✓ present |
| ready green | `#5db97a` | `columnAccent[2]` | ✓ present |
| changelog muted | `#9aa0aa` | `columnAccent[3]`/`dimText` | ✓ present |
| session live dot | `#57d977` | `sessionDot` | ✓ present |
| needs-you red | `#ff6b6b` | `waitAccent` | ✓ present |
| uat purple | `#b392f0` | `uatAccent` | ✓ present |
| title text | `#e6e9ef` | `titleText` | ✓ present |
| bright title (focus) | `#f2f4f8` | `focusFg` | ✓ present |
| body/dim | `#9aa0aa` | `dimText` | ✓ present |
| secondary body | `#b7bdc9` | — | NEW (light body on strip/footer) |
| faint (dates/pending) | `#5f6572` / `#4a5060` | — | NEW (changelog dates, pending dots) |
| font | JetBrains Mono | (terminal font) | n/a |

**Conclusion: the palette is done. The redesign is layout + new elements, not recoloring.**

---

## 1b — Refined TUI (incremental refinement of the existing 4-column board)

### Header — identity + attention summary
```
gogo cockpit   14 features                         ⏸ 2 need you   ● 1 session
```
- Left: `gogo cockpit` (bold) + `N features` (dim).
- Right-aligned: a **needs-you pill** `⏸ K need you` — red text on `rgba(255,107,107,.1)`
  bg, `rgba(255,107,107,.35)` border — shown only when K>0; then `● S session` in green.
- K = count of cards where `WaitingForInput()` (decision/plan/uat gates). S = live sessions.

### Column header — underlined title + dim count
```
plan 1        (accent color, bold; count is dim, weight-400; border-bottom 1px #262b36)
```
- Replaces current `plan (0)` / `▸ plan (0)`. Count is a trailing dim number, not `(N)`.
- Keep a focus indicator for the active column (accent underline is heavier, or keep ▸).

### Card (plan/in-progress/ready columns) — richer, 3 rows + dots
```
┌─ (left border 3px accent when gate) ────────┐
│ workspace-changelog-viewer            ●      │  ← name (bright) + live session dot
│ Workspace-level changelog viewer            │  ← one-line desc (dim / secondary)
│ [review r2]    ①②③④⑤                        │  ← status PILL + phase dots
└─────────────────────────────────────────────┘
```
- **Status pill:** the badge becomes a chip — colored text on a tinted bg
  (`rgba(accent,.12)`), rounded. Values: `⏸ accept plan` (red), `review r2` (amber),
  `implement r1` (amber), `⏸ awaiting-uat` (purple), `running`/`● session`.
- **Phase dots `①②③④⑤`:** one glyph per phase (plan/implement/review/test/report).
  green `#5db97a` = done, amber `#e6a14a` = current, grey `#3a3f4b` = pending. Derived
  from the feature's current phase + round. This is the headline NEW per-card element.
- **Left-border accent stripe:** `border-left 3px` — red `#ff6b6b` when the card needs
  you (plan/decision gate), purple `#b392f0` at the uat gate — independent of focus.
- Focused card keeps the full accent border + `#222834` bg (as today).

### Changelog column — COLLAPSED list (not full cards)
```
changelog  10 shipped        (muted header, underlined)
✓ persistent-session-orch…                     07-08
✓ unattended-ops-input-signals                 07-06
✓ cli-orchestrator                             07-04
...
↓ 3 more · enter to browse
```
- Rows: `✓ slug` (secondary `#b7bdc9`, truncated) left, `MM-DD` (faint `#5f6572`) right.
- No borders/boxes. Overflow → `↓ N more · enter to browse`.

### Contextual footer — the focused card's keys (not the static help line)
```
● workspace-changelog-viewer  —  [l] peek  [a] attach  [enter] drill  [w] web        [?] all keys
```
- Shows the FOCUSED card's applicable actions as little key-chips (`bg #222834`,
  `border #3a3f4b`). A live card leads with the green `●`. `[?] all keys` right-aligned
  reveals the full key list (today's long help line moves behind `?`).

---

## 1c — Needs-you strip (structural: an answer-first inbox above the board)

### Top strip — gates pulled OUT of the columns into an inbox
```
┌ ⏸ NEEDS YOU (2) ──────────────────────────────────────────────────────────────┐
│ [plan gate] diagram-viewer-and-uml-diff                                         │
│ plan ready for acceptance — 6 changes, 2 new files, mermaid set drawn           │
│ [1] read plan · [m] accept                                                      │
│ ───────────────────────────────────────────────────────────────────────────── │
│ [uat gate]  changelog-merged-entries                                            │
│ report done, awaiting your verification — merged entries ship as one            │
│ [2] read report · [d] ship                                                      │
└─────────────────────────────────────────────────────────────────────────────────┘
```
- Red-bordered box (`rgba(255,107,107,.4)` border, `rgba(255,107,107,.05)` bg), title
  `⏸ NEEDS YOU (N)` red bold.
- One card per gate. **Gate-type pill:** `plan gate` (red) / `uat gate` (purple) — the
  decision gate would be a third (red). Then feature name + a one-line "what's blocked".
- **Number-key shortcut per gate:** `[1] read plan · [m] accept`,
  `[2] read report · [d] ship`. Pressing `1..N` jumps to / answers gate N directly.
- Every gate ALSO still appears in its column below (the strip is a shortcut, not a move).

### Board below — segmented phase progress bars replace/augment the dots
```
workspace-changelog-viewer ● review r2
▓▓▓▓ ▓▓▓▓ ▓▓▓▓ ░░░░ ░░░░     ← 5 segments: green done · amber current · faint pending
```
- Each card carries a 5-segment bar (one per phase), same color semantics as 1b's dots.
  1c uses full-width segmented bars; 1b uses compact dots — pick per card density.
- Changelog collapses further to a `shipped N` list (`✓ slug`, `↓ N more · enter to browse`).

### Footer (1c)
```
[1–2] answer a gate   [/] filter   [?] all keys
```

---

## Current-code anchor (the delta is here)

- `view.go` — `viewBoard` (header/columns/help), `renderColumn`, `columnHeader`,
  `renderCard`, `cardBadgeText`/`badgeStyleFor` (badge → pill), `sessionsLine`.
- `styles.go` — tokens (mostly present); ADD: faint `#262b36`/`#5f6572`, secondary
  `#b7bdc9`, strip bg `#171b24`; ADD pill + phase-dot/progress-bar + left-stripe styles.
- `model.go` — `columnTitles`, `badge()` (feeds the pill text), `WaitingForInput()` set
  drives the needs-you count + strip; ADD a phase→dot/segment mapper + gate enumeration.
- `update.go` — number-key (`1..N`) handling for the 1c strip; `?` toggles full help.

**Fidelity rule:** the redesign must be *visibly* different from today's board. The
acceptance check is a side-by-side of the live TUI against 1b/1c, not a token diff.
