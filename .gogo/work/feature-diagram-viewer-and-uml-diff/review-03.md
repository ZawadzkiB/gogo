# Review round 03 — feature `diagram-viewer-and-uml-diff` (Stage 3 + cross-cutting, final pass)

Scope: **FR10 + FR11 delta only.** Stages 1–2 (the rich renderer + before/after
compare internals) are already reviewed and verified as REV-001..005 — not
re-reviewed here; assumed correct unless this pass broke them.

- **FR10** — `/gogo:done` builds + prints the interactive viewer link:
  `skills/gogo-done/SKILL.md`, `commands/done.md`.
- **FR11** — version + docs + enumeration sync: `.claude-plugin/plugin.json` (0.6.0),
  `skills/gogo/SKILL.md`, `templates/state.template.md`, `docs/architecture.md`,
  `docs/commands.md`, `docs/flow.md`, `README.md`.

Fresh-eyes review by `gogo-reviewer`. Reviewed against `plan.md` (FR10/FR11) and
`.gogo/knowledge/{code-review-standards,coding-rules,non-functional-requirements}.md`.

## Verdict: **APPROVE**

No open blockers or majors. FR10 is a faithful, DRY, graceful-by-default
implementation and FR11's version bump + doc/enumeration sync is essentially
complete. Two new findings — **REV-006 (minor, P3)** and **REV-007 (nit, P3)** —
are both **AGENT-FIXABLE** description/enumeration polish that do **not** block
approval. Prior findings REV-001..005 remain **verified**.

## The three focus checks

- **(a) gogo-done reuses gogo-view + graceful fallback — CONFIRMED.**
  `skills/gogo-done/SKILL.md` step 5 explicitly *reuses the `gogo-view` build*: it
  loads the `gogo-view` skill and runs its **Step 2 (ensure shared resources)** +
  **Step 3 (build the page)** against the just-archived changelog entry, and
  **skips Step 4 auto-open** (it prints the link in Return instead). The page
  **assembly is not forked** — it says "assemble the page ... exactly as gogo-view
  Step 3 does" (template tokens, report.md→HTML, one `figure.diagram` per `.mmd`,
  compare-pair markup, `GOGO_VIEW_LAYOUT` seed). Output goes to
  `.gogo/resources/view/<date>-<slug>.html` — the **same** filename gogo-view would
  produce for that entry, so the two are idempotent/interchangeable. Graceful:
  every failure mode (mermaid missing, no diagrams, build error) is explicitly told
  **not to fail `/gogo:done`** — the archive + `shipped` state are the durable
  result; Return prints the built page's absolute `file://` link **and** the static
  `diagrams.html` fallback (or the changelog folder path). `commands/done.md`
  frontmatter is valid and lists **`Skill`** in `allowed-tools` (needed to load
  gogo-view); the command stays thin. The mermaid source path
  `${CLAUDE_PLUGIN_ROOT}/assets/mermaid/mermaid.min.js` was verified to exist on
  disk and matches gogo-view; runtime copied **copy-if-missing**, viewer `*.js` +
  `viewer.css` refreshed each run (best-effort `2>/dev/null || true`, an intentional
  variant of gogo-view Step 2 so it can't hard-fail the ship). `.gogo/`-only,
  offline (no `http(s)://`).

- **(b) version 0.6.0 + before-folder enumerations synced — CONFIRMED (with two
  polish gaps).** `.claude-plugin/plugin.json` `version` = **0.6.0**; no stale
  `0.5.0` version claim remains (the one `0.5.0` mention in gogo-view is an
  intentional reference to the prior renderer's fallback). The new `charts/before/`
  **and** `report/before/` sub-folders were added to every feature-folder
  enumeration: `skills/gogo/SKILL.md`, `templates/state.template.md`,
  `docs/architecture.md` project map, and the `README.md` table. `docs/architecture.md`'s
  plugin tree now lists the `assets/viewer/` module set (geometry · viewport ·
  mermaid-parse · render · interactive · viewer.css · viewer.template.html — matches
  disk exactly). Gaps: the **changelog** one-liners still say "(report.md +
  diagrams)" (REV-007) and the **gogo-done skills-tree** comment omits the FR10
  build+link (REV-006).

- **(c) no stale "pan/zoom only" for `/gogo:view` — CONFIRMED.** Every remaining
  "pan/zoom" mention across `docs/`, `README.md`, and the skills is the **documented
  non-flowchart fallback** ("other kinds ... fall back to a pan/zoom/drag canvas").
  `/gogo:view` is now consistently described as **interactive** (rich draggable
  token-styled node cards, live-re-routing edges, zoom/fit/minimap, persisted layout,
  before/after compare) across `docs/commands.md`, `docs/flow.md`, `README.md`, and
  `skills/gogo/SKILL.md`. No "flat mermaid image" / "one image" / "pan/zoom only"
  claim survives the sweep. **Command count unchanged at 12** (`ls commands/*.md` = 12;
  `docs/architecture.md:102` still says "12 slash commands"; no new command added —
  `done`/`view` already existed).

## What else was verified clean

- **Archived-bundle self-containment.** `/gogo:done` step 3 now also copies the
  `report/before/` set into the changelog entry (`[ -d "${src}/before" ] && { mkdir
  -p "${dst}/before"; cp "${src}/before"/* ...; }`), idempotent (overwrite, no
  delete), consistent with FR8. The existing copy/idempotency/`shipped` logic is
  intact — `report.md` still required, `.mmd`/`diagrams.html`/`manifest.json` still
  best-effort, re-run overwrites the same dated dir.
- **Compare mode works from the archive.** Because `before/` is copied beside
  `report.md` in the entry, gogo-view Step 3's "before/*.mmd beside report.md"
  compare trigger fires from the changelog entry alone — matches the built page's
  self-containment claim.
- **No accidental JS edits in the FR10/FR11 delta.** The delta is markdown + JSON
  only (`git diff --stat` for the scope = 9 files, no `.js`). All five
  `assets/viewer/*.js` modules pass `node --check`. (Note: the whole feature is a
  single uncommitted working tree, so git cannot isolate "this pass" from Stage 1/2;
  the JS/CSS/template changes in the tree are the already-reviewed Stage-1 work and
  are untouched by the FR10/FR11 markdown/json diff.)
- **ASCII/glyph hygiene + `${CLAUDE_PLUGIN_ROOT}`.** The FR10/FR11 additions use
  only intentional glyphs (phase glyphs, dashes, arrows); all in-plugin asset paths
  go through `${CLAUDE_PLUGIN_ROOT}` — no hard-coded absolute paths. Writes stay
  under `.gogo/`.

## Findings

| id | sev | pri | status | fix owner | title |
|---|---|---|---|---|---|
| REV-001 | minor | P2 | verified | AGENT-FIXABLE | Emptied `<pre class="mermaid">` left in DOM — not byte-for-byte with 0.5.0 |
| REV-002 | minor | P1 | verified | (decision resolved D7=A) | Layout persistence was in-memory only; `onPersist` unwired |
| REV-003 | minor | P2 | verified | AGENT-FIXABLE | Rich drag listeners omitted `pointercancel` |
| REV-004 | nit | P3 | verified | AGENT-FIXABLE | Edge-label index desynced on a dropped edge |
| REV-005 | minor | P2 | verified | AGENT-FIXABLE | `report/before/manifest.json` copied verbatim kept `charts/before/*.mmd` paths |
| REV-006 | minor | P3 | new | AGENT-FIXABLE | architecture.md gogo-done tree-comment omits FR10 build/print link |
| REV-007 | nit | P3 | new | AGENT-FIXABLE | changelog one-liners omit the newly-copied `before/` set |

REV-001..005 are Stage-1/2 findings, all **verified** (see `review-01.md` /
`review-02.md` and their `fix_summary` in `issues.json`); not re-litigated here.

### REV-006 — architecture.md gogo-done tree-comment omits FR10 (minor, P3, AGENT-FIXABLE)
FR10 gives `/gogo:done` a materially new behaviour (build the interactive viewer
page + print its `file://` link). Every FR11 surface reflects it — `commands/done.md`,
`docs/commands.md` (with a new "Prints:" bullet), `docs/flow.md`, `README.md`, and
`skills/gogo/SKILL.md`'s Ship section — **except** the skills-tree comment in
`docs/architecture.md` (~line 114), which still reads
`gogo-done/  #  ship: copy report bundle → .gogo/changelog/`. The adjacent
`gogo-view` line in the **same tree** *was* refreshed this pass, so the map now
describes gogo-done's pre-FR10 behaviour and is inconsistent with its own sibling.
Low impact (a terse tree comment) but a real cross-file-consistency gap
(code-review-standards.md #1). **Fix:** extend the comment, e.g.
`ship: copy report bundle → .gogo/changelog/ + build/print viewer link`.

### REV-007 — changelog one-liners omit the before/ set (nit, P3, AGENT-FIXABLE)
`/gogo:done` now archives the `report/before/` set into the changelog entry, and the
`report/` enumeration rows were updated everywhere to add `report/before/`. But the
two **changelog** description one-liners were not: `docs/architecture.md:147`
(`changelog/  # ... (report.md + diagrams)`) and `README.md:294`
(`the feature's report/ bundle (report.md + diagrams) is copied to ...`). Within
`README.md` this is internally inconsistent — the `report/` row one line up (290)
now lists `report/before/`, the changelog line just below still uses the pre-before
phrasing. Low impact ("diagrams" loosely subsumes the before `.mmd` set) but a real
enumeration-sync miss (coding-rules.md "keep enumerations in sync"). **Fix:** append
"+ the before/ set" to both one-liners, mirroring the `report/` rows.

## Notes / non-issues (verified, not raised)

- **DRY of the resource-copy bash.** `gogo-done` step 5 inlines a short vendored-
  runtime-ensure snippet that mirrors gogo-view Step 2, but this is framed as
  "ensure the runtime is present first" and deliberately made best-effort
  (`2>/dev/null || true`) so it can never hard-fail the ship — the **page assembly**
  (the substantive logic) is *referenced*, not forked. Acceptable; not raised.
- **Return snippet uses `${date}`/`${slug}`.** These are illustrative snippets the
  agent substitutes at run time (same convention as the rest of the skill's bash
  blocks); not a defect.
