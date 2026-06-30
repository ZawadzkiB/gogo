---
description: Audit .gogo/knowledge/* against a line budget and extract bloated sections into on-demand skills (propose-then-approve; idempotent). Directed via "<prompt>".
argument-hint: "[\"<prompt>\"] [--warn N] [--max N] [--include <path>]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, AskUserQuestion
---

Run the `gogo-skills` skill for this project.

Arguments: $ARGUMENTS

- **no prompt** → audit / auto-discover: scan every `.gogo/knowledge/*.md`,
  measure body lines (OK `<200` · WARN `200-400` · OVER `>400`), discover cohesive
  extraction candidates, classify each as `knowledge` (→ `.gogo/skills/`) or
  `standalone` (→ `.claude/skills/`), and **propose them — then STOP for your
  per-candidate approval** before writing anything.
- **`"<prompt>"`** → directed: extract exactly the section you name.
- **`--warn N` / `--max N`** → override the 200 / 400 thresholds.
- **`--include <path>`** → also audit a path outside `.gogo/` (**report-only** —
  never extracted).

Follow the skill: on approval it writes each skill's `SKILL.md` (+ optional
`scripts/` / `.env.example`), replaces the parent section with a `**Load when:**`
pointer, updates `.gogo/skills/index.md`, and re-measures to confirm the parent is
under budget. Default writes stay inside `.gogo/`; the only write outside is an
approved standalone skill's `.claude/skills/<slug>/`.
