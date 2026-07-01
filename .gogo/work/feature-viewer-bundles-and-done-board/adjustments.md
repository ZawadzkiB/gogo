# Adjustments — feature `viewer-bundles-and-done-board`

Running log of changes / clarifications requested during planning.

## 2026-07-01 — D2 → B: build the interactive kanban now (scope grows)
User accepted D1/D3/D4 as recommended but chose to **build the interactive kanban
now** (D2=B), not defer it to roadmap #7. This materially grows item 4, so the
feature is restructured into **Stage A** (items 1-3: view menu + plan bundle +
friendlier output — low-risk) and **Stage B** (item 4: interactive kanban). Added
**D5 — kanban mechanism** (the load-bearing open fork): a `file://` HTML page can't
execute `/gogo:done` (no FS writes), so only a **local terminal TUI** (tmux pane)
truly closes *drag-card → ship*; an offline HTML kanban can only display/arrange +
export a selection. Awaiting D5 before finalizing item 4's plan; re-presenting.

_(initial plan for roadmap items 1-4 presented 2026-07-01.)_
