# Review round 02 — `plans-tab-launch-diagnostics-and-view`

**Date:** 2026-07-29 · **Reviewer:** gogo-reviewer (fresh eyes) · **Scope:** the round-01 fixes
(19 modified files, +954/-143) plus `cli/plan_fold_test.go` · **Contract:** `plan.md`,
`decisions.md` D1-D6, `adjustments.md` A1-A3.

**Gates re-run independently:** `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...`
green (13 packages) · `gogo --version` → `0.28.0`.

**Nothing in the fix report was taken on trust.** Every claim below was re-derived with my own
harness, my own fixtures and my own mutations, written before reading the developer's tests.

---

## 1. The mutation claim — re-derived, and it goes further than reported

I wrote my own harness (fresh tree per mutation, `go build` first so a syntax error can never
masquerade as a passing test, noise from the three path-dependent root tests filtered). **21
reverts**, each applied alone:

| revert | result |
|---|---|
| REV-001 floor → `i > 0` | **BITES** `TestSanitizeLabelBounded` |
| REV-002a drop the fold at `cli/plan.go` planGo | **BITES** `TestCmdPlanGoFoldsOversizedBrief` |
| REV-002b drop the fold at `cli/plan.go` planPromote | **BITES** `TestCmdPlanPromoteFoldsOversizedBrief` |
| REV-003a drop the `if err != nil` branch in `attachOutcome` | **BITES** `TestAttachFailureIsReported` |
| REV-003b re-inline the closure at `attachSession` | **BITES** `TestAttachSitesShareOneOutcome` |
| REV-003c re-inline the closure at `attachFromPeek` | **BITES** `TestAttachSitesShareOneOutcome` |
| REV-004 `planKebab` → old regex | **BITES** `TestPlanKebabMatchesTUITransform` |
| REV-005a `planSlugHint` → old regex | **BITES** `TestPlanSlugHintMatchesSessionTransform` |
| REV-005b drop the fold at the `c` site | **BITES** `TestCreateWorkItemFoldsOversizedBrief` |
| REV-005c drop the fold at the `A` site | **BITES** `TestPlanWithClaudeFoldsOversizedGoal` |
| REV-006 planAcceptUAT 2nd arm → bare | **BITES** `TestProjectUATRefusalIsBlocked` |
| REV-006 finishKill → bare count | **BITES** `TestPartialKillFailureIsReported` |
| REV-006 clean-kill wording changed | **BITES** `TestPartialKillFailureIsReported` |
| REV-007 unconditional override note | **BITES** `TestForceMoveOnSelectionClaimsNoOverride` |
| REV-008a `attach-session` → bare target | **BITES** `TestAttachArgs` |
| REV-008b `switch-client` → bare target | **BITES** `TestAttachArgs` |
| *(round-01 guards, re-checked for blunting)* planViewing esc · viewDrill nil guard · resolveTargets refusal · plans help line · renderStatus · capture-pane pane form · FoldToPointer no-op | **all 7 BITE** |

**The three previously blind spots are genuinely closed, not re-asserted.** `attachSession`'s
FR1.6 branch, `planSlugHint`'s alignment and both TUI fold sites were green-on-revert in round 01
and now fail — and REV-003's fix went further than asked: `TestAttachSitesShareOneOutcome` reads
the two sources and fails if either site *rebuilds the status inline*, so a future copy-paste
cannot re-open the hole. That is the right shape for this class.

**Four reverts still leave the suite green** — all in REV-006's classification work, raised as
**REV-011**. One caveat on the fix report's arithmetic: my first pass mis-scored one of these as
blind when it was really a syntax error in my own mutation. Compile-checking every mutation is
what separated the two, and I would not trust a mutation count produced without it.

## 2. Under-budget byte-for-byte parity — re-derived end-to-end through the real CLI doors

Rather than re-run the developer's assertion, I built the comparison at a level their test cannot
reach: I checked out the **0.27.0 (HEAD) `cli` module** and the **shipped** one side by side, put
the *same* probe in both, and drove the **real `cli/plan.go` doors** — `cmdPlanStore("go", …)`
fan-out and `cmdPlanStore("promote", …)` — through the injected `planLauncher` over 8 under-budget
plans, dumping the full `TmuxNewSessionArgs` for every launch. 24 launches per tree. Inputs were
chosen to break things: shell metacharacters (`$(whoami)`, backticks, `;`, `*`), an embedded
newline, unicode, a `" - "` title, trailing dashes, and both REV-001 degenerate titles.

**Result: the launched `Command` argv element is byte-identical in 24/24.** The only differing
lines are 6, and every one differs solely in the `-s` session-NAME element, on the two >48-char
titles:

```
< …"-s" "gogo-plan-refactor-notificationdeliveryorchestrationpipelineforrealtimeevents"…
> …"-s" "gogo-plan-refactor-notificationdeliveryorchestrationpipeli"…
```

That is FR1.4's specified cap doing exactly what REV-001 asked for — a 48-byte hard cut, no
trailing dash, no collapse to `refactor`. Nothing else about a launch moved.

**`Intent.Body` and `Intent.Root` cannot reach an argv or a serializer** — re-proved structurally
on the current tree, not inherited from round 01. `TmuxNewSessionArgs` and `TmuxPersistentArgs`
read exactly `in.Session` and `in.Command` and nothing else; the other six argv builders read no
`Intent` field at all. `Intent.Body` is referenced in exactly four places, all inside
`FoldToPointer`. `Intent.Root` is read only by `intentFits`, `pickup.go`, and `move.go`'s
confirm/root/skip lookups — never spliced into an argv. No `json.Marshal` / `Encoder` / `gob`
anywhere touches an `Intent`. The new `intent.Root = src.Path` at the two CLI doors is consumed
only by the budget measurement; the launcher still receives `src.Path` explicitly, which the
24/24 parity result confirms empirically.

## 3. REV-008 / `AttachArgs` — shipped form confirmed, live sessions untouched

Shipped source, both branches:

```go
func AttachArgs(session string) []string {
	if os.Getenv("TMUX") != "" {
		return []string{"switch-client", "-t", exactTarget(session)}
	}
	return []string{"attach-session", "-t", exactTarget(session)}
}
```

`exactTarget` is `"=" + name` — the bare **session** form. **No trailing `:`** (the pane form) on
either branch, and `TestAttachArgs` asserts that property explicitly as well as pinning both argv
shapes; reverting either branch fails it. The live tmux verification is recorded in A3 with the
reproduced hazard, which is the A2 discipline applied correctly.

**Live-session safety, checked rather than assumed.** A read-only `tmux list-sessions` shows the
user's two sessions present and intact:

```
gogo-author-for-gogo-project-lets-add-few-new-tasks-to-plan
gogo-plan-catalogue-side-of-the-matching-engine---normalise-store-embed-hard-filter
```

I ran no other tmux command. And nothing in the module's test suite *can* touch them: grepping
every `_test.go` finds no call to `launch.Launch`, `launch.LaunchPersistent` or
`launch.KillSession` and no `exec.Command("tmux", …)` — every launcher and killer path goes
through an injected seam. The only real exec in the new tests is `sh -c "exit 1"`. One
consequence for the *first* of those sessions is a genuine finding — see **REV-009**.

## 4. Fix-induced checks

- **`finishKill` both paths** — verified by driving the real `finishKill` with a stubbed killer,
  not by reading the test. Clean 2 → `"killed 2 sessions"`, clean 1 → `"killed 1 session"`, both at
  `statusLevelOK` with **no marker in `View()`** — byte-for-byte with pre-change. Partial →
  `"killed 1 session, 1 failed: tmux kill-session failed: exit status 1: can't find session:
  gogo-go-b"` at `statusLevelErr`, rendering the error marker. Total failure likewise. Both
  directions are pinned: mutating the failure wording *and* mutating the clean wording each fail
  `TestPartialKillFailureIsReported`.
- **One slug transform, no third copy** — grepped the whole module. Exactly one label transform
  survives (`sessionUnsafe` in `launch.go`); `plan.go:678` and `plans_tab.go:51` both delegate to
  `launch.SlugFromLabel`. The three other regexes found are the CLI slug-arg validator, the plan-id
  sanitizer and a diagram helper — unrelated.
- **The CLI fold sites pass the right arguments** — `planGo` passes `plans.Path(project, p.ID)` and
  section `target`, the same key `BriefFor(p, target)` was called with two lines above, matching the
  TUI fan-out exactly. `planPromote` passes section `sname`, which `sourceInProject` returns as the
  source's canonical **name** — so a user who passes a *path* still gets a pointer naming the
  `### <name>` heading the plan actually uses. Both correct.
- **A3's scope call** is treated as in-contract, as instructed; `cli/plan.go` is now in scope for
  0.28.0. One doc consequence was missed — see **REV-012**.

---

## Round-01 findings — all 8 closed

| id | sev | status | evidence |
|---|---|---|---|
| REV-001 | major | **verified** | floor present; degenerate titles hard-cut at 48 and stay distinct; confirmed independently in the 24-launch parity dump |
| REV-002 | major | **verified** | both doors fold; 24/24 argv parity through the real doors; section/path args correct |
| REV-003 | minor | **verified** | `attachOutcome` package-level, shared by both sites; 3/3 mutations bite incl. the new anti-drift guard |
| REV-004 | minor | **verified** | one transform module-wide, no third copy |
| REV-005 | minor | **verified** | all three round-01 green-on-revert cases now fail |
| REV-006 | minor | **verified** | all four sites classified; `finishKill` verified on four paths by driving the real function (test gap → REV-011) |
| REV-007 | minor | **verified** | selection arm closed and reproduced clean (second arm → REV-010) |
| REV-008 | minor | **verified** | bare `=` session form both branches, no pane colon; live sessions intact |

## New in round 02

| id | sev | pri | status | title |
|---|---|---|---|---|
| REV-009 | minor | P2 | new | A pre-0.28.0 live session with a >48-char label loses its board attribution after the upgrade |
| REV-010 | minor | P2 | new | `M` on a plan-pending card still claims to force past a cap the accept arm never consulted |
| REV-011 | minor | P2 | new | Three of REV-006's four classifications are unguarded; the "other arm" test block re-tests the same arm |
| REV-012 | minor | P2 | new | `docs/cli-contract.md` still calls 0.28.0 "all TUI/`launch`-side" after `cli/plan.go` changed |

### REV-009 — a pre-upgrade live session loses attribution · minor · P2

The cap changes the **slug** side of `SessionMatchesSlug` as well as the name side, so sessions
minted by 0.27.0 with an uncapped label stop matching. Verified against the user's actual running
session:

```
live session      : gogo-plan-catalogue-side-of-the-matching-engine---normalise-store-embed-hard-filter (83)
0.28.0 would mint : gogo-plan-catalogue-side-of-the-matching-engine
SessionMatchesSlug(live, title) = false
```

The moment the user runs 0.28.0, that session shows no ● dot, `a` says "no running session", `l`
falls back to the log, and the cap under-counts it — the exact degradation FR1.7 removes, arriving
as an upgrade transition. Their other live session (46-char label) still matches (`true`). It
self-heals on relaunch and destroys nothing, so it is a minor — but it is worth a deliberate call
because the one affected session is the plan this feature was built for. Fix options in the issue:
widen `SessionMatchesSlug` to accept the pre-cap base too (~6 lines, a strict widening of an exact
match, TEST-005 untouched), or accept it with a changelog line.

### REV-010 — the same wrong-confirm class, on the accept arm · minor · P2

REV-007's gate is `force && !isShip && len(m.selectedFeatures()) == 0`, but the ClassUnfinished arm
returns the **accept** intent before any `capBounce` — the code's own comment says "Accept is
uncapped" — and an accept has `isShip=false` and no selection. Reproduced (busy in-progress + live
session, `pending` at `awaiting-plan-acceptance`, cap 1, cursor on `pending`, press `M`):

```
┃ will run: claude "/gogo:accept pending"  in tmux session gogo-accept-pending
┃ FORCING past the source cap - cap 1 reached in web - already building busy (th
```

The general fix is simpler than the current clause: ask the guard what the force actually
overrode — `if force { if _, _, b := m.attemptActionForce(ship, false); b != "" { override = b } }`
— which is correct for every arm by construction, with no list to keep in sync.

### REV-011 — three classifications unguarded, and a test that covers less than it claims · minor · P2

Reverting `statusBlocked` → bare `m.status` at `plans_tab.go:398` (planAcceptUAT's **no-members**
arm), `plans_tab.go:458` (finishPlanDone) or `update.go:167` (headless analyst) leaves the whole
suite green; so does reverting `attachSession`'s `setStatus`. The no-members gap has a concrete
cause — `TestProjectUATRefusalIsBlocked`'s second block is commented *"The OTHER arm (no members at
all) must agree"* but reuses the same seeded home, so the active column holds both plans and
`planCardIdx[2] = 0` lands back on the 2-member one. Probe output:

```
ACTIVE column holds 2 plan(s); cursor is on plan-8a8cbbad ("Cross-repo migration") with 2 member(s)
status = "refusing — 2 of 2 member(s) not shipped: web:web-item, api:api-item"
```

This is REV-003's shape recurring inside a different fix — a comment claiming coverage the fixture
does not deliver — so it is worth closing properly rather than leaving as a known-thin spot.

### REV-012 — a doc line the REV-002 fix made stale · minor · P2

`docs/cli-contract.md:85` still opens the 0.28.0 note with *"Three linked fixes to the cockpit, all
TUI/`launch`-side."* `cli/plan.go` now changes too, and with it the runtime behaviour of two
documented commands. The adjacent *"No command-surface change (the CLI enum-sync is untouched)"* is
still accurate — no verb, flag or output shape moved — so this is description drift, not an
enum-sync break. `README.md`'s launch-diagnostics bullet has the same gap: it presents the fold as a
cockpit behaviour and never says the scriptable doors do it too, which is precisely the reassurance
a user hitting this on the CLI wants. No other `docs/*.md` or skill describes it (grepped), so two
sentences close it.

---

## Plan fidelity

All 11 FRs are built and now all 11 are guarded. The A2 (capture-pane pane target) and A3 (headless
doors, floor, AttachArgs) corrections are real, correctly implemented, and each recorded with its
measurement. Nothing unplanned crept in; the deliberate out-of-scope items (D4's session recording,
D5's cross-repo over-count, `--plan-file`, the cap default) are still untouched. The `~/.gogo/`-only
write invariant holds and is asserted against a real temp source dir. No new em dash in any added
line.

---

**Verdict: APPROVE** — no open blockers and no open majors. Both round-01 majors are independently
verified fixed, and the highest-risk property in the change (under-budget byte-for-byte parity)
holds 24/24 through the real CLI doors after the new wiring. The four round-02 findings are all
minors, all AGENT-FIXABLE, and carried forward in `issues.json` for batching; **REV-009 is the one
worth a deliberate call** before release, because it will be visible on the user's own running
session the moment they upgrade. Advance to phase ④ test.
