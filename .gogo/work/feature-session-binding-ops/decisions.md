# Decisions — feature `session-binding-ops`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

<!-- Template for each decision — copy and fill:

## D1 — <short title>
- **Phase:** <plan | implement | review | test>
- **Question:** <the fork, stated plainly>
- **Options:**
  - A. <option> — <trade-off>
  - B. <option> — <trade-off>
- **gogo recommends:** <A / B> — <one-line why>
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — card-only `P`; a brand-new item stays `gogo plan <new-slug> --attach`. C (a
`--slug` param on `/gogo:plan`) is noted as the follow-up that kills the
unbound-at-birth class.        # OPEN | RESOLVED

### RESOLVED (user, <YYYY-MM-DD>)
<the decision, in the user's terms>
-->

## D1 — Does `P` also mint a BRAND-NEW work item from a typed goal?
- **Phase:** plan
- **Question:** FR1's `P` plans the **focused card**. Should the board also offer
  "new item from a goal" (the plans-tab `A` shape: a goal form → `claude "/gogo:plan
  <goal>"`)? It is arguably the incident's root incentive — the user retasked an open
  pane because starting a fresh plan from the cockpit is not possible on a lone repo.
- **Options:**
  - A. **No** — `P` is card-only. A new item is `gogo plan <new-slug> --attach` from the
    shell (already works, already attachable). Smallest surface.
  - B. **Yes** — a goal form on the board. But the session name would come from the goal
    LABEL while the analyst derives its own slug, so the new item is born **unbound** and
    needs an `R` to fix — i.e. B needs FR3 anyway, and re-creates the plans-tab mismatch
    on the work board.
  - C. Yes **plus** a new `--slug <kebab>` param on `/gogo:plan` (skill change, precedent:
    `--correlation`, `--skip-acceptance`) so the cockpit pins the slug and the session is
    bound from birth — and the plans-tab spawn could reuse it later.
- **gogo recommends:** **A** now, **C** as the follow-up that would kill the
  unbound-at-birth class entirely (it changes a skill contract, so it deserves its own
  item rather than riding along).
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — card-only `P`; a brand-new item stays `gogo plan <new-slug> --attach`. C (a
`--slug` param on `/gogo:plan`) is noted as the follow-up that kills the
unbound-at-birth class.

## D2 — Should re-binding ever be AUTOMATIC?
- **Phase:** plan
- **Question:** When the claude inside a session starts working on a different item,
  should the cockpit re-bind by itself?
- **Options:**
  - A. **Manual only**, plus a *detector*: the board counts sessions bound to nothing and
    the `· stalled` card's footer points at `[R] adopt`.
  - B. **Heuristic auto-adopt** — e.g. exactly one `· stalled` card and exactly one
    unbound session in that root → re-bind silently.
  - C. **Writer-side**: teach `/gogo:plan` / `/gogo:go` to rename their own host session
    at entry when `$TMUX` names a `gogo-*` session.
- **gogo recommends:** **A**. There is **no deterministic item-level evidence** —
  `#{session_path}` gives the repo, nothing gives the work item — so B guesses, and this
  repo's NFR is explicit: *prefer degrading to MISSING over WRONG* (a card showing
  plausible-looking wrong data is unrecoverable; a visibly-missing one is not). C is
  prose enforcement, and `coding-rules.md` records that the phase-entry write it would
  imitate was **skipped on all three** of its live runs — fine as a future *extra*, never
  as the mechanism.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — manual `R` only, plus the visible detector (unbound count + `[R] adopt` chip).
No heuristic auto-rebind.

## D3 — How far does a "move" go?
- **Phase:** plan
- **Question:** Besides renaming the tmux session, should `R` also move the CLI-owned
  bookkeeping (`.gogo/resources/cli/sessions/<slug>.json`, `locks/<slug>.lock`)?
- **Options:**
  - A. **Rename only.** Every reader derives from the name, so the rename alone is the
    whole correction. The old card's tracked registry leg then renders `stale` — which is
    true: from its point of view the session is gone.
  - B. Rename **+ registry hand-off** — mark the old leg (a new `moved` status) and seed
    the target's leg with the new tmux name (UUID stays empty: the claude session uuid
    belongs to a human-driven pane, and `--resume`ing a live session would fork it).
  - C. B **+ lock transfer.**
- **gogo recommends:** **A** — the smallest change that fully works. The lock needs
  nothing: after the rename the old slug has no live matching session (so a stale lock is
  reclaimable) and the new slug is protected by the *untracked-owner* branch of
  `orchestrator.Acquire`, which already refuses a second `gogo go` while a live
  `gogo-*-<slug>` exists.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — the move is the rename, nothing else; the old registry leg truthfully renders
`stale`.

## D4 — Which ACTION does a moved session take?
- **Phase:** plan
- **Question:** `gogo-<action>-<slug>` — after a move, is the action preserved or
  re-derived? It matters: the concurrency cap counts **only** `gogo-go-<slug>`
  (`orchestrator/cap.go` `liveBuildSession`), and the agent chip reads from it.
- **Options:**
  - A. **Derive from the target's status** — `RunnableStatus` → `go`, else `plan`.
  - B. **Preserve** the session's current action component.
  - C. Ask the user (a second Select: build / authoring).
- **gogo recommends:** **A**, and show the resulting name in the picker. The incident's
  surviving pane is typically a `gogo-done-<A>` (the ship's own host): preserving that
  action would re-bind it as `gogo-done-<B>`, which the cap does **not** count — leaving
  a real build invisible to the guard that stops two claudes editing one working tree.
  Deriving from the target keeps the cap honest with one pure, testable rule.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — the action is derived from the target's status (`RunnableStatus` → `go`, else
`plan`), and the picker shows the resulting name.

## D5 — Where do the ops live: card keys, or a sessions screen?
- **Phase:** plan
- **Question:** `P`/`K`/`R` are card-anchored. Should there also be a full-screen session
  inbox (`S`) listing every live `gogo-*` session with attach / kill / move?
- **Options:**
  - A. **Card keys only** (this plan) — the adopt picker already *shows* every live
    session (with its binding, repo and age), so the list exists where it is needed.
  - B. Add the screen — better for orphans across repos, but it is a new TUI mode, and the
    tabs it would naturally join **do not exist on a lone repo**, which is exactly where
    the reported incident happened.
- **gogo recommends:** **A**; revisit B if managing cross-repo orphans becomes a habit
  (the machine currently carries live sessions from two different repos).
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — card-anchored keys only; no sessions screen in this item.

## D6 — What happens to a session on a SHIPPED item?
- **Phase:** plan
- **Question:** Should `/gogo:done` detach the pane it cannot reap (rename it to a
  neutral name) so no shipped card ever shows `●`?
- **Options:**
  - A. **Keep the truthful name**, make it actionable (`K` / `R` from the changelog card)
    and print one line at ship saying the pane was spared (FR5).
  - B. Auto-rename it to a neutral `gogo-free-<n>` at ship.
- **gogo recommends:** **A**. B would make that session an **orphan** under
  `sweep.go`'s own rule ("no owning feature"), so a later bare `gogo sweep` would kill a
  live claude with no confirmation — trading a cosmetic `●` for a data-losing reap.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — keep the truthful name; shipped cards offer `K`/`R`, and `/gogo:done` prints
the spared-pane line. No auto-detach.

## D7 — Version number, alongside the in-flight release
- **Phase:** plan
- **Question:** `feature-notify-only-at-user-gates` is mid-implement and already claims
  **0.31.0** (`plugin.json` + `cli/main.go` `Version`, mirrored by a test). This feature
  is behavioural, so it needs a bump too.
- **Options:**
  - A. **0.32.0** — ship after notify; the two never touch the same version line.
  - B. Fold into **0.31.0** if notify has not shipped when this lands (one release, two
    features) — cheaper for users, but the two branches then race the same two files.
- **gogo recommends:** **A**.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — 0.32.0; ships after the notify feature, never racing its version lines.
