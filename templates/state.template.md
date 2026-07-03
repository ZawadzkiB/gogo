# State — feature `<slug>`

<!-- Files in this folder (.gogo/work/feature-<slug>/):
  - plan.md        — the accepted plan (the contract) + the feature's functional requirements
  - adjustments.md — log of changes / clarifications you asked for during planning
  - state.md       — THIS file: current phase / status / iterations; lets work resume
  - decisions.md   — forks that needed your call + gogo's recommendation + your answer
  - review/issues.json — living, typed review findings (the contract; see templates/contracts/)
  - review-NN.md   — each code-review round's rendered snapshot of issues.json
  - test/issues.json   — living, typed test findings (same contract)
  - test-NN.md     — each test round's rendered snapshot
  - events.jsonl   — append-only progress telemetry (one schema'd JSON line per phase transition; read by the gogo CLI; a missing file is never an error)
  - report/        — the as-built bundle (written at phase ⑤): report.md + the UML set (.mmd) + report/before/ (the plan-time "before" set, copied in for before/after compare) + diagrams.html + result.json
  - charts/        — mermaid diagrams (.mmd) + charts/before/ (the plan-time as-is baseline) + manifest.json + an offline diagrams.html viewer
  (and at the .gogo/ level: .gogo/changelog/<YYYY-MM-DD>-<slug>/ — the shipped report bundle archived by /gogo:done)
-->

- **feature:** <one-line title>
- **phase:** plan            <!-- plan | implement | review | test | knowledge | done -->
- **status:** awaiting-plan-acceptance   <!-- awaiting-plan-acceptance | plan-accepted | implementing | reviewing | testing | waiting-for-user | done | shipped | aborted -->
- **created:** <YYYY-MM-DD>
- **branch:** <git branch | n/a>
- **iterations:** plan=0 · implement=0 · review=0 · test=0
- **resume:** none           <!-- <phase to re-enter> — <next action> | none -->
- **open-decision:** none    <!-- <decisions.md anchor> | none -->
