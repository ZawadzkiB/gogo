---
name: gogo-plan
description: >-
  Phase ① of the gogo pipeline — analyse a goal against the project's knowledge
  docs and write a plan the user must accept before any code is written. Creates
  .gogo/work/feature-<slug>/. Invoked by the gogo orchestrator or on /gogo:plan.
  Hard gate: never implement an unaccepted plan.
---

# gogo-plan — phase ① (plan, then STOP for acceptance)

## Preconditions
- Config gate: `.gogo/knowledge/` must exist (else tell the user to run `/gogo:build`).
- Read: `project-knowledge.md`, `tech-stack.md`, `non-functional-requirements.md`,
  `coding-rules.md` (follow their `Source:` links for detail when needed).

## Steps
1. **Slug + folder.** Derive a kebab-case slug from the goal. Create
   `.gogo/work/feature-<slug>/`. If it already exists, you are **revising** — read the
   existing `plan.md`/`adjustments.md`/`state.md`; don't overwrite blindly.
2. **Analyse** the goal against the knowledge + the actual codebase (Glob/Grep/
   Read the relevant code paths). Identify reuse, affected files, and edge cases.
3. **Write `plan.md`** with this shape:
   - **Goal**
   - **Context** — what exists; the key code paths
   - **Functional requirements** — what this change must do (a feature's
     requirements live here, not in `.gogo/knowledge/`)
   - **Approach** (recommended) + alternatives considered
   - **Changes checklist** — files to add/modify, in build order
   - **Tests** — what will be verified, at which level
   - **Out of scope**
   - `Status: awaiting acceptance`

   Design **within** the bars in `non-functional-requirements.md`.
4. **Draw the intended design** (not the task list). Use the `gogo-mermaid` skill
   to diagram how the feature will *work* — the control/data flow, the runtime
   interaction between real components, or the domain states it touches — as a
   fenced block in `plan.md`, a `.mmd` in `charts/`, and the offline viewer.
   Label nodes with real endpoints/modules/states, **never** with FR numbers,
   build steps, or the gogo phases. If the change is pure process (docs/tests/
   merge/config) with nothing structural to show, skip the diagram and say so.
5. **Init state.** Copy `${CLAUDE_PLUGIN_ROOT}/templates/state.template.md` →
   `state.md` and `decisions.template.md` → `decisions.md`; create
   `adjustments.md` (header only). Set `state.md`: phase=plan,
   status=awaiting-plan-acceptance, created=<today>, iterations all 0.
6. **Present + STOP.** Show the plan; ask the user to accept or request changes
   (`AskUserQuestion` with Accept / Request changes when the forks are clear).
   **Write no product code.**
   - Changes / clarification → append to `adjustments.md`, revise `plan.md`,
     re-present (stay in phase ①).
   - Accept → set `state.md` status=plan-accepted and add a top line to `plan.md`:
     `Status: **accepted** (user, <today>)`. Tell the user to run `/gogo:go`.

## Hard rule
Never start implementing in this phase. Acceptance is the gate between plan and
implement — `/gogo:go` refuses unless `state.md` reads `plan-accepted`.
