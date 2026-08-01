# Decisions — feature `sessions-panel-and-no-auto-board`

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
- **Status:** OPEN        # OPEN | RESOLVED

### RESOLVED (user, <YYYY-MM-DD>)
<the decision, in the user's terms>
-->

## D1 — What happens to `board.py` and the done-board machinery?

- **Phase:** plan
- **Question:** The goal says two things that pull apart: *"do not open this board on
  gogo done"* (unambiguous) and *"lets keep it as separate function"* (ambiguous — it may
  mean *keep the board, just make it deliberate*, or it may simply introduce the very next
  clause, *"lets keep this board under shift + S"*, i.e. the **sessions** panel). So: does
  `assets/kanban/board.py` survive, and if so, reachable from where?
- **Options:**
  - A. **Retire it.** Bare `/gogo:done` becomes the in-chat table + multi-select (the path
    already written as the fallback); delete `board.py`, the `.gogo/resources/kanban/`
    scratch contract, the schema-v2 intent + exit-code prose, and the ~8 doc surfaces that
    describe them. Leanest end state, and it lets FR2.4's **durable guard** exist (a source
    scan that fails if `board.py` / `board-intent` reappears anywhere).
  - B. **Keep it behind an opt-in flag** — `/gogo:done --board`. Most literal reading of
    "keep it as separate function": the board survives, it is simply never a side effect.
    Cost: two cockpits stay alive, the whole intent/exit-code protocol stays documented and
    maintained, `board.py` keeps owing the vendored-asset rules (pure stdlib, pure ASCII,
    `--selftest`, exit-code contract), every doc surface keeps carrying a second board's
    prose, and the guard weakens from a test to a promise.
  - C. **Leave the asset in the tree, unreferenced.** Worst of both — dead code plus
    permanently stale documentation. Listed only to be rejected.
- **gogo recommends:** **A**. The `gogo` binary has been the interactive cockpit since
  0.10.0 and is a strict superset of `board.py`'s keys, in milliseconds, with no relaunch
  loop — two parallel cockpits is exactly the duplication this repo's own rules warn
  against. And **A is the only option where "no gogo command auto-opens a board" becomes a
  test rather than a sentence.**
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-01) — **A, by inference**

> ⚠️ **Flagged for the re-acceptance gate: this was NOT an explicit pick.** Asked to choose
> between retire and keep-behind-a-flag, you answered with a different need entirely — the
> `/gogo:session-update` command — and closed it with *"thats all what we need"*. Read
> together with *"opening this board automatically on a gogo action is bad idea"* and the
> fact that you never defended the kanban itself, the plan takes that as **A (retire)**:
> your stated needs are fully covered by the in-chat ship + the `S` panel + the new command.
>
> **If that inference is wrong, say so at re-acceptance** — B is a small delta from here
> (keep `assets/kanban/board.py`, gate the board on an explicit `--board` flag, and drop
> FR2.4's guard to a prose rule). Nothing else in the plan changes either way.

### CONFIRMED (user, 2026-08-01) — A, explicitly
Asked point-blank at re-acceptance ("retire + delete the python kanban board?"), the
user picked **"Yes, retire it"**. The inference is now an explicit pick.

## D2 — Where is `S` legal?

- **Phase:** plan
- **Question:** The panel is card-independent (it lists host-wide sessions), so it could
  live on every tab. Which handler owns `S`?
- **Options:**
  - A. **The board tab only** (`updateBoard`), like `P`/`K`/`R`.
  - B. **Every tab** (`updateActive`, beside `q`/`?`/`tab`). A key handled there is parsed
    by **neither** `TestBoardKeyHelpInSync` (reads `updateBoard`/`updateDrill`) **nor**
    `TestPlansTabKeyHelpInSync` (reads `updatePlanList`), so it would ship documented by
    nothing.
  - C. **Board + drill** (`updateBoard` **and** `updateDrill`).
- **gogo recommends:** **A** — smallest surface, automatically guarded.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-01)
**C — board and drill.** Your correction is right and my recommendation was
over-cautious: `updateDrill` **is** the guard's second case, so C is covered by
`TestBoardKeyHelpInSync` exactly as A is — the enumeration objection only ever applied to
B. FR3/FR6 now require `S` in both handlers and in **both** help lines
(`boardAllKeysLine`, `drillKeysLine`) plus **both** `cli/main.go` blocks, and the panel
remembers its opener so `esc` returns to the board or the drill accordingly.

## D3 — Does the panel also **attach** (`a`)?

- **Phase:** plan
- **Question:** The panel is the only place an **unbound** session is visible, and the
  board's card-anchored `a` cannot reach one — so without `a` the panel shows sessions it
  cannot open.
- **Options:**
  - A. **No attach** — exactly the scope named in the goal.
  - B. **Add `a`** — one line reusing `attachSession(name)`.
- **gogo recommends:** **B**, narrowly — cheapest possible addition, closes a real gap.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-01)
**A — no attach.** Pointing back at the original wording: *"reassign them to different item
or close them thats all what was needed"*. `R` and `K` only. Recorded in Out-of-scope with
the workarounds (`tmux attach -t <name>`, or `R` first and then the card's own `a`), so it
stays a one-line addition later if the gap ever bites.

## D4 — Version number

- **Phase:** plan
- **Question:** `0.32.0` is **shipped** (changelog entry + `state.md: shipped`) but the
  whole release is still **uncommitted**. Does this take `0.33.0` or fold into `0.32.0`?
- **Options:**
  - A. **0.33.0** — its own line, per 0.32.0's own D7 precedent.
  - B. Fold into **0.32.0** — one commit, but it rewrites a version whose changelog entry
    already exists, and this change **removes** a feature that entry describes.
- **gogo recommends:** **A**.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-01)
A — **0.33.0**.

## D5 — May `/gogo:session-update` rename a host session that is **not** already `gogo-*`?

- **Phase:** plan
- **Question:** Your flow describes a session that is already `gogo-<action>-<slug>` (it
  drifted off a shipped item). But the command can also be run from a **plain** tmux session
  you started yourself. Renaming that one into `gogo-<action>-<slug>` does bind it to the
  work item — and also **enrols it in gogo's session lifecycle**: `gogo sweep` reaps
  `gogo-*` sessions whose owning feature is terminal, and the per-source concurrency cap
  starts counting it. A shell you thought was yours becomes one gogo may later kill.
- **Options:**
  - A. **Rename it, but say so** — one extra line in the report: *"this session is now
    gogo-managed: `gogo sweep` may reap it once `<slug>` is terminal."* Does what the
    command says on the tin; the consequence is disclosed at the moment it is created.
  - B. **Rename only `gogo-*` sessions**; refuse otherwise, naming the reason and pointing
    at the cockpit's `R`. Safest, but it refuses the plausible case of "I started claude in
    my own tmux window and I want the board to see it".
  - C. **Ask** (`AskUserQuestion`) when the host session is not `gogo-*`.
- **gogo recommends:** **A**. The command is explicitly user-invoked, so the intent is not
  in doubt; the repo's Diagnosability bar is about *naming* consequences, not preventing
  deliberate acts. C is defensible if you would rather be asked — it costs one question on
  a path you will rarely hit.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-01)
A — rename any host session, disclosing the enrolment: "this session is now
gogo-managed; `gogo sweep` may reap it once `<slug>` is terminal."
