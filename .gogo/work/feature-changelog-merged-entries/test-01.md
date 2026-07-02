# Test — round 1 — feature `changelog-merged-entries`

**Phase:** ④ test · **Round:** 1 · **Date:** 2026-07-02  
**Prior gate:** APPROVE (REV-001..004 all verified in review-02.md)  
**Approach:** dogfood fixture + spec-read (markdown plugin — no automated suite)

---

## What was exercised

| Level | Method | Scope |
|---|---|---|
| Artifact | Fixture dogfood (bash simulation in scratchpad) | Write changelog entry writer: merged A+B, single C |
| Spec-read | SKILL.md line-by-line | Arg grammar, gates, N=1 rule, D2, idempotency |
| Schema | python3 json validation | charts-manifest.schema.json back-compat (5 real + 7 work manifests) |
| Classifier | python3 simulation | gogo-status members[] detection + malformed-manifest safety |
| Viewer compat | python3 + spec-read | Slug-prefix pairing logic for compare mode |
| CLI | Direct execution | board.py --selftest |
| FR4 sweep | grep + python3 | plugin.json, command count, stale wording, git diff |

Scratchpad fixture: `/private/tmp/claude-502/-Users-bartlomiej-zawadzki-repos-gogo/f759479a-3eec-4068-9cfc-9d5b6edb19a8/scratchpad/fixture-project/`

---

## TC1 — Fixture dogfood: merged ship (A+B)

**Setup:** 3 fixture features in scratchpad `.gogo/work/`:
- `feature-add-appointments` — completed 2026-06-15, has `flow.mmd` + `sequence.mmd` + `before/flow.mmd`, `manifest.json`
- `feature-manage-appointments` — completed 2026-06-20, has `flow.mmd` + `activity.mmd`, `manifest.json`
- `feature-cancel-appointments` — completed 2026-06-10, has `flow.mmd`, `manifest.json`

**Execution:** Followed `skills/gogo-done/SKILL.md` step-by-step for a merged ship of A+B (D2=A: accepted suggested name "appointments"):

**Step 2 — Date derivation (bash, per SKILL.md code):**  
Newest member date = `2026-06-20` (manage-appointments). Release name = `appointments` (common theme of `add-appointments` + `manage-appointments`, longest shared word). Entry dir: `.gogo/changelog/2026-06-20-appointments/`.

**Step 3 — Synthesized report.md:**  
Written (not copied). 32 lines. Diff against each member's report: DIFFERENT in all 140+ lines. Contains: lead paragraph (both features), key decisions (3 one-liners), review/test verdict (1 sentence), member table (slug · title · one-line outcome), per-member sections, links back to `.gogo/work/feature-*/`.

**Step 4 — Slim file set:**

```
.gogo/changelog/2026-06-20-appointments/
├── add-appointments-flow.mmd          (slug-prefixed from feature A)
├── add-appointments-sequence.mmd     (slug-prefixed from feature A)
├── manage-appointments-activity.mmd  (slug-prefixed from feature B)
├── manage-appointments-flow.mmd      (slug-prefixed from feature B)
├── manifest.json                      (members: [add-appointments, manage-appointments])
├── report.md                          (synthesized, written)
└── before/
    └── add-appointments-flow.mmd     (slug-prefixed from feature A's before/)
```

No `diagrams.html` present. PASS.

**manifest.json schema validation:** PASS — all required fields (slug, diagrams), members[] present, all diagrams[] entries have {kind, file, title}, additionalProperties:false respected.

**Step 5 — State flip:**  
feature-add-appointments/state.md: `status: shipped`, `resume: none — shipped to .gogo/changelog/2026-06-20-appointments/`  
feature-manage-appointments/state.md: `status: shipped`, `resume: none — shipped to .gogo/changelog/2026-06-20-appointments/`  
feature-cancel-appointments/state.md: `status: done` (UNTOUCHED). PASS.

**Idempotency:** Re-ran the same merge ship. Directory contents unchanged, same file count, same timestamps (updated). Still exactly 1 changelog entry. Feature C still untouched. PASS.

**Verdict: TC1 GREEN**

---

## TC2 — Single-slug ship (feature C)

Feature C shipped alone (`/gogo:done cancel-appointments` semantics).

Entry: `.gogo/changelog/2026-06-10-cancel-appointments/`  
Files: `cancel-appointments-flow.mmd`, `manifest.json` (members: ["cancel-appointments"]), `report.md` (synthesized — not a copy of the member's report.md, 17 lines).  
No `diagrams.html`. PASS.

**manifest.json validation:** PASS — same writer, same shape, `members: ["cancel-appointments"]`.

**Single path in SKILL.md:** The SKILL.md contains no divergent single-vs-merged code path. Grep for "Ship one feature" returns 0 hits. Both paths funnel through the single "Write changelog entry (1..N members)" section. PASS.

**Verdict: TC2 GREEN**

---

## TC3 — Arg grammar + gates (spec-read)

All grammar/gate checks against `skills/gogo-done/SKILL.md`:

| Check | Result |
|---|---|
| `slug1+slug2+slug3` arg grammar documented | PASS |
| `+` pre-answers merge gate (skips board) | PASS |
| board ≥2 → one `AskUserQuestion` (separate vs merged) | PASS |
| N=1 → no merge question | PASS |
| D2 = suggest + confirm release name | PASS |
| Date = newest member `completed:` | PASS |
| `Write` tool (not `cp`) for report.md | PASS |
| `diagrams.html` dropped | PASS |
| `members[]` array in manifest | PASS |
| Idempotent `rm -f *.mmd; rm -rf before` | PASS |
| No divergent "Ship one feature" legacy path | PASS |
| Partial-valid `+list` behavior documented (skip, continue) | PASS |

**One potential ambiguity noted** (non-blocking): `gogo-view` SKILL.md uses the variable `<kind>` in the compare-mode pairing description ("Match each `before/<kind>.mmd` to the after `<kind>.mmd`") which historically referred to the diagram kind enum (flow/sequence/class) but now must be read as "filename stem including slug prefix". TC5 confirms the behavior is correct and the review verified this point (REV-004), but an executing agent reading only gogo-view without gogo-done context could misread it as kind-only matching. The actual pairings work correctly because both sides carry the same slug prefix. Tagged as informational — not a blocker.

**Verdict: TC3 GREEN** (one informational ambiguity in gogo-view's <kind> variable naming — tagged below as TEST-001, nit severity)

---

## TC4 — Classifier members[] detection

gogo-status classifier simulation against fixture (post-merge):

| Feature | State.md status | Folder-slug match | members[] match | Classified as |
|---|---|---|---|---|
| add-appointments | shipped | no (folder = "appointments") | YES (in 2026-06-20-appointments/manifest.json) | **shipped** |
| manage-appointments | shipped | no (folder = "appointments") | YES | **shipped** |
| cancel-appointments | done | yes (2026-06-10-cancel-appointments/) | YES (members=["cancel-appointments"]) | **shipped** |

**Malformed manifest test:** Added a 4th fake entry `.gogo/changelog/2026-01-01-broken-entry/manifest.json` with `{invalid json <<<`. Python JSON parse raised `JSONDecodeError` — caught, logged as a note, skipped. Classifier continued cleanly. PASS.

**Verdict: TC4 GREEN**

---

## TC5 — Viewer compatibility (spec-read + pairing simulation)

`skills/gogo-view/SKILL.md` checks:

| Check | Result |
|---|---|
| Enumerates `.gogo/changelog/*/` | PASS |
| Picks `*.mmd` beside `report.md` for changelog entries | PASS |
| `data-diagram` set to basename (no extension) | PASS |
| Skips `diagrams.html` in diagram gather | PASS |
| Pairs by basename (before/ basename matches after basename) | PASS |
| `compare-before` / `compare-after` CSS classes | PASS |
| `<date>-<name>` arg grammar for changelog entries | PASS |
| Missing `diagrams.html` is non-event | PASS |

**Slug-prefix pairing simulation** against the merged fixture entry:
- `before/add-appointments-flow.mmd` ↔ `add-appointments-flow.mmd` → PAIR (compare side-by-side)
- `add-appointments-sequence.mmd` → SOLO "Added" (no before counterpart)
- `manage-appointments-flow.mmd` → SOLO "Added"
- `manage-appointments-activity.mmd` → SOLO "Added"

Pairing logic confirmed correct: both `before/<slug>-x.mmd` and `<slug>-x.mmd` share the same basename, so the viewer's "match by basename" rule works without any code change.

**Verdict: TC5 GREEN**

---

## TC6 — Schema back-compat

Validated against `templates/contracts/charts-manifest.schema.json`:

| Manifest | members | Result |
|---|---|---|
| `.gogo/changelog/2026-06-29-skill-extraction/manifest.json` | (absent) | PASS |
| `.gogo/changelog/2026-06-30-docs-and-verified-discovery/manifest.json` | (absent) | PASS |
| `.gogo/changelog/2026-06-30-workspace-changelog-viewer/manifest.json` | (absent) | PASS |
| `.gogo/changelog/2026-07-01-diagram-viewer-and-uml-diff/manifest.json` | (absent) | PASS |
| `.gogo/changelog/2026-07-01-viewer-bundles-and-done-board/manifest.json` | (absent) | PASS |
| 7 × `.gogo/work/feature-*/charts/manifest.json` | (absent) | PASS |

**additionalProperties:false intact:** Test manifest with `UNKNOWN_KEY` correctly rejected. PASS.

**Members optional:** All 5 real changelog manifests (no members field) still validate. PASS.

**Verdict: TC6 GREEN**

---

## TC7 — FR4 sync sweep

| Check | Result |
|---|---|
| `plugin.json` version = 0.8.0 | PASS |
| commands/ count = 12 | PASS (build, done, go, implement, plan, report, resume, review, skills, status, test, view) |
| Stale "copies the bundle/report bundle" in product files | PASS (none — clean) |
| Stale "one entry per feature" | PASS (none — clean) |
| `board.py` untouched (not in git diff) | PASS |
| `board.py --selftest` exit 0 | PASS (7 tests, all PASS) |
| `diagrams.html` wording in product docs: all legitimate references (work-folder bundle) or "no copy"/"dropped" language | PASS |
| docs/architecture.md, docs/index.md, commands/done.md, docs/commands.md, docs/flow.md, README.md, skills/gogo/SKILL.md — synthesis language present | PASS |

**Verdict: TC7 GREEN**

---

## Issues

No new issues raised. All test cases GREEN.

| id | sev | pri | status | title |
|---|---|---|---|---|
| TEST-001 | nit | P3 | new | gogo-view uses `<kind>` variable for compare-mode pairing but slug-prefixed filenames make "kind" ambiguous |

---

## Verdict against done-bar

- Build: N/A (no compile step — markdown plugin)
- Unit: N/A (no automated suite — confirmed by testing-tools.md)
- E2e: N/A (no e2e suite — confirmed; this is dogfood-based testing per test-strategy.md)
- Hands-on dogfood: DONE — merged entry writer simulated end-to-end, artifacts inspected, schema validated
- All open issues: **1 nit** (TEST-001) — informational only, non-blocking

**OVERALL: GREEN** — the done-bar is met. The single nit (TEST-001) is a wording ambiguity in gogo-view's compare-mode description that does not affect behavior (the pairing is confirmed correct by simulation and review REV-004). No blockers, no majors, no minors. Recommend advancing to phase ⑤ report.
