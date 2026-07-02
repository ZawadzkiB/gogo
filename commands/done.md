---
description: Ship report-complete features into a high-level changelog — with a slug, ship that one; with slug1+slug2 ship those as ONE merged release entry; with no slug, open the work board COCKPIT (terminal kanban, or a status table + multi-select fallback) where action keys view (v), ship (s), ship-merged (m), go/resume (g), and filter (/) each write a single-shot intent the orchestrator runs before relaunching the board; when ≥2 are shipped merged, confirm a release name. Every entry is a SYNTHESIZED report.md (not a copy of the report bundle) + the slug-prefixed .mmd set + a manifest.json with members[] + before/, plus a printed interactive viewer link; each member's state.md becomes shipped. Validates inputs.
argument-hint: "[feature-slug | slug1+slug2+...]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

Mark features **shipped** and write a high-level entry into the changelog, via the
`gogo-done` skill. The explicit post-report "this is the end" gate. The changelog
reads like a release history; the full audit trail stays in `.gogo/work/` (linked).

Target: $ARGUMENTS  (a **slug** ships that one; **`slug1+slug2+...`** ships those as
ONE merged release entry; **no slug opens the work board cockpit** over every
`.gogo/work/feature-*` so you can view, ship, merge-ship, go/resume, and filter.)

Load `gogo-done` and follow it:

1. **validate-in** — each **named** slug (or every `+`-joined slug) must be
   report-complete: `.gogo/work/feature-<slug>/report/report.md` must exist. Missing →
   STOP with: "No report found for `<feature>` — run `/gogo:report <feature>` first,
   then `/gogo:done`." (`/gogo:report` works even on a past/broken run.) For the
   **board** (no slug), if there are no work items at all, say so and stop — don't open
   an empty board (with items but nothing ready-to-ship the cockpit still opens for
   view/go/filter).
2. **Cockpit** — a slug / `slug1+slug2` resolves directly; **no slug** opens the work
   board cockpit: classify every work item via the shared gogo-status Step A classifier
   (shipped · ready-to-ship · in-progress · unfinished) and act on it with **action
   keys** — an **interactive terminal kanban** (`assets/kanban/board.py` in a tmux pane)
   when `python3` + `tmux` + a tty are present, else a **status table + `AskUserQuestion`
   multi-select** ship fallback (never fail over the board). Keys: **space/enter** select
   a ready-to-ship card, **v** view the focused card, **s** ship separately, **m** ship
   merged (≥2), **g** run/resume the pipeline on an unbuilt card, **/** filter (Esc
   clears), **q** cancel. Each key writes a single-shot **intent** (`{schema:2, action,
   items}`); the orchestrator executes it (view build / ship writer / pipeline handoff)
   and **relaunches** the board — `go` ends the loop, `cancel` stops. The board only
   *collects intents*; it never mutates gogo state.
3. **Merge gate** — when a fallback selection is **≥2** slugs, one `AskUserQuestion`: ship
   **separately** (N entries) or **merged** (1 entry)? A `+`-joined arg pre-answers
   *merged*; a single slug never asks. For a merged entry, gogo suggests a release name
   from the members' common theme and confirms it (you can override).
4. **Write the entry (per entry)** — a **synthesized** high-level `report.md` (*what was
   changed/done/implemented*, key outcomes, one-line decisions, review/test verdict,
   member table + per-member section when merged, links back to each `.gogo/work/`
   folder) — **written, never a copy** — plus the slug-prefixed `.mmd` set, a merged
   `manifest.json` carrying a `members[]` array, and the merged `before/` set, into
   `.gogo/changelog/<date>-<name>/` (date = newest member's `completed:`; append-only,
   idempotent). **No `diagrams.html` copy** — the viewer builds from source.
5. **Finish** — set **each member's** `state.md` to `status: shipped`; **build the
   interactive viewer page** for each entry (reusing the `gogo-view` build,
   best-effort) and **print its `file://` link** (changelog folder path as fallback —
   never fail `/gogo:done` over the link); confirm each entry.
