# Decisions — feature `changelog-merged-entries`

Open/closed forks that needed the user. Format: D<n> question · options · gogo's
recommendation · RESOLVED answer.

## D1 — Member detail inside the merged entry

**Question:** what of each member's own report survives in the merged entry?
- **A (recommended):** synthesized top-level `report.md` **plus** each member's full
  report preserved as `report-<slug>.md` — the entry stays a self-contained audit
  trail even if `.gogo/work/` is ever cleaned up.
- **B:** the synthesized `report.md` only — leaner entry, but detail lives only in
  the work folders.

**Recommendation:** A.

**RESOLVED (2026-07-02):** **custom — synthesis only, and for single entries too.**
The user rejected both options as too heavy: "synthesis should be enough also for
single gogo:done work item, changelog should be just high level info of what was
changed/done/implemented." No full-report copies in any entry; the audit trail
stays in `.gogo/work/` (linked). This widens FR2 to cover the single-slug "Ship
one feature" flow as well.

## D2 — Release naming

**Question:** how is the merged entry's `<release-name>` chosen?
- **A (recommended):** gogo derives a suggestion (common theme of the member slugs,
  e.g. `appointments`) and asks — one short prompt, user can override.
- **B:** always auto-derive silently.

**Recommendation:** A — the name is the release's identity; a bad auto-derivation
is annoying to live with in an append-only archive.

**RESOLVED (2026-07-02):** **A** (suggest + ask).

## D3 — Where the merge question lives

**Question:** how does the user express "merge these"?
- **A (recommended):** post-selection gate — after the board/multi-select returns
  ≥2 slugs, one `AskUserQuestion` (separate vs merged). `board.py` untouched; works
  identically for TUI, fallback, and the `slug1+slug2` arg (which pre-answers it).
- **B:** a merge toggle inside the TUI — touches `board.py` + its exit contract.

**Recommendation:** A.

**RESOLVED (2026-07-02):** **A** (post-selection gate; `board.py` untouched) —
accepted with the plan ("Accept (all recs)").
