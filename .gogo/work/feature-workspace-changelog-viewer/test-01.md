# Test round 01 — feature `workspace-changelog-viewer`, Stage 1

**Date:** 2026-06-30
**Stage:** 1 of 3 (mechanical refactor: FR1 rename, FR2 resources move, FR3 build migration)
**Verdict:** 1 open issue (TEST-001, minor/P2) — all other Stage 1 checks green

---

## What was exercised

All tests are shell/Read (no UI — gogo is a markdown plugin with no app surface).
Playwright skipped per testing-tools.md ("N/A to the plugin itself").

| Level | Method |
|---|---|
| Structural grep | `grep -rn` across plugin source (commands/, skills/, agents/, templates/, docs/, README.md) |
| Artifact read | Read all `diagrams.html` files; resolve relative path; stat `.gogo/resources/mermaid.min.js` |
| Dogfood — clean migration | Full Step 0 bash executed against a scratch fixture in the scratchpad |
| Dogfood — idempotency | Step 0 bash executed a second time on same fixture |
| Dogfood — partial (no assets) | Step 0 bash executed on fixture where both `.gogo/plans/` and `.gogo/work/` exist but no `.assets/` |
| Dogfood — partial (with assets) | Step 0 bash executed on fixture where both dirs exist AND `.gogo/plans/.assets/` is present |
| Portability | Grep SKILL.md for sed-unavailable fallback documentation |

---

## Results by test case

### TC-1 — Rename completeness (FR1)

**Scope:** grep plugin source (`commands/`, `skills/`, `agents/`, `templates/`, `docs/`, `README.md`) for `.gogo/plans` or `.assets`.

**Expected:** ONLY `commands/build.md` and `skills/gogo-build/SKILL.md` may contain these strings.

**Result: PASS**

All hits were exclusively in those two files (migration prose + illustrative bash). No other plugin source file contains stale path references. The `.gogo/work/feature-*/` folders contain historical artifacts (written before the rename) but are NOT plugin source — excluded from scope per the test spec.

### TC-2 — Resources path (FR2)

**Scope:** `.gogo/` top-level structure; `diagrams.html` script src; `mermaid.min.js` existence.

**Checks:**
- `.gogo/` contains exactly `knowledge/`, `resources/`, `work/` — no `plans/`, no `.assets/`.
- All four `charts/diagrams.html` files (`feature-docs-and-verified-discovery`, `feature-pipeline-commands`, `feature-skill-extraction`, `feature-workspace-changelog-viewer`) use `src="../../../resources/mermaid.min.js"`.
- `.gogo/resources/mermaid.min.js` exists.
- Relative path resolution: from `.gogo/work/feature-<slug>/charts/`, `../../../resources/mermaid.min.js` resolves to `.gogo/resources/mermaid.min.js` — file confirmed present.

**Result: PASS**

### TC-3a — Migration dogfood: clean migration (FR3)

**Fixture:** scratchpad `/migtest/fixture1`
- `.gogo/plans/feature-demo/charts/diagrams.html` (script src `../../.assets/mermaid.min.js`)
- `.gogo/plans/.assets/mermaid.min.js` (dummy content)
- `.gogo/knowledge/tech-stack.md`

**Step 0 output:** `migrated: .gogo/plans->.gogo/work .gogo/work/.assets->.gogo/resources`

**Assertions:**
- `.gogo/plans/` gone; `.gogo/work/` present: PASS
- `.gogo/work/.assets/` gone; `.gogo/resources/mermaid.min.js` present: PASS
- `diagrams.html` script src rewritten to `../../../resources/mermaid.min.js`: PASS
- `mermaid.min.js` dummy content preserved (move-never-delete): PASS
- `knowledge/` dir preserved: PASS

**Result: PASS**

### TC-3b — Idempotency (FR3)

**Step 0 run 2 output:** `migration: already current (no-op)`

**Assertions:**
- No files moved or changed: PASS
- Output is exactly "already current (no-op)": PASS

**Result: PASS**

### TC-3c — Partial migration WARN, no assets (FR3 / REV-002 fix)

**Fixture:** scratchpad `/migtest/fixture3`
- `.gogo/plans/feature-old/plan.md` AND `.gogo/work/feature-new/plan.md` both present
- No `.assets/` directory

**Step 0 output:** `WARN: legacy .gogo/plans/ remains alongside .gogo/work/ — not moved (no clobber); merge by hand, then re-run`

**Assertions:**
- `.gogo/plans/` NOT clobbered: PASS
- WARN message printed: PASS
- Output is NOT "already current (no-op)": PASS

**Result: PASS**

### TC-3d — Partial migration WARN, assets present (FR3 / REV-002 fix — gap)

**Fixture:** scratchpad `/migtest/fixture2`
- `.gogo/plans/feature-old/plan.md` AND `.gogo/work/feature-new/plan.md` both present
- `.gogo/plans/.assets/mermaid.min.js` present

**Step 0 output:** `migrated: .gogo/plans/.assets->.gogo/resources`

**Expected:** WARN should ALSO be printed (`.gogo/plans/feature-old/` still exists alongside `.gogo/work/`).

**Actual:** No WARN printed. The `if [ -n "$moved" ]` branch fires (assets moved), prints "migrated: ...", and the `elif [ -d .gogo/plans ]` branch is never reached — even though `.gogo/plans/feature-old/` still exists and needs a manual merge.

**Result: FAIL — raises TEST-001**

### TC-4 — Portability: sed fallback documented (FR3)

`skills/gogo-build/SKILL.md` line 77-78: "Where `sed` is unavailable, do step 3 via Grep/Read/Write (same substitution) — never install a tool."

**Result: PASS**

---

## New issues raised this round

| ID | Severity | Priority | Status | Title |
|---|---|---|---|---|
| TEST-001 | minor | P2 | new | Step 0 WARN for partial migration is swallowed when .gogo/plans/.assets is also present and moveable |

**Agent-fixable:** yes

**Fix guidance:** Restructure the final log block so the WARN and the "migrated" line are independent. The `elif` on the WARN branch means it is only reachable when `moved` is empty. When `moved` is non-empty (`.assets` moved) but `.gogo/plans/` still exists alongside `.gogo/work/`, both should print. See `proposed_solution` in `test/issues.json` for the exact replacement block.

---

## Counts by severity

| Severity | Count |
|---|---|
| blocker | 0 |
| major | 0 |
| minor | 1 |
| nit | 0 |

---

## Done-bar assessment

Per `test-strategy.md`: "All enumerations in sync (grep); version bumped; portability intact."

| Bar | Status |
|---|---|
| No stale `.gogo/plans`/`.assets` in plugin source | PASS |
| All `diagrams.html` use correct `../../../resources/mermaid.min.js` | PASS |
| `.gogo/resources/mermaid.min.js` exists | PASS |
| `.gogo/` contains only `knowledge/`, `resources/`, `work/` | PASS |
| Migration: clean run migrates correctly | PASS |
| Migration: idempotent (second run is no-op) | PASS |
| Migration: partial-migration WARN fires (no-assets case) | PASS |
| Migration: partial-migration WARN fires (with-assets case) | FAIL — TEST-001 |
| Portability: sed fallback documented | PASS |
| All open/new issues resolved | FAIL — 1 open (TEST-001) |

**Overall: NOT fully green — 1 open issue; route back to implement.**
