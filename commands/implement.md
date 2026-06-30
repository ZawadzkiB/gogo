---
description: Run phase ② implement standalone — build the accepted plan, or with --issues fix a typed issues list. Validates inputs and outputs.
argument-hint: "[feature-slug] [--issues <path>]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill, Task, TodoWrite, AskUserQuestion
model: opus
---

Run **phase ② (implement)** standalone for a feature, via the `gogo-implement`
skill, with **validate-in → work → validate-out** (using `gogo-contracts`).

Target: $ARGUMENTS  (if no slug, pick the most recent `.gogo/work/feature-*/`
whose `state.md` is `plan-accepted` or mid-loop; if several, ask which.)

Arguments:
- `[feature-slug]` — which feature to implement.
- `--issues <path>` — optional. A typed issues list
  (`review/issues.json` or `test/issues.json`, per `issues-list.schema.json`).
  Given it, implement **fixes the `open`/`new` issues** and writes back
  `status: fixed` + `fix_summary` + `fixed_in_round`. Without it, implement
  **builds the accepted plan from scratch**.

Documents it accepts: `plan.md` (required, accepted), `coding-rules.md` and
`tech-stack.md` (required knowledge), and the `--issues` list (optional).

Load `gogo-implement` and follow it:

1. **validate-in** — `state.md` must be `plan-accepted` or a resumable in-loop
   state, `plan.md` present; if `--issues` is given, validate it against
   `issues-list.schema.json`. Invalid/missing → STOP with a contract error. Never
   implement an unaccepted plan.
2. **Work** — build the plan / fix the open issues, keep the tree green, emit the
   as-built `charts/` set + `charts/manifest.json`, and (in `--issues` mode) write
   fixes back into the issues list.
3. **validate-out** — validate `charts/manifest.json` (and the updated issues
   list) against their schemas; write `implement/result.json`. Update `state.md`.
