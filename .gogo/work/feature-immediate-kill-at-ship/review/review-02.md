# Review — `immediate-kill-at-ship` · round 02

Fresh-eyes re-review (phase ③) of the **round-2 delta** for decision **D4 → B**
(targeted ship-reap supersedes the plan's D1=A). Reviewed against `plan.md`,
`decisions.md` (D4→B RESOLVED), `adjustments.md`, `code-review-standards.md`,
`coding-rules.md`, and `non-functional-requirements.md`. Gates re-run locally:
`gofmt -l .` clean · `go vet ./...` clean · `TestSweepTargetedOnlyNamedSlug` ·
`TestSweepSparesSelf` · `TestSweepReapsOrphansAndTerminal` ·
`TestSweepDryRunKillsNothing` · `TestSkillsBashNoUnsafeRm` all PASS.

## Scope reviewed (the D4=B delta)
- `cli/internal/orchestrator/sweep.go` — new `Sweeper.Only []string`; `Sweep()`
  `continue`s on `!matchesOnly(sess)` (after the `Self` self-guard); new
  `matchesOnly` / `inScope`; `inScope` guards in `cleanupTerminalRegistries` +
  `cleanupStaleLocks`.
- `cli/go.go` — `cmdSweep` parses `gogo sweep [--dry-run] [<slug>...]`
  (`validSlug`-guarded per slug), passes `Only`; `sweepHelp` rewritten.
- `cli/internal/orchestrator/orchestrator_test.go` — new
  `TestSweepTargetedOnlyNamedSlug`.
- `skills/gogo-done/SKILL.md` — step 6 reap now targeted (`gogo sweep <member-slug>...`).
- `docs/cli-contract.md` — command-surface line + 0.17.0 block.

Round-1 code re-confirmed still solid: the `Self` self-guard,
`launch.CurrentSession()`, and the FR4 `remain-on-exit` drop — all unchanged and
correct.

## Verified solid (no finding)
- **REV-002 fix genuinely closes the concurrent-ship truncation.** Traced against
  the real tree for feature `x` shipping (`Self=gogo-done-x`, `Only=[x]`) with a
  concurrent terminal `z` whose `gogo-done-z` is live:
  - `matchesOnly("gogo-done-z", ["x"])` uses the **same exact**
    `launch.SessionMatchesSlug` parse (no substring): `gogo-done-z` matches no
    `gogo-<action>-x` base nor a `gogo-go-x-<digits>` collision form → `false` →
    the scan `continue`s and `gogo-done-z` is **never** `Kill`'d.
  - `cleanupTerminalRegistries` / `cleanupStaleLocks` skip `z` because
    `inScope("z")==false`, so `z`'s registry + lock are untouched.
  No `x`-vs-`x-2` / `oauth`-vs-`auth` cross-attribution: the boundary-exact
  `SessionMatchesSlug` (TEST-005) is reused unchanged.
- **Whole-board mode is byte-identical to round 1.** `matchesOnly` and `inScope`
  both short-circuit `len(Only)==0 ⇒ true`, so the added `continue` and the two
  `inScope` guards are never taken when `Only` is empty. Orphans + every terminal
  feature + TTL + full lock/registry cleanup behave exactly as before;
  `TestSweepReapsOrphansAndTerminal` / `TestSweepDryRunKillsNothing` still pass.
- **Self-guard still holds inside the targeted scan.** `sess == sw.Self` is checked
  **before** `matchesOnly`, so the host `gogo-done-x` is spared regardless of mode;
  `TestSweepTargetedOnlyNamedSlug` asserts it alongside the spared `gogo-done-z` and
  orphan.
- **`cmdSweep` arg parsing is safe.** Each non-flag arg is `validSlug`-guarded
  (kebab-case, no `..`/`/`) before joining `Only`, else exit 1; `--dry-run`/`-n`
  and slugs compose in any order; unknown `-…` flags error; `-h`/`--help` prints
  the updated `sweepHelp`. `Only` slugs are only ever **compared** (never
  `filepath.Join`'d into a path), so write-scope stays confined to `.gogo/`.
- **Skill reap line is best-effort + classifier-safe.**
  `command -v gogo … && gogo sweep <member-slug>... >/dev/null 2>&1 || true` — no
  `rm` (guard test green), can never fail a ship, and the merged-ship case passes
  **all** member slugs. Ordering intact: step 5 flips `state.md → shipped` **before**
  the step-6 reap (load-bearing, and the prose states it).
- **REV-001 acceptance is sound and honestly documented.** There is genuinely no
  safe in-scope code fix (self-reaping the host truncates the ship — exactly what
  the self-guard prevents); the limitation is disclosed in
  `docs/cli-contract.md:205-206` + `adjustments.md`, not silently dropped. Kept
  `wontfix`.

## Findings

### REV-001 · minor · P3 · NEEDS-USER-DECISION · status: wontfix (carried)
Shipped card can briefly keep a live "running" badge from its own lingering
`gogo-done-<slug>` host session (the self-guard correctly spares it). Inherent to
self-reaping; accepted works-as-designed (D4→B) and documented. Re-confirmed sound
this round. **No code change.**

### REV-002 · minor · P2 · status: verified (was fixed in round 2)
A ship's sweep could truncate a **different** feature's concurrent `/gogo:done`.
Fixed via D4→B targeted ship-reap (`Sweeper.Only` + `matchesOnly`/`inScope`) and
**verified** this round: a targeted `gogo sweep x` reaps only slug `x`'s sessions
and spares a concurrent terminal `gogo-done-z`, an orphan, and its own host — both
in the session scan and in the lock/registry cleanup. Whole-board mode unchanged.

### REV-003 · minor · P3 · AGENT-FIXABLE · status: new (round 2)
**`gogo sweep` command-surface enumerations not synced with the new `[<slug>...]`
arg.** The D4=B delta added the positional slug argument and updated `cmdSweep`,
`sweepHelp`, and `docs/cli-contract.md:52` + the 0.17.0 block — but two other
curated command-surface enumerations that *do* spell out positional args were left
on the pre-arg form `gogo sweep [--dry-run]`:
- `README.md:405` (the "Scriptable" list — elsewhere shows `gogo go [<slug>] …`),
- `skills/gogo-cli/SKILL.md:50` (the CLI command table).

Trips `coding-rules.md` "Keep enumerations in sync" (README named explicitly) and
`code-review-standards.md` item #1. A genuine round-2 regression (round-1
review-01.md recorded the README enumeration as accurate — it was, before the arg
existed). Additive/non-blocking: the authoritative contract table + `gogo sweep
--help` are already correct, so no reader is led into an error, only an incomplete
usage string. `cli/main.go` `printHelp` intentionally defers full usage to
`--help`, so it is **not** part of this finding.
*Fix:* one-line edit in each — `gogo sweep [--dry-run] [<slug>...]`.

## Verdict

**APPROVE** — no open blockers or majors. The D4→B delta is correct: REV-002 is
genuinely closed (verified), whole-board mode is unchanged, the self-guard and the
FR4 drop still hold, and the required `cli/` gates are green. REV-001 stays an
accepted, documented works-as-designed limitation. The single new finding (REV-003)
is a minor, additive doc-sync miss that does not gate the ship — sync the two
`gogo sweep` usage strings before/at report.
