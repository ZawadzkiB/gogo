# Review round 2 — feature `analyst-uat-and-cli-ops`

**Scope:** Stage B (UAT gate + `## Custom`) delta, fresh eyes, on top of the
Stage-A-approved working tree. Reviewed against plan.md FR4/FR5, decisions.md
D1/D2 + non-forks, and `.gogo/knowledge/{code-review-standards,coding-rules,
non-functional-requirements}.md`. Stage C (CLI, 0.11.0) has not run — plugin.json
still `0.10.0` and no `cli/` changes is **correct**.

**Verdict: CHANGES** — 2 open majors, 3 minors, 1 nit. (REV-001/002 verified.)

| id | sev | prio | status | one-line |
|---|---|---|---|---|
| REV-001 | major | P1 | verified | README "nine files" → "ten files" (fixed round 2) |
| REV-002 | nit | P3 | verified | orchestrator-first one-liner added to implement.md/report.md (fixed round 2) |
| REV-003 | major | P1 | new | `commands/report.md:46` still says "set `state.md` to done" — pre-UAT status |
| REV-004 | major | P2 | new | UAT loop leaves `status: awaiting-uat` mid-re-plan → ship-able / rerun-able before re-acceptance |
| REV-005 | minor | P2 | new | UAT re-accept has orchestrator emit `plan-accepted` (owner is `gogo-plan`) — single-owner contradiction |
| REV-006 | minor | P2 | new | `architecture.md:18` pipeline one-liner still ends "⑤ REPORT → done" |
| REV-007 | minor | P2 | new | `/gogo:skills` can flag/extract a `## Custom` section (not exempted like `## gogo overrides`) |
| REV-008 | nit | P3 | new | `commands/report.md:48-49` garbled nested-parens orchestrator note; implies ⑤ is agent-delegated |

## Findings

### REV-003 (major) — commands/report.md still writes the old `done` status
`commands/report.md:45-46` validate-out: "…set `state.md` to done." Phase ⑤ now
ends at `status: awaiting-uat` on a green run (per `gogo-knowledge/SKILL.md`), and
the parallel prose in `docs/commands.md:141-143` **was** updated to "sets
`state.md` to **`awaiting-uat`** (the UAT gate — no longer `done`)". This command
reference is the one summary the sweep missed — it now describes the old behaviour
(code-review-standards §1). `phase` IS still `done`, which is exactly what makes
the half-true line misleading. **Fix:** match `gogo-knowledge` + `docs/commands.md`.

### REV-004 (major) — mid-re-plan status stays `awaiting-uat`
`gogo/SKILL.md:191-207`: the UAT loop's step 1 sets only `resume: report`; the
STATUS stays `awaiting-uat` while `gogo-analyst` rewrites `plan.md` and until the
user re-accepts (step 3 → `plan-accepted`). So a re-plan-in-progress is
indistinguishable from "awaiting first verdict". Because `state.md` is the durable
resume file and the CLI board shows an `awaiting-uat` badge with launch actions,
this is reachable out-of-band: (i) the classifier reports **ready-to-ship** during
a re-plan; (ii) `/gogo:done` validate-in accepts on `awaiting-uat` and would ship
the revised-but-unimplemented plan (shipped `report.md`/`plan.md` then describe a
plan the code doesn't implement — bypassing re-acceptance AND re-implementation);
(iii) `go.md:14-15,19-21` allows a "resumable in-loop state" as an alternative to
`plan-accepted`, and `awaiting-uat` is undefined w.r.t. that clause. Contradicts
D1 (accept vs re-plan branches must be mutually exclusive) and code-review-standards
§5 ("state.md kept current at transitions"); the established decision-gate pattern
(set `waiting-for-user` on entry) is not followed. Happy path (one continuous chat)
is safe. **Fix:** set `status: waiting-for-user` when the UAT loop opens, hold until
re-acceptance flips to `plan-accepted`; optionally state `awaiting-uat` is not a
resumable-in-loop state for `/gogo:go`.

### REV-005 (minor) — plan-accepted single-owner contradiction
`gogo/SKILL.md:202` (UAT step 3) tells the orchestrator to "append the plan's
`plan-accepted` event", but `gogo/SKILL.md:279-282` says the orchestrator emits
**only** the gate + UAT events, `cli-contract.md:220` pins `plan-accepted` to
`gogo-plan`, and `gogo-plan/SKILL.md:129-131` says "the orchestrator emits none".
The documented invariant is one owner per event. Low functional impact (lenient
consumer) but a real contract inconsistency. **Fix:** drop the plan-accepted emit
from UAT step 3 (uat-failed already marks the re-accept), or reconcile the owner
docs.

### REV-006 (minor) — architecture.md flow one-liner not updated
`docs/architecture.md:18` still shows "…⑤ REPORT → done" while `flow.md`,
`index.md`, the `gogo` skill and README all now show the UAT gate → shipped.
Doc-sync completeness gap (same class as REV-001, coarser). **Fix:** add the UAT
gate to the one-liner.

### REV-007 (minor) — /gogo:skills is not `## Custom`-aware
The feature installs an identical `## Custom` stub in all 20 knowledge files
promising "gogo never rewrites this section", but `skills/gogo-skills/SKILL.md`
(untouched — `git diff --stat` empty) excludes only `gogo:meta` and
`## gogo overrides` from candidate discovery (Step 2:52-53) and counts all body
lines in the budget (Step 1/Budget:30). A ≥20-line `## Custom` in a WARN/OVER file
could be proposed and, on approval, extracted to a stub — gogo rewriting a section
it swears it never touches. User-gated (Step 4 STOP) and outside FR5's literal
scope, hence minor. **Fix:** add `## Custom` to Step 2's never-flag list and
exclude it from the budget count, mirroring `## gogo overrides`; or record the
deferral.

### REV-008 (nit) — garbled report.md orchestrator note
`commands/report.md:48-49`: "…delegates the phase to its specialist (gogo-knowledge
(run by the orchestrator))…" — nested parens and ⑤ is not agent-delegated
(everywhere else: "⑤ orchestrator + gogo-knowledge"). **Fix:** reword to the house
phrasing.

## What was checked and is clean
- **UAT state machine** walks end-to-end with no deadlock: ⑤ → `awaiting-uat`
  (resume hint set), `/gogo:done` accepts (records `uat.md` round + `uat-passed`),
  or feedback → analyst → `uat.md` round → plan delta → **re-accept required** →
  `/gogo:go` reruns ②→⑤. `/gogo:go` still refuses unless `plan-accepted` in the
  happy path. The one gap is the durable mid-loop status (REV-004).
- **Back-compat is precise:** a NEW (0.11) feature can never reach report-complete
  at `status: done` (⑤ sets `awaiting-uat`; lenient leaves the real pre-report
  value, never `done`), so the `done`-accepting back-compat clause cannot skip UAT
  on a new feature — it only matches genuinely pre-0.11 / already-shipped members.
- **events.schema.json** is purely additive (3 enum values inserted, nothing
  removed/renamed); the feature's own `events.jsonl` and synthetic
  `uat-opened`/`uat-passed`/`uat-failed` lines all validate; the single-owner table
  (cli-contract §5) covers all 3 new events with owners matching every emission
  instruction (`uat-opened`/`uat-failed` = orchestrator, `uat-passed` = gogo-done).
- **Classifier truth:** `awaiting-uat` + report ⇒ ready-to-ship stated identically
  in `gogo-status` and `cli-contract §3`; four classes/columns unchanged; badge is
  a Stage-C CLI concern.
- **`## Custom`:** all 20 stubs byte-identical; build preserves across reconcile,
  `--force`, and `_discovered` regen; ⑤ guard present ("never touch a `## Custom`
  section"); overrides-vs-Custom distinction crisp in architecture/commands/README.
- **Slim-done:** no step requires script execution — date derivation, copies, and
  validation are all optional/graceful (Read-or-bash, `jq`/`python` optional);
  board-mode text untouched.
- **Scope:** `plugin.json` correctly 0.10.0; no `cli/` changes; REV-001/002 fixes
  present and not regressed.
