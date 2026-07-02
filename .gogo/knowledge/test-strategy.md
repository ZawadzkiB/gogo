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
