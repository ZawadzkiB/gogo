# .gogo/skills — extracted on-demand skills

**Purpose:** the registry of skills `/gogo:skills` has extracted from this
project's `.gogo/knowledge/*` files. Each row is a cohesive section lifted out of
an always-read knowledge file into a skill that loads **only when relevant** — so
the always-read context stays lean and the pipeline workers stay deterministic.

Generated + maintained by `/gogo:skills`. Re-run it anytime to re-audit the line
budget and extract more.

## Kinds

- **knowledge** → `.gogo/skills/<slug>/` — project-/pipeline-scoped detail. **Not**
  harness-auto-discovered; the gogo pipeline loads it via the parent knowledge
  file's `**Load when:**` pointer, only when a task touches it. Honors `.gogo/`-only.
- **standalone** → `.claude/skills/<slug>/` — a self-contained, reusable capability
  the Claude Code harness auto-discovers and can invoke by name. Written **only**
  when the user approved that candidate as standalone.

## Extracted skills

| Skill | Kind | Destination | Trigger / description | Source (file › section) | Lines saved |
|---|---|---|---|---|---|
| `release-history` | knowledge | `.gogo/skills/release-history/` | a change touches a specific past release's mechanism/rationale | `project-knowledge.md › gogo overrides` | 459 |
| `assertion-vacuity-catalog` | knowledge | `.gogo/skills/assertion-vacuity-catalog/` | writing/reviewing test assertions; mutation sweeps | `test-strategy.md › The FOURTEEN variants` | 84 |
| `tui-tmux-testing` | knowledge | `.gogo/skills/tui-tmux-testing/` | hands-on TUI testing (tmux runbook, two-layer strategy) | `test-strategy.md › Live TUI testing via tmux + Go TUI` | 49 |

<!-- One row per extracted skill. Empty table until /gogo:skills extracts the first. -->
