---
description: Implement an accepted plan through the implement → review → test → report loop, pausing only at real decisions.
argument-hint: "[feature-slug]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, Task, TodoWrite, AskUserQuestion
model: opus
---

Act as the **gogo orchestrator** (in this chat, so you can pause for the user at
gates) and run **phases ② → ③ → ④ → ⑤** for the target feature. The orchestrator
delegates every phase to its specialist agent (① `gogo-analyst` · ② `gogo-developer`
· ③ `gogo-reviewer` · ④ `gogo-tester` · ⑤ orchestrator + `gogo-knowledge`) and owns
the gates in chat.

Target: $ARGUMENTS  (if empty, pick the most recent `.gogo/work/feature-*/` whose
`state.md` is `plan-accepted` or a resumable mid-pipeline state
(`implementing` / `reviewing` / `testing`); if several are candidates, ask which.)

Load the `gogo` skill and follow it:

- **Acceptance gate:** read `state.md`. Run only when status is `plan-accepted` or a
  resumable mid-pipeline state (`implementing` / `reviewing` / `testing`).
  **`awaiting-uat` and `waiting-for-user` are NOT runnable here** — `awaiting-uat` is
  the *user's* UAT gate (run `/gogo:done` to accept, or give feedback to loop back), and
  a `waiting-for-user` feature is paused on a decision or a mid-UAT re-plan: **only the
  user's re-acceptance (→ `plan-accepted`) reruns the pipeline.** Otherwise **STOP** —
  tell the user to run `/gogo:plan`, accept the plan, or (at the UAT gate) run
  `/gogo:done` or resume with feedback. **Never implement an unaccepted plan.**
- Delegate ② implement → `gogo-developer`, ③ review → `gogo-reviewer`, ④ test →
  `gogo-tester` via `Task`; route findings through the loop (fixable →
  re-implement; decision → write `decisions.md` + ask the user; clean/green →
  advance). Keep `state.md` current at every transition; bound implement↔review
  at ~3 rounds.
- On all-green, run the `gogo-knowledge` skill (⑤): finalize the plan, update the
  gogo-owned knowledge docs, and summarise.

For a fully hands-off run, you may instead spawn the `gogo` agent via `Task`.
