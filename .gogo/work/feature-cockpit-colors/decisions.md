# Decisions — feature `cockpit-colors`

Forks that need a human call. gogo appends each as `D<n>` with options and a
recommendation; the orchestrator records the answer as a `RESOLVED` block at the gate.

## D1 — FR1 fix seam
- **Phase:** plan
- **Question:** How to make the home-dir board fix (`root==data-home → global cockpit`) testable?
- **Options:**
  - A. Inject a `dataHome string` param into `chooseBoard` (pure-testable at the seam; `runBoard` passes `projects.Home()`).
  - B. Guard only in `runBoard` (recompute `rootFound` before the call) — one line smaller, not seam-testable.
- **gogo recommends:** A — the FR explicitly wants a pure `chooseBoard` test; matches the existing injected `listProjects`/`initialized` fakes.
- **Status:** RESOLVED → A (accepted by user 2026-07-19)

## D2 — Color model / palette representation
- **Phase:** plan
- **Question:** How is a swatch color persisted and rendered?
- **Options:**
  - A. Persist the swatch **Dark hex**; render **adaptive** on a swatch match, **direct** for an arbitrary hex, **fallback by index** when blank (AdaptiveColor consistent with styles.go; editable hex intact; never blank).
  - B. Persist a single raw hex, render `lipgloss.Color(hex)` directly (simplest; non-adaptive on light terminals).
  - C. Persist a swatch token/name (fully adaptive) — breaks the free-form editable "label color" hex field + hand-typed-hex back-compat.
- **gogo recommends:** A — honors "AdaptiveColor consistent with styles.go" AND the editable-hex field AND never-blank, for ~one helper more than B.
- **Status:** RESOLVED → A (accepted by user 2026-07-19)

## D2.1 — Exact palette values (low-stakes, tweakable)
- **Phase:** plan
- **Question:** Which 8 swatch hexes?
- **Options:**
  - A. blue `#58a6ff`/`#2f6fe0` · teal `#35c9b5`/`#0f9e8c` · cyan `#4fc3e0`/`#0e8bb0` · green `#5db97a`/`#2e8b57` · amber `#e6a14a`/`#b9721c` · coral `#f4826b`/`#cf5136` · pink `#eb7bb5`/`#c14b8a` · purple `#b392f0`/`#8250df` (design teal/pink/blue verbatim; rest reuse styles.go; avoids alert-red).
  - B. A different / brand-specific set.
- **gogo recommends:** A.
- **Status:** RESOLVED → A (accepted by user 2026-07-19)

## D3 — Changelog dot vs the live-session cue
- **Phase:** plan
- **Question:** The changelog leading `●` today = live session; the design wants it = source. How do they coexist?
- **Options:**
  - A. Leading dot = **source color**; move the live-session cue to a **trailing** session-green `●` (`● slug … ● MM-DD`). Single-repo (no source) keeps the leading session dot (byte-for-byte).
  - B. Leading dot = **source color**; the live-session state tints the `✓` glyph green (one dot only).
- **gogo recommends:** A — keeps the recognizable green session dot (relocated), gives the design's leading source dot, reads unambiguously (origin left, liveness right).
- **Status:** RESOLVED → A (accepted by user 2026-07-19)

## D4 — Project color: a persisted field vs derived
- **Phase:** plan
- **Question:** Is the project color stored or computed?
- **Options:**
  - A. Add `Project.Color` (persisted, assigned at add, user-editable in config; additive `omitempty`; empty → fallback).
  - B. Derive from the project's index/name at render (no stored field).
- **gogo recommends:** A — the design's config shows an editable "label color" per project; derived can't be edited or stay stable across a rename/reorder.
- **Status:** RESOLVED → A (accepted by user 2026-07-19)

## D5 — Combination visual (project + source)
- **Phase:** plan
- **Question:** How to surface project + source together in a multi-project context?
- **Options:**
  - A. **Two dots** `●P ●S` left of the name (project then source).
  - B. A **project-tinted left accent stripe** + the source dot.
- **gogo recommends:** A — (b) collides with the existing gate left-border stripe (`gateBorder`, red/purple for gates); two dots are collision-free, legible at narrow widths, reuse the source-tag truncation.
- **Status:** RESOLVED → A (accepted by user 2026-07-19)
