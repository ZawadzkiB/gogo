# State — feature `session-binding-ops`

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
  - (optional) the `correlation:` line below — the plan id(s) this work item belongs to (a LIST; stamped when spawned via `/gogo:plan --correlation plan-XXXX`; absent = the item belongs to no cross-source plan)
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

- **feature:** Session binding ops — start, kill and re-assign a work item's tmux session from the cockpit
- **phase:** done            <!-- plan | implement | review | test | knowledge | done -->
- **status:** shipped   <!-- awaiting-plan-acceptance | plan-accepted | implementing | reviewing | testing | waiting-for-user | awaiting-uat | done | shipped | aborted -->
<!-- `awaiting-plan-acceptance` is only meaningful once plan.md EXISTS and is written (>= 2 `## ` sections):
     write this file AFTER plan.md. Until then every reader shows the item as `✎ authoring`, leaves it out of
     the "need you" gate count, and `m`/`M`/`gogo go`/`/gogo:accept` all refuse it (there is nothing to accept).
     Likewise fill in every `<...>` placeholder below - a placeholder reads as an EMPTY value. -->
- **created:** 2026-07-31
- **accepted:** 2026-08-01 (user, via /gogo:accept)
- **completed:** 2026-08-01
- **branch:** main
- **iterations:** plan=0 · implement=3 · review=2 · test=1   <!-- add · uat=N once a UAT round loops back to planning -->
- **resume:** none — shipped to .gogo/changelog/2026-08-01-session-binding-ops/
- **open-decision:** none
