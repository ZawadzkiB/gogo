---
title: Architecture
nav_order: 7
---

# How gogo works

Reference for the gogo architecture: the two splits that make it portable and
deterministic, and a complete map of what is stored where. For the quick pitch and
the command list, see the [Home](index.md) page (or the
[README](https://github.com/ZawadzkiB/gogo) on GitHub).

## 1. The flow-vs-knowledge split

gogo runs every non-trivial change through five fixed phases:

```
goal → ① PLAN → ② IMPLEMENT → ③ REVIEW → ④ TEST → ⑤ REPORT → UAT gate → shipped
```

**The flow ships with the plugin. The rules are yours.** The phases never change
and carry no project-specific logic — that lives in plain-markdown **knowledge
files** under your project's `.gogo/knowledge/`, which gogo wires up from your
existing docs with `/gogo:build`. *Same pipeline everywhere; the behaviour is
configuration.*

Why split it this way:

- **Portable flow** — the generic plan→implement→review→test→report loop needs
  zero external dependencies and works in any language/ecosystem. You never fork
  or rewrite it.
- **Project-specific rules, without forking the flow** — what to plan against, how
  to write code, what review flags, how to test all come from your knowledge
  files. A new project adopts gogo with one command (`/gogo:build`); the flow is
  untouched.

Each phase reads only the knowledge it needs:

| Knowledge file | What it holds | Read in phase | Typical mode |
|---|---|---|---|
| `analysis.md` | how to analyze a feature before planning (procedure + which files to read; code = truth) | plan | owned |
| `project-knowledge.md` | architecture, domains, glossary, key decisions | plan | proxy |
| `tech-stack.md` | languages, frameworks, build/run/test commands | plan · implement · test | proxy |
| `non-functional-requirements.md` | standing perf/security/a11y/reliability bars | plan · review · test | owned |
| `coding-rules.md` | conventions the implementation must follow | implement · review | proxy |
| `code-review-standards.md` | what the review phase checks for | review | owned |
| `testing-tools.md` | the test tools and how to run them | test | proxy |
| `test-strategy.md` | how to test: journeys, UI, e2e levels, the done-bar | test | owned |
| `index.md` | the folder's purpose-map + the proxy convention | — | — |
| `_discovered.md` | what `/gogo:build` found and how it wired each file | build | owned |

Knowledge files are mostly **proxies**: a short gogo-owned summary + a `Source:`
link to your real doc (an existing `CLAUDE.md`, `README`, Copilot/Cursor/Windsurf/
Codex config, manifest, test config). Where no doc exists, gogo authors the file
(**owned**). Re-running `/gogo:build` reconciles — picks up new docs, refreshes
summaries, and **preserves** every `## gogo overrides` section and owned body.

**Two kinds of never-rewritten section.** A knowledge file can carry a gogo-authored
`## gogo overrides` block (gogo's own notes, kept across re-runs) **and** a user-owned
**`## Custom`** section — anything *you* write there. `/gogo:build` re-runs and phase-⑤
reconciles **copy every `## Custom` section 1:1** (byte-for-byte) and never rewrite it
(build reports what it preserved). The distinction: **overrides = gogo-authored;
Custom = yours, untouchable.**

## 2. The knowledge-vs-on-demand-skills split

Knowledge files are **always-read context** for their phase. As a real project
grows, those files grow — and large always-loaded context is the problem: it gives
the LLM workers too much at once, so they wander, lose focus, and make more
mistakes. **Less context, more determinism.**

The fix (introduced by the `/gogo:skills` command) is a second split *inside*
knowledge: keep the always-read file lean, and move cohesive, situational detail
into **on-demand skills** that load only when a task actually needs them.

**The line budget.** Each knowledge file's gogo-owned body is held to:

| Band | Lines | Meaning |
|---|---|---|
| OK | `<200` | lean — leave it |
| WARN | `200-400` | getting heavy — consider extracting |
| OVER | `>400` | too big — extract to get back under budget (target `<200`) |

Defaults are overridable (`/gogo:skills --warn N --max N`). For a proxy file, only
the gogo-owned summary is measured — never the linked upstream. A user-owned
`## Custom` section is likewise excluded from the measured body and is never an
extraction candidate (`/gogo:skills` never proposes or rewrites it — mirroring
`## gogo overrides`). `/gogo:build` prints a nudge when a file passes the warn line.

**Two kinds of extracted skill.** `/gogo:skills` classifies each candidate and the
user confirms per candidate at an approval gate (it proposes, then STOPS — nothing
is written until you approve):

| Kind | Lives in | Loaded by | When |
|---|---|---|---|
| **knowledge** | `.gogo/skills/<slug>/` | the gogo pipeline, via the parent file's `**Load when:**` pointer | project-/convention-specific detail, only meaningful to a gogo phase |
| **standalone** | `.claude/skills/<slug>/` | the Claude Code harness, auto-discovered + invokable by name | a self-contained, reusable capability useful beyond this project |

A `knowledge` skill keeps the `.gogo/`-only invariant; it is **not** harness-auto-
discovered — the pipeline loads it only when the parent pointer's trigger matches
the task. A `standalone` skill is the **one sanctioned write outside `.gogo/`**,
and only ever for a candidate you explicitly approved as standalone. The parent
section is replaced by a short summary + a `**Load when:** <trigger> → <path>`
pointer, and `.gogo/skills/index.md` registers every extraction.

## 3. The complete file map

### Plugin side (ships with gogo, read-only to your project)

```
gogo/
├── commands/                 # ultra-thin entry points — 13 slash commands
│   ├── build.md  plan.md  go.md  accept.md  implement.md  review.md
│   ├── test.md  report.md  done.md  view.md  status.md  resume.md  skills.md
├── skills/                   # the operating manuals (all the logic)
│   ├── gogo/                 #   orchestrator: phases, loops, decision gates
│   ├── gogo-build/           #   wire/refresh .gogo/knowledge config
│   ├── gogo-plan/            #   ① plan
│   ├── gogo-implement/       #   ② implement
│   ├── gogo-review/          #   ③ review
│   ├── gogo-test/            #   ④ test
│   ├── gogo-knowledge/       #   ⑤ report + knowledge update (strict + lenient)
│   ├── gogo-done/            #   ship: synthesize high-level entry (single or merged) → .gogo/changelog/ + build/print viewer link; no-slug work board cockpit (view/ship/merge/go/filter intents + relaunch loop)
│   ├── gogo-view/            #   interactive viewer for plans + reports (rich draggable nodes + before/after compare)
│   ├── gogo-status/          #   read-only overview + the shared work-index classifier (shipped/ready/in-progress/unfinished)
│   ├── gogo-accept/          #   accept a plan from the board (records via gogo-plan's single-owner recording; Slice C)
│   ├── gogo-skills/          #   audit knowledge budget + extract on-demand skills
│   ├── gogo-contracts/       #   validate-in / validate-out at every hand-off
│   └── gogo-mermaid/         #   diagram generation + offline viewer
├── agents/                   # gogo, gogo-analyst, gogo-developer, gogo-reviewer, gogo-tester
├── templates/
│   ├── knowledge/            #   the 10 knowledge-file scaffolds
│   ├── contracts/            #   JSON Schemas for the typed artifacts + README
│   ├── skill.template.md     #   scaffold for an extracted skill
│   ├── skills-index.template.md  # scaffold for .gogo/skills/index.md
│   ├── state.template.md  decisions.template.md  report.template.md
├── hooks/                    # config-check.sh, notify.sh, hooks.json (best-effort)
├── cli/                      # the `gogo` CLI — a Go/Bubble Tea cockpit (NOT a slash
│   │                         #   command; a separate binary): deterministic reader of
│   │                         #   the cli-contract, kanban board, terminal viewers,
│   │                         #   Claude-launching moves. `cd cli && go build -o gogo .`
│   ├── main.go + status/view/events   #   root board + non-interactive subcommands
│   ├── internal/contract/    #   parse state.md/manifests/issues/events + the classifier
│   ├── internal/pages/       #   goldmark `w` page builder (+ go:embed of assets/viewer +
│   │                         #   mermaid.min.js; re-sync via `make sync-assets`)
│   ├── internal/tui/         #   bubbletea board · drill-in · glamour/huh
│   ├── internal/{launch,diagram,textfmt}/  # tmux/claude spawn · mmd→ASCII · shared formatters
│   └── go.mod / go.sum       #   pinned deps (binary is gitignored, built from source)
├── assets/
│   ├── mermaid/              #   vendored mermaid.min.js + viewer.template.html
│   ├── viewer/               #   the interactive viewer (modular, vanilla, no build):
│   │   │                     #   geometry.js · viewport.js · mermaid-parse.js ·
│   │   │                     #   render.js · interactive.js · viewer.css · viewer.template.html
│   │   │                     #   (also go:embed-copied into cli/internal/pages/assets/)
│   └── kanban/              #   board.py — vendored python3 curses TUI for the /gogo:done work board (soft dep; --selftest headless)
├── .mcp.json                 # Playwright MCP (optional; UI testing)
└── .claude-plugin/
    ├── plugin.json           # manifest + version (bump on any behaviour change)
    └── marketplace.json      # marketplace entry
```

### Project side (created in your repo)

```
your-project/
├── .gogo/
│   ├── knowledge/            # your config — 10 files (see the table in §1)
│   ├── skills/               # knowledge-kind skills live here; index.md registers ALL extractions
│   │   ├── index.md          #   the registry of every extraction: kind · destination · trigger · source · lines saved
│   │   └── <slug>/SKILL.md   #   one per knowledge extraction (+ optional scripts/, .env.example)
│   ├── resources/            # vendored mermaid.min.js (shared by all features) + viewer/ module set + view/ built pages + kanban/ (work board scratch: board.py, work-index.json, board-intent.json, board-exit.code)  [gogo-mermaid, /gogo:view, /gogo:done write]
│   ├── changelog/            # append-only shipped archive: <YYYY-MM-DD>-<name>/ (SYNTHESIZED report.md + slug-prefixed .mmd + manifest.json{members[]} + before/; single or merged; no diagrams.html)  [/gogo:done writes]
│   └── work/
│       └── feature-<slug>/   # one folder per piece of work:
│           ├── plan.md            # the accepted plan (the contract) + functional requirements   [① writes]
│           ├── adjustments.md     # log of changes/clarifications during planning                 [① writes]
│           ├── state.md           # current phase/status/iterations — lets work resume            [every phase]
│           ├── decisions.md       # forks that needed your call + recommendation + answer          [gates]
│           ├── uat.md             # the UAT gate log — one round per user check after ⑤ (/gogo:done accept, or an analyst issues round)  [UAT gate]
│           ├── events.jsonl       # append-only progress telemetry — one JSON line per phase transition (read by the gogo CLI)  [every transition]
│           ├── review/issues.json # living, typed review findings (the contract)                  [③ writes, ② reads]
│           ├── review-NN.md        # each review round's rendered snapshot                          [③ writes]
│           ├── test/issues.json    # living, typed test findings (same contract)                   [④ writes, ② reads]
│           ├── test-NN.md          # each test round's rendered snapshot                            [④ writes]
│           ├── report/             # as-built bundle: report.md + UML .mmd set + report/before/ (plan-time "before" set, copied in) + diagrams.html + manifest.json  [⑤ writes]
│           ├── <phase>/result.json # per-run phase result (implement/review/test/report)           [each phase writes]
│           ├── pipeline.json       # feature-level index of current artifacts + validity           [every phase]
│           └── charts/             # mermaid .mmd + charts/before/ (plan-time as-is baseline) + manifest.json + offline diagrams.html  [①/② write]
└── .claude/
    └── skills/<slug>/SKILL.md  # approved STANDALONE skills (harness auto-discovers)  [/gogo:skills, user-gated]
```

**Who reads/writes what, by phase:**

- **① Plan** (`gogo-analyst`, delegated by the orchestrator) — reads `analysis.md`
  (the procedure), `project-knowledge`, `tech-stack`, `non-functional-requirements`,
  `coding-rules`; analyses the goal against the actual codebase (**code = source of
  truth**); creates the feature folder; writes `plan.md`, `state.md`, the
  intended-design `charts/`. **The orchestrator owns the acceptance gate in chat.**
- **② Implement** (the **orchestrator, in-context** on `/gogo:go` — kept warm
  across the fix loop so it never re-explores the tree; `gogo-developer` backs
  standalone `/gogo:implement` + hands-off) — reads `plan.md`, `coding-rules`,
  `tech-stack`; writes code, the as-built `charts/`, `implement/result.json`; in
  fix-mode reads/writes `*/issues.json`.
- **③ Review** (`gogo-reviewer`) — reads the diff, `code-review-standards`,
  `non-functional-requirements`; writes `review/issues.json` + `review-NN.md`.
- **④ Test** (`gogo-tester`) — reads `testing-tools`, `test-strategy`,
  `tech-stack`; writes `test/issues.json` + `test-NN.md`.
- **⑤ Report** (orchestrator) — finalizes `plan.md`, writes the `report/` bundle
  (`report/report.md` + the as-built UML set + `diagrams.html`), updates the
  gogo-owned knowledge summaries that drifted. `/gogo:done` then **synthesizes** a
  high-level entry from it into `.gogo/changelog/<date>-<name>/` (single or merged) —
  it does not copy the bundle; the `report/` bundle stays the full audit trail.

The typed artifacts (`*/issues.json`, `charts/manifest.json`, per-run
`result.json`, the feature `pipeline.json`) follow the JSON Schemas in
`templates/contracts/`; each phase validates them in and out via `gogo-contracts`
so a bad LLM hand-off is caught, not propagated.

## See also

- [Home](index.md) — the pitch, install/update, and the documentation map.
- [Commands](commands.md), [Flow](flow.md), [Agents](agents.md),
  [Discovery](discovery.md), [Contracts](contracts.md).
- `.gogo/knowledge/index.md` (in a wired project) — the live purpose-map.
- `skills/gogo/SKILL.md` — the orchestrator's operating manual (the authoritative
  description of the flow, the loops, and the decision gates).
