# board-session-picker

- **shipped:** 2026-07-15
- **feature:** Per-session attach/kill pickers + changelog live-session dot (TUI cockpit)
- **branch:** main (ships within the in-flight 0.20.0 release)
- **verdict:** review APPROVE (0 blockers/majors/minors) · test green (unit `-race` + live tmux TUI drive)

## What shipped

The `gogo` TUI cockpit now makes lingering pipeline sessions **visible and individually
actionable**. Three surfaces changed, all presentation/interaction-only inside
`cli/internal/tui/` over the same `contract.Repo` - no contract, classifier, skill,
pipeline-state, or launch-package change:

- **Changelog live-session dot (FR-1):** a green `●` now precedes the slug on collapsed
  changelog rows that hold a live session (`✓ ● slug … MM-DD`), so you can see at a glance
  which shipped item is still driving a session. Rune-width math reserves the extra cells so
  the `MM-DD` date stays right-aligned; the dot rides the focus fill on a focused row.
- **Attach picker (FR-2):** attach (board `a` / drill `a`) now branches on the live-session
  count. 0 gives the "no running session" hint, 1 attaches directly (unchanged UX), and ≥2
  opens a `huh` Select of one option per session plus Cancel, so you pick *which* session to
  attach to.
- **Kill picker (FR-3):** kill (drill `K`) splits 1 (the existing single Confirm, unchanged)
  from ≥2 (a picker offering one session, an explicit **"all N sessions"**, and Cancel), so you
  can kill a single stray session or clear them all.

Both pickers bind their choice through the same heap-stable `*formBinding` the existing
Confirm/ship forms use, and resolve sessions via `launch.SessionMatchesSlug` (exact
`gogo-<action>-<slug>` parse, never substring) so a sibling slug is never captured. Single-session
UX is untouched and the collapsed list layout is preserved.

## Key decisions

- **D1 (surface which shipped item has a session):** keep the 0.18.0 collapsed list and add a
  per-row `●` dot, rather than reverting the changelog column to full cards.
- **D2 (choice for attach/kill over ≥2 sessions):** attach = pick one; kill = pick one OR "all N"
  plus Cancel; single-session UX unchanged - the simplest interaction that matches the real need.
- **REV-001 (review nit):** the untested board-origin attach-cancel plus selection-preservation
  branch was closed with a test-only addition - the product code was already correct, so no code
  change.

## Review / test verdict

One review round, verdict **APPROVE** - 0 blockers / 0 majors / 0 minors, 1 nit (REV-001,
agent-fixable, fixed in-context). One test round, **green** at two levels: unit (`gofmt`/`go vet`
clean, `go test -race ./...` all packages `ok`, `--version` gives `0.20.0`) plus a hands-on live
tmux TUI drive confirming the dot, the attach picker, and the kill picker (including a real
single-session kill with the sibling untouched - exact-match attribution held); all fixture
sessions cleaned up.

## Diagrams

One **flow** diagram (`board-session-picker-flow.mmd`) carries the signal - the change is
control-flow branching (attach 0/1/≥2, kill 1 vs ≥2, completion routing through
`binding.selected`) plus the changelog dot. A `before/` set is included, so the viewer renders
the as-is flow beside the as-built flow (compare mode).

## Known limitations

- The FR-1 dot covers single shipped items carrying their own sessions; per-member session
  attribution for **merged** changelog entries (release name not equal to member slug) is a
  separate concern, left out.
- No new session-reaping behaviour - this surfaces and targets existing sessions; it does not
  change `gogo sweep`, kill-at-ship, or the registry.
- Attach has no injectable seam (uses `tea.ExecProcess` directly); the chosen session is asserted
  via the `"attaching <session>"` status line. A full attach seam is deferred.

## Full audit trail

The complete as-built report, review/test rounds, per-file changes table, and decision detail live in
[`.gogo/work/feature-board-session-picker/`](../../work/feature-board-session-picker/):
`report/report.md`, `review-01.md`, `test-01.md`, `decisions.md`, `plan.md`.
