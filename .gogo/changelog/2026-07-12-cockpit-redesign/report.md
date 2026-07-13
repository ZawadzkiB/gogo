# cockpit-redesign — terminal cockpit restyled to the Claude-Design 1b + 1c mockup

- **shipped:** 2026-07-12 · CLI **0.17.0 → 0.18.0**
- **members:** `cockpit-redesign` (single feature)
- **full audit trail:** [.gogo/work/feature-cockpit-redesign/](../../work/feature-cockpit-redesign/)

## What shipped

The gogo terminal cockpit (`cli/internal/tui/`) now renders the Claude-Design **1b + 1c**
mockup — a *visibly, obviously* different board, not a token diff. New per-card and
per-board elements landed together: a **header attention summary** (`⏸ K need you` ·
`● S session`), status **pills** (tinted chips), phase **dots ①②③④⑤**, left **gate
stripes** (heavy `┃`, red plan/decision · purple uat), a **collapsed changelog** list, a
**contextual footer** (focused-card key chips + `?` for full help), and a top **needs-you
inbox strip** with `1..9` number-key gate answering. All of it renders over the **same
`contract.Repo`** — a presentation-only refactor with **no contract change and no new
pipeline state**; the CLI stays a deterministic, LLM-free reader that never mutates
pipeline state. This landed the redesign the prior attempt missed (that one only re-ported
an already-correct palette and produced no visible change).

## Key outcomes

- **One shared progress model, thin renderers.** A single `phaseProgress(f) [5]phaseState`
  vector (done/current/pending) feeds *both* the FR-4 dots and the FR-9 segmented bar — so
  "dots and/or bars" is a rendering choice, not two code paths.
- **`badge()` stays canonical.** A new `pillLabel`/`pillStyleFor` transform drives the pill
  chip while mirroring `badge()`'s precedence, so a card's pill color can never disagree
  with its text; the existing badge tests are untouched.
- **Gate stripe is a real border, not a width hack** — a custom `gateBorder` with a heavy
  `┃` left edge, focus-independent, width-preserving, and substring-assertable.
- **Graceful degradation.** The needs-you strip gives up its height in `colAvail()` so
  strip + board both fit; on a short terminal it collapses to a one-line summary (never
  overflows). The strip↔windowing coupling was confirmed cycle-free.
- Every new element stays substring-assertable (no TTY under `go test` → lipgloss emits
  plain text), so the redesign is fully unit-pinned.

## Decisions (one-liners)

- **D1 — scope:** ship **1b + 1c together** as one 0.18.0 release (not a 1b-first slice).
- **D2 — dots vs bars:** keep **both**, driven by one shared `phaseProgress` vector — dots
  on every dense board card, the segmented bar on the roomy needs-you strip rows.
- **D3 — strip duplicates gates:** the strip is a **shortcut** (each gate also stays in its
  column) that **degrades gracefully** on short terminals.
- **FR-10 number key = "read":** pressing `1..9` focuses that gate's card *and* opens its
  primary view (plan.md / report.md via `quickView`).
- Review fixes folded in-context: `[g]`→`[m] resume` (real go/resume key), removed dead
  `uatStyle`/`colStyleSet.badge`, and numbered only the first 9 strip gates (single-digit key).

## Review / test verdict

**APPROVE** (one round, fresh-eyes `gogo-reviewer` — 3 non-blocking findings, all fixed
in-context and re-verified) · **GREEN** (one round, fresh-eyes `gogo-tester` — automated
suite `gofmt`/`vet`/`go test -race` clean **and** a live-TUI fidelity check via a render
harness plus a live tmux drive of the actual binary, every color decoded and matched to its
design token; FR-1..FR-10 and D1/D2/D3 all present and matching the mockup).

## Files changed (as-built)

`cli/internal/tui/styles.go`, `model.go`, `view.go`, `update.go`, `window.go` (the
redesigned render path + the shared `phaseProgress` model), `cli/main.go` +
`.claude-plugin/plugin.json` (version `0.18.0`), and tests
(`cli/internal/tui/redesign_test.go` added; `tui`/`waiting`/`window` tests updated for the
restyle). Knowledge: `.gogo/knowledge/project-knowledge.md` (proxy) updated to describe the
redesigned board; the proxied upstream `README.md` was left untouched.

## Follow-ups & known limitations

- **>9 simultaneous gates** are reachable only in their columns, not by number key (the
  strip numbers the first 9; keys are single-digit). Negligible in practice; documented.
- **Variant 1d (phone companion)** stays a separate future web app consuming `.gogo` data —
  out of scope for this terminal cockpit.
- The change shipped from the working tree (uncommitted at report time) — commit when ready.

---

*This is a synthesized changelog entry. The full audit trail — plan, per-round review/test
snapshots (`review-01.md` / `test-01.md`), decisions detail, and the per-file changes table
— lives in [.gogo/work/feature-cockpit-redesign/](../../work/feature-cockpit-redesign/).*
