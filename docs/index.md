---
title: Home
nav_order: 1
permalink: /
---

# gogo

A portable, knowledge-grounded development pipeline for Claude Code.
{: .fs-6 .fw-300 }

> **The flow is generic and ships with the plugin. The rules are yours.**

gogo runs every non-trivial change through five fixed phases — **plan ->
implement -> review -> test -> report** — but *what* it plans against, *how* it
writes code, *what* review flags, and *how* it tests are all driven by plain
markdown **knowledge files** that gogo wires up from your existing project docs.
Same pipeline everywhere; the behaviour is configuration.

That is the first of two splits that make gogo work:

- **Flow vs knowledge** — the generic phases ship with the plugin; the
  per-project rules live in `.gogo/knowledge/`.
- **Always-read knowledge vs on-demand skills** — situational detail is pulled
  out of the always-read config into skills that load only when a task needs
  them, so each phase's context stays small and the LLM workers stay
  deterministic.

> **Code and skills are authoritative.** This site is *generated from* the
> plugin's `commands/`, `agents/`, `skills/`, and `templates/`, and may lag them.
> When in doubt, the `skills/*/SKILL.md` files are the source of truth — and
> that same principle now drives discovery itself (see
> [Discovery](discovery.md): code is checked against the docs, and code wins).

## Install

```
/plugin marketplace add ZawadzkiB/gogo
/plugin install gogo@gogo
```

Hacking on gogo itself? Add your local clone as the marketplace instead
(they share the name `gogo`, so use one or the other):
`/plugin marketplace add /path/to/gogo`.

## Update

`/plugin install` reads a **local copy** of the marketplace, so installing on its
own never pulls a newer version. Refresh the marketplace first, then reinstall:

```
/plugin marketplace update gogo   # fetch the latest gogo from GitHub
/plugin install gogo@gogo         # install the bumped version
/reload-plugins                   # apply it to the running session
```

Using a local clone as the marketplace? A plain `git pull` in the clone is
enough — no `marketplace update` — followed by `/reload-plugins`.

## Quick start

```
/gogo:build                 # wire gogo to this project's docs (run once; re-run anytime)
/gogo:plan "add CSV export to the reports page"
# review the plan, accept it, then:
/gogo:go
```

`/gogo:build` discovers your existing docs and wires the `.gogo/knowledge/`
config (and now **verifies the distilled facts against your actual code**).
`/gogo:plan` writes an accept-pending plan and stops for you. `/gogo:go` runs
implement -> review -> test -> report, pausing only at real decisions.

## Documentation map

| Page | What it covers |
|---|---|
| [Commands](commands.md) | Every `/gogo:*` command — purpose, args, what it reads and writes |
| [Flow](flow.md) | The five phases, the implement<->review<->test loops, decision gates, resume |
| [Agents](agents.md) | The I/O reference — what each agent consumes and produces |
| [Discovery](discovery.md) | How `/gogo:build` finds and wires knowledge, and verifies it against code |
| [Contracts](contracts.md) | The typed artifacts that cross phase boundaries + the validate gate |
| [Architecture](architecture.md) | The two splits and the complete file map |

## Portability

The core **plan -> implement -> review -> test** loop needs **no external
dependencies**. Mermaid is vendored for the offline diagram viewer; the
Playwright MCP (UI testing), `mmdc`, and `jq` are all optional and degrade
gracefully. See the [README](https://github.com/ZawadzkiB/gogo) for the full
prerequisites list.
