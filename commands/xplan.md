---
description: Open the gogo work as a browser kanban — a React board (ported from xplan) served by a python3 stdlib server on localhost. Four columns plan · in progress · ready · changelog fed by the shared gogo-status classifier + the changelog entries; drag-and-drop, a text filter, a "view" button that opens each item's pre-built HTML page, and mark-done from the board (select ready cards or drag one onto changelog — multiple = ONE merged entry). Pre-builds all view pages at launch; a long-running server + polling board refresh live after each ship. python3 is a soft dep — absent, it points you at /gogo:done's list. Never writes outside .gogo/.
argument-hint: "(no args)"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

Open the gogo work as a **browser kanban**, via the `gogo-xplan` skill.

Load `gogo-xplan` and follow it:

1. **Soft-dep gate** — if `python3` is absent, say so and point at **`/gogo:done`**
   (the filterable ready-to-ship list) — never hard-fail.
2. **Classify + build `board.json`** — run the shared `gogo-status` Step A classifier
   over every `.gogo/work/feature-*` and enumerate `.gogo/changelog/*/`; write the
   board payload to `.gogo/resources/xplan-board/board.json` (four columns
   **plan · in progress · ready · changelog**).
3. **Pre-build all view pages** (D3=A) — reuse the **`gogo-view` build** to (re)build
   every item's page under `.gogo/resources/view/` so the board's "view" links serve
   offline.
4. **Serve + open** — start `assets/xplan-board/server.py` (python3 stdlib, localhost
   only) in the background against the committed `dist/`, print the
   `http://127.0.0.1:<port>` link, and best-effort open a browser.
5. **Watch + ship** (D2=A) — wait for `.gogo/resources/xplan-board/ship-intent.json`;
   on each intent, run the **`gogo-done` "Write changelog entry (1..N members)"** writer
   (multiple = ONE merged entry; release name suggested + confirmed in chat), rebuild
   `board.json` + the affected pages, and keep watching — the polling board moves the
   card live. Stop when the user says stop/done (kill the server pid).

All flow lives in the skill. Only ever writes under `.gogo/`; offline (localhost only).
