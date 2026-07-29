# plans-tab-launch-diagnostics-and-view - 0.28.0 (2026-07-29)

Launching a session from the cockpit's plans tab **failed at tmux with no explanation**. The
real cause was a hard limit nobody had measured: **tmux refuses a command line over ~16 KB**
(16317 bytes, bisected on tmux 3.7b) with `command too long`, and the plans tab **inlined the
entire plan brief into that command line**. The user's actual spawn was **20128 bytes**. It
surfaced as a bare `exit status 1` because `Launch` ran `exec.Command(...).Run()`, which
discards stderr - so the one sentence that explained the failure was thrown away every time.

0.28.0 makes the launch **self-reporting** and keeps it **launchable**: tmux's real stderr now
reaches the error and the status line, an over-budget brief **folds to an on-disk pointer**
instead of failing, and a typed error naming the byte count is the backstop. Alongside that,
the plans tab gained the **`v` / `w` viewers** the work board already had, and the per-source
concurrency cap became **legible** rather than indistinguishable from a crash.

Review **APPROVE** (2 rounds, 12 findings). Test **PASS** (2 rounds, hands-on in real tmux).

## What changed

- **Diagnosable launches.** A typed `TmuxError{Sub, Args, Stderr, Err}` captures tmux's stderr
  into a bounded buffer, so a failure reads
  `tmux new-session failed: exit status 1: command too long` instead of `exit status 1`.
  Applied to `Launch`, `LaunchPersistent` and `KillSession`.
- **The command line is measured before tmux sees it.** `MaxTmuxCommandBytes` (16317, measured
  and deliberately ~46 bytes conservative) plus `TmuxCommandBytes`; `Launch`/`LaunchPersistent`
  preflight and return a typed `CommandTooLongError` naming the size and the limit.
- **Fold-to-pointer (decision D1=A).** Over budget, the launched command drops the inlined
  brief and carries an absolute pointer instead - *"read your brief at `<planPath>`, section
  `## Source briefs` -> `### <source>`"*. The brief was already a file in `~/.gogo/`, so nothing
  is lost. **Under budget the command is byte-for-byte what it was in 0.27.0** - verified by
  compiling both `launch` packages side by side and diffing produced argv over 51 launches.
- **Session names are bounded and probed exactly.** `sanitizeLabel` caps at 48 chars on a word
  boundary; `has-session` / `kill-session` use tmux's exact-match `-t "=<name>"` and
  `capture-pane` the pane form `-t "=<name>:"`. This closes a real footgun: `kill-session -t
  gogo-plan-foo` provably killed `gogo-plan-foobar-long`, and `capture-pane` on a bare prefix
  read the wrong pane. `new-session -s` deliberately gets **no** `=` - it would become part of
  the name.
- **Plans-tab `v` and `w`.** `v` renders the plan markdown in the same glamour viewer the board
  uses; `w` builds the self-contained offline page under
  `~/.gogo/projects/<name>/.gogo/resources/view/<plan-id>.html`. A nil guard was added to
  `viewDrill`, which dereferenced `m.drill` unguarded - a naive wiring panicked.
- **A legible cap.** Dangling plan targets are now refused **by name before any confirm opens**
  (previously a plan targeting a source of another project showed *"spawn 1 work item(s)"* and
  then launched **zero**). Status messages gained severity - red for failure, amber for
  blocked, dim for success - where previously every message was `Faint(true)` and a cap bounce
  looked identical to a crash. `M` force-moves past the cap without leaving the cockpit.
- **`m` -> Enter now confirms on the plans tab**, matching the work board. The
  **confirm-default convention** is now written down: forward pipeline moves (launch / spawn /
  accept) confirm on Enter; destructive actions (delete, kill) default to Cancel so Enter is
  safe. The asymmetry is the rule.

## Key outcomes

- **The reported bug is fixed at every door.** Review caught that the fold was wired only into
  the TUI while `gogo plan go` / `gogo plan promote` still built **20951 bytes**; both CLI doors
  now fold too.
- **Upgrade is safe for already-running sessions.** The 48-char cap also shortened the *slug*
  side of session attribution, so a session minted by 0.27.0 with a long label stopped matching
  under 0.28.0 - losing its `●` dot, its `a` attach, and **its place in the cap count**, which
  would have let a second build clobber a shared working tree. Matching was widened read-side
  (minting stays bounded) and verified live against the two real sessions on the host.
- **Three shipped wirings had tests that could not fail.** Reverting them left the whole suite
  green. Found by review, fixed, and pinned by a mutation sweep.

## Decisions (one-liners)

- **D1 = A** - an over-budget brief folds to an on-disk pointer; the typed hard error is only
  the backstop for when even the pointer form will not fit.
- **D2** - a plan's `w` page is written under `~/.gogo/projects/<name>/`, never a source repo.
- **D3** - the plan viewer returns via a flag mirroring the existing `m.peeking` pattern.
- **D4** - session attribution improved now (all four actions), full plan-spawn attribution
  deferred: the CLI cannot know the slug the analyst will derive.
- **D5** - the cross-repo same-slug cap over-count is **reproduced and deliberately deferred**.
- **D6** - shipped as one release rather than three slices.

## Corrections made during the run

Recorded rather than hidden, because two of them contradicted the accepted plan:

- **A2** - FR1.5 as accepted said `capture-pane` takes the bare `-t "=<name>"` session form. It
  does not; that fails with `can't find pane` and **would have broken every log peek**. Shipped
  as the pane form `-t "=<name>:"`.
- **A3** - the fold was wired at `cli/plan.go`, a file the plan's Changes checklist never
  listed. An orchestrator scope call, taken so the CLI door was not left broken.
- **A4** - read-side session back-compat for pre-0.28.0 session names.
- **A5** - the confirm-default convention, per the user's call.

Three of the briefing's own assumptions were **wrong** and are recorded as such: session-name
length was not the cause (names to 2010 chars launch fine), no TUI-key enum-sync test existed
(one was added), and tmux does not shell out - argv reaches it verbatim.

## Known limitations (carried forward)

- **Cross-repo same-slug cap over-count** (`orchestrator/cap.go`) - two repos with an
  identically-named slug can over-count each other. Reproduced, deferred by D5.
- **Pre-0.28.0 sessions rely on the read-side widening** for attribution. Self-heals on
  relaunch.
- **Plan-spawned session attribution is partial** - a session is named for the plan title while
  the analyst derives its own slug, so `SessionMatchesSlug` can miss it (D4).
- **`state.md` still narrates the past, not the present** - it is written at each phase's exit,
  so a work item mid-build reads as its previous state. Not in this item's scope; planned
  separately as `feature-plan-readiness-gate`.

## Method note worth keeping

Review found three tests that passed with and without their fix. Two rules came out of fixing
that, and both are now in `.gogo/knowledge/test-strategy.md`:

1. **Compile-check every mutation before running the suite.** A mutation that does not build
   gets miscounted as a passing guard.
2. **A mutation can compile, be valid, and still not exercise the assertion.** Removing one
   member's correlation left a 1-of-2 tally unchanged, so the test read green. Following that
   thread showed two other fixtures were refusing for the *wrong reason* - "member not found"
   rather than "member unshipped".

Final sweep: **24 mutations, compile-checked, all fail.** Gates green (`gofmt`, `go vet`,
`go test -race ./...`, 13 packages). The user's two live tmux sessions were untouched
throughout.

Full audit trail: [.gogo/work/feature-plans-tab-launch-diagnostics-and-view/](../../work/feature-plans-tab-launch-diagnostics-and-view/)
(plan · decisions · adjustments A1-A5 · 2 review rounds · 2 test rounds · report bundle).
