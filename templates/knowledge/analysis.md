# Analysis

**Purpose:** how to analyze a feature *before* planning it — the ordered procedure
the plan phase (the `gogo-analyst`) follows to ground a plan in *this* project's
real code, and which knowledge to read while doing it. (The plan itself, with the
feature's functional requirements, lives in the feature's `plan.md`, not here.)

<!-- gogo:meta
Mode: owned            # owned — a gogo-authored procedure, not a proxied doc
Source: [ ]            # only if the project documents its own "how we scope work" process
Confidence: low
Generated-by: /gogo:build (scaffold)
-->
> How to analyze a feature before planning. **Code is the source of truth:** when a
> doc/knowledge claim conflicts with the tree, the code wins — verify, then plan.

## Analysis procedure (in order)
1. **Restate the goal** in one line — the outcome, not the implementation. Note the
   acceptance signal ("done when …").
2. **Locate the entry points + touched modules.** Glob/Grep/Read from the goal's
   nouns and verbs to the real files: the entry point(s), the module(s) the change
   flows through, the config/data it reads. List the concrete paths.
3. **Read the tests as the behavior spec.** The existing tests around those paths
   define current behavior — read them before proposing a change; a plan that would
   break them must say so.
4. **Check recent git history** on those paths (`git log --oneline -n 20 -- <path>`,
   `git log -p` on the hot file) — what changed lately and why; avoid re-treading a
   reverted approach.
5. **Identify reuse + blast radius.** What already exists to extend (don't rebuild);
   every caller/consumer the change ripples to; the contracts it must keep stable.
6. **Enumerate edge cases** the code already handles (empty/missing data, errors,
   limits) so the plan preserves them.
7. **Surface risks + unknowns** for the plan — what's ambiguous, what could break,
   what needs a decision. These become the plan's *alternatives* and *decisions*.

## Which knowledge to read (by name, by phase)
Read these while analyzing — they are the **plan phase's** grounding (follow each
file's `Source:` links when a claim needs the detail):

| File | Why — for analysis |
|---|---|
| `analysis.md` (this file) | the procedure above — *how* to analyze this feature |
| `project-knowledge.md` | the architecture, domains, and key decisions the change sits within |
| `tech-stack.md` | how the project builds/runs/tests — the mechanics the plan must respect |
| `non-functional-requirements.md` | the standing bars (perf/security/a11y/reliability) to design **within** |
| `coding-rules.md` | the conventions the implementation will follow — plan reuse and shape accordingly |

`code-review-standards.md`, `testing-tools.md`, and `test-strategy.md` belong to the
later phases — skim them only if the change touches how the project is reviewed or
tested.

## Code is the source of truth
Knowledge files and upstream docs go stale; the code does not. When a knowledge
claim (or a linked doc) conflicts with what the tree actually does, **the code
wins** — verify the claim against the real files, plan against the code, and **note
the drift** so the plan (and later `/gogo:build`) can reconcile it. Never plan
against a doc you have not checked against the code it describes.

## External specs (hook — only if available)
If the feature references an external spec/ticket (a Notion page, a Confluence doc,
a Jira/Linear issue, a design file) **and** a capability to read it is available — a
docs MCP or skill such as `notion`, `confluence`, `atlassian`, `jira`, or similar —
consult it for the authoritative spec, then reconcile it against the code (the code
still wins for *what exists today*). If no such capability is present, **proceed
from the code + the user's description** and record the external reference as an
assumption/unknown for the plan. This is a conditional, **capability-detected** step
— never a hard dependency.

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
