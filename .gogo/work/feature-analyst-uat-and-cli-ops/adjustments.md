# Adjustments — feature `analyst-uat-and-cli-ops`

Running log of user-requested changes / clarifications during planning.

## 2026-07-04 — origin (verbatim intent, grouped)

CLI: delete cards/work · check logs of attached sessions without opening fully.
gogo: /gogo:done sessions must not ask for script approvals (separate session →
all-accept) · plan phase must name the exact knowledge files + new `analysis.md`
describing how to analyze (what files to check; maybe notion/confluence skills
for external docs) · /gogo:build re-runs should preserve a user `#custom`
section 1:1 (ask to copy, document it) · every gogo command starts the
orchestrator first, which starts the lower agents · a separate planning agent
(`gogo-analyst`, very detailed, code = source of truth) · a UAT step between
gogo:go and gogo:done — user checks the work; on issues go back to planning,
verify new requirements vs current plan+decisions, adjust plan.md, save uat.md
with the user input, rerun gogo:go — SAME work item, never a new one.

## 2026-07-04 — gate comments (user), plan revised

- **D1 custom — the plan-gate symmetry:** no confirm question. After go, state.md
  = awaiting-uat; the user either ASKS (→ gogo-analyst analyzes the input vs
  plan.md + decisions.md + code, appends the uat.md round, proposes the plan
  delta, user re-accepts, go reruns — same item) or TRIGGERS /gogo:done — which
  IS the acceptance (recorded in uat.md), mirroring how plan acceptance unlocks
  go.
- **D2 custom — auto mode, not bypass:** launches use Claude's auto
  (classifier-based) permission mode + env override; AND gogo-done's skill is
  slimmed to read/write/copy + synthesis ("just prepare the changelog entry from
  the work item report and files") so auto mode covers it without bypass.
- D3 = A (trash). D4 = A (badge) implied unobjected.
