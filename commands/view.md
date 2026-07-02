---
description: Build and open a self-contained, offline interactive webpage for a gogo plan or report — the plan.md / report.md summary as readable HTML plus its mermaid diagrams made interactive (vendored runtime, no network, no build). Enumerates a grouped Work / Changelog picker with a case-insensitive text filter.
argument-hint: "[feature-slug[:plan|:report] | changelog-entry | path | filter-text]"
allowed-tools: Read, Write, Bash, Glob, Grep, Skill, TodoWrite, AskUserQuestion
model: opus
---

View a gogo plan or report as an interactive webpage, via the `gogo-view` skill.

Target: $ARGUMENTS  (a feature `<slug>` — its **report if one exists, else its plan**;
`<slug>:plan` or `<slug>:report` to force one; a `<date>-<name>` changelog entry; or a
path to a `plan.md`/`report.md`. A **non-resolving word** is treated as a text **filter**
over the enumerated items. If absent, `gogo-view` presents its grouped **Work** (each
feature's plan + report) / **Changelog** (shipped reports) picker, newest first.)

Load `gogo-view` and follow it: enumerate the grouped Work (plans + reports) /
Changelog menu, resolve `$ARGUMENTS` per the skill's arg grammar (an explicit target
that resolves to nothing stops; a **bare non-resolving arg becomes a case-insensitive
filter** over the enumerated items, and with more than 4 items the picker asks for a
filter first), else present the `AskUserQuestion` grouped picker, build the
self-contained offline page under `.gogo/resources/view/`, and open it — best-effort,
printing the absolute `file://` path on failure. All flow lives in the skill.
