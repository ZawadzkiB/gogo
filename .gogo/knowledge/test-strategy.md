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

### Live TUI testing (tmux) + the Go-TUI two-layer strategy — since 0.9.0/0.10.0
The interactive path is AUTOMATABLE (tmux send-keys / capture-pane) and unit
tests alone are NOT enough for the TUI — never sign off an interactive flow that
has not been driven to completion with real keystrokes; TTY-only defects are
invisible to `go test`.
**Load when:** hands-on testing / reviewing the gogo TUI or any interactive flow → `../skills/tui-tmux-testing/SKILL.md`

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

### Assertion vacuity — the fourteen-variant catalog (0.28.0 + 0.29.0)
Fourteen distinct ways an assertion (or the mutation sweep itself) looks like a
check while checking nothing. The standing rule stays above (mutation IS the
coverage check); the catalog is the on-demand reference.
**Load when:** writing/reviewing test assertions or auditing a mutation sweep → `../skills/assertion-vacuity-catalog/SKILL.md`

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
