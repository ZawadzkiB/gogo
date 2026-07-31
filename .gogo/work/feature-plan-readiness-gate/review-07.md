# Review round 07 — plan-readiness-gate (ships as 0.29.0)

**Track:** review (③) · **Round:** 7 · **Date:** 2026-07-31 (local; 2026-07-30 UTC) ·
**Reviewer:** fresh eyes, did not write this code

Scope per the coordinator: **only** the eight round-06 findings, all fixed IN-CONTEXT by the
orchestrator (REV-025/026/028/029/030/031/032/033), plus REV-034 (the `state.md` resume line,
resolved in passing) and REV-024 recorded-not-fixed. Rounds 01-05's other findings were not
re-reviewed. The author asked to be reviewed at least as hard as the developer; that is what
follows — including a re-derivation of everything they scored, because the brief says their
scores have been wrong before.

**Method.** Every mutation, injection and reproduction ran in a **sandbox copy**
(`/tmp/gogo-rev07`, `cp -a`), never in the repo — this reviewer has no Edit tool by design and
touched no product file (`diff -rq` against the repo tree is clean; `git status` still 75
entries, `git stash list` empty). Every mutation was `gofmt`/`go vet`-checked **before**
running (harness rule 1), used `-count=1`, and the sandbox was restored and re-verified after
each. Verification fixtures were throwaway `zz_*_test.go` files, deleted afterwards.
**Sweep: 12 mutations, all compile-checked first; 9 CAUGHT, 3 SURVIVED — the three survivors
are reported below, with the test that should have bitten.**

## Baseline (re-derived independently, not taken on trust)

| Gate | Result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | green, **12** tested packages (+1 with no test files) |
| hermetic `go test -count=1 ./...` (`env -i`, minimal PATH, no `claude`) | green, 12 packages |
| `.claude-plugin/plugin.json` / `version_test.go` | `0.29.0` / `0.29.0` |
| em dashes on added lines (`cli/`, `.gogo/knowledge/`, new test files) | none |

Matches the coordinator's baseline exactly.

---

## 1. REV-026 — the not-runnable guard. **VERIFIED — and my round-06 fix was wrong**

The coordinator asked me to check three things. All three check out, and the first one is a
correction to my own previous round.

**(a) The placement is right, and the round-06 proposal was wrong.** I reproduced the
counter-evidence: removing both copies and hoisting one above `switch f.Class` is **CAUGHT** by
`TestAttemptActionGuards` (tui_test.go:165) and
`TestForceMoveClaimsOverrideOnlyWhenItOverrode/ready_card_focused,_no_selection`
(plans_view_test.go:851) — `ClassReadyToShip`'s legal status **is** `awaiting-uat`, so an
above-switch `RunnableStatus` check refuses a ship. The suite is right and round 06's proposed
fix was wrong. Duplicating into the two go-producing arms is the correct shape.

**(b) No third arm can produce `ActionGo` unguarded.** The switch is exhaustive: two
`intentFor(launch.ActionGo, …)` sites (move.go:131, :147), both now guarded; `ClassReadyToShip`
→ done; `ClassShipped` → bounce; fall-through → "no legal move". The one other unattended
producer, `autoPickupCmds` (pickup.go:113), is gated by `autoPickupReady` requiring
`f.Status == "plan-accepted"` **exactly** (pickup.go:37) — a runnable status — plus
`!f.PlanUnwritten`.

**(c) The duplication is NOT a REV-002/012/016/031-shaped drift pair.** Both copies are
*independently mutation-caught*, so a divergence fails the suite:

| Mutation | Result |
|---|---|
| guard removed from `ClassUnfinished` (move.go:125) | **CAUGHT** — 2 subtests, each `returned go with an EMPTY bounce` |
| guard removed from `ClassInProgress` (move.go:141) | **CAUGHT** — `TestNotRunnableStatusIsRefusedByName/…LAGGING…` **and** `TestFooterChipMatchesWhatMActuallyDoes/uat_gate…` |

That is the distinction from the cap-rule pairs: those were *prose* copies with one asserted
side. These are two copies of one expression, each with a test that bites. The coordinator's
score reproduces exactly.

**Re-derived through the real loader**, as asked — real repo on disk, `contract.LoadRepo`, four
shapes:

| `phase:` / `status:` | class | `m` |
|---|---|---|
| `knowledge` / `awaiting-uat` | unfinished | bounce names the UAT gate |
| `review` / `awaiting-uat` (lagging line) | in-progress | same bounce — **the round-06 hole, closed** |
| `test` / `awaiting-plan-acceptance` | in-progress | `accept`, no bounce — **accept route survives**, and FR4a still precedes it (move.go:101) |
| `report` / `awaiting-uat` + `report/report.md` | ready-to-ship | `done`, isShip=true — **the ship survives** |

## 2. REV-025 — the gate bounce's artifact. **VERIFIED**

**The predicate genuinely cannot disagree with the pill.** `decisionGateBounce` is reached only
under `if f.WaitingForUser()` (move.go:93); `pillLabel`'s `case "waiting-for-user"`
(model.go:1298) calls the same `isUATReplan`; and `WaitingForUser()` is exactly
`Status == "waiting-for-user"` (contract.go:119). One predicate, one status, two readers — they
agree by construction, not by convention.

**`uatRound`'s digit rule matches what the orchestrator writes** — `skills/gogo/SKILL.md:265`
writes `open-decision: UAT round N`, and `stripComment`/`stripPlaceholder` (state.go:183/175)
strip the trailing HTML comment, so the parser sees a clean value. MUTATION back to the
substring heuristic is **CAUGHT** by all three assertions of the
`a_decision_that_merely_mentions_uat` subtest, including the pill-agreement one.

One residual, filed as **REV-036** (nit): the rule is *looser* than the contract it is anchored
on — the first integer **anywhere** after "uat", so `D4 - uat asked for a v2 header` still reads
as round 2. Not a regression (the pill has always done this, so bounce and pill still agree) and
it needs a digit inside a decision title that also says "uat", hence a nit.

## 3. REV-033 — the resurrecting selection. **VERIFIED for the resurrect; one new residual**

`pruneSelection` runs at the top of `rebuild()` (model.go:675) and `rebuild()` runs from
`reload()` (model.go:724), so it fires on every reload. MUTATION removing the call is **CAUGHT**
by all three assertions of `TestStaleSelectionDoesNotResurrect`, including `m shipped [shipme]
from a resurrected selection`. The resurrect path is closed.

**But the prune keys on ABSENCE, and the reader is deliberately lenient** — that is the
question the coordinator asked me to check, and the answer is yes, it can drop a real selection.
`LoadRepo` swallows the `os.ReadDir` error (contract.go:260); `LoadProject` skips an unreadable
source (contract.go:287); `parseStateFile` returns an empty Feature for a truncated file
(state.go:23). Verified **through the real loader on a real repo**:

```
select the ready card              -> selectedFeatures = 1
state.md rewritten TRUNCATED       -> reload: class="unfinished", m.selected = map[]
state.md restored (card is ready)  -> reload: selectedFeatures = 0     <- gone for good
```

Filed as **REV-035** (minor). Note the obvious narrowing does **not** work — I tried "prune only
keys that are present-but-not-selectable" and the mid-write case still drops, because a
truncated `state.md` leaves the feature *present* and classified `unfinished`. Honest
reachability: the 300 ms debounce (watch.go:14) makes the mid-write window narrow; a slow mount,
a `git checkout` that swaps `.gogo/work/`, or a source toggled in the config tab are wider.
Harm is bounded and semi-visible (the ✓ vanishes), so: minor, not major. It does violate the
project's own reader principle — `non-functional-requirements.md:47`, *"prefer degrading to
MISSING over degrading to WRONG"* — this is the first place a degraded read causes a permanent,
silent mutation of user intent.

## 4. REV-030 — the footer chips. **PARTIALLY FIXED — reopened**

I swept the chip against `attemptActionForce` over the **full cross product** — 4 classes × 12
statuses × `PlanUnwritten` true/false = **96 shapes** — rather than the shapes the coordinator
enumerated. Exactly two disagreements, both on `footerChips`' `ClassInProgress` arm
(view.go:843-851), which is a **3-case copy of the `ClassUnfinished` arm's 5 cases**:

| shape | chip | `m` actually | |
|---|---|---|---|
| in-progress + `awaiting-plan-acceptance` (lagging line) | `[m] ✗ not runnable` | **`/gogo:accept`, no bounce** | **lies — refuses a legal move** |
| in-progress + `plan-accepted` + PlanUnwritten | `[m] go` | FR8 bounce: *"…plan.md is not written…"* | **lies — promises a refused move** |
| in-progress + `awaiting-uat` (lagging line) | `[m] ✗ not runnable` | UAT-gate bounce | control, correct |
| unfinished + `plan-accepted` | `[m] go` | go | control, correct |

The first shape is the *exact* card the author's own `TestNotRunnableStatusIsRefusedByName`
case 4 uses, and it reproduces through `contract.LoadRepo` on a real repo — so it is reachable
by the same argument that justified REV-026's second arm.

**And one of the two new arms is unpinned.** MUTATION: reverting the `ClassUnfinished` arm's new
`[m] ✗ not runnable` (view.go:837-839) back to `[m] go` leaves the **whole suite GREEN** — the
original REV-030 defect (`unfinished` + `awaiting-uat` advertising `[m] go`) can be restored with
no test failing, because the table has no unfinished+not-runnable row. That is test-strategy
variant **14** / code-review-standards **#11(b)** verbatim, in the round that *added* variant 14.
(The `ClassInProgress` arm's copy **is** pinned.) Removing either `✗ paused` case also survives —
the card falls through to `✗ not runnable`, still a refusal, and the test asserts the outcome
rather than the reason (#11: name the exact reason).

The deeper point: `footerChips` **re-decides** what `attemptActionForce` already decided. Both
guards are I/O-free (`planUnready`, `WaitingForUser`, `RunnableStatus` — no disk), so the chip
can be derived from the decision and be right by construction.

## 5. REV-031 — the two-copy prose pair. **Judgement: yes, a shared producer is warranted**

The coordinator asked for a judgement rather than a test, and the evidence is stronger than
expected. Both concrete arms **are** fixed and pinned (MUTATION removing `done` → CAUGHT;
MUTATION removing the `case ""` arm → CAUGHT, both assertions). Three things remain:

1. **The board now states a falsehood.** `notRunnableBounce` folds `aborted` into the shipped
   arm, so an aborted card reads **"x is already shipped - no move (illegal)"**. Verified:
   ```
   "aborted" -> "x is already shipped - no move (illegal)"
   ```
   Before this round the same card read *"x is aborted - not a runnable status; …"* — generic
   but **true**. The fix traded a correct message for a wrong one, and the test asserts the
   wrong string for `aborted`. Under the Diagnosability bar, misstating the state is worse than
   being generic.
2. **The arm it was aligned to is unreachable.** `runnableHint` has exactly **one** production
   call site (go.go:160), and go.go:152 returns **first** for `TerminalStatus` with a different
   sentence and **exit 0**: *"gogo go: x is done - nothing to run; reaped any tracked session."*
   So `runnableHint`'s `case "shipped","done","aborted"` is dead from the CLI's perspective, and
   the board was matched to a string `gogo go` never prints. Code-review-standards **#11(c)**:
   the test passes for a weaker reason than it claims.
3. **It is still two hand-written copies with no shared producer.**
   `TestBounceArmsMatchRunnableHint` never calls `runnableHint` — it *cannot*, `runnableHint` is
   in package `main` — so its name and doc claim a cross-surface guarantee its assertions do not
   provide. Proof the pair is unlinked: the **generic arms already disagree** — for an unknown
   status the board says *"not a runnable status; `/gogo:go` needs plan-accepted, implementing,
   reviewing or testing"*, `runnableHint`'s `default` says *"run /gogo:plan and accept a plan
   first."*

**So: a shared producer, not a test.** Fifth occurrence of the shape in one feature; the
project's own rules mandate it (`coding-rules.md:102`, `code-review-standards.md:76` #12) and the
precedent shipped in *this* release (`orchestrator.CapRuleClause` / `CapSweepRemedy` /
`CapRefusal`). `internal/orchestrator` already holds `RunnableStatus`/`TerminalStatus` and is
already imported by both `main` and `tui` — no import problem.

## 6. REV-032 — the orphaned doc comments. **HALF FIXED, HALF RE-BROKEN — third occurrence**

model.go is correct: `pruneSelection`, `selectableForShip` and `selectedFeatures` each carry
their own doc. move.go was not split — the comment was **relocated into the same defect**.
`notRunnableBounce`'s two lines now sit at the **bottom** of `decisionGateBounce`'s doc block
(move.go:186-187), so `notRunnableBounce` (move.go:156) has **no doc at all** and
`decisionGateBounce`'s godoc ends with two sentences about a different function. Confirmed with
the tool, not by eye:

```
$ go doc -all -u ./internal/tui
func decisionGateBounce(f *contract.Feature) string
    …Routed through statusBlocked by launchActionForce - nothing failed, the
    user is blocked. notRunnableBounce explains why a card whose status is not
    runnable cannot take `m`, naming the legal move. …
```

Round 06's proposed first-word doc guard was **not** added (no `go/ast` doc test exists; the
only `go/ast` test is `plans_view_test.go`'s call-site check) — which is why the same defect
landed for the third time (REV-014, REV-032, now).

## 7. REV-028 — the self-reverting plan text. **The 4th site landed GARBLED; two sub-items open**

The grep does return **0** live stale claims, as the coordinator says — but only because the
stale string was **broken apart**, not because the instruction is correct. `plan.md:689-692` now
reads:

```
>    **Carried to ⑤:** refine TEST-004 with the sanctioned exception (…) and add
>    *a phase writes its occupancy status at entry AND AGAIN at exit - two
>    status at exit*.
```

Line 691 was rewritten; line 692's dangling `status at exit*.` from the old sentence was left
behind. The instruction ⑤ follows now ends *"- two status at exit"*. Test-strategy variant **13**
("re-read the bytes you are about to mutate, and verify the edit landed") applies to a doc edit
too.

The other two sub-items are untouched: `plan.md:33`'s fold-in bullet still lists TEST-001 only,
and grepping plan.md for `TEST-002`, `TEST-003`, `selectableForShip`, `notRunnableBounce`,
`decisionGateBounce`, `pruneSelection`, `isUATReplan`, `footerChips` returns **zero** hits for
every one — so the planned-vs-shipped table ⑤ regenerates from plan.md will not mention TEST-002's
decision-gate refusal or anything from rounds 05-07. `adjustments.md` still has no round-05, -06
or -07 section (last heading: *phase ④ round 3*).

## 8. REV-029 — the FOURTEEN variants. **Heading fixed, body not; one duplicate**

- **The stale count moved one line down.** :166 says *"The FOURTEEN variants"*; :167 — the very
  next line — still says *"Two releases produced **ten distinct** ways"*. The file contradicts
  itself in adjacent sentences, which is the defect REV-029 was filed for.
- **The grouping still totals ten.** *"The four harness rules"* (:173) = items 1, 2, 9, 10;
  *"The six ways an assertion misses its target"* (:187) = items 3-8. So 11-14 (:225-237) belong
  to no group and sit **after** the section's closing *"The shape that recurs"* summary and its
  *"Corollary"*.
- **Item 11 duplicates item 10.** :183 *"Never `&&` after a pipe when the pipe's exit code is the
  result — the `&&` sees the last stage's status"* and :225 *"A shell `&&` after a pipe masks the
  exit code… `head` supplies the status"* are one rule with one remedy. So **three** of the four
  appended variants are new and the true count is thirteen.
- **12, 13 and 14 read correctly and are distinct.** 14 overlaps 8 (producer vs wiring) but adds
  *"mutate EVERY surface the fix claims to unify"*, which is the part that bites — and which
  REV-030's own new chip test then failed to apply to itself.

## 9. REV-034 / REV-024

- **REV-034 → verified.** The `resume:` line no longer records the false "known gap"; it lists
  round 06's fixes factually, and I re-derived the arm discrimination independently (§1).
- **REV-024 → open, carried.** `report/report.md:436` and `:439` both still state the superseded
  entry-only rule. ⑤'s artifact. **Note for ⑤:** the same file also says test-strategy gained
  *"The **ten variants**"*, so the disputed count now lives in a third place (see REV-029).

---

## Findings this round

| Id | Sev | Pri | Status | Finding | Fix |
|---|---|---|---|---|---|
| REV-030 | minor | P2 | **open** ↺ | `footerChips`' `ClassInProgress` arm is a 3-case copy of the 5-case `ClassUnfinished` arm: it refuses a legal accept (`in-progress` + `awaiting-plan-acceptance`) and promises `[m] go` where FR8 bounces (`in-progress` + `plan-accepted` + PlanUnwritten). Both verified over a 96-shape sweep and through the real loader. **And the `ClassUnfinished` arm's new chip is unpinned — reverting it to `[m] go` leaves the whole suite green.** | **AGENT-FIXABLE** — derive the chip from `attemptActionForce` (both are I/O-free) instead of re-deciding it; failing that, add the two missing cases. Replace the 3-row table with the full cross product, both directions |
| REV-031 | minor | P2 | **open** ↺ | Arms aligned, but: an **aborted** card now reads *"already shipped"* (a falsehood the fix introduced); `runnableHint`'s terminal arm is **dead code** (go.go:152 returns first, exit 0), so the board was matched to a string `gogo go` never prints; and the pair is still two hand-written copies whose generic arms already disagree. `TestBounceArmsMatchRunnableHint` never calls `runnableHint`. | **AGENT-FIXABLE — judgement: a shared producer IS warranted.** `orchestrator.NotRunnableReason(...)`, quoted by both; pin the wirings; split `aborted` out; fix the test's name/doc |
| REV-028 | minor | P2 | **open** | The 4th site landed **garbled**: `plan.md:691-692` now reads *"…at entry AND AGAIN at exit - two / status at exit*."*. Grep returns 0 only because the stale string was broken apart. Fold-in bullet (`:33`) and `adjustments.md` rounds 05-07 still missing; 8/8 round-05-07 symbols have **zero** hits in plan.md. | **AGENT-FIXABLE** — repair the sentence (re-read before editing), extend `:33`, add the adjustments sections |
| REV-032 | minor | P3 | **open** ↺ | The move.go half was **relocated, not split**: `notRunnableBounce` has no doc, `decisionGateBounce`'s godoc ends with two sentences about it. Confirmed via `go doc -all -u`. Third occurrence; the proposed doc guard was not added. | **AGENT-FIXABLE** — move the two lines above `func notRunnableBounce`; add the `go/ast` first-word doc guard that ends the family |
| REV-035 | minor | P3 | **new** | `pruneSelection` keys on **absence**, but the reader degrades to absence on purpose — a truncated `state.md` or an unreadable source drops a legitimate ship selection **permanently and silently**. Verified through the real loader t0→t1→t2. The obvious narrowing does not fix it (tested). | **AGENT-FIXABLE** — announce the drop in `m.status` (Diagnosability), or prune only on a POSITIVE non-ready status, never on an empty/unknown one; add a degraded-read leg to the test |
| REV-029 | nit | P3 | **open** | Heading says FOURTEEN, :167 still says *"ten distinct"*; groups still total ten so 11-14 are orphaned after the closing summary; **item 11 duplicates item 10**, so only three of the four are new. 12/13/14 read correctly and are distinct. | **AGENT-FIXABLE** — drop or justify 11, renumber, fix :167, re-group |
| REV-036 | nit | P3 | **new** | `uatRound` is looser than the contract it anchors on — first integer **anywhere** after "uat", so `D4 - uat asked for a v2 header` reads as round 2. Pre-existing in the pill, now on one more surface. Bounce/pill agreement is unaffected. | **AGENT-FIXABLE** — anchor on `^\s*uat\s+round\s+(\d+)`; add the false-positive row |
| REV-024 | minor | P3 | **open** | Carried by instruction: `report/report.md:436` **and `:439`**; ⑤ also owns the *"ten variants"* line in the same file. | **AGENT-FIXABLE at ⑤'s re-reconcile** |
| REV-025 | major | P1 | **verified** | `isUATReplan` cannot disagree with the pill (one predicate, one status, two readers); digit rule matches the contract writer; mutation-caught 3/3. | — |
| REV-026 | major | P1 | **verified** | Placement correct; the hoist really is refused by the suite (my round-06 fix was wrong); no third `ActionGo` arm; both copies independently mutation-caught, so **not** a drift pair; re-derived on 4 real-loader shapes. | — |
| REV-033 | minor | P3 | **verified** | Resurrect closed, mutation-caught 3/3 on the real `rebuild()` path. Residual filed as REV-035. | — |
| REV-034 | nit | P3 | **verified** | The false "known gap" is gone; the new resume line is factually correct. | — |

**Counts:** 0 open blockers · **0 open majors** · 5 open/new minors · 2 open/new nits ·
1 carried minor (⑤'s) · 28 verified (36 total).

## Verdict — APPROVE

**Both round-06 majors are genuinely closed, and I checked them harder than the rest because
one of them was my own mistake.** REV-026's shipped placement is correct and my round-06
proposal was wrong for exactly the reason the author gave — I reproduced the refused ship. The
duplication is not the drift shape we keep finding: unlike the cap-rule pairs, **both** copies
are independently mutation-caught, so a divergence fails the suite. REV-025's predicate agrees
with the pill by construction, not by convention, and its digit rule matches what the
orchestrator actually writes. REV-033's resurrect is closed on the real reload path. Four gates
green, 12 packages, hermetic included, 0.29.0 everywhere.

Nothing open is a blocker or a major, so this **advances to ④** rather than looping back. But
three of the eight fixes did not fully land, and the pattern in them is worth naming before ⑤
writes the report:

- **REV-030 and REV-032 are the same failure the round was fixing, one level over.** REV-030
  brought `attemptActionForce`'s two arms into agreement and then left `footerChips`' two arms
  disagreeing — including one shape where the chip refuses a move that is legal. REV-032 moved a
  doc comment out of one wrong place into another. Both are cheap; both keep recurring because
  the fix was applied to the instance instead of the shape.
- **REV-030 also shipped an unpinned arm in the round that added test-strategy variant 14** —
  "mutate EVERY surface the fix claims to unify". Reverting the unfinished arm's chip restores
  the original defect with the whole suite green. Worth applying the new variant to the change
  that introduced it.
- **REV-031 is the one to fix properly rather than again.** Judgement as asked: a shared
  producer, not another test. The pair's generic arms already disagree, the arm it was aligned
  to is dead code, and the fix put a false statement ("aborted → already shipped") in front of
  the user. `internal/orchestrator` is already the right home and this release already set the
  precedent three times.

REV-024 stays ⑤'s. REV-028 and REV-029 are ⑤-adjacent doc repairs and should ride into the
report pass. ④ takes its final hands-on pass; ⑤ re-reconciles.
