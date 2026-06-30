# Review — round 01 · feature `skill-extraction`

Fresh-eyes review of `/gogo:skills` + supporting files against `plan.md`
(FR1–FR12, D1–D4), `code-review-standards.md`, `coding-rules.md`, and
`non-functional-requirements.md`. The living contract is `review/issues.json`;
this is the round-01 snapshot.

## Verdict: APPROVE

No blockers, no majors. Plan fidelity is high (FR1–FR12 all satisfied), the
hard invariants hold, every enumeration is in sync, and the version is bumped.
Two minor consistency/completeness nits remain — both agent-fixable, neither
blocks advancing to test.

## Findings

| id | severity | file:line | finding | suggested fix | tag |
|---|---|---|---|---|---|
| REV-001 | minor | `.gogo/knowledge/coding-rules.md:29-31` | The "Hard invariants (never violate)" bullet still states "Only ever write under `.gogo/`" absolutely, while the sibling `non-functional-requirements.md:22` now carries "(one user-gated exception — see gogo overrides)". The exception is documented in this file's gogo-overrides (57-60), but the top-of-file invariant reads as unconditional — the asymmetry FR10 wanted reconciled. | Mirror NFR's parenthetical in the hard-invariant bullet ("…one user-gated exception — approved standalone skills; see gogo overrides…"). Repo file only; the template scaffold has no such line. | AGENT-FIXABLE |
| REV-002 | minor | `skills/gogo-skills/SKILL.md:35-39, 110` | `--include <path>` (report-only audit) is documented in the command and the Safety bullet, but Step 1 (audit) only globs `.gogo/knowledge/*.md`. An agent following the numbered steps literally never measures the included path, so the advertised flag does nothing. | In Step 1, after the glob, also measure each `--include <path>` (read-only) and show it in the table flagged "report-only — not a candidate". | AGENT-FIXABLE |

## Dimension notes (what was checked)

- **Plan fidelity (FR1–FR12):** all present. FR2 budget bands `<200 / 200-400 / >400`
  + `--warn/--max` are stated identically across command, skill, README,
  coding-rules, NFR, and architecture (no drift). FR4 per-candidate
  `knowledge`/`standalone` classification + FR5 propose-then-STOP (via
  `AskUserQuestion`) + FR7/FR10 destination routing with the user-gated
  `.claude/skills/` exception + FR8 under-budget pointer + FR9 registry + FR11
  build nudge (warn=200) + FR12 `docs/architecture.md` all match the contract.
- **Hard invariants:** `commands/skills.md` is thin (invoke + args; no flow
  logic). `${CLAUDE_PLUGIN_ROOT}` used for both template references; no
  hard-coded absolute paths. The write-rule + user-gated exception is stated
  consistently in NFR / coding-rules-overrides / gogo-skills / architecture
  (the only nit is REV-001's missing cross-link in the coding-rules invariant
  list). ASCII + `①②③④⑤` glyphs only.
- **Budget self-consistency:** `gogo-skills/SKILL.md` is 112 lines (under its own
  200 budget); templates are 26/38 lines. `gogo-build` Step 6 nudge uses the
  warn threshold (200) coherently.
- **Enumeration sync:** `/gogo:skills` + the new file-set are reflected in README
  (Commands + "What gets created"), `docs/architecture.md` (command tree, skill
  tree, project tree), and `skills/gogo/SKILL.md` (knowledge-read note). The
  feature-folder file set (`templates/state.template.md`, `gogo/SKILL.md`
  workspace list) correctly does NOT list `.gogo/skills/` — it is a `.gogo/`
  top-level sibling, not a feature-folder file. No drift found.
- **architecture.md accuracy (FR12):** flow-vs-knowledge split, knowledge-vs-on-
  demand-skills split + rationale, and the file map (10 commands, 10 skills, 4
  agents, 9 knowledge scaffolds, contracts, `.claude/skills/` user-gated) all
  match the actual tree.
- **Frontmatter:** `commands/skills.md` (description/argument-hint/allowed-tools,
  incl. `AskUserQuestion` for the gate) and `gogo-skills/SKILL.md`
  (name/description) are well-formed and match sibling conventions.

## Routing

Clean (no open/new blockers or majors). The two minors can be batched. Advance
to ④ test; optionally fold REV-001/REV-002 into the next implement pass.
