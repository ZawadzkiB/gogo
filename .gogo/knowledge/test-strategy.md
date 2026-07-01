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
An interactive terminal surface (curses/tmux) can't be driven by Playwright and
often isn't runnable on the dev host (no tmux). Treat the **graceful-fallback path
as the tested path**, and verify the interactive path by other means:
- **Run the fallback for real** — the status table + `AskUserQuestion` multi-select
  is the live path when the soft dep is absent; dogfood it on a fixture with every
  work-index class (add a plan-only `unfinished` exemplar).
- **Exercise the vendored tool headlessly** — `python3 assets/kanban/board.py
  --selftest` and `--headless --ship a,b` assert the exit-code contract
  (0 confirm / 1 cancel / 2 error) and the ready-only guard without a terminal.
- **Code-read the interactive routing** — confirm launch is nesting-safe and that
  launch-failure vs. cancel vs. confirm route to the right outcome; record manual
  steps for a tmux-capable host rather than claiming the TUI was run.
