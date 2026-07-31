# Review round 06 — plan-readiness-gate (ships as 0.29.0)

**Track:** review (③) · **Round:** 6 · **Date:** 2026-07-30 · **Reviewer:** fresh eyes, did not write this code

Scope per the coordinator: **only** the four round-05 minors (REV-025/026/027/028), fixed
IN-CONTEXT by the orchestrator with no spawned developer, plus REV-024 recorded-not-fixed.
Rounds 01-05's other findings were not re-reviewed. The author asked to be reviewed at least
as hard as the developer was; that is what follows.

**Method.** Every mutation, injection and reproduction below ran in a **sandbox copy**
(`/tmp/gogo-rev06`, `cp -a`), never in the repo — this reviewer has no Edit tool by design and
touched no product file (`diff -rq` against the repo tree is clean; `git status` unchanged).
Every mutation was `gofmt`/`go vet`-checked before running (test-strategy harness rule 1), used
`-count=1`, and the sandbox was restored and re-verified green after each. Independent
verification fixtures were written as throwaway `zz_*_test.go` files and deleted.

## Baseline (re-verified independently, not taken on trust)

| Gate | Result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | green, **12** tested packages (+1 with no test files) |
| hermetic `go test -count=1 ./...` (`env -i`, minimal `PATH`, no `claude`) | green |
| `gogo --version` / `plugin.json` | `gogo 0.29.0` / `"version": "0.29.0"` |
| `events.jsonl` | 24 lines, all parse, non-decreasing timestamps; newest `phase-started`/`review` |

Matches the coordinator's baseline.

---

## 1. REV-027 — the stale-selection bypass. **VERIFIED, 3/3 re-derived**

`selectableForShip` (model.go:779) is quoted by all three surfaces. I mutated each
independently, one at a time:

| Surface reverted to its own literal | Result |
|---|---|
| `selectedFeatures` (model.go:786) | **CAUGHT** — 3 assertions, incl. `stale selection shipped {done [shipme] …} with no bounce` |
| `renderCard`'s tick (view.go:613) | **CAUGHT** — `renderCard still marks a reclassified card as selected`, with the card printed |
| `toggleSelect` (update.go:305) | **CAUGHT** — `space armed a card at a decision gate` + `refused silently` |

**3/3. The extended test is complete** — the coordinator's own score reproduces exactly.

**The bypass is closed on the REAL path, not only in the fixture.** I built a repo on disk
(`t.TempDir()` + `.gogo/work/feature-shipme/` + `report/report.md`), loaded it through
`contract.LoadRepo`, selected the genuinely-ready card, rewrote `state.md` to
`waiting-for-user` + `open-decision: UAT round 1`, and called `m.reload()` — the real
`reloadMsg` path (update.go:97-119) never clears `m.selected`, so the selection did survive by
design. After the reload the card dropped out **at the READ** (`selectedFeatures` = 0) and
`m`/`M` fell through to the gate bounce. Exactly as claimed.

**No fourth surface carries its own literal.** `move.go:69` and `move.go:136` are the
*focused-card ship* route and the class switch — routing, not selection membership. Judged
acceptable, though quoting the predicate at `:69` would cost nothing.

One residual, filed as **REV-033**: the stale selection is *filtered*, not cleared, so it
**resurrects**. Verified over a full loop — ready → `waiting-for-user` (✓ gone, selected=0) →
`awaiting-uat` again → **selected=1, ✓ back**, `m` returns `/gogo:done shipme`, no keystroke
re-armed it.

## 2. REV-026 — not-runnable statuses. **HALF FIXED; the open half is a MAJOR**

**What landed works, and the coordinator's self-reported gap is FALSE.** Deleting the guard
(move.go:124-126) fails **both** subtests of `TestNotRunnableStatusIsRefusedByName` —
`awaiting-uat` *and* `shipped` — each at `returned go with an EMPTY bounce`. The
`awaiting-uat` arm discriminates. I also re-derived the original defect through the real
loader as instructed (temp repo → `LoadRepo` → `awaiting-uat` + no `report/report.md` → class
`unfinished` → `m`), for `phase: knowledge` and `phase: plan`: both bounce correctly today,
both go empty when the guard is removed. The accept route survives (`awaiting-plan-acceptance`
→ `/gogo:accept acceptme` through the real loader), and `M` cannot force past either.

**What is still open — and it is the case round 05 itself named.** `classify()` derives
`ClassInProgress` from the **`phase:` line alone** (`inProgressPhaseOrStatus`: phase ∈
{implement, review, test} **or** status ∈ {implementing, reviewing, testing}), and the
`case contract.ClassInProgress:` arm (move.go:131-135) has **no** `RunnableStatus` check. So a
stale `phase:` line — the state *this entire feature exists to describe* — reopens the hole.
Reproduced end-to-end on a real repo (`phase: review` + `status: awaiting-uat`, no report):

```
$ gogo go x
gogo go: feature "x" is "awaiting-uat" - not runnable here. it's at the UAT gate -
  run /gogo:done to ship, or give feedback to loop it back.        # exit 1

board `m` on the same card  ->  /gogo:go x, EMPTY bounce, launch confirm opens
gogo status  ->  WAIT  -  in-progress  review  awaiting-uat  x
```

Every display surface says the user is holding the card; `m` launches it (and the confirm
seeds `true`, so a bare Enter fires). That directly contradicts the fix's own comment —
*"cmdGo keys on the same predicate, so board and CLI refuse alike"* — which is false for this
arm: `cmdGo` (go.go:159) checks `RunnableStatus` **unconditionally, with no class involvement
at all**. Same hole on the accept route: `phase: test` + `awaiting-plan-acceptance` →
`/gogo:go x`, not `/gogo:accept x`. The test cannot see any of it because **both** fixtures
hard-set `Class: contract.ClassUnfinished` — the fixture weakness the coordinator suspected,
with the opposite consequence to the one they recorded (see **REV-034**).

**Severity raised to major** in place.

## 3. REV-025 — the gate bounce named the wrong artifact. **Judged: the heuristic is wrong, and unpinned**

The coordinator asked me to judge the substring test. It fails in **both** directions, and a
deterministic answer already exists in the same package.

**A false positive — new damage, the mirror image of the defect being fixed.** A *plain*
decision whose prose happens to contain "uat" is routed to the UAT gate:

| `open-decision` | `pillLabel` | bounce says | |
|---|---|---|---|
| `UAT round 1 - header wraps` | `⏸ re-planning · UAT 1` | uat.md | agree |
| `UAT feedback - the header wraps` | `⏸ decision` | **uat.md** | **disagree** |
| `D4 - uat asked for a different header` | `⏸ decision` | **uat.md** | **disagree** |
| `round 2 - the header wraps` | `⏸ decision` | decisions.md | agree (and wrong, if it *is* a UAT round) |
| `D7 - re-plan the header after user feedback` | `⏸ decision` | decisions.md | agree |

Row 3 is the sharp one: a genuine D4 whose real exit **is** `/gogo:resume` now gets *"answer it
in **uat.md**, then **re-accept the adjusted plan**"* — wrong file **and** an action that does
not apply. Row 4 is the coordinator's own worry, confirmed reachable.

**It duplicates a discriminator 550 lines away.** `uatRound` / `isUATReplan`
(model.go:1289-1320) is the package's single "is this a UAT re-plan" predicate;
`docs/cli-contract.md:260-264` makes `open-decision: UAT round N` a documented **contract** and
`skills/gogo/SKILL.md:265` is its writer. Quoting `isUATReplan(f)` kills row 3 (it requires a
digit after "uat"), makes the bounce agree with the pill **by construction**, and anchors the
branch on the contract instead of on free prose. This is precisely the shape
`code-review-standards.md` check **#12** — added in *this* release — tells a reviewer to flag.

**And the branch is entirely unpinned.** Deleting the two-line UAT branch leaves **all 12
packages green**. The only test touching `decisionGateBounce` (authoring_test.go:367) pins the
decision arm. Nothing bites if the branch is reverted, re-inlined or inverted.

**Severity raised to major** in place; REV-025 stays `open`.

## 4. REV-028 — the self-reverting plan text. **Three sites fixed, two items left**

Verified fixed: `plan.md:21-24` (records the removal-and-restoration), `:40` ("at entry AND
AGAIN at exit - two writers on purpose"), and `:805` — the Changes-checklist **instruction ⑤
follows** — which now carries the shipped rule. Those match `.gogo/knowledge/coding-rules.md:83`
byte-for-meaning.

**There IS a fourth site under `.gogo/work/**`, and it is another instruction.** `plan.md:691`,
inside the round-02 blockquote, still reads *"**Carried to ⑤:** … and add *a phase writes its
occupancy status at entry, not its completion status at exit*"*. `plan.md` now **contradicts
itself** about which rule ⑤ should write (:805 vs :691) — the same self-reverting mechanism
REV-021/028 exist for. A repo-wide grep of the exact phrasings over
`.gogo/ docs/ skills/ README.md templates/ commands/ cli/` returns only: `plan.md:315` (FR11's
**planned** heading — correct to leave; it is the as-planned contract and the as-built section
carries the delta), `plan.md:691` (this one), `report/report.md:436` (REV-024),
`adjustments.md:164`/`:272` (historical log entries — correct to leave), and the review
artifacts. So: exactly one genuine fourth site.

**The fold-in bullet was not extended.** `plan.md:33` still lists TEST-001 only. Grepping
`plan.md` for `selectableForShip`, `notRunnableBounce`, `decisionGateBounce`, `TEST-002`,
`TEST-003` returns **zero hits** — so the planned-vs-shipped table ⑤ regenerates from it will
not mention a new production function, a shared predicate, a new user-visible refusal branch or
two new tests. `adjustments.md` likewise has **no round-06 section** (rounds 01-04 each got one;
the last heading is phase ④ round 3).

## 5. REV-024 — recorded, not fixed

Confirmed unchanged: `report/report.md:436` (coding-rules row) **and `:439`** (NFR row) both
still state the superseded entry-only rule. ⑤'s artifact; carried.

## 6. The spot-checks the coordinator asked for

- **`strings` in the script-edited imports — landed cleanly.** Both `cli/internal/contract/
  authoring_test.go` (3 uses) and `cli/internal/tui/authoring_test.go` (62 uses) have correctly
  grouped, gofmt-clean import blocks; `gofmt -l .` and `go vet ./...` are clean.
- **`notRunnableBounce` vs `runnableHint` — the pair has ALREADY drifted.** `done` is terminal
  for the CLI ("it's already shipped." / reap + exit 0) but falls to the board's generic
  "not a runnable status" arm; the empty-status arm renders `x is  - not a runnable status`
  (double space, names no value). Filed as **REV-031**.
- **Em dashes.** Every line added this round uses plain `-`. The `—` occurrences near the new
  code (move.go:110/114/139, model.go:766, update.go:297/306) are all pre-existing lines.
- **Write scope.** The round is pure read-side: the new guard, `notRunnableBounce`,
  `selectableForShip` and the gate branch do no I/O; the render path still uses `planUnready`,
  so `TestFooterChipDoesNoDiskIO` holds. Nothing written outside `.gogo/`.
- **test-strategy variant 8 (a producer asserted, its call sites unguarded) — considered and
  dismissed for REV-027**: the test asserts all three *wirings* behaviourally and bites 3/3, so
  the structural half is not required here.

## 7. One finding outside the four, in the same family

**REV-030** — `footerChips` (view.go:814-839) still advertises `[m] go` on cards whose `m` now
refuses. Verified with two clean controls:

| card | chip | `m` | |
|---|---|---|---|
| unfinished + `awaiting-uat` (no report) | `[m] go` | "x is at the UAT gate…" | **lies** |
| unfinished + `done` | `[m] go` | "x is done - not a runnable status…" | **lies** |
| unfinished + `aborted` | `[m] go` | "x is aborted - no move (illegal)" | **lies** |
| in-progress + `waiting-for-user` (D3) | `[m] go` | "x is paused on D3…" | **lies** (round-05 arm) |
| in-progress + `implementing` | `[m] go` | — | control, correct |
| unfinished + `plan-accepted` | `[m] go` | — | control, correct |

The chip's own comment calls this "the same 'the board says one thing and does another' defect
in miniature". Two new refusals were added in two rounds and neither updated it.

## Findings this round

| Id | Sev | Pri | Status | Finding | Fix |
|---|---|---|---|---|---|
| REV-026 | **major** ↑ | P1 | open | The not-runnable guard is in the `ClassUnfinished` arm only. `ClassInProgress` (move.go:131-135) returns `ActionGo` unguarded, and `classify()` derives in-progress from `phase:` alone — so `phase: review` + `awaiting-uat` **launches on the board while `gogo go` refuses it (exit 1)** and `gogo status` marks it `WAIT`. Accept route has the same hole. Both test fixtures hard-set `Class`, so nothing bites. | **AGENT-FIXABLE** — hoist the check ahead of `switch f.Class` (moving the accept route above it), matrix test through the real loader |
| REV-025 | **major** ↑ | P1 | open | The UAT branch is a raw `uat` substring on free prose. Verified false positive: `D4 - uat asked for a different header` (a plain decision) → "answer it in **uat.md**, then **re-accept the adjusted plan**", while the pill on the same card says `⏸ decision`. It duplicates the contract-anchored `isUATReplan` (model.go:1318) instead of quoting it, and the whole branch **deletes green** across 12 packages. | **AGENT-FIXABLE** — `if isUATReplan(f)`, plus a pill-vs-bounce agreement guard |
| REV-028 | minor | P2 | open | Three exit-write sites corrected; **`plan.md:691` is a fourth**, and it is a "Carried to ⑤" **instruction**, so `plan.md` now contradicts itself. Fold-in bullet (`:33`) still TEST-001 only — round 06's own symbols appear nowhere in `plan.md`; `adjustments.md` has no round-06 section. | **AGENT-FIXABLE** — one line at :691, extend :33, add the adjustments section |
| REV-030 | minor | P2 | new | `footerChips` advertises `[m] go` on four card shapes whose `m` bounces (table above, two controls clean). | **AGENT-FIXABLE** — both new refusals are I/O-free; add them to the chip switch + an "advertised IFF launchable" table guard |
| REV-031 | minor | P2 | new | `notRunnableBounce` claims to mirror `runnableHint` and already disagrees on `done`; the empty-status arm renders `x is  - …`. Pair unpinned — the REV-002/012/016 shape. | **AGENT-FIXABLE** — one producer both quote (as the cap already does), or fix `done` + empty and correct the comment; add the agreement table |
| REV-032 | minor | P3 | new | Both round-06 insertions orphaned the doc comment of the function below: `notRunnableBounce` carries `decisionGateBounce`'s 10-line doc (move.go:144-166) and `selectableForShip` carries `selectedFeatures`' (model.go:764-783). Four symbols now show the wrong prose. Exact REV-014 regression, twice. | **AGENT-FIXABLE** — split the blocks; a first-word doc guard would make it unrepeatable |
| REV-033 | minor | P3 | new | The stale selection is filtered, not cleared, so it **resurrects** when the card returns to ready — verified t0→t1→t2 through the real loader: `m` then returns `/gogo:done shipme` with no keystroke. | **AGENT-FIXABLE** — prune non-selectable keys in `reload()`; extend the test with the loop |
| REV-034 | nit | P3 | new | `state.md`'s `resume:` line records a "Known gap for ④" that is provably false (both arms discriminate under mutation). A wrong hand-off note costs ④ a round. | **AGENT-FIXABLE** — one line; and score a self-reported gap before recording it as fact |
| REV-027 | minor | P2 | **verified** | 3/3 mutation score re-derived independently; bypass closed on the real loader path; no fourth predicate. | — |
| REV-024 | minor | P3 | open | Carried by instruction: `report/report.md:436` **and `:439`**. | **AGENT-FIXABLE at ⑤'s re-reconcile** |
| REV-029 | nit | P3 | open | Carried, out of round-06 scope: `test-strategy.md` still says "TEN variants". | **AGENT-FIXABLE** |

**Counts:** 0 open blockers · **2 open majors** · 5 open/new minors · 2 open/new nits · 25 verified (34 total).

## Verdict — CHANGES

Two of the four fixes are good: **REV-027 is verified** at full mutation score and holds on the
real path, and **REV-028** closed the dangerous instruction site (`plan.md:805`). The
coordinator's two self-reported worries were both worth raising and both resolved differently
than expected — the `awaiting-uat` arm *does* discriminate, and the `strings` import landed
clean.

But two fixes do not close their issue, and the evidence raised both to **major**:

- **REV-026** guarded one arm of a two-arm decision. On the sibling arm the board launches a
  card that `gogo go` refuses with exit 1 and that `gogo status` prints as `WAIT` — the
  board-narrates-a-lie defect this whole release exists to remove, reproduced end-to-end. The
  reachable trigger is a lagging `phase:` line, which is the release's own subject matter.
- **REV-025** replaced a wrong message with a heuristic that is wrong in the other direction
  (a plain decision now gets UAT advice), disagrees with the pill on the same card, duplicates
  a contract-anchored predicate that already exists in the file, and is **completely unpinned**
  — the entire branch deletes green.

Both are one-line fixes with a clear, deterministic shape, and both need a guard that bites.
The five minors and the nit are cheap and should ride along — REV-030 in particular, since the
footer chip has now gone stale on two consecutive rounds for the same reason.

This loops back to ②. Once REV-025/026 land with tests that fail when the fix is reverted,
④ takes its final pass and ⑤ re-reconciles.
