# Review — round 3 snapshot (Stage 3: FR8–FR10)

- **feature:** `workspace-changelog-viewer`
- **scope:** Stage 3 only — FR8 (`/gogo:view` command + `gogo-view` skill), FR9
  (readable summary + custom pan/zoom/drag interactive diagrams, D3 whole-diagram
  only), FR10 (offline / portable over `file://`, graceful fallback)
- **reviewed (all new, untracked):** `assets/viewer/viewer.template.html`,
  `assets/viewer/interactive.js`, `assets/viewer/viewer.css`,
  `skills/gogo-view/SKILL.md`, `commands/view.md`; sanity-checked the generated
  artifact `.gogo/resources/view/docs-and-verified-discovery.html` (not committed source)
- **date:** 2026-06-30
- **deferred (NOT flagged, per orchestrator):** `plugin.json` version bump and the
  command-count enumerations / README / docs-site updates for `/gogo:done` +
  `/gogo:view` — these are the FR11 cross-cutting sweep (next step after Stage 3 test).

## Findings this round

| id | severity | priority | status | file:line | finding | proposed fix | tag |
|---|---|---|---|---|---|---|---|
| REV-005 | minor | P2 | new | assets/viewer/interactive.js:125-127 | One batched `mermaid.run` (no `suppressErrors`) + a single global `.catch → fail()` that replaces every `pre.mermaid`: one invalid `.mmd` in a bundle blanks ALL diagrams on the page (good ones included), losing per-diagram graceful degradation (FR10). | Pass `{ querySelector: "pre.mermaid", suppressErrors: true }` so good diagrams render and mermaid inlines an error graphic for the bad one; reserve the global `fail()` for the whole-runtime-missing case; optionally try/catch per `setupViewport`. | AGENT-FIXABLE |
| REV-006 | minor | P2 | new | skills/gogo-view/SKILL.md:47 · Step 3 (98) | Legacy root-layout `report.md`s are added to the picker, but Step 3 gathers `*.mmd` only from the bundle dir (= the feature root for that layout), while those features keep diagrams under `charts/`. Selecting one yields a silent summary-only page — the viewer's headline feature missing, no warning. Does not crash. | When the chosen `report.md` is at a feature root, also scan the sibling `charts/`; OR drop root-layout from the picker; OR warn in the Return when a bundle has no `.mmd`. | AGENT-FIXABLE |
| REV-007 | nit | P3 | new | assets/viewer/interactive.js:80 · 114 | `.controls` is a child of `viewport`, and the drag handler is on `viewport`, so clicking a control button bubbles a pointerdown that starts a zero-distance drag, toggles the grab cursor, and calls `setPointerCapture` on the ancestor mid-press. Clicks still work; cosmetic flicker + fragile capture. | At the top of the pointerdown handler add `if (e.target.closest('.controls')) return;` (or `stopPropagation` on the controls' pointerdown). | AGENT-FIXABLE |
| REV-008 | nit | P3 | new | skills/gogo-view/SKILL.md:83 | The `GOGO_VIEW_TITLE` guidance yields a doubled "report —" and unstripped markdown backticks in `<title>` (sample: `gogo report — Report — feature \`…\``). Defect traces to the skill instruction; cosmetic. | Clarify: use the first `# ` heading as plain text with inline markdown stripped; only prefix "gogo report — " when the heading doesn't already start with gogo/Report. | AGENT-FIXABLE |

## Prior findings (carried, verified)

| id | severity | status | note |
|---|---|---|---|
| REV-001 | nit | verified | architecture.md `resources/` comment alignment — fixed round 2. |
| REV-002 | minor | verified | gogo-build partial-migration log no longer misreports a conflict as no-op — fixed round 2. |
| REV-003 | major | verified | contracts docs chart-kind enum synced to add `use-case` — fixed round 4. |
| REV-004 | nit | verified | charts-manifest schema top-level description notes the ⑤ `report/manifest.json` reuse — fixed round 4. |

## What was checked and passed

- **Offline / portability (FR10) — SOLID.** Template + generated page load only
  vendored assets via relative paths: `GOGO_MERMAID_SRC=../mermaid.min.js`,
  `GOGO_VIEWER_SRC=../viewer/interactive.js`, `GOGO_VIEWER_CSS=../viewer/viewer.css`.
  For a page at `.gogo/resources/view/<name>.html` these resolve to
  `.gogo/resources/mermaid.min.js` and `.gogo/resources/viewer/…` — correct. No
  `http(s)://` resource refs anywhere (the one `https://…github.io` hit is literal
  text inside a `<code>` block, not a load). `.mmd` sources are inlined verbatim
  into `<pre class="mermaid">` — no `fetch()` (file:// forbids it).
- **Zoom/pan correctness (FR9, D3) — CORRECT.** Transform is on the `.canvas`
  wrapper, not per node (D3 honoured). Zoom-toward-cursor math
  (`tx = cx - ((cx - tx)/s)*ns`, transform-origin 0,0, cursor taken as
  `clientX - rect.left`) holds the anchor under the cursor exactly. Wheel is scoped
  to the viewport with `preventDefault` + `{passive:false}` — diagram zoom, not a
  global page-scroll hijack. Division-by-zero guarded (natural size falls back to
  600×400; `fit` uses `… || 1`; scale clamped to [0.1, 8] so `s` is never 0).
  Pointer capture is auto-released on pointerup/pointercancel (both wired to
  `endDrag`) — no leak. Missing-runtime path is graceful (`!window.mermaid → fail()`,
  summary still renders). `node --check` passes (vanilla, zero deps).
- **gogo-view skill (FR8).** Enumerates BOTH `.gogo/changelog/*` and
  `.gogo/work/feature-*/report/`, and additionally legacy root `report.md`s (keeps
  only those that contain a `report.md`); pick via `$ARGUMENTS` else `AskUserQuestion`;
  ensures/copies vendored resources idempotently via `${CLAUDE_PLUGIN_ROOT}` (mermaid
  copied once, JS/CSS each run); pre-renders md→HTML itself (D7, no JS md lib) with
  escaping + comment-stripping; inlines diagrams; writes only under `.gogo/`; opens
  best-effort with an absolute `file://` path fallback (FR10). 143 lines (≤200).
- **Token sync.** All six template tokens (`GOGO_VIEW_TITLE`, `GOGO_VIEW_SUMMARY`,
  `GOGO_VIEW_DIAGRAMS`, `GOGO_MERMAID_SRC`, `GOGO_VIEWER_SRC`, `GOGO_VIEWER_CSS`)
  match exactly between `viewer.template.html` and the skill's replace table.
- **Command thinness + frontmatter.** `commands/view.md` is thin (26 lines, logic
  in the skill), valid frontmatter, `allowed-tools` includes Bash (open) + Skill +
  AskUserQuestion; `model: opus` matches the other commands. Plugin command count
  is 12 (matches the plan's target).
- **Hard invariants.** `${CLAUDE_PLUGIN_ROOT}` used for every asset copy (no
  hard-coded plugin paths); writes confined to `.gogo/`; no new runtime dependency;
  vendored mermaid present at `assets/mermaid/mermaid.min.js` (copy source valid);
  ASCII apart from intentional glyphs (em-dash, `−` zoom-out label, phase glyphs) in
  JS/CSS/markdown — allowed. `initialize` opts (`theme:"default"`,
  `securityLevel:"loose"`) match the existing offline diagrams viewer.

## Verdict

**APPROVE** — no blockers, no majors. Stage-3 offline-safety and the pan/zoom/drag
renderer are correct and portable. Four non-blocking findings remain (REV-005,
REV-006 minor; REV-007, REV-008 nit), all AGENT-FIXABLE and worth folding in before
the FR11 sweep; none gate advancing to ④ test.
