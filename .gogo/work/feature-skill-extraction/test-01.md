# Test — round 01 · feature `skill-extraction`

Verification of `/gogo:skills` and supporting files against `plan.md`
(FR1–FR12, Tests section), `test-strategy.md`, and `testing-tools.md`.
No automated suite exists (markdown plugin). Dogfood method: fixture-based
hands-on exercise of the skill procedure + artifact inspection.

## Approach

gogo is a markdown plugin with no compile step or unit suite. Per `test-strategy.md`,
verification = dogfood the behavior and inspect the produced artifacts.

**Fixture**: a throwaway project in the scratchpad
(`/private/tmp/claude-502/.../scratchpad/skilltest/.gogo/knowledge/`) with four
knowledge files covering all audit bands and the proxy case:

| File | Body lines | Mode | Band |
|---|---|---|---|
| `ops-runbook.md` | 419 | owned | OVER |
| `api-conventions.md` | 206 | owned | WARN |
| `coding-style.md` | 65 | owned | OK |
| `project-knowledge.md` | 17 (proxy — owned body only) | proxy | OK |

No real `.gogo/knowledge/` or `.gogo/skills/` in this repo were touched.

**Method**: walk the `gogo-skills` SKILL.md procedure literally against the fixture;
verify each plan Tests item by a mix of executed shell operations and reasoned
instruction analysis; produce sample extraction artifacts in the scratchpad.

## Results by test item

### 1. Audit / thresholds — PASS (executed)

Line counting via `wc -l` (minus the gogo:meta block) correctly classifies:
- `ops-runbook.md`: 419 body lines → **OVER** ✓
- `api-conventions.md`: 206 body lines → **WARN** ✓
- `coding-style.md`: 65 body lines → **OK** ✓
- `project-knowledge.md`: 17 body lines (proxy — owned summary only, upstream not counted) → **OK** ✓

Threshold overrides verified:
- With `--warn 250 --max 450`: `api-conventions.md` moves WARN→OK; `ops-runbook.md` moves OVER→WARN ✓
- With `--warn 150 --max 300`: bands stay WARN/OVER/OK appropriately ✓

The proxy detection works: `Mode: proxy` is read from the `gogo:meta` block; only the
gogo-owned lines after `-->` are measured; the linked upstream docs are never counted. ✓

### 2. Discover candidates — PASS (executed + reasoned)

H2 section analysis of `ops-runbook.md`:

| Section | Lines | Scripts | Verdict |
|---|---|---|---|
| Overview | 7 | 0 | too small |
| Project configuration conventions | 6 | 0 | too small |
| Team conventions | 8 | 0 | too small |
| **Database migration runbook** | **130** | **4** | **CANDIDATE** |
| Incident response | 17 | 0 | marginal / too small |
| Deployment pipeline | 13 | 0 | too small |
| On-call rotation | 12 | 0 | too small |
| Backup verification procedures | 57 | 2 | CANDIDATE |
| Service health checks | 51 | 1 | CANDIDATE |
| Log management | 24 | 1 | borderline |
| (others) | 6-15 | 0 | too small |

Good extraction candidates: migration runbook (130 lines, 4 bash scripts, 7 env vars),
backup verification (57 lines, 2 scripts), service health checks (51 lines, 1 script).
These are cohesive, context-local, standalone-able, and big enough to matter.

`coding-style.md` (OK file, 65 body lines): sections are 6-25 lines, all interrelated
naming/formatting rules, all needed on every implement/review read. Correctly yields
**no candidates** — no false positives. ✓

`project-knowledge.md` (proxy): only 17 gogo-owned body lines; classified OK. The
upstream docs linked in Source: are never candidates. ✓

### 3. Classify (FR4) — PASS (reasoned)

SKILL Step 3 instructions are clear and unambiguous for the fixture's cases:

- **Database migration runbook** (4 bash scripts, 7 env vars, crisp trigger, reusable
  across any Flyway/RDS project): correctly maps to **standalone** heuristics
  ("crisp trigger, self-contained, carries its own scripts/env, useful beyond this
  project"). ✓
- **Project-specific prose sections** (On-call rotation, Incident response — PagerDuty
  schedule, #ops-alerts channel, team-specific escalation): correctly maps to **knowledge**
  heuristics ("project-/convention-specific, prose-heavy, only meaningful to a gogo phase"). ✓
- Step 3 requires "Recommend a kind with a one-line rationale" — rationale instruction
  present. ✓

### 4. Propose-then-STOP (FR5) — PASS (reasoned)

- `AskUserQuestion` is in `commands/skills.md` `allowed-tools` list. ✓
- SKILL Step 4 says: "Write nothing until the user approves — per candidate. Use
  `AskUserQuestion` for the approve / override-kind / decline gate." Unconditional. ✓
- Step 4's proposal format covers: slug, kind+destination, description, source file›section,
  lines saved, scripts/env bundle, replacement stub. ✓
- Issue TEST-003 raises that the Modes section's "(unless trivially unambiguous)" caveat
  could be misread as allowing the gate to be skipped in directed mode. Step 4 mitigates.

### 5. Extraction (knowledge) — PASS (executed)

Simulated extraction of `Database migration runbook` as a **knowledge** skill to the
fixture's `.gogo/skills/db-migration-runbook/`:

Artifacts produced (in scratchpad fixture):
- `SKILL.md`: valid frontmatter (`name: db-migration-runbook`, `description` engineered
  for on-demand triggering starting with "Use when a task requires..."), standalone body
  with no dangling references, "When this applies" section, "Scripts" section, "Environment"
  table. ✓
- `.env.example`: 7 env vars documented (`DB_INSTANCE_ID`, `AWS_REGION`, `APP_HOST`,
  `ADMIN_API_TOKEN`, `FLYWAY_URL`, `FLYWAY_USER`, `FLYWAY_PASSWORD`). Never real secrets. ✓
- `scripts/01-snapshot.sh`: materialized bash script with `set -euo pipefail`, env var
  references. ✓

Parent replacement stub format:
```
## Database migration runbook

Step-by-step procedure for safely applying Flyway schema migrations in production.
Includes pre-migration checklist, snapshot, scheduler management, rollback.

**Load when:** a task involves applying, running, or planning a production database
migration → `../skills/db-migration-runbook/SKILL.md`
```
Relative path `../skills/<slug>/SKILL.md` is correct for knowledge files in `.gogo/knowledge/`
pointing to `.gogo/skills/`. ✓

Re-measure after extracting migration runbook (130 lines → 5-line stub, net -125 lines):
- `ops-runbook.md`: 419 → ~294 body lines. Still WARN (not yet under budget). ✓
- The SKILL would continue proposing backup verification (57 lines) and service health
  (51 lines) candidates; approving all three yields ~419-233=186 body lines → OK. ✓

`index.md` created from template with registry row:
| `db-migration-runbook` | knowledge | `.gogo/skills/db-migration-runbook/` | trigger | `ops-runbook.md › Database migration runbook` | 125 |
✓

### 6. Extraction (standalone) — PASS (reasoned)

SKILL Step 5 routes a standalone-approved candidate to `.claude/skills/<slug>/SKILL.md`.
SKILL Safety section states: "The **only** sanctioned write outside `.gogo/` is an
**approved** `standalone` candidate's `.claude/skills/<slug>/` dir — never automatic,
always per-candidate." ✓

Declining at Step 4's AskUserQuestion gate: nothing is written (Step 5 is never reached). ✓

The `commands/skills.md` correctly includes `AskUserQuestion` in `allowed-tools` so the
gate can fire. ✓

### 7. Idempotency — PASS (executed)

Detection methods confirmed:
1. Grep for `**Load when:**` in each section — sections with pointer are skipped as
   candidates (already extracted). ✓
2. `.gogo/skills/index.md` registry check — if a slug is already registered, its source
   section is not re-proposed. ✓

Verified: `grep -n "Load when:" <file>` correctly identifies replaced sections.
SKILL Idempotency section: "Re-runs skip already-extracted sections: a section already
replaced by a `**Load when:**` pointer (or already listed in `.gogo/skills/index.md`) is
never a candidate again. Safe to run repeatedly." ✓

### 8. Directed mode — PASS (reasoned)

SKILL Modes section: "`/gogo:skills "<prompt>"` — skip discovery, target exactly the
section the user names; still classify + propose + confirm (unless trivially unambiguous)."
`commands/skills.md` passes `$ARGUMENTS` to the skill. ✓

For `/gogo:skills "extract the migration runbook"`: the agent would identify the
`## Database migration runbook` H2 section without running the discovery scan,
classify it, propose it with the full Step 4 format, and await approval. ✓

Minor ambiguity in "(unless trivially unambiguous)" noted as TEST-003.

### 9. Safety / portability — PASS (executed)

- `commands/skills.md` `allowed-tools`: `Read, Write, Edit, Bash, Glob, Grep, Skill,
  AskUserQuestion` — no Node.js, no `jq`, no `npm`. ✓
- `wc -l` used for counting with "else read and count" fallback — degrades gracefully. ✓
- `${CLAUDE_PLUGIN_ROOT}` used for template references — no hard-coded absolute paths. ✓
- `--include <path>` outside `.gogo/`: SKILL Step 1 adds the path to the audit table
  flagged "report-only (never extracted) — never a candidate, never written." ✓
  (This is the REV-002 fix, confirmed present in SKILL Step 1 and verified against
  `review/issues.json` status=verified.)
- All writes default to `.gogo/`; standalone writes to `.claude/skills/` only after
  explicit per-candidate approval. ✓

### 10. Integration — PASS (executed)

| Integration point | Status |
|---|---|
| `gogo-build` Step 6 over-budget nudge | Present: "for any knowledge file whose gogo-owned body exceeds the warn threshold (200 lines), print `<file> is NNN lines — consider /gogo:skills`" ✓ |
| `templates/skill.template.md` | Well-formed: YAML frontmatter with `name`+`description`, body sections (When this applies, Details, Scripts optional, Environment optional) ✓ |
| `templates/skills-index.template.md` | Well-formed: table with all required columns (Skill, Kind, Destination, Trigger, Source, Lines saved) ✓ |
| `docs/architecture.md` file map | Mostly matches real tree. Two nit findings: TEST-001 (implement/ missing) and TEST-002 (comment misleading). All plugin-side entries verified present. |
| `commands/skills.md` frontmatter | Valid: description, argument-hint, allowed-tools (includes AskUserQuestion) ✓ |
| `plugin.json` version | Bumped 0.2.0 → 0.3.0 ✓ |
| `coding-rules.md` hard-invariant | REV-001 fix verified: "Only ever write under `.gogo/` (one user-gated exception — approved standalone skills; see ## gogo overrides)" ✓ |
| `non-functional-requirements.md` | Budget rule + safety exception documented in gogo overrides ✓ |
| `gogo/SKILL.md` on-demand note (FR11) | Present: "Load a pointed skill only when the task actually touches it — that's the whole point" ✓ |
| `skills/` enumeration in README + architecture.md | `/gogo:skills` and `.gogo/skills/` both documented correctly in README Commands section and "What gets created" ✓ |
| Charts: `skills-flow.mmd` + `manifest.json` | manifest.json valid against `charts-manifest.schema.json`; .mmd file present; diagram matches plan's intended design ✓ |
| `review/issues.json` | All issues verified (REV-001/002 status=verified, 0 open/new) ✓ |

## New tests added

No automated test suite to extend (markdown plugin). The fixture built for this round
is in the scratchpad and serves as a reference for future manual re-runs of these checks.

## Issues

| ID | Severity | Priority | Status | Summary | Fixable? |
|---|---|---|---|---|---|
| TEST-001 | nit | P3 | new | architecture.md file tree missing implement/ dir and per-run result artifacts | fixable |
| TEST-002 | nit | P3 | new | architecture.md .gogo/skills/ comment says "KNOWLEDGE skills" — misleading for index.md which tracks both kinds | fixable |
| TEST-003 | minor | P2 | new | gogo-skills SKILL.md directed mode "(unless trivially unambiguous)" could be misread as skipping AskUserQuestion gate | fixable |
| TEST-004 | nit | P3 | new | Step 2 "big enough to matter" has no concrete minimum line threshold | fixable |

## Verdict

**GREEN** — all 10 plan test items verified; 4 nit/minor findings, all agent-fixable,
none blocking function.

Done-bar check (`test-strategy.md`):
- Changed commands exercised end-to-end on scratch fixture: ✓ (no live plugin install —
  see degradation note below)
- Artifacts conform to their contracts: ✓ (produced sample SKILL.md, .env.example,
  scripts/, index.md; templates valid; review issues.json validated against schema)
- Bad inputs rejected / not propagated: ✓ (proxy detection, idempotency, safety guardrails
  all verified)
- Enumerations in sync: ✓ (commands/, skills/, agents/, templates/ all match architecture.md
  and README)
- Version bumped: ✓ (0.3.0)
- Review clean: ✓ (REV-001/002 verified)

**Degradation note**: The plugin was not installed via `/plugin marketplace add` and
`/reload-plugins` for a live test run — the session environment doesn't support the
plugin marketplace workflow. Verification was fixture-based (creating and inspecting
artifacts the skill would produce) plus SKILL.md instruction analysis. This is the
expected approach per `testing-tools.md`. A live dogfood run against a real project
with `/gogo:skills` would additionally confirm the AskUserQuestion gate fires correctly
in the real harness — manual check recommended.
