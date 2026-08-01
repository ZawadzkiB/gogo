# session-binding-ops — cockpit session-binding ops (0.32.0)

**Shipped 2026-08-01.** The cockpit can now edit the one thing that binds a tmux session
to a work item — the session **name** (`gogo-<action>-<slug>`). Three card-anchored keys
landed on the board and drill: **`P`** starts a `/gogo:plan <slug>` session anchored at
the card's own repo and attaches you in (a live plan session is joined, never
duplicated), **`K`** — promoted from the drill — kills a *chosen* session behind the
confirm / one-all-cancel picker (including a shipped changelog card's lingering pane),
and **`R`** re-assigns a live session onto the item it is really driving: one
exact-target `tmux rename-session`, after which every reader — the card dot, the agent
chip, the building/stalled cues, the per-source concurrency cap, the one-owner lock and
the sweeper — corrects itself on its next tick. The idle status line now **counts
unbound sessions** (live `gogo-*` panes anchored in a board repo that match no card), so
a retasked or title-named session is visible instead of silently lost, and the footer
chips follow the symptom (`[R] adopt` on a stalled card · `[K]` on a shipped card
holding a session · `[P]` on any plannable card).

**Why:** the session name was minted once at launch and could never change, so a human
retasking a live pane (the reported incident) left the board narrating the past — a
shipped card showing `●`, the real work reading `· stalled`, and a live build invisible
to the working-tree-clobber cap.

**Key decisions (one line each):**
- **D2** — re-binding stays *manual* (`R`) with a visible detector; no heuristic
  auto-adopt (prefer degrading to MISSING over WRONG).
- **D3** — the move IS the rename; no registry hand-off (the old tracked leg truthfully
  renders stale).
- **D4** — the moved session's action derives from the *target's* status
  (runnable → `gogo-go-`, else `gogo-plan-`), so the cap keeps counting builds.
- **D6** — a shipped item's pane keeps its truthful name and becomes actionable, never
  auto-renamed into a sweep-killable orphan.
- **REV-003 (review correction)** — a shipped card offers `[K]` only; an `[R]` chip
  there was a guaranteed bounce (the adopt lives on the card the session should drive).
- `/gogo:done` now prints one line naming the pane its ship-reap self-guard spares.

**Review:** clean after 2 rounds — 7 findings (1 major, 2 minor, 4 nits), all fixed; the
first five verified by mutation testing. **Test:** green — full `-race` suite (13
packages, 559 test functions) plus real-tmux hands-on checks of every op (adopt cleared
`· stalled` → `● developer` and restored the cap count; the picker killed exactly the
chosen session; foreign-repo sessions stayed out of the unbound count).

**Diagrams:** as-built flow / sequence / activity (session lifecycle) with the plan-time
before set for side-by-side compare.

Full audit trail: [.gogo/work/feature-session-binding-ops/](../../work/feature-session-binding-ops/)
— plan, decisions, review/test rounds, per-file changes table, UAT log.
