# Review — round 02 · `project-first-plan-authoring`

**Date:** 2026-07-31 · **Baseline:** `abb3def` (0.29.0), whole change uncommitted on `main`
**Scope:** fix-verification re-review of the 7 round-1 findings + a fresh-eyes sweep of the round-2 fix delta.
**Contract:** `review/issues.json` (this file is its rendered snapshot).

## Gates (re-run by review, not taken on trust)

| Gate | Result |
|---|---|
| `gofmt -l .` (in `cli/`) | clean |
| `go vet ./...` | clean |
| `go test -race ./...` (`-count=1`, uncached) | **green** — 12 packages, `internal/tui` 8.3s |

## Round-1 findings — verification outcome

All seven were marked `fixed`; **all seven verify**. Two were verified by *mutation* in a
throwaway copy of `cli/` rather than by reading (product code was never touched).

| id | severity | title | outcome |
|---|---|---|---|
| REV-001 | major | analyst front-matter contract omits `attachments:` | **verified** |
| REV-002 | major | `newForm()` is the single construction site by convention only | **verified** |
| REV-003 | minor | `WithAttachments` appends after the trailing params | **verified** |
| REV-004 | minor | comma refusal checks the raw line, not the stored path | **verified** |
| REV-005 | minor | project header row overspends the kanban's row budget | **verified** |
| REV-006 | minor | `plans.AddAttachment` exported with no caller | **verified** |
| REV-007 | nit | a DIRECTORY is accepted as an attachment | **verified** |

**REV-001** — `skills/gogo-project-plan/SKILL.md:68` now carries `attachments:` in the
output-contract block; `:90-94` replaces "keep the `id:` line" with a general **PRESERVE
every front-matter key you did not author** rule naming `id:` / `created:` / `attachments:` /
`members:` and stating that the path is the ONLY record; `:95-97` tells the analyst to READ
each entry (local path = open it, URL = a link gogo never fetches). `gogo-plan` re-checked as
the issue asked: it writes only a source `.gogo/work/`, never a `~/.gogo/` plan file, so it
has no plan front matter to preserve.

**REV-002** — `TestNewFormIsTheOnlyFormConstructionSite` added. Verified by mutation:
rewriting `move.go`'s `newForm(huh.NewGroup(` back to `huh.NewForm(huh.NewGroup(` in a scratch
copy compiles and vets clean, and the guard **fails** with
`move.go calls huh.NewForm( directly (1×) — use newForm(...)`. `grep` confirms exactly one
`huh.NewForm(` in non-test source (`model.go:150`, inside `newForm`). A *reachability* gap in
the new guard is filed separately as **REV-009** — it does not reopen this one.

**REV-003** — `skills/gogo-plan/SKILL.md:42-48` / `:58-59` now describe the shipped
composition order exactly: the id is the LAST `--correlation` token **anywhere** in the
invocation, possibly followed by the 0.30.0 attachments clause (prose, part of the brief).
Cross-checked against both `/gogo:plan` sites and against `FoldToPointer`, which substitutes
`in.Body` inside `in.Command` — so an over-budget fold still preserves the appended clause.

**REV-004** — `normalizeAttachment` re-checks `strings.Contains(abs, ",")` **after**
`filepath.Abs` and **before** `os.Stat`, with the named resolved-path message. Both re-entry
routes (relative path under a comma-bearing cwd, `~` under a comma-bearing `$HOME`) funnel
through that one check; the URL branch returns earlier, so the raw check still owns it. The
new `a,b` TempDir case passes uncached under `-race`.

**REV-005** — `planColAvail()` added in `window.go` and used by **both** `reflowPlanColumns`
and `renderPlanColumn`; `grep` shows no plans path still on `colAvail()`. Arithmetic checks
out: plans-tab chrome is tab bar + blank (`view.go:33`) + project header + blank + status +
help = 6, plus the column's own head + blank = 8, and `planColAvail` returns `height-8`
(`height-6` when `m.project` is nil, matching the header-less render). Verified by mutation:
reverting it to `return m.colAvail()` makes `TestPlansBoardFitsTerminalHeight` fail with
*"renders 25 rows into a 24-row terminal"*.

**REV-006** — `grep -rn AddAttachment` over `*.go` / `*.md` / `*.mmd` / `*.json` finds no live
reference: gone from `plans.go`, from the `docs/cli-contract.md` 0.30.0 note, from
`charts/class.mmd` and from the regenerated `charts/layouts.json`. `SetAttachments` keeps its
round-trip / re-save / trim coverage. One straggler remains → **REV-008**.

**REV-007** — `!st.Mode().IsRegular()` returns a distinct message that also hedges correctly
for a socket/fifo; `os.Stat` follows symlinks, so a symlink to a real file still passes.

## New findings this round

### REV-008 · minor · P3 · `new` · AGENT-FIXABLE
**The as-built `charts/manifest.json` note still advertises the `plans.AddAttachment` that REV-006 deleted.**

`charts/manifest.json:4` was rewritten this round to *"As-built (phase 2)"* and still reads
`class = the structures the change added (plans.Plan.Attachments + SetAttachments/AddAttachment, …)`.
It therefore claims a shipped API that exists neither in `cli/internal/plans/plans.go` nor in
the class diagram the same sentence describes. Code-review-standards #1 in miniature, and the
manifest note rides into the report bundle at phase ⑤, so the stale claim outlives this folder.

*Fix:* drop `/AddAttachment` from the `note` — one token. Leave `plan.md`'s class diagram
alone; that is the accepted plan-time intent record and phase ⑤ reconciles it.

### REV-009 · minor · P2 · `new` · AGENT-FIXABLE
**`TestNewFormIsTheOnlyFormConstructionSite` passes VACUOUSLY if its scan finds nothing — and a sibling test in the same file mutates the cwd it depends on.**

The guard scans `os.ReadDir(".")` and reports only from inside the per-file loop
(`plans_tab_test.go:1455-1476`). With zero matching entries the loop never runs and the test
goes green — `test-strategy.md` variant 6 (*"an invariant assertion that is true VACUOUSLY …
pair it with a reachability guard"*). It never asserts it saw `model.go`, or any file. The
precedent REV-002 explicitly named, `TestSkillsBashNoUnsafeRm`, has exactly that guard and
this one dropped it (`cli/skills_lint_test.go:44-46`:
`if len(skills) == 0 { t.Fatal("no skills/*/SKILL.md found — wrong cwd? …") }`).

The dependency is not hypothetical: `TestAttachmentNormalize`, in the **same file** and
running just before it, changes the process cwd with raw `os.Chdir` + a manual `t.Cleanup`
restore (`plans_tab_test.go:1333-1342`) instead of Go 1.25's `t.Chdir` (`go.mod`: `go 1.25.0`),
whose restore is a framework guarantee.

*Verified by two-part mutation* (test-strategy variant 7) in a scratch copy: with the defect
the guard exists to catch reintroduced (`move.go` calling `huh.NewForm(` directly) **and** the
manual restore dropped, `go vet` is clean and **both tests report PASS** — the wiring guard
silently stops guarding. It bites correctly today (same defect, cwd intact → FAIL naming
`move.go`), so nothing is broken now; the guard is one cwd away from being decorative.

*Fix:* (1) add the reachability guard — count the non-test `.go` files scanned and whether
`model.go` was seen, `t.Fatal` when zero/not seen with a "wrong cwd?" message; (2) replace the
`os.Chdir` dance in `TestAttachmentNormalize` with `t.Chdir(commaDir)`.

## Also checked, no finding

- **Plan fidelity.** Nothing unplanned crept in this round; the delta is exactly the seven
  fixes plus their tests. `AddAttachment` — the one round-1 scope creep — is gone.
- **Version.** `.claude-plugin/plugin.json` + `cli/main.go` + `version_test.go` all at `0.30.0`.
- **Write scope.** No new write path outside `~/.gogo/`; attachments are referenced, never
  copied (D3); URLs are shape-checked, never fetched (portability bar holds).
- **Injection safety.** The attachments clause stays inside the single trailing argv element;
  bounded twice (`MaxAttachmentEntries` 12 / `MaxAttachmentClauseBytes` 2048) with a
  points-at-the-plan-file fallback, so it can never be what overflows the 16 KB tmux budget.
- **`docs/cli-contract.md`.** The 0.30.0 note follows the established placement of the
  0.22.0-0.28.0 notes (blockquote paragraphs ahead of the trailing table rows) — not new drift.
- **Doc-sync (FR6.1).** `p` is present in `viewPlansBoard`'s help line, `main.go printHelp`,
  `README.md` and `skills/gogo-cli/SKILL.md`; `TestPlansTabKeyHelpInSync` passes.
- **`charts/layouts.json`** regenerated coherently with `class.mmd` (picked up
  `attachmentsField` / `normalizeAttachment` / `focusChosenProject`, dropped `AddAttachment`).

## Verdict

Open blockers/majors: **0**. Open minors: **2** (REV-008, REV-009) — both agent-fixable, both
batchable, neither blocks the next phase.

**APPROVE**
