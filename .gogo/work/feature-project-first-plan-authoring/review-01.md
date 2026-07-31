# Review — round 1 · `project-first-plan-authoring`

**Date:** 2026-07-31 · **Reviewer:** `gogo-reviewer` (fresh context) · **Baseline:** `abb3def` (0.29.0), change UNCOMMITTED on `main`
**Contract:** `review/issues.json` (this file is its rendered snapshot)
**Standards read:** `.gogo/knowledge/code-review-standards.md` · `coding-rules.md` · `non-functional-requirements.md`

## Scope reviewed

`cli/internal/plans/plans.go` · `cli/internal/launch/launch.go` · `cli/internal/tui/{model.go, plans_tab.go, update.go, delete.go, move.go, config_tab.go}` · `cli/plan.go` · `cli/main.go` · `cli/version_test.go` · the three test files · `README.md` · `skills/gogo-cli/SKILL.md` · `docs/cli-contract.md` · `.claude-plugin/plugin.json` · the feature's `charts/`. 946 insertions / 66 deletions across 26 files.

## Gates (re-run by the reviewer, not taken on trust)

| Gate | Result |
|---|---|
| `gofmt -l .` in `cli/` | clean |
| `go vet ./...` | clean |
| `go test -count=1 -race ./...` | green, all 12 packages |

## Plan fidelity

| FR | Verdict |
|---|---|
| FR1 project-first minting (Select first, only when >1 project, pre-seeded, focus switches BEFORE minting, cancel mints nothing) | **met** — `projectSelectField` + `focusChosenProject`; `TestMintLandsInChosenProject` and `TestPlanWithClaudeAnchorFollowsChoice` assert the plan file *and* the session anchor both follow the choice (the FR1.5 trap, asserted directly) |
| FR2 `p` switcher + always-on project header | **met** — `updatePlanList` case `"p"` reuses `switchProject`, not a new mover; `TestPlansTabKeyHelpInSync` (AST-derived) forces the key into the help line; header row asserted by `TestPlansTabPSwitchesInPlace`. See **REV-005** for its row budget |
| FR3 one `gogoKeyMap()` behind `newForm()` at all 12 sites | **met in code** — `grep` confirms zero `huh.NewForm(` outside `newForm`, and `TestGoalMultilineEntry` drives the real huh form. See **REV-002**: the wiring is unpinned |
| FR4 `Plan.Attachments` + parse/render/SetAttachments + submit validation + ATTACHMENTS block | **met** — round-trip, re-save and no-line-when-empty all covered. See **REV-004** / **REV-007** for validation edges |
| FR5 bounded `WithAttachments` decorator at 3 sites, composed BEFORE `FoldToPointer` | **met** — verified the fold still finds `Intent.Body` after decoration and that the preflight measures the decorated command; the double bound (12 entries / 2048 bytes) holds arithmetically and under test. See **REV-003** for the ordering-vs-contract drift |
| FR6 docs/help/README/skill/contract + version 0.30.0 | **met** — `p` is in all four enumerations; `plugin.json`, `cli/main.go` and `version_test.go` all read 0.30.0; `docs/*.md` swept (only `cli-contract.md` enumerates this surface). See **REV-001** for the one doc surface the plan's list omitted |

Out-of-scope items stayed out: no all-projects view (D2), no copying into the store (D3), no `NewCockpit` default change (D1), no `gogo plan new --attach`, no huh v2. `plan-1948afcd` was not consumed or rewritten. No writes outside `~/.gogo/`; URLs are shape-checked and never fetched; the attachments clause stays inside the single trailing argv element.

## Findings — 7 (0 blocker · 2 major · 4 minor · 1 nit) — all open

### REV-001 · major · P1 · new · AGENT-FIXABLE
**The analyst session launched by `A` is given a front-matter contract that omits `attachments:` — it can drop the attachments the CLI just wrote**
`skills/gogo-project-plan/SKILL.md:59-91` · consequence at `cli/internal/tui/plans_tab.go:1124` → `:1157`

`finishPlanWithClaude` writes `attachments:` into the plan file and *then* launches a Claude session on that same file with "read and edit ONLY that one file". That session follows `gogo-project-plan`, whose **Output contract ("keep it exact")** shows the front matter as literally `id / title / status / targets` and protects exactly one key by name ("Keep the front-matter correlation id. Do not remove or change the `id:` line."). Nothing tells the analyst `attachments:` exists or must be preserved, so an LLM normalising the front matter to the shown template silently drops it — and per **D3=A** the path is the *only* record, so the loss is unrecoverable. Code-review-standards check #1: every enumeration that changed must be in sync in **every** place that enumerates it; the plan's FR6 doc-sweep list did not include this skill.

**Fix:** add `attachments:` to the skill's front-matter template; generalise "keep the `id:` line" to "PRESERVE every front-matter key you did not author (`id:`, `created:`, `attachments:`, `members:`) — you own only `title:`, `status:`, `targets:`"; add one line telling the analyst it may read the listed paths/URLs.

### REV-002 · major · P1 · new · AGENT-FIXABLE
**`newForm()` is the single form construction site by convention only — nothing fails if a future `huh.NewForm(` re-appears**
`cli/internal/tui/model.go:149`

**D5=A** chose the 12-site swap over the 2-site fix for exactly one reason: to stop the next `huh.NewText` silently regressing to "enter submits". The producer shipped and all 12 sites call it *today*, but a new form written as `huh.NewForm(huh.NewGroup(...))` compiles, vets and passes the whole suite — the exact shape code-review-standards **#12** names ("pin the wirings, not just the producer — a call site can stop calling the producer and hand-write fresh copy with the whole suite green") and **#11(a)** ("add a test that fails if either site re-inlines it"). The repo already has both cheap precedents: `TestSkillsBashNoUnsafeRm` (source grep) and `TestPlansTabKeyHelpInSync` (AST walk).

**Fix:** a ~15-line `TestNewFormIsTheOnlyFormConstructionSite` in `cli/internal/tui` that scans every non-test `*.go` for `huh.NewForm(` and fails unless the only hit is inside `newForm` in `model.go`, naming the offending file and saying "use `newForm(...)` so `gogoKeyMap`'s Text bindings apply".

### REV-003 · minor · P2 · new · AGENT-FIXABLE
**`WithAttachments` appends prose AFTER `--correlation` / `--skip-acceptance`, contradicting the gogo-plan skill's "always the FINAL token" contract**
`cli/internal/launch/launch.go:308` · applied at `cli/internal/tui/plans_tab.go:537` and `:730` · consumer at `skills/gogo-plan/SKILL.md:42-45, 54-56`

The skill states "the cockpit CLI **always appends it as the FINAL token of the invocation**" and that `--skip-acceptance` "is a **trailing literal**, so a body that merely mentions the words is ordinary prose". After 0.30.0 both sentences are false for any plan with attachments. Verified impact today is limited — the clause contains no lookalike, so "the LAST token" still resolves correctly and the stripped-goal remainder legitimately carries the attachments prose — so this is contract drift, not a live parse break, but it removes the disambiguation guarantee the skill leans on.

**Fix:** either restate the rule as "honor the LAST `--correlation plan-XXXX` / `--skip-acceptance` **token anywhere in the invocation**, which may be followed by a trailing attachments clause", or insert the clause before the trailing params at the two `/gogo:plan` sites. Keep `docs/cli-contract.md`'s 0.30.0 note consistent with whichever is chosen.

### REV-004 · minor · P2 · new · AGENT-FIXABLE
**The comma refusal (D4/FR4.4) checks the RAW line, but the STORED value is the absolutized path**
`cli/internal/tui/plans_tab.go:859` (check) vs `:869-874` (`~`-expand + `filepath.Abs`)

A relative path typed inside a directory whose name contains a comma (or a `~` path under such a home dir) passes the check and is stored as e.g. `attachments: /Users/me/a,b/shot.png`. `render` joins on `", "` and `parseList` splits on `","`, so one attachment silently becomes two bogus entries on the next read — the "silently corrupted" outcome **D4=A** was resolved to prevent ("rare; refused loudly, never silently corrupted").

**Fix:** re-check `strings.Contains(abs, ",")` after normalisation and refuse with a named error; optionally have `plans.SetAttachments`/`AddAttachment` refuse comma-bearing entries too. Cover with a `t.TempDir()` case whose directory name contains a comma.

### REV-005 · minor · P2 · new · AGENT-FIXABLE
**The always-on project header row spends 2 more terminal rows than the plans kanban budgets for**
`cli/internal/tui/plans_tab.go:1258-1263` (header) vs `cli/internal/tui/window.go:38-45` (`colAvail`), used at `plans_tab.go:1297`

`colAvail`'s own doc comment defines the budget as "the whole height minus the board chrome (header, status, footer = **3 rows**) and the column's own head + blank line (2 rows)". The plans tab's chrome just grew from `tabbar + blank + [status] + help` to `tabbar + blank + PROJECT + blank + [status] + help`, but the budget it windows against did not move — so a full column renders ~2 rows past the viewport, and the rows that fall off the bottom are the **status and help lines**, precisely the surfaces standards **#8** and the 0.28.0 diagnosability bar depend on. No test measures rendered height, so nothing catches it.

**Fix:** `planColAvail() = colAvail() - planHeaderRows` used by `renderPlanColumn` and `reflowPlanColumns`, plus a test that sets a known `m.height`, overfills a column with a status set, and asserts `lipgloss.Height(m.View()) <= m.height`.

### REV-006 · minor · P3 · new · AGENT-FIXABLE
**`plans.AddAttachment` ships exported with no caller and no test**
`cli/internal/plans/plans.go:471`

`grep -rn AddAttachment cli/` finds only its own definition. The plan's Changes checklist for `plans.go` lists only `SetAttachments` (`AddAttachment` appears solely in the class diagram), and `gogo plan new --attach` is explicitly out of scope — yet `docs/cli-contract.md`'s 0.30.0 note already documents it as shipped API. Untested exported surface plus scope creep against coding-rules' "keep diffs minimal and scoped to the plan".

**Fix:** drop it (and its mentions in `docs/cli-contract.md` + `charts/class.mmd`), **or** give it the guard `AddTarget` has — append, idempotent re-add, blank no-op, missing-plan error.

### REV-007 · nit · P3 · new · AGENT-FIXABLE
**A DIRECTORY is accepted as an attachment though the refusal message promises "an existing local file"**
`cli/internal/tui/plans_tab.go:878`

A bare `os.Stat` only rejects on error, so a directory passes; the launched session is then told "a local path is a file on this machine" (`launch.go:283`) and will fail to read it. Cosmetic, but it makes a validation message untrue and defers a knowable refusal into the session.

**Fix:** require `st.Mode().IsRegular()` with a distinct "is a directory — attach a file" message; cover with `t.TempDir()` as the input.

## What was checked and found clean

- **Injection safety / single argv.** The attachments clause stays inside `Intent.Command`, which reaches tmux/exec as one argv element; entries are newline-free (line-split) and comma-free (list format). No shell anywhere on the path.
- **No external deps.** URLs are prefix-shape-checked only (`plans_tab.go:862-867`) — never fetched.
- **CLI never mutates pipeline state.** Every new write lands under `~/.gogo/` (`plans.SetAttachments`); no source `.gogo/work/` write was added; every state change still goes through a launched session.
- **TEST-001 heap-stable bindings.** `planProject` / `planAttach` are fields of the shared `*formBinding`; both mint forms bind through the local `b`, never through `&m.binding.…` on a copied Model.
- **Composition with the sibling's `FoldToPointer`.** Decoration leaves `Intent.Body` intact, so the fold's `strings.Replace` still excises the brief, and the preflight measures the decorated command. `TestWithAttachmentsNamesEach` asserts this directly.
- **Bounds.** `MaxAttachmentEntries=12` / `MaxAttachmentClauseBytes=2048` hold on both arms, and the `named == 0` fallback points at the plan file instead of dropping silently.
- **Enumeration sweep.** `p` is in `viewPlansBoard`'s help line, `cli/main.go printHelp`, `README.md` and `skills/gogo-cli/SKILL.md`; no other `docs/*.md` (incl. `index.md`) enumerates this surface. Version 0.30.0 is consistent across `plugin.json`, `main.go` and `version_test.go`.
- **Regression guards intact.** Cancel-mints-nothing (0.25.1) re-asserted with the new field; the 10 non-`Text` forms' existing tests stayed green untouched; `TestPlansTabKeyHelpInSync` still bites.

---

**Verdict: CHANGES** — 2 open majors (REV-001, REV-002); no blockers, no finding needs a user decision (all 7 are AGENT-FIXABLE).
