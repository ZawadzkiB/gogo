# Review 03 - plan-readiness-gate (0.29.0)

Round **3** · reviewer: `gogo-reviewer` (fresh eyes) · 2026-07-30
Scope: the round-02 fix set (50 files, +2202/-277 total)
Contract: `review/issues.json` (round 3) - this file is its rendered snapshot

Four gates, re-run independently, all green:
`gofmt -l .` clean · `go vet ./...` clean · `go test -race -count=1 ./...` green ·
`env PATH="$(dirname $(which go)):/usr/bin:/bin" go test -count=1 ./...` green with
`claude` absent. Version `0.29.0` in both `plugin.json` and `cli/main.go`.

## Round-02 findings: all 7 re-verified

| id | severity | now | how I verified it |
|---|---|---|---|
| REV-009 | minor | **verified** | Arm B added for entry events. Both of my round-02 probe shapes now fire (`fix-round`/implement vs line `review`; `phase-started`/review vs line `implement`), each pinned as a positive subtest that asserts the **cue**, not just the predicate. Five mutations against the arms (remove B · widen `isEntryEvent` to any event · drop `fix-round` · invert the comparison · drop `EventsPhase`) are all caught. The narrowing is correct - see REV-017 for the reasoning, which is not |
| REV-010 | minor | **verified** | The working-status whitelist mirrors `stalledPhase`; `aborted` and `shipped` with a lingering build session are pinned negatives; removing the whitelist is caught by four subtests **and** by the exclusivity test |
| REV-011 | major | **verified** | `CapSweepRemedy` can only emit the targeted form; both refusals quote it. Producer, board bounce and headless refusal each assert the targeted substring **and** the absence of a bare `gogo sweep`. Emitting a bare sweep from the producer is caught by 3 tests across 2 packages; hand-writing one at the board site is caught; dropping it from `gogo go` is caught. Multi-blocker reads correctly (`gogo sweep one two`) |
| REV-012 | minor | **verified** (with a new gap one level up) | The structural guard now strips `//` comments and counts **per surface**; the behavioural guard asserts the producers' rendered strings. My decoy (hand-write the field **and** add `var _ = orchestrator.CapRuleClause` to restore the count) is caught by the behavioural half. The producers are guarded; their **call sites** are not - REV-016 |
| REV-013 | minor | **verified, and the push-back is right** | I re-derived the disjointness independently over a matrix **wider** than the shipped test's - 13 statuses × 9 phases × 85 events × 11 session sets = **109,395 combinations, 0 overlaps**, all three arms reached (building 4590 / lags 954 / stalled 6885). So "Order matters" was indeed an unheld claim, my UAT-rerun overlap fixture is no longer an overlap (REV-010's whitelist removed it), and swapping the arms still survives - correctly. The replacement guard bites where it should: widening `phaseLineLags` to include `plan-accepted` fails `TestCueArmsAreMutuallyExclusive`, and shrinking the matrix so an arm is unreachable fails its reachability check |
| REV-014 | minor→nit | **verified** | The two doc blocks are split back onto their own functions, and a stale cross-reference to a non-existent test was corrected on the way |
| REV-015 | minor | **verified** | `capBlock` quotes the same producer; `go_cap_test.go` asserts the exact targeted substring and the absence of the bare form |

I also re-checked that the round-02 work did not regress the earlier load-bearing invariants:
re-adding `!force` to the plan-readiness bounce (FR4a), deleting the auto-pickup `PlanUnwritten`
guard (FR8/REV-003), and reverting the cap to any-session attribution (FR12) are each still
caught, by 4, 1 and 6 tests respectively.

## On the two push-backs you asked me to judge independently

**REV-009's narrowing: the decision is right, the stated reason is not.** Restricting arm B to
entry events avoids a real false positive, but not for the reason the comment gives - see
REV-017. The comparison you asked for (is the false negative worse than the false positive it
avoids?): both cost exactly one half-completed step 1, in mirror directions, so neither
dominates. The admitted FN - a phase that skips step 1 but later appends `issues-found`, which
replaces the `phase-done` arm A was reading - is probe-confirmed silent. The avoided FP would
have accused a **correct** `state.md`. Given the cue's job is to say "the phase line is behind",
staying silent when the line is right is the better error, so the narrower rule is the right
call. It is also worth recording that silence is not proof of health.

**REV-013's disjointness claim: true, and better than what I asked for.** My round-02 request
was unsatisfiable - I asked for a test that fails when the arms are swapped, and no such test
can exist once the arms are disjoint. The developer noticed that instead of writing a test that
passes for the wrong reason, which is the failure mode this feature keeps hitting. The
reachability guard is genuinely non-vacuous: my independent wide matrix reaches all three arms
hundreds of times, and shrinking the shipped matrix to one status makes the guard fail.

**On A13b's fairness:** the argument is sound. A guard-only revert cannot fail while the
production code is correct, so scoring it as SURVIVED would be meaningless - the honest test is
to weaken the guard **and** introduce the defect it exists to catch, then check that something
else still bites. I re-ran that construction myself (weaken the structural guard back to
per-file **and** hand-write the field description): `TestConfigTabCapSurfacesRenderTheRule`
catches it. A13b is a fair test and it passes. I have adopted the **nameless-CAUGHT ⇒ unscored**
rule; one of my own mutations (C1) hit exactly that signature and turned out to be a
mutation-side compile error (`strings` imported and not used), so I re-ran it compile-clean -
it is CAUGHT by 3 tests.

## Round-03 mutation sweep - 21 mutations, `go vet` first, landed-edit verified

`go vet ./...` as the compile check (it type-checks `_test.go`; B9 and D3 mutate test files),
a marker unique to the new text for the landed-edit check, a nameless CAUGHT scored UNSCORED,
and a pristine restore between mutations. Run in a `/tmp` copy; the repo was never modified.

**17 CAUGHT · 1 SURVIVED-as-expected · 1 BUILD-FAIL (rerun clean → CAUGHT) · 2 SURVIVED (findings).**

| # | mutation | outcome |
|---|---|---|
| B1-B4 | remove arm B · widen `isEntryEvent` to any event · drop `fix-round` · invert arm B's comparison | CAUGHT (arm-B positives, and B2 by the `issues-found` boundary case) |
| B5-B7 | remove the status whitelist · widen it to `plan-accepted` · drop `EventsPhase` | CAUGHT (B5 also by the exclusivity test; B6 **only** by it - which is the point) |
| B8 | swap `cardStateCue`'s arms | **SURVIVED as expected** - the arms are disjoint, so order is not load-bearing |
| B9 | shrink the exclusivity matrix to one status | CAUGHT by the reachability guard |
| C1 | `CapSweepRemedy` emits a bare sweep | first run BUILD-FAIL (my own unused import → UNSCORED); rerun compile-clean → CAUGHT by 3 tests in 2 packages |
| C2-C4 | hand-write a bare sweep at the board site · drop the remedy from `gogo go` · make the empty case emit a bare sweep | CAUGHT |
| D1 | hand-write `capFieldDescription`'s text | CAUGHT by both guards |
| D2 | **decoy**: hand-write it **and** add `var _ = orchestrator.CapRuleClause` to restore the per-surface count | CAUGHT by the behavioural guard |
| D3 | **A13b**: weaken the structural guard to per-file **and** hand-write the field | CAUGHT by the behavioural guard |
| **D4** | **`startSourceForm` stops calling `capFieldDescription()` and hand-writes the field text** | **SURVIVED → REV-016** |
| **D5** | **`viewConfigRight` stops calling `capScopeNote()` and hand-writes the cap row** | **SURVIVED → REV-016** |
| D6 | both wirings hand-written using a KNOWN stale phrasing | CAUGHT `TestNoStaleCapRuleWording` (the narrow case) |
| E1-E3 | regression checks on FR8/auto-pickup, FR4a/force, FR12/cap attribution | CAUGHT (1, 4, 6 tests) |

## New findings - 3 minor, 2 nit, no blockers or majors

### REV-016 - minor · P2 · new
**The producers are asserted; the wirings are not.** REV-012 extracted `capFieldDescription()`
so a guard could assert a rendered string - but the guard calls the producer directly, and the
per-surface count is satisfied by the producers' own bodies. So either call site can stop using
its producer and hand-write fresh copy with the whole suite green (D4, D5). That is standard
#11(b) exactly, one level up from REV-012, on the surface REV-002 went stale on - the **eighth**
variant of the "assertion that looks like a check and isn't" class in this feature.
**Fix:** `viewConfigRight` is a `Model` method returning a string, so assert the *rendered*
detail row contains the clause; the huh field's `Description` cannot be read back, so assert
`startSourceForm`'s body calls `capFieldDescription(` using the comment-stripping `tuiFuncBody`
helper already in the file.

### REV-017 - minor · P3 · new
**The detector's justification comments claim a precision the evidence cannot deliver.** The
mid-phase exclusion is justified as protecting "a writer that did everything right" - but in
that shape implement wrote `state.md` and its best-effort event append did not land, so the
right justification is simply that `state.md` is **correct** there. And the same failure mode is
*not* excluded from arm B: `state.md implement/implementing` + newest `phase-started`/review is
byte-identical whether review skipped its state write (lag, cue right) or implement wrote its
state and its append failed (state correct, cue wrong). Symmetrically, a later `issues-found`
can mask arm A. All three residuals are acceptable - the cue still marks a real disagreement -
but the comment is what a future maintainer will tune the rule from. Comment-only fix.

### REV-018 - minor · P3 · new
**`README.md` still states arm A as the whole cue rule** after round 02 widened it;
`docs/cli-contract.md`, `docs/flow.md` and `skills/gogo-cli` were all swept. Standard #1 names
README.md explicitly.

### REV-019 - nit · P3 · new
**`CapSweepRemedy("")` would render as `, ,`** at both call sites, and its doc claims callers
"omit the remedy" when neither does. Unreachable today (both sites sit behind `CapExceeded`,
which implies a non-empty blocker list), so it is a latent-contract nit, not a defect.

### REV-020 - nit · P3 · new
**The FR11 limitation record says n=2; round 03 makes it n=3.** The wording was **not**
softened - "a hypothesis that has not yet paid off", "the release claim must therefore rest on
the DETECTOR", "Do not soften this" are all intact and correct. But this round ran the same way
(`state.md` still `implement`/`implementing`, `iterations: review=2`, no review `phase-started`),
so the prose fix has now been skipped on three consecutive runs. Worth one line before ⑤ quotes
the number - and worth noting that this feature's own card is currently in exactly the shape
arm A detects, which is the most convincing thing the report can say about the detector.

## Cross-cutting re-checks (all clean)

- **Write scope.** No new `os.WriteFile` / `MkdirAll` / `Remove` / `exec.Command` in product
  code; the three files with added writes are all `_test.go` writing under `t.TempDir()`.
- **Plan fidelity.** Nothing over- or under-built: `CapSweepRemedy`, `capFieldDescription`, arm
  B and the status whitelist are all review-driven fixes, and the two things deliberately NOT
  done are recorded in `plan.md` with reasoning (hardening the bare `gogo sweep` itself, and
  resolving a session's owner across all registered sources so a cross-repo session stops
  looking like an orphan) rather than silently dropped.
- **Conventions.** Zero em dashes in added lines. `gofmt`/`vet` clean.
- **`coding-rules.md` deferral to ⑤ is still correct and still recorded** - `.gogo/knowledge/`
  is untouched in the diff, and `state.md`'s `resume:` line still names the carry.

## Verdict

**APPROVE** - no open blockers or majors. Every round-01 and round-02 finding is verified, the
two push-backs are correct (one of them corrected a request of mine that was unsatisfiable), and
the fix set survives a 21-mutation sweep with two escapes, both in guards rather than in
behaviour. The five open items are minors and nits: REV-016 is the one worth doing before ship
(it re-opens REV-012's hole at the wiring level), the rest are comment, doc and record accuracy.
Advance to **④ test**.
