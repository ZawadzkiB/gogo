---
name: gogo-xplan
description: >-
  The /gogo:xplan browser kanban — open the gogo work as a React board (ported from
  xplan: the TONE dark palette, .wf-panel cards, native HTML5 drag-drop) served by a
  python3 stdlib server on localhost. Four columns plan · in progress · ready ·
  changelog are fed by the shared gogo-status classifier plus the changelog entries;
  every card has a "view" button that opens its pre-built HTML page, ready cards get a
  checkbox, and shipping happens by selecting ready cards (or dragging one onto the
  changelog column) — multiple picks = ONE merged changelog entry. It classifies +
  writes board.json, pre-builds all view pages (reusing gogo-view), starts server.py in
  the background, opens the browser, then watches for a ship intent: on each it runs the
  gogo-done synthesis writer, rebuilds board.json + pages, and the polling board moves the
  card live (multiple ships per session). python3 is a soft dep — absent, point at
  /gogo:done's list, never hard-fail. Use when the user runs /gogo:xplan or asks for the
  browser board / kanban. Only ever writes under .gogo/; localhost only, offline.
---

# gogo-xplan — the browser kanban (FR4–FR7)

`/gogo:xplan` puts the gogo work where you can see it: a **React board** (xplan's own
`BoardColumns`/`Card` look, ported) with four fixed columns —
**plan · in progress · ready · changelog** — drag-and-drop, a live text filter, a
**view** button per card that opens its built HTML page, and **mark-done from the
board**. The board is static files (`assets/xplan-board/dist/`, committed) served by a
tiny **python3 stdlib** server (`assets/xplan-board/server.py`, localhost only) that
also exposes `GET /api/board` and `POST /api/ship`. The orchestrator watches the ship
intent, ships via the **gogo-done** writer, rebuilds the board, and the polling board
re-renders — the shipped card **moves to changelog live** (D2=A).

Same intent protocol as the retired 0.9.0 terminal surface, new transport: the classifier
emits, the orchestrator's writer executes — now over browser + HTTP. **npm/node is
dev-time only** (D4=A): the `dist/` is committed, so a user needs only `python3`; a gogo
dev rebuilds with `npm install && npm run build` when the board source changes.

Pure `Read` / `Write` / `Bash` / `Glob` / `Grep` (+ `Skill` to reuse `gogo-status` and
the `gogo-view` build). Only ever writes under `.gogo/`; **localhost only, offline**.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in | the shared **work-index** (gogo-status Step A classifier) | the four-class record shape |
| in | `.gogo/changelog/*/` entries | the shipped release history (changelog column) |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/xplan-board/{dist/, server.py}` | committed React build + stdlib server |
| in (assets) | `${CLAUDE_PLUGIN_ROOT}/assets/{mermaid/mermaid.min.js, viewer/*}` | vendored viewer runtime (via gogo-view) |
| out | `.gogo/resources/xplan-board/board.json` | the board payload the server serves |
| out | `.gogo/resources/view/*.html` | pre-built pages (via the gogo-view build) |
| out | `.gogo/resources/xplan-board/{ship-intent.json, server.pid}` | server runtime (intent + pid; server-written) |
| out | `.gogo/changelog/<date>-<name>/` + each member's `state.md` | per ship, via the gogo-done writer |

## ① Soft-dep gate (degradation-first)

`python3` is a **soft dependency**. If `command -v python3` fails, **do not fail** —
say so and hand off to the list:

> No `python3` on PATH, so the browser board can't start. Use `/gogo:done` — the same
> ready-to-ship items as a filterable multi-select (multiple picks = one merged entry).

Then stop. The board is a convenience surface; the list is always available.

Also resolve the board app now (**`${CLAUDE_PLUGIN_ROOT}` first, repo fallback**):

```bash
BOARD="${CLAUDE_PLUGIN_ROOT}/assets/xplan-board"
[ -f "$BOARD/dist/index.html" ] || BOARD="$(pwd)/assets/xplan-board"   # local-dev fallback
```

If **neither** has a built `dist/index.html`, the committed build is missing — tell the
dev to build it (`cd assets/xplan-board && npm install && npm run build`) and stop.
(Plugin users always get the committed `dist/`; this only bites a dev editing the source.)

## ② Classify + write `board.json`

Run the **gogo-status Step A classifier** over every `.gogo/work/feature-*` (the same
read-only classifier `/gogo:status` and `/gogo:done` use — reuse it, don't re-derive)
and enumerate `.gogo/changelog/*/`. Map each record into a board **item** and write the
payload to `.gogo/resources/xplan-board/board.json`.

**Columns (fixed).**

```json
"columns": [
  { "id": "plan",        "name": "plan" },
  { "id": "in-progress", "name": "in progress" },
  { "id": "ready",       "name": "ready" },
  { "id": "changelog",   "name": "changelog" }
]
```

**Class → column** (the classifier's four classes map 1:1):

| class | column | `view_url` (matches gogo-view's output names) |
|---|---|---|
| unfinished | `plan` | `<slug>-plan.html` |
| in-progress | `in-progress` | `<slug>-plan.html` |
| ready-to-ship | `ready` | `<slug>.html` |
| shipped | `changelog` | `<basename of changelog_path>.html` (i.e. `<date>-<name>.html`) |

**Items = every work feature (by class) + orphan changelog entries.** Emit one item per
`.gogo/work/feature-*` using the class→column map above — a shipped work feature keeps
its **slug** and lands in the changelog column (so the ship reconciliation below can see
it "move"). Then enumerate `.gogo/changelog/*/`; for any entry **not already** covered by
a shipped work feature's `changelog_path`, add a changelog item (`slug` = `<date>-<name>`,
`title` = the release name, `view_url` = `<date>-<name>.html`) so historical releases
still show even when their work folder is gone.

**Item shape** (the payload the React app consumes):

```json
{
  "repo": "<repo name>",
  "generated": "<ISO timestamp>",
  "columns": [ ... the four above ... ],
  "items": [
    {
      "slug": "<feature slug or <date>-<name> for an orphan entry>",
      "title": "<feature title / release name>",
      "class": "shipped|ready-to-ship|in-progress|unfinished",
      "column": "plan|in-progress|ready|changelog",
      "view_url": "<filename per the table above>",
      "report_path": "<repo-relative report.md, or null>",
      "changelog_path": "<.gogo/changelog/<date>-<name>/ path, or null>",
      "iterations": "plan=N implement=N review=N test=N",
      "status": "<raw state.md status>"
    }
  ]
}
```

`repo` = the project name (git remote basename, else the cwd basename). `view_url` is a
**bare filename**; the board requests it at `/view/<view_url>` (the server serves it from
`.gogo/resources/view/`). Write the file with the **Write tool** to
`.gogo/resources/xplan-board/board.json` (create the dir first). This is the one file the
board re-reads on every poll, so **rewrite it in full** on each build/rebuild.

## ③ Pre-build all view pages (D3=A — reuse gogo-view)

So the board's **view** links serve offline and instantly, build **every** item's page
up front via the **`gogo-view` build** — do not reimplement it. For each item:

- **plan / in-progress** → build the feature's **plan** bundle → `<slug>-plan.html`.
- **ready** → build the feature's **report** bundle → `<slug>.html`.
- **shipped / changelog entry** → build the changelog entry → `<date>-<name>.html`.

Load `gogo-view` and run its **Step 2 (ensure shared resources)** once, then its **Step 3
(build the page)** per item, writing to `.gogo/resources/view/<name>.html` — but **skip
its Step 4 auto-open** (the board opens pages itself). Build only **missing or stale**
pages (skip a page whose source `.md`/`.mmd` is older than the built `.html`) so relaunch
is fast. A page that can't be built (no diagrams, mermaid missing) is best-effort — the
board still lists the card; the view button just opens whatever page exists (or 404s
gracefully). This also seeds `.gogo/resources/{mermaid.min.js, viewer/*}`, which the
server serves under `/mermaid.min.js` + `/viewer/*` so a page opened under `/view/`
reaches its `../` siblings.

## ④ Serve + open (long-running, localhost only)

Start the server in the **background** against the committed `dist/` and the runtime
dirs, then print the link and best-effort open a browser:

```bash
set -euo pipefail
mkdir -p .gogo/resources/xplan-board .gogo/resources/view
python3 "$BOARD/server.py" \
  --dist "$BOARD/dist" \
  --data .gogo/resources/xplan-board \
  --view-root .gogo/resources/view \
  --port 4173 \
  > .gogo/resources/xplan-board/server.log 2>&1 &
# the server prints `gogo board: http://127.0.0.1:<port>` and writes server.pid
```

Read the chosen URL from `.gogo/resources/xplan-board/server.log` (or reconstruct it from
the port), **print it**, and open it best-effort:

```bash
url="$(grep -oE 'http://127\.0\.0\.1:[0-9]+' .gogo/resources/xplan-board/server.log | head -1)"
echo "gogo board: ${url:-http://127.0.0.1:4173}"
if command -v open >/dev/null 2>&1; then open "$url" >/dev/null 2>&1 || true
elif command -v xdg-open >/dev/null 2>&1; then xdg-open "$url" >/dev/null 2>&1 || true
fi
```

The server binds **127.0.0.1 only**, tries the next free port if `--port` is busy (the
printed URL reflects the actual port), and writes its PID to
`.gogo/resources/xplan-board/server.pid`. The board polls `GET /api/board` every ~3s
(paused while a drag is in progress).

## ⑤ Watch + ship (D2=A — the live-refresh loop)

Keep watching `.gogo/resources/xplan-board/ship-intent.json` (the server writes it
atomically on a valid `POST /api/ship`; it already enforces the only-ready guard, so a
present intent is trusted). Do this as a **background wait** so the chat stays responsive:

```bash
INT=.gogo/resources/xplan-board/ship-intent.json
while [ ! -f "$INT" ]; do sleep 1; done   # (background) fires when a ship arrives
```

On each intent:

1. **Read + delete it** (read `action` + `items`, then `rm -f "$INT"` so the next ship can
   be posted — a lingering intent makes the server answer 409).
2. **Ship via the gogo-done writer.** Load `gogo-done` and run its single
   **"Write changelog entry (1..N members)"** flow with `members = items`:
   **1 item = one entry; ≥2 items = ONE merged entry** (suggest + confirm the release name
   in chat, per the writer). This is the same writer `/gogo:done` uses — do not
   reimplement shipping. It synthesizes the entry, marks each member `shipped`, and builds
   the entry's viewer page.
3. **Rebuild `board.json` + affected pages** — re-run ② (rewrite the full `board.json`)
   and ③ for the changed items (the shipped members leave `ready`, the new/updated
   changelog entry appears). The board's next poll re-renders: the shipped card **moves to
   the changelog column live** (the app clears its "shipping…" toast once the shipped
   slugs read `column: changelog`).
4. **Keep watching** — loop back; multiple ships per session are expected.

**Illegal drags never reach here** — the board only POSTs a `ready → changelog` transition
(any other column move bounces client-side with a hint), and the server rejects a
non-ready slug with 400.

## Stop / shutdown

Watch the chat: when the user says **stop / done / close** (or ends the session), shut the
server down and stop the watch loop:

```bash
kill "$(cat .gogo/resources/xplan-board/server.pid)" 2>/dev/null || true
```

The pid also prints on launch. `kill <pid>` (SIGTERM) makes the server clean up its own
`server.pid`. The board app is static — closing the browser tab is harmless; the server
keeps running until killed.

## Degradation (never hard-fail)

| Situation | Behaviour |
|---|---|
| `python3` absent | say so, point at `/gogo:done` (the list); stop cleanly (① gate) |
| committed `dist/` missing | dev-only — tell the dev to `npm install && npm run build` in `assets/xplan-board`; stop |
| `--port` busy | `server.py` binds the next free port; the printed URL reflects it |
| browser won't open | print the `http://127.0.0.1:<port>` URL; the user opens it |
| a view page can't be built | best-effort — the card still lists; its view button opens what exists |
| `board.json` briefly rewritten | written by the Write tool (whole-file); `GET /api/board` re-reads per request |

Everything is under `.gogo/` and bound to `127.0.0.1` — no network, no writes outside
`.gogo/`. The board is a surface over the same classifier + writer the terminal uses;
losing it never blocks shipping (that is `/gogo:done`).
