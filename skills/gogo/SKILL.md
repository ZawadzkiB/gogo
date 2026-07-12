---
name: gogo
user-invocable: false
description: >-
  The project's development pipeline — plan → implement → review → test → report.
  Use for ANY non-trivial change: a new feature, a meaningful refactor, a
  behavioural bug fix, anything touching multiple files or with design choices.
  Triggers when the user runs /gogo:plan or /gogo:go, says "build", "implement",
  "add", "develop", or describes a feature/change. This skill is the
  orchestrator's operating manual: the phases, the loops, the decision gates, the
  feature-folder state, and which knowledge files each phase reads.
---

# gogo — the plan → implement → review → test → report pipeline

This is how non-trivial work happens in a gogo-enabled project. **Never
free-style a non-trivial change** — drive it through the phases below, looping
back as findings demand, and **stopping for the user whenever a decision is
theirs to make.**

Trivial work (a typo, an obvious one-line fix, a rename, a doc tweak) does NOT
need the pipeline — just do it.

You may be running this **in the chat** (the default for `/gogo:go` — you can
pause for the user at any gate) or **as the spawned `gogo` agent** (a hands-off
run — when you hit a gate you can't ask interactively, so you stop and *return*
the decision to the chat, which asks the user and re-invokes you). The flow is
identical either way.

## 0. Config gate — check before anything else

Confirm `.gogo/knowledge/` exists and is non-empty. If it's missing, **STOP** and
tell the user:

> gogo isn't configured for this project yet — run `/gogo:build` first.

Do not invent project rules or proceed without config.

## Knowledge — read before you plan, review, or test

The pipeline is grounded in `.gogo/knowledge/` (proxies that link to the
project's real docs — follow the links, don't assume). Read what's relevant:

| File | Read in phase |
|---|---|
| `analysis.md` | plan |
| `project-knowledge.md` | plan |
| `tech-stack.md` | plan, implement, test |
| `non-functional-requirements.md` | plan, review, test |
| `coding-rules.md` | implement, review |
| `code-review-standards.md` | review |
| `testing-tools.md` | test |
| `test-strategy.md` | test |

`index.md` is the purpose-map for the folder. If a file is still a bare scaffold
(`Confidence: low`, empty `Source:`), wire it (or run `/gogo:build`) before
relying on it.

A knowledge file may point to an **on-demand skill** under `.gogo/skills/` via a
`**Load when:** <trigger> → <path>` pointer (extracted by `/gogo:skills` to keep
the always-read file lean). **Load a pointed skill only when the task actually
touches it** — that's the whole point: keep always-read context small and pull in
the detail on demand. `.gogo/skills/index.md` lists what exists.

The optional **`gogo` CLI** (a companion binary — deterministic board/cockpit +
persistent-session launcher) has its own on-demand plugin reference; keep this
always-read manual lean and pull the detail only when it is relevant.
**Load when:** the gogo CLI is relevant (the user asks to launch/attach/sweep
sessions, manage/view existing work from the terminal, mentions `gogo` / the
board, or you're weighing the CLI vs the in-chat flow) → `skills/gogo-cli`.

## Feature workspace

Everything for one piece of work lives in **`.gogo/work/feature-<slug>/`** (kebab
slug from the feature name). These files are the pipeline's memory + audit trail:

- `plan.md` — the accepted plan (the contract), incl. the feature's *functional* requirements
- `adjustments.md` — running log of user-requested changes/clarifications during planning
- `state.md` — current phase / status / iteration counters / resume info
- `decisions.md` — open/closed forks that needed the user
- `uat.md` — the UAT gate log (appears once ⑤ reaches `awaiting-uat`): one round per user check — a `/gogo:done` accept line, or an analyst-authored issues round (verbatim input + analysis + plan delta + disposition + verdict) when feedback loops back
- `review/issues.json` — the living, typed review findings (the contract); `review-NN.md` renders each round's snapshot
- `test/issues.json` — the living, typed test findings (same contract); `test-NN.md` renders each round's snapshot
- `events.jsonl` — append-only progress telemetry: one schema'd JSON line per phase transition (beside every `state.md` write), for the `gogo` CLI cockpit; a missing file is never an error
- `report/` — the as-built bundle (written at ⑤): `report/report.md` (planned-vs-shipped, implementation, decisions+reasons, review/test outcomes), the UML set (`.mmd` chosen by the diff), `report/before/` (the plan-time "before" set copied in for a self-contained before/after compare), `diagrams.html`, `manifest.json`, `result.json`. This is the full audit trail; `/gogo:done` **synthesizes** a high-level entry from it into `.gogo/changelog/` (it does not copy the bundle).
- `charts/` — mermaid `.mmd` + `manifest.json` + offline `diagrams.html` (plan's intended design + `charts/before/` the plan-time as-is baseline; ② emits the as-built flow/sequence/class/activity set for review/test)

A **high-level synthesis** of the shipped work is archived (chronologically) under `.gogo/changelog/<YYYY-MM-DD>-<name>/` once the user runs `/gogo:done` — one or several related features can ship as a single merged release entry; the full detail stays in `.gogo/work/`.

The typed artifacts (`*/issues.json`, `charts/manifest.json`, per-run
`result.json`, the feature `pipeline.json`) follow JSON Schemas in
`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`; each phase validates them in/out via
the `gogo-contracts` skill (portable: `jq`/schema if present, else agent-checks).

Create the folder in the plan phase (copy `state.md`/`decisions.md` from
`${CLAUDE_PLUGIN_ROOT}/templates/`). **Keep `state.md` current at every phase
transition** so a fresh session — or a resume after a user decision — picks up
exactly where it left off.

## The flow

```
user goal ─▶ ① PLAN ──(user accepts)──▶ ② IMPLEMENT ─▶ ③ REVIEW ─▶ ④ TEST ─▶ ⑤ REPORT ─▶ UAT gate ──(/gogo:done accepts)─▶ shipped
              ▲  │                            ▲            │           │   (awaiting-uat:      (synthesize →
              │  └──(clarify / changes)──▶ wait            │           │    user verifies)     .gogo/changelog/)
              │                                └──issue─────┘           │        │
              │                                  (fix → re-review, ≤3)  │        └──UAT feedback → uat.md round → adjust plan
              │                                                         │           (SAME item) → re-accept → /gogo:go reruns ②→⑤
              └──── issue needs a USER DECISION (from review or test) ──┘
```

The **UAT gate** is the plan-gate symmetry at the exit: ⑤ ends at `status: awaiting-uat`
(not `done`); running `/gogo:done` **is** the acceptance, or the user's issues/questions
loop back into planning on the **same work item** (see *The UAT gate* below).

## Who runs each phase

**Commands invoke the orchestrator; it runs ② implement in-context and delegates
the fresh-eyes phases (①③④) to specialist agents, owning the gates in chat.**
Concretely:

- **You (the orchestrator) own the gates in chat** — the plan-acceptance gate after
  ①, every decision gate, and the ⑤ report step.
- **② implement → you run it yourself, in-context** (follow the `gogo-implement`
  skill). Keep this context **warm across the whole implement↔review↔test fix
  loop** — that is the point: a re-spawned developer would re-read `plan.md` and
  re-explore the codebase from scratch every round; running ② in-context means you
  read the tree once and apply later review/test fixes **without re-exploring**.
- You **delegate the fresh-eyes phases** via the `Task` tool, each to a
  **fresh-context** specialist (fresh context = unbiased eyes, which ①③④ need):
  - ① plan → **`gogo-analyst`** agent (follows the `gogo-plan` skill) — reads the
    named knowledge set incl. `analysis.md`, analyses the goal against the real
    codebase (**code = source of truth**), drafts `plan.md` + the intended-design
    charts, and STOPs for acceptance (you own that gate).
  - ③ review → **`gogo-reviewer`** agent (follows the `gogo-review` skill) — it must
    NOT have written the code it reviews.
  - ④ test → **`gogo-tester`** agent (follows the `gogo-test` skill) — a fresh
    perspective plus the bundled Playwright MCP.
  - ⑤ report → you run the `gogo-knowledge` skill in chat.
- A delegated worker that hits a real fork **returns** it to you; you handle the
  gate (below) and re-delegate with the answer.
- `gogo-developer` still backs **standalone `/gogo:implement`** and **hands-off**
  runs — only the orchestrator's *default routing* for ② is in-context.

If browser/agent tooling is unavailable, you may run a phase's skill yourself
in-context instead of delegating — the phase skills are written to run either way.

## The phases

### ① Plan → skill `gogo-plan` (delegate to `gogo-analyst`)
Delegated to the **`gogo-analyst`**: it reads the named knowledge set (incl.
`analysis.md`), analyses the goal against the actual codebase (**code = source of
truth**, following `analysis.md`'s procedure), creates `.gogo/work/feature-<slug>/`,
writes `plan.md` (Goal / Context / Functional requirements / Approach +
alternatives / Changes checklist / Tests / Out-of-scope), draws the intended design
with `gogo-mermaid`, and inits `state.md`. **Present the plan and STOP for
acceptance** — you (the orchestrator) own that gate. Changes/clarification → log to
`adjustments.md`, revise, re-present. **Do not implement until the user accepts.**
Hard gate.

### ② Implement → skill `gogo-implement` (you run it in-context)
Build the accepted `plan.md` following `coding-rules.md`; keep changes scoped;
keep build/typecheck/unit green. **Run this yourself, in-context** — do not delegate
to a fresh `gogo-developer`, so your understanding of the code carries across the
loop. Re-enter here to apply review/test fixes **without re-exploring the tree**.
(`gogo-developer` still backs standalone `/gogo:implement` and hands-off runs.)

### ③ Review → skill `gogo-review` (delegate to `gogo-reviewer`)
Review the diff against `code-review-standards.md` + `non-functional-requirements.md`.
Findings → `review/issues.json` (the living, typed contract) + a `review-NN.md`
rendered snapshot per round.
- **Fixable** → back to ② (fix), then re-review. Bound: if the same issue resists
  ~3 rounds, treat it as a decision and stop.
- **Needs a user decision** → decision gate (below).
- **Clean** → ④.

### ④ Test → skill `gogo-test` (delegate to `gogo-tester`)
e2e at every relevant level per `test-strategy.md`/`testing-tools.md` — UI
(bundled Playwright MCP), CLI, API — plus exploration (does it work? does it look
right?). Results → `test/issues.json` (the living, typed contract) + a
`test-NN.md` rendered snapshot per round.
- **Issue (fixable)** → back to ② → ③ → ④.
- **Issue needing a user decision** (a code/scope fork) → back to ① (re-plan how
  to handle it, re-accept), via a decision gate.
- **Hands-on/e2e check can't run** (no emulator/device/browser/dev-server/app, or
  a failed connection) → **user decision gate**, resuming at ④ — *never a silent
  skip*. Ask the user: help set up the env and retry (e.g. they boot the emulator
  + app, you reconnect), try an alternative, or explicitly skip. **Only the user
  may skip** a hands-on check.
- **All green** (incl. every relevant hands-on check run or user-skipped) → ⑤.

### ⑤ Report → skill `gogo-knowledge`
Update `plan.md` to as-built; draw the as-built UML set (chosen by what changed —
class / sequence / activity / use-case / flow) via `gogo-mermaid` into the
feature's `report/` folder; write the final `report/report.md` (planned-vs-shipped,
**implementation**, **decisions + reasons**, review/test outcomes, diagram + audit
links); update whatever `.gogo/knowledge/*` drifted (gogo-owned summaries only —
never the proxied originals, and **never a `## Custom` section**); set `state.md` to
**`awaiting-uat`** (the UAT gate — no longer `done`); summarise to the user (point them
at `report/report.md` and `report/diagrams.html`, and tell them to verify the work).

The in-pipeline ⑤ keeps a strict gate (green ④ required). Run **standalone via
`/gogo:report <feature>`, it is lenient** — it also reports on a past/broken/
incomplete run, synthesizing a best-effort `report/report.md` from whatever exists
and marking which phases ran and what's still open (`plan.md` is the one
prerequisite).

### UAT → the gate between ⑤ and ship (the plan-gate symmetry)
⑤ leaves the feature at **`status: awaiting-uat`** — the user verifies the shipped work.
This mirrors the ① plan-acceptance gate, at the *exit* instead of the entrance, and there
is **no extra confirmation question**. The user does exactly one of two things:

- **Accepts by running `/gogo:done`** — that command *is* the acceptance (its validate-in
  requires `awaiting-uat`; it appends the accept round to `uat.md`, emits `uat-passed`, and
  ships). You do nothing here; `/gogo:done` owns it.
- **Raises questions/issues instead** — then **you (the orchestrator) run the UAT loop**,
  treating it exactly like a decision gate:
  1. **Lock the gate BEFORE delegating anything.** The moment the user raises UAT issues,
     set `state.md` `status: waiting-for-user`, `open-decision: UAT round N`,
     `resume: plan`, and emit **`uat-opened`**
     (`{"event":"uat-opened","phase":"report","status":"waiting-for-user","note":"UAT round N: <one line>","slug":"<slug>"}`).
     The feature **stays `waiting-for-user` for the whole re-plan stretch** — analysis, plan
     revision, and re-presentation — so it is never `awaiting-uat` (which would classify it
     ready-to-ship and let `/gogo:done` ship an un-re-implemented plan) and never
     `plan-accepted` (which would let `/gogo:go` rerun) until the user actually re-accepts.
  2. **Delegate to `gogo-analyst`** (its second job): analyse the user's input against the
     current `plan.md` + `decisions.md` **and THE CODE** (code = source of truth), and
     propose the plan delta. The analyst **appends a `uat.md` round** — verbatim user input,
     its analysis, the proposed plan delta, and a **disposition per point**
     (`fix-needed` / `works-as-designed` (explain) / `new-scope`) — and updates `plan.md`;
     **`adjustments.md` logs the delta**. (Create `uat.md` from
     `${CLAUDE_PLUGIN_ROOT}/templates/uat.template.md` if absent.) `state.md` **stays
     `waiting-for-user`** throughout.
  3. **The user RE-ACCEPTS** the adjusted plan — you own this gate in chat exactly like the
     ① acceptance gate, and **only this re-acceptance** flips the feature off
     `waiting-for-user`. The re-acceptance lands through the **normal plan-acceptance flow**:
     `gogo-plan` records it — sets `state.md` `status: plan-accepted`, clears
     `open-decision`, and emits its own terminal **`plan-accepted`** event (**you emit no
     `plan-accepted`** — that event has a single owner, `gogo-plan`). You then bump
     `iterations` `uat=N` (the loop-back count) and emit **`uat-failed`**
     (`{"event":"uat-failed","phase":"report","status":"plan-accepted","note":"UAT round N: <summary>","slug":"<slug>"}`
     — "failed" = the gate sent the work back; its `note` is the round summary).
     **Also close out round N's `uat.md` verdict:** in the same step, append the
     template's post-acceptance line to that round's **`Verdict:`** — `re-accepted
     (user, <YYYY-MM-DD>) → /gogo:go reruns ②→⑤` — so the round's own log records the
     outcome (the analyst left it at `re-planned — awaiting re-acceptance` and stopped;
     recording the re-acceptance is yours, not the analyst's).
  4. **`/gogo:go` reruns ②→⑤** — the SAME work item, never a new one — landing back at
     `awaiting-uat` for the next check.

  Ownership: **`uat-opened` and `uat-failed` are yours (the orchestrator)**; `plan-accepted`
  stays `gogo-plan`'s (recorded by the normal re-acceptance flow, step 3) and `uat-passed`
  is `gogo-done`'s. A `works-as-designed` point is still written into `uat.md` with its
  explanation (a "not a bug" answer is recorded, never silently dropped); a `new-scope`
  point is noted as out of this item.

### Ship → command `/gogo:done` (skill `gogo-done`)
The explicit post-report gate. A **slug** ships that one feature; **`slug1+slug2+...`**
ships those as ONE merged release entry; with **no slug** `/gogo:done` opens the **work
board cockpit** — the shared `gogo-status` classifier labels every `.gogo/work/feature-*`
(shipped · ready-to-ship · in-progress · unfinished) and from the four-class table the
user **views** any card (`v`), **ships** ready cards separately (`s`) or **merged**
(`m`), **runs/resumes** the pipeline on an unbuilt card (`g`), and **filters** (`/`) —
an interactive terminal-TUI kanban, `assets/kanban/board.py`, when `python3` + `tmux` +
a tty are present; otherwise a status table + `AskUserQuestion` multi-select ship
fallback — never failing over the board (D5=A). Each key writes a single-shot **intent**
the orchestrator executes before **relaunching** the board (`go` hands off to the
pipeline; `cancel` stops); the board never mutates gogo state. When shipping merged (or a
≥2 fallback selection), one `AskUserQuestion` gates separate (N entries) vs merged (1
entry). Every entry is a **high-level synthesis, not a copy** of the report
bundle: gogo **writes** a `report/report.md`-style summary (*what was
changed/done/implemented*, key outcomes, one-line decisions, review/test verdict, a
member table + per-member section when merged, links back to each `.gogo/work/` folder),
plus the **slug-prefixed** `.mmd` set, a `manifest.json` carrying a `members[]` array,
and the merged `before/` set — into `.gogo/changelog/<YYYY-MM-DD>-<name>/` (date = newest
member's `completed:`; **no `diagrams.html` copy** — the viewer builds from source). It
**builds the interactive viewer page and prints its `file://` link** (best-effort,
reusing the `gogo-view` build; falls back to the changelog folder path), and sets **each
member's** `state.md` to a terminal `shipped` status. The audit trail stays in
`.gogo/work/`; idempotent. A named slug with no report STOPs with "run `/gogo:report
<feature>` first".

### View → command `/gogo:view` (skill `gogo-view`)
Read any **plan or report** as a self-contained, offline interactive webpage (the
`plan.md` / `report.md` summary as HTML + its mermaid diagrams made **interactive**:
flowchart-family kinds get an xplan-style rich renderer — draggable token-styled node
cards with a live-re-routing edge layer, minimap, zoom/fit, and a persisted layout —
other kinds fall back to a pan/zoom/drag canvas; a bundle carrying a `before/` set
renders **before / after side by side**). Surfaces both **plans and reports** via a
grouped **Work** (each feature's plan + report) / **Changelog** (shipped reports)
picker — plans rendered in place from `plan.md` + `charts/` (D1=A) — builds the page
from the vendored `.gogo/resources/` assets, and opens it.

## Loops & bounds

- **implement ↔ review**: loop until review is clean; bound ~3 rounds on the same
  finding → escalate it as a decision. You hold the implement context **warm**
  across these rounds (no re-spawn, no re-read); the ~3-round bound doubles as a
  guard on orchestrator context growth (rely on auto-compaction beyond it).
- **test → implement → review → test**: a test issue re-enters implementation
  (still in-context), then re-review (fresh subagent), then re-test (fresh subagent).
- Track rounds in `state.md` `iterations:`.

## Pipeline telemetry — events.jsonl

At **every** phase/status transition, append one compact JSON line to
`.gogo/work/feature-<slug>/events.jsonl` **beside** (never instead of) the
`state.md` write, per `events.schema.json`
(`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`). This append-only stream is what a
deterministic consumer (the `gogo` CLI cockpit) reads for live progress and
per-item history; `state.md` stays the single human resume file. Create the file
if absent; **best-effort** — a failed append never fails a phase, and a missing
`events.jsonl` is never an error.

**Ownership — one emitter per transition.** Phase lifecycle events are emitted by
the **phase skills** — the orchestrator emits **only the gate events**. Each phase
skill owns *all* of its phase's events (they must, because `/gogo:implement`,
`/gogo:review`, … also run standalone with no orchestrator), so **never emit
`phase-started` / `phase-done` from here**: `gogo-plan` owns `phase-started`/plan +
`plan-accepted`/plan (its terminal event), `gogo-implement` owns
`phase-started`/`fix-round` + `phase-done`/implement, `gogo-review` owns
`round-opened`/`issues-found` + `phase-done`/review, `gogo-test` the same for test,
`gogo-knowledge` owns `phase-started`/`phase-done`/report, and `gogo-done` owns
`uat-passed` + `shipped`/done. As the orchestrator you emit **only** `gate-opened` /
`gate-resolved` around a decision gate (below) **and the two UAT-loop events
`uat-opened` / `uat-failed`** (the UAT gate between ⑤ and ship — see *UAT* above;
`uat-passed` is `gogo-done`'s, not yours). Each transition is emitted **exactly once, by
its owning skill** — the timeline never double-counts.

## Decision gates — stopping for the user

Stop **only** for genuine forks: ambiguous requirements, scope changes,
destructive/irreversible actions, trade-offs with no obvious right answer, **or a
hands-on/e2e verification that can't run** (missing emulator/device/browser/
dev-server/app, or a failed connection). For everything else, decide, note it, and
keep moving.

For the e2e-blocked case specifically: **never silently skip it.** Ask the user how
to proceed — help set up the environment and retry (e.g. they boot the emulator +
start the app; you re-run ④ to reconnect), try an alternative verification, or
explicitly skip. Loop on retries; the check is **only** skipped when the user says
so. On resolve, resume at **④** (re-test), not ①.

When you do stop:
1. Append the question + options + **your recommendation** to `decisions.md`
   (use the template's `D<n>` shape).
2. Set `state.md` → `status: waiting-for-user`, `resume: <phase>`,
   `open-decision: D<n>`. **Append the transition event** (best-effort, per
   `events.schema.json`):
   `{"ts":"<RFC3339>","event":"gate-opened","phase":"<resume phase>","status":"waiting-for-user","note":"D<n>","slug":"<slug>"}`.
   Gate events use the **events** `phase` vocabulary — if the resume phase in
   `state.md` is `knowledge` (the fifth phase's skill name), map it to **`report`**
   in the event's `phase` field (the events enum has `report`, not `knowledge`).
3. End your turn and ask (use `AskUserQuestion` for clear forks; prose for
   open-ended). The Notification hook pings the user.
4. On the answer: append a `RESOLVED` block to `decisions.md`, clear
   `open-decision`, and resume at `state.md`'s `resume` phase. **Append the
   transition event** (best-effort, same `knowledge`→`report` phase mapping as in
   step 2): `{"ts":"<RFC3339>","event":"gate-resolved","phase":"<resume phase>","status":"<resumed status>","note":"D<n>","slug":"<slug>"}`.

## Resume

To resume (fresh session, or after a decision): read `state.md` + `decisions.md`,
then continue at `resume:`. `/gogo:status` lists every feature's state;
`/gogo:resume` folds in an answer and continues.
