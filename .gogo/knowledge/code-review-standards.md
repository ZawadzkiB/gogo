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
