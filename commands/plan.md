---
description: Plan a feature/change — writes an accept-pending plan to .plans/feature-<slug>/ and stops for your acceptance.
argument-hint: "\"<what you want built>\""
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

Act as the **gogo orchestrator** and run **phase ① (plan) only** for this goal:

$ARGUMENTS

Load the `gogo` skill (your operating manual) and the `gogo-plan` skill, then:

1. **Config gate** — if `.gogo/knowledge/` is missing, STOP and tell the user to
   run `/gogo:build` first. Don't invent project rules.
2. Create `.plans/feature-<slug>/`, read the relevant knowledge, analyse the
   codebase, and write `plan.md` (incl. the feature's **functional requirements**),
   `adjustments.md`, `state.md`, and a mermaid chart via the `gogo-mermaid` skill.
3. **Present the plan and STOP for acceptance — do NOT implement.** On changes,
   log to `adjustments.md` and re-present. On acceptance, mark `state.md`
   `plan-accepted` and tell the user to run `/gogo:go`.
