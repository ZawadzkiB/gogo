---
description: Resume a feature that paused for your decision, folding in your answer.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, Task, TodoWrite, AskUserQuestion
model: opus
---

Resume a paused gogo feature.

Target: $ARGUMENTS  (if empty, the feature whose `state.md` is `waiting-for-user`).

Load the `gogo` skill, then:

1. Read `state.md` + `decisions.md`. Apply the user's latest answer to the open
   decision — append a `### RESOLVED (user, <today>)` block under it and clear
   `open-decision`.
2. Re-enter the pipeline at `state.md`'s `resume:` phase via the orchestrator, and
   continue the loop to done.
