# State — feature `project-scope-and-gates`

<!-- Files in this folder (.gogo/work/feature-<slug>/):
  - plan.md        — the accepted plan (the contract) + the feature's functional requirements
  - adjustments.md — log of changes / clarifications you asked for during planning
  - state.md       — THIS file: current phase / status / iterations; lets work resume
  - decisions.md   — forks that needed your call + gogo's recommendation + your answer
  - charts/        — mermaid diagrams (.mmd) + charts/before/ (the plan-time as-is baseline) + manifest.json + an offline diagrams.html viewer
  UAT gate (from 0.11.0): phase ⑤ ends at status `awaiting-uat`, not `done`. Running `/gogo:done` IS the acceptance.
-->

- **feature:** project model + project-UAT gate + per-source gate-skip flags
- **phase:** done       <!-- plan | implement | review | test | knowledge | done -->
- **status:** shipped   <!-- /gogo:done accepted + shipped as 0.24.0 (2026-07-20) -->
- **created:** 2026-07-20
- **accepted:** 2026-07-20
- **completed:** 2026-07-20
- **branch:** main
- **iterations:** plan=0 · implement=2 · review=0 · test=0
- **resume:** pass 2 done — FR3 (project-UAT: plans-tab `D` + derived render) + FR4 (skip-flag UI + the gogo-plan/gogo/gogo-done skill honoring of `--skip-acceptance`/`--skip-uat`) + version→0.24.0 + docs; tree builds + `go test -race ./...` green → ready for review
- **open-decision:** none
