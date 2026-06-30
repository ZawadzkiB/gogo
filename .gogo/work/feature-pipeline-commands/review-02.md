# Review 02 — feature `pipeline-commands` (Stage A) — round-2 re-review

**Verdict: APPROVE** — 0 blocker · 0 major · 0 minor · 0 nit (no new findings)

Focused re-verification of the five round-1 findings (REV-001..REV-005). All are
resolved, the chart-kind enumeration is now synced everywhere a producer is told
what to draw/name, the four contract schemas still parse, and the fixes introduce
no new cross-file inconsistency.

---

## Round-1 finding status

| Id | Severity | Status | Evidence |
|---|---|---|---|
| REV-001 | major | **RESOLVED** | The required grep returns no *manifest-kind* uses of `actions.mmd`/`structure.mmd`/`actions/lifecycle`/`kind: structure|actions` (exit 1, clean). `gogo-mermaid/SKILL.md:104-111`, `gogo-knowledge/SKILL.md:50-58`, `gogo-implement/SKILL.md:60-67`, `gogo/SKILL.md:69,141`, and `report.template.md:39` all now use **class**/**activity** as the kind names and explicitly state `kind ∈ {flow, sequence, class, activity}` — matching the `charts-manifest.schema.json` enum. The only residual "structure"/"actions" string is the schema's own `kind` *description* mapping (`class=structure/types; activity=action/lifecycle/state machine`), which is a concern→kind mapping, not a manifest `kind` value — correctly NOT drift. |
| REV-002 | minor | **RESOLVED** | `gogo/SKILL.md:123-124` now reads "Findings → `review/issues.json` (the living, typed contract) + a `review-NN.md` rendered snapshot per round"; `:133-134` the same for `test/issues.json` + `test-NN.md`. |
| REV-003 | minor | **RESOLVED** | `gogo-knowledge/SKILL.md` now: lists `charts/manifest.json` (re-written) in the out row (`:24`); step 2 (`:56-60`) instructs **re-write `charts/manifest.json`** to match the refreshed `.mmd` set; validate-out (`:84-87`) validates it against `charts-manifest.schema.json` and no longer claims the manifest is unschematized (only the `.mmd`/`.html` viewer are called prose/visual). |
| REV-004 | minor | **RESOLVED** | `README.md:212-213` typed-artifacts note now lists `pipeline.json` alongside `*/issues.json`, `charts/manifest.json`, and per-run `result.json` — symmetric with `gogo/SKILL.md:71-72` and `templates/contracts/README.md:24`. |
| REV-005 | nit | **RESOLVED** | `gogo-implement/SKILL.md:102-104` degradation note now cross-links the fallback: "Contract validation degrades per `gogo-contracts` — when no `jq`/schema validator is present, the agent checks the document against the schema directly (the semantic checks always run)." |

## New findings (round-2 drift check)

None. Verified no renamed filename is referenced in one place but not another:

- Chart kinds `{flow, sequence, class, activity}` agree across `gogo-mermaid`,
  `gogo-knowledge`, `gogo-implement`, `gogo/SKILL.md`, `report.template.md`,
  `README.md`, `contracts/README.md`, and the schema enum.
- `<phase>/result.json` path forms (`implement/`, `review/`, `test/`,
  `report/result.json`) all map to valid members of `phase-result.schema.json`'s
  `phase` enum `[plan, implement, review, test, report]`; `phase`/`status` values
  written by the skills are all valid enum members.
- `report/result.json` (the new gogo-knowledge output) is consistent with the
  schema's `<phase>/result.json` convention and the contracts README.

## Verification commands run

| Check | Result |
|---|---|
| `grep -rnE 'actions\.mmd\|structure\.mmd\|actions/lifecycle\|kind:\s*"?(structure\|actions)' skills/ templates/ README.md` | clean (exit 1, no matches) |
| `python3 -m json.tool` on all 4 `templates/contracts/*.schema.json` | all VALID (parse) |
| Broad sweep for stale `"structure"`/`"actions"` kind names | only the schema `kind` description mapping (not drift) |
| Phase/path enumeration cross-check vs `phase-result.schema.json` | consistent |

**Verdict: APPROVE** — all five round-1 findings resolved; no new drift.
