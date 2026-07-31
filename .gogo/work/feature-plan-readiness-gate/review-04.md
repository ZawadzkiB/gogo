# Review 04 - plan-readiness-gate (0.29.0)

Round **4** · reviewer: `gogo-reviewer` (fresh eyes) · 2026-07-30
Scope: the **exit-write restoration** and its fallout (plus the Go that changed after round
03's approval - see "Scope note")
Contract: `review/issues.json` (round 4) - this file is its rendered snapshot

Four gates green: `gofmt -l .` · `go vet ./...` · `go test -race -count=1 ./...` ·
`env PATH="$(dirname $(which go)):/usr/bin:/bin" go test -count=1 ./...` with `claude` absent.
`gogo --version` → 0.29.0.

## The judgement call: accept the hand-off blink. Do not narrow it.

**Verdict: the developer's trade-off is right, and I would reject the narrowing I can think of.**

What I checked before agreeing. Arm A fires when the newest event is a `phase-done` for the
phase `state.md` names, with a live build session and a working status. Restoring §④'s write
means `state.md` reliably names the phase that just ended, so that shape is now reached at
every healthy hand-off, for the gap up to the next phase's entry write.

1. **It is a true statement, not a false positive.** The named phase has ended; nothing has
   claimed the next. Per REV-017 the cue's documented meaning is already "`state.md` and
   `events.jsonl` disagree about the current phase", which is literally the case there.
2. **The restoration makes arm A *more* useful, not less** - the point I think settles it.
   BEFORE the restoration, when the entry write was skipped (n=3, every live run), `state.md`
   kept naming some *earlier* phase, so `e.Phase == line` failed and **arm A was silent exactly
   when the file was worst**. §④'s write is what makes arm A's precondition hold, so the
   whole-phase lit case is now reliably detected. Deleting arm A to stop the blink would give
   back both properties at once - the warning in the code is correct.
3. **Precedent, and duration is the discriminator.** `● building` covers the
   launch-to-first-write window on the same self-clearing terms (D6=A). A blink of seconds
   versus a cue lit for a whole phase is a difference a user can actually read.
4. **The one feasible narrowing is worse.** `contract.Event` does carry `TS`/`TSValid`, so arm A
   could require `time.Since(e.TS) > grace`. I would not take it: it puts a wall clock inside a
   pure, file-derived display predicate (the read path's determinism is an NFR, and every test
   would need an injectable clock); the constant has to exceed the slowest hand-off, which
   includes a subagent spawn on an unknown host, so too small restores the blink and too large
   masks the real failure for that long; and decisively, **the board re-renders on fsnotify, not
   on a timer**, so a grace period would only move *when* the cue appears - it would still be
   there on the next reload - unless a ticker were added to the TUI for a cosmetic cue. That is
   plainly a worse trade than a self-clearing blink.

The one thing missing is that the blink is explained only in a Go comment - see **REV-023**.

## Scope note: production Go DID change after round 03's approval

The round-04 restoration's Go touch is genuinely comment-only - I diffed
`phaseLineLags`, `isEntryEvent` and `cardStateCue` against the text I verified in round 03 and
they are **byte-identical**, and `authoring_test.go` gains one case (the hand-off shape). That
claim holds.

But three production-Go changes landed between my round-03 APPROVE and now, and none had a ③
pass: `state.go`'s block-aware comment parse (④'s TEST-001), `orchestrator.CapRefusal` plus both
cap-refusal call sites (my REV-019), and the two config-tab wiring guards (my REV-016). The
gogo-test route requires `② implement → ③ review` after a test-round fix, so this round is that
pass. I reviewed all three fresh and mutation-tested them; **no defects found**, and REV-016-020
are now `verified`. Flagging the boundary rather than the code: rounds 01-03 did not cover this.

| id | now | evidence |
|---|---|---|
| REV-016 | **verified** | The two mutations that SURVIVED in round 03 - `startSourceForm` hand-writing the field text, and `viewConfigRight` hand-writing the cap row - are both now CAUGHT by `TestConfigTabCapSurfacesRenderTheRule` |
| REV-017 | **verified** | The comment now states the ambiguity ("a skipped state write and a failed event append look identical on disk") and that silence is not proof of health; `skills/gogo-cli` and `plan.md` carry the same bound |
| REV-018 | **verified** | README now names both shapes |
| REV-019 | **verified** | `CapRefusal(remedies ...string)` drops empty parts before joining, so the empty-sweep case reads "press M to force, ship one, or run `gogo go x --force`" with no double comma; deleting the empty filter is CAUGHT |
| REV-020 | **verified** | `plan.md` records n=3 and keeps "a hypothesis that has not paid off" |
| TEST-001 | reviewed here | `advanceComment` is correct on my read (alternating open/close scan; single-line behaviour unchanged; every edge decided toward "missing, never wrong" and documented). Three mutations - stop skipping commented lines, never find the closer, never find the opener - are each CAUGHT, including by `TestShippedTemplateScaffoldParsesClean`, which reads the real template |

## Round-04 checks

- **The restored §④ text is in all four places.** `gogo-implement`, `gogo-review` and
  `gogo-test` each write `phase`/`status` + the `iterations` bump, each carrying the identical
  "**Write phase/status here EVEN THOUGH §② step 1 already did. The redundancy IS the design -
  do not 'clean it up'**" paragraph with the floor/ceiling rationale. `skills/gogo/SKILL.md`
  carries it in its own voice ("**§④ then writes `phase`/`status` AGAIN** ... belt and braces on
  purpose, because the entry write is prose an LLM follows and has been skipped in practice").
  A future reader cannot mistake it for duplication. One gap: the sentence is **unscoped** with
  respect to the decision-gate route - **REV-022**.
- **The skip-count sweep fixed the claim, not just the number.** All three phase skills now say
  the write "has been skipped on **all three** of its live runs so far, **twice AFTER the move
  into the numbered steps, so the move helped less than hoped**", and `plan.md` keeps the theory
  labelled as a hypothesis that has not paid off. That is the falsified claim retired honestly.
- **`coding-rules.md` is right and complete** - "at entry AND again at exit", "Keep both
  writers; the redundancy IS the design", the honest history of the removal, and the caveat that
  no safety property may lean on it (with the cap and the detector named as the deterministic
  halves). **Its three siblings are not** - **REV-021**, the one blocker-adjacent finding.
- **`docs/cli-contract.md` and `docs/flow.md`** both restored correctly ("the exit write ALSO
  writes `phase`/`status` ... belt and braces").
- **The `state.md` walk-back was the right call.** With `report/` on disk, `awaiting-uat` would
  make `classify()` return ready-to-ship and `/gogo:done` could ship the pre-restoration bundle;
  `implementing` fails `readyToShipStatus`, so the card is in-progress. Verified against the
  live reader: `gogo status` reports **ready 0**, this feature in-progress, and no other feature
  claims ready-to-ship. `report/report.md:436` is indeed still wrong - **REV-024**.
- **Write scope, conventions.** No new writes in product code; zero em dashes in added lines.

## Mutation sweep - 13 mutations, re-derived independently

`go vet` as the compile check, insertion-aware landed-edit verification, nameless-CAUGHT scored
UNSCORED, pristine restore between runs, in a `/tmp` copy. I re-derived the arm-A/arm-B set
rather than reusing the reported table.

| # | mutation | outcome |
|---|---|---|
| A1 | delete arm A | CAUGHT (incl. the new hand-off case) |
| A2 | delete arm B | CAUGHT |
| A3 | drop `fix-round` from `isEntryEvent` | CAUGHT |
| A4 | widen the status whitelist to `awaiting-uat` | CAUGHT (the user-gate negative) |
| A5 | drop `EventsPhase` (naive phase compare) | CAUGHT |
| W1/W2 | the two round-03 escapes (hand-write either config-tab surface) | **both now CAUGHT** |
| R1 | `CapRefusal` stops filtering empties | CAUGHT |
| R2 | pass an extra `""` into `CapRefusal` | SURVIVED - **equivalent mutant, my error**: filtering empties is the function's contract, so the mutation is semantically null. Not a finding |
| T1 | remove the commented-line skip | first shape BUILD-FAIL (unused var) → **unscored**; reshaped (`commented && false`) → CAUGHT by 4 tests |
| T2 | closer scan never matches | first shape (`line[i:]`) SURVIVED - **equivalent mutant, my error** (the remaining text cannot contain `<!--`); reshaped (`"--!>"`) → CAUGHT by 11 |
| T3 | opener scan never matches | CAUGHT by 8 |

Two of my own mutations were equivalent mutants and one was a compile error - all three are
scored as such rather than as escapes, per the harness rules this feature has accumulated.

## Findings

### REV-021 - major · P1 · new
**The knowledge sweep is incomplete, and one of the misses tells the next reviewer to undo the
fix.** `coding-rules.md` was corrected; three sibling always-read statements were not:
- `non-functional-requirements.md:35` - "the `iterations` bump and `phase-done` **stay at the
  exit**", i.e. the exit carries only those two;
- `project-knowledge.md:443`, inside the **0.29.0** entry (the release being shipped, not dated
  history) - "③/④'s exit write **no longer sets `phase`/`status` at all**, so a skipped entry
  write leaves the line stale **indefinitely**";
- `code-review-standards.md:85` - "**Flag** a phase skill whose `phase`/`status` write sits
  after the work rather than as its first act after validate-in", which instructs a future
  fresh-eyes reviewer to flag §④'s restored write as a defect. That is how this exact regression
  comes back, through a reviewer following the standards in good faith.

Three one-line edits, no code. This is the "always-read context propagates to every future
worker" hazard, and right now the knowledge layer contradicts both the skills and itself.

### REV-022 - minor · P2 · new
**The restored §④ write is unscoped.** `gogo-review`/`gogo-test` §④ list the decision-gate route
("set `state.md` `waiting-for-user` ... stop and ask") and then say unconditionally "Update
`state.md`: `phase: review`, `status: reviewing`". Read sequentially, that clobbers the gate -
losing `WaitingForInput()`, the `⏸ K need you` count, the gate stripe, and the protection that
keeps `/gogo:go` off a feature parked on a user answer. The pre-0.28.0 text had the same shape,
but the 0.29.0 text it replaced ("leave phase/status as they are") did not, so the restoration
brings the ambiguity back - in the release whose thesis is that prose gets misread. Scope the
sentence to the routes that continue.

### REV-023 - minor · P3 · new
**The blink is explained only in a Go comment.** `skills/gogo-cli` and `README.md` describe the
cue purely as a defect claim, so a user watching a normal run sees `· state lags` at every
transition with nothing saying it is expected and self-clearing - and duration, the one
discriminator, appears nowhere they read. One clause each.

### REV-024 - minor · P3 · new
**`report/report.md:436` states the superseded rule** ("at ENTRY, not its completion status at
exit"). Known, deliberately left to ⑤'s re-reconcile; recorded so it cannot be lost between
phases, since the walk-back off `awaiting-uat` exists precisely to stop that bundle shipping.

## Verdict

**CHANGES** - one open major. The restoration itself is correct and well-argued: the four §④
sites, the two docs and `coding-rules.md` all say the right thing with the right rationale, the
skip-count claim was retired honestly rather than renumbered, the `state.md` walk-back is the
right call and verified against the live reader, and the Go really is comment-only. The
hand-off blink is the right trade - accept it, do not narrow it. What blocks is **REV-021**:
three always-read knowledge statements still describe the removed exit write, one of them
instructing a future reviewer to flag the restoration as a defect. Fix that sweep (plus the
three minors, of which REV-022 is the one with teeth), and this is ready for ④'s single pass
and ⑤'s re-reconcile.
