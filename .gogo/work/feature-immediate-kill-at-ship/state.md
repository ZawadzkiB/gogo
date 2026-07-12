# State — feature `immediate-kill-at-ship`

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

- **feature:** immediate kill-at-ship — /gogo:done reaps its driving session at ship + board Launch() stops leaking (D5=B)
- **phase:** done            <!-- plan | implement | review | test | knowledge | done -->
- **status:** awaiting-uat    <!-- awaiting-plan-acceptance | plan-accepted | implementing | reviewing | testing | waiting-for-user | awaiting-uat | done | shipped | aborted -->
- **created:** 2026-07-12
- **completed:** 2026-07-12
- **accepted:** 2026-07-12 (user; D1-D3 accepted at plan; D4→B accepted 2026-07-12: targeted ship-reap supersedes D1=A)
- **branch:** n/a
- **iterations:** plan=1 · implement=3 · review=2 · test=1   <!-- add · uat=N once a UAT round loops back to planning -->
- **resume:** awaiting UAT — verify the work; `/gogo:done immediate-kill-at-ship` accepts (ships to changelog), or describe issues to loop back into planning (same item, reruns ②→⑤). Report: report/report.md · report/diagrams.html.
- **open-decision:** none    <!-- D1-D3 at plan; D4→B resolved 2026-07-12 -->
