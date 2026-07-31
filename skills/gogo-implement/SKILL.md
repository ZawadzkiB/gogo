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
resumable in-loop state, and `plan.md` **exists and is written** - present with at least
two `## ` sections, not a scaffold stub. A folder whose `state.md` says
`plan-accepted` while `plan.md` is absent or a stub was **never planned**: the readers
refuse it (`gogo go` and the board's `m`/`M` both bounce), so STOP and send the user to
`/gogo:plan <slug>` rather than inventing a plan here. **If `--issues <path>` is given,**
validate that file against `issues-list.schema.json` (structural + semantic:
right slug, real paths, unique ids, valid enums). Missing/invalid required input
→ **STOP** with a precise contract error; never build on bad input. **Never
implement an unaccepted plan.**

## ② Steps

1. **Record occupancy - FIRST, before you read a line of product code (FR11).** Write
   `state.md` `phase: implement` + `status: implementing`, and append the entry event (the
   `phase-started` / `fix-round` line specified in §④). Only then continue to step 2.

   `state.md` is what every reader believes - the board's column, `gogo status`, the
   concurrency cap, `pages`, any headless consumer. Written ONLY at §④, after the work, the line
   records that implementing just **finished**: for the whole of a build the disk still says
   `plan-accepted`, so the card sits in the **plan** column, the header reads
   `in progress 0`, and a **second `gogo go` in the same repo is allowed to clobber the
   working tree**. Recording occupancy at entry makes the file describe what is happening
   *now*.

   §④ writes `phase`/`status` **again** at the end, and bumps `iterations` there. That
   duplication is deliberate belt-and-braces, not redundancy to tidy away: this write is prose,
   and if you skip it the §④ write is what still moves the line (see §④). Do **not** bump
   `iterations` here - that is a completion count. The write
   is idempotent: on a resume the values are already correct, so re-writing them changes
   nothing. (This is **step 1 of the numbered flow on purpose** - it shipped once as a
   separate `①b` section and the very next phase skill to run skipped it. The move helped less
   than hoped: the write has been skipped on **all three** live runs so far, twice after the
   move, which is why §④ writes `phase`/`status` again rather than trusting this one.)

   **The CLI does not depend on this for safety**, and does not depend on it for visibility
   either: the cap counts a live `gogo-go-<slug>` session whatever any file says; a card
   whose status has not caught up reads `● building`; a killed phase reads `· stalled`; and
   if you skip this write, the board reads **`· state lags`** as soon as the telemetry
   contradicts the phase line while a build session is live - either the previous phase's
   `phase-done` is the newest event, or an **entry event names a different phase than the line
   does**, which is exactly the shape a `--issues` re-entry leaves when you append `fix-round`
   but skip the `state.md` half. So a skipped occupancy write is usually *visible* rather than
   silent - but it is still wrong, because it makes every reader describe the previous phase,
   and the cue is a detector, not a guarantee (a later mid-phase event can mask it).

2. **Pick the work set:**
   - **Plain mode** → work the `plan.md` **Changes checklist** in order, scoped to
     the plan.
   - **`--issues` mode** → fix every issue whose `status` is `open` or `new`
     (skip `verified`/`wontfix`). Address exactly those findings, using each
     issue's `proposed_solution` as the guide.
3. Follow `coding-rules.md`; match surrounding code. Smallest correct change; no
   opportunistic refactors outside the plan.
4. Keep it green: run build / typecheck / unit (commands from `tech-stack.md`)
   and fix what you break. Don't leave the tree broken.
5. **Write fixes back into the issues list** (`--issues` mode - FR6). For each
   issue you fixed, set `status: fixed`, `fixed_in_round: <this round>`, and a
   one-line `fix_summary` of what you changed. Leave anything you intentionally
   skipped as `wontfix` with the reason in `fix_summary`. Do **not** flip to
   `verified` — that's the next review/test's job. Bump the list's `round`/`updated`.
6. **Emit the as-built chart set** via `gogo-mermaid` (FR7). Diagram the *shipped
   product* — never the gogo phases or the plan's task checklist. Produce only the
   kinds that carry signal (per the diagram-subject rules): **flow** (control/data
   flow), **sequence** (key runtime interaction), **class** (structure/types),
   **activity/state** (a new state machine or action flow). Skip any that would be
   trivial; if the change is pure process, draw nothing. Write each as a `.mmd` in
   `charts/`, refresh `charts/diagrams.html`, and write `charts/manifest.json`
   listing the kinds/files/titles you produced (empty `diagrams` + a `note` if you
   drew nothing). Review ③ and test ④ consume this set.
7. Small, obvious plan corrections → make them and note in `plan.md`. A
   **material** change, a new fork, or anything destructive/irreversible →
   **don't decide it**: return it as a decision for the orchestrator (it owns the
   gate), with enough context to log to `decisions.md`.
8. Commit only if the user has asked for commits (gogo defers to the user on
   commits). If committing, use small safe increments.

## ③ validate-out (gate — FR3)

Via `gogo-contracts`: validate `charts/manifest.json` against
`charts-manifest.schema.json`, and (in `--issues` mode) the updated `issues.json`
against `issues-list.schema.json` (every `fixed` issue now has `fixed_in_round` +
`fix_summary`). Repair once on failure; if still failing, write
`implement/result.json` with `status: blocked`, `validated_out: false` and stop.
On success, write `implement/result.json` (`phase: implement`, `status: ok`,
`inputs`, `outputs`, `validated_in: true`, `validated_out: true`, `summary`).

## ④ Update state (exit)

Update `state.md`: **`phase: implement`, `status: implementing`**, and bump
`iterations: implement=<n+1>` - the *completion* count.

**Scope: only on the routes that CONTINUE.** A route that parks the item at a **user gate**
(a decision gate → `waiting-for-user`, and for ④ a blocked hands-on check → the same) has
already written the status that matters, and it is the one status a reader must not lose:
it feeds the `⏸ K need you` count, the card's gate stripe, and the `/gogo:go` refusal that
stops an unattended rerun. **Never overwrite a gate status with the working status** - write
`phase`/`status` here only when the round loops back to ② or advances to the next phase.
(`issues.json`/`charts/manifest.json`/`result.json` are the machine state; `state.md`
stays the human-facing file.)

**Write phase/status here EVEN THOUGH §② step 1 already did. The redundancy IS the design -
do not "clean it up".** 0.29.0 briefly dropped this exit write on the theory that the entry
write covers it. It does not: the entry write is **prose an LLM follows**, and it was skipped
on all three of its first live runs. With only the entry write, `state.md` stops advancing at
all - it sticks at whatever phase last actually wrote it, which is *worse* than the
one-phase-behind lag this release set out to fix. Two writers, one at each end, means the
floor is that old one-phase lag and the ceiling is the entry write's accuracy. One of the two
is an LLM following prose, so the other one is what makes the line move at all.

**Append the terminal event (telemetry).** The **entry** event was appended at §② step 1 -
**plain mode** → `{"ts":"<RFC3339>","event":"phase-started","phase":"implement","status":"implementing","slug":"<slug>"}`;
**`--issues` mode** (re-entering to fix) →
`{"ts":"<RFC3339>","event":"fix-round","phase":"implement","status":"implementing","round":<this round>,"slug":"<slug>"}`.
Do **not** re-emit it here. Now, because `implement/result.json` was written `ok` in ③
(validate-out passed - this run hands off to review), append the phase's **terminal**
event to `.gogo/work/feature-<slug>/events.jsonl` per `events.schema.json`
(`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`) - this skill owns
`phase-done`/implement; the orchestrator no longer emits it:
`{"ts":"<RFC3339>","event":"phase-done","phase":"implement","status":"implementing","slug":"<slug>"}`.
Because step 1 already appended the start, the stream now reads start → done **with the
work between them**, in real time - not both lines in one burst at the end (which made
`events.jsonl` a post-hoc log rather than a live stream). A run that stops `blocked` in
③ never reaches here, so `phase-done` marks only a successful hand-off. Create the file
if absent; **best-effort** - never fail the phase if the append fails (append-only
telemetry; `state.md` stays the human resume file).

## Return

A concise summary: what you changed (files), what's green, which issues you fixed
(ids + fix_summary), and anything you couldn't decide (forks to escalate). Hand
back to the orchestrator → review.

## Degradation

If `git` is unavailable, track touched files via the plan's Changes checklist so
the review phase still has a scope to work from. If `very-nice-mermaid` is
absent, the `.mmd` sources + the offline viewer are still the durable charts
(never install a renderer mid-run). Contract validation degrades per `gogo-contracts` - when no
`jq`/schema validator is present, the agent checks the document against the schema
directly (the semantic checks always run).
