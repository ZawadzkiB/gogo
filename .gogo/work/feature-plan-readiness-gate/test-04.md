# Test round 04 — plan-readiness-gate (ships as 0.29.0)

**Track:** test (④) · **Round:** 4 (final pass before ⑤) · **Date:** 2026-07-31

Scope per the coordinator: verify TEST-002/TEST-003's fixes (round 06/07 review, 36
findings, 28 verified + 7 fixed + REV-024 deferred to ⑤) hands-on, plus five specific
user-visible changes since round 03, plus a regression sweep since `move.go`, `view.go`
and `model.go` were all touched again.

## Environment / isolation

Fresh scratch root (`/tmp/gogo-e2e-r4-<pid>/`, deleted at the end). **Host safety
verified before AND after:** the two protected user sessions present and unchanged; no
bare `gogo sweep` run (only `gogo sweep buildslug-a`, targeted). Nothing written under
`~/.gogo/` or this repo's own `.gogo/work/`.

## Baseline (re-verified independently)

| Check | Result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | green, 12 tested packages |
| hermetic `go test -count=1 ./...` (minimal `PATH`) | green |
| `gogo --version` | `gogo 0.29.0` |

Matches the coordinator's independently-verified baseline.

## Item 1 — the footer chip matches what `m` actually does

Built the two specific lying shapes `moveChip`'s collapse fixes:

- **`chip-inprog-accept`**: `phase: review` (stale, in-progress-classifying) +
  `status: awaiting-plan-acceptance`, written plan. Footer: **`[m] accept`**. Pressed
  `m` → opened `will run: claude "/gogo:accept chip-inprog-accept" ...` — chip and
  behaviour agree.
- **`chip-inprog-unwritten`**: `phase: implement` + `status: plan-accepted`, no
  `plan.md`. Footer: **`[m] ✗ plan not written`**. Pressed `m` → `⚠ ... plan-accepted
  but its plan.md is not written ... nothing to build` (FR8). Chip and behaviour agree.

Both previously-lying combinations now tell the truth. Cancelled both confirmations
without launching.

## Item 2 — decision-gate refusal names the right artifact, including the false-positive shape

Three fixtures, all `status: waiting-for-user`:

| Fixture | `open-decision` | `m`/`M` message |
|---|---|---|
| `gate-plain` | `D1` | `... paused on D1 - answer it in **decisions.md**, then run `/gogo:resume gate-plain` ...` |
| `gate-false-uat` (the false-positive shape) | `D4 - uat asked for a different header` | `... paused on D4 - uat asked for a different header - answer it in **decisions.md** ...` — **correctly NOT routed to uat.md** despite the literal substring "uat" in the decision text |
| `gate-real-uat` (a genuine UAT round) | `UAT round 1` | `... paused on UAT round 1 - answer it in **uat.md**, then re-accept the adjusted plan ...` |

`gate-real-uat`'s pill read `⏸ re-planning · UAT 1`, agreeing with its `uat.md`
message. `M` on `gate-plain` refused identically to `m` (checked live). The
digit-after-"uat" rule (`isUATReplan`) correctly discriminates the false positive —
this is the exact regression the coordinator flagged (round 06's substring branch)
and it does not reproduce.

## Item 3 — selection persistence, both directions, plus the REV-035 judgment call

Built `feature-shipme` (`ClassReadyToShip`) in the ready column:

1. **Selected it** (`space`) → `✓ shipme`.
2. **Ordinary reload** (touched an unrelated file in the same feature folder to fire
   fsnotify, `state.md` untouched): selection **survived** — still `✓ shipme`. A
   legitimate selection is not lost on a normal refresh.
3. **The REV-035 probe**: truncated `state.md` mid-write (cut it down to `feature:` +
   a bare `phase:` line, no `status:`). The card reclassified `unfinished` and moved to
   the plan column, `ready 0`, **selection dropped** (`m.selected` pruned).
4. **Restored the full `state.md`**: the card returned to the ready column showing
   **`○` (unselected)** — confirmed, per the coordinator's explicit ask, **the stale
   selection did NOT resurrect.**

**My judgment on the coordinator's question ("is the drop worse than the resurrect it
replaced"): no, it is the strictly safer direction, and I would not spend more effort
closing it before shipping.** Reasoning:
- A resurrected selection (the old bug, REV-033) can cause `d`/`m` to **ship a card the
  user did not currently intend to ship** — an active, potentially damaging,
  unattended-looking action.
- A dropped selection (REV-035's finding, still reproducible exactly as described)
  only ever costs the user a re-press of `space` before shipping; per REV-035's own
  bounded-harm analysis, the worst case is a merged ship silently carrying fewer slugs
  or `d` bouncing "only ready cards can ship" — both are safe, recoverable, and
  legible failures, never an unintended action.
- This is exactly the project's own stated principle, already quoted in REV-035's
  finding text: *"Prefer degrading to MISSING over degrading to WRONG"*
  (`non-functional-requirements.md:47`). Dropping is degrading to missing; resurrecting
  is degrading to wrong.

I did not find any code change that eliminates the transient-drop symptom itself (the
`pruneSelection` logic that unconditionally deletes any key absent from the current
read is unchanged) — so if "fixed in round 07" refers to more than accepting/recording
this trade-off, I could not locate it. I am not filing a new test issue for this: review
already triaged it as `minor` with the same reasoning I independently reached, and the
coordinator's own framing ("tell me if it is worse") reads as wanting a judgment, not a
demand for further code change. Recorded here for the audit trail rather than as an
open `test/issues.json` entry.

## Item 4 — `notRunnableBounce` truthfulness

Built four fixtures + a fifth for the true empty-status arm (status `""` alone hits
`Authoring()` first when `plan.md` is unwritten, so a written-plan companion fixture was
needed to reach `notRunnableBounce`'s own `case ""`):

| Fixture | status | Board `m` message | Headless `gogo go` | Agree? |
|---|---|---|---|---|
| `nr-aborted` | `aborted` | `was aborted - no move (illegal)` | `is aborted - nothing to run; reaped any tracked session.` (exit 0) | yes - both say "aborted", neither says "shipped" |
| `nr-done` | `done` | `is already shipped - no move (illegal)` | `is done - nothing to run ...` (exit 0) | yes - `done` is genuinely shipped-equivalent (`Shipped()`); no false claim |
| `nr-shipped` | `shipped` | `already shipped — no move (illegal)` (ClassShipped's own bounce) | `is shipped - nothing to run ...` (exit 0) | yes |
| `nr-nostatus2` (empty status, written plan) | `""` | `has no status on disk - run \`gogo plan nr-nostatus2\` and accept a plan first` | `is "" - not runnable here. run /gogo:plan and accept a plan first.` (exit 1) | yes - same recommended action, board is more specific (names the reason), CLI uses its shared default phrasing; no contradiction |

No falsehood found on any of the four statuses; board and CLI never contradict each
other (mild wording-specificity differences only, never a different fact).

## Item 5 — regression sweep of round 01's six scenarios

Re-drove all six live, since `move.go`/`view.go`/`model.go` were all touched again this
round:

- **Authoring gate**: verbatim `templates/state.template.md` scaffold (no `plan.md`) →
  `✎ authoring`. Unchanged.
- **FR4a**: `M` on the same card refuses identically to `m`. Unchanged.
- **FR8, headless**: `gogo go accepted-noplan` (plan-accepted, no plan.md) → same
  refusal, exit 1. Unchanged.
- **The cap hole**: live `gogo-go-buildslug-a` session correctly blocked `buildslug-b`'s
  headless `gogo go`, citing only `buildslug-a`; `planning-c`'s live authoring session
  never appeared in the blocking list. Unchanged.
- **The targeted-sweep remedy**: marked `buildslug-a` shipped (session still live);
  `gogo sweep buildslug-a` reaped exactly that session (both protected sessions
  untouched); the board's `m` on `buildslug-b` then opened the real launch
  confirmation (cap cleared) — cancelled without launching. Unchanged.
- **TEST-001 (`⛓` absence)**: the same verbatim-template fixture showed **no** `⛓` chip
  at 400 columns (the width the original bug appeared at). Unchanged.

**No regressions found in any of the six.**

## Issues found this round

None new. All three prior findings now closed:

- **TEST-001**: `verified` (carried forward, re-confirmed live again in item 5).
- **TEST-002**: `verified` — the decision-gate launch guard holds for both `m` and `M`,
  on the exact shape it was written for, and its interaction with REV-022 (a gate must
  hold, and must release when legitimately resolved) was implicitly exercised: the
  card stayed refused throughout, and no code path here suggests it could not release
  via `/gogo:resume` (not independently re-tested this round — REV-022/TEST-002's own
  round-03 mutation tests already cover the resume-releases-the-gate direction).
- **TEST-003**: `verified` — independently re-reproduced the exact line-wrapped
  regression this guard was fixed for (`MUTATION TEST R4`, restored byte-for-byte,
  md5-verified): the hardened guard correctly **fails** against a naturally-wrapped
  reintroduction of the forbidden phrase, where the pre-fix guard would have passed.

## Verdict — PASS

- Build/unit: green (all four gates, independently re-verified).
- Item 1: **PASS**, both previously-lying chip/behaviour combinations now agree.
- Item 2: **PASS**, including the specific false-positive shape (`D4 - uat asked for a
  different header` correctly routes to `decisions.md`, not `uat.md`).
- Item 3: **PASS** on both explicit asks (no resurrection; a legitimate selection
  survives an ordinary reload) plus a direct, reasoned answer to the judgment
  question asked (drop is not worse than resurrect — it is the safer, principle-
  consistent direction; not escalated as a new issue).
- Item 4: **PASS**, all four statuses read truthfully, board and CLI never
  contradict each other.
- Item 5: **PASS**, no regressions in any of the six round-01 scenarios.
- No hands-on check was blocked.

**`test/issues.json`: 0 open/new issues; TEST-001/002/003 all `verified`.** Recommend
closing ④ and running ⑤.
