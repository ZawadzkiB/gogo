---
name: gogo-plan
description: >-
  Phase ① of the gogo pipeline — the operating manual for the gogo-analyst agent:
  read the named knowledge set (incl. analysis.md), analyse the goal against the
  actual codebase (code = source of truth), and write a plan the user must accept
  before any code is written. Creates .gogo/work/feature-<slug>/. Invoked by the
  gogo orchestrator (delegating ① to gogo-analyst) or on /gogo:plan; also runs
  in-chat. Hard gate: never implement an unaccepted plan.
---

# gogo-plan — phase ① (plan, then STOP for acceptance)

This skill is the operating manual for the **`gogo-analyst`** agent — the phase-①
specialist the orchestrator delegates to — and for the orchestrator when it plans
in-context. It works either way. You are the *planner*; the **acceptance gate is a
hard gate the orchestrator owns** — never implement an unaccepted plan.

## Preconditions
- Config gate: `.gogo/knowledge/` must exist (else tell the user to run `/gogo:build`).
- **Read the named knowledge set** (follow each file's `Source:` links for detail):

  | File | Why — for planning |
  |---|---|
  | `analysis.md` | **the analysis procedure** — how to analyze this feature before planning (start here) |
  | `project-knowledge.md` | architecture, domains, and the key decisions the change sits within |
  | `tech-stack.md` | how the project builds/runs/tests — the mechanics the plan must respect |
  | `non-functional-requirements.md` | the standing bars (perf/security/a11y/reliability) to design **within** |
  | `coding-rules.md` | the conventions the implementation will follow — plan reuse and shape accordingly |

  If a file is a bare scaffold (`Confidence: low`, empty `Source:`), wire it (or run
  `/gogo:build`) before relying on it.

## Steps
1. **Slug + folder.** Derive a kebab-case slug from the goal. Create
   `.gogo/work/feature-<slug>/`. If it already exists, you are **revising** — read the
   existing `plan.md`/`adjustments.md`/`state.md`; don't overwrite blindly.
2. **Analyze — follow `analysis.md`'s procedure against the real codebase** (code =
   source of truth). Working from the goal's nouns/verbs: restate the goal + its
   acceptance signal; Glob/Grep/Read to the **entry points + the modules/files the
   change touches**; **read the tests around those paths as the behavior spec**;
   check **recent git history** on them (`git log`); identify **reuse + blast radius
   + edge cases**; and surface the **risks/unknowns** that become the plan's
   alternatives/decisions. **When a doc/knowledge claim conflicts with the tree, the
   code wins** — verify against the code and note the drift.
   - **External-specs hook (conditional, capability-detected).** If the feature
     references an external spec/ticket **and** a docs capability is available (a
     `notion`/`confluence`/`atlassian`/`jira` MCP or skill), consult it for the
     spec, then reconcile against the code (the code wins for what exists today). If
     none is available, proceed from the code + the user's description and record the
     external ref as an assumption for the plan.
3. **Write `plan.md`** with this shape:
   - **Goal**
   - **Context** — what exists; the key code paths
   - **Functional requirements** — what this change must do (a feature's
     requirements live here, not in `.gogo/knowledge/`)
   - **Approach** (recommended) + alternatives considered
   - **Changes checklist** — files to add/modify, in build order
   - **Tests** — what will be verified, at which level
   - **Out of scope**
   - **Summary (TL;DR)** — the FINAL section (`## Summary (TL;DR)`, at the very
     end): 3-5 bold-led lines that close the plan — **what** is being built,
     **why**, the **chosen approach**, and **what happens next**. A skimmer who
     reads only this should get the whole shape.
   - `Status: awaiting acceptance`

   Design **within** the bars in `non-functional-requirements.md`.

   **Write it like a readable article (FR3 — legibility, keep the sections above).**
   `plan.md` is viewable in `/gogo:view` (see Step 4), so author it for a human
   reader — this is phrasing/emphasis only, **not** new sections beyond the
   closing `## Summary (TL;DR)` (D4=A):
   - **Lead with a 1-2 sentence summary** of the goal/approach before the detail.
   - **Short, scannable sections** — a few tight paragraphs, never walls of text.
   - **Bold the decisions, outcomes, and key terms** so a skim surfaces them.
   - Prefer **lists and tables** over long prose runs; plain language; define a
     term once, then reuse it.
   - **Close with `## Summary (TL;DR)`** — the final section: 3-5 bold-led lines
     (what's being built · why · the chosen approach · what happens next), so a
     reader who skims only the lead and this closing block still gets the plan.
   The viewer renders this with article typography (readable measure, styled
   headings, a lead paragraph, visible emphasis).
4. **Draw the intended design** (not the task list). Use the `gogo-mermaid` skill
   to diagram how the feature will *work* — the control/data flow, the runtime
   interaction between real components, or the domain states it touches — as a
   fenced block in `plan.md`, a `.mmd` in `charts/`, and the offline viewer.
   Label nodes with real endpoints/modules/states, **never** with FR numbers,
   build steps, or the gogo phases. If the change is pure process (docs/tests/
   merge/config) with nothing structural to show, skip the diagram and say so.

   **Also draw the "before" (as-is) baseline** (FR7). In addition to the
   intended-design diagram above, use `gogo-mermaid` to draw the UML of the
   *existing* flow the change will touch — captured now, before any code changes —
   into `charts/before/` (`charts/before/<kind>.mmd` + `charts/before/manifest.json`;
   same kinds/schema — see gogo-mermaid's "before" set). Keep it scoped to **what the
   change actually touches**, clearly the *before* baseline (separate from the
   intended-design diagram). Report ⑤ later draws the *after* set and compares the
   two. If the existing flow has nothing structural to show (a brand-new area, or a
   pure-process change), skip `charts/before/` and say so.

   **The plan is a viewable bundle (FR2 / D1=A — nothing to move).** `plan.md` (at
   the feature root) + its `charts/*.mmd` (intended design) + `charts/before/*.mmd`
   (as-is baseline) are already a coherent, viewer-ready set: `/gogo:view <slug>:plan`
   renders `plan.md` as an article plus these diagrams as interactive figures (rich
   flowchart cards + before/after compare), using the same renderer as reports. Keep
   the diagram set clean and per-kind so the plan page reads well; do **not** move
   `plan.md` into a `plan/` folder — its path is the contract every phase reads.
5. **Init state.** Copy `${CLAUDE_PLUGIN_ROOT}/templates/state.template.md` →
   `state.md` and `decisions.template.md` → `decisions.md`; create
   `adjustments.md` (header only). Set `state.md`: phase=plan,
   status=awaiting-plan-acceptance, created=<today>, iterations all 0.

   **Append the transition event (telemetry).** Beside this `state.md` write,
   append one compact JSON line to `.gogo/work/feature-<slug>/events.jsonl` per
   `events.schema.json` (`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`):
   `{"ts":"<RFC3339>","event":"phase-started","phase":"plan","status":"awaiting-plan-acceptance","slug":"<slug>"}`.
   Create the file if absent; **best-effort** — never fail the phase if the append
   fails (append-only telemetry; `state.md` stays the human resume file).
6. **Present + STOP.** Show the plan; ask the user to accept or request changes
   (`AskUserQuestion` with Accept / Request changes when the forks are clear).
   **Write no product code.**
   - Changes / clarification → append to `adjustments.md`, revise `plan.md`,
     re-present (stay in phase ①).
   - Accept → set `state.md` status=plan-accepted and add a top line to `plan.md`:
     `Status: **accepted** (user, <today>)`. Tell the user to run `/gogo:go`.
     **Append the transition event** (beside the `state.md` write, best-effort, per
     `events.schema.json`):
     `{"ts":"<RFC3339>","event":"plan-accepted","phase":"plan","status":"plan-accepted","slug":"<slug>"}`.
     `plan-accepted` is the plan phase's **terminal** event — this skill owns both
     plan events and there is no separate `phase-done`/plan (the orchestrator emits
     none).

## Hard rule
Never start implementing in this phase. Acceptance is the gate between plan and
implement — `/gogo:go` refuses unless `state.md` reads `plan-accepted`.
