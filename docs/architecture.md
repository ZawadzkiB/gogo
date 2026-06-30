# How gogo works

Reference for the gogo architecture: the two splits that make it portable and
deterministic, and a complete map of what is stored where. For the quick pitch and
the command list, see the [README](../README.md).

## 1. The flow-vs-knowledge split

gogo runs every non-trivial change through five fixed phases:

```
goal → ① PLAN → ② IMPLEMENT → ③ REVIEW → ④ TEST → ⑤ REPORT → done
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
the gogo-owned summary is measured — never the linked upstream. `/gogo:build`
prints a nudge when a file passes the warn line.

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
├── commands/                 # ultra-thin entry points (one per slash command)
│   ├── build.md  go.md  plan.md  implement.md  review.md
│   ├── test.md   report.md  status.md  resume.md  skills.md
├── skills/                   # the operating manuals (all the logic)
│   ├── gogo/                 #   orchestrator: phases, loops, decision gates
│   ├── gogo-build/           #   wire/refresh .gogo/knowledge config
│   ├── gogo-plan/            #   ① plan
│   ├── gogo-implement/       #   ② implement
│   ├── gogo-review/          #   ③ review
│   ├── gogo-test/            #   ④ test
│   ├── gogo-knowledge/       #   ⑤ report + knowledge update
│   ├── gogo-skills/          #   audit knowledge budget + extract on-demand skills
│   ├── gogo-contracts/       #   validate-in / validate-out at every hand-off
│   └── gogo-mermaid/         #   diagram generation + offline viewer
├── agents/                   # gogo, gogo-developer, gogo-reviewer, gogo-tester
├── templates/
│   ├── knowledge/            #   the 9 knowledge-file scaffolds
│   ├── contracts/            #   JSON Schemas for the typed artifacts + README
│   ├── skill.template.md     #   scaffold for an extracted skill
│   ├── skills-index.template.md  # scaffold for .gogo/skills/index.md
│   ├── state.template.md  decisions.template.md  report.template.md
├── hooks/                    # config-check.sh, notify.sh, hooks.json (best-effort)
├── assets/mermaid/           # vendored mermaid.min.js + viewer.template.html
├── .mcp.json                 # Playwright MCP (optional; UI testing)
└── .claude-plugin/
    ├── plugin.json           # manifest + version (bump on any behaviour change)
    └── marketplace.json      # marketplace entry
```

### Project side (created in your repo)

```
your-project/
├── .gogo/
│   ├── knowledge/            # your config — 9 files (see the table in §1)
│   ├── skills/               # knowledge-kind skills live here; index.md registers ALL extractions
│   │   ├── index.md          #   the registry of every extraction: kind · destination · trigger · source · lines saved
│   │   └── <slug>/SKILL.md   #   one per knowledge extraction (+ optional scripts/, .env.example)
│   └── plans/
│       ├── .assets/          # one vendored mermaid runtime per project (not per feature)
│       └── feature-<slug>/   # one folder per piece of work:
│           ├── plan.md            # the accepted plan (the contract) + functional requirements   [① writes]
│           ├── adjustments.md     # log of changes/clarifications during planning                 [① writes]
│           ├── state.md           # current phase/status/iterations — lets work resume            [every phase]
│           ├── decisions.md       # forks that needed your call + recommendation + answer          [gates]
│           ├── review/issues.json # living, typed review findings (the contract)                  [③ writes, ② reads]
│           ├── review-NN.md        # each review round's rendered snapshot                          [③ writes]
│           ├── test/issues.json    # living, typed test findings (same contract)                   [④ writes, ② reads]
│           ├── test-NN.md          # each test round's rendered snapshot                            [④ writes]
│           ├── report.md           # the as-built final report                                     [⑤ writes]
│           ├── <phase>/result.json # per-run phase result (implement/review/test/report)           [each phase writes]
│           ├── pipeline.json       # feature-level index of current artifacts + validity           [every phase]
│           └── charts/             # mermaid .mmd + manifest.json + offline diagrams.html          [①/②/⑤ write]
└── .claude/
    └── skills/<slug>/SKILL.md  # approved STANDALONE skills (harness auto-discovers)  [/gogo:skills, user-gated]
```

**Who reads/writes what, by phase:**

- **① Plan** (orchestrator, in chat) — reads `project-knowledge`, `tech-stack`,
  `non-functional-requirements`; creates the feature folder; writes `plan.md`,
  `state.md`, the intended-design `charts/`. **Stops for your acceptance.**
- **② Implement** (`gogo-developer`) — reads `plan.md`, `coding-rules`,
  `tech-stack`; writes code, the as-built `charts/`, `implement/result.json`; in
  fix-mode reads/writes `*/issues.json`.
- **③ Review** (`gogo-reviewer`) — reads the diff, `code-review-standards`,
  `non-functional-requirements`; writes `review/issues.json` + `review-NN.md`.
- **④ Test** (`gogo-tester`) — reads `testing-tools`, `test-strategy`,
  `tech-stack`; writes `test/issues.json` + `test-NN.md`.
- **⑤ Report** (orchestrator) — finalizes `plan.md`, writes `report.md` + as-built
  `charts/`, updates the gogo-owned knowledge summaries that drifted.

The typed artifacts (`*/issues.json`, `charts/manifest.json`, per-run
`result.json`, the feature `pipeline.json`) follow the JSON Schemas in
`templates/contracts/`; each phase validates them in and out via `gogo-contracts`
so a bad LLM hand-off is caught, not propagated.

## See also

- [README](../README.md) — the pitch, the command list, install/update.
- `.gogo/knowledge/index.md` (in a wired project) — the live purpose-map.
- `skills/gogo/SKILL.md` — the orchestrator's operating manual (the authoritative
  description of the flow, the loops, and the decision gates).
