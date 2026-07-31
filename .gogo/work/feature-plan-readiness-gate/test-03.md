# Test round 03 — plan-readiness-gate (ships as 0.29.0)

**Track:** test (④) · **Round:** 3 · **Date:** 2026-07-30

Scope per the coordinator: the exit-write restoration (phase ⑤ found FR11 had, as an
unintended side effect, removed the pre-existing exit write from
`gogo-implement`/`gogo-review`/`gogo-test` §④; the user's call was to restore it and
keep the entry write) and its fallout — REV-022's gate scoping, the resulting "blink",
and a regression sweep of round 01's six scenarios. Rounds 01/02 already covered
everything else and are not repeated here.

## Environment / isolation

Fresh scratch root (`/tmp/gogo-e2e-r3-<pid>/`, deleted at the end), one scratch source
+ scratch `GOGO_DATA_HOME` project registration for the cap regression check. **Host
safety verified before AND after:** the two protected user sessions present and
unchanged; no bare `gogo sweep` run (only `gogo sweep <scratch-slug>`, twice, both
targeted). Nothing written under `~/.gogo/` or this repo's own `.gogo/work/`.

**One self-caught methodology note, in the spirit of transparency the coordinator's own
message asked for.** Verifying the targeted-sweep-frees-the-slot regression, I ran
headless `gogo go buildslug-b` twice: once while the cap was still blocking it (a safe
refusal, returns before any launch), and once **after** sweeping the blocker — at which
point the cap no longer blocked, and the command proceeded to a REAL, synchronous
`claude -p "/gogo:go buildslug-b"` invocation against the scratch fixture (headless
`gogo go`, unlike the TUI board, has no confirm step). It ran briefly, made no changes
(fixture's `state.md` was unchanged afterward - `plan-accepted`, and the only artifact
left was a session-registry JSON fully inside the scratch tree), and nothing escaped
`GOGO_DATA_HOME`/the scratch repo. But it was a real, avoidable API call I should not
have made; for the rest of this round I switched to using the confirm-gated TUI board
for anything past a refusal check. Noting it plainly rather than omitting it.

## Baseline (re-verified independently)

| Check | Result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | green, 12 tested packages |
| hermetic `go test -count=1 ./...` (minimal `PATH`) | green |
| `gogo --version` | `gogo 0.29.0` |

Matches the coordinator's independently-verified baseline.

## 1. The line actually advances at exit, entry skipped (item 1)

Built one fixture per phase transition, each shaped as "the entry write never landed,
only the phase's own exit write did" (no `phase-started`/`fix-round` entry event for
the CURRENT phase anywhere in `events.jsonl` - only the prior phase's entry+exit, plus
the current phase's own `phase-done`):

| Fixture | state.md after | `gogo status` PHASE / STATUS |
|---|---|---|
| `handoff-plan-implement` | phase: implement | **implement / implementing** |
| `handoff-implement-review` | phase: review | **review / reviewing** |
| `handoff-review-test` | phase: test | **test / testing** |

All three: the line advanced to the current phase with **zero** entry event ever
recorded for it - confirmed for all three phases (not just two), headlessly via
`gogo status` and live on the board (badges `implement r1`/`review r1`/`test r1`,
correctly `· stalled` since no session was attached to these particular fixtures).
**This is the direct proof the restoration works: the floor is restored.**

## 2. The blink - self-clears on a healthy hand-off, stays lit when entry is skipped (item 2)

Two fixtures, both starting identically (`phase: implement`, `status: implementing`,
newest event `phase-done`/`implement`, live `gogo-go-<slug>` session) - arm A's exact
shape:

- **`blink-selfclear`**: both showed `● developer  · state lags` at T0. Then simulated
  the next phase's entry write landing promptly (state.md → `phase: review` /
  `status: reviewing`; appended `phase-started`/`review` to `events.jsonl`). Recaptured
  the live board: **`reviewing  ● reviewer` - no cue at all.** The blink self-cleared
  the moment the telemetry and the phase line agreed again.
- **`blink-stuck`**: left completely untouched (the distinguishing case - entry
  genuinely skipped, nothing else writes to the file). Re-captured the SAME fixture
  twice, ~20 real seconds apart, with **zero** file changes in between:
  `implement r1  ● developer  · state lags` both times, unchanged. The cue does not
  auto-expire - it is pure file state, so "stays lit for a whole phase" is exactly what
  happens when nothing updates the file for that long.

**Both halves of item 2 verified live: self-clear on a real write, persistence with no
write.**

## 3. REV-022's gate scoping (item 3) - two different answers for two different questions

**The exact mechanism the coordinator asked about (does the exit-write TEXT scope away
from a gate status) checks out, verified two ways:**

- **Read all three phase skills' §④**: each carries the identical scoping sentence,
  "**Never overwrite a gate status with the working status**", naming the ⏸ count, the
  stripe, and the `/gogo:go` refusal it protects.
- **Reproduced REV-022's regression live** (see "Independent mutation checks" below):
  temporarily stripped that sentence from `skills/gogo-review/SKILL.md` and confirmed
  `TestPhaseSkillsWriteOccupancyAtEntryAndExit` FAILS with exactly the expected
  message; restored the file byte-for-byte (md5-verified) and reconfirmed PASS.

**Read-side, deterministic confirmation** that a gate genuinely surviving (state.md
correctly reading `waiting-for-user`) behaves safely: built `feature-gate-survives`
(`status: waiting-for-user`, `phase: implement`, `open-decision: D1`). Live board: `⏸
decision` pill, gate stripe, counted in `⏸ 1 need you`. Headless `gogo go
gate-survives` → `"waiting-for-user" - not runnable here. it's paused on a decision -
resolve it and re-accept (→ plan-accepted) first.`, exit 1.

**But driving this hands-on surfaced a different, real gap (TEST-002, new this
round) at the BOARD LAUNCH layer, not the skill-prose layer:** pressing `m` **or** `M`
on `gate-survives` does **not** bounce - it opens a live `will run: claude "/gogo:go
gate-survives" ...` confirmation, identically for both keys. `cli/internal/tui/move.go`'s
`attemptActionForce` switches purely on `f.Class`; its `ClassInProgress` arm never
checks `f.WaitingForUser()` anywhere. `gate-survives` classifies `ClassInProgress`
because its (necessarily stale, since a gate freezes mid-phase) `phase` field still
says `implement` - the ordinary shape of a paused gate, not a contrived one. Two
existing, independent precedents in this SAME codebase already enforce exactly this
property elsewhere - `cli/go.go`'s `cmdGo` (`!orchestrator.RunnableStatus(f.Status)`)
and `internal/tui/pickup.go`'s `autoPickupReady` (`f.Status != "plan-accepted"`
disqualifies a gated member) - so `move.go`'s manual-launch path is the one surface
where the project's own established pattern was not applied. Cancelled both
confirmations without launching; confirmed via `tmux list-sessions` that no
`gogo-go-gate-survives` session was ever created.

**Mitigating factor** (why I did not escalate this to a top-severity blocker): reading
`commands/go.md`, the spawned session's very first documented instruction is exactly
the same acceptance gate ("`waiting-for-user` is NOT runnable here ... Otherwise
STOP") - so a competent session launched from this hole would very likely self-abort
immediately rather than doing anything to the paused feature. But that is prose
enforcement again, and this project's own stated principle
(`code-review-standards.md` check #13: *"a guard that prevents damage must key on a
deterministic signal ... a writer that can skip must be detectable rather than
silent"*) is precisely why I am not treating "the spawned session would probably stop
itself" as sufficient. Filed as **major**, not blocker, given the mitigation; see
`test/issues.json` TEST-002 for the full writeup and proposed fix (a hard-coded
`WaitingForUser()` bounce in `attemptActionForce`, evaluated outside the `!force`
guards like FR4/FR4a, so `M` cannot force past it either).

## Independent mutation checks - verifying the reshaped guards actually bite (not on trust)

Per the coordinator's explicit instruction, reproduced each regression the guards
claim to catch, always with `-count=1` (see note below on why), always restoring the
file byte-for-byte (md5-verified before/after) and reconfirming green afterward:

| Mutation | File | Result |
|---|---|---|
| Remove the exit write's `status: reviewing` line from §④ | `skills/gogo-review/SKILL.md` | `TestPhaseSkillsWriteOccupancyAtEntryAndExit` **FAILS**, exact expected message (the "EXIT" half) |
| Remove the "Never overwrite a gate status" scoping sentence | `skills/gogo-review/SKILL.md` | Same test **FAILS**, exact expected message (the "scope" half) - a SEPARATE assertion from the one above, confirmed to fire independently |
| Remove "Do NOT flag the exit write as duplication" + reintroduce "write sits after the work rather than" (kept on ONE unwrapped line) | `.gogo/knowledge/code-review-standards.md` | `TestReviewStandardsDoNotInviteTheExitWriteRemoval` **FAILS**, both sub-checks fire |

**A methodology catch worth recording**: my first attempt at the third mutation
reintroduced the forbidden phrase but let it wrap across a markdown line break (the
natural shape of an edited paragraph) - `go test`'s **cached** result then reported
PASS, because Go's test cache does not know that `os.ReadFile`-read markdown is a
build input and happily replayed the prior (correct, unmutated) result. Re-running
with `-count=1` (forcing a real re-execution) exposed that the guard's second check
*also* did not fire against the wrapped phrase - not a caching artifact but a genuine
gap in the guard itself (**TEST-003**, new this round, minor/fixable: the `Contains`
check should whitespace-normalize the body first). Every mutation check in the table
above was re-run with `-count=1` to make sure this class of false confirmation could
not recur. This mirrors the coordinator's own caution about the developer's `cd`/`GATES
GREEN` near-miss - worth stating plainly rather than glossing over.

All three files restored byte-for-byte (md5-verified) after every mutation; the full
`skills_lint_test.go` suite re-confirmed green (`-count=1`) at the end.

## 4. Regression re-sweep of round 01's six scenarios (item 4)

`state.go` and the cue logic are shared by all of Slice A, so re-drove each live
rather than trusting the prior rounds:

- **Authoring gate**: verbatim `templates/state.template.md` scaffold (no `plan.md`) →
  `✎ authoring`, slug-fallback title. Unchanged.
- **FR4a**: `m` and `M` both refuse identically (`⚠ plan.md not written yet ...`), no
  confirm form opens. Unchanged.
- **FR8, headless**: `gogo go accepted-noplan` (status `plan-accepted`, no `plan.md`) →
  same refusal, exit 1. Board footer: `[m] ✗ plan not written`. Unchanged.
- **The cap hole**: `buildslug-a` (live `gogo-go-<slug>` session) correctly blocked
  `buildslug-b`'s headless `gogo go`, citing only `buildslug-a`; `planning-c`'s live
  **authoring** (`gogo-plan-<slug>`) session never appeared in the blocking list.
  Unchanged.
- **The targeted-sweep remedy**: marked `buildslug-a` shipped (session still live);
  `gogo sweep buildslug-a` reaped exactly that session; the cap cleared immediately
  after (confirmed - see the methodology note above for what happened next). Unchanged.
- **TEST-001 (`⛓` absence)**: `noplan-probe` (the same verbatim-template fixture)
  showed **no** `⛓` chip at both 200 and 400 columns. Unchanged.

**No regressions found in any of the six.**

## On F06/F07 (the developer's "unkillable by construction" mutations)

I could not locate F06/F07 by id anywhere on disk (not in `review/issues.json`,
`test/issues.json`, or any `review-NN.md`/`test-NN.md`) to check the developer's exact
reasoning verbatim, so this is my own independent judgment from first principles, not
a confirmation of their specific writeup.

**I largely agree, with a caveat that turned into TEST-002.** The property "the LLM
running a phase skill will actually follow the exit-write scoping instruction" has no
possible deterministic runtime witness - there is no Go function representing "the
skill's §④ logic" to unit-test; the only available proxy is the text-presence check
`skills_lint_test.go` already has, and I could not think of a better one. A mutation
that preserves every literal string the guard checks for while subtly changing the
*meaning* a human/LLM would extract from the surrounding prose is, as far as I can
tell, genuinely unwitnessable short of running many real LLM sessions and observing
their behaviour empirically - which is a different kind of evidence (what rounds 1-3 of
this very testing effort, and the feature's own three real live runs that first
surfaced this regression, already provide) than a repeatable CI assertion.

**But "the write-side prose property is unkillable" should not be read as "there is
nothing further to test here" - and TEST-002 is the proof.** The READ side of the same
risk (does something downstream stay safe if a gate status is ever wrong or ignored)
absolutely does have deterministic runtime witnesses, and I found one that is
currently missing: `move.go`'s launch guard. So my answer to "is there a runtime
witness" is split precisely along the line the developer's framing suggests: no,
for the write-compliance property; yes, for an adjacent, currently-unguarded
consequence, which is exactly what TEST-002 files.

## Issues found this round

- **TEST-002** (major, P1, new, fixable) - the board's `m`/`M` never bounces a
  `waiting-for-user` card whose stale phase still classifies `ClassInProgress`; opens a
  real `/gogo:go` launch confirmation instead. See above and `test/issues.json`.
- **TEST-003** (minor, P2, new, fixable) - `TestReviewStandardsDoNotInviteTheExitWriteRemoval`'s
  second check is fragile to markdown line-wrapping (verified: a naturally-wrapped
  reintroduction of the forbidden phrase does not fail the guard). See above and
  `test/issues.json`.
- **TEST-001**: unaffected this round, `verified` carried forward from round 02
  (re-confirmed live again in the regression sweep, item 4).

## Verdict — NOT YET PASS (two new open findings, neither a blocker)

- Build/unit: green (all four gates, independently re-verified).
- Item 1 (line advances at exit, entry skipped): **PASS**, all three phases, hands-on.
- Item 2 (the blink self-clears / stays lit): **PASS**, both halves, hands-on,
  including a real-time persistence check.
- Item 3 (REV-022 gate scoping): **the specific mechanism asked about is fine**
  (verified by reading + an independent mutation test proving the guard bites) - but
  driving it hands-on surfaced **TEST-002**, a real, major, board-launch-layer gap in
  the same safety family. Not a blocker (a documented prose mitigation exists one layer
  down), but real and worth fixing before or shortly after this ships.
- Item 4 (regression re-sweep): **PASS**, all six round-01 scenarios, no regressions.
- Reshaped skills-lint guards: **verified to bite**, not taken on trust - three
  regressions reproduced and caught, one guard fragility found in the process
  (TEST-003).
- `events.jsonl` chronology: not re-checked this round (already confirmed in round 02
  and unaffected by this round's changes).
- No hands-on check was blocked. One self-caught process mistake (an avoidable real
  `claude -p` invocation against a scratch fixture), contained and reported above.

**`test/issues.json`: 2 open/new issues (TEST-002 major, TEST-003 minor), 1 verified
(TEST-001).** Per the done-bar, this round is not clean - recommend looping TEST-002
(and, cheaply, TEST-003) back to ② implement before advancing to ⑤, or the user
explicitly deciding to ship with them tracked as fast-follows. I would not block ⑤ on
TEST-003 alone, but TEST-002 is a real gap in the exact safety family this whole
feature exists to close, found by doing precisely what was asked (driving a decision-gate
round hands-on) - so I'm surfacing it rather than downgrading it myself.
