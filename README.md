<p align="center">
  <img src="docs/assets/logo.png" alt="gogo — make your flow more agentic" width="360" />
</p>

<h1 align="center">gogo</h1>

<p align="center"><em>make your flow more agentic</em></p>

**A portable, knowledge-grounded development pipeline for Claude Code.**

📖 **Documentation: <https://zawadzkib.github.io/gogo/>** — commands, the flow,
the per-agent I/O reference, discovery, and the contracts, with rendered diagrams.
The site is **generated from** this repo's `commands/`, `agents/`, `skills/`, and
`templates/`; the **code and skills are authoritative** and the site may lag them.

> **The flow is generic and ships with the plugin. The rules are yours.**
> gogo runs every non-trivial change through five fixed phases — **plan →
> implement → review → test → report** — but *what* it plans against, *how* it
> writes code, *what* review flags, and *how* it tests are all driven by plain
> markdown **knowledge files** that gogo wires up from your existing project docs.
> Same pipeline everywhere; the behaviour is configuration.

## The flow

![gogo pipeline — goal → plan → implement → review → test → report, with review/test fix-loops back into implement, grounded in your .gogo/knowledge config](assets/flow.png)

<details>
<summary>Same flow as an editable Mermaid diagram</summary>

```mermaid
flowchart LR
    G([goal]) --> P["① PLAN"]
    P ==>|accepted| IMP["② IMPLEMENT"]
    IMP ==> REV["③ REVIEW"]
    REV ==>|clean| TEST["④ TEST · e2e"]
    TEST ==>|all green| REP["⑤ REPORT"]
    REP ==> DONE([done])
    REV -->|"issues → fix"| IMP
    TEST -->|"issues → fix"| IMP
    P -. "changes / clarify" .-> P

    classDef phase fill:#e8ecff,stroke:#7c8bd9,stroke-width:1.5px,color:#111
    classDef io fill:#fff3d6,stroke:#caa54a,color:#111
    class P,IMP,REV,TEST,REP phase
    class G,DONE io
```
</details>

*Plan waits for your acceptance before any code is written. Review and test loop
fixes back into implement, and either can **pause for your decision** at any point
— you answer and it resumes. On success, Report writes an as-built `report/` bundle
(`report/report.md` + diagrams) and updates your knowledge docs.
**Every phase is grounded in your `.gogo/knowledge/` config.***

## Generic flow, your rules

The five phases never change. What changes per project lives in **`.gogo/knowledge/`**
— small markdown files, one concern each, that gogo reads at the relevant phase.
**These files are the configuration**: they're what make the generic flow behave
like *your* project.

| File | What it holds | Read in |
|---|---|---|
| `project-knowledge.md` | architecture, domains, glossary, key decisions | Plan |
| `tech-stack.md` | languages, frameworks, and the build / run / test commands | Plan · Implement · Test |
| `non-functional-requirements.md` | standing quality bars: performance, security, accessibility, reliability, limits | Plan · Review · Test |
| `coding-rules.md` | conventions the implementation must follow | Implement · Review |
| `code-review-standards.md` | what review checks for: correctness, security, performance, error handling, style | Review |
| `testing-tools.md` | the test tools and exactly how to run them | Test |
| `test-strategy.md` | how to test: user journeys, UI / design checks, e2e levels, deploy checks, the done-bar | Test |
| `index.md` | a purpose-map of the folder + the proxy convention | — |
| `_discovered.md` | what `/gogo:build` found and how each file was wired (regenerated each run) | build |

On success, the **Report** phase writes anything it learned back into these files
(your gogo-owned summaries), keeping them current.

These files are **proxies**: they link to your project's real docs (an existing
`CLAUDE.md`, `README`, `CONTRIBUTING`, Copilot / Cursor / Windsurf / Codex configs,
manifests, test configs) and add a short gogo-specific summary — they don't
duplicate them. Where a project has no doc for a topic, gogo authors that file
from your codebase. You create them once with `/gogo:build` and refresh anytime;
re-runs pick up new docs and **preserve your edits**.

So adopting gogo in a new project is just `/gogo:build` — no flow to rewrite.

## How it works

Want the full picture — the flow-vs-knowledge split, *why* knowledge is split
again into always-read config vs on-demand skills, and exactly what gets stored
where (plugin side vs your project's `.gogo/`)? See
[**docs/architecture.md**](docs/architecture.md).

In short: the **flow ships with the plugin** (`commands/`, `skills/`, `agents/`),
the **rules live in your project** (`.gogo/knowledge/`), and situational detail
that would bloat the always-read config is extracted into **on-demand skills**
(`.gogo/skills/`, or `.claude/skills/` for reusable ones) that load only when a
task needs them — keeping each phase's context small and the LLM workers
deterministic.

## Quickstart

```
/plugin marketplace add ZawadzkiB/gogo
/plugin install gogo@gogo

/gogo:build                 # wire gogo to this project's docs (run once; re-run anytime)
/gogo:plan "add CSV export to the reports page"
# review the plan, accept it, then:
/gogo:go
```

> Hacking on gogo itself? Add your local clone as the marketplace instead of the
> GitHub one (they share the name `gogo`, so use one or the other):
> `/plugin marketplace add /path/to/gogo`.

## Updating

`/plugin install` reads a **local copy** of the marketplace, so installing on its
own never pulls a newer version. Refresh the marketplace first, then reinstall:

```
/plugin marketplace update gogo   # fetch the latest gogo from GitHub
/plugin install gogo@gogo         # install the bumped version
/reload-plugins                   # apply it to the running session
```

To confirm which version is active, run `/plugin` and check gogo's version, or
inspect the install cache:

```
ls ~/.claude/plugins/cache/gogo/gogo/   # newest dir = active version
```

> Using a local clone as the marketplace? A plain `git pull` in the clone is
> enough — no `marketplace update` needed — followed by `/reload-plugins`.

## Commands

Each command is an ultra-thin entry point to the orchestrator — no flow logic
lives in the commands themselves.

**`/gogo:build [--force]`**

Set up or refresh the project's knowledge config. Discovers your existing docs
(`CLAUDE.md`, Copilot / Cursor / Windsurf / Codex configs, README, manifests,
test/CI configs) **at any depth** — including nested monorepo packages like
`frontend/.github/` — plus a sweep of all project markdown and a light pass over
in-code doc comments, then wires each knowledge file as a proxy, or synthesizes it
from the codebase when none exists. It then **verifies the high-signal facts
against your actual code** (tech stack, build/run/test commands, test framework,
entry points) — on a conflict **code wins**, so a stale doc can't quietly poison
the config (gogo corrects its own summary, never your upstream file). Idempotent:
re-run anytime to pick up new docs while preserving your edits. `--force` resets to
fresh scaffolds.

**`/gogo:skills ["<prompt>"] [--warn N] [--max N] [--include <path>]`**

Keep your knowledge config lean so the pipeline stays deterministic. Audits every
`.gogo/knowledge/*.md` against a line budget (OK `<200` · WARN `200-400` · OVER
`>400`), auto-discovers cohesive sections worth pulling out, classifies each as a
`knowledge` skill (→ `.gogo/skills/`) or a `standalone` skill (→ `.claude/skills/`),
and **proposes them, then stops for your per-candidate approval** before writing
anything. On approval it extracts each into a `SKILL.md` (+ optional `scripts/` /
`.env.example`) and leaves a `**Load when:**` pointer in the parent. Directed mode
— `/gogo:skills "extract the deploy runbook"` — pulls out exactly what you name.
Idempotent; re-run anytime. Knowledge-maintenance sibling of `/gogo:build`.

**`/gogo:plan "<goal>"`**

Runs the plan phase only. Writes an accept-pending plan to
`.gogo/work/feature-<slug>/` (with the feature's functional requirements, a changes
checklist, and a mermaid chart) and **stops for your acceptance** — no code is
written until you accept.

**`/gogo:go [feature-slug]`**

Implements the accepted plan through the implement → review → test → report loop,
delegating to the specialist agents and pausing only at real decisions. Refuses to
start until a plan is accepted.

The implement → review → test → report phases are **also runnable on their own**
— each is a thin, idempotent entry point to its phase skill that **validates its
inputs** before working and **validates its outputs** before hand-off (the
contract layer, below). `/gogo:go` chains these same commands.

**`/gogo:implement [feature-slug] [--issues <path>]`**

Phase ② standalone. Plain, it builds the accepted plan from scratch and emits the
as-built diagram set. With `--issues <path>` (a `review/issues.json` or
`test/issues.json`) it fixes the **open** issues and writes back what was fixed
(`status: fixed`, `fix_summary`, `fixed_in_round`).

**`/gogo:review [feature-slug]`**

Phase ③ standalone. Fresh-eyes review against your standards; emits the living,
typed `review/issues.json` (the contract) and renders a `review-NN.md` snapshot.
Re-run it after fixes — it updates the same list in place (open → fixed/verified,
adds new).

**`/gogo:test [feature-slug]`**

Phase ④ standalone. e2e/UI/CLI/API testing per your strategy; emits the living
`test/issues.json` + a `test-NN.md` snapshot, looping issues back to implement.

**`/gogo:report [feature-slug]`**

Phase ⑤ standalone. For an all-green feature: finalizes the plan to as-built,
writes the `report/` bundle (`report/report.md` + the as-built UML diagrams), and
updates your gogo-owned knowledge docs. Run standalone, it **also (re)generates a
report for a past or broken run** — synthesizing a best-effort `report/report.md`
from whatever artifacts exist (plan, decisions, review/test issues, state, charts)
and clearly marking which phases ran and what's still open. `plan.md` is the one
prerequisite. (The in-pipeline ⑤, right after a green test, keeps its strict gate.)

**`/gogo:done [feature-slug | slug1+slug2+...]`**

Ship report-complete features into a high-level changelog. A **slug** ships that one;
**`slug1+slug2+...`** ships those as ONE merged release entry; **no slug opens the
ready-to-ship list** over every `.gogo/work/feature-*` — the shared `/gogo:status`
classifier labels each **shipped · ready-to-ship · in-progress · unfinished**, prints the
four-class table for context, then offers the ready-to-ship items as a **filterable
`AskUserQuestion` multi-select**. **Selecting multiple items merges them into ONE entry**
(release name suggested + confirmed); one pick is one entry — multi-select *is* the merge
signal, so there is no extra merge-or-split question. A non-slug arg (or, with more ready
items than fit one question, an answer) is a case-insensitive substring filter over
slug+title. It is a plain terminal list — always available, no soft dep (the **browser
board** is `/gogo:xplan`, below). Every entry is a **high-level synthesis, not a copy** of
the report bundle —
gogo **writes** a `report.md` summarizing *what was changed/done/implemented* (key
outcomes, one-line decisions, a review/test verdict, a member table + per-member section
when merged) with a **link back** to each member's `.gogo/work/` folder for the full
audit trail — plus the slug-prefixed `.mmd` set, a `manifest.json` carrying a
`members[]` array, and the merged `before/` set, into
`.gogo/changelog/<YYYY-MM-DD>-<name>/` (date = newest member's `completed:`; **no
`diagrams.html` copy** — the viewer builds from source). It **builds the interactive
viewer page for the entry and prints its `file://` link** (best-effort, reusing the
`/gogo:view` build; falls back to the changelog folder path — never failing over the
link), and sets **each member's** `state.md` to a terminal `shipped` status. The audit
trail stays in `.gogo/work/`; idempotent. A named slug with no report stops and tells
you to run `/gogo:report <feature>` first.

**`/gogo:view [changelog-entry | feature-slug[:plan|:report]]`**

Open a gogo **plan or report** as a self-contained, offline **interactive webpage** —
the `plan.md` / `report.md` summary rendered as readable HTML plus its mermaid diagrams
made **interactive** (vendored runtime, no network, no build). Flowchart-family diagrams
get an xplan-style rich renderer: custom-styled node cards you **drag** with edges
that **re-route live**, plus **zoom / fit / minimap** and a **persisted layout**;
other kinds fall back to a pan/zoom/drag canvas. A bundle carrying a `before/` set
renders **before / after side by side** (compare mode). With no arg it presents a
grouped **Work** (each feature's plan + report) / **Changelog** (shipped reports)
picker — plans render in place from `plan.md` + `charts/` — and opens your pick; falls
back to printing the `file://` path if it can't auto-open.

**`/gogo:xplan`**

Open the gogo work as a **browser kanban** — a React board (ported from xplan) served by
a `python3` **stdlib** server on **localhost**.
Four fixed columns **plan · in progress · ready · changelog**, fed by the shared
`/gogo:status` classifier plus the changelog entries: drag-and-drop, a live text filter,
a **view** button per card that opens its pre-built HTML page, and **mark-done from the
board** — check ready cards (or drag a ready card onto the changelog column), where
**multiple picks = ONE merged entry** (same signal as `/gogo:done`). It pre-builds every
view page at launch, then keeps a **long-running** server and **watches for a ship
intent**: each one runs the same synthesis writer, rebuilds the board index, and the
polling board moves the card to changelog **live** (multiple ships per session). `python3`
is a **soft dep** — without it, `/gogo:xplan` points you at `/gogo:done`'s list and never
fails; the built board is **committed** (`assets/xplan-board/dist/`) so no npm is needed
at runtime (npm is dev-time only). Localhost only, offline, writes only under `.gogo/`.

**`/gogo:status`**

Lists every feature under `.gogo/work/` with its phase, status, and iteration counts.
Read-only. It also hosts the shared **work-index classifier** (shipped · ready-to-ship
· in-progress · unfinished) that `/gogo:done`'s ready-to-ship list and `/gogo:xplan`'s
browser board reuse to decide what is shippable.

**`/gogo:resume [feature-slug]`**

Resumes a feature that paused for your decision, folding your answer into
`decisions.md` and continuing the loop.

## Agents

- **`gogo`** — the orchestrator: owns the flow/loop, knows what to run when, and
  delegates to the specialists. Also usable hands-off ("build X end-to-end").
- **`gogo-developer`** — implements the accepted plan and applies review/test fixes.
- **`gogo-reviewer`** — fresh-eyes, adversarial code review.
- **`gogo-tester`** — e2e/UI testing via the bundled Playwright MCP.

## What gets created in your project

gogo keeps everything under one **`.gogo/`** folder — plain markdown you can read,
edit, and commit:

**`.gogo/knowledge/`** — your project's configuration: the nine files described in
[**Generic flow, your rules**](#generic-flow-your-rules) above. Every file states
its own purpose in its header, and `index.md` is the folder's purpose-map.

**`.gogo/skills/`** — on-demand skills `/gogo:skills` has extracted from your
knowledge files: cohesive, situational detail moved out of the always-read config
into skills that load **only when relevant**, keeping the pipeline lean and
deterministic. `.gogo/skills/index.md` is the registry. A candidate the command
classifies as **standalone** (a reusable, self-contained capability) instead lands
in **`.claude/skills/<slug>/`** so Claude Code auto-discovers it — written only
when you approve that candidate (the one sanctioned write outside `.gogo/`).

**`.gogo/resources/`** — one vendored mermaid runtime per project
(`mermaid.min.js`, shared by every feature) plus the interactive viewer module set
(`viewer/`) that `/gogo:view`, `/gogo:done`, and `/gogo:xplan` build pages from (into
`view/`), and `xplan-board/` (the `/gogo:xplan` browser-board runtime — `board.json`,
the `ship-intent.json` the board POSTs, and `server.pid`). Offline, no network, no build.

**`.gogo/work/feature-<slug>/`** — one folder per piece of work:

| File | Purpose |
|---|---|
| `plan.md` | The accepted plan (the contract), incl. the feature's functional requirements |
| `adjustments.md` | Log of changes/clarifications you asked for during planning |
| `state.md` | Current phase/status/iterations — lets work resume across sessions |
| `decisions.md` | Forks that needed your call, with gogo's recommendation + your answer |
| `review/issues.json` | The living, typed review findings — the **contract** review hands to implement (one list, updated in place across rounds) |
| `review-NN.md` | Each code-review round's rendered snapshot of `issues.json` |
| `test/issues.json` | The living, typed test findings (same contract) |
| `test-NN.md` | Each test round's rendered snapshot |
| `report/` | The as-built bundle (written at report phase): `report/report.md` (planned-vs-shipped, implementation, decisions + reasons, review/test outcomes), the UML set (`.mmd` chosen by the diff), `report/before/` (the plan-time "before" set copied in for a self-contained before/after compare), `diagrams.html`, `manifest.json`. This is the full audit trail; `/gogo:done` **synthesizes** a high-level entry from it into `.gogo/changelog/<date>-<name>/` (it does not copy the bundle) |
| `charts/` | Mermaid diagrams (`.mmd`) + `charts/before/` (the plan-time as-is baseline) + `manifest.json` + an offline `diagrams.html` viewer — the plan's intended design, plus the implement as-built flow / sequence / class / activity set |

**`.gogo/changelog/`** — the append-only shipped archive, a high-level release
history. When you run `/gogo:done`, gogo **synthesizes** an entry into
`.gogo/changelog/<YYYY-MM-DD>-<name>/` — a written `report.md` (not a copy of the
report bundle) + the slug-prefixed `.mmd` set + a `manifest.json` with a `members[]`
array + the `before/` set. One or several related features can ship as a single merged
release entry; the full detail stays in `.gogo/work/`. `/gogo:view` reads from here too.

The typed artifacts (`*/issues.json`, `charts/manifest.json`, per-run
`result.json`, the feature `pipeline.json`) follow JSON Schemas shipped in the
plugin (`templates/contracts/`). Each phase command validates its inputs and
outputs against them so a bad LLM hand-off is caught, not propagated — the
validation is portable (`jq`/schema if present, else the agent checks against the
schema; no new required dependency).

## Portability & prerequisites

gogo is built to run anywhere it's installed:

- The core **plan → implement → review → test** loop needs **no external
  dependencies**.
- **Mermaid** diagrams render natively in GitHub / VS Code / JetBrains from
  fenced ` ```mermaid ` blocks; the bundled offline viewer needs only a browser
  (mermaid is vendored — no network, no CLI).
- **Browser / UI testing** uses the bundled **Playwright MCP**, which boots via
  `npx` on first use (needs **Node.js**). Without it, the test phase falls back to
  API/CLI tests plus written manual steps.

Optional: set `GOGO_NTFY_TOPIC` in your shell to get a phone push (via
[ntfy.sh](https://ntfy.sh)) when gogo pauses for your input. Without it you still
get a local desktop notification + a terminal bell.

## License

MIT — see [LICENSE](./LICENSE).
