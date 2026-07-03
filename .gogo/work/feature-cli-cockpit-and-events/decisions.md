# Decisions — feature `cli-cockpit-and-events`

## D1 — The live-progress contract

**Question:** how does the CLI learn what's happening *inside* the pipeline?
- **A (recommended):** new **`events.jsonl`** per feature (append-only, schema'd:
  ts/event/phase/status/round/note), emitted by the skills beside every state.md
  update; state.md stays the human resume file. Gives timestamps, rounds, and the
  full history the user asked for ("when it started implementation, when it moved
  to review, when review was done and moved back").
- **B:** parse state.md only — zero plugin change, but no timestamps/history; the
  board can show the current phase, never the timeline.

**Recommendation:** A.

**RESOLVED (2026-07-02):** **A** (events.jsonl contract).

## D2 — How column moves run Claude

**Question:** the CLI launches `/gogo:go` / `/gogo:done` — in what mode?
- **A (recommended):** **interactive `claude` inside a tmux session**
  (`gogo-<action>-<slug>`) when tmux exists — sessions are saveable, listable,
  attachable from the TUI, and **decision gates stay answerable** on attach; no
  tmux → background `claude -p` + log file, with gate-parked runs surfaced as
  "waiting for user — resume in chat".
- **B:** always `claude -p` (print mode) — simpler, but gates can never be
  answered in-place and there is nothing to attach to.

**Recommendation:** A (tmux returns in its correct role: managing background
Claude sessions — not rendering UI).

**RESOLVED (2026-07-02):** **A** (interactive claude in tmux; -p fallback) — accepted with the plan ("Accept (all recs)").

## Non-forks (user-stated, recorded)

- **CLI lives in the gogo repo (`cli/`)** — monorepo; contracts beside their spec.
- **Actions are Claude, always** — moving to done requires Claude's synthesis
  (`claude … /gogo:done`); plan→implement runs `claude … /gogo:go`. The CLI never
  writes changelog entries or mutates pipeline state itself; a card moves columns
  only when the contract files actually change.
- **The CLI is a deterministic parser** of the files the plugin creates (plan.md,
  report.md, decisions.md, state.md, issues.json, diagrams, changelog entries) —
  board items are the folders; drill-in lists and renders their files in-terminal.
- **MVP = full loop** (board + view + moves/ship + status); binary name **`gogo`**;
  stack **Go + Bubble Tea** (+ glamour/goldmark/fsnotify).

## Implementation note (review round 1 — orchestrator-resolved, recorded)

**Event-emitter ownership (REV-001):** each phase skill owns ALL lifecycle
events for its phase (`phase-started`, `phase-done`, rounds, `plan-accepted`,
`shipped`); the orchestrator emits ONLY `gate-opened`/`gate-resolved` (mapping
state.md's `knowledge` → events' `report` per REV-003). Guarantee for the CLI:
each transition is emitted exactly once, by its owning skill. Not escalated —
a precision fix within D1's design, no trade-off.

## Implementation note (review round 2 — orchestrator-resolved, recorded)

**REV-012 (embedded mermaid duplication):** accepted. `go:embed` requires the
file inside the module, so `cli/internal/pages/assets/mermaid.min.js` duplicates
`assets/mermaid/mermaid.min.js` (~3.3 MB). The alternative (a build-time sync
step) breaks `go build ./cli` out-of-the-box for contributors. Kept, with the
Makefile `sync-assets` target as the source-of-truth bridge; to be recorded in
the Footprint NFR at phase ⑤. Not escalated: simple trade-off, clear winner.

## Implementation note (test round 1 fixes — orchestrator-resolved, recorded)

**Confirm-form default = Launch (affirmative).** Raised by the round-5 fixer:
Enter/Tab submits-and-launches (prior default was Cancel). Kept: the form is
always shown with a full summary, three abort paths exist (Esc, Ctrl+C, toggle
`n`), the launched action is non-destructive (an attachable claude session that
itself gates), and affirmative-default matches standard dialog UX. Not
escalated. If it ever bites, the alternative (require explicit toggle before
Enter) is a one-line change.
