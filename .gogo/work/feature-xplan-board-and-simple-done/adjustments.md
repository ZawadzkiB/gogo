# Adjustments — feature `xplan-board-and-simple-done`

Running log of user-requested changes / clarifications during planning.

## 2026-07-02 — origin (from the request)

The tmux/terminal kanban (0.9.0) failed its first real user test — the detached
session + attach dance never connected ("this terminal approach and tmux doesnt
work... :/"). Direction change, verbatim intent:
- **remove** the tmux approach and the terminal kanban entirely;
- `/gogo:done` = a simple terminal **list with multi-select**; selecting multiple
  items **means merge into one changelog entry** — no extra question; add **text
  filtering**;
- `/gogo:view` = the list of viewable items, also **filterable**;
- new **`/gogo:xplan`** command: host an **xplan-style kanban board in the
  browser** (copy the board view from ~/repos/xplan) with columns
  plan / in progress / ready / changelog; open HTML reports from the board;
  select/drag one or a few **ready** items → mark done → a new changelog entry
  appears (multiple = one merged entry).

## 2026-07-02 — mid-plan: React build allowed (user)

> "it doesnt need to be just vanilla, we have npm install for claude code, so it
> can be normal npm full react build, so it could looks nice and reactive"

Constraint relaxed: the board is a **full React+Vite app** (port xplan's actual
board components), not a vanilla re-write. Plan updated: FR4 → React port;
npm/node = **dev-time** dependency; recommended shape ships a **committed
`dist/`** so plugin users still only need python3 at runtime (new D4).
