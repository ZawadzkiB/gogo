# Code review standards

**Purpose:** what review checks for in a gogo change.

<!-- gogo:meta
Mode: owned
Source: [ ]
Confidence: high
Generated-by: /gogo:build
-->

## What a gogo review must check
1. **Cross-file consistency.** Every enumeration that changed is in sync across
   `skills/gogo/SKILL.md`, the phase skill(s), the templates, and `README.md`.
   No place still describes the old behaviour. (Grep the old terms.) A doc-sync
   sweep must enumerate **all** of `docs/*.md` — including the `docs/index.md`
   quick-reference table — never just the plan's hand-listed subset (the surface
   REV-001 caught slipping through in 0.8.0).
2. **Version bumped.** `.claude-plugin/plugin.json` `version` advanced for any
   behavioural change.
3. **Portability preserved.** No new hard dependency for the core loop. Optional
   tools still degrade gracefully (silent skip + a note, never an error).
4. **Write-scope safety.** New logic only ever writes under `.gogo/`; never edits
   a proxied upstream file.
5. **Hard gates intact.** Plan-acceptance gate, decision gates, and bounded loops
   (~3 rounds/finding) still hold; `state.md` is kept current at transitions.
6. **Idempotency.** `gogo-build` re-runs still preserve `## gogo overrides` and
   `Mode: owned` files.
7. **Contract clarity (for pipeline changes).** Any artifact that flows between
   phases has a clear shape, and producers/consumers agree on it.
8. **Go TUI: rendered, not just set (0.16.0).** For a `cli/internal/tui` change,
   any user-visible field a handler sets (`m.status`, hints, confirmations) must
   actually be **rendered by the mode's `View()`** — a new panel/mode has to
   surface the status line the way `viewBoard` does. Flag a status/hint that no
   `View()` path renders, and a test that asserts only `Model.status` (not the
   `View()` output) for such a path — that gap shipped a silent no-op once
   (the drill-card status line).
9. **A "terminal" feature can still hold a transiently-live session (0.17.0).**
   "Reaping a terminal feature's session is safe by definition" is **false**: a
   just-shipped (terminal) feature can still hold a live `gogo-done-<slug>` session —
   the one *running* `/gogo:done` mid-ship. So any **ship-time** reap must be BOTH
   (a) **slug-targeted** (`gogo sweep <slug>` / `Sweeper.Only`), so it can't kill a
   *different* feature's concurrent ship, AND (b) **self-guarded** (`Sweeper.Self`,
   from `tmux display-message -p '#S'`), so it can't kill the session it runs in.
   Flag a whole-board sweep invoked from a ship/skill path, or a reaper that trusts
   `TerminalStatus` alone to decide a session is dead (REV-002, `immediate-kill-at-ship`).

10. **External-tool failures must be diagnosable (0.28.0).** Flag any `exec.Command(...)`
    on a user-visible path that leaves `cmd.Stderr` nil, or that returns a bare
    `fmt.Errorf("%w")` around an exit code: the tool's own words are the diagnosis, and
    discarding them shipped `exit status 1` as the only symptom of a 16 KB tmux limit.
    Also flag a **non-exact tmux `-t` target** (`=<name>` for a session target,
    `=<name>:` for `capture-pane`'s pane target) - a prefix target provably killed,
    peeked and attached the *wrong* session - and a size/quota check that does not name
    the measured number and the limit.
11. **A test must assert the PRODUCTION decision, and for the RIGHT reason (0.28.0).**
    Three variants of the same defect shipped in one release, all with green suites:
    (a) the test asserted a **test-local copy** of the callback, so deleting the
    production branch changed nothing (fix: make the decision a package-level function
    both call sites use, and add a test that fails if either site re-inlines it);
    (b) a **wiring** was named in the plan, shipped, and had no test that bites - the
    launch-package function was tested, the TUI call site was not;
    (c) the test passed for a **weaker reason than it claimed** - a fixture reused its
    data home so the "other arm" block re-tested the first arm, and two refusal tests
    refused because the member was *not found* rather than *not shipped*.
    So: flag a test whose comment claims coverage its assertions do not provide, and
    require the assertion to name the **exact** reason (the tally, the specific string),
    not just the outcome. The check is a mutation, compile-checked first - see
    `test-strategy.md`.
12. **A remedy must be as safe as the refusal that recommends it (0.29.0).** Flag any
    user-visible message that names a **host-global destructive** command. 0.29.0's cap
    bounce recommended a bare `gogo sweep`, which judges every `gogo-*` session on the
    machine against *this* repo's feature list - so another source's in-flight build is
    classified "orphan - no owning feature" and killed, unconfirmed. The refusal was
    correct and its advice could destroy work. Require the **targeted** form from a single
    producer, and a guard asserting the targeted string is present **and the bare form
    absent**. Same review pass: flag a user-visible **rule stated in more than one place**
    that is not a single constant (three of four copies had gone stale), and - one level up -
    a constant whose **call sites** are unpinned, so a surface can stop calling the producer
    and hand-write fresh copy with the suite green. The **fourteen** known shapes of "an
    assertion that looks like a check and isn't" are enumerated in `test-strategy.md`; walk
    them before approving a change that adds guards.
13. **`state.md` must be current DURING a phase, not after it - and it takes TWO writers
    (0.29.0).** Check #5 says "`state.md` is kept current at transitions"; that is not enough.
    A phase writes `phase`/`status` as its **first act after validate-in** **and again at its
    exit**. Flag a phase skill that writes occupancy **only at exit** (the line then describes
    work that has already stopped) *and* one that writes it **only at entry** (the entry write
    is LLM prose - skipped on all three of its live runs in the release that added it - so
    without the exit write the line stops advancing at all, which is worse).
    **Do NOT flag the exit write as duplication of the entry write: the redundancy is
    deliberate and load-bearing.** 0.29.0 first removed the exit write as newly-redundant and
    had to restore it by user decision; a reviewer "tidying" it away is exactly how that
    regression comes back. Also flag any **safety** rule that depends on either write: a guard
    that prevents *damage* must key on a deterministic signal (a live session, a file on disk),
    and a writer that can skip must be **detectable** rather than silent.
14. **A test seam must never gate the side effect it exists to pin (REV-010, 0.31.0).**
    `notify.sh`'s dry-run flag was added so the selftest could drive the whole script
    without sending — and the same flag also skipped the latch WRITE, so every
    whole-script test ran with the production write path disabled: deleting the write
    call kept the suite green while resurrecting the shipped bug. When reviewing a
    dry-run/no-op/test-mode seam, ask which statements it disables and whether any of
    them is a wiring the suite claims to cover; prove it with a call-site mutation
    (no-op the call — at least one test must fail). A seam may suppress **deliveries**
    (network, UI, notifications), never the **state transitions** under test.

## Severity guide
- **Blocker** — breaks a hard invariant (writes outside `.gogo/`, implements
  without acceptance, hard-codes a path, adds a required dep, drops a gate).
- **Major** — an enumeration left out of sync; missing version bump; a producer
  output a consumer can't parse.
- **Minor** — wording drift, an example that no longer matches, a missing
  cross-link.
- **Nit** — style/tone.

## gogo overrides
<!-- Preserved across re-runs. -->

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
