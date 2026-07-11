---
name: gogo-implement
user-invocable: false
description: >-
  Phase ② of the gogo pipeline — implement an accepted plan, and re-enter to
  apply review/test fixes from a typed issues list. The operating manual for the
  gogo-developer agent. Builds only what the accepted plan describes; emits the
  as-built chart set; keeps the build and tests green.
---

# gogo-implement — phase ② (build the accepted plan / fix open issues)

This skill is the operating manual for the **`gogo-developer`** agent (and for
the orchestrator when it implements in-context). You are the *implementer* — the
coordination and the decision gates belong to the orchestrator.

Two modes, same skill (idempotent, input-driven):
- **Plain** `/gogo:implement <slug>` — build the accepted plan from scratch.
- **`--issues <path>`** — fix the open issues in a typed issues list (a
  review/test loop-back) and write back what was fixed.

**`--in-session`** (orthogonal flag): run this skill **in the current session,
in-context** — do not delegate to a fresh `gogo-developer` `Task`. This is the mode
the **CLI process-orchestrator** (`gogo run`) drives over `claude -p`, so it can
`--resume` the SAME warm worker across fix rounds (a delegated inner `Task` would be a
cold shell on resume). Same steps below; only the executor differs.

## Inputs (declared) and outputs (typed)

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `plan.md` (accepted) | prose contract |
| in (required) | `coding-rules.md`, `tech-stack.md` | knowledge docs |
| in (optional) | `--issues <path>` (`review/issues.json` or `test/issues.json`) | `issues-list.schema.json` |
| out | code changes | tree stays green |
| out | `charts/` as-built set + `charts/manifest.json` | `charts-manifest.schema.json` |
| out | the same `issues.json` (fixed back, when `--issues`) | `issues-list.schema.json` |
| out | `implement/result.json` (per run) | `phase-result.schema.json` |

## ① validate-in (gate — FR2)

Via `gogo-contracts`: confirm `state.md` is `plan-accepted` (first pass) or a
resumable in-loop state, and `plan.md` exists. **If `--issues <path>` is given,**
validate that file against `issues-list.schema.json` (structural + semantic:
right slug, real paths, unique ids, valid enums). Missing/invalid required input
→ **STOP** with a precise contract error; never build on bad input. **Never
implement an unaccepted plan.**

## ② Steps

1. **Pick the work set:**
   - **Plain mode** → work the `plan.md` **Changes checklist** in order, scoped to
     the plan.
   - **`--issues` mode** → fix every issue whose `status` is `open` or `new`
     (skip `verified`/`wontfix`). Address exactly those findings, using each
     issue's `proposed_solution` as the guide.
2. Follow `coding-rules.md`; match surrounding code. Smallest correct change; no
   opportunistic refactors outside the plan.
3. Keep it green: run build / typecheck / unit (commands from `tech-stack.md`)
   and fix what you break. Don't leave the tree broken.
4. **Write fixes back into the issues list** (`--issues` mode — FR6). For each
   issue you fixed, set `status: fixed`, `fixed_in_round: <this round>`, and a
   one-line `fix_summary` of what you changed. Leave anything you intentionally
   skipped as `wontfix` with the reason in `fix_summary`. Do **not** flip to
   `verified` — that's the next review/test's job. Bump the list's `round`/`updated`.
5. **Emit the as-built chart set** via `gogo-mermaid` (FR7). Diagram the *shipped
   product* — never the gogo phases or the plan's task checklist. Produce only the
   kinds that carry signal (per the diagram-subject rules): **flow** (control/data
   flow), **sequence** (key runtime interaction), **class** (structure/types),
   **activity/state** (a new state machine or action flow). Skip any that would be
   trivial; if the change is pure process, draw nothing. Write each as a `.mmd` in
   `charts/`, refresh `charts/diagrams.html`, and write `charts/manifest.json`
   listing the kinds/files/titles you produced (empty `diagrams` + a `note` if you
   drew nothing). Review ③ and test ④ consume this set.
6. Small, obvious plan corrections → make them and note in `plan.md`. A
   **material** change, a new fork, or anything destructive/irreversible →
   **don't decide it**: return it as a decision for the orchestrator (it owns the
   gate), with enough context to log to `decisions.md`.
7. Commit only if the user has asked for commits (gogo defers to the user on
   commits). If committing, use small safe increments.

## ③ validate-out (gate — FR3)

Via `gogo-contracts`: validate `charts/manifest.json` against
`charts-manifest.schema.json`, and (in `--issues` mode) the updated `issues.json`
against `issues-list.schema.json` (every `fixed` issue now has `fixed_in_round` +
`fix_summary`). Repair once on failure; if still failing, write
`implement/result.json` with `status: blocked`, `validated_out: false` and stop.
On success, write `implement/result.json` (`phase: implement`, `status: ok`,
`inputs`, `outputs`, `validated_in: true`, `validated_out: true`, `summary`).

## ④ Update state

Update `state.md`: phase=implement, status=implementing, bump
`iterations: implement=<n+1>`. (`issues.json`/`charts/manifest.json`/`result.json`
are the machine state; `state.md` stays the human-facing file.)

**Append the transition event(s) (telemetry).** Beside this `state.md` write,
append compact JSON line(s) to `.gogo/work/feature-<slug>/events.jsonl` per
`events.schema.json` (`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`). First the
**entry** event — **plain mode** → `{"ts":"<RFC3339>","event":"phase-started","phase":"implement","status":"implementing","slug":"<slug>"}`;
**`--issues` mode** (re-entering to fix) →
`{"ts":"<RFC3339>","event":"fix-round","phase":"implement","status":"implementing","round":<this round>,"slug":"<slug>"}`.
Then, because `implement/result.json` was written `ok` in ③ (validate-out passed —
this run hands off to review), the phase's **terminal** event (this skill owns
`phase-done`/implement; the orchestrator no longer emits it):
`{"ts":"<RFC3339>","event":"phase-done","phase":"implement","status":"implementing","slug":"<slug>"}`
(emit it *after* the entry event so ordering reads start → done). A run that stops
`blocked` in ③ never reaches here, so `phase-done` marks only a successful hand-off.
Create the file if absent; **best-effort** — never fail the phase if the append
fails (append-only telemetry; `state.md` stays the human resume file).

## Return

A concise summary: what you changed (files), what's green, which issues you fixed
(ids + fix_summary), and anything you couldn't decide (forks to escalate). Hand
back to the orchestrator → review.

## Degradation

If `git` is unavailable, track touched files via the plan's Changes checklist so
the review phase still has a scope to work from. If `mmdc` is absent, the `.mmd`
sources + the offline viewer are still the durable charts (never install a
renderer). Contract validation degrades per `gogo-contracts` — when no
`jq`/schema validator is present, the agent checks the document against the schema
directly (the semantic checks always run).
