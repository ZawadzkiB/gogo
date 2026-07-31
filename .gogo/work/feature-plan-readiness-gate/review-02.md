# Review 02 - plan-readiness-gate (0.29.0)

Round **2** · reviewer: `gogo-reviewer` (fresh eyes) · 2026-07-30
Scope: the round-01 fix set (+389/-51 on top of round 01; 49 files, +2039/-275 total)
Contract: `review/issues.json` (round 2) - this file is its rendered snapshot

Gates re-run independently, all green:
`gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` green (12 packages) ·
**hermetic gate** `env PATH="$(dirname $(which go)):/usr/bin:/bin" go test -count=1 ./...`
green with `command -v claude` **empty** - the standing gate REV-001 created.

## Round-01 findings: all 8 re-verified, none re-opened

| id | severity | round-01 | now | how I verified it |
|---|---|---|---|---|
| REV-001 | blocker | fixed | **verified** | Hermetic run green with no claude on PATH; the fix uses the real `writeStubClaude`/`wireStub` seam (not a skip) and **strengthened** the test - it now also asserts no `argv.log` and no `locks/` dir, pinning "refuses before launch and before the lock" |
| REV-002 | major | fixed | **verified** | Single-sourced to `orchestrator.CapRuleClause`, quoted by all four surfaces; my own repo-wide grep finds **zero** stale phrasings in production `.go` (only negative test assertions + the dated 0.28.0 notes in `docs/cli-contract.md:88` and `project-knowledge.md:402`). Mutations: gutting the clause → caught; reverting one surface to a known stale phrasing → caught; dropping the `--help` interpolation → build fails (unused import). Residual weakness → REV-012 |
| REV-003 | major | fixed | **verified** | Re-probed the unattended path: unwritten → `autoPickupReady=false`, 0 cmds, 0 launcher calls, nothing recorded in the fire-once set, and `autoPickupBlocked=false` (the cue and the fire path really do share the helper); written → fires **exactly once**. Removing the guard is caught; removing the fixture's new `plan.md` makes `TestAutoPickupFiresOnReload` **fail**, so the on-disk arm is load-bearing and no other test uses that fixture |
| REV-004 | minor | fixed | **verified** | Comments now state the session rule; the `ready` row gained a `gogo-done-ready` session **plus** a discriminating pre-assert (attributed ✓, build ✗), and the cases assert exact slug sets instead of tallies. Removing that fixture session is caught by `TestActiveWorkCount` - it is not decoration |
| REV-005 | minor | fixed | **verified** | `got.View()` now asserts the glyph, the reason and the unblock. Two mutations catch it: deleting the drill note, **and** deleting `viewDrill`'s `m.status` render (the literal 0.16.0 regression) |
| REV-006 | minor | fixed | **verified** (with a caveat, below) | Prose moved into step 1 of `## ② Steps` in all three phase skills; the deterministic detector `phaseLineLags` → `· state lags` fires on exactly the observed shape; five mutations against its four conditions are all caught by six negative subcases; recorded as a limitation in `plan.md` and carried to ⑤ |
| REV-007 | minor | fixed | **verified** | `planUnready` is pure; `footerChips` uses it; **the structural guard reads code, not comments** (`tuiFuncBody` strips `//`), and it is paired with a genuinely discriminating behavioural half: delete `plan.md` under a written Feature and the footer must not change. Reverting `footerChips` to `planReadinessBounce` is caught. Cosmetic fallout → REV-014 |
| REV-008 | minor | fixed | **verified** | The decision is documented in `cap.go` and pinned by `TestCapCountsATerminalFeatureStillHoldingABuildSession` (with a `TerminalStatus` fixture pre-check); re-adding the class filter is caught. **The escape hatch works**: probe shows the sweep reaps `gogo-go-shipped-one` ("owning feature shipped-one is shipped") and `ActiveWorkSlugs` then returns empty - the slot frees. The **wording** of that remedy is the problem → REV-011 |

**REV-006's caveat, for the record.** Round 02 *also* ran with `state.md` at
`implement` / `implementing` and no review `phase-started` in `events.jsonl` - the prose half
did not fire a second time. That is not a regression and I am not re-opening it: the fix's
load-bearing half is the detector, and this feature's own card is currently in exactly the
shape that trips it. It does mean the ⑤ report must keep the "advisory writer" limitation
`plan.md` already records, unsoftened.

## Round-02 mutation sweep - 20 mutations, `go vet` first, landed-edit verified

Method per the coordinator's warning list: **`go vet ./...`** as the compile check (it
type-checks `_test.go`, which `go build` does not - N17/N20 mutate test files), and a
landed-edit assertion that tolerates insertion-shaped mutations (a marker unique to the new
text, not "the anchor is gone"). Run in a throwaway copy under `/tmp`, restored from a
pristine snapshot between mutations; the repo was never modified.

**17 CAUGHT · 1 BUILD-FAIL (not a result) · 2 SURVIVED (both are findings below).**

| # | mutation | outcome |
|---|---|---|
| N1-N5 | `phaseLineLags`: drop the user-gate guard · accept any event · naive `f.Phase` compare (no `EventsPhase`) · drop the live-session test · remove the cue arm | CAUGHT (each by the matching negative subcase; N3 by the knowledge→report assertion) |
| N6 | `autoPickupReady`: remove the `PlanUnwritten` guard | CAUGHT `TestAutoPickupRefusesAnUnwrittenPlan` |
| N7 | `planUnready`: drop the FR8 arm | CAUGHT `TestPlanUnreadyAgreesWithTheBounce` + `TestFooterDoesNotOfferAnIllegalMove` |
| N8 | `footerChips`: back to `planReadinessBounce` | CAUGHT `TestFooterChipDoesNoDiskIO` |
| N9 | gut `CapRuleClause` to "is per source" | CAUGHT `TestCapRuleClauseSaysWhatItCounts` |
| N10 | drop the `--help` clause interpolation | BUILD-FAIL (unused import) - not a result, but a real accidental guard |
| **N11** | **decoy: hand-write ONE of config_tab.go's two surfaces with fresh wording, keep the comments** | **SURVIVED → REV-012** |
| N12 | revert a surface to a known stale phrasing | CAUGHT `TestNoStaleCapRuleWording` |
| N13 | re-add `Class == ClassInProgress` to the cap | CAUGHT (2 tests, incl. the new terminal-feature one) |
| N14 | remove `gogo sweep` from the bounce | CAUGHT `TestCapBounceStatesTheNewRule` |
| N15 | remove `f.PlanUnwritten = !planWritten(dir)` from `loadFeature` | CAUGHT (2 contract tests) |
| **N16** | **swap `cardStateCue`'s documented arm order** | **SURVIVED → REV-013** |
| N17 | remove the on-disk auto-pickup fixture's `plan.md` | CAUGHT `TestAutoPickupFiresOnReload` |
| N18 | delete the drill note (`quickView`'s `setStatus`) | CAUGHT `TestQuickViewNamesTheMissingPlan` |
| N19 | stop rendering `m.status` in `viewDrill` | CAUGHT (3 tests, incl. the REV-005 fix) |
| N20 | remove `gogo-done-ready` from the cap fixture | CAUGHT `TestActiveWorkCount` |

**A correction I owe the record.** My first read of the `state lags` logic concluded it would
fire for the whole of ⑤ (because FR11 gives ⑤ no occupancy write). That is **wrong**:
`gogo-knowledge` appends `phase-started`/report as its first act, so during ⑤ the newest event
is not a `phase-done` and the cue is silent. I withdrew the finding before writing it. The
detector's real gaps are the two below, which are the opposite shape.

## New findings

### REV-011 - major · P1 · new

**The cap bounce recommends the whole-board `gogo sweep`.** The remedy REV-008 asked for
works, but the bounce (`move.go:244`) names the **bare** form:

> press M to force, ship one, run `gogo sweep` if a blocker already shipped, or run `gogo go <slug> --force`

Bare = `Sweeper.Only` empty = every `gogo-*` session on the machine judged against *this*
repo's features, so another source's live build has no owning feature here and
`shouldReap` calls it `orphan - no owning feature`. Probe:

```
before sweep: active=[shipped-one] capExceeded(1)=true
bare sweep reaped=[gogo-go-shipped-one]   log="reaped gogo-go-shipped-one (owning feature shipped-one is shipped)"
after sweep:  active=[] capExceeded(1)=false          <- the remedy DOES free the slot
bare sweep on a multi-source host killed=[gogo-go-shipped-one gogo-go-someone-elses-live-build]
TARGETED `gogo sweep shipped-one`  killed=[gogo-go-shipped-one]   <- the safe form
```

`cmdSweep` has no confirmation (only an opt-in `--dry-run`), and this message appears in the
multi-source cockpit by construction (a per-source cap on the unified board).
`test-strategy.md` singles the bare form out as the one that "can reap the user's REAL
in-flight sessions"; standard #9 requires slug-targeting for reaps. The targeted form is free -
the bounce already prints the blocking slugs. In fairness the bare form *is* the sanctioned
**manual** cleanup; what makes this a finding is recommending it inline, as the fix for one
named blocker, without the context `sweepHelp` gives.
**Fix:** interpolate the slug (`gogo sweep <slug>`), and assert in
`TestCapBounceStatesTheNewRule` that a bare `gogo sweep` is *not* offered.

### REV-012 - minor · P2 · new

**The cap-rule single-source guard can be evaded.** `TestCapRuleIsSingleSourced` greps each
surface **file** (comments included) for `CapRuleClause`. `config_tab.go` holds **two**
surfaces and three comments naming the constant, so hand-writing one of them still passes -
mutation N11 hand-wrote the cap **form field** ("N caps this repo's running items") and the
whole suite stayed green, i.e. the field the user reads while setting the cap can go stale
again, which is REV-002 verbatim. The grep companion only knows four historical phrasings, so
fresh wording passes both. The repo already has the right technique - the tui guard strips
comments - and `TestHelpStatesTheCapRule` shows the stronger pattern: assert the **rendered**
string. Do that for `capScopeNote(1)` and the form field too.

### REV-009 - minor · P2 · new

**The detector only sees forward hand-offs.** `phaseLineLags` needs a `phase-done` for the
phase `state.md` names, so two probe-verified shapes are invisible - both with a live build
session, both exactly the failure REV-006 exists to expose:

```
state.md review/reviewing   + newest event fix-round/implement      -> phaseLineLags=false, cue ""
state.md implement/implementing + newest event phase-started/review  -> phaseLineLags=false, cue ""
```

The first is the pipeline's most common re-entry (③→②); the second is a partial step 1 (event
appended, `state.md` write skipped) - the likeliest partial compliance, since the skill pairs
the two writes. In both, the telemetry *names the newer phase*, which is stronger evidence
than the shape the detector does check. **Fix:** add the symmetric arm - an entry event
(`phase-started`/`fix-round`) whose phase ≠ `EventsPhase(f.Phase)` is also a lag - and pin
both shapes as positive cases, keeping a negative for an entry event that matches.

### REV-013 - minor · P3 · new

**`cardStateCue`'s "Order matters" is unpinned.** Swapping the `buildingDisagreement` and
`phaseLineLags` arms (N16) compiles and leaves the suite green. The overlap is reachable: a
UAT-rerun card (`plan-accepted` + `phase: knowledge` + newest event `phase-done`/report + live
`gogo-go`) makes **both** true, so the order alone chooses `● building` (correct) over
`· state lags`. One test that asserts both predicates are true *and* that the cue is the
building one closes it.

### REV-010 - minor · P3 · new

**`phaseLineLags` has no status whitelist**, unlike its sibling `stalledPhase`. An `aborted`
feature with `phase: implement`, a `phase-done`/implement event and a lingering `gogo-go`
session renders `· state lags` - nothing is lagging, the item is dead. Narrow, but it is
REV-008's own failed-reap scenario, and `shipped` escapes only because `ClassShipped` renders
in the collapsed changelog list rather than through `renderCard` - an accident, not a rule.

### REV-015 - minor · P3 · new

**`gogo go`'s cap refusal still says "ship/finish one first"** (`go.go:253`) - the impossible
advice for an already-shipped blocker that REV-008 removed from the board. The two cap
surfaces share their decision helpers precisely so they "never drift"; the headless one is
what an unattended operator reads. Add the same targeted `gogo sweep <slug>` remedy.

### REV-014 - nit · P3 · new

**A doc comment lost its function.** The REV-007 split inserted `planUnready` between
`planReadinessBounce`'s doc comment and its `func`, and the blocks were merged
(`move.go:114-140`): the comment on `planUnready` opens "planReadinessBounce returns the
status-line refusal…", describes the two gates, then pivots mid-block; `planReadinessBounce`
(`move.go:148`) has no doc at all. Split it back.

## Cross-cutting re-checks (all clean)

- **Write scope.** No new `os.WriteFile` / `MkdirAll` / `exec.Command` in product code; every
  added write is in a `_test.go` under `t.TempDir()`. The CLI stays a read-side reader.
- **Plan fidelity.** Nothing unplanned crept in: `phaseLineLags`/`· state lags` and
  `CapRuleClause` are review-driven fixes, and `plan.md` records the REV-006 one as a
  **known limitation on evidence** with the honest claim ("it does not narrate the past for
  ②, and for ③/④/⑤ it either narrates the present or says out loud that it cannot") and
  carries it to ⑤. No FR is under-built that I can find; nothing is over-built.
- **Enumeration sync.** The new cue is documented everywhere the other two are: `README.md`,
  `docs/cli-contract.md`, `docs/flow.md`, `skills/gogo-cli` and the three phase skills.
- **Conventions.** Zero em dashes in added lines (product, docs and all five new test files).
- **`coding-rules.md` deferral to ⑤ is still correct and still recorded** - the plan assigns
  that write to ⑤, knowledge files are reconciled by the report phase, and `state.md`'s
  `resume:` line names the carry.

## Verdict

**CHANGES** - one open major (REV-011: the bounce recommends a destructive whole-board sweep;
one-line fix, the slug is already in the message). Everything from round 01 is verified, the
fixes are better than the findings asked for in three places (the single-sourced clause, the
load-bearing cap fixture, the strengthened FR8 CLI test), and 17 of 20 fresh mutations are
caught. Fix REV-011, batch the five minors + the nit, and re-review.
