# sessions-panel-and-no-auto-board — quiet `/gogo:done` · the `S` sessions panel · `/gogo:session-update` (0.33.0)

**Shipped 2026-08-01.** Born from one line of user direction the day after 0.32.0
shipped — *an interactive board popping up as a side effect of `/gogo:done` is a bad
idea* — this release makes that a **standing rule** and delivers what was actually
wanted: a place to see live sessions and fix their bindings, plus a way for a session
to fix itself.

**What changed:**

1. **Bare `/gogo:done` ships in chat.** No slug → the four-class status table + one
   multi-select of ready items + the separate-vs-merged gate — never a terminal board.
   The 0.7.0-era python curses kanban (`assets/kanban/board.py`), its intent protocol
   and its `.gogo/resources/kanban/` scratch are **deleted**; `python3` is no longer a
   gogo dependency at all. The rule — **no gogo command opens an interactive TUI as a
   side effect of another action** — is enforced by a source-scan test
   (`TestNoInteractiveBoardInSkills`), not prose.
2. **`S` opens the sessions panel** in the `gogo` cockpit (board + drill): every live
   `gogo-*` session in a live-refreshed list (`bound item | unbound · repo · age
   [· attached]`). `R` re-assigns the focused session onto a picked drivable work item
   — the picker shows the resulting name, and both `R` doors (panel and card) share
   **one** extracted rename core; `K` closes it behind a Cancel-default confirm;
   `esc`/`q` return to wherever `S` was pressed. Zero drivable items → a named,
   rendered refusal. The unbound-session count points at `S`.
3. **`/gogo:session-update [slug]`** — run inside any claude session, any time: the
   session resolves its own tmux name, determines the item it is *actually* driving
   (arg slug → its own conversation → asks, never guesses), and **renames itself** —
   fixing the drift where a shipped item keeps the `●` while the new plan started in
   the same pane shows nothing. Writes no file; a non-`gogo-*` host is renamed with
   the sweep-lifecycle enrolment disclosed. It is the honest, user-invoked form of the
   auto-rename 0.32.0's D2 rejected — legitimate because the claude *inside* the
   session has the item-level evidence the cockpit provably lacks.

**Key decisions (one line each):**
- **D1** — the kanban is retired outright (confirmed explicitly): the `gogo` binary
  has been a strict superset since 0.10.0, and deletion makes the rule testable.
- **D2** — `S` on board + drill; **D3** — re-assign + close only, no attach (the
  user's own scope); **D4** — 0.33.0, never folded into the shipped 0.32.0.
- **D5** — session-update may rename a non-`gogo-*` host, **disclosing** that it
  becomes gogo-managed (sweep may reap it once the item is terminal).

**Review:** approved after 2 rounds — 11 findings (1 major: a stale `docs/index.md`
sentence on the repo's named top-trap surface), all resolved, every fix verified by
mutation testing. **Test:** green, zero findings — full `-race` suite (566 test
functions) plus real-tmux hands-on: the panel end-to-end (open → re-assign proven by
independent `list-sessions` → Cancel-default close), esc-to-opener from both surfaces,
and `/gogo:session-update`'s bash contract executed literally inside a fixture session
(self-resolve, a real collision + suffix loop, the self-rename).

**Diagrams:** as-built flow (the three doors onto the one name binding), the
drift-fix sequence, the panel runtime, and the cockpit mode machine — with the
plan-time before set (including the unrepairable-drift baseline) for side-by-side
compare.

Full audit trail: [.gogo/work/feature-sessions-panel-and-no-auto-board/](../../work/feature-sessions-panel-and-no-auto-board/)
— plan (with the gate-revision round), decisions, review/test rounds, UAT log.
