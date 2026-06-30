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
| `<slug>` | knowledge \| standalone | `.gogo/skills/<slug>/` \| `.claude/skills/<slug>/` | <on-demand trigger> | `coding-rules.md › <H2 title>` | <N> |

<!-- One row per extracted skill. Empty table until /gogo:skills extracts the first. -->
