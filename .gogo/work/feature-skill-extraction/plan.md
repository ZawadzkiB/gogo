# Plan — Knowledge skill-extraction command (`/gogo:skills`)

Status: **done** (2026-06-29). Accepted (user, 2026-06-29) — per-candidate kind model; D1–D4 as recommended.

## As-built outcome
- **Shipped as planned** — `/gogo:skills` (audit + auto-discover + directed modes),
  the `gogo-skills` operating manual (116 lines, under its own 200 budget), the
  `skill.template.md` + `skills-index.template.md` scaffolds, the `gogo-build`
  over-budget nudge, the orchestrator `Load when:` note, the 200/400 budget +
  user-gated `.claude/skills/` exception documented in coding-rules + NFR (this
  repo **and** templates), and **FR12** `docs/architecture.md`. Version **0.2.0 →
  0.3.0**. Working tree **uncommitted** (no commit requested).
- **All FR1–FR12 satisfied.** Per-candidate `knowledge`/`standalone` classification
  with destination routing; `.claude/skills/` is the one user-gated write outside
  `.gogo/`.
- **Review:** 2 rounds — APPROVE with 2 minors (REV-001 coding-rules/NFR invariant
  asymmetry; REV-002 `--include` not wired into the audit step) → both fixed +
  **verified**. **Test:** GREEN — all 10 plan test items verified by dogfooding a
  scratch fixture; 4 nit/minor doc findings (TEST-001–004) fixed + **verified**.
- **Residual:** the plugin was not live-installed in this environment, so the
  `AskUserQuestion` gate + live `/gogo:skills` invocation were verified by
  walking the skill against fixtures, not a harness run. Follow-up: one live
  dogfood run against a real over-budget project to fully close the done-bar.
- Full write-up: [report.md](./report.md). Diagrams: [charts/diagrams.html](./charts/diagrams.html).

## Goal
Add a new gogo command — `/gogo:skills` (the user's *"goSkils"*) — that keeps
knowledge files **lean enough that the pipeline agents stay deterministic**. Big
always-loaded context makes LLM workers wander and err; the fix is to move
self-contained, situational detail out of the always-read `.gogo/knowledge/*.md`
files and into **on-demand skills** that load only when a task actually needs
them. The command **audits** knowledge files against a line budget, **auto-
discovers** good extraction candidates, **proposes** them and **waits for the
user**, then — on approval — **extracts** each into a skill (proper description, optional
scripts/env) and replaces the parent section with a short pointer. It also runs in
a **directed** mode: `/gogo:skills "<prompt>"` extracts exactly the part the user
names.

Crucially, each candidate is **classified per-skill** into one of two kinds: a
**knowledge skill** — situational detail only the gogo pipeline needs here, which
stays under `.gogo/skills/` and is read by the pipeline via a pointer — or a
**standalone skill** — a self-contained, reusable capability worth Claude Code
auto-discovery, which goes to `.claude/skills/`. The command recommends a kind;
the user confirms per candidate at the gate.

## Context — what exists today
- **Commands** are ultra-thin `.md` entry points over skills: `build`, `plan`,
  `go`, `implement`, `review`, `test`, `report`, `status`, `resume`. The new
  command is a **knowledge-maintenance utility**, a sibling of `build` — *not* a
  pipeline phase.
- **Knowledge** lives in a target project at `.gogo/knowledge/*.md`. The pipeline
  reads specific files at specific phases (plan/implement/review/test). Today
  every read pulls the **whole** file — there is no "load this part only when
  relevant" mechanism. Current files here are small (29–69 lines); the problem is
  **forward-looking**: real projects grow these files past the point of
  determinism.
- **`gogo-build`** discovers docs and wires `.gogo/knowledge/` (proxy/owned); it
  is the model for a pure, idempotent, propose-then-act utility (Glob/Grep/Read/
  Write only, no compiled tool).
- **Skills format** is already used by the plugin: `skills/<name>/SKILL.md` with
  YAML frontmatter (`name`, `description`) + prose. `description` is what drives
  on-demand recognition. The extracted skills reuse this exact shape.
- **Hard invariants** (`.gogo/knowledge/coding-rules.md`, `non-functional-
  requirements.md`): only ever write under `.gogo/`; never edit a proxied
  upstream file; the core loop needs **zero external deps**; keep every
  enumeration in sync; bump `plugin.json` on any behavioural change.

## Functional requirements
- **FR1 — New command `/gogo:skills`.** Thin entry over a new `gogo-skills`
  skill. Two modes: **audit/auto-discover** (no prompt, default) and **directed**
  (`/gogo:skills "<prompt>"`). Idempotent and re-runnable. A maintenance utility
  alongside `build`, not a pipeline phase.
- **FR2 — Budget audit.** Scan every `.gogo/knowledge/*.md`; count body lines;
  classify **OK** `<200`, **WARN** `200–400`, **OVER** `>400`. Emit a table.
  Thresholds default to 200/400 (overridable: `--warn N`, `--max N`). For a
  **proxy** file, measure only the gogo-owned summary body we control — never the
  linked upstream length.
- **FR3 — Auto-discover candidates.** For WARN/OVER files, parse the heading
  structure and propose extraction candidates: cohesive, self-contained sections
  that are (a) **context-local** (not needed on every read), (b) **standalone-
  able** with a clear trigger, and (c) **big enough to matter**. Rank by
  `lines-saved × locality`. A tight, cohesive file yields **no** candidates (no
  false positives).
- **FR4 — Classify each candidate (kind + destination).** Each candidate is
  classified as **`knowledge`** (project/pipeline-scoped → `.gogo/skills/`) or
  **`standalone`** (a self-contained, reusable capability → `.claude/skills/`),
  with a recommendation and a one-line rationale. Heuristics: project-/convention-
  specific, prose-heavy, only meaningful to a gogo phase ⇒ `knowledge`; crisp
  trigger, self-contained, carries its own scripts/env, useful beyond this project
  ⇒ `standalone`. The user confirms or overrides the kind **per candidate**.
- **FR5 — Propose, then STOP.** For each candidate present: proposed **slug**,
  **kind + destination** (FR4), **description** (the on-demand trigger),
  **source** file+section, **est. lines saved**, any **scripts/env** it would
  bundle, and the **stub** that replaces it. **Write nothing until the user
  approves** (per candidate).
- **FR6 — Directed extraction.** `/gogo:skills "<prompt>"` skips discovery and
  targets exactly the section the user names; it still classifies (FR4), shows the
  proposal, and confirms (unless trivially unambiguous).
- **FR7 — Skill output (by kind).** Each extracted skill is a `<dir>/<slug>/
  SKILL.md` with YAML frontmatter (`name`; `description` engineered to trigger
  correct on-demand loading), the lifted content rewritten to **stand alone** (no
  dangling references), plus optional `scripts/` (materialized from fenced commands
  / runbooks in the section) and `.env.example` documenting required env vars.
  Destination by kind (FR4): `knowledge` → `.gogo/skills/<slug>/`; `standalone` →
  `.claude/skills/<slug>/`. Skills obey the same budget (a skill `>400` lines is
  itself a smell → split).
- **FR8 — Parent stays lean + pointer.** The extracted section is replaced in the
  parent with a short summary + a `**Load when:** <trigger> → <path>` pointer (to
  `../skills/<slug>/SKILL.md` for knowledge skills, or naming the now-discoverable
  `.claude/skills/<slug>` skill for standalone ones). Extraction must bring an OVER
  file **under budget** (target `<200`).
- **FR9 — Skills registry.** `.gogo/skills/index.md` lists every extracted skill
  (kind · destination · trigger description · source · lines saved), so the
  pipeline and the user know what skills exist and where each lives.
- **FR10 — Safety + portability.** Default writes are confined to `.gogo/`. The
  **only** sanctioned write outside `.gogo/` is a `.claude/skills/<slug>/` skill
  for a candidate the user has **explicitly approved as `standalone`** — never
  automatic, always per-candidate. Auditing a non-`.gogo/` path (`--include
  <path>`) is **report-only**. Pure Glob/Grep/Read/Write; no new dependency.
  (This is a deliberate, user-gated relaxation of the "only write under `.gogo/`"
  invariant — documented as such in `coding-rules.md`.)
- **FR11 — Pipeline awareness + build nudge.** The orchestrator's "knowledge —
  read before you plan" guidance notes that knowledge files may point to on-demand
  skills under `.gogo/skills/` — **load a pointed skill only when relevant**.
  `/gogo:build` prints a nudge (`<file> is NNN lines — consider /gogo:skills`)
  when a knowledge file exceeds the warn threshold. The 200/400 budget becomes a
  documented authoring rule in `coding-rules.md` + `non-functional-requirements.md`
  (this repo) and in `templates/knowledge/*` (so new projects inherit it).
- **FR12 — "How gogo works" documentation.** A dedicated `docs/architecture.md`
  (linked from README, which keeps a short "How it works" pointer) documents the
  current model end-to-end: (a) the **flow-vs-knowledge split** — the generic
  plan→implement→review→test→report flow ships with the plugin; the per-project
  rules live in `.gogo/knowledge/` ("same pipeline everywhere; behaviour is
  configuration"); (b) the **knowledge-vs-on-demand-skills split** introduced here
  and *why* (context budget → agent determinism); (c) the **complete file map** —
  what is stored where on both the **plugin side** (`commands/`, `skills/`,
  `agents/`, `templates/contracts/`, `hooks/`, `assets/`) and the **project side**
  (`.gogo/knowledge/`, `.gogo/plans/feature-*/`, `.gogo/skills/`, and standalone
  `.claude/skills/`), and which phase reads/writes each.

## Approach (recommended)
A **pure knowledge-maintenance command**, modelled on `gogo-build`: read-first,
classify, propose, gate on the user per candidate, then act.

1. **`gogo-skills` skill (new)** — the operating manual: `audit → discover →
   classify → propose → STOP → extract → re-measure → report`, both modes, the
   budget rules, the kind heuristics, and the safety/portability guardrails.
2. **`commands/skills.md` (new, thin)** — argument-hint, allowed-tools, invoke the
   skill, pass `$ARGUMENTS`.
3. **`templates/skill.template.md` (new)** — the SKILL.md scaffold for an
   extracted skill (frontmatter + standalone body + optional `scripts/`/`.env`),
   plus a `.gogo/skills/index.md` registry scaffold.
4. **Per-candidate kind decides destination** (D1): `knowledge` skills live in
   `.gogo/skills/` and are read by the pipeline via the parent's pointer only when
   the task touches them (the same selective-read the pipeline already does for
   knowledge, one level deeper); `standalone` skills go to `.claude/skills/` so the
   harness auto-discovers them — written **only** when the user approves that
   candidate as standalone.
5. **Integrate** — `gogo-build` over-budget nudge; orchestrator knowledge-read
   note; the 200/400 budget documented in coding-rules + NFR + templates.
6. **Sync + docs + version** — update every enumeration (README commands + "what
   gets created", any command/file-set lists), bump `plugin.json`.

### Alternatives considered
- **A single global location for all extractions** (everything to `.gogo/` *or*
  everything to `.claude/skills/`) — *rejected*: conflates two kinds. Some detail
  is project-local (belongs in `.gogo/`); some is a reusable capability (deserves
  auto-discovery). Per-candidate classification (FR4) fits both.
- **Auto-extract above the hard limit without asking** — *rejected*: the user
  explicitly wants a proposal + approval gate; silent restructuring of a config
  file is surprising and risky.
- **A typed `skill-proposal` JSON contract** (like `issues-list`) for a
  validatable propose→extract hand-off — *deferred* (D4): over-engineered for a
  human-gated v1; the durable artifact is `.gogo/skills/index.md`. Revisit only
  if `/gogo:go` ever needs to chain extraction.
- **A compiled line-counter/analyzer** — *rejected*: breaks the dependency-free
  bar; `wc -l`/agent-reading suffices.

## Open decisions (recommendations — see `decisions.md`)
- **D1 — Loading model = per-candidate kind, not a global location.** Each
  candidate is classified `knowledge` (→ `.gogo/skills/`, read by the pipeline via
  pointer; honors `.gogo/`-only; **not** harness-auto-discovered) or `standalone`
  (→ `.claude/skills/`, harness auto-discovers + invokable by name; session-
  global). The command recommends a kind; the user confirms per candidate;
  `.claude/skills/` is written only for an approved standalone. *Load-bearing —
  confirm this is the model you want.*
- **D2 — Extraction granularity.** Section unit = H2 by default; drop to H3 when a
  single H2 is itself oversized. **Rec: H2 default, H3 when needed.**
- **D3 — Thresholds.** 200 warn / 400 hard (per the request), overridable via
  `--warn`/`--max`. **Rec: as specified.**
- **D4 — Proposal artifact.** Presented prose + durable `.gogo/skills/index.md`
  registry, **vs** a typed `skill-proposal` JSON contract. **Rec: keep simple
  (prose + registry) for v1.**

## Changes checklist (build order)
1. `templates/skill.template.md` + `.gogo/skills/index.md` registry scaffold
   (a `templates/skills-index.template.md`).
2. `skills/gogo-skills/SKILL.md` — the operating manual (both modes, budget,
   discover/propose/extract, safety/portability).
3. `commands/skills.md` — thin entry point.
4. `skills/gogo-build/SKILL.md` — over-budget nudge in the report step.
5. `skills/gogo/SKILL.md` — on-demand `.gogo/skills/` note in the knowledge-read
   guidance.
6. `coding-rules.md` + `non-functional-requirements.md` (this repo's knowledge)
   **and** `templates/knowledge/{coding-rules,non-functional-requirements}.md` —
   document the 200/400 budget + the determinism rationale, and the **user-gated
   `.claude/skills/` exception** to the `.gogo/`-only write rule (for approved
   standalone skills).
7. `README.md` — `/gogo:skills` in Commands + `.gogo/skills/` (and the
   standalone-skill `.claude/skills/` destination) in "What gets created".
8. `.claude-plugin/plugin.json` — bump version `0.2.0 → 0.3.0`.
9. Sync sweep — grep every enumeration (command lists, file-set lists) and update.
10. `docs/architecture.md` (new) — full "how gogo works" doc (FR12); README gains
    a short "How it works" section linking it.

## Tests (how we'll verify — see `test-strategy.md`)
- **Audit:** a >400-line knowledge file → **OVER**; a ~250-line file → **WARN**;
  a <200-line file → **OK** (correct table + thresholds; `--warn/--max` honored).
- **Discover (true/false positives):** a file with a clearly separable section →
  proposed with sensible slug/description/lines-saved; a tight cohesive file →
  **no** candidates.
- **STOP gate:** proposals presented; **nothing written** until approval.
- **Classify:** a project-/convention-specific section is recommended `knowledge`;
  a self-contained, script-carrying runbook is recommended `standalone`; the user
  can override the kind per candidate.
- **Extraction (knowledge):** approving a `knowledge` candidate creates a valid
  `.gogo/skills/<slug>/SKILL.md` (frontmatter + standalone body), materializes
  `scripts/`/`.env.example` when the section had commands, replaces the parent
  section with a pointer, brings the parent **under budget**, and updates
  `.gogo/skills/index.md`. Re-run is **idempotent** (extracted sections skipped).
- **Extraction (standalone):** approving a `standalone` candidate writes
  `.claude/skills/<slug>/SKILL.md` (the only sanctioned write outside `.gogo/`) and
  it becomes harness-discoverable; declining leaves nothing outside `.gogo/`.
- **Directed mode:** `/gogo:skills "extract the X runbook"` extracts exactly X.
- **Safety/portability:** no write outside `.gogo/` except an approved standalone
  skill's `.claude/skills/` dir; works with no jq/node; `--include <path>` outside
  `.gogo/` is report-only.
- **Integration:** `/gogo:build` prints the over-budget nudge; enumerations + the
  budget rule are in sync; version bumped; the diagram viewer renders.

## Diagrams (intended design)
The `/gogo:skills` runtime flow — modes, classification, the approval gate, and
the kind-routed extract sub-steps. Also at `charts/skills-flow.mmd`; open
`charts/diagrams.html` offline.

```mermaid
flowchart TD
  ARGS{"invocation"}:::gate
  ARGS -->|"\"extract X\" (directed)"| TARGET["resolve the named section"]:::proc
  ARGS -->|"no prompt (audit)"| SCAN["scan .gogo/knowledge/*.md"]:::proc
  SCAN --> MEASURE["measure body lines\nOK<200 · WARN 200-400 · OVER>400"]:::proc
  MEASURE --> ANALYZE["analyze sections in WARN/OVER\n(cohesion · context-locality · standalone-able)"]:::proc
  ANALYZE --> CLASSIFY
  TARGET --> CLASSIFY["classify kind\nknowledge | standalone"]:::proc
  CLASSIFY --> PROPOSE[/"proposals: slug · kind · destination · description · lines saved · scripts/env · stub"/]:::art
  PROPOSE --> GATE{"user approves\n(per candidate)?"}:::gate
  GATE -->|"no / edit"| STOP([stop — nothing written]):::io
  GATE -->|"knowledge"| WK["write .gogo/skills/&lt;slug&gt;/SKILL.md\n+ scripts/ + .env.example"]:::out
  GATE -->|"standalone"| WC["write .claude/skills/&lt;slug&gt;/SKILL.md\n(harness auto-discovers)"]:::out
  WK --> STUB
  WC --> STUB["replace parent section\nwith summary + Load-when pointer"]:::out
  STUB --> INDEX["update .gogo/skills/index.md\n(kind · destination · trigger)"]:::out
  INDEX --> REMEASURE["re-measure parents\n(confirm under budget)"]:::proc
  REMEASURE --> REPORT([report: lines saved · skills created]):::io
  classDef proc fill:#e8ecff,stroke:#7c8bd9,color:#111
  classDef art fill:#fff3d6,stroke:#caa54a,color:#111
  classDef gate fill:#ffe0e6,stroke:#d98aa0,color:#111
  classDef out fill:#e6f5e6,stroke:#86b886,color:#111
  classDef io fill:#eeeeee,stroke:#999999,color:#111
```

## Out of scope
- Auto-extracting without approval (always propose-first; directed-unambiguous is
  the only fast path).
- Auto-promoting a `knowledge` skill to `standalone` (`.claude/skills/`) without
  explicit per-candidate approval.
- Rewriting the pipeline phases or the contract layer.
- Cross-file skill synthesis / de-duplicating knowledge across files (v1 is
  per-file section extraction).
- The reverse op (re-inlining a skill back into knowledge) — possible later.
- A compiled analyzer or any new runtime dependency.
