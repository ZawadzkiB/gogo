# Review — smart-project-plans (round 02)

Fresh-eyes review of the **0.25.1 patch** to the 0.25.0 `A` flow: the plans-tab
`A` ("plan-with-claude") now (1) opens a **goal form** and mints the draft with that
goal as its description (no more blank "Untitled plan"), and (2) after minting,
**launches AND attaches** the user into the live `claude` analyst session (no more
detached, unseen run). The `n` quick-draft also gains an optional description.

**Gates:** `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` green
(cli + every internal package; fresh `-count=1` run of tui/launch/plans). Version
**0.25.1** in `.claude-plugin/plugin.json`, `cli/main.go`, `version_test` pinned.
No command-surface change → enum-sync legitimately untouched. Skill-bash /
no-unsafe-rm guards unaffected (no skill bash changed).

## What I verified holds (the high-risk surface)

- **Mint → launch → attach is single-shot and correctly ordered.** On form submit
  `finishPlanWithClaude` (plans_tab.go:699) mints the draft ONCE via
  `plans.New(project, title, goal)` with the goal as the description, THEN returns a
  single `tea.Cmd` that fires `m.launcher` exactly once and returns a
  `planAuthorLaunchedMsg`. The completion dispatch clears `m.form`/`pendingPlanWithClaude`
  before returning, so no stray message can re-mint or re-fire. `calls == 1` asserted.
- **Attach only on a real session; the session name is the one just created.** The
  handler (update.go:121-134) attaches via `attachSession(msg.session)` where
  `msg.session == res.Session` — the freshly created tmux session — and **only when
  it is non-empty**. `TestPlanWithClaudeSubmitMintsSeedsAndAttaches` asserts the
  `attaching <session>` observable.
- **No-tmux path is safe.** When `res.Session == ""` (backgrounded `claude -p`) the
  handler does NOT attach — it surfaces a headless status and `loadPlans()`; no crash,
  no orphan, and the draft is still minted (`TestPlanWithClaudeNoTmuxHeadless`).
- **Reload-on-detach holds.** `attachSession` returns `launchDoneMsg{"detached …"}`,
  whose handler (update.go:112) calls `m.loadPlans()`, so the board reflects the
  analyst's writes after the user detaches.
- **Cancel / empty goal mints NOTHING.** A whitespace-only goal trims to `""` and is
  treated as cancel (no `plans.New`, no launcher); Esc through `updateForm` clears
  `pendingPlanWithClaude` via `cancelForm`. Both asserted
  (`TestPlanWithClaudeCancelMintsNothing`).
- **Pending state is leak-free.** `pendingPlanWithClaude` is set in
  `startPlanWithClaudeForm`, cleared in `finishPlanWithClaude` (both success and
  empty-goal) and in `cancelForm`, and is included in `formPreservesSelection`.
  `pendingPlan` vs `pendingPlanWithClaude` are mutually exclusive; dispatch order is
  safe.
- **TEST-001 heap-stability.** The goal/title/description fields bind the heap-stable
  `*formBinding` (`planGoal`/`planTitle`/`planDesc`), never `.Value(&m.field)` on the
  value-type Model; `form.Init()` is returned so huh's async protocol starts.
- **`AuthorPlanIntent` stays one injection-safe argv.** The goal is spliced into the
  single `strings.Builder` Command (launch.go:414-455) — still a PLAIN prompt (no
  slash command, no `--correlation` flag), still carries the `gogo-project-plan`
  directive + planPath + `.knowledge/` + each source label→path. The whole prompt is
  the last argv element even with spaces/newlines in the goal
  (`argv[len-1] == Command` asserted; a session name from a non-ASCII title degrades
  safely to `gogo-author-run` via `sanitizeLabel`).
- **Additive / no regression.** `n` still works and now persists + renders a
  description (`TestPlanNewCapturesDescription`; detail view plans_tab.go:991); the
  0.25.0 `r` auto-spawn, `+`/`c`/`D` are untouched; the no-source project still falls
  back to the project-home anchor.
- **Write-scope clean.** Every write is still `plans.New` under `~/.gogo/`; the launch
  is a `claude` session, never a source `.gogo/` write.

## Findings

| id | sev | title | fix |
|---|---|---|---|
| REV-005 | minor | `deriveTitle` (plans_tab.go:772-781) byte-slices the goal — a >50-byte multibyte first line with no late ASCII space yields an **invalid-UTF-8** title (verified: `utf8.ValidString == false`). No panic; cosmetic + editable, but ships mojibake for non-ASCII goals. | AGENT-FIXABLE — rune-aware cut (`[]rune`/`utf8.DecodeLastRune`) + a multibyte test case. |
| REV-006 | minor | No-tmux headless status (update.go:129-132) **drops the background log path** the pre-0.25.1 `A` surfaced; `planAuthorLaunchedMsg` carries only `session` (+ an unused `id`), so a failed/stalled headless run leaves the user no pointer to inspect. | AGENT-FIXABLE — carry `res.LogPath` on the msg and name it in the status; assert it in `TestPlanWithClaudeNoTmuxHeadless`. |
| REV-007 | nit | `planAuthorLaunchedMsg.id` (plans_tab.go:640-643, set at :756) is **never read** by the handler — dead field. | AGENT-FIXABLE — drop it, or repurpose as `logPath` (see REV-006). |
| REV-008 | nit | The no-source fallback (plans_tab.go:742-745) **dropped its anchor note** ("runs in the project home; approve it if Claude prompts"); the task's intent is "fall back … with the note". Moot under tmux (attach shows the prompt), but a no-tmux + no-source background run can silently stall on a trust prompt with no note (and, per REV-006, no log). | AGENT-FIXABLE — re-add the heads-up on the `!atSource` branch. |
| REV-009 | nit | The shipped `updateForm → finishPlanWithClaude` completion dispatch (update.go:428-432) is **not driven by a real huh-completion message** in tests (submit is called directly via `authWithClaude`) — the same REV-004 class, for the `A` path; Esc-cancel IS message-driven. | AGENT-FIXABLE — add a message-driven submit test mirroring `TestPlansTabAcceptSpawnFormMessageDriven`. |

(REV-001..004 from round 01 remain **fixed** — that 0.25.0 auto-spawn code is
untouched by this patch and its regression tests stay green.)

## Verdict

**APPROVE** — no open blockers or majors (0 blocker · 0 major · 2 minor · 3 nit).
The two UAT-critical bugs are correctly fixed and well-tested: `A` mints the plan
with the goal as its description exactly once BEFORE launching, fires the launcher
exactly once, ATTACHES only when a real tmux session name comes back, and on the
no-tmux path correctly does NOT attach (no crash, no orphan) while still minting the
draft and reloading. The five findings are small robustness (UTF-8 truncation),
observability (dropped log path / anchor note), and coverage-parity polish that can
land as follow-ups without blocking the merge.
