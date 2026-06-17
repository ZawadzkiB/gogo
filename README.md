# gogo

**A portable, knowledge-grounded development pipeline for Claude Code.**

gogo drives any non-trivial change through five phases — **plan → implement →
review → test → report** — grounded in your project's own docs, pausing for you
only when a decision is genuinely yours.

The *flow* is generic and ships with the plugin. Your *project's* knowledge
(tech stack, conventions, review standards, testing strategy, non-functional
requirements) lives in small markdown config files that gogo wires up for you.
Everything the plugin needs travels with it — no global CLIs, no machine-specific
setup.

## The flow

```
user goal ─▶ ① PLAN ──(you accept)──▶ ② IMPLEMENT ─▶ ③ REVIEW ─▶ ④ TEST ─▶ ⑤ REPORT ─▶ done
              ▲  │                          ▲           │          │        (update plan +
              │  └──(clarify / changes)──▶ wait         │          │         knowledge docs)
              │                              └──issue────┘          │
              │                                (fix → re-review)    │
              └──── issue needs YOUR decision (from review or test) ┘
```

The **orchestrator** keeps the interactive bits — planning, the acceptance gate,
decision gates, and the final report — in the chat with you, and delegates the
heads-down work (implement, review, test) to specialist sub-agents.

## Quickstart

```
/plugin marketplace add ZawadzkiB/gogo
/plugin install gogo@gogo

/gogo:build               # wire gogo to this project's docs (run once; re-run anytime)
/gogo:plan "add CSV export to the reports page"
# review the plan, accept it, then:
/gogo:go
```

## Commands

| Command | What it does |
|---|---|
| `/gogo:build [--force]` | Initialize **or refresh** the knowledge config — discover your project's docs and wire them as proxies. Re-run anytime to pick up new docs; `--force` resets to fresh scaffolds. |
| `/gogo:plan "<goal>"` | Run the plan phase; writes an accept-pending plan to `.plans/feature-<slug>/`. Stops for your acceptance — no code is written. |
| `/gogo:go [slug]` | Implement the accepted plan through the review→test loop, pausing only at real decisions. |
| `/gogo:status` | List all features and their phase/status/iterations. |
| `/gogo:resume [slug]` | Resume a feature that paused for your decision, folding in your answer. |

## Agents

- **`gogo`** — the orchestrator: owns the flow/loop, knows what to run when, and
  delegates to the specialists. Also usable hands-off ("build X end-to-end").
- **`gogo-developer`** — implements the accepted plan and applies review/test fixes.
- **`gogo-reviewer`** — fresh-eyes, adversarial code review.
- **`gogo-tester`** — e2e/UI testing via the bundled Playwright MCP.

## What gets created in your project

gogo writes two top-level folders — both plain markdown you can read, edit, and
commit.

**`.gogo/knowledge/`** — your project's configuration (the pipeline reads these):

| File | Purpose |
|---|---|
| `index.md` | Purpose-map of this folder + the proxy convention |
| `project-knowledge.md` | Architecture, domains, glossary, key decisions |
| `tech-stack.md` | Languages, frameworks, build/run/test commands |
| `non-functional-requirements.md` | Standing bars: performance, security, accessibility, reliability, limits |
| `coding-rules.md` | Conventions the implementation must follow |
| `code-review-standards.md` | What the review phase checks for |
| `testing-tools.md` | The test tools and how to run them |
| `test-strategy.md` | How to test: journeys, UI checks, e2e levels, deploy checks |
| `_discovered.md` | What `/gogo:build` found (regenerated each run) |

**`.plans/feature-<slug>/`** — one folder per piece of work:

| File | Purpose |
|---|---|
| `plan.md` | The accepted plan (the contract), incl. the feature's functional requirements |
| `adjustments.md` | Log of changes/clarifications you asked for during planning |
| `state.md` | Current phase/status/iterations — lets work resume across sessions |
| `decisions.md` | Forks that needed your call, with gogo's recommendation + your answer |
| `review-NN.md` | Each code-review round's findings |
| `test-NN.md` | Each test round's results |
| `charts/` | Mermaid diagrams (`.mmd`) + an offline `diagrams.html` viewer |

## Configuration: the knowledge proxies

`.gogo/knowledge/*` files are **proxies** — they link to your project's real docs
(README, CONTRIBUTING, an existing `CLAUDE.md`, `.github/copilot-instructions.md`,
Cursor/Windsurf rules, etc.) and add a short gogo-specific summary, rather than
duplicating them. When a project has no doc for a topic, the file becomes the
home for it (gogo authors it from your code).

Re-running `/gogo:build` **reconciles**: it picks up newly-added docs, refreshes
summaries from the current upstream, and **preserves your edits** — anything
under a `## gogo overrides` section and any gogo-owned file is never clobbered.
Use `--force` for a full reset.

A feature's **functional** requirements (what *this* change must do) live in its
`plan.md`. The project's **standing non-functional** requirements (performance,
security, accessibility, …) live in config at
`.gogo/knowledge/non-functional-requirements.md`.

## Portability & prerequisites

gogo is built to run anywhere it's installed:

- The core **plan → implement → review → test** loop needs **no external
  dependencies**.
- **Mermaid** diagrams render natively in GitHub / VS Code / JetBrains from
  fenced ` ```mermaid ` blocks; the bundled offline viewer needs only a browser
  (mermaid is vendored — no network, no CLI).
- **Browser / UI testing** uses the bundled **Playwright MCP**, which boots via
  `npx` on first use (needs **Node.js**). Without it, the test phase falls back
  to API/CLI tests plus written manual steps.

Optional: set `GOGO_NTFY_TOPIC` in your shell to get a phone push (via
[ntfy.sh](https://ntfy.sh)) when gogo pauses for your input. Without it you still
get a local desktop notification + a terminal bell.

## License

MIT — see [LICENSE](./LICENSE).
