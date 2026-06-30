---
name: gogo-skills
description: >-
  Keep .gogo/knowledge/* lean so the pipeline stays deterministic. Audits each
  knowledge file against a line budget, auto-discovers cohesive sections worth
  pulling out, classifies each as a knowledge skill (→.gogo/skills/) or a
  standalone skill (→.claude/skills/), proposes them and STOPS for per-candidate
  approval, then extracts each into a SKILL.md and replaces the parent section
  with a Load-when pointer. Runs on /gogo:skills; also directed via
  /gogo:skills "<prompt>". Pure Glob/Grep/Read/Write, idempotent, no new dependency.
---

# gogo-skills — extract knowledge bloat into on-demand skills

Big always-loaded context makes LLM workers wander and err. This skill moves
self-contained, situational detail out of the always-read `.gogo/knowledge/*.md`
files into **on-demand skills** that load only when a task needs them. It is
**pure** (Glob/Grep/Read/Write only), **idempotent**, and **propose-then-act** —
it writes nothing until the user approves each candidate. Modelled on `gogo-build`.

## Modes
- **Audit / auto-discover (default):** `/gogo:skills` — scan, measure, discover
  candidates, classify, propose, STOP for approval, then extract.
- **Directed:** `/gogo:skills "<prompt>"` — skip discovery, target exactly the
  section the user names; still classify + propose + extract. "Trivially
  unambiguous" only lets you skip asking *which* section is meant — the
  propose-then-STOP approval gate (Step 4) **always** runs before any write.

## Budget
- Count the **body lines** of each knowledge file (exclude the `gogo:meta` HTML
  header). Classify **OK** `<200` · **WARN** `200-400` · **OVER** `>400`. Defaults
  overridable: `--warn N`, `--max N`.
- For a **proxy** file (`Mode: proxy`), measure only the gogo-owned summary body
  we control — **never** the linked upstream's length.

## Step 1 — audit (read-only)
Glob `.gogo/knowledge/*.md`. For each, count body lines (`wc -l` if present, else
read and count) and emit a table: `file · lines · OK|WARN|OVER`. Also measure each
`--include <path>` target the same way and add its row flagged **report-only
(never extracted)** — never a candidate, never written (see Safety). An all-OK set
with no directed prompt → say so and stop (nothing to do).

## Step 2 — discover candidates (WARN/OVER files)
Parse the heading structure. Unit = **H2** by default; drop to **H3** when a
single H2 is itself oversized (D2). Propose a section only when it is:
- **cohesive** — one self-contained concern;
- **context-local** — not needed on every read of the file;
- **standalone-able** — a clear trigger, no hard dependency on its siblings;
- **big enough to matter** — roughly **≥20 lines** saved; smaller sections aren't
  worth their own skill (leave them inline).
Rank candidates by `lines-saved × locality`. A tight, cohesive file yields **no**
candidates (no false positives). Never flag the `gogo:meta` header or a
`## gogo overrides` section.

## Step 3 — classify each candidate (kind + destination) (D1)
- **knowledge** → `.gogo/skills/<slug>/` — project-/convention-specific, prose-
  heavy, only meaningful to a gogo phase. Loaded by the pipeline via the parent
  pointer; honors `.gogo/`-only; **not** harness-auto-discovered.
- **standalone** → `.claude/skills/<slug>/` — crisp trigger, self-contained,
  carries its own scripts/env, useful beyond this project. Harness auto-discovers
  it and can invoke it by name.
Recommend a kind with a one-line rationale. The user confirms or overrides the
kind **per candidate** at the gate.

## Step 4 — propose, then STOP (FR5)
For every candidate present, show: proposed **slug**, **kind + destination**,
**description** (the on-demand trigger), **source** file › section, **est. lines
saved**, any **scripts/env** it would bundle, and the **stub** that will replace
it. **Write nothing until the user approves** — per candidate. Use
`AskUserQuestion` for the approve / override-kind / decline gate.

## Step 5 — extract each approved candidate
Write `<dir>/<slug>/SKILL.md` from
`${CLAUDE_PLUGIN_ROOT}/templates/skill.template.md`:
- **frontmatter** — `name: <slug>`; `description` engineered to trigger correct
  on-demand loading (lead with WHEN / keywords).
- **body** — the lifted content rewritten to **stand alone**: no "see above", no
  dangling reference to the parent file.
- **scripts/** — materialize fenced shell commands / runbooks from the section as
  `scripts/<name>.sh` (best-effort: `set -euo pipefail`, `|| true`).
- **.env.example** — document any env vars those scripts need (`NAME=` + comment;
  never real secrets).
Destination by kind: `knowledge` → `.gogo/skills/<slug>/`; `standalone` →
`.claude/skills/<slug>/`. A skill that would itself exceed `--max` is a smell —
split it.

## Step 6 — replace the parent section (FR8)
Swap the lifted section for a short summary + a pointer:
`**Load when:** <trigger> → <path>` — `../skills/<slug>/SKILL.md` for a knowledge
skill, or naming the now-discoverable `.claude/skills/<slug>` skill for a
standalone one. Keep the summary to a line or two.

## Step 7 — update the registry (FR9)
Create `.gogo/skills/index.md` from
`${CLAUDE_PLUGIN_ROOT}/templates/skills-index.template.md` if absent; append a row
per extracted skill: `skill · kind · destination · trigger · source (file ›
section) · lines saved`.

## Step 8 — re-measure + report
Re-count each touched parent; confirm it is under budget (target `<200` for an
OVER file). Report: per-file before/after lines, skills created (slug · kind ·
destination), total lines saved, anything declined.

## Idempotency
Re-runs skip already-extracted sections: a section already replaced by a
`**Load when:**` pointer (or already listed in `.gogo/skills/index.md`) is never a
candidate again. Safe to run repeatedly.

## Safety + portability (FR10)
- **Default writes are confined to `.gogo/`.** The **only** sanctioned write
  outside `.gogo/` is an **approved** `standalone` candidate's
  `.claude/skills/<slug>/` dir — never automatic, always per-candidate. (A
  deliberate, user-gated relaxation of the `.gogo/`-only invariant.)
- `--include <path>` may audit a path outside `.gogo/`, but that path is
  **report-only** — never a candidate, never written.
- Pure Glob/Grep/Read/Write; no new dependency.
