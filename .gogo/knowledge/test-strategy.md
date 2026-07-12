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
- **Exercise the vendored tool headlessly** — `python3 assets/kanban/board.py
  --selftest` and `--headless --ship a,b` assert the exit-code contract
  (0 confirm / 1 cancel / 2 error) and the ready-only guard without a terminal.
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
  `tmux new-session -d -s "gogo-test-board-$$" "python3 assets/kanban/board.py --index <idx> --result <res>"`.
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

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
