# Decisions — feature `smart-project-plans`

Each fork lists the options, gogo's recommendation, and the reason. The orchestrator
owns the user's call; record the answer in the Resolutions block at acceptance.

## D1 — How `A` loads/acts-as the analyst + analyzes the sources (the biggest)
**Fork:** how does `A` become an analyst that reads the source repos and auto-selects
targets, while writing only the `~/.gogo/` project plan?

- **(a) NEW `gogo-project-plan` skill — RECOMMENDED.** `A` launches a session that loads
  + follows a new cross-repo analyst skill: it reads the project `.knowledge/` + the
  CLI-seeded **source paths**, analyzes each source repo read-only, decides targets, and
  writes the project plan (`targets:` front-matter + per-source briefs). A skill add,
  isolated to `skills/gogo-project-plan/` — the precedent 0.24.0 FR4 set.
- (b) Enhance `AuthorPlanIntent`'s **prose only** (no skill): tell a plain session to do
  deep multi-repo analysis + auto-targets. Smallest surface, but the output schema the
  FR2 auto-spawn parses is **not guaranteed** (prose is ignorable — the D3-rejected
  failure), and a plain session doing analyst work is what 0.24.0 already rejected.
- (c) Launch the **`gogo-analyst` agent** directly. Agents are Task-invoked subagents, not
  a launchable top-level `claude` verb, and the analyst writes a **source** `plan.md`
  (scaffolding a source `.gogo/work/`), not a project plan with per-source briefs — the
  wrong output shape.

**Recommendation: (a).** Only (a) gives a **strict, parseable output contract**
(`targets:` + per-source brief sections) that FR2 relies on, without the existing
`gogo-plan` skill's wrong shape (a source scaffold). It respects the invariants: the CLI
**launches** the session (skill-writes), the session **reads** many sources by absolute
path but **writes only** the `~/.gogo/` plan file (never a source `.gogo/`). Isolated to
one new skill dir + a seed change; `user-invocable: false` so it adds **no** per-repo
slash-command surface. Justify the new file in the report.
**Sub-note (skill vs command):** keep it a skill loaded via the seed prose (`A` session
uses the Skill tool), NOT a `commands/project-plan.md`. Add a thin command later only if
a direct `/gogo:project-plan` invocation is wanted.
**Decision:** RESOLVED (see Resolutions)

## D2 — Where the auto-selected targets + per-source briefs live in the plan file
**Fork:** how are the analyst's chosen targets + briefs stored so `plans.go` parses them
and the FR2 spawn reads them?

- **(RECOMMENDED) front-matter `targets:` (as today) + per-source briefs as BODY
  sections.** The skill fills the existing `targets: a, b` front-matter (already parsed by
  `parseList`) and writes a `## Source briefs` section with a `### <source-name>`
  subsection per target. Add a pure `plans.BriefFor(p, sourceName) string` extractor. **No
  front-matter/schema change.** `BriefFor` returns "" when absent → the spawn falls back
  to `Description`/`Title` (today's body), so hand-authored / `n` plans still spawn.
- (b) Structured front-matter `briefs:` map. Front-matter is flat `key: value`; multi-line
  prose briefs don't fit the `parseList` shape. Rejected.
- (c) A separate brief file per target under the plan dir. A second store to sync; overkill
  for prose that belongs with the plan. Rejected.

**Recommendation: front-matter `targets:` + body brief sections + `BriefFor`.** Keeps the
plan file human-editable, the parser untouched, and the change backward-compatible.
**Decision:** RESOLVED (see Resolutions)

## D3 — Auto-spawn trigger + mechanics (FR2)
**Fork:** what fires the fan-out, and how does it reuse the spawn seam?

- **(RECOMMENDED) bind it to the ACCEPT step — overload `r` / `gogo plan ready`.** The
  vision is "when the plan is accepted it creates work items." `r` (draft→ready) is the
  natural "I accept this plan" action. On `r`, if the plan has ≥1 target, open a **huh
  confirm** listing the un-spawned targets, then **loop** the existing `planCreateWorkItem`
  mechanics: `PlanIntent(title, BriefFor(target) || body, planID)` per target, append the
  target source's `--skip-acceptance` (via `SkipForSource` + `SkipParams(planSkip,false)`),
  fire **once** through the launcher seam, record a member + `SetStatus(active)` on
  success. Idempotent (skip already-spawned). A **targetless** plan → today's plain
  `MarkReady` (zero launches). A failed launch records no member (REV-005).
- (b) A **dedicated "spawn all" key** (`S`) separate from `r`. Keeps `r` a pure status
  flip, but adds surface and splits "accept" from "spawn," diverging from the vision.
  Documented alternative.
- (c) Fire the spawn **inside the analyst skill** at author time. Moves the spawn to the
  skill's side of the boundary and before the user accepts — breaks the CLI-owns-the-spawn
  invariant and the "accept then spawn" order. Rejected.

**Recommendation: (a).** Reuses the exact fire-once launcher seam the manual `c` uses, in
a loop, gated by an explicit confirm so a multi-launch side effect is never silent.
**Interactions:** after spawn the plan is `active` → the 0.24.0 project-UAT still gates
`done` (`D`); `planAcceptanceSkip` rides the spawned `/gogo:plan` (`--skip-acceptance`),
`uatAcceptanceSkip` applies later at each member's `/gogo:go` (unchanged). `c` (spawn
one) stays the manual fallback.
**Decision:** RESOLVED (see Resolutions)

## D4 — Keep `n` + manual `c`/`+` as the fallback (additive guarantee)
**Fork:** does the smart flow replace or sit beside the manual targeting?

- **(RECOMMENDED) additive — keep all of `n`, `+`, `c` byte-for-byte.** The smart flow is
  only the `A` upgrade (FR1) + the `r` auto-spawn (FR2). A plan with no analyst-chosen
  targets still works: hand-pick with `+`, spawn with `c`; a `n`-drafted or hand-authored
  plan is unchanged. `r` on a targetless plan = today's `MarkReady`.
- (b) Replace manual targeting with the smart flow. Rejected — the invariant is additive;
  a user must still be able to hand-pick.

**Recommendation: additive.** No regression to the manual path; the upgrade is opt-in via
`A` + confirmed at `r`.
**Decision:** RESOLVED (see Resolutions)

## D5 — Session anchoring/trust for a multi-repo-reading analyst session
**Fork:** which root does the analyst session anchor at, and how does it read the other
sources without tripping first-run trust prompts / writing where it must not?

- **(RECOMMENDED) anchor at the FIRST source root (today's `A` behavior); read the other
  sources by absolute path; write only the `~/.gogo/` plan file.** The first source is a
  repo the user already trusts in Claude (the `~/.gogo/` project home is untrusted — a
  first-run trust prompt parks the session there, TEST-013). The launched session runs
  under `--permission-mode auto` (classifier), so read-only cross-repo reads are
  classified-safe; the plan file is edited by its absolute `~/.gogo/` path, so anchoring
  at a source is safe and never touches that source's `.gogo/`. No source yet (rare) →
  fall back to the project home with a note (today's behavior).
- (b) Anchor at the `~/.gogo/` project home. Untrusted → first-run trust prompt parks the
  session (TEST-013). Rejected.

**Recommendation: (a).** Byte-for-byte the anchoring the current `A` already uses; the only
new thing is the session reads MULTIPLE sources, all by absolute path.
**Risk/unknown to watch:** reading a NON-anchor source repo by absolute path may still
prompt for directory approval even under `auto`; the interactive tmux session lets the
user approve, and the skill can fall back to `Bash`-based reads if needed. Flag it; do not
block on it. (Verify in the manual smoke.)
**Decision:** RESOLVED (see Resolutions)

## Phasing / versioning
- Ship FR1 (new skill + smart `A`) **and** FR2 (auto-spawn) together as **0.25.0** — they
  are two halves of one flow (auto-select targets → auto-spawn them) and share the
  plan-file brief contract. Both are CLI + one skill, additive, behind the existing seams.
- Alternative: FR1 as 0.25.0, FR2 as 0.26.0. Acceptable if the user wants a smaller first
  release, but FR1 without FR2 leaves the user still pressing `c` per source (half the
  value).
**Recommendation:** all-in-one **0.25.0**.
**Decision:** RESOLVED (see Resolutions)

## Resolutions (accepted by user <date>)
- **D1 = (a)** new `gogo-project-plan` skill (`user-invocable: false`, loaded by the `A` session; strict `targets:` + per-source-brief output contract).
- **D2 = recommended** front-matter `targets:` + body `## Source briefs`/`### <name>` sections + a pure `plans.BriefFor`; no schema change; absent brief → falls back to body/title.
- **D3 = (a)** auto-spawn on the ACCEPT step (`r` / `gogo plan ready`): huh-confirm the un-spawned targets, loop the fire-once `PlanIntent(BriefFor||body, planID)` + per-source `--skip-acceptance`, record members + `SetStatus(active)`; targetless → plain `MarkReady`; idempotent (skip already-spawned); a failed launch records no member.
- **D4 = additive** — `n`/`+`/`c` byte-for-byte.
- **D5 = (a)** anchor at the first source root (trusted), read other sources by absolute path under `--permission-mode auto`, write only the `~/.gogo/` plan.
- **Phasing = all-in-one 0.25.0** (FR1 + FR2 together).
