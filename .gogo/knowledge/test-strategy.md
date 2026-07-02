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
- **UI** — Playwright MCP: *target* projects via gogo-tester, and — since 0.10.0 —
  the plugin's own `/gogo:xplan` browser board (see overrides).

## Done-bar
- The changed command(s) run end-to-end on a scratch repo.
- Artifacts conform to their contract; bad inputs are rejected, not propagated.
- All enumerations in sync (grep); version bumped; portability intact.
- For a full feature: review clean + tests green → `report.md` + as-built charts.

## gogo overrides
<!-- Preserved across re-runs. -->

### Soft-dep degradation (the /gogo:xplan browser board) — since 0.10.0
A soft-dep surface must always degrade to a path that is testable without the dep:
- **Run the fallback for real** — the `/gogo:done` status table + `AskUserQuestion`
  multi-select is the live path when `python3` is absent; dogfood it on a fixture with
  every work-index class (add a plan-only `unfinished` exemplar).
- **Code-read the degradation routing** — a missing `python3`, a busy port, and an
  un-openable browser must each route gracefully to the stated fallback, never hard-fail.

### Testing the browser board — since 0.10.0
The `/gogo:xplan` board is a normal web app (committed React `dist/`) + a stdlib HTTP
server, so the interactive path is driven directly at three seams:
1. **Server, headless** — `python3 assets/xplan-board/server.py --selftest` asserts the
   exit-code contract (0 pass / 2 bad args or selftest fail) and the guards offline:
   intent validation, ready-only, path-traversal, Host/Origin.
2. **Server, live (curl matrix)** — start it on a **scratch `--data` fixture** and assert
   the API contract: `GET /api/board` 200; `POST /api/ship` 202 valid (intent file
   written) / 400 non-ready or bad shape / 409 intent pending / **403** on a non-localhost
   `Host` or a cross-site `Origin`; traversal probes (`/view/../..` + encoded variants)
   contained — 404, never a leaked file. Kill via `server.pid`; SIGTERM must remove it.
3. **Board UI (Playwright MCP)** — drive the real React board against the fixture server:
   columns render from `board.json`, the filter narrows live, "view" opens a page,
   checkbox + "Mark done" and drag ready→changelog both fire the POST and the card moves
   to changelog after the next poll, an illegal drag bounces (no POST), and the toast
   lifecycle holds (persistent shipping toast, error toasts, the watchdog).

Clean up: kill the server (`kill $(cat .gogo/resources/xplan-board/server.pid)`); write
fixtures to the scratchpad only; `__pycache__`/`node_modules` are gitignored. (The 0.9.0
tmux/curses TUI path — `tmux send-keys`/`capture-pane` — was retired with the TUI.)
