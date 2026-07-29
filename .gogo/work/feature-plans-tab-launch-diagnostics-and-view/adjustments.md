# Adjustments — feature `plans-tab-launch-diagnostics-and-view`

Changes and clarifications requested during planning, in order. Each entry records what was
asked and what changed in `plan.md`.

---

## A1 — 2026-07-29 · concurrency cap folded in as a third leg

**Asked:** the user suspects the concurrency cap — not tmux — is why they "cannot run new
sessions or work items" (they saw a cap of 1). Their expectation: cap per **source**, many plans
at once, many work items at once in **different** sources, and only **in-progress work items**
counted, never plans. Fold this into the same plan as a third leg; verify against the code and
decide, with evidence, what is a genuine defect vs working-as-intended.

**Changed in `plan.md`:**
- Added **Leg 3** to the Goal, a "the user's model is already the implementation" section to
  Context (with probe output), and **FR3.1-FR3.4**.
- Recorded the verification: the cap **is** per-source (`CapForSource` matches `s.Path == root`),
  counts only `ClassInProgress` + live-session features scoped to that root
  (`ActiveWorkSlugs`), structurally cannot count plans (they are `plans.Plan`, not
  `contract.Feature`), and **no plans-tab path calls `capBounce`** — all confirmed by running the
  real helpers. `DefaultConcurrentWorkItems = 1` is deliberate and already self-documented in the
  config form.
- Recorded the two genuine defects found beside it: **dangling plan targets silently dropped**
  after the confirm (reproduced; matches the user's live `dotai/plan-1948afcd`) and **one
  typographic voice for every status outcome** (`statusStyle` is `Faint(true)` for success,
  cap bounce and failure alike).
- Added **D5** (cross-repo same-slug over-count — reproduced, recommend defer) and **D6**
  (one release vs three slices, with a slice order) so the size question is a decision rather
  than a silent split.
- Stated plainly that the cap default stays **1** and the cap model is **not** redesigned.

---

## A2 - 2026-07-29 · FR1.5 corrected during implement: `capture-pane` needs the PANE target form

**Found while implementing** (not asked for): FR1.5 specified `-t "=" + name` for all three
probes. That is right for `has-session` and `kill-session` (session targets) but **wrong for
`capture-pane`**, whose `-t` takes a **pane** target. Measured live on this host (tmux 3.7b):

```
capture-pane -t "=gogo-x"    -> can't find pane: =gogo-x      (would break EVERY log peek)
capture-pane -t "=gogo-x:"   -> ok, and refuses to match gogo-x-long
capture-pane -t "gogo-x"     -> READ gogo-x-long's pane        (the hazard FR1.5 closes)
```

**Changed in the code + `plan.md`:** `launch.exactPaneTarget(name)` = `"=" + name + ":"` is used
by `CapturePaneArgs`; `exactTarget` (the bare `=`) stays for `has-session` / `kill-session`. FR1.5
in `plan.md` carries the correction inline. The plan's *intent* is unchanged and now actually
holds - verified end-to-end through the real `launch.CapturePane` against scratch tmux sessions.

Also recorded (no behaviour change): `MaxTmuxCommandBytes` stays the plan's pinned **16317**, but
an independent bisection at implement time put the real boundary at **16363** accepted / 16364
refused under this package's byte accounting, and confirmed the *whole* command line is what is
bounded (a 28-char-longer session name moved the payload boundary by exactly 28). 16317 is
therefore ~46 bytes conservative, which is the safe direction: over budget means fold-to-pointer,
never a lost brief. The measurement is in the constant's doc comment.

---

## A3 - 2026-07-29 · review round 01: the fold reaches the headless doors too (REV-002)

**Found by review, scope call made by the orchestrator.** `plan.md`'s Changes checklist scoped
`FoldToPointer` to the three `internal/tui` sites and did not list `cli/plan.go`. But
`gogo plan go` (plan.go planGo) and `gogo plan promote` build the **identical**
`launch.PlanIntent`, so they blew the identical budget: measured **20 951 bytes** against the
16 317 limit on the user's real plan shape. The new preflight caught it with a good typed error,
but that is D1's **rejected option B** - the user blocked on a limit they did not create - and
`README.md` advertises `gogo plan go` as the scriptable equivalent of the cockpit move.

FR1.3 is phrased as a property of the *launch*, not of a key binding, and D1=A is the decision
for over-budget launches generally. So the checklist omission was treated as a gap in the plan's
coverage, not a scope boundary.

**Changed:** the same two-line seam at both doors (`intent.Root = src.Path` so the budget is
measured against the real anchor, then `launch.FoldToPointer(intent, plans.Path(project, id),
<source>)` after `SkipParams` so the params survive the fold). Covered by `cli/plan_fold_test.go`
driving the real `cmdPlanStore` through the injected `planLauncher` seam.

**Under-budget parity re-verified after the change** (the property D1=A hangs on): the 0.27.0
`launch` package and the shipped one were compiled side by side and their produced argv diffed
over 8 representative under-budget intents. The launched **command** element is byte-identical in
all 8. The only delta anywhere is the `-s` session NAME for a title past 48 chars, which is
FR1.4's specified cap.

Two smaller review corrections worth recording beside it:

- **REV-001** - the FR1.4 dash-boundary cut had no floor, so a realistic title
  (`"Refactor NotificationDeliveryOrchestrationPipelineForRealtimeEvents"`) collapsed to
  `"refactor"`. Two plans sharing a first word then minted the same session base - the TEST-005
  attribution ambiguity, arriving through a lossy transform instead of a substring match. The
  boundary is now honoured only past `MaxSessionLabel/2`.
- **REV-008** - `AttachArgs` moved to the exact target form as well, but only **after** live
  verification of both branches on tmux 3.7b (the A2 discipline): `attach-session -t "=<exact>"`
  attaches and `"=<prefix>"` is refused; `switch-client -t "=<exact>"` returns rc=0 and moves the
  client, `"=<prefix>"` is refused, and a **bare** prefix resolved to a *different* session - the
  hazard, reproduced. Testing `switch-client` needed a control-mode client (`tmux -C attach`,
  addressed by `#{client_name}`); the host's only real client is the user's live session and was
  never touched.

---

## A4 - 2026-07-29 · review round 02: FR1.4's cap is read-side back-compatible (REV-009)

**Found by review; the transition would have shipped otherwise.** FR1.4's `MaxSessionLabel` cap
applies to the **slug side** of `SessionMatchesSlug` as well as the name side, so any session a
**pre-0.28.0** gogo minted with a >48-char label stops matching the moment the user upgrades.
Verified against the host's real running session (read-only `tmux list-sessions`):

```
gogo-plan-catalogue-side-of-the-matching-engine---normalise-store-embed-hard-filter   83 chars
  0.28.0 mints:  gogo-plan-catalogue-side-of-the-matching-engine
  -> SessionMatchesSlug == false  (before the fix)
```

That is not cosmetic. The lost attribution means `ActiveWorkSlugs` **under-counts** the running
build, so the per-source cap would let a second build start in that repo and clobber the shared
working tree - the exact safety property Leg 3 exists to protect, reopened by an upgrade
transition. The affected session was the plan this feature was built for.

**Changed:** `SessionMatchesSlug` now matches against **both** label transforms - the bounded one
0.28.0 mints with and the pre-0.28.0 unbounded one (`unboundedLabel`; `sanitizeLabel` is now that
plus the cap). This is a **read-side compatibility widening, not a relaxation of FR1.4**: minting
stays bounded, so no new long name is ever created. It cannot reintroduce REV-001's ambiguity
because each candidate is still compared as a WHOLE base (exact, or base + a purely-numeric
suffix), so adding a candidate can never turn a prefix into a match - pinned by 7 non-match cases
including TEST-005's originals. Both of the host's real sessions now attribute correctly.

Two smaller round-02 corrections beside it:

- **REV-010** - REV-007 had closed only the selection arm; `M` on a plan-pending card still
  claimed to force past a cap the **accept** arm never consults. The fix is now general and
  *shorter* than the clause it replaces: ask the guard what the force overrode
  (`attemptActionForce(ship, false)`) instead of enumerating the arms that do not consult the
  cap. Correct for every arm by construction, with no list to keep in sync.
- **REV-012** - the doc drift after A3 (`cli/plan.go` changed, so "all TUI/`launch`-side" was
  false). Fixed in `docs/cli-contract.md` + `README.md`, and while there two further drifts of
  the same class were corrected: both docs still listed only three exact-match probes (stale
  after REV-008 added `attach-session`/`switch-client`) and both described `capture-pane` with
  the bare `-t "=<name>"` form when it needs the PANE form `-t "=<name>:"` - the **A2 correction
  had never reached the docs**.

**Method note (adopted from the reviewer).** The mutation harness now runs `go build ./...`
**before** the suite for every mutation, so a mutation that fails to compile is reported as
`BUILD-FAIL` rather than counted as a bite. A mutation count produced without compile-checking is
not trustworthy. Under that method: **19/19 reverts compile and fail the suite** (the 4 round-02
fixes, 4 additional round-02 sub-properties, and all 11 round-01 fixes re-swept for regression).

---

## A5 - 2026-07-29 · test round 01: `m` -> Enter confirms on the plans tab (TEST-001)

**The user's call, in their words: "m -> enter should confirm."** The cause was a missing seed,
not a design choice: `move.go`'s `startFormOverriding` (the board's `m`/`M`) explicitly sets
`&formBinding{confirm: true}`, so a bare Enter submits, while both plans-tab confirm constructors
built an unseeded `&formBinding{}` - Go's zero value means Cancel. The same keystroke that
launches on the board therefore **silently cancelled** on the plans tab. Pre-existing since the
0.25.0 plans-board work; three existing tests had been quietly working around it by overriding
`m.binding = &formBinding{confirm: true}` by hand.

**Changed:** `confirm: true` seeded at `startPlanSpawnForm` and `startPlanDoneForm`.

**Deliberately NOT changed** - `startDeleteForm` (delete.go) and `startKillForm` (update.go) keep
their explicit `confirm: false`. A destructive or irreversible action must stay safe on Enter.

The distinction is now **written down as a named convention** rather than left implicit, because
it looks like an inconsistency to anyone who has not read both sides. The canonical statement
lives at `move.go`'s `startFormOverriding`:

> **CONFIRM-DEFAULT CONVENTION.** A **forward pipeline move** (launch / spawn / accept - the `m`
> family) seeds `confirm: true`, so **Enter submits**. A **destructive** action (delete, kill)
> seeds `confirm: false`, so **Enter is safe** and the user must arrow over. The asymmetry IS the
> rule; never align the two families for consistency.

Pointer comments at the four other sites name the convention and say "do not flip this".

**Tested by outcome, not by inspection.** `confirm_default_test.go` drives a **bare Enter** through
the real huh lifecycle (`keyPress` pumps the async `NextField`/`nextGroup`/`StateCompleted` chain)
and asserts the real side effect at every site: board `m` launches; plans-tab `m` spawns (launcher
fires once, plan flips `active`); plans-tab `m` accepts the project-UAT (plan flips `done`); a bare
Enter on **delete** leaves the feature on disk and trash empty; a bare Enter on **kill** calls the
killer zero times. A third test derives each constructor's seeded default from the source and fails
on an unseeded `&formBinding{}` - the exact shape that caused this. **6/6 mutations
(compile-checked first) fail, each in the expected test.**

**Two round-02 test fixtures fixed while here** (found by the same "does it pass for the right
reason?" question): `TestProjectUATRefusalIsBlocked` and `TestFinishPlanDoneRefusalIsBlocked`
omitted their member features' `Correlations`, so `memberFeature` never resolved them and both
refused because the member was **not found** rather than **not shipped**. The fixtures now carry
the plan id, and the refusal asserts the exact `1 of 2` tally so a fixture that stops resolving
its members fails loudly. Mutation-confirmed: removing the *shipped* member's correlation now
fails the test.
