# Adjustments — feature `cli-cockpit-and-events`

Running log of user-requested changes / clarifications during planning.

## 2026-07-02 — origin (proposal round, verbatim intent)

- `/gogo:done` + `/gogo:view` "loads, thinks and it is really slow" for what is
  just managing/displaying existing work — wants a terminal interface (not
  necessarily tmux; "maybe some bubble tea ui?") that opens the board
  **immediately**.
- At the proposal gate the user redirected two recommendations:
  - CLI lives **inside the gogo repo** (`cli/`), not a separate repo;
  - shipping/moves must run **Claude** (`claude -p /gogo:done`, `/gogo:go`) —
    no native Go ship-writer; runs "maybe in tmux? so we can save sessions and
    restore or view/switch later"; the board shows in-progress indications and
    needs a state/history file the orchestrator/agents keep updated ("when it
    started implementation, when it moved to review… so we can read state and
    see whats happening inside pipeline/flow") → the events.jsonl contract.
- Follow-up: the CLI is a **deterministic parser of the contract files the
  plugin creates** — board items are the work folders; press an item → file list
  (report, decisions, …) → view in terminal; select a ticket → move it
  plan→implement, implement→done etc.

## 2026-07-03 — plan revision: name the Charm repos explicitly (user)

User paused /gogo:go to fold in specific libraries:
- **glow** (charmbracelet/glow) — "must have to show/view reports, plans etc".
  Resolution: our in-board viewer uses **glamour** — glow's own rendering engine —
  in-process (glow-quality output, zero external dep); PLUS when the `glow`
  binary is installed, `G` opens the current file in full glow (pager/browse
  mode) as a soft-dep nicety.
- **huh** (charmbracelet/huh) — forms: the merge release-name prompt (suggested
  default + confirm), ship confirmations, and any future in-TUI dialogs.
- bubbletea + bubbles — already the planned foundation (confirmed).

## 2026-07-03 — plan revision 3: ASCII mermaid diagrams in terminal (user)

> "lets add ASCII mermaid diagrams libs to display diagrams in terminal, show
> diagrams files to open from report and open them as ASCII in terminal view"

Resolution: diagram files (`charts/*.mmd`, `report/*.mmd`, changelog `.mmd`) are
listed in every item's drill-in; opening one renders **ASCII in-terminal** via
the Go `mermaid-ascii` package (AlexanderGrooff/mermaid-ascii) for the
**flowchart family** (flowchart/graph — the majority of gogo diagrams); kinds it
can't draw (sequence/class/state) show the highlighted .mmd source with a
"press w for the full browser view" hint. Honest-limits note recorded.
