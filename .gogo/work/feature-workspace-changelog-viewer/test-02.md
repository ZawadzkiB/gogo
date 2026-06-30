# Test — round 3 (Stage 2) — workspace-changelog-viewer

**Date:** 2026-06-30
**Stage:** 2 of 3 (FR4–FR7: report/ bundle, richer report, use-case + chosen-by-diff, .gogo/changelog/ + /gogo:done)
**Level:** structural inspection + dogfood CLI (no UI; markdown plugin — no browser automation needed)
**Prior round summary:** TEST-001 (Stage 1, partial-migration WARN) verified fixed.

---

## What was exercised

### TC-1 — FR6 schema: `use-case` kind enum
**Method:** structural (Read + python3 validation)

Read `templates/contracts/charts-manifest.schema.json`. Verified:
- `kind` enum at `definitions.diagram.properties.kind.enum` is exactly `["flow","sequence","class","activity","use-case"]` — 5 members, no more, no less.
- `additionalProperties: false` present at both the schema root (line 7) and in the `diagram` definition (line 33).

Constructed sample manifest entry with `kind: "use-case"` → passed all field checks (kind in enum, file ends `.mmd`, title non-empty).
Constructed sample with `kind: "foo"` → correctly rejected ("Invalid kind: foo").

**Result:** PASS

### TC-2 — FR6 docs: gogo-mermaid documents use-case pattern + chosen-by-diff rule
**Method:** structural (Read)

Read `skills/gogo-mermaid/SKILL.md`. Confirmed:
- Lines 103–119: "Use-case diagrams (no native mermaid type)" section documents the `flowchart LR` actor↔use-case pattern with `classDef actor` / `classDef uc`, and specifies `kind: use-case` in the manifest.
- Lines 121–137: "Choose the kinds by what changed" section gives a table mapping diff characteristics to diagram kinds, including "A new user-facing capability (actor can now do X) → use-case".
- Line 153: "The report ⑤ bundle lives in `report/`, not `charts/`."

**Result:** PASS

### TC-3 — FR6 5-kind set in docs/contracts.md + templates/contracts/README.md
**Method:** structural (grep)

Both files at line 55 now read: `kind ∈ {flow, sequence, class, activity, use-case}` — the 5-kind set is consistent across both consumer-facing references.

**Result:** PASS (REV-003 fix verified)

### TC-4 — FR4 report/ layout: gogo-knowledge outputs
**Method:** structural (Read + path-math verification)

Read `skills/gogo-knowledge/SKILL.md`. Confirmed outputs table:
- `report/report.md`, the as-built UML `.mmd` set + `report/diagrams.html`, `report/manifest.json`, `report/result.json`
- Explicitly NOT the feature root: Step 3 says "Copy … → `.gogo/work/feature-<slug>/report/report.md` (NOT the feature root)"

Path math verification (python3): `os.path.relpath('.gogo/resources/mermaid.min.js', '.gogo/work/feature-demo/report')` = `../../../resources/mermaid.min.js` — same depth as `charts/`, same path. SKILL.md line 155 confirms: "The `report/diagrams.html` viewer loads the shared runtime at `../../../resources/mermaid.min.js` (`report/` is the same depth as `charts/`)."

Built a sample report bundle in the scratchpad at `donetest/.gogo/work/feature-demo/report/` comprising: `report.md`, `flow.mmd`, `use-case.mmd`, `diagrams.html` (with `<script src="../../../resources/mermaid.min.js">`), `manifest.json`. All five files present, HTML path correct, manifest validates.

**Result:** PASS

### TC-5 — FR5 report template: Implementation + Decisions & rationale
**Method:** structural (python3)

Read `templates/report.template.md`. Confirmed:
- `## Implementation` section present.
- `## Decisions & rationale` section present with table header `| Decision | Choice | Reason |` — three-column format capturing both the choice and the reason.
- Comment on line 4: "rendered to `.gogo/work/feature-<slug>/report/report.md` (NOT the feature root)" — self-documenting placement.

**Result:** PASS

### TC-6 — FR7 gogo-done dogfood
**Method:** executed (bash, following gogo-done SKILL.md literally)

Fixture: `scratchpad/donetest/.gogo/work/feature-demo/` with `report/report.md` (containing `- **completed:** 2026-06-30`), `report/flow.mmd`, `report/use-case.mmd`, `report/diagrams.html`, `report/manifest.json`, and `state.md` (status: done).

**TC-6a — validate-in pass:** `report/report.md` present → PASS (proceeds normally).

**TC-6b — date derivation:** Extracted ISO date `2026-06-30` from the `completed:` field using `grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}'`. Changelog dir correctly named `2026-06-30-demo/`. (See TEST-002 for the nit on the naive-extraction edge case.)

**TC-6c — copy-not-move:** Bundle copied to `.gogo/changelog/2026-06-30-demo/` (5 files: `report.md`, `flow.mmd`, `use-case.mmd`, `diagrams.html`, `manifest.json`). Source `report/` bundle confirmed intact (all 5 files still present). PASS.

**TC-6d — state.md terminal:** `sed` updated `status: done` → `status: shipped`. Confirmed with `grep`. PASS.

**TC-6e — idempotency:** Re-ran the copy commands for the same `2026-06-30-demo` dir. File count before: 5; file count after: 5. No duplicates created. Only one dated dir per slug exists. PASS.

**TC-6f — validate-in failure:** Created `scratchpad/donetest-fail/.gogo/work/feature-nodocs/report/` (empty, no `report.md`). Validate-in check confirmed: exits with "not report-complete: …/report.md missing". No changelog entry written. PASS.

**TC-6g — only .gogo/ written:** `find donetest -type f | grep -v .gogo/` → 0 results. All writes confined to `.gogo/`. PASS.

**Result:** PASS (with nit TEST-002 noted)

### TC-7 — Enumeration sync
**Method:** structural (grep)

- `grep -rn "report\.md"` across skills/commands/docs: zero results claiming `report.md` at the feature root. All references say `report/report.md` or `report/` bundle. PASS.
- `shipped` in `state.template.md` status comment enum (line 19): confirmed. PASS.
- `commands/done.md`: 25 lines, thin, says "via the `gogo-done` skill", delegates `validate-in` → `work` → `finish`. PASS.

---

## Issues this round

| ID | Title | Severity | Priority | Status |
|---|---|---|---|---|
| TEST-001 | Step 0 WARN swallowed for partial migration (Stage 1) | minor | P2 | verified |
| TEST-002 | gogo-done date extraction: `completed:` field is markdown-bolded; naive bash regex captures `**YYYY-MM-DD` | nit | P3 | new |

**Open/new count:** 1 (nit only, fixable — no blocking issues)

---

## New/extended tests added

None added to a test file (markdown plugin — dogfood structural checks are the test suite). The dogfood bash fixture at `scratchpad/donetest/` constitutes the persistent test artifact for TC-6.

---

## Verdict

**Stage 2 DONE-BAR: GREEN with one nit.**

All Stage 2 FRs (FR4, FR5, FR6, FR7) verified:
- FR4: report bundle lives in `report/` (not feature root); confirmed in skill, template comment, and architecture doc.
- FR5: `report.template.md` has Implementation + Decisions & rationale table (Choice + Reason columns).
- FR6: `use-case` added to kind enum; `additionalProperties: false` enforced; docs/contracts.md + templates/contracts/README.md both show the 5-kind set; `gogo-mermaid` documents the use-case flowchart pattern and chosen-by-diff rule.
- FR7: `/gogo:done` copy-not-move confirmed; date derived from `completed:` field; idempotent; validate-in guards; terminal `shipped` state; only `.gogo/` written.

TEST-002 is a nit (P3, fixable, LLM execution path unaffected). No blockers. Stage 2 may advance to Stage 3 or to report phase.
