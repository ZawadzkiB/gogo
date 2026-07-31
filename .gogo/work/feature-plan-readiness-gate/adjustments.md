# Adjustments — feature `plan-readiness-gate`

Log of changes / clarifications requested during planning (and, later, at the UAT
gate). Each entry: date · what changed · why. The plan above is kept current; this
is the running history.

## 2026-07-29 — scope: fold in the "building item sits in the plan column" case (Slice B)

Mid-plan, the coordinator supplied a second live sighting and asked for it to be folded in
rather than planned separately: the board's **plan** column showed `catalogue-ingestion
● dotai` at `plan-accepted` while a Claude session was actively editing `messages/pl.ts`,
writing component tests and running `npm run test:component`; the header read
`in progress 0` and `● 2 session`.

**Verified, and the unification holds.** Both sightings are the same defect —
**`state.md` records phase COMPLETION, not phase OCCUPANCY**, so the board can only
describe where work *has been*, never where it *is*:

- Sighting 1: the boundary write lands **too early relative to its own output**
  (`awaiting-plan-acceptance` before `plan.md` exists).
- Sighting 2: it lands **too late relative to the work it names** — `gogo-implement` writes
  `status: implementing` at **§④**, *after* §② does all the work and §③ validates out
  (same in `gogo-review/SKILL.md:96` and `gogo-test/SKILL.md:100`).

**Plan delta:**
- Restructured `plan.md` around the one root cause, with the FRs split into **Slice A**
  (plan readiness, FR1-FR8) and **Slice B** (phase occupancy, FR11-FR15), plus the
  cross-cutting FR9/FR10.
- Added **FR11** (phases write occupancy at entry), **FR12** (the cap counts a live `go`
  session rather than the file-derived class), **FR13** (`launch.SessionAction()`),
  **FR14** (`● building` chip + `activeAgent` from the session + a `gogo status` session
  marker), **FR15** (`· stalled` cue).
- Added decisions **D4** (where the Slice-B fix belongs — writer vs classifier vs display),
  **D5** (drop the cap's `Class` filter?), **D6** (cue vs column override).
- Added `charts/sequence.mmd` and `charts/before/sequence.mmd`; extended
  `charts/flow.mmd` and `charts/state.mmd`.

**Severity note added:** `orchestrator/cap.go:37` requires `Class == ClassInProgress`, so
an item building under a stale `plan-accepted` is **not counted** against its source cap —
a second build can start in the same repo and clobber the working tree, and the
per-**slug** owner lock does not cover it. That is the severity argument for Slice B, not
the cosmetics.

**Correction recorded:** the coordinator's note that "`launch.SessionMatchesSlug` already
discriminates by action … so the action is available" is only half right. The naming
convention encodes the action and the parse correctly lives in `launch`, but the function
collapses it to a bare `bool` (`launch.go:481-495`). Hence FR13 adds `SessionAction()` and
refactors `SessionMatchesSlug` to delegate, so there stays exactly one parser (TEST-005).

## 2026-07-30 — re-verification against HEAD (0.28.0, `40c70a1`) — not a re-plan

0.28.0 shipped while this plan sat at the acceptance gate, touching `launch.go` (+349/-21),
`plans_tab.go`, `update.go` and `move.go`. Re-checked every premise against HEAD. Baseline:
`go build ./...` clean; `contract` / `orchestrator` / `launch` tests green.

**Survived unchanged — the plan's core is intact.** The files this plan depends on were not
touched by 0.28.0: `orchestrator/cap.go` (last `78691bc`), `contract/contract.go`
(`c084960`), `contract/state.go` (`78691bc`), `cli/status.go` (`621c6dd`),
`skills/gogo-accept/SKILL.md` (`621c6dd`), `skills/gogo-implement/SKILL.md` (`a377a2f`),
`templates/state.template.md` (`78691bc`). Verified still true at HEAD: `classify()` has no
`plan.md` check and falls to `default: ClassUnfinished`; `cap.go:37` still carries
`f.Class != contract.ClassInProgress`; `gogo-implement` still writes `state.md` only at §④;
`cli/status.go` still has **zero** `ListSessions()` calls; `SessionAction()` still absent;
the `awaiting-plan-acceptance → ActionAccept` branch still present. **D2 re-measured** across
the 28 work items now on disk (up from 25): all 28 have a `plan.md`, minimum still **5,494
bytes**, minimum `## ` count still **8** — the ≥2 threshold's 4× margin holds.

**Three premises moved; re-derived, no decision reversed.**
1. **FR13** — `SessionMatchesSlug` now loops **six** actions (`ActionAuthor`, `ActionResume`
   added) and **two** label transforms (48-bounded `sanitizeLabel` + the pre-0.28.0
   `unboundedLabel`, kept for upgrade back-compat). Re-derived the design and answered the
   ambiguity question: the `Action` return is **unambiguous** because the match is on
   `"gogo-" + action + "-"` and **no action name contains a `-`**; every genuine ambiguity
   (48-char prefix collisions, a label ending in digits vs the `-N` suffix) lives in the
   *label* and leaves the action identical. Added a **structural guard test** that no
   `Action` constant may contain a `-`, plus **FR13a** recording the plan-title/slug
   attribution gap 0.28.0 carried forward (it gates D3-B, and does **not** affect FR12,
   since a build session is minted from the real slug).
2. **FR4/FR8 insertion point** — `attemptAction` is now a thin wrapper over
   `attemptActionForce(ship, force)`, with a new **`M`** force key. Added **FR4a**: `force`
   skips only the cap, so a missing-`plan.md` refusal is a *legality* rule and must sit
   outside the `!force` guards. `M` must never force an acceptance for a nonexistent plan.
3. **FR14** — 0.28.0 added a real status-severity system (`setStatus` / `statusBlocked` /
   `statusFailed`, reset at `update.go:178`, rendered by `renderStatus`) plus a
   **Diagnosability** NFR. Added **FR14a** (route every new message through those helpers;
   refusals are `statusBlocked` and name their unblock) and **FR14b** (severity must survive
   a colourless terminal → glyph **and** word, assertable in `View()`; a refusal names its
   number).

**Two decisions came out better founded (recommendations unchanged).**
- **D3** — 0.28.0 wrote down a confirm-default convention: forward moves seed `confirm: true`
  so a bare **Enter submits**. That *strengthens* A (bounce): opening no form means the
  convention cannot fire, whereas B (resume-authoring) would launch on a stray Enter, onto a
  folder a live analyst may be mid-write on. B is now **more** dangerous and must be gated on
  FR13a being fixed first.
- **D5** — the repo now argues this side itself: `SessionMatchesSlug`'s new doc comment states
  the cap under-count as a working-tree-clobber safety risk, and `cap.go`'s own comment
  already says the rule is *"a LIVE build session"* — which the `Class` filter contradicts.
  FR12 is reframed from "change the rule" to "remove a contradiction". Added **FR12a**:
  0.28.0 wrote the old rule into the user-visible bounce string (`move.go:175`), so that
  sentence must move with the rule.

**New motivating evidence folded in (cited in `plan.md`).**
- 0.28.0's shipped report
  (`.gogo/changelog/2026-07-29-plans-tab-launch-diagnostics-and-view/report.md:100-102`)
  carries the limitation *"`state.md` still narrates the past, not the present — it is
  written at each phase's exit, so a work item mid-build reads as its previous state … planned
  separately as `feature-plan-readiness-gate`"* — an independent phase ⑤ confirming this
  plan's thesis and naming this work item as the fix.
- Observed during 0.28.0's own phase ⑤: `state.md` was **stale on entry**, reading
  `phase: implement` / `status: implementing` / `test=1` after implementation *and* testing
  had finished, with a `resume:` line still warning about a `test/result.json` that test
  round 2 had already corrected to `open_issues: 0`. The file described a phase that had
  ended two phases earlier, and its stale hint actively misdirected the next reader.

**Naming collision flagged:** 0.28.0 defers its *own* **D5** — the cross-repo same-slug cap
**OVER**-count. This plan's D5 is the **UNDER**-count. Same function, opposite directions,
complementary; noted in `decisions.md` and in Out-of-scope so review cannot conflate them.

**Also updated:** the target release is now **0.29.0** (0.28.0 took the number this plan
originally claimed). Tests section gained a subsection for 0.28.0's new standards — the
compile-check-first mutation sweep with reported counts, assert-the-exact-reason,
unescapable structural guards, review checks #10/#11, and the Diagnosability bar.

`state.md` stays at `awaiting-plan-acceptance`; the orchestrator owns the gate.

## 2026-07-30 — implement ② and three review fix rounds (0.29.0)

Log of what moved during the build, beyond the accepted plan. The plan itself was not
re-planned; every entry below is either a plan correction recorded in place or a review
finding fixed.

**Build-time plan corrections** (recorded in `plan.md`'s Changes-checklist note): the
`docs/cli-contract.md` note ships as `### Changed in 0.29.0`, not `0.28.0`;
`.gogo/knowledge/coding-rules.md` was deliberately left to phase ⑤ per the checklist's own
parenthetical; the doc sweep was widened past the plan's hand-list to every `docs/*.md` plus
`skills/gogo-cli` and `skills/gogo-status`, which found two further copies of the cap's rule
sentence; and one un-planned fix inside FR4/FR14b's intent - the footer key-chip advertised
`[m] accept` on a card whose `m` bounces.

**Review round 01 (1 blocker, 2 majors, 5 minors - all fixed).** The blocker was a real CI
break: three new `cli` tests needed `claude` on PATH, so the release gate would have failed on
a clean runner. The hermetic run (`env PATH="$(dirname $(which go)):/usr/bin:/bin" go test
./...`) became a permanent fourth gate. The cap's rule sentence became one shared constant
(`orchestrator.CapRuleClause`) after three of its four user-visible copies were found stale,
and FR8's refusal was extended to the reload auto-pickup - the one launch path with no human
in the loop.

**Review round 02 (1 major, 5 minors, 1 nit - all fixed).** The major was a remedy that could
destroy work: the cap bounce recommended a bare `gogo sweep`, which on a multi-source host
reaps another source's in-flight build as an "orphan". Both cap refusals now name the TARGETED
`gogo sweep <slug>` through one shared producer. The `state lags` detector gained a second arm
(an entry event naming a different phase than the phase line) after review showed the first arm
missed the pipeline's most common re-entry, ③→②.

**Review round 03 (5 minors/nits - all fixed).** APPROVE with follow-ups: two guard wirings
pinned, the detector's justification comment corrected (the decision was right, the stated
reason was not), README synced to the two-arm rule, the empty-remedy contract made true rather
than documented down, and the FR11 evidence count updated to n=3.

**Two things carried to phase ⑤ rather than done here:**
1. `.gogo/knowledge/coding-rules.md` - the TEST-004 sanctioned exception (*a presence check may
   only ever REFUSE, never PROMOTE, and only on a MONOTONIC artifact*) and the new rule *a phase
   writes its occupancy status at entry, not its completion status at exit*.
2. The report's limitations section must carry FR11's **n=3** record verbatim, including that
   this feature's own card sat in the detector's shape throughout round 03.

**One thing recorded as out of scope** (in `plan.md`): hardening bare `gogo sweep` itself. The
reachable defect - this feature recommending it - is fixed at the root; making the command
confirm needs its own decision about non-TTY callers, and the deeper repair is that the bare
sweep judges host-wide sessions against one repo's feature list, so a confirmation would only
paper over a misclassification.

## 2026-07-30 — phase ④ finding TEST-001 folded in on the user's fix-now call

Phase ④ passed all six hands-on scenarios and raised one new finding. **The user decided to fix
it in this release rather than defer it**, and the reasoning is worth recording because it is a
scope judgement, not a defect judgement.

**The finding.** `parseStateFile` matched `- **key:** value` on any line and `stripComment` only
ever removed a comment that opened on the SAME line, so it had no notion of a multi-line
`<!-- … -->` block. The shipped template's optional-`correlation` legend wraps an EXAMPLE
`- **correlation:** [plan-XXXX]` line inside such a block; that example parsed as a real field and
`parseCorrelationList` split its prose on commas into three bogus plan ids, painting a
`⛓ ×3` chip on any item scaffolded straight from the template.

**Why it belongs in 0.29.0 rather than a follow-up.** It is the same defect family Slice A already
addresses - the template's own legend leaking into card UI - and it lives in the same function
FR6's `stripPlaceholder` extends, three lines away. It is triggered by *precisely* the fixture
this plan's own Slice-A scenarios prescribe ("scaffold a work item from
`templates/state.template.md` with NO plan.md"), so shipping Slice A while leaving this would mean
shipping a release whose own prescribed test fixture renders a false claim. It also explains an
artifact from the original bug report - a card showing `x3` - which an earlier analysis
mis-attributed to the header's `⏸ 3 need you` pill. In-family, not scope creep.

**What shipped.** `contract.advanceComment` carries an "inside an unclosed `<!--`" flag across
lines; `parseStateFile` skips any line that starts inside a block. Single-line behaviour is
byte-for-byte unchanged. Every edge case is decided in the "may only ever make a value MISSING,
never wrong" direction, including an unterminated opener, which comments out the rest of the file
because that is what a markdown renderer shows - and which is visible rather than silent, since
the card then reads malformed/authoring instead of plausibly wrong.

**One thing this round taught us.** The guards read `templates/state.template.md` *itself* rather
than a copy, and that paid off within minutes: the first draft of the template's new warning note
contained a literal comment closer, which ended the legend block early and reopened the exact
defect. The test failed immediately. Both the template and the parser's doc comment now record
that a closer appearing in prose inside a block ends it - that is what the file means, not a
parser bug to route around - and the template warns against writing comment tokens inside a
legend at all.

`test/result.json` is deliberately untouched: it is phase ④'s artifact, and the tester re-emits it
after re-verifying.

## 2026-07-30 — phase ⑤ found an FR11 regression; user's call: restore the exit write

**What ⑤ found.** FR11 moved each phase's `state.md` occupancy write to phase **entry** - and, as
an unintended consequence, **removed the exit write** that `gogo-implement|review|test` §④ had
performed since before 0.28.0. Verified against git:

    0.28.0 (a377a2f)  gogo-review §④: "Update state.md: phase=review, status=reviewing, bump iterations"
    0.29.0 (HEAD)     gogo-review §④: "bump iterations: review=<n+1> ... and leave phase/status"

The theory was that the entry write superseded the exit write. It does not: the entry write is
**prose an LLM follows**, and it has been skipped on all three of its live runs. With only that
half, `state.md` stopped advancing at all - it stuck at whatever phase last actually wrote it.
The file went from **reliably one phase behind** (0.28.0) to **arbitrarily stale** (0.29.0 as
built), which is a net regression on the exact complaint this plan set out to fix, only partly
masked by the new `· state lags` cue. Proof on this feature's own disk: it read
`implement`/`implementing` with `review=3 · test=1` on entry to ⑤.

**This was not a designed part of FR11.** No FR called for dropping the exit write; the plan
argued only for *adding* an entry write. The removal rode along with it.

**The user's decision: restore the exit write AND keep the entry write - belt and braces.**
Restored `phase`/`status` in §④ of `gogo-implement`, `gogo-review`, `gogo-test`, and in
`skills/gogo/SKILL.md`'s in-context ② path; §①b/step 1 is untouched and the `iterations` bump
stays where it now lives. Each §④ now states explicitly that **the redundancy IS the design** and
must not be tidied away, because one of the two writers is an LLM following prose.

**Interaction checked before finishing, because this is where the fix could have made the release
worse.** With the exit write back, arm A of the `state lags` detector (`phase-done` for the phase
`state.md` names + a live build session) is now reached at **every healthy hand-off**, for the gap
between one phase's exit write and the next phase's entry write - seconds to about a minute, since
that gap contains the next phase's validate-in and, for ③/④, spawning a subagent. Assessment: it
is not a false positive in the strict sense (the named phase HAS ended and nothing has claimed the
next), it self-clears on the next write exactly as the `● building` chip does over the
launch-to-first-write window, and the SAME shape stays lit for a whole phase when the entry write
is genuinely skipped - which is the difference a user actually sees. Arm A is therefore weaker
evidence than arm B (which is an active disagreement between the two files), and that is now
written down. Both shapes are pinned in `TestPhaseLineLagsCue` so the trade-off is deliberate
rather than inherited, with a note not to delete arm A to silence the blink - that would blind the
detector to the n=3 failure it exists for. **Flagged for ③/④ as the judgement call in this change.**

**Also swept (⑤ findings 2/3):** three shipped-prose sites still carried FR11's old skip count -
`skills/gogo-review/SKILL.md`, `skills/gogo-test/SKILL.md` ("its very first live run") and
`cli/internal/tui/model.go` ("the first TWO live runs"). Each was re-checked for TRUTH rather than
renumbered: the old sentences carried an implied causal claim - *an instruction outside the
numbered steps is one that gets skipped* - which the evidence falsifies, since it was skipped twice
more after being moved INTO the numbered steps. They now say the move helped less than hoped and
point at the exit write as the reason §④ does not trust the entry write alone.

**Two follow-on corrections this fix forced, and one carried to ⑤.**

⑤ had already run and written the two deferred knowledge items, so restoring the exit write made
one of them false: `.gogo/knowledge/coding-rules.md` stated *"A phase writes its OCCUPANCY status
at entry, NOT its COMPLETION status at exit"*. That is always-read context for every future
worker, so it was corrected in place to *"at entry AND again at exit"*, with the reason the
redundancy exists. `docs/cli-contract.md`'s heading carried the same "not at exit" claim and was
corrected too.

**Carried to ⑤ (do not let this ship as-is):** `report/report.md` still describes the delivered
knowledge rule as *"a phase writes its occupancy status at ENTRY, not its completion status at
exit"*. The report is ⑤'s artifact and it re-reconciles after ③/④, so it was deliberately left
alone here - but that line is now wrong and must move with the rule.

**`state.md` was moved OFF `awaiting-uat` on purpose.** ⑤ had parked it at the UAT gate with a
report bundle; re-entering ② means the bundle predates the change, so leaving `awaiting-uat`
would have let `/gogo:done` ship work that has not been reviewed or tested since it landed -
exactly the class of lie this feature exists to remove, and exactly what the restored §④ rule
says to prevent. Now `implement`/`implementing` with `implement=6`, resume pointing at ③.

## 2026-07-30 — review round 04: knowledge sweep completed, exit write scoped, blink accepted

**The blink was ACCEPTED, with a better argument than mine.** Review kept arm A of the
`· state lags` detector and supplied the reason I had missed: *before* the exit write was
restored, arm A was **silent exactly when the file was worst** - a skipped entry write left
`state.md` naming an earlier phase, so `e.Phase == line` never held and nothing fired. §④'s
write is what makes arm A's precondition hold at all, so restoring it made that arm **more**
useful, not less. It also constructed and rejected the one feasible narrowing (a
`time.Since(e.TS) > grace` guard): a wall clock in a pure file-derived predicate breaks the
read-path determinism NFR, no constant is safe across unknown hosts and subagent spawn times,
and the board re-renders on **fsnotify, not a timer**, so a grace would only change *when* the
cue appears unless a ticker were added to drive a cosmetic cue. All of that now lives in
`phaseLineLags`'s comment.

**REV-021 (major) - the knowledge sweep was incomplete, and one site made the regression
self-reverting.** Three always-read statements still described the removed exit write. The
dangerous one was `code-review-standards.md` check #13: *"flag a phase skill whose
`phase`/`status` write sits after the work"* - which instructs the next fresh-eyes reviewer to
flag the restored write as a defect. That is the precise mechanism by which this regression
comes back, and it would have looked like good-faith review. All three fixed
(`code-review-standards.md`, `non-functional-requirements.md`, `project-knowledge.md` - the last
sitting *inside* the 0.29.0 entry, so it described the release as shipping the regression), and
re-grepped for a fourth site by pattern rather than memory.

**Then made deterministic, because this rule has now failed twice as prose.** Two new guards in
`cli/skills_lint_test.go`: all three phase skills must write the occupancy status at **entry and
exit** and must scope the exit write away from a gate status; and the review standards must still
carry the "do not flag the exit write as duplication" instruction and must not have reverted to
the wording that invited the removal. Both verified by reproducing the exact regressions.

**REV-022** - the restored §④ sentence was unscoped, so a decision-gate round could overwrite
`waiting-for-user` with the working status, losing the ⏸ gate count, the card stripe and the
`/gogo:go` refusal that stops an unattended rerun. Scoped in all three skills (the finding named
two) and pinned by the new guard.

**REV-023** - the hand-off blink is now explained where users read it (`skills/gogo-cli`,
`README.md`), with the actionable half: a brief cue at hand-off is expected and self-clearing;
one that stays lit for a whole phase is the real signal.

**Process note recorded from review round 04:** three production-Go changes (the block-aware
parse, `CapRefusal` + call sites, the config-tab wiring guards) landed *after* round 03's APPROVE
with no ③ pass; round 04 reviewed and mutation-tested all three and found no defects, so the gap
is closed - but the lesson stands: **a fix round after an APPROVE still needs a review pass.**

## 2026-07-30 — phase ④ round 3: decision-gate launch guard (user's fix-now call)

**TEST-002 (major) - the board offered a launch on a card paused at a decision gate.** I
reproduced it before changing anything: a `ClassInProgress` card at `waiting-for-user` returned
intent `/gogo:go demo` with an **empty** bounce, for both `m` and `M`. `WaitingForUser()` existed
and was used for **display** (the badge, the ⏸ count, the stripe) but `attemptActionForce` never
consulted it - so the only thing between a keypress and a relaunch was a STOP instruction inside
the spawned session's own prompt.

**That is why the user chose fix-now.** A gate guarded only by an instruction to the thing being
gated is not guarded, and this release's central evidence is that exactly this mechanism fails
(FR11's entry write: n=3, skipped every time). Fixed in the shape FR4a already established -
before the class switch, **outside every `!force` condition**, because a decision gate is a
legality rule and `M` overrides the cap *and only the cap*. The refusal names the open decision
**by ID** when `state.md` carries one, points at `decisions.md`, and names the legal move
(`/gogo:resume <slug>`), mirroring `cmdGo`'s `runnableHint` so the board and the headless surface
tell the same story; routed through `statusBlocked`.

**The interaction with REV-022's scoped exit write was checked in BOTH directions**, since the
coordinator rightly flagged that a gate you cannot leave is worse than one you can bypass:
- the gate **holds** - a paused card keeps `WaitingForInput`, its stripe, its ⏸ count and its card
  cue, so it is refused *and* visibly explains why (not silently unlaunchable), and §④ no longer
  overwrites the gate status, so it stays paused;
- the gate **opens** - once the status legitimately moves on (what `/gogo:resume` does), the same
  card yields `ActionGo` again. Asserted, not assumed.

**TEST-003** - the standards guard's phrase-match is now whitespace-normalised so it survives a
markdown re-wrap. Proven with a **control pair** rather than a single mutation, because a
guard-hardening revert cannot fail while the defect is absent: with the forbidden phrase
reintroduced *wrapped across a line break*, the hardened guard fails and the raw-matching guard
passes. Both files restored byte-for-byte, md5-verified.

**A process slip of my own, recorded because it is the third of its kind.** My first attempt at
that verification used an anchor written from **memory** of the file's line wrapping. It did not
match, the edit never landed, and the run reported "0 failures" - a false pass. The landed-edit
check caught it, which is precisely why that rule and the nameless-CAUGHT rule are in the
harness. Separately, I have now mis-ordered an `events.jsonl` entry timestamp three times in the
same way (guessing when the round "started" while the wall clock moved on between rounds); the
entry timestamp is now **derived from the previous terminal event** rather than guessed.

## 2026-07-31 — review rounds 05-07 and their fix rounds (recorded at ⑤; REV-028's last leg)

Written by phase ⑤'s second pass, because REV-028 asked for these sections during review and only
its `plan.md` half landed. Compact by design - the full detail is in
[review-05.md](review-05.md) / [review-06.md](review-06.md) / [review-07.md](review-07.md).

**The theme of all three rounds: the release's own defect, one level up.** Rounds 01-04 removed
"the card says one thing and the code does another" from the *display*. Rounds 05-07 found the
same shape in the **move path**, and then in the fixes for it:

- **REV-025 (major)** - the new gate bounce branched on a raw `uat` **substring**, so an ordinary
  decision whose text merely mentions uat was sent to `uat.md`, disagreeing with the card's own
  pill. Re-anchored on the package's existing `isUATReplan` (a **digit** after "uat", which is
  what the orchestrator writes), so bounce and pill now agree **by construction**.
- **REV-026 (major)** - the `RunnableStatus` guard had been added to the `ClassUnfinished` arm
  only; `ClassInProgress` still returned `ActionGo` with an empty bounce for a status `gogo go`
  refuses. Duplicated into both go-producing arms - **not** hoisted above the class switch, which
  round 06 proposed and round 07 disproved by reproducing the refused ship (`ClassReadyToShip`'s
  legal status *is* `awaiting-uat`). Both copies are independently mutation-caught, so this is
  not a REV-002-shaped drift pair.
- **REV-030 (minor)** - `footerChips` was two hand-kept copies of one rule and the short one lied
  twice, including a `[m] go` chip on a card FR8 bounces. **Two of the four verified cases were
  created by that same round's fixes.** Collapsed into one `moveChip` producer walking
  `attemptActionForce`'s own precedence.
- **REV-031 (minor)** - round 06's alignment of `notRunnableBounce` to `runnableHint`'s terminal
  arm aligned it to **dead code** (`cmdGo` returns earlier for `TerminalStatus`), and told an
  **aborted** feature it had *"already shipped"*. Both surfaces now key on
  `orchestrator.TerminalStatus`, with `aborted` split out.
- **REV-033 → REV-035 (minor)** - a stale ship selection was *filtered* at the read but survived,
  so it **resurrected** when the card became ready again. Pruning it on reload closed that and
  left a residue: a transiently degraded read now **drops** a legitimate selection. Kept
  deliberately - *degrade to missing, never to wrong* - and confirmed by ④ as the safer direction.
- **REV-036 (nit)** - `uatRound` takes the first integer **anywhere** after "uat". The doc comment
  was made honest rather than the rule tightened; carried as a known limitation.
- **REV-029 (nit)** - the variant list's count was disputed across three files at once. Settled:
  `test-strategy.md` says **FOURTEEN**, item 11 no longer duplicates item 10, and ⑤ regrouped
  11-14 under their own lead and fixed `code-review-standards.md` #12's stale "ten" reference.
- **REV-032 (minor)** - the orphaned-doc-comment family, third occurrence. ⑤ found a **fourth**
  still on disk at `move.go:192-193` and carried it as a follow-up rather than editing a product
  file.
- **REV-024** - ⑤'s own: the first report bundle described the superseded entry-only rule. Carried
  open across four rounds on purpose so it could not be lost between phases; **closed by ⑤'s
  second pass**, which regenerated the whole bundle from the reconciled plan.

**The process itself is a finding, and is recorded as one in the report.** The `gogo` skill bounds
implement↔review at ~3 rounds; this ran to **7 review rounds and 12 implement rounds**, with
several later rounds fixing defects earlier fixes introduced - including the orchestrator's own.
The rounds that ended a defect family produced a **single producer** or a **structural guard**;
the ones that produced another instance fix did not.
