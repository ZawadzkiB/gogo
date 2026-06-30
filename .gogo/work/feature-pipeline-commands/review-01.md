# Review 01 — feature `pipeline-commands` (Stage A)

**Verdict: CHANGES** — 0 blocker · 1 major · 3 minor · 1 nit

Stage A is substantially correct: all four schemas are valid JSON (verified with
both `jq` and `python3 -m json.tool`) and coherent draft-07 (enums match FR4
exactly; the `status == fixed ⇒ fixed_in_round + fix_summary` conditional is
expressed correctly via `allOf/if/then`). Artifact paths are consistent across
schemas, the `gogo-contracts` registry, the phase skills, the commands, and the
README. Two-tier portable validation with an agent fallback is specified. Write
scope, the plan-acceptance gate, decision gates, and bounded loops are intact.
Stage B is correctly deferred (`commands/go.md` and the orchestrator loop are
unchanged except enumeration/vocabulary edits). The version is bumped
0.1.4 → 0.2.0.

One **major** cross-file enumeration drift blocks approval: the chart-`kind`
contract (`{flow, sequence, class, activity}`) is not synced into the two skills
that actually drive diagram production, so a conformant producer can emit a
manifest that fails its own validate-out gate.

---

## Findings

### REV-001 — Chart-kind enumeration out of sync with the schema (producers still say "actions"/"structure")  **[agent-fixable]**
- **Severity:** major
- **Where:** `skills/gogo-mermaid/SKILL.md:105-109`; `skills/gogo-knowledge/SKILL.md:49-52`; `templates/report.template.md:39`
- **Description:** `charts-manifest.schema.json` fixes the machine contract:
  `kind ∈ {flow, sequence, class, activity}` (verified). `gogo-implement` step 5
  (the primary producer) and `gogo/SKILL.md` were updated to this vocabulary, but
  the two skills that actually tell the author *how to draw and name the diagrams*
  were not:
  - `gogo-mermaid/SKILL.md:105-109` still instructs producing an
    **"actions/lifecycle"** diagram and a **"structure"** view, and names files
    `actions.mmd` / `structure.mmd`.
  - `gogo-knowledge/SKILL.md:49-52` names the kinds **"Actions / lifecycle"** and
    **"Structure"**.
  - `templates/report.template.md:39` lists "flow, sequence, actions/state,
    structure".

  `gogo-implement` step 5 delegates the actual drawing to `gogo-mermaid`, and
  `gogo-knowledge` writes/refreshes charts directly. An LLM following that prose
  will naturally record `kind: "structure"` or `kind: "actions"` in
  `charts/manifest.json`, which is **not** in the enum → the manifest fails
  validate-out against `charts-manifest.schema.json`. This is precisely the
  "enumeration left out of sync" Major (and a latent "producer output a consumer
  can't parse") in `code-review-standards.md`. The schema's `kind` *description*
  already maps `class=structure/types` and `activity=action/lifecycle/state`, so
  the intent is settled — the producer prose just needs to use the contract's
  value names.
- **Proposed solution:** In `gogo-mermaid/SKILL.md:104-109`,
  `gogo-knowledge/SKILL.md:49-52`, and `report.template.md:39`, rename the kind
  labels to the contract values: use **class** (was "structure") and **activity**
  (was "actions/lifecycle"), e.g. "**class** (`classDiagram`) — structure/types"
  and "**activity** (`stateDiagram-v2`/activity flowchart) — actions / lifecycle /
  state machine". Update example filenames to `class.mmd` / `activity.mmd`. State
  once that the manifest `kind` must be one of `{flow, sequence, class, activity}`.

### REV-002 — Orchestrator phase descriptions still point only at `review-NN.md`/`test-NN.md`, not the `issues.json` contract  **[agent-fixable]**
- **Severity:** minor
- **Where:** `skills/gogo/SKILL.md:123` ("Findings → `review-NN.md`.") and `:132` ("Results → `test-NN.md`.")
- **Description:** The same file's artifact list (`:66-72`) was updated to make
  `review/issues.json` / `test/issues.json` the living contract with the `-NN.md`
  as the rendered snapshot, but the ③/④ phase descriptions in the body still
  describe output only as the markdown files. This is prose drift from the new
  contract vocabulary (the routing logic itself is correctly left for Stage B, so
  no behavioural change is expected here — only the description). Cross-file
  consistency check #1.
- **Proposed solution:** Update both lines to name the living list as the output
  and the snapshot as the rendered view, e.g. "Findings → `review/issues.json`
  (living, typed) + a `review-NN.md` snapshot." Same for test at `:132`.

### REV-003 — `gogo-knowledge` refreshes charts but never updates/validates `charts/manifest.json`  **[agent-fixable]**
- **Severity:** minor
- **Where:** `skills/gogo-knowledge/SKILL.md:23` (out row), `:39-54` (step 2), `:76-80` (validate-out)
- **Description:** The skill consumes `charts/manifest.json` as an input and step
  2 redraws/refreshes the as-built diagram set ("refreshed `charts/`";
  `gogo/SKILL.md:69` says "⑤ refreshes it"). But it never re-writes the manifest to
  match the refreshed `.mmd` set, and validate-out (`:76-80`) explicitly says the
  diagrams are "prose/visual artifacts (no JSON schema)" — overlooking that
  `charts/manifest.json` *is* schema-governed. If report adds/renames diagrams,
  the manifest goes stale (its `file` paths / coverage no longer match disk),
  which a later semantic "real paths" check would flag. Report is terminal so
  nothing in the loop re-reads it, hence minor — but it leaves an inconsistent
  contract artifact.
- **Proposed solution:** In step 2, have report re-write `charts/manifest.json`
  to reflect the refreshed set, and in validate-out add it to the schema-validated
  outputs (`charts-manifest.schema.json`) rather than excluding all diagrams as
  "no schema". Add `charts/manifest.json` to the `out` row.

### REV-004 — README "What gets created" omits per-run `result.json` / `pipeline.json` while the plugin-internal docs include them  **[agent-fixable]**
- **Severity:** minor
- **Where:** `README.md:213-218`
- **Description:** The README's new typed-artifacts note lists
  `*/issues.json`, `charts/manifest.json`, and `result.json`, but not
  `pipeline.json`, whereas `skills/gogo/SKILL.md:67-72` and
  `templates/contracts/README.md` both enumerate `pipeline.json` as a feature-level
  typed artifact. Minor enumeration gap in the user-facing list. (Acceptable to
  defer `pipeline.json` mention to Stage B since it is the orchestrator index, but
  then it should also be deferred in `gogo/SKILL.md` for symmetry — pick one.)
- **Proposed solution:** Either add `pipeline.json` to the README note, or note
  that `result.json`/`pipeline.json` are emitted by the standalone commands and
  consumed by `go` in Stage B — and keep that framing consistent with
  `gogo/SKILL.md:70-72`.

### REV-005 — `gogo-implement` degradation note covers `git`/`mmdc` but not the validation-tool absence the new gates introduce  **[agent-fixable]**
- **Severity:** nit
- **Where:** `skills/gogo-implement/SKILL.md` "Degradation" section (`:96-103`)
- **Description:** The new validate-in/out gates lean on `jq`/`python3`/a schema
  validator, all optional. The portable fallback is correctly specified centrally
  in `gogo-contracts/SKILL.md` (tier 1 tool-detected, tier 2 agent-always), and the
  phase skills delegate validation there, so this is not a portability hole — just
  a readability nit that the implement skill's own degradation note doesn't
  cross-link the agent-validate fallback. Optional.
- **Proposed solution:** Optionally add one line: "validation degrades per
  `gogo-contracts` — no validator present ⇒ the agent checks against the schema."

---

## Dimension checklist (for the orchestrator)

| Check (per `code-review-standards.md` / plan) | Result |
|---|---|
| JSON validity (jq + python3, all 4 schemas) | PASS — all parse |
| Schema coherence (enums = FR4; `fixed ⇒ fixed_in_round+fix_summary`) | PASS |
| Cross-file enumeration sync (chart kinds) | **FAIL — REV-001 (major)** |
| Cross-file enumeration sync (issues.json vocabulary) | minor drift — REV-002 |
| Artifact path agreement (producers/consumers) | PASS |
| Version bumped (0.1.4 → 0.2.0) | PASS |
| Portability — no new required dep, agent-validate fallback specified | PASS |
| Write-scope confined to `.gogo/` | PASS |
| Plan-acceptance gate / decision gates / bounded loops intact | PASS |
| Commands stay thin (logic in skills) | PASS |
| Stage B deferred (go.md + orchestrator loop unchanged) | PASS |

**Verdict: CHANGES** — clear REV-001 (major) to approve; REV-002/003/004 are
small sync fixes worth folding in the same pass; REV-005 is an optional nit.
