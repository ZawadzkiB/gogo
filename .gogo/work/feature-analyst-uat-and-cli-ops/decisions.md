# Decisions — feature `analyst-uat-and-cli-ops`

## D1 — How the UAT gate is recorded and passed

**Question:** what's the mechanic between report ⑤ and ship?
- **A (recommended):** ⑤ ends at **`status: awaiting-uat`**; `/gogo:done`'s
  validate-in requires it and asks ONE confirm ("UAT passed?") — pass records
  `uat.md` + ships; issues are collected in chat → `uat.md` round → plan adjusted
  on the SAME item → re-accept → `/gogo:go` reruns. No new command.
- **B:** a dedicated `/gogo:uat` command (13th command) owning the gate.

**Recommendation:** A — fewer commands, the gate lives where shipping already is.

**RESOLVED (2026-07-04):** **custom — the plan-gate symmetry.** state.md records
awaiting-uat after go; running /gogo:done IS the acceptance (uat.md verdict
line, no confirm question); questions instead route to gogo-analyst (analyze vs
plan+decisions+code → uat.md round → plan delta → re-accept → rerun go). Same
item always.

## D2 — Launched-session permission mode

**Question:** how do board-launched claude sessions stop asking for approvals?
- **A (recommended):** launch with full auto-accept
  (`--dangerously-skip-permissions`) by DEFAULT — the session is user-confirmed
  (huh), purpose-built, cwd-anchored to the repo; `GOGO_CLAUDE_PERMISSION_MODE`
  env restores prompting or picks a milder mode; the confirm form states the
  mode.
- **B:** `--permission-mode acceptEdits` (edits auto, Bash still prompts — the
  py-script nag the user hit would REMAIN).
- **C:** rely on per-project `settings.local.json` allowlists (setup burden per
  project, silently incomplete).

**Recommendation:** A (with the env escape hatch) — it's the only mode that
actually delivers "no asking", and the human already confirmed the launch.

**RESOLVED (2026-07-04):** **custom — auto mode + slimmer done.** Launch in
Claude's auto (classifier) permission mode with the env override — NOT full
bypass — and slim gogo-done to read/write/copy + synthesis ("just prepare the
changelog entry from the report and files") so auto mode covers everything it
does.

## D3 — Delete semantics on the board

**Question:** what does deleting a card do?
- **A (recommended):** move the work folder to **`.gogo/trash/<ts>-<slug>/`**
  (recoverable; `gogo trash` lists/restores); changelog entries not deletable.
- **B:** hard `rm -rf` behind a typed-slug confirm.

**Recommendation:** A — destructive-from-a-TUI must be reversible.

**RESOLVED (2026-07-04):** **A** (trash + restore).

## D4 — awaiting-uat on the board

**Question:** new column or badge?
- **A (recommended):** stays in **ready** with an `awaiting-uat` badge —
  classifier classes remain stable (frozen contract stays additive).
- **B:** a fifth column (contract-breaking for consumers).

**Recommendation:** A.

**RESOLVED (2026-07-04):** **A** (badge on ready; classes stay stable) — implied
unobjected at the gate.

## Non-forks (recorded)

- `analysis.md` = 10th knowledge file, owned-mode, build-synthesized; external
  docs (notion/confluence) are a HOOK in analysis.md, not skills built now.
- `gogo-analyst` mirrors the developer/reviewer/tester pattern; orchestrator
  keeps the acceptance gate in chat.
- `## Custom` = user-owned, preserved 1:1 by build AND phase-⑤ reconciles;
  distinct from `## gogo overrides` (gogo-authored). Build's summary must state
  what it preserved.
- Orchestrator-first is a documentation/architecture formalization — commands →
  orchestrator → specialist agents; no behavioral rewrite of working flows.
- UAT loop never creates a new work item; iterations gain `uat=N`.
- Versions: plugin + CLI → **0.11.0** together.

## Implementation note (review round 2 — orchestrator-resolved, recorded)

**REV-004 mid-UAT status:** when the user raises UAT issues, the orchestrator
immediately sets `status: waiting-for-user` (the existing decision-gate value;
`open-decision: UAT round N`, `resume: plan`) for the whole analyst-analysis +
re-plan stretch; only user re-acceptance flips it to `plan-accepted`. This makes
mid-loop features fail /gogo:done's validate-in (needs awaiting-uat) AND
/gogo:go's gate (needs plan-accepted) — no ship/rerun without re-acceptance.
Mirrors the decision-gate pattern; classifier: waiting-for-user + report ⇒ still
ready-to-ship class but the CLI badge (Stage C) shows waiting-for-user (already
implemented in 0.10.0's badge logic). Not escalated: the reviewer's recommended
fix, no trade-off.

## Implementation note (test round 1 — orchestrator-resolved, recorded)

**TEST-004 classifier fix:** the ready-to-ship rule becomes "a final report
exists AND status is `awaiting-uat` (or legacy `done`)" — report-presence alone
no longer suffices, so a mid-UAT rerun (stale report/, status implementing/
plan-accepted/waiting-for-user) classifies in-progress. Amended in the frozen
contract (additive clarification, 0.11.0 block), gogo-status, and the Go
classifier together. Consistent with the REV-004 gate-lock philosophy. Not
escalated: it's the only reading under which the UAT loop's one-legal-command
property survives at the classifier layer.
