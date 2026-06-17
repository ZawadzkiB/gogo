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

## Feature workspace

Everything for one piece of work lives in **`.plans/feature-<slug>/`** (kebab
slug from the feature name). These files are the pipeline's memory + audit trail:

- `plan.md` — the accepted plan (the contract), incl. the feature's *functional* requirements
- `adjustments.md` — running log of user-requested changes/clarifications during planning
- `state.md` — current phase / status / iteration counters / resume info
- `decisions.md` — open/closed forks that needed the user
- `review-NN.md` — each review round's findings
- `test-NN.md` — each test round's results
- `charts/` — mermaid `.mmd` + offline `diagrams.html`

Create the folder in the plan phase (copy `state.md`/`decisions.md` from
`${CLAUDE_PLUGIN_ROOT}/templates/`). **Keep `state.md` current at every phase
transition** so a fresh session — or a resume after a user decision — picks up
exactly where it left off.

## The flow

```
user goal ─▶ ① PLAN ──(user accepts)──▶ ② IMPLEMENT ─▶ ③ REVIEW ─▶ ④ TEST ─▶ ⑤ REPORT ─▶ done
              ▲  │                            ▲            │           │       (update plan +
              │  └──(clarify / changes)──▶ wait            │           │        knowledge docs)
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
Analyse the goal against the knowledge docs; create `.plans/feature-<slug>/`;
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
Findings → `review-NN.md`.
- **Fixable** → back to ② (fix), then re-review. Bound: if the same issue resists
  ~3 rounds, treat it as a decision and stop.
- **Needs a user decision** → decision gate (below).
- **Clean** → ④.

### ④ Test → skill `gogo-test` (delegate to `gogo-tester`)
e2e at every relevant level per `test-strategy.md`/`testing-tools.md` — UI
(bundled Playwright MCP), CLI, API — plus exploration (does it work? does it look
right?). Results → `test-NN.md`.
- **Issue (fixable)** → back to ② → ③ → ④.
- **Issue needing a user decision** → back to ① (re-plan how to handle it,
  re-accept), via a decision gate.
- **All green** → ⑤.

### ⑤ Report → skill `gogo-knowledge`
Update `plan.md` to as-built; update whatever `.gogo/knowledge/*` drifted
(gogo-owned summaries only — never the proxied originals); re-render charts; set
`state.md` to done; summarise to the user.

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
