---
name: gogo
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

## Feature workspace

Everything for one piece of work lives in **`.gogo/work/feature-<slug>/`** (kebab
slug from the feature name). These files are the pipeline's memory + audit trail:

- `plan.md` — the accepted plan (the contract), incl. the feature's *functional* requirements
- `adjustments.md` — running log of user-requested changes/clarifications during planning
- `state.md` — current phase / status / iteration counters / resume info
- `decisions.md` — open/closed forks that needed the user
- `review/issues.json` — the living, typed review findings (the contract); `review-NN.md` renders each round's snapshot
- `test/issues.json` — the living, typed test findings (same contract); `test-NN.md` renders each round's snapshot
- `report/` — the as-built bundle (written at ⑤): `report/report.md` (planned-vs-shipped, implementation, decisions+reasons, review/test outcomes), the UML set (`.mmd` chosen by the diff), `report/before/` (the plan-time "before" set copied in for a self-contained before/after compare), `diagrams.html`, `manifest.json`, `result.json`. `/gogo:done` copies this bundle to `.gogo/changelog/`.
- `charts/` — mermaid `.mmd` + `manifest.json` + offline `diagrams.html` (plan's intended design + `charts/before/` the plan-time as-is baseline; ② emits the as-built flow/sequence/class/activity set for review/test)

The shipped bundle is also archived (chronologically) under `.gogo/changelog/<YYYY-MM-DD>-<slug>/` once the user runs `/gogo:done`.

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
user goal ─▶ ① PLAN ──(user accepts)──▶ ② IMPLEMENT ─▶ ③ REVIEW ─▶ ④ TEST ─▶ ⑤ REPORT ─▶ /gogo:done ─▶ shipped
              ▲  │                            ▲            │           │       (update plan +   (copy bundle →
              │  └──(clarify / changes)──▶ wait            │           │        knowledge docs)  .gogo/changelog/)
              │                                └──issue─────┘           │
              │                                  (fix → re-review, ≤3)  │
              └──── issue needs a USER DECISION (from review or test) ──┘
```

## Who runs each phase

- **You (the orchestrator)** run the *interactive* phases in chat: ① plan +
  acceptance gate, every decision gate, and ⑤ report.
- You **delegate the heads-down phases** via the `Task` tool, each to a
  fresh-context specialist:
  - ② implement → **`gogo-developer`** agent (follows the `gogo-implement` skill)
  - ③ review → **`gogo-reviewer`** agent (follows the `gogo-review` skill)
  - ④ test → **`gogo-tester`** agent (follows the `gogo-test` skill)
- A delegated worker that hits a real fork **returns** it to you; you handle the
  gate (below) and re-delegate with the answer.

If browser/agent tooling is unavailable, you may run a phase's skill yourself
in-context instead of delegating — the phase skills are written to run either way.

## The phases

### ① Plan → skill `gogo-plan`
Analyse the goal against the knowledge docs; create `.gogo/work/feature-<slug>/`;
write `plan.md` (Goal / Context / Functional requirements / Approach +
alternatives / Changes checklist / Tests / Out-of-scope); draw the change/flow
with `gogo-mermaid`; init `state.md`. **Present the plan and STOP for
acceptance.** Changes/clarification → log to `adjustments.md`, revise,
re-present. **Do not implement until the user accepts.** Hard gate.

### ② Implement → skill `gogo-implement` (delegate to `gogo-developer`)
Build the accepted `plan.md` following `coding-rules.md`; keep changes scoped;
keep build/typecheck/unit green. Re-enter here to apply review/test fixes.

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
- **Issue needing a user decision** → back to ① (re-plan how to handle it,
  re-accept), via a decision gate.
- **All green** → ⑤.

### ⑤ Report → skill `gogo-knowledge`
Update `plan.md` to as-built; draw the as-built UML set (chosen by what changed —
class / sequence / activity / use-case / flow) via `gogo-mermaid` into the
feature's `report/` folder; write the final `report/report.md` (planned-vs-shipped,
**implementation**, **decisions + reasons**, review/test outcomes, diagram + audit
links); update whatever `.gogo/knowledge/*` drifted (gogo-owned summaries only —
never the proxied originals); set `state.md` to done; summarise to the user (point
them at `report/report.md` and `report/diagrams.html`).

The in-pipeline ⑤ keeps a strict gate (green ④ required). Run **standalone via
`/gogo:report <feature>`, it is lenient** — it also reports on a past/broken/
incomplete run, synthesizing a best-effort `report/report.md` from whatever exists
and marking which phases ran and what's still open (`plan.md` is the one
prerequisite).

### Ship → command `/gogo:done` (skill `gogo-done`)
The explicit post-report gate. A **slug** ships that one feature; with **no slug**
`/gogo:done` opens a **work board** — the shared `gogo-status` classifier labels every
`.gogo/work/feature-*` (shipped · ready-to-ship · in-progress · unfinished) and the
user picks which ready-to-ship features to ship (an interactive terminal-TUI kanban,
`assets/kanban/board.py`, when `python3` + `tmux` + a tty are present; otherwise a
status table + `AskUserQuestion` multi-select — never failing over the board; D5=A).
Shipping is a single flow looped over the picks: copy each `report/` bundle (report.md
+ diagrams + the `before/` set) into the append-only
`.gogo/changelog/<YYYY-MM-DD>-<slug>/`, **build the interactive viewer page for the
entry and print its `file://` link** (best-effort, reusing the `gogo-view` build;
falls back to the static `diagrams.html` path), and set `state.md` to a terminal
`shipped` status. Copy-not-move (the work folder stays the source); idempotent. A
named slug with no report STOPs with "run `/gogo:report <feature>` first".

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
  finding → escalate it as a decision.
- **test → implement → review → test**: a test issue re-enters implementation,
  then re-review, then re-test.
- Track rounds in `state.md` `iterations:`.

## Decision gates — stopping for the user

Stop **only** for genuine forks: ambiguous requirements, scope changes,
destructive/irreversible actions, or trade-offs with no obvious right answer. For
everything else, decide, note it, and keep moving.

When you do stop:
1. Append the question + options + **your recommendation** to `decisions.md`
   (use the template's `D<n>` shape).
2. Set `state.md` → `status: waiting-for-user`, `resume: <phase>`,
   `open-decision: D<n>`.
3. End your turn and ask (use `AskUserQuestion` for clear forks; prose for
   open-ended). The Notification hook pings the user.
4. On the answer: append a `RESOLVED` block to `decisions.md`, clear
   `open-decision`, and resume at `state.md`'s `resume` phase.

## Resume

To resume (fresh session, or after a decision): read `state.md` + `decisions.md`,
then continue at `resume:`. `/gogo:status` lists every feature's state;
`/gogo:resume` folds in an answer and continues.
