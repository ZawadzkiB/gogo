# Review round 02 — feature `diagram-viewer-and-uml-diff` (Stage 2 of 3)

Scope: **Stage 2 only** — FR7 (plan ① draws the `charts/before/` as-is baseline),
FR8 (report ⑤ draws the after set, copies the before set into `report/before/`, adds
a before↔after comparison to `report.md`), FR9 (`/gogo:view` before | after compare
mode). **Stage 3** (done-link, FR10) and the **cross-cutting sweep** (FR11: version
bump 0.5.0→0.6.0, README, and the top-level enumeration sync) are later and were
**not** flagged as missing.

Fresh-eyes review by `gogo-reviewer`. Reviewed against `plan.md`, `decisions.md`
(**D4=A** side-by-side + prose, no node-diff · **D5** separate `before/manifest.json`,
no schema change · D6/D7 persistence), and
`.gogo/knowledge/{code-review-standards,coding-rules,non-functional-requirements}.md`.

Files reviewed (Stage 2 changes):
- `skills/gogo-mermaid/SKILL.md` — the `charts/before/` (as-is) set + path math.
- `skills/gogo-plan/SKILL.md` — plan ① also draws the before baseline.
- `skills/gogo-knowledge/SKILL.md` — draw after + copy before + write the comparison.
- `skills/gogo-view/SKILL.md` — before | after compare mode (pure markup).
- `templates/report.template.md` — the "Before / after comparison" section.
- `assets/viewer/viewer.css` — the `.compare` two-column grid.
- Evidence artifact: `.gogo/resources/view/sample-compare.html`.

## Verdict: **APPROVE**

No open blockers or majors. Stage 2 is a faithful, decision-aligned implementation:
side-by-side + prose only (D4=A), separate before manifest with no schema change
(D5), and compare mode delivered as pure markup with **zero JS change**. One new
finding (REV-005), **minor**, **AGENT-FIXABLE**, does not block approval. Prior
Stage-1 findings REV-001..004 remain **verified**.

## What was verified clean

- **(a) Path-math for `charts/before/` and `report/before/`.** `gogo-mermaid`
  states the offline viewer for `charts/before/diagrams.html` must use
  **`../../../../resources/mermaid.min.js` — four `../`, not three** (SKILL.md:170-173),
  which is correct: `charts/before/` is one level deeper than `charts/` (`before/` →
  `charts/` → `feature-<slug>/` → `work/` → `.gogo/` = 4 up). `charts/` and `report/`
  stay at 3× (`gogo-knowledge` SKILL.md:90 keeps `report/` at `../../../resources/...`).
  `report/before/` builds **no** standalone `diagrams.html` — `gogo-knowledge` copies
  only `*.mmd` + `manifest.json` there (SKILL.md:95-96) — so there is **no** path to
  get wrong at that location (the before set is viewed via `report.md`'s comparison
  and `/gogo:view` compare mode, not a per-folder viewer). No wrong `../` count found.
- **(b) No schema change (D5).** `templates/contracts/charts-manifest.schema.json` is
  **byte-for-byte unchanged** (empty `git diff`). No `role`/`before`/phase field added;
  the before set reuses the schema via its own `charts/before/manifest.json` and the
  copied `report/before/manifest.json`. The skills say so explicitly
  (`gogo-mermaid`:166-169, `gogo-knowledge`:99-100,155-158).
- **(c) No node-diff (D4=A).** The comparison is side-by-side + prose only. Every
  mention of "node-diff" across `gogo-knowledge`, `gogo-view`, and `report.template.md`
  is a **negation** ("do NOT compute a structural node-diff"). No structural-diff /
  added-removed-changed compute logic crept into any skill or the JS.
- **FR8 self-containment + no-before case.** `gogo-knowledge` copies
  `charts/before/*.mmd` + `manifest.json` into `report/before/` (SKILL.md:95-103);
  the report template + skill both handle the no-before case gracefully (one-line
  note, show only the after set); validate-out conditionally validates
  `report/before/manifest.json` against the same schema (SKILL.md:155-158). (The one
  gap in the copy is the manifest `file` paths — see REV-005.)
- **FR9 compare mode is pure markup / no JS change.** `git` shows no Stage-2 edits to
  `assets/viewer/*.js`; `grep` for `compare`/`before` in all five JS modules returns
  only comments (no logic). `interactive.js` iterates **every** `figure.diagram`
  (interactive.js:137) and keys layout on `data-diagram` (interactive.js:127-128), so
  wrapping figures in `.compare` needs no renderer change. `node --check` passes on
  all five modules (geometry, viewport, mermaid-parse, render, interactive).
- **Distinct persistence keys.** Compare mode gives the before figure
  `data-diagram="before-<basename>"` and the after figure `data-diagram="<basename>"`
  (`gogo-view`:164-166; confirmed in `sample-compare.html`: `before-flow` vs `flow`),
  so localStorage/sidecar layouts (`gogo-view:layout:<name>`) never collide.
- **`.compare` CSS.** `viewer.css` `.compare` is a `1fr 1fr` grid that stacks to one
  column at `@media (max-width: 720px)`; `.compare-solo` spans `1 / -1` for an
  unmatched (added/removed) kind. Purely cosmetic captions via role tone vars.
  Consistent with `sample-compare.html`.
- **Enumeration/consistency of the before/after model.** The before/after model is
  described consistently across `gogo-mermaid` (produce), `gogo-plan` (produce at ①),
  `gogo-knowledge` (copy + compare at ⑤), `gogo-view` (render compare), and
  `report.template.md`. Filenames are `<kind>.mmd` on both sides so `/gogo:view`
  pairs them by kind. `${CLAUDE_PLUGIN_ROOT}` used for all plugin asset paths.
- **ASCII/glyph hygiene + budget.** Only intentional glyphs (phase glyphs, em/en
  dashes, arrows, `↔`) in the Stage-2 additions (the `🚫`/`✅` in `gogo-mermaid` and
  the `2–4` in the template are pre-existing, outside the diff). `SKILL.md` line
  budget is defined for `.gogo/knowledge/*.md` only, not `skills/*/SKILL.md`, so
  `gogo-view` at 237 lines is not a budget violation (it grew with the rich renderer +
  compare + persistence docs).

## Findings

| id | sev | pri | status | fix owner | title |
|---|---|---|---|---|---|
| REV-001 | minor | P2 | verified | AGENT-FIXABLE | Emptied `<pre class="mermaid">` left in DOM — not byte-for-byte with 0.5.0 |
| REV-002 | minor | P1 | verified | (decision resolved D7=A) | Layout persistence was in-memory only; `onPersist` unwired |
| REV-003 | minor | P2 | verified | AGENT-FIXABLE | Rich drag listeners omitted `pointercancel` |
| REV-004 | nit | P3 | verified | AGENT-FIXABLE | Edge-label index desynced on a dropped edge |
| REV-005 | minor | P2 | new | AGENT-FIXABLE | `report/before/manifest.json` copied verbatim keeps `charts/before/*.mmd` paths |

REV-001..004 are Stage-1 findings, all **verified** fixed (see `review-01.md` and
their `fix_summary` in `issues.json`); they are not re-litigated here.

### REV-005 — before manifest copied verbatim keeps `charts/before/` paths (minor, P2, AGENT-FIXABLE)
`gogo-knowledge` SKILL.md:95-103 copies the before set into `report/before/` so the
bundle is "self-contained ... with no dependency on the `charts/` folder", but then
instructs: *"Do not rewrite the copied `before/manifest.json` (... D5, no schema
change)."* The manifest `file` field is a **folder-prefixed** path by the established
convention (schema description + every on-disk manifest: `charts/<name>.mmd`,
`report/flow.mmd`). So an ordinary before manifest at `charts/before/` carries
`"file":"charts/before/<kind>.mmd"`, and copying it **verbatim** leaves
`report/before/manifest.json` still pointing at `charts/before/<kind>.mmd` —
which (a) points **outside** the report bundle (contradicting the same sentence's
"no dependency on `charts/`") and (b) becomes a **dangling reference** once
`/gogo:done` archives only `report/` to `.gogo/changelog/<date>-<slug>/`. The gate
does **not** catch it: validate-out checks only the schema, whose `file` pattern is
just `\.mmd$`, so a wrong-but-`.mmd` path passes. Consumer impact is limited —
`/gogo:view` compare mode globs `before/*.mmd` by filename, not via the manifest
`file` paths — so rendering still works; the defect is that the copied/archived
manifest is itself an inaccurate, non-self-contained contract artifact. The
instruction conflates D5 ("no schema change" = don't alter the shape / don't add a
role field) with "don't touch the content" — updating location-relative `file` paths
is **not** a schema change.
**Fix (AGENT-FIXABLE):** change "Do not rewrite the copied `before/manifest.json`" to
"copy it and rewrite its `file` entries to `report/before/<kind>.mmd` (or bare
basenames), keeping slug/kind/title and the schema shape unchanged" — D5 forbids a
schema change, not a path-string update. Optionally add a semantic validate-out check
that each `report/before/manifest.json` `file` resolves under `report/before/`.

## Deferred (verified, tracked — NOT a Stage-2 defect)

- **Top-level enumeration sync (FR11 cross-cutting).** The feature-folder file set is
  enumerated in `skills/gogo/SKILL.md:74-75` and `templates/state.template.md:12-13`;
  neither yet lists the new `charts/before/` and `report/before/` sub-folders (README
  has no before/after mention either). This is exactly the "sync enumerations" work
  the plan schedules under the **cross-cutting** stage (plan build-order item 11,
  bundled with the 0.5.0→0.6.0 version bump), so it is expected to be still open at
  Stage 2 and is **not** raised as an issue (raising it could wrongly trigger a
  Stage-2 re-implement loop). Logged here so the FR11 sweep does not forget it.

## Judgment on the three focus areas
- **(a) `../` path depths:** CORRECT. `charts/before/` = 4× (`gogo-mermaid` states it
  explicitly and contrasts "four, not three"); `charts/`/`report/` = 3×;
  `report/before/` builds no viewer so there is no path to mis-count there.
- **(b) No schema change:** CONFIRMED. `charts-manifest.schema.json` is unchanged; the
  before set reuses it via separate manifests; no `role`/phase field added.
- **(c) No node-diff:** CONFIRMED. Side-by-side + prose only; every "node-diff"
  reference is an explicit prohibition; no diff-compute logic anywhere (D4=A honored).
