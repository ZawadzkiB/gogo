---
name: tui-tmux-testing
description: >-
  Use for HANDS-ON testing of the gogo TUI/cockpit (or any curses/Bubble Tea
  surface in this repo): the tmux send-keys / capture-pane runbook, the two-layer
  Go-TUI strategy (model unit tests + live tmux drive), and the TTY-only defect
  classes go test cannot see. Keywords: tmux, capture-pane, live TUI, huh form,
  render assertion.
---

# tui-tmux-testing — driving the real TUI with tmux (+ why unit tests are not enough)

Lifted from `knowledge/test-strategy.md › Live TUI testing via tmux + Go TUI` by /gogo:skills on 2026-08-02.

## When this applies
Phase ④ hands-on checks on the cockpit/board, reviewing a TUI change, or any
task that must prove an interactive flow works with real keystrokes.

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
