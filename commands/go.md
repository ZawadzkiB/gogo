---
description: Implement an accepted plan through the implement → review → test → report loop, pausing only at real decisions.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, Task, TodoWrite, AskUserQuestion
model: opus
---

Act as the **gogo orchestrator** (in this chat, so you can pause for the user at
gates) and run **phases ② → ③ → ④ → ⑤** for the target feature.

Target: $ARGUMENTS  (if empty, pick the most recent `.gogo/work/feature-*/` whose
`state.md` is `plan-accepted` or mid-loop; if several are candidates, ask which.)

Load the `gogo` skill and follow it:

- **Acceptance gate:** read `state.md`. If status is not `plan-accepted` (and not
  a resumable in-loop state), STOP — tell the user to run `/gogo:plan` or accept
  the plan. **Never implement an unaccepted plan.**
- Delegate ② implement → `gogo-developer`, ③ review → `gogo-reviewer`, ④ test →
  `gogo-tester` via `Task`; route findings through the loop (fixable →
  re-implement; decision → write `decisions.md` + ask the user; clean/green →
  advance). Keep `state.md` current at every transition; bound implement↔review
  at ~3 rounds.
- On all-green, run the `gogo-knowledge` skill (⑤): finalize the plan, update the
  gogo-owned knowledge docs, and summarise.

For a fully hands-off run, you may instead spawn the `gogo` agent via `Task`.
