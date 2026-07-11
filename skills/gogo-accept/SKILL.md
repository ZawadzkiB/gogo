---
name: gogo-accept
user-invocable: false
description: >-
  The plan-acceptance gate, reachable from the board (Slice C of
  unattended-ops-input-signals). Given a feature slug, it resolves the feature,
  refuses unless state.md status is awaiting-plan-acceptance, presents plan.md for
  the user to eyeball, and — on the user's confirmation — records acceptance EXACTLY
  as gogo-plan does (state.md → plan-accepted, the Status: **accepted** line on
  plan.md, open-decision cleared, the single-owner plan-accepted event), then stops.
  Accept-only: it does not chain into /gogo:go. Invoked by /gogo:accept, which the Go
  board launches when the user presses `m` on an awaiting-plan-acceptance card — so a
  plan-pending card the board now SHOWS can also be CLEARED from the board, never a
  blind CLI state-flip (the CLI never mutates pipeline state).
---

# gogo-accept — the board-reachable plan-acceptance gate

This skill is the thin present-then-record step behind `/gogo:accept <slug>`. Its
**only** job is to let a human eyeball a written plan and, on confirmation, record
acceptance — **reusing gogo-plan's recording, never inventing a second acceptance
path**. It writes no product code and runs no pipeline phase.

It exists so the board's `m` on an `awaiting-plan-acceptance` card has a working
move: the Go CLI **launches** this command in an attachable session (it never
mutates pipeline state itself); this session does the state write, exactly as the
in-chat plan-acceptance gate does.

## Inputs / outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `<slug>` | the feature to accept |
| in (required) | `plan.md`, `state.md` | the feature folder |
| out | `state.md` → `plan-accepted`, `open-decision: none` | human resume file |
| out | `plan.md` top line `Status: **accepted** (user, <today>)` | the contract |
| out | one `plan-accepted` line appended to `events.jsonl` | `events.schema.json` |

## Steps

1. **Resolve the feature.** From the slug, find `.gogo/work/feature-<slug>/`. If it
   does not exist → **STOP** with a precise error (unknown feature).
2. **Gate on status (hard).** Read `state.md`. Proceed **only** when
   `status: awaiting-plan-acceptance`. Otherwise **STOP** and guide:
   - `plan-accepted` (or any downstream state) → "already accepted — run
     `/gogo:go <slug>` to build it."
   - `implementing` / `reviewing` / `testing` / `awaiting-uat` / `waiting-for-user`
     / `shipped` / `done` → "nothing to accept here — this feature is <status>."
   **Never** record acceptance for a feature that is not at the plan-acceptance gate.
3. **Present the plan.** Show `plan.md`'s summary (Goal / Functional requirements /
   Approach / Changes checklist / Out-of-scope) plus any open decisions from
   `decisions.md`, so the user eyeballs what they are accepting. This is the
   built-in eyeball (no prior `v` view is required — D10).
4. **Ask to accept.** Use `AskUserQuestion` (Accept / Request changes) or prose.
   - **Request changes / anything but a clear accept** → do **not** record; tell the
     user to take it back through `/gogo:plan` (planning owns plan revision). Stop.
   - **Accept** → step 5.
5. **Record acceptance — exactly as `gogo-plan` does** (one owner of the recording;
   do not invent a second path):
   - `state.md`: set `status: plan-accepted`; set `open-decision: none`; leave
     `phase: plan`. Record the acceptance on the `accepted:` line
     (`<today> (user, via /gogo:accept)`).
   - `plan.md`: add the top line `Status: **accepted** (user, <today>)` (replace an
     existing `Status:` line if present).
   - **Append the single-owner terminal event** beside the `state.md` write
     (best-effort, per `events.schema.json` in `${CLAUDE_PLUGIN_ROOT}/templates/contracts/`):
     `{"ts":"<RFC3339>","event":"plan-accepted","phase":"plan","status":"plan-accepted","slug":"<slug>"}`.
     `plan-accepted` is the plan phase's terminal event — **gogo-plan owns it, and so
     does this skill on its behalf** (there is exactly one emitter per acceptance;
     never emit a second `plan-accepted` for the same acceptance).
6. **Stop — accept-only (D9).** Tell the user acceptance is recorded and to run
   `/gogo:go <slug>` (or press `m` on the now-`plan-accepted` card) to build it. Do
   **not** chain into `/gogo:go`.

## Hard rules

- **Accept-only, never build.** This skill records the gate and stops. It never runs
  a pipeline phase and writes no product code.
- **Gate before recording.** Only an `awaiting-plan-acceptance` feature can be
  accepted here; anything else stops with guidance.
- **One acceptance recording.** Reuse gogo-plan's recording (state flip + plan line
  + single-owner `plan-accepted` event). Never a second acceptance path or a second
  event emitter.
