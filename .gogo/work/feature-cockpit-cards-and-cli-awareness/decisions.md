# Decisions — feature `cockpit-cards-and-cli-awareness`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

## D1 — where the canonical CLI reference lives (Part A)
- **Phase:** plan
- **Question:** The passive "gogo CLI companion" reference must live in the plugin
  (it is a gogo-universal fact, not per-project knowledge). Should it be a
  dedicated **on-demand skill** or a **plugin doc** the orchestrator points to?
- **Options:**
  - A. **A dedicated on-demand skill `skills/gogo-cli/SKILL.md`** (+ a lean pointer
    in `skills/gogo/SKILL.md`) — its frontmatter description is *itself* the
    discoverability mechanism (an installed Claude sees it in the skill list and
    loads the body only when the CLI is relevant); the deferred **active** half
    later extends the same file → one canonical source.
  - B. **A plugin doc `docs/cli-companion.md`** the orchestrator skill explicitly
    Reads — a plain single source, but NOT auto-surfaced (the orchestrator must
    already know to read it, which is the gap we're closing).
- **gogo recommends:** **A** — the skill's frontmatter is exactly the "installed
  Claude knows the CLI" mechanism, with zero body loaded until relevant, and it
  gives the later active half one canonical source to extend. (`docs/cli-contract.md`
  is the wrong home — it's the consumer file contract, a different purpose.)
- **Status:** RESOLVED → A (user, 2026-07-12; see RESOLVED block at end)

## D2 — drill-in kill/attach as inline keys vs a sub-menu (Part B)
- **Phase:** plan
- **Question:** How does the user trigger kill/attach on a card's session from the
  drill-in?
- **Options:**
  - A. **Inline keys** — `a` attaches (reuse `attachFocused`), `K` kills behind a
    confirm (reuse `launch.KillSession`). `k` stays up-nav, so kill is capital `K`.
    Matches the board's existing key-driven model; lightest change.
  - B. **A session sub-menu** — `enter`/a key opens a "session actions" panel to
    pick attach/kill/peek. More discoverable but heavier and off-pattern.
- **gogo recommends:** **A** — inline `a`/`K` are consistent with the board and the
  drill's other keys; a confirm on `K` covers the (recoverable) session kill. Revisit
  B only if the drill key surface feels cramped.
- **Status:** RESOLVED → A (user, 2026-07-12; see RESOLVED block at end)

## D3 — events timeline inline vs a link (Part B)
- **Phase:** plan
- **Question:** The drill file list already exposes `events (timeline)` as an
  openable row. Should the new detail panel embed the events history inline too?
- **Options:**
  - A. **A compact recent-events tail inline** (last ~5 via `contract.ReadEvents` +
    a small `textfmt` tail) AND keep the full timeline openable via the existing
    artifact row — at-a-glance history without a second renderer.
  - B. **Link only** — the panel just notes "events: open the timeline row"; smallest
    change but no at-a-glance history.
- **gogo recommends:** **A** — a compact inline tail is the better balance; the full
  `textfmt.Timeline` (what `gogo events` renders) stays reachable via the existing row.
- **Status:** RESOLVED → A (user, 2026-07-12; see RESOLVED block at end)

## D4 — how the drill-in reflects the one-owner-lock / live-session state (Part B)
- **Phase:** plan
- **Question:** What sources does the session panel read to convey ownership/liveness?
- **Options:**
  - A. **Registry + live-tmux cross-check** — show tracked sessions (registry) each
    flagged live/stale by exact `SessionMatchesSlug`, plus untracked-live sessions.
    Don't read the lock file. Minimal, and "live" already conveys ownership.
  - B. **Also read `.gogo/resources/cli/locks/<slug>.lock`** to print the current
    owner (PID / host / since) explicitly.
- **gogo recommends:** **A** for the first cut (registry + live-tmux is enough to
  show sessions + live/stale and to target kill/attach); the lock-owner line (B) is
  an easy additive follow-up if you want an explicit owner display.
- **Status:** RESOLVED → A (user, 2026-07-12; see RESOLVED block at end)

## D5 — slice order (both parts)
- **Phase:** plan
- **Question:** Build Part A (CLI-awareness) or Part B (drill-in) first?
- **Options:**
  - A. **A → B** — markdown/skills discoverability win first (low-risk, no Go), then
    the heavier Go/TUI drill-in. A also lands the "when to suggest the CLI" guidance
    B's richer board benefits from.
  - B. **B → A** — ship the visible board improvement first.
- **gogo recommends:** **A → B** — A is the quick, low-risk win and each slice is
  independently shippable; B is the bigger change and lands second.
- **Status:** RESOLVED → A (user, 2026-07-12; see RESOLVED block at end)

---

## RESOLVED (user, 2026-07-12) — plan-acceptance gate

Accepted the two-slice plan **as-is**, taking every recommendation:

- **D1 → A** — the canonical CLI reference is a dedicated on-demand skill `skills/gogo-cli/SKILL.md` (frontmatter = the discoverability mechanism) + a lean pointer in `skills/gogo/SKILL.md`; the later active gogo-cli half extends this one source.
- **D2 → A** — drill-in kill/attach are inline keys (`a` attach, `K` kill behind a confirm; `k` stays up-nav).
- **D3 → A** — a compact recent-events tail inline + the full timeline still openable via the existing artifact row.
- **D4 → A** — the session panel reads the registry + live-tmux cross-check (no lock-file read this cut; lock-owner line is a follow-up).
- **D5 → A** — slice order A → B (CLI-awareness first, then the board drill-in).

`state.md` → `plan-accepted`; `/gogo:go` unlocked to build Slice A → B.

---

## D6 — Slice A hands-on proof (installed Claude surfaces the CLI) — verify or skip
- **Phase:** test (④)
- **Question:** Part A's acceptance signal includes a **hands-on proof** the plan
  explicitly flags as a *user-decision check, never a silent skip*: does an
  **installed Claude**, in a gogo project with the `gogo` binary on PATH, actually
  surface/suggest the CLI (load the `gogo-cli` skill) when asked to manage/view
  work? This is a live skill-selection behaviour of a *separate* Claude session —
  the tester can't assert it deterministically (TEST-002). Every artifact-level
  prerequisite IS verified green: `skills/gogo-cli/SKILL.md` exists with a
  trigger-worded frontmatter description; `skills/gogo/SKILL.md` carries the lean
  `Load when: … → skills/gogo-cli` pointer; the enumeration-sync lint is green +
  non-tautological. Only the live behavioural proof remains.
- **Options:**
  - A. **Skip the hands-on proof for now** (recommended) — accept on the verified
    artifact-level prerequisites; the live proof happens naturally when you next use
    the CLI. Only you may skip a hands-on check. Advances to ⑤ report.
  - B. **You run the check now** — in a gogo project with `gogo` on PATH (e.g.
    `cd cli && go build -o gogo .`), start a FRESH Claude Code session and ask
    "manage my gogo work" / "what's the status of my gogo features"; confirm Claude
    surfaces/suggests the CLI. Report pass → mark verified → ⑤. Report fail → a small
    implement-round wording tweak to the skill frontmatter/pointer, then re-check.
- **gogo recommends:** **A** — the discoverability *mechanism* (the skill frontmatter
  + the lean pointer) is in place and verified; the remaining proof is a live
  behaviour best confirmed in real use, and it blocks nothing structural. Skip now,
  verify opportunistically. (Everything else — Slice B + the Slice A lint — is green.)
- **Status:** RESOLVED → A (user, 2026-07-12) — **skip the hands-on proof for now.**
  The artifact-level prerequisites are verified green; the live proof will be
  confirmed opportunistically in real CLI use. TEST-002 marked user-skipped
  (`wontfix`); ④ is green (TEST-001 fixed + regression-tested, TEST-002 skipped) →
  advance to ⑤ report.
