# Decisions — feature `xplan-board-and-simple-done`

## D1 — Release hygiene: commit 0.8.0 + 0.9.0 before this feature?

**Question:** the working tree holds two uncommitted, report-complete releases
(0.8.0 merged-entries, 0.9.0 TUI cockpit) — and this feature DELETES the 0.9.0
TUI. Commit first, or fold?
- **A (recommended):** commit + push the current state first (one commit, tag
  v0.9.0), then build this as **0.10.0**. The TUI's history stays in a frame
  (its reports/changelog references stay truthful); the removal is a clean diff.
- **B:** fold — never commit the TUI, renumber. Messy: five reports, knowledge
  files, and plugin.json already reference 0.8.0/0.9.0.

**Recommendation:** A.

**RESOLVED (2026-07-02):** **A** — commit + tag v0.9.0 first; this feature = 0.10.0.

## D2 — Board server lifetime

**Question:** how long does `server.py` run, and how do ships come back?
- **A (recommended):** long-running server + polling board: server stays up,
  `POST /api/ship` writes the intent file; the orchestrator (background watcher)
  ships via the synthesis writer, rebuilds the work-index; the board polls
  `GET /api/board` and the card visibly moves to changelog. Multiple ships per
  session, live feedback.
- **B:** single-shot — server exits on the first ship intent (the 0.9.0 pattern
  over HTTP); board goes stale after one action.

**Recommendation:** A — the whole point of the browser surface is staying open.

**RESOLVED (2026-07-02):** **A** (long-running + live refresh).

## D3 — Page pre-build

**Question:** when are the view pages built?
- **A (recommended):** pre-build ALL items' pages at `/gogo:xplan` launch —
  server stays static/dumb (stdlib), view buttons always work offline.
- **B:** build on demand — needs the orchestrator in the loop per click.

**Recommendation:** A (rebuild after each ship for the affected items).

**RESOLVED (2026-07-02):** **A** (pre-build at launch).

## D4 — Where the React build happens

**Question:** the user allows a full npm/React build — what do plugin users need
at runtime?
- **A (recommended):** ship **source + committed `dist/`** — npm/node is a
  DEV-TIME dep (gogo contributors rebuild when the board changes); plugin users
  serve the static `dist/` with the python3 stdlib server (one soft dep, offline,
  no install step at use time).
- **B:** build at first `/gogo:xplan` run (`npm install && npm run build` on the
  user's machine) — no committed artifacts, but runtime needs npm + network and
  the first run is slow.

**Recommendation:** A.

**RESOLVED (2026-07-02):** **A** (committed dist/, npm dev-time only) — accepted with the plan ("Accept (all recs)").

## Non-forks (user-stated, recorded)

- **Multiple selection = ONE merged entry** — in the `/gogo:done` list AND the
  board; the 0.8.0/0.9.0 separate-vs-merged gate is removed (user: "if selected
  multiple items that means we want to merge them as one changelog item and
  thats all").
- **The TUI goes away entirely** (board.py, tmux machinery) — no slimmed
  leftover.
- **Columns:** plan · in progress · ready · changelog (classifier classes 1:1).
- **`/gogo:xplan`** is the command name; the board copies xplan's board view.

## D5 — REV-007: Host/Origin residual (orchestrator-resolved, noted)

Review flagged the missing Host/Origin check on server.py as a conscious
accept-or-harden. **Resolved: harden.** A Host allowlist (127.0.0.1/localhost)
plus an Origin same-or-absent requirement on POST is ~10 lines, is NOT the auth
the plan scoped out, costs zero UX, and closes the DNS-rebind/CSRF residual.
Not escalated: cheap, strictly better, no trade-off.
