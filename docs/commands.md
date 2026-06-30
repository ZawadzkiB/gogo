---
title: Commands
nav_order: 2
---

# Commands

Every command is an **ultra-thin entry point** — it carries no flow logic, it
just invokes a skill and passes arguments. The logic lives in the skills (the
"operating manuals"). Source of truth: `commands/*.md` and the `skills/*/SKILL.md`
they invoke.

There are three groups: **orchestration** (`build`, `plan`, `go`, `status`,
`resume`), the **standalone phase commands** (`implement`, `review`, `test`,
`report` — each a typed function with validate-in / validate-out), and
**knowledge maintenance** (`skills`).

## Orchestration

### `/gogo:build [--force]`

Set up or refresh the project's knowledge config. Runs the `gogo-build` skill.

- **Reads:** the project's existing docs at any depth — Claude / Copilot / Cursor
  / Windsurf / Codex configs, README/CONTRIBUTING/ARCHITECTURE, `docs/`,
  manifests + lockfiles, test/lint/CI configs — plus a full markdown sweep and a
  light pass over in-code doc comments. On a re-run it also reads the existing
  `.gogo/knowledge/*`.
- **Writes:** `.gogo/knowledge/*` (each wired as a **proxy** that links the real
  source, or **owned** and synthesized when none exists) and `_discovered.md`.
- **Now also:** verifies the high-signal distilled facts against the actual code
  and records verified / corrected / unverifiable (see [Discovery](discovery.md)).
- **Idempotent:** re-run anytime — picks up new docs, preserves every
  `## gogo overrides` section and every `Mode: owned` body. `--force` resets to
  fresh scaffolds.

### `/gogo:plan "<goal>"`

Runs **phase ① (plan) only**. Acts as the orchestrator via the `gogo` + `gogo-plan`
skills.

- **Reads:** `.gogo/knowledge/*` (config gate — stops if missing) and the codebase.
- **Writes:** `.gogo/plans/feature-<slug>/` with `plan.md` (incl. the feature's
  functional requirements), `adjustments.md`, `state.md`, and an intended-design
  mermaid chart.
- **Stops for acceptance** — no code is written until you accept. Hard gate.

### `/gogo:go [feature-slug]`

Runs **phases ② -> ③ -> ④ -> ⑤** for an accepted plan. Acts as the orchestrator
in chat, so it can pause at gates.

- **Reads:** `state.md` (refuses unless `plan-accepted` or a resumable in-loop
  state) and the relevant knowledge.
- **Delegates:** ② implement -> `gogo-developer`, ③ review -> `gogo-reviewer`,
  ④ test -> `gogo-tester`; routes findings through the loop (fixable ->
  re-implement; decision -> ask the user; clean/green -> advance) and keeps
  `state.md` current. Bounds implement<->review at ~3 rounds.
- The four phase commands below are the same steps it chains.

### `/gogo:status`

Lists every `.gogo/plans/feature-*/` with slug, title, phase, status, iteration
counts, and resume hint; flags any `waiting-for-user` feature with its open
decision. Read-only.

### `/gogo:resume [feature-slug]`

Resumes a feature that paused for your decision.

- **Reads:** `state.md` + `decisions.md`.
- **Writes:** appends a `### RESOLVED (user, <date>)` block, clears
  `open-decision`, and re-enters the pipeline at `state.md`'s `resume:` phase.

## Standalone phase commands

Each is a thin, idempotent entry point to its phase skill that **validates its
inputs** before working and **validates its outputs** before hand-off, via the
[contract layer](contracts.md). `/gogo:go` chains these same commands.

### `/gogo:implement [feature-slug] [--issues <path>]`

Phase ② via `gogo-implement` (delegates to `gogo-developer`).

- **Reads:** `plan.md` (accepted), `coding-rules.md`, `tech-stack.md`; with
  `--issues <path>`, a `review/issues.json` or `test/issues.json`.
- **Writes:** code, the as-built `charts/` set + `charts/manifest.json`,
  `implement/result.json`. In `--issues` mode it fixes the `open`/`new` issues
  and writes back `status: fixed`, `fix_summary`, `fixed_in_round`.

### `/gogo:review [feature-slug]`

Phase ③ via `gogo-review` (delegates to `gogo-reviewer`).

- **Reads:** `plan.md`, `code-review-standards.md`, `coding-rules.md`,
  `non-functional-requirements.md`, the as-built `charts/manifest.json`, any prior
  `review/issues.json`.
- **Writes:** the living `review/issues.json` (open -> fixed/verified, append
  `new`), a `review-NN.md` snapshot, `review/result.json`. Routes: open issues ->
  implement with `--issues`; clean -> test.

### `/gogo:test [feature-slug]`

Phase ④ via `gogo-test` (delegates to `gogo-tester`).

- **Reads:** `plan.md` (Tests section), `testing-tools.md`, `test-strategy.md`,
  `tech-stack.md`, `non-functional-requirements.md`, the as-built
  `charts/manifest.json`, any prior `test/issues.json`.
- **Writes:** the living `test/issues.json`, a `test-NN.md` snapshot,
  `test/result.json`. Routes: open issues -> implement with `--issues`;
  all-green -> report.

### `/gogo:report [feature-slug]`

Phase ⑤ via `gogo-knowledge`. For an all-green feature.

- **Reads:** `plan.md`, `state.md`, `review/issues.json`, `test/issues.json`,
  `charts/manifest.json`, the gogo-owned `.gogo/knowledge/*` summaries.
- **Writes:** the finalized as-built `plan.md`, `report.md`, the as-built diagram
  set, updated gogo-owned knowledge docs (never the proxied originals),
  `report/result.json`, and sets `state.md` to done.

## Knowledge maintenance

### `/gogo:skills ["<prompt>"] [--warn N] [--max N] [--include <path>]`

Keep `.gogo/knowledge/*` lean so the pipeline stays deterministic. Runs the
`gogo-skills` skill.

- **No prompt** -> audit / auto-discover: measure each file's body lines (OK
  `<200` · WARN `200-400` · OVER `>400`), discover cohesive extraction candidates,
  classify each as a `knowledge` skill (-> `.gogo/skills/`) or a `standalone`
  skill (-> `.claude/skills/`), and **propose them, then STOP for per-candidate
  approval** before writing anything.
- **`"<prompt>"`** -> directed: extract exactly the section you name.
- **`--warn N` / `--max N`** -> override the 200 / 400 thresholds.
- **`--include <path>`** -> also audit a path outside `.gogo/` (report-only,
  never extracted).
- On approval it writes each skill's `SKILL.md` (+ optional `scripts/` /
  `.env.example`), replaces the parent section with a `**Load when:**` pointer,
  and updates `.gogo/skills/index.md`. The only write outside `.gogo/` is an
  approved standalone skill's `.claude/skills/<slug>/`. See
  [Discovery](discovery.md#knowledge--on-demand-skills) for the budget rationale.
