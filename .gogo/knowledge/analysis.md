# Analysis

**Purpose:** how to analyze a gogo change *before* planning it — the ordered
procedure the plan phase (`gogo-analyst`) follows to ground a plan in this repo's
real code, and which knowledge to read while doing it. (A feature's functional
requirements live in its `plan.md`, not here.)

<!-- gogo:meta
Mode: owned
Source: [ ]
Confidence: medium
Generated-by: /gogo:build
-->
> How to analyze a gogo feature before planning. **Code is the source of truth:**
> the markdown skills + the Go `cli/` are what actually runs — when a doc claim
> conflicts with them, the code wins.

## Analysis procedure (in order)
1. **Restate the goal** in one line + its acceptance signal. gogo work is usually a
   pipeline/skill/knowledge/CLI change — name which surface it touches.
2. **Locate the entry points + touched files.** The chain is thin -> logic:
   `commands/<name>.md` (ultra-thin) -> `skills/<name>/SKILL.md` (all the logic) ->
   `agents/*.md`, `templates/` (scaffolds + `contracts/*.schema.json`), and `cli/`
   (the Go/Bubble Tea cockpit). Glob/Grep to the exact skill/agent/template/Go file
   the change flows through.
3. **Read the "behavior spec."** For a skill, the SKILL.md steps + the JSON Schemas
   in `templates/contracts/` are the contract; for `cli/`, the Go `*_test.go` files
   (`go test ./...`). Read them before proposing a change.
4. **Check recent git history** on those paths (`git log --oneline -n 20 -- <path>`)
   and the `.gogo/changelog/` entries — gogo evolves fast; avoid re-treading a
   reverted approach.
5. **Reuse + blast radius.** Enumeration-sync is the top trap: a change to the phase
   list / feature-folder file set / knowledge-file set / status enum must land in
   **every** place that enumerates it (grep before finishing — see `coding-rules.md`).
   Trace every skill/doc/README/template that names the thing you are changing.
6. **Edge cases + invariants.** Writes stay under `.gogo/`; the core loop degrades
   with no external deps; `${CLAUDE_PLUGIN_ROOT}` for asset paths; the CLI never
   mutates pipeline state. Preserve them.
7. **Risks + unknowns** -> the plan's alternatives/decisions; a behavioural change
   bumps `.claude-plugin/plugin.json` `version`.

## Which knowledge to read (by name, by phase)
Plan-phase grounding (follow each file's `Source:` links for detail):

| File | Why — for analysis |
|---|---|
| `analysis.md` (this file) | the procedure — how to analyze a gogo change |
| `project-knowledge.md` | the architecture: thin commands -> skills -> agents/templates/cli |
| `tech-stack.md` | markdown plugin + Go `cli/`; how to build/run/test each |
| `non-functional-requirements.md` | portability / `.gogo/`-only / determinism-budget bars to design within |
| `coding-rules.md` | authoring conventions + the **enumeration-sync** invariant |

`code-review-standards.md`, `testing-tools.md`, and `test-strategy.md` belong to the
later phases — skim them only if the change touches how gogo is reviewed or tested.

## Code is the source of truth
The skills' prose + the Go `cli/` are what runs; docs and this file can drift. On
any conflict, **verify against `skills/*/SKILL.md`, `templates/contracts/*.json`,
and `cli/`, and let the code win** — then note the drift so the plan (and
`/gogo:build`) reconcile it.

## External specs (hook — only if available)
gogo has no external ticket system; roadmap notes live in this repo's `docs/` and
the session memory. If a change references an external spec and a docs MCP/skill
(`notion`/`confluence`/`atlassian`/…) is available, consult it and reconcile against
the code; otherwise plan from the code + the user's description. This is a
conditional, **capability-detected** step — never a hard dependency.

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
