# Review round 01 — `projects-sources-plans`

Phase ③ fresh-eyes review of the whole uncommitted rework (projects → sources →
project-scoped plans → correlation-list-in-state.md), shipped as one `0.21.0`.
Reviewed against `plan.md`, `decisions.md` (D1–D9 / L1), and the project standards.

## Gate results (all green)
- `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` **PASS** (all
  packages, incl. `projects`, `plans`, `contract`, `tui`, `launch`, enum-sync +
  no-unsafe-rm guards).
- Version bumped once to `0.21.0` (`.claude-plugin/plugin.json` + `cli/main.go`).

## What I verified holds (the load-bearing invariants)
- **Spawn invariant (CRITICAL) — HELD.** Every spawn path — plan-detail `c`
  (`planCreateWorkItem`), `gogo plan promote` (`planPromote`), and `A`
  (`planWithClaude`) — routes through `launch.PlanIntent` + the injectable
  `launcher` seam anchored at a source root (or the project home for `A`), and
  fires exactly once. The CLI itself writes ONLY `~/.gogo/…` (projects/plans
  stores); no code path writes a source's `.gogo/work/`. `projects.Remove` is
  guarded to stay strictly under `ProjectsDir()` and never deletes a source.
- **Correlation round-trip — HELD.** `parseCorrelationList` handles
  one/many/bare/`[]`/absent → nil-when-absent byte-for-byte; `launch.PlanIntent`
  folds `--correlation` into a SINGLE trailing argv element (test proves
  injection-safety with embedded spaces + newlines); many-to-many chips render.
- **Single-repo fallback — HELD.** `New(root)` → `project == nil`, `global()`
  false: no tab bar, no chips, no project count, no source tags, cap inert, plans
  tab empty. `chooseBoard` four-way is pure/no-TTY and correct.
- **Removed cleanly.** No `modeConfig/modeDrafts/modeEpics`, no C/D/E/e board-key
  handlers, no `StampEpics`/`Feature.Epics`/`contract→epics` import remain (grepped).
- **`gogo plan <slug>` coexistence** works via a small reserved-verb set (one
  edge caveat — REV-004).
- **Migration** is guarded (one-shot), non-destructive, best-effort, and never
  crashes on malformed legacy files.
- **TEST-001 heap-stable `*formBinding`** is respected in the new config-tab and
  plans-tab forms (bindings target the shared heap struct, not the value Model).

## Findings

| id | sev | pri | title | fix |
|----|-----|-----|-------|-----|
| REV-001 | major | P1 | README + gogo-cli SKILL still document the superseded P1–P4 UX (dead C/D/E/e keys, `~/.config/gogo`, failing `draft edit`/`promote --to`, `epic add <repo>:<slug>`) | AGENT-FIXABLE |
| REV-002 | major | P1 | `A` plan-with-claude relies on advisory prose to stop gogo-plan scaffolding a work item — the anti-pattern D3 rejected | NEEDS-USER-DECISION |
| REV-003 | minor | P2 | `gogo epic list` filters status==active, but `epic add` never flips status → a just-linked epic disappears from the list | AGENT-FIXABLE |
| REV-004 | minor | P3 | `gogo plan <slug>` shadows reserved store verbs; the "never ambiguous" comment overstates | AGENT-FIXABLE |
| REV-005 | minor | P2 | Spawn records member + flips plan active BEFORE the launch fires → phantom active member on a failed spawn | AGENT-FIXABLE |
| REV-006 | nit | P3 | Project-board source tag is right-aligned un-truncated → name row can wrap at narrow widths | AGENT-FIXABLE |
| REV-007 | nit | P3 | MigrateLegacy touches `~/.gogo` on a no-op run; `gogo plan -h` shows store help only | AGENT-FIXABLE |

### REV-001 (major) — stale user/agent docs
`printHelp` and `docs/cli-contract.md` were reworked to the tabbed `~/.gogo` model,
but `README.md` (## The gogo CLI + keymap) and `skills/gogo-cli/SKILL.md` were only
_added_ the old P1–P4 narrative as current. They tell users/agents to press board
keys `C`/`D`/`E`/`e`, use a `C` config **screen**, read `~/.config/gogo/` stores,
and run `gogo draft edit`/`gogo draft promote … --to …`/`gogo epic add <id>
<repo>:<slug>` — none of which exist in the shipped tree (the aliases forward to
`cmdPlanStore`, which has no `edit`/`--to`, so those commands exit 2). The enum-sync
test passes because it greps only verb presence, not narrative accuracy. Violates
code-review-standards §1 ("no place still describes the old behaviour"). (The
matching `project-knowledge.md` staleness is explicitly deferred to report ⑤ per
plan.md Context — acknowledged, not raised here.)

### REV-002 (major) — `A` authoring vs the gogo-plan scaffold contract
`planWithClaude` seeds `/gogo:plan "Author the project plan brief in place at
<plans>/<id>.md … do not scaffold a source work item" --correlation plan-XXXX`, but
gogo-plan SKILL Step 1 unconditionally creates `.gogo/work/feature-<slug>/`. The
prose redirect contradicts the skill's own numbered step, so `A` likely produces a
stray work-item scaffold under the project home (correlation orphaned there) and
leaves the plan file empty — the very "prose is ignorable" failure D3 cited to pick
the explicit param. No write-scope breach; the seam-fires-once test can't observe
it. Needs a design call on the intended `A` mechanism.

## Verdict
**CHANGES-REQUESTED** — 0 blockers · 2 majors · 3 minors · 2 nits.

The engine of the rework (stores, correlation round-trip, spawn seam, fallback,
tabs) is correct, well-tested, and holds every hard invariant; the gates are green.
The two majors are (1) user/agent-facing docs that describe removed/failing
behaviour and (2) an `A`-authoring flow whose mechanism contradicts the gogo-plan
skill contract and needs a design decision. Resolve REV-001 (agent) and REV-002
(user decision) before approve; the minors/nits are worth folding in the same pass.
