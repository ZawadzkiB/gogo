# Decisions — feature `viewer-bundles-and-done-board`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block.

## D1 — Plan bundle location (load-bearing)
- **Question:** Where does the viewable plan bundle live?
- **Options:**
  - A. **Keep `plan.md` at the feature root**; the viewer renders `plan.md` +
    `charts/` in place as the "plan" bundle. Low churn — `plan.md` stays the
    contract path every phase reads.
  - B. **Move to `plan/plan.md` + `plan/` diagrams**, symmetric with `report/`.
    Cleaner symmetry, but `plan.md`'s path is referenced across every phase skill,
    command, and doc — big blast radius (like the earlier report/ move, but on the
    contract file).
- **gogo recommends:** A — deliver plan-viewing without churning the contract path;
  revisit B later if the symmetry is worth it.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
A — keep `plan.md` at the feature root; view it in place (plan.md + charts/).

## D2 — `/gogo:done` board interactivity
- **Question:** How interactive is the work board in v1?
- **Options:**
  - A. A rendered **status table** (grouped by state) + **multi-select** of
    ready-to-ship items via `AskUserQuestion`. Offline, in-terminal, ships now.
  - B. The **fully-interactive kanban** (drag items across columns) — pulls
    roadmap #7's board machinery forward.
- **gogo recommends:** A now; B as roadmap #7.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
**B — build the interactive kanban now.** Item 4 grows into **Stage B**; the
mechanism is the new **D5**. (Stage A = items 1-3, unaffected.)

## D5 — Interactive kanban mechanism (load-bearing; raised by D2=B)
- **Phase:** plan
- **Question:** How is the drag-and-drop work board actually built, given a
  `file://` page can't run `/gogo:done`?
- **Options:**
  - A. **Terminal TUI in a tmux pane** — a vendored `python3` **curses** app
    (stdlib, no install) that reads `.gogo/work` + `.gogo/changelog` status, shows
    columns (unfinished · in-progress · ready · shipped), lets you move a card, and
    on drop into "ship" **actually runs the archive** (it's a local process with FS
    access) — truly closes drag→ship. Matches the "tmux/canvas" vision. Biggest
    build; hardest to auto-test (no Playwright for TUIs); adds a `python3`/`tmux`
    dependency (degrade to the table if absent).
  - B. **Offline HTML kanban** (reuse the 0.6.0 viewer infra: vanilla-JS drag
    between columns, browser). Low-risk, testable via Playwright, fits gogo's
    offline-HTML strength — **but** it can only display/arrange + **export a
    selection**; the actual shipping still runs back in the terminal (the page
    can't call `/gogo:done`). So "drag to ship" is two steps (drag in browser →
    confirm in terminal).
  - C. **Hybrid** — the offline HTML kanban for viewing/arranging **plus** the
    terminal table + multi-select (D2=A) as the actionable path. Best of both,
    two surfaces to maintain.
- **gogo recommends:** **A** if you want a single interactive surface that truly
  ships on drop (accepting the TUI build + `python3`/`tmux` soft-dep, with graceful
  fallback to the table); **B/C** if you'd rather keep everything in gogo's proven
  offline-HTML + `AskUserQuestion` lane and accept the two-step ship.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
**A — terminal TUI (tmux + vendored `python3` curses)** that truly ships on drop.
`python3`/`tmux` are **soft deps**: if either is absent, degrade to the plain status
table + `AskUserQuestion` multi-select (never a hard failure — honors the
portability bar). This TUI is the shared base for roadmap #7's plan/decision commenter.

## D3 — View menu mechanism
- **Question:** How is the "what to view" menu presented?
- **Options:** A. `AskUserQuestion` grouped picker (Work: plans/reports · Changelog:
  reports) — the pick then opens the rich HTML page. B. a generated HTML index page.
- **gogo recommends:** A — simplest, stays in-terminal; the item still opens as the
  interactive page.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
A — `AskUserQuestion` grouped picker.

## D4 — Friendlier-output scope
- **Question:** How far does "more user-friendly" go?
- **Options:** A. Authoring guidance (article lead, bold key parts, short sections)
  in gogo-plan/gogo-knowledge + viewer CSS typography. B. A structural redesign of
  the report/plan section set.
- **gogo recommends:** A — legibility only; don't churn the proven section structure.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-01)
A — authoring guidance + viewer CSS typography; no structural redesign.
