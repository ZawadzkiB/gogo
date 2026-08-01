---
name: gogo-session-update
user-invocable: false
description: >-
  Re-bind THIS claude session's tmux name to the work item it is actually driving —
  runnable at any time via /gogo:session-update [slug]. A session's binding to a work
  item is its tmux name (gogo-<action>-<slug>), minted once at launch and never
  re-derived, so it goes stale the moment attention moves: ship item A, start planning
  item B in the same pane, and the board shows a live session on A's CHANGELOG card
  while B shows none. The cockpit provably cannot know which item a session is driving
  (session_path gives the repo, nothing gives the item — 0.32.0 D2); the claude INSIDE
  the session has the conversation, so this skill supplies exactly that evidence:
  resolve the host session from $TMUX, determine the target item (arg slug → this
  conversation → ask, never guess), derive the action from the target's status (the
  bindAction rule, cited), and rename the session with the exact-target tmux form.
  Writes NOTHING — not even under .gogo/; its entire effect is one tmux rename-session.
  The explicit, user-invoked form of the auto-rename idea 0.32.0 rejected.
---

# gogo-session-update — a session re-binds itself, on demand

Run **inside** any claude session, at any time. One job: rename this session's own
tmux name to `gogo-<action>-<slug>` for the work item it is *now* driving, so every
board reader (the card dot, the agent chip, the cues, the per-source cap, the lock,
the sweeper, the unbound count) corrects itself on its next tick.

**This skill writes NOTHING.** No `.gogo/` file, no registry, no pipeline state. The
rename **is** the move (0.32.0 D3=A): every reader re-derives from the name. The old
card's tracked registry leg then truthfully renders `stale`.

## Inputs / outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (optional) | `$ARGUMENTS` slug | pins the target work item |
| in | `$TMUX` + `tmux display-message -p '#S'` | this session's own name |
| in | `.gogo/work/feature-<slug>/state.md` | the target's status (read-only) |
| out | one `tmux rename-session` | the whole effect — no file written |

## Steps

1. **Resolve the host session.** If `$TMUX` is unset, say so plainly — *"not inside a
   tmux session; nothing to update"* — and **do nothing** (graceful, never an error).
   Otherwise the session's own name is:
   ```bash
   old=$(tmux display-message -p '#S')
   ```
   (Bash inside a claude session can run tmux — the session IS a tmux pane; verified.)

2. **Determine the target work item**, in this precedence — **never guess**
   (the repo's prefer-MISSING-over-WRONG rule):
   - an explicit `$ARGUMENTS` slug **pins** it. Validate `.gogo/work/feature-<slug>/`
     exists; an unknown slug → **STOP**, listing the nearest existing slugs.
   - else infer from **this session's own conversation** — the work item this session
     has been planning or building is usually unambiguous to you (you are the session).
     This is exactly the evidence the cockpit lacks (0.32.0 D2: *"session_path gives
     the repo, nothing gives the work item"*) and the reason this command exists.
   - else — the conversation does not clearly identify one item — **ask** via
     `AskUserQuestion` over the non-terminal work items, newest first.

3. **Validate the target.** `.gogo/work/feature-<slug>/` must exist and its `state.md`
   status must be **non-terminal**: `shipped` / `aborted` / `done` → **refuse**, naming
   the reason (*"<slug> is <status> — nothing left to drive"*). That is the direction
   the drift already went; re-binding onto it would recreate the problem. (Same
   `TerminalStatus` refusal the cockpit's `R` applies.)

4. **Derive the action from the target's status — quote the one rule, never fork it.**
   The rule is `tui/session_ops.go:bindAction` (0.32.0 D4=A), which applies
   `orchestrator.RunnableStatus`:
   - `plan-accepted` / `implementing` / `reviewing` / `testing` → **`go`**
   - anything else (the plan gates, `waiting-for-user`, no status) → **`plan`**

   The distinction is load-bearing: the per-source concurrency cap counts **only**
   `gogo-go-<slug>` sessions, so a runnable target must mint a build name. If
   `bindAction`'s rule ever changes, this step is the one other place to update.

5. **Mint the new name by the documented contract** (`launch.sessionName`):
   `gogo-<action>-<sanitized-slug>` — lowercase, `[^a-z0-9-]+` → `-`, trim `-`, and cap
   the **sanitized slug** (the label — NOT the whole name) at 48 chars
   (`launch.MaxSessionLabel`); when that cut lands past char 24, cut back to the last
   `-` and trim it, else keep the hard cut. For a real work-item slug (already kebab,
   under 48 chars) the transform is a no-op and the name is simply
   `gogo-<action>-<slug>`.

   **Collision check — mandatory, measured:** a rename onto a live duplicate is
   **refused** by tmux (`rc=1`, `duplicate session: <name>`), never merged. Pre-check
   and suffix `-2`, `-3`, … exactly like the launcher does — minting from the **capped
   label** computed above (`$label`, NOT the raw slug, so a >48-char slug can never make
   this skill and `launch.sessionName` mint different names):
   ```bash
   label="${slug:0:48}"   # then, if the cut landed past char 24, cut back to the last '-' and trim it
   new="gogo-${action}-${label}"
   if tmux has-session -t "=${new}" 2>/dev/null && [ "$new" != "$old" ]; then
     n=2; while tmux has-session -t "=${new}-${n}" 2>/dev/null; do n=$((n+1)); done
     new="${new}-${n}"
   fi
   ```

6. **Already correct → a no-op.** If `old` already equals the target name (or is the
   base plus a numeric suffix that attributes to the same slug + action), say so
   plainly and stop — nothing to rename.

7. **Non-`gogo-*` host → rename, but disclose the enrolment (D5=A).** If `old` does
   not start with `gogo-`, this rename **enrols the session in gogo's lifecycle**:
   `gogo sweep` may reap it once `<slug>` is terminal, and the cap starts counting it
   if the name is a build name. Proceed (the command is explicitly user-invoked), but
   print one line: *"this session is now gogo-managed — `gogo sweep` may reap it once
   `<slug>` is terminal."*

8. **Rename with the exact target form** (`-t` is a session target → `=`; the new name
   is a bare NAME, never `=`-prefixed — measured on tmux 3.7b):
   ```bash
   tmux rename-session -t "=${old}" "${new}"
   ```
   On failure, report tmux's own words — never a bare exit status.

9. **Report what changed:** `old → new`, which work item now owns the session, and
   that the board corrects itself on its next ~5-second session tick (dot, agent chip,
   cues, cap, unbound count). If the item the session drifted OFF was a shipped one,
   note its changelog `●` disappears too.

## Hard rules

- **Write nothing.** No `.gogo/` file, no registry move, no lock transfer — the rename
  is the entire move (0.32.0 D3=A).
- **Never guess the target.** Arg → conversation → ask. An ambiguous inference asks.
- **Exact tmux targets only** (`=<name>`); pre-check collisions; degrade gracefully
  with no `$TMUX`.
- **One action rule.** `RunnableStatus` → `go`, else `plan` — cited from `bindAction`,
  never re-invented.
