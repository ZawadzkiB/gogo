# State — feature `projects-sources-plans`

<!-- Files in this folder (.gogo/work/feature-<slug>/):
  - plan.md        — the accepted plan (the contract) + the feature's functional requirements
  - adjustments.md — log of changes / clarifications you asked for during planning
  - state.md       — THIS file: current phase / status / iterations; lets work resume
  - decisions.md   — forks that needed your call + gogo's recommendation + your answer
  - uat.md         — the UAT gate log: one round per user check after ⑤ (verbatim input + analyst analysis + plan delta + verdict); only appears once ⑤ reaches awaiting-uat
  - review/issues.json — living, typed review findings (the contract; see templates/contracts/)
  - review-NN.md   — each code-review round's rendered snapshot of issues.json
  - test/issues.json   — living, typed test findings (same contract)
  - test-NN.md     — each test round's rendered snapshot
  - events.jsonl   — append-only progress telemetry (one schema'd JSON line per phase transition; read by the gogo CLI; a missing file is never an error)
  - report/        — the as-built bundle (written at phase ⑤): report.md + the UML set (.mmd) + report/before/ (the plan-time "before" set, copied in for before/after compare) + diagrams.html + result.json
  - charts/        — mermaid diagrams (.mmd) + charts/before/ (the plan-time as-is baseline) + manifest.json + an offline diagrams.html viewer
  (and at the .gogo/ level: .gogo/changelog/<YYYY-MM-DD>-<slug>/ — the shipped report bundle archived by /gogo:done)

  UAT gate (from 0.11.0): phase ⑤ ends at status `awaiting-uat`, not `done`. The user verifies
  the work; running `/gogo:done` IS the acceptance. Or UAT feedback re-plans the SAME item: the
  orchestrator sets status `waiting-for-user` for the WHOLE re-plan stretch (so a mid-loop feature
  can't ship — /gogo:done needs awaiting-uat — or rerun — /gogo:go needs plan-accepted), records the
  round in uat.md, and only re-acceptance flips it to `plan-accepted`, after which /gogo:go reruns
  ②→⑤ back to awaiting-uat — `uat=N` tracks the loops.
-->

- **feature:** Re-architect the cockpit to projects · sources · plans (home-folder project → many sources → work items with a correlation list in state.md); tabbed TUI
- **phase:** done       <!-- plan | implement | review | test | knowledge | done -->
- **status:** shipped   <!-- /gogo:done accepted UAT + shipped as 0.21.0 (2026-07-19) -->
- **implement-done:** 2026-07-18 — Phases A–D all green, version 0.21.0, go test -race ./... clean
- **report-done:** 2026-07-18 — as-built report bundle written (report/report.md + class/sequence/activity .mmd + before/ compare); plan.md finalized to as-built; project-knowledge.md gogo-overrides reconciled (P1-P4 0.21-0.24 narration -> the shipped 0.21.0 rework); uat.md created
- **created:** 2026-07-18
- **accepted:** 2026-07-18
- **completed:** 2026-07-19
- **branch:** n/a
- **iterations:** plan=0 · implement=6 · review=1 · test=1 · uat=1   <!-- uat=1: round 1 looped back; implement=6 = FR19–FR22 two-mode delta -->
- **resume:** none — SHIPPED as 0.21.0; changelog at .gogo/changelog/2026-07-19-projects-sources-plans/
- **open-decision:** none   <!-- UAT round 1 command surface confirmed = option A (gogo global + outside-a-repo) -->
- **plan-status-lifecycle:** draft → ready → active → done (D8)   <!-- plans are one entity with a status; draft/epic are CLI aliases (D9) -->
- **ship-plan:** phases A→B→C→D built + tested in order, shipped as ONE 0.21.0 drop (D7)
