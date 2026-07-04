# Coding rules

**Purpose:** the conventions the implement and review phases must follow.

<!-- gogo:meta
Mode: proxy
Source: [ ]            # e.g. ../../CLAUDE.md, ../../CONTRIBUTING.md, ../../.cursor/rules/
Confidence: low
Generated-by: /gogo:build (scaffold)
-->
> Conventions the implementation must follow. Source of truth: links above.

## General
- Match surrounding code (naming, idiom, comment density).
- Smallest correct change; stay scoped to the plan; no opportunistic refactors.
- Keep build + tests green; commit in safe increments (only when asked to commit).

## Project-specific
<lint / format config or "match nearby files"; import conventions; layering
rules; framework idioms — filled from the linked source or the codebase>

## gogo overrides
<!-- Preserved across re-runs. -->

### Knowledge file line budget
- Keep each `.gogo/knowledge/*.md` body **lean**: OK `<200` lines · WARN
  `200-400` · OVER `>400` (defaults; `/gogo:skills --warn N --max N` overrides).
  Big always-read context makes the LLM pipeline workers wander and lose
  determinism — measure the **gogo-owned body** only (for a proxy, never the
  linked upstream).
- When a file goes over budget, extract cohesive, situational sections into
  **on-demand skills** with `/gogo:skills` (the parent keeps a `**Load when:**`
  pointer). `/gogo:build` prints a nudge once a file passes the warn line.
- **Write rule + its one exception.** Default writes stay under `.gogo/`. The
  **only** sanctioned write outside `.gogo/` is an extracted **standalone** skill's
  `.claude/skills/<slug>/` dir — and only when the user approves that candidate as
  standalone (never automatic). Everything else still honors `.gogo/`-only.

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
