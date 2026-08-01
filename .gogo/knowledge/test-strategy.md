# Test strategy

**Purpose:** how to test a gogo change — what to exercise and the done-bar.

<!-- gogo:meta
Mode: owned
Source: [ ]
Confidence: medium
Generated-by: /gogo:build
-->

## How to verify a gogo change (dogfood)
1. **Install the dev build** — `marketplace add /path` or `marketplace update` +
   `install` + `/reload-plugins`. Confirm the new `version` is active
   (`ls ~/.claude/plugins/cache/gogo/gogo/`).
2. **Exercise the affected command(s)** on a scratch target repo:
   - changed `gogo-build` → run `/gogo:build`, check `.gogo/knowledge/*` wiring +
     `_discovered.md`.
   - changed a phase → run `/gogo:plan` then `/gogo:go`, watch the phase behave.
   - changed diagrams → open `charts/diagrams.html`, confirm the right subject.
3. **Inspect artifacts, not vibes.** Open the produced files and assert they match
   the contract (plan shape, issues-list fields, state transitions, report).
4. **Validation hand-offs (for pipeline changes).** Confirm each command rejects a
   malformed/missing input and produces an output the next command accepts.

## Levels
- **CLI / command** — the primary surface; every command runnable standalone.
- **Artifact** — the markdown/JSON each phase writes (the real "output under test").
- **UI** — only for *target* projects via Playwright MCP (gogo-tester); N/A to the
  plugin itself.

## Done-bar
- The changed command(s) run end-to-end on a scratch repo.
- Artifacts conform to their contract; bad inputs are rejected, not propagated.
- All enumerations in sync (grep); version bumped; portability intact.
- For a full feature: review clean + tests green → `report.md` + as-built charts.

## gogo overrides
<!-- Preserved across re-runs. -->

### Soft-dep interactive surfaces (e.g. the /gogo:done curses TUI) — since 0.7.0
An interactive terminal surface (curses/tmux) can't be driven by Playwright.
When tmux is absent, treat the **graceful-fallback path as the tested path** and
verify the interactive path by other means (when tmux is present, drive the real
TUI — see the 0.9.0 section below):
- **Run the fallback for real** — the status table + `AskUserQuestion` multi-select
  is the live path when the soft dep is absent; dogfood it on a fixture with every
  work-index class (add a plan-only `unfinished` exemplar).
- **(historical — the vendored board.py was retired in 0.33.0;** bare
  `/gogo:done` is an in-chat flow with no headless tool to exercise. The
  headless-selftest pattern below stays valid for any future vendored tool.)
- **Code-read the interactive routing** — confirm launch is nesting-safe and that
  launch-failure vs. cancel vs. confirm route to the right outcome. Recording
  manual steps instead of running the TUI is only the **tmux-absent** fallback —
  when tmux is present, drive the real TUI (below).

### Live TUI testing via tmux (since 0.9.0) — the interactive path is AUTOMATABLE
When `tmux` is present (it is on this dev host), the curses TUI is **not**
manual-test-only: drive it for real with `tmux send-keys` / `capture-pane`
(proven in the 0.9.0 board-cockpit round — guards, filter, per-action intents,
cancel, all asserted live):
- **Launch detached** into a throwaway session on a fixture work-index:
  `tmux new-session -d -s "gogo-test-board-$$" "<the TUI under test>"` (today
  that is the Go `gogo` board; the retired board.py was driven the same way).
  Use a unique per-run session name; NEVER a real session name like `gogo-done`.
- **Send keystrokes** with `tmux send-keys -t <sess>` (keys like `v`, `s`, `m`,
  `g`, `/text`, `Space`, `C-m`, `Escape`, `q`) and **assert the rendered screen**
  with `tmux capture-pane -pt <sess>` (headers, hints, counters, filter line).
  Allow for curses `ESCDELAY` (~1.5 s) after `Escape`.
- **Assert the contract, not just pixels** — after exit check the exit code and
  the emitted intent file (or its documented absence on cancel).
- **Clean up**: kill every test session; write fixtures to the scratchpad only;
  remove `__pycache__` (it's gitignored, but keep runs tidy).

### Go TUI (the `gogo` CLI) — unit tests are NOT enough (since 0.10.0)
The 0.10.0 lesson (TEST-001): the CLI shipped a green 50-test `-race` suite and
two review approvals, yet **every launch form was unsubmittable live** — the
model's Update() dropped huh's async messages, a class of bug no model-level
unit test had exercised. The strategy therefore has two mandatory layers:
- **Model unit tests for logic** — drive `Update()` directly for guards,
  classification, badges, filters; for forms/dialogs, **pump the full command
  graph** (execute returned `tea.Cmd`s, expand `tea.Batch`, re-feed each msg)
  to the terminal state (`huh.StateCompleted`/aborted) and assert an injected
  fake launcher fires exactly once/never.
- **Live tmux driving for integration** — same send-keys/capture-pane method as
  above, against a fixture `.gogo/` tree with a PATH-stubbed `claude`: real
  keystrokes to real completion (submit AND cancel paths), then assert the stub's
  recorded argv + call count and the board's rendered state. **Only this layer
  catches message-routing/focus/lifecycle integration bugs** — never sign off an
  interactive flow that has not been driven to completion with real keystrokes.

- **TTY-dependent behaviour is invisible to `go test`** (no TTY in CI): glamour's
  `WithAutoStyle()` froze the live TUI for 5s per render (termenv OSC query swallowed
  by Bubble Tea's stdin reader) while every unit test passed in ~4ms. Detect terminal
  properties ONCE before the TUI starts; never query the terminal from a render path;
  always include one live tmux drive before shipping a TUI change (TEST-003, 0.10.0).
- **A model-level status assertion is NOT a render assertion (0.16.0 drill-card
  finding).** The rich drill-in shipped with unit tests asserting `Model.status`
  after `a`/`K` — all green — yet `viewDrill()` never rendered that status, so the
  hints/confirmations were **silent no-ops in the live TUI** (a `View()` path the
  unit tests never exercised; the live tmux drive caught it). Rule: whenever a key
  handler sets `m.status` (or any user-visible field), add a test that asserts the
  string appears in the relevant `View()` **output**, not just on the model — and
  new mode/panel must render the status line the way `viewBoard` already does.

### State-machine / UAT-loop testing (since 0.11.0)
The 0.11.0 UAT gate was verified by **spec-executing the state machine
status-by-status** on scratch fixtures; the pattern generalizes to any gogo
state/gate change:
- **Walk every status, both branches.** Build a fixture at the entry state
  (⑤-green → `awaiting-uat`), then execute each skill's instructions literally
  on the accept path AND the issues path (lock → analyst round → re-accept →
  rerun), asserting `state.md` + `events.jsonl` after EVERY transition; reset
  to a snapshot between branches. Include a legacy-shape fixture (pre-0.11
  `status: done`) for every back-compat clause.
- **The one-legal-command property is an explicit test target.** For each
  status, assert which commands REFUSE (quote the refusal text verbatim, then
  spec-execute the gate against the fixture) as well as which one proceeds —
  and check the property at the **classifier layer** too: TEST-004 (0.11.0)
  showed a stale `report/` made a mid-rerun feature classify ready-to-ship
  until the classifier gated on status, not artifact presence.
- **Validate the emitted events line-by-line** against `events.schema.json`
  and check single-owner emission (each transition exactly once, by its owning
  skill) — a structural hand validator suffices when no jsonschema tool exists.

### Reaper / `gogo sweep` testing is HOST-GLOBAL — never whole-board kill (since 0.17.0)
`launch.ListSessions()` lists **every** `gogo-*` tmux session on the whole
machine, not just the repo under test, and `owningFeature` attributes them against
whatever repo the sweep runs in. So a **real killing whole-board `gogo sweep`
(no slug) run from any test harness can reap the user's REAL in-flight sessions**
(they look like orphans against a scratch repo's empty feature list) — including,
potentially, the session driving the very pipeline doing the testing.
- **Prove reaper behaviour with the *targeted* form** (`gogo sweep <scratch-slug>`,
  0.17.0's `Sweeper.Only`): it is safe by construction — it only touches the named
  slug's sessions. Use a clearly-scratch slug (e.g. `kastest-scratch-N`) that can't
  collide with a real feature, create fake `tmux new-session -d -s gogo-go-<slug>`
  sessions, run the targeted sweep, assert only that slug's session died, and
  **`tmux kill-session` any scratch sessions you made** on the way out.
- **Whole-board behaviour → `--dry-run` only** (lists candidates, kills nothing), or
  the Go unit tests with injected `List`/`Kill` (no real tmux). Baseline the host
  session list before/after every experiment (`immediate-kill-at-ship`, 0.17.0).

### Mutation is the coverage check - and the harness needs two rules (since 0.28.0)
"Green suite" says nothing about whether a change is *guarded*. In 0.28.0 review found
**three shipped wirings whose reverts left the entire suite green** (a slug-transform
alignment and two of the three fold call sites), plus a test asserting a **test-local
copy** of the production callback, plus three of four status classifications unguarded.
The check that found them is a revert-mutation sweep - with two rules, both learned the
hard way in that same round:
- **`go build ./...` FIRST, for every mutation.** A mutation that does not compile is
  `BUILD-FAIL`, not a result: the reviewer's own first pass mis-scored one because it was
  a syntax error in the mutation, not a fact about the code. **A mutation count produced
  without compile-checking is not trustworthy in either direction.**
- **A mutation can compile, be semantically valid, and still never reach the assertion,
  because something else compensates.** A fixture that reuses its data home puts the
  cursor back on the previous case; a fixture missing a `Correlations` link makes a test
  refuse because the member was *not found* rather than *not shipped*, so the property
  the test is named for is never exercised. The remedy is not more mutations: make the
  assertion name the **exact reason** (the `1 of 2` tally, the verbatim refusal string,
  the specific style/marker), so a fixture that quietly stops resolving **fails loudly**.
- **Prefer a guard that cannot be escaped.** Where two call sites must agree, assert it
  structurally (read both sources and fail if either re-inlines the decision) rather than
  asserting the behaviour twice - a future copy-paste then cannot re-open the hole.
- **Report the sweep, with counts.** e.g. "24 mutations, compile-checked first, all fail,
  each in the expected test" is a claim a reader can audit; "tests added" is not.

### The FOURTEEN variants of "an assertion that looks like a check and isn't" (0.28.0 + 0.29.0)
Two releases produced **fourteen distinct** ways for a test to look like a check while checking
nothing - **or for the mutation sweep itself to report a false result**. Two were the
reviewer's own harness mistakes, which is exactly why they are here: *a mutation count
produced by a broken harness is not trustworthy in either direction.* Walk this list before
signing off a guard.

**The four harness rules (a wrong sweep is worse than no sweep):**
1. **Compile-check every mutation FIRST, with `go vet ./...` - not `go build`.** A mutation
   that does not compile is `BUILD-FAIL`, not a result. And **`go build` does not type-check
   `_test.go`**, so a mutation to a test file passes `go build` and then fails at test time
   for the wrong reason. `go vet` type-checks tests.
2. **Assert the edit landed via a marker unique to the NEW text**, never via "the anchor is
   gone" - an **insertion whose replacement contains its anchor** trips a naive check and
   reports `EDIT-DID-NOT-LAND` for a perfectly applied mutation.
9. **A nameless CAUGHT is UNSCORED.** A failure with no test name attached is usually a
   compile error in the mutation, not a catch. Re-run it compile-clean before scoring.
10. **Never `&&` after a pipe when the pipe's exit code is the result** - the `&&` sees the
    last stage's status, so a broken mutation reports success. Check the compile step's
    status directly, or `set -o pipefail`.

**The six ways an assertion misses its target:**
3. **A structural guard that matches its own comment.** Grepping source text is satisfied by
   the doc comment *describing* the rule, so deleting the code passes. **Strip `//` comments
   before a structural grep** (`tuiFuncBody`) and pair the structural half with a
   **behavioural** half.
4. **Two styles that render identically under a TTY-less terminal.** A "these differ"
   assertion comparing *rendered* lipgloss strings passes for the right AND the wrong style,
   because colour is flattened. Compare style **properties**
   (`GetForeground()`/`GetBackground()`/`GetBold()`), and make every user-visible cue
   **glyph + word** so `View()` substring matching works without colour.
5. **A fixture whose removal changes no assertion** - decoration masquerading as an input.
   **Mutate the fixture, not only the code**: if deleting a fixture element leaves the suite
   green, that element is not under test.
6. **An exclusivity/invariant assertion that is true VACUOUSLY** - "no two arms overlap"
   passes trivially if the matrix never reaches an arm. **Pair it with a reachability
   guard**; shrinking the matrix must FAIL.
7. **A guard-only mutation can never fail while the production code is correct**, so scoring
   it SURVIVED is meaningless. Use a **two-part mutation**: weaken the guard **and**
   introduce the defect it exists to catch, then check something else still bites.
8. **A guard satisfied by its own producer's body.** Extracting a decision into a producer
   and asserting the producer leaves the **call sites** unguarded - either surface can stop
   calling it and hand-write fresh copy with the whole suite green. **Assert the wiring**:
   the rendered output where the surface is readable, and a structural call-site check where
   the value cannot be read back (a huh field's `Description`, for instance).

**The four found by applying this list to the guards written for it (0.29.0 rounds 04-07):**

11. **A guard that is unreachable because an earlier branch always returns.** A message arm
    was aligned to another surface's terminal case - which that surface never reaches, because
    it returns earlier. Aligning to dead code propagates a falsehood (an `aborted` feature was
    told it had "already shipped"). Check the arm you are matching can actually execute.
12. **A guard matched a substring that its own subject also contains.** `Contains(exit,
    "reviewing")` passed against the regression because the section also carries a
    `phase-done` JSON event with `"status":"reviewing"`. Match the shape of the instruction
    (`status: reviewing`), not a word that appears in the neighbourhood.
13. **An anchor written from memory never lands.** A mutation whose anchor was recalled
    rather than read did not match; the edit silently did nothing and the run reported a
    PASS. Always re-read the bytes you are about to mutate, and verify the edit landed.
14. **A test that pins ONE surface of a shared predicate.** A fix unified three call sites
    behind one predicate; the test asserted only the action path, so reverting the renderer
    or the toggle left the suite green. Mutate EVERY surface the fix claims to unify.

**The shape that recurs:** the thing asserted was **one level away from the thing that
matters** - the producer instead of the wiring (8), the comment instead of the code (3), the
arm instead of its reachability (6, 11), one surface instead of all of them (14). When you
cannot write the test you want, say so instead of writing one that passes for a weaker
reason: 0.29.0's review asked for a test that fails when two disjoint predicates are swapped,
and **no such test can exist** - the honest answer was a disjointness proof plus a
reachability guard.

**Corollary for a guard over a SHIPPED template or asset:** read the shipped file itself, not
a copy, and **first assert the file still contains the hazard** - otherwise the guard passes
because someone deleted the hazard instead of handling it. 0.29.0's TEST-001 guard caught a
literal comment closer in the template's own new warning note within minutes of it being
written.

**Corollary for scoring a GUARD-HARDENING change (the control pair).** Variant 7 says a
guard-only mutation cannot fail while the defect is absent; the practical form is a **control
pair**. Reintroduce the defect in the shape the hardening targets (0.29.0: the forbidden
phrase *wrapped across a line break*), then assert the **hardened** guard fails **and** the
old raw-matching one passes. One run, two data points, and it distinguishes "the guard is
stronger" from "nothing changed". Restore both files byte-for-byte and md5-verify afterwards.

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
