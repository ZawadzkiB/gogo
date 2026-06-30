---
name: <slug>
description: >-
  <One or two sentences engineered for on-demand triggering: WHAT this skill does
  and WHEN to load it. Lead with the trigger — the situation/keywords a worker
  would recognise (e.g. "Use when a task touches <X> ..."). This text is the only
  thing the harness/pipeline sees when deciding whether to load the skill, so make
  it specific, not generic.>
---

# <slug> — <short title>

<Lifted from `<source file> › <section>` by /gogo:skills on <YYYY-MM-DD>.>

<The standalone body: the content that used to live in the parent knowledge
section, rewritten to stand on its own — no "see above", no dangling reference
back to the parent file. A reader loading only this skill must have everything
they need.>

## When this applies
<The trigger, in prose: the exact situation in which the pipeline (or, for a
standalone skill, Claude Code) should load this. Mirror the frontmatter
`description`.>

## Details
<The actual content. Keep it numbered + imperative where it's a procedure; use
tables for enumerations. Stay under the line budget — a skill over 400 lines is
itself a smell, so split it.>

## Scripts (optional — delete if none)
<If the source section carried fenced shell commands or a runbook, materialize
them as `scripts/<name>.sh` next to this SKILL.md and reference them here by
relative path. Keep them dependency-light and best-effort
(`set -euo pipefail`, `|| true`).>

## Environment (optional — delete if none)
<If the scripts need env vars, document each in a sibling `.env.example`
(`NAME=` + a comment) and list them here. Never commit real secrets.>
