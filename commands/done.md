---
description: Ship report-complete features — with a slug, ship that one; with no slug, open the work board (terminal kanban, or a status table + multi-select) to pick which ready-to-ship features to ship. Each copies its report bundle into the append-only .gogo/changelog/<date>-<slug>/, builds + prints the interactive viewer link, and sets state.md to a terminal shipped status. Validates inputs.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

Mark features **shipped** and archive them to the changelog, via the `gogo-done`
skill. The explicit post-report "this is the end" gate.

Target: $ARGUMENTS  (a slug ships that one; **no slug opens the work board** over
every `.gogo/work/feature-*` so you pick which ready-to-ship features to ship.)

Load `gogo-done` and follow it:

1. **validate-in** — for a **slug**, the feature must be report-complete:
   `.gogo/work/feature-<slug>/report/report.md` must exist. Missing → STOP with:
   "No report found for `<feature>` — run `/gogo:report <feature>` first, then
   `/gogo:done`." (`/gogo:report` works even on a past/broken run.) For the **board**
   (no slug), if nothing is ready-to-ship, say so and stop — don't open an empty board.
2. **Board mode (no slug)** — classify every work item via the shared gogo-status
   Step A classifier (shipped · ready-to-ship · in-progress · unfinished) and let the
   user pick which ready-to-ship features to ship: an **interactive terminal kanban**
   (`assets/kanban/board.py` in a tmux pane) when `python3` + `tmux` + a tty are
   present, otherwise a **status table + `AskUserQuestion` multi-select** fallback
   (never fail over the board). The board only *selects*; shipping is the single flow
   below, looped over the picks.
3. **Ship (per feature)** — derive the entry date (the report's `completed:` field,
   else a date you supply, else today; never hardcoded) and **copy** (not move) the
   report bundle — `report.md` + the `.mmd` UML set + the `before/` set +
   `diagrams.html` (+ `manifest.json`) — into `.gogo/changelog/<date>-<slug>/`.
   Append-only and idempotent: re-running overwrites the same dated dir.
4. **Finish** — set `state.md` to `status: shipped`, `resume: none`; **build the
   interactive viewer page** for each entry (reusing the `gogo-view` build,
   best-effort) and **print its `file://` link** (with the static `diagrams.html`
   path as a fallback — never fail `/gogo:done` over the link); confirm each archived
   path.
