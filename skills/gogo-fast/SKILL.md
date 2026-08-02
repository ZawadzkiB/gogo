---
name: gogo-fast
user-invocable: false
description: >-
  The token-lean ②→⑤ path for /gogo:go --fast — one warm build+verify context
  (implement + self-review + self-test, no per-round artifacts), ONE fresh-eyes
  review pass before UAT, an in-context final suite run, and a short report.
  Loaded only when the /gogo:go invocation carries --fast; the full pipeline
  path stays byte-identical without it.
---

# gogo-fast — fast mode (②→⑤ in one warm context + one fresh review)

Run this INSTEAD of the full ②③④⑤ routing in the `gogo` skill when the
`/gogo:go` invocation carries **`--fast`** (typed by the user, or appended by the
gogo CLI when the work item's SOURCE opted in via `fastMode`). Everything else is
inherited from the `gogo` skill unchanged: the config gate, the plan-acceptance
gate, decision gates, the UAT gate/loop, and `/gogo:done`.

**The trade, stated honestly:** fast mode drops the per-round fresh reviewer/
tester spawns and their artifacts (`review-NN.md`, `test-NN.md`, mid-loop
`issues.json` churn, mid-loop charts) in exchange for self-review/self-test
inside the warm implement context. It keeps the two quality mechanisms that
matter most — the accepted plan and **one fresh-eyes review before UAT** — and
surfaces remaining non-critical findings to the user at the UAT gate instead of
grinding rounds. Prefer the **full** pipeline for high-risk or security-sensitive
work; fast mode is for small/medium features where round-trips dominate cost.

## Inputs (declared) and outputs (typed)

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `plan.md` (accepted) | prose contract |
| in (required) | `coding-rules.md`, `tech-stack.md`, `code-review-standards.md`, `non-functional-requirements.md` | knowledge docs |
| in (optional) | `test-strategy.md` (the done-bar only) | knowledge doc |
| out | code changes | tree stays green |
| out | `implement/result.json`, `review/result.json`, `test/result.json`, `report/result.json` | `phase-result.schema.json` |
| out | `review/issues.json` (written by the ONE fresh review) | `issues-list.schema.json` |
| out (only if issues stay open) | `test/issues.json` | `issues-list.schema.json` |
| out | `report/report.md` (short), `charts/manifest.json` | prose / `charts-manifest.schema.json` |

**Never produced in fast mode:** `review-NN.md`, `test-NN.md`, mid-loop
`issues.json` updates, mid-loop charts. The events/state contract is unchanged —
same event vocabulary, same phase order (implement → review → test → report),
same belt-and-braces `state.md` writes at each phase's entry AND exit (the entry
write is prose an LLM follows and has been skipped in practice; the exit write is
what keeps the line moving — see the phase skills for the full rationale; the
same discipline applies here).

## ② Warm build + verify loop (in-context, no artifacts)

1. **Occupancy at entry** — write `state.md` `phase: implement` +
   `status: implementing`, **plus the additive marker line `- **mode:** fast`**
   (stamped here, at the first fast-mode write, so every reader — the board's
   `⚡ fast` chip included — knows the mode for the whole run; readers that don't
   know the key ignore it). Append
   `{"ts":"<RFC3339>","event":"phase-started","phase":"implement","status":"implementing","slug":"<slug>"}`.
2. **Build the plan** per `gogo-implement` §② steps 2–4 and 7 (work the Changes
   checklist in order; follow `coding-rules.md`; smallest correct change; keep the
   tree green; material forks → decision gate, never decided solo). Self-review
   against `code-review-standards.md` and self-test as you go — as **working
   practice**, writing NO findings artifacts and NO charts.
3. **Exit bar — OBJECTIVE, never self-assessed.** The loop ends only when, via
   real commands (from `tech-stack.md`) with real exit codes: build + typecheck +
   unit + the relevant e2e suites are green, AND every plan-checklist item is
   done. "It looks right to me" is not the bar; the commands are.
4. **Exit** — write `implement/result.json` (per `phase-result.schema.json`,
   validated via `gogo-contracts`), re-write `state.md` `phase`/`status`, bump
   `iterations: implement=<n+1>`, append `phase-done`/implement.

## ③ One fresh review pass (hard bound: 2 rounds total)

1. **Entry** — `state.md` `phase: review` + `status: reviewing`; append
   `phase-started`/review, then `round-opened` (round 1).
2. **Delegate ONCE** to `gogo-reviewer` via `Task`: the diff scope, `plan.md`,
   and the standards docs. There is no prior issues history to pass — it writes
   `review/issues.json` fresh (`issues-list.schema.json`; ids `REV-NNN`,
   `origin: review`, `found_in_round: 1`). **No `review-NN.md`** — the ⑤ report
   carries the findings table. Append `issues-found` if anything was found.
3. **Route by severity:**
   - **Blockers/criticals** → fix in the warm context (`state.md` flips to
     `implement`/`implementing` for the fix stretch — same entry/exit writes —
     with a `fix-round` event; set each fixed issue `status: fixed` +
     `fixed_in_round` + `fix_summary` per `gogo-implement` §② step 5), then **ONE
     delta re-review**: back to `review`/`reviewing`, re-spawn `gogo-reviewer`
     with ONLY the open issues + the fix diff (append `round-opened`, round 2 —
     not a full re-review). A critical still open after round 2 → **decision at
     the UAT gate**: log it to `decisions.md` (`D<n>`), list it in the report; do
     NOT grind more rounds.
   - **Majors/minors** → leave `open` in `issues.json`; they go to the user in
     the ⑤ report's findings table to accept or bounce at UAT.
   - **`needs-user-decision`** finding → decision gate per the `gogo` skill
     (`decisions.md` + `waiting-for-user`, resume: review).
4. **Exit** (routing forward only) — `review/result.json` with
   `open_issues: <count of open/new>`; re-write `state.md`, bump
   `iterations: review=<n>`, append `phase-done`/review.

## ④ Final suite run (in-context — no tester spawn)

1. **Entry** — `state.md` `phase: test` + `status: testing`; append
   `phase-started`/test.
2. **Run the full suites once** (after any ③ fixes): build, unit, e2e — the
   done-bar from `test-strategy.md`. Add a **targeted** hands-on smoke check
   where the change warrants it (UI via the bundled Playwright MCP, CLI via
   shell, API via HTTP) — prefer `browser_find`/verify-style targeted calls over
   full-page snapshots; screenshot only failures.
3. **A blocked hands-on check is a user decision gate** — never a silent skip;
   exactly `gogo-test`'s rule (record in `decisions.md`, `waiting-for-user`,
   resume: test; only the user may skip).
4. **Failures** → fix in the warm context, re-run (still no artifacts). Bound ~2
   re-runs on the same failure → decision gate.
5. **Green** — `test/result.json` (summary; `test/issues.json` ONLY if something
   stays open for the user to see); re-write `state.md`, bump
   `iterations: test=<n>`, append `phase-done`/test.

## ⑤ Short report → the UAT gate

1. Append `phase-started`/report (events call this phase `report`).
2. **Write `report/report.md` — 1–2 pages, not a dossier:** what shipped vs the
   plan (the delta — link `plan.md`, don't restate it), the findings table
   (fixed / **open for your acceptance** / decisions), the test summary, and any
   `decisions.md` outcomes. This file must exist — it is what makes the item
   report-complete for `/gogo:done` and the board.
3. **Charts only if the change genuinely warrants one.** Apply
   `gogo-implement`'s diagram-subject rules with a high bar: at most the one or
   two kinds with real signal (typically a flow), via `gogo-mermaid`, into
   `charts/`. Otherwise write `charts/manifest.json` with empty `diagrams` and a
   note (`"fast mode - no chart-worthy change"`). Charts happen at most ONCE,
   here — never mid-loop.
4. Tick the plan's checklist (no full as-built rewrite). Update
   `.gogo/knowledge/*` only on material drift (default: skip).
5. **Exit** — `state.md`: `phase: done`, `status: awaiting-uat` (the `mode: fast`
   line from ② stays); write `report/result.json`; append `phase-done`/report.
6. **Tell the user:** what to verify, where the report is, that `/gogo:done`
   ships (open findings in the table will be recorded as **accepted-by-user** —
   see `gogo-done`), and that feedback loops back via the normal UAT loop (the
   `gogo` skill owns it, unchanged — including a full-pipeline rerun if they want
   the thorough path for the fix).

## Bounds (the whole point)

Review: **2 rounds max.** Suite re-runs on the same failure: **~2 max.**
Anything that resists a bound becomes a decision — at the gate or in the UAT
report — never another round. If mid-run you find the feature is riskier than
fast mode should carry (security surface, destructive migrations, sprawling
diff), STOP and tell the user to rerun without `--fast`.
