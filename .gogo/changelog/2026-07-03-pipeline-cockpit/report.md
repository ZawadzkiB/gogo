# Release — `pipeline-cockpit`

- **shipped:** 2026-07-03
- **members:** `pipeline-commands` · `changelog-merged-entries` · `board-actions-and-filter` · `cli-cockpit-and-events`
- **plugin versions covered:** 0.2.0 → 0.10.0 (+ the `gogo` CLI, first release)

## What shipped

**The gogo pipeline became a typed, observable system you drive from a cockpit.**
Four related features form the arc: first every phase became a **standalone,
validatable command** with a JSON-Schema contract layer catching bad hand-offs
between phases; then `/gogo:done` learned to **synthesize high-level changelog
entries** — including shipping several related features as ONE merged release
entry (this very entry is one); then the work board grew into an **interactive
cockpit** — action keys to view, ship, merge-ship, run/resume, and filter, each a
single-shot intent the orchestrator executes before relaunching the board; and
finally the cockpit went **native**: a Go binary (`gogo` in `cli/`) that opens the
kanban board in milliseconds by deterministically parsing the contract files —
no LLM in the read path — with card moves launching Claude in tmux for the two
jobs only it can do (pipeline execution, changelog synthesis), and a new
**`events.jsonl` telemetry contract** making every pipeline run observable from
outside the chat for the first time.

Key outcomes:

- **Typed hand-offs** — four standalone phase commands (`/gogo:implement`,
  `/gogo:review`, `/gogo:test`, `/gogo:report`), JSON-first living issues lists,
  and validate-in/out gates over `templates/contracts/` schemas.
- **A changelog that reads like a release history** — every entry (single or
  merged) is a written synthesis with a slim file set (`report.md` + `.mmd` +
  `manifest.json` with `members[]` + `before/`); the audit trail stays in
  `.gogo/work/`, linked.
- **One cockpit, two surfaces** — the `board.py` TUI (schema-v2 intents +
  relaunch loop, crash-safe exit codes) and the `gogo` CLI (bubbletea board,
  live fsnotify refresh, terminal viewers with ASCII mermaid, tmux-launched
  Claude moves) — both pure selectors/readers; only the pipeline mutates state.
- **External observability** — one schema'd JSON line per phase transition,
  single-owner emitters (phase skills own lifecycle events, the orchestrator
  owns gates), frozen in `docs/cli-contract.md`.

## Decisions (one line each)

- **Issues are JSON-first**, one living list per track, with `review-NN.md` /
  `test-NN.md` as human snapshots (pipeline-commands D1/D2).
- **Two-stage delivery** — the contract layer shipped first; orchestrator
  chaining on `result.json`/`pipeline.json` stayed a follow-up (D7, still open).
- **Every changelog entry is synthesis-only** — the user widened D1 at the
  acceptance gate: single AND merged entries are high-level summaries, never
  full-report copies.
- **The merge question lives post-selection** (one `AskUserQuestion`; a
  `+`-joined arg pre-answers it), so `board.py` and its exit-code contract stay
  untouched.
- **One mode, action keys** for the board — modes would double key-map and state
  for zero extra capability; every action is a single-shot intent + relaunch.
- **`events.jsonl` over state.md history** — append-only telemetry gives the
  timeline; `state.md` stays the human resume file.
- **Moves launch interactive `claude` in tmux** (not `-p`) so decision gates stay
  answerable; the launch is a single argv element, no shell — injection-probed.

## Review / test verdict

All four members closed green with zero open issues — 40+ findings across the
release (including two blockers found only by live tmux-driven TUI testing and
three user-UAT rounds on the CLI) were fixed and verified, with one consciously
accepted wontfix (the CLI's embedded mermaid copy, REV-012).

## Members

| Member | Feature | Outcome |
|---|---|---|
| [`pipeline-commands`](../../work/feature-pipeline-commands/) | typed phase commands + validation gates (0.2.0) | four standalone commands + `gogo-contracts` skill + four JSON-Schema contracts; Stage B (orchestrator chaining) deferred |
| [`changelog-merged-entries`](../../work/feature-changelog-merged-entries/) | synthesized + merged changelog entries (0.8.0) | one "Write changelog entry (1..N members)" writer; `members[]` manifests; slim entries, no `diagrams.html` |
| [`board-actions-and-filter`](../../work/feature-board-actions-and-filter/) | `/gogo:done` board cockpit (0.9.0) | action keys v/s/m/g + `/` filter; schema-v2 intents + relaunch loop; crash-safe exit contract |
| [`cli-cockpit-and-events`](../../work/feature-cli-cockpit-and-events/) | `gogo` CLI + events telemetry (0.10.0) | ms-startup Go board over the frozen contract (`docs/cli-contract.md`); tmux Claude launches; `events.jsonl` dogfooded |

### `pipeline-commands` — typed phase commands + validation gates

Each phase became an idempotent command that declares its inputs, validates them,
works, and validates its outputs — with the contract layer
(`templates/contracts/*.schema.json` + `gogo-contracts`) making cross-phase data
machine-checkable. Review round 1 caught real contract drift (chart-kind prose vs
the schema enum); the negative-case schema suite ran green. Shipped as Stage A;
the orchestrator still drives the loop itself.
Full trail: [.gogo/work/feature-pipeline-commands/](../../work/feature-pipeline-commands/)

### `changelog-merged-entries` — the changelog reads like a release history

All shipping funnels through one entry-writer: a written synthesis (this
document's own format), slug-prefixed diagrams, one `manifest.json` carrying
`members[]` (which the classifier reads to mark merged members shipped), and an
idempotent dated dir. Two review rounds → APPROVE; a fixture dogfood exercised
merged + single ships end-to-end, GREEN in one test round.
Full trail: [.gogo/work/feature-changelog-merged-entries/](../../work/feature-changelog-merged-entries/)

### `board-actions-and-filter` — the work board becomes the cockpit

The one-shot ship-picker became a persistent-feeling cockpit: `v` views any
card's page, `s`/`m` ship separately/merged, `g` runs or resumes the pipeline,
`/` live-filters — each key a schema-v2 intent the orchestrator routes and then
relaunches the board (re-classified). Review hardened the crash path (a TUI
failure is exit 2 → fallback, never mistaken for a cancel); the test round drove
the real curses TUI over tmux for the first time. GREEN, zero issues.
Full trail: [.gogo/work/feature-board-actions-and-filter/](../../work/feature-board-actions-and-filter/)

### `cli-cockpit-and-events` — the cockpit goes native, the pipeline gets a pulse

A Go 1.25 binary reads the frozen contract deterministically (state.md grammar,
classifier, manifests, events) and renders the board, drill-in viewers, ASCII
mermaid, and web pages instantly; column moves launch `claude "/gogo:go <slug>"`
/ `"/gogo:done a+b"` in attachable tmux sessions. The `events.jsonl` contract
(single-owner emitters, RFC3339, lenient consumers) closes the loop — fsnotify
refreshes the board as skills append. 24 findings over two review rounds + three
UAT rounds — including the huh-form routing blocker and the `WithAutoStyle` TTY
freeze that only live driving could catch — all resolved; 64 Go tests green
under `-race`.
Full trail: [.gogo/work/feature-cli-cockpit-and-events/](../../work/feature-cli-cockpit-and-events/)

## Diagrams

Slug-prefixed as-built set beside this report (open interactively with
`/gogo:view 2026-07-03-pipeline-cockpit` or the `gogo` CLI); `before/` carries
each member's plan-time baseline for the viewer's compare mode
(`pipeline-commands` predates before-sets).

## Summary (TL;DR)

Four features, one arc: typed phase commands with validated hand-offs →
synthesized (and merged) changelog entries → the interactive board cockpit →
the native `gogo` CLI with `events.jsonl` telemetry. The LLM is out of the read
path; the pipeline is observable from outside the chat; everything mechanical is
instant, and Claude keeps the thinking.
