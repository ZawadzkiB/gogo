# State — feature `board-session-picker`

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

- **feature:** Per-session attach/kill pickers + changelog live-session dot (TUI cockpit)
- **phase:** done            <!-- plan | implement | review | test | knowledge | done -->
- **status:** shipped    <!-- awaiting-plan-acceptance | plan-accepted | implementing | reviewing | testing | waiting-for-user | awaiting-uat | done | shipped | aborted -->
- **created:** 2026-07-14
- **accepted:** 2026-07-14 (user, via /gogo:accept)
- **completed:** 2026-07-15
- **branch:** main
- **iterations:** plan=0 · implement=2 · review=1 · test=1   <!-- add · uat=N once a UAT round loops back to planning -->
- **resume:** none — shipped to .gogo/changelog/2026-07-15-board-session-picker/   <!-- <phase to re-enter> — <next action> | none -->

<!-- review r1: gogo-reviewer verdict APPROVE (0 blockers/majors/minors, 1 nit REV-001 fixed in-context implement r2, test-only). Gates green. -->
<!-- REV-001 fixed: added board-origin attach-picker cancel + ready-ship selection-preservation test. -->
<!-- test r1: gogo-tester green - unit -race suite + live tmux TUI drive of FR-1/FR-2/FR-3; TEST-005 held; fixtures cleaned up. -->
<!-- report ⑤: report/ bundle written (report.md + flow.mmd as-built + before/ compare + diagrams.html + manifest + result.json); project-knowledge.md 0.20.0 note extended. -->

- **open-decision:** none    <!-- <decisions.md anchor> | none -->
