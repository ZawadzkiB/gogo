# State — feature `cockpit-redesign`

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

- **feature:** Restyle the terminal cockpit TUI (cli/internal/tui/) to the Claude-Design 1b + 1c mockup
- **phase:** done            <!-- plan | implement | review | test | knowledge | done -->
- **status:** shipped   <!-- awaiting-plan-acceptance | plan-accepted | implementing | reviewing | testing | waiting-for-user | awaiting-uat | done | shipped | aborted -->
- **created:** 2026-07-12
- **accepted:** 2026-07-12 (user; D1→B one feature 1b+1c together @0.18.0, D2→A both dots+bars via shared phaseProgress, D3→A strip is a shortcut + graceful degrade)
- **branch:** main
- **iterations:** plan=1 · implement=3 · review=1 · test=1   <!-- add · uat=N once a UAT round loops back to planning -->
- **resume:** none — shipped to .gogo/changelog/2026-07-12-cockpit-redesign/
- **open-decision:** none     <!-- D1–D3 resolved 2026-07-12 -->

### Implement notes (② round 1, in-context)
- **styles.go:** new tokens (secondaryText, faintText, pendingDot, tinted pill bgs, stripBg) + precomputed pill/phase-dot/segment styles + `gateBorder` (heavy `┃` left) + `stripBoxStyle`.
- **model.go:** the shared `phaseProgress(f) [5]phaseState` vector (D2) with `phaseDots`/`phaseDotsPlain`/`phaseBar` renderers; `pillLabel`/`pillStyleFor` (badge() stays canonical); `stripeAccent` (FR-5); `gates()`/`gateFor` enumerator (FR-8); `isChangelogCol`; `showAllKeys` field.
- **view.go:** FR-1 header attention summary, FR-8 `renderNeedsYouStrip` + D3 degradation, FR-2 underlined column headers, FR-3/4/5 richer `renderCard` (pill + dots + heavy-`┃` gate stripe), FR-6 `renderChangelogColumn` collapsed list, FR-7 `contextualFooter`/`footerChips`; removed `cardBadgeText`/`badgeStyleFor` (superseded by pillLabel).
- **update.go:** FR-10 number keys (`jumpToGate` → focus + `quickView` read plan/report) + `?` full-help toggle.
- **window.go:** `colAvail` subtracts the strip height (D3); `cardHeights` unit rows for the collapsed changelog.
- **Decision (within D2→A):** cards show compact dots (dense board); the segmented bar renders on the roomy needs-you strip gate rows ("bars where space allows"). Number key = "read plan/report" via quickView (matches the mockup `[1] read …` labels; FR-10's "route its primary action").
- **Tests:** added `redesign_test.go` (phaseProgress/pill/stripe/gates/strip/header/footer/keys); updated `TestBoardViewRenders`, `TestSessionIndicatorOnCard`, `TestWaitingCardCue`, and the window integration tests (repointed card-windowing to the plan column, added changelog-collapse coverage). `TestBadgeAwaitingPlanAcceptance`/`TestColumnSeparatorRendered` stay green.
