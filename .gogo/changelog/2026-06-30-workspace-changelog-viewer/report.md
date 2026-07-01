# Report — feature `workspace-changelog-viewer`

- **feature:** Workspace rename, richer report, changelog + interactive viewer
- **status:** done
- **completed:** 2026-06-30
- **branch / commits:** main · uncommitted (0.5.0 release pending)

## Summary
A multi-part upgrade to gogo's workspace, reporting, and diagram experience,
delivered in three staged loops plus a final cross-cutting pass. The feature
workspace was renamed `.gogo/plans/` → **`.gogo/work/`** and the vendored mermaid
runtime moved up to a shared **`.gogo/resources/`**; report ⑤ now writes a richer
**`report/` bundle** (implementation + decisions-and-reasons + a diff-chosen UML
set incl. the new **`use-case`** kind); a new **`/gogo:done`** archives the bundle
into an append-only **`.gogo/changelog/`**; a new **`/gogo:view`** opens a
self-contained offline webpage that renders the summary readably and the diagrams
with **custom pan/zoom/drag** interactivity; and `/gogo:report` gained a **lenient
mode** so it can document past/broken runs. Version **0.4.0 → 0.5.0**.

## Run status / gaps
Clean (strict) report — all phases completed green. Review **APPROVE**
(REV-001..009 all `verified`); test **GREEN** (TEST-001..004 all `verified`); no
open issues. (This section is the lenient-mode honesty hook introduced by FR12;
for this run there are no gaps.)

## Planned vs shipped
Shipped **as planned** (FR1–FR11) plus the mid-build addition **FR12**
(`/gogo:report` on past/broken runs + `/gogo:done` guidance — logged in
`adjustments.md`). Decisions D1–D7 honored as accepted. One layout reconciliation
during Stage 1 (see Decisions). Staged exactly as D1 prescribed.

## Implementation
By stage:
- **Stage 1 — workspace + resources refactor.** Renamed `.gogo/plans` → `.gogo/work`
  across ~20 plugin files; moved the vendored runtime to `.gogo/resources/`
  (viewer `<script src>` → `../../../resources/mermaid.min.js`); added an
  idempotent **Step 0 migration** to `gogo-build` (move-never-delete; warns on a
  partial-migration conflict); dogfood-migrated this repo.
- **Stage 2 — report bundle + UML + changelog + done.** report ⑤ writes the
  `report/` bundle; `report.md` gained **Implementation** + **Decisions &
  rationale** sections; added the **`use-case`** chart kind + a "choose UML by what
  changed" rule to `gogo-mermaid`; new `.gogo/changelog/<YYYY-MM-DD>-<slug>/` +
  **`/gogo:done`** (`commands/done.md` + `skills/gogo-done/SKILL.md`, copy-not-move,
  dated, idempotent, sets `shipped`).
- **Stage 3 — interactive viewer.** Vendored `assets/viewer/` (template +
  `interactive.js` pan/zoom/drag canvas + `viewer.css`); **`/gogo:view`**
  (`commands/view.md` + `skills/gogo-view/SKILL.md`) enumerates reports, pre-renders
  the summary to HTML, inlines the `.mmd`, and opens a self-contained offline page.
- **Final pass — FR11 + FR12.** Docs/Pages/README sweep (command set → **12**),
  `plugin.json` → **0.5.0**; `/gogo:report` strict-vs-lenient modes (lenient
  documents past/broken runs with a Run-status/gaps section and never falsely marks
  a run `done`); `/gogo:done` missing-report message names `/gogo:report <feature>`.

### Changes (as-built, by area)
| Area | Files |
|---|---|
| Rename + resources | ~20 files (`commands/*`, `skills/*`, `agents/gogo.md`, `templates/*`, `docs/*`, `README.md`); `gogo-build` Step 0; `.gogo/work` + `.gogo/resources` |
| Report bundle + UML | `skills/gogo-knowledge/SKILL.md`, `templates/report.template.md`, `templates/contracts/charts-manifest.schema.json` (+`use-case`), `skills/gogo-mermaid/SKILL.md`, `templates/contracts/README.md`, `docs/contracts.md` |
| Changelog + done | `commands/done.md`, `skills/gogo-done/SKILL.md` |
| Viewer | `assets/viewer/{viewer.template.html,interactive.js,viewer.css}`, `commands/view.md`, `skills/gogo-view/SKILL.md` |
| Docs/version | `README.md`, `docs/{commands,architecture,flow,agents,contracts}.md`, `skills/gogo/SKILL.md`, `templates/knowledge/index.md`, `.claude-plugin/plugin.json` (0.5.0) |

## Decisions & rationale
| Decision | Choice | Reason |
|---|---|---|
| D1 Staging | 3 stages under one feature | Rename is mechanical/low-risk → first; viewer is the biggest unknown → last |
| D2 Migration | `/gogo:build` auto-migrates | Idempotent, in-bounds (`.gogo/`-only), spares every project a manual `mv` |
| D3 Viewer interactivity | Pan/zoom/drag canvas (per-node = stretch) | Realistic offline/zero-dep vanilla JS over mermaid SVG; delivers "move them around" |
| D4 Use-case | Add `use-case` kind (flowchart actor↔use-case) | Mermaid has no native use-case type; the goal wants it "when relevant" |
| D5 Changelog naming | `<YYYY-MM-DD>-<slug>/` | A changelog is chronological; dated dirs sort + dedupe re-ships |
| D6 Report layout | Consolidate under `report/` | Groups the bundle (md + UML + result) that `done` copies and `view` reads |
| D7 Viewer summary | Pre-render md→HTML | Keeps the offline / zero-dep / no-build bar |
| Layout reconciliation (Stage 1) | `_config.yml`-style: deploy resources at `.gogo/` level; `_config` n/a — runtime at `.gogo/resources/`, charts reach it via `../../../resources/` | resources shared by viewer + future skills; sibling of `work/`, not nested |
| FR12 (added mid-build) | `/gogo:report` lenient mode + `/gogo:done` guidance | User needs to report past/broken runs; `done` should point at `/gogo:report` when no report exists |

See [decisions.md](../decisions.md) and [adjustments.md](../adjustments.md).

## Review outcome
**APPROVE.** Across the stages: REV-001/002 (Stage 1 — tree alignment + migration
WARN), REV-003/004 (Stage 2 — `use-case` enum doc sync + schema description),
REV-005..008 (Stage 3 — suppressErrors, legacy-layout diagram gathering, controls
drag guard, clean `<title>`), REV-009 (final — flow.md heading). All **verified**.
The reviewer confirmed the strict/lenient gating is safe (a broken run can't be
silently archived as green) and the 12-command enumeration is fully in sync.
See [review/issues.json](../review/issues.json) · snapshots `review-01..04.md`.

## Test outcome
**GREEN.** Dogfooded each stage in scratch fixtures: the `gogo-build` migration
(incl. the partial-migration WARN), the `report/` bundle + `/gogo:done`
(copy-not-move, dated, idempotent, validate-in refusal), the `/gogo:view` viewer
(legacy + new layouts, a malformed diagram not blanking the others, offline-safe),
and the FR12 lenient report (best-effort with a Run-status/gaps section; never
stamps a broken run `done`). TEST-001..004 all **verified**. Browser automation
skipped (no UI); the GitHub-side Pages build verified structurally.
See [test/issues.json](../test/issues.json) · snapshots `test-01..04.md`.

## Diagrams
As-built — open [report/diagrams.html](./diagrams.html) (or the interactive
[`/gogo:view`](../../../..) page):
- **Flow** (`flow.mmd`) — work → report/ → `/gogo:done` → changelog; `/gogo:view`
  renders from `.gogo/resources`.
- **Sequence** (`sequence.mmd`) — the `/gogo:view` runtime: enumerate → pick →
  ensure resources → build self-contained page → open.
- **Use-case** (`use-case.mmd`) — the new user capabilities (report incl.
  broken runs, ship→changelog, view, auto-migrate). *(dogfoods the new kind.)*

## Knowledge updates
- `.gogo/knowledge/project-knowledge.md` — gogo-overrides note for 0.5.0
  (`.gogo/work` + `.gogo/resources` + `.gogo/changelog`, the `report/` bundle,
  `/gogo:done` + `/gogo:view`, `/gogo:report` lenient mode). Earlier stages already
  corrected the workspace/resources references in `testing-tools.md` +
  `project-knowledge.md`. No upstream/proxied file edited.
- Shipped templates updated so new projects inherit the changes:
  `templates/report.template.md`, `templates/state.template.md` (status enum +
  `shipped`), `templates/contracts/*`, `templates/knowledge/*`.

## Follow-ups & known limitations
- **Logo (pending):** add the gogo logo to README + the docs site once the PNG is
  on disk (a separate small task; awaiting the file).
- **Release 0.5.0:** commit + push + tag (working tree uncommitted by design).
- **Per-node diagram repositioning** in `/gogo:view` is a deliberate stretch (D3) —
  canvas pan/zoom/drag ships now.
- **Out of scope (as planned):** a structured node/edge diagram model, a
  served/hosted viewer, versioned docs.
- **Roadmap (in memory):** pre/post per-phase agent extensions; xplan integration.
