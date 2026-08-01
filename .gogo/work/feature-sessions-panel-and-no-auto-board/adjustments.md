# Adjustments — feature `sessions-panel-and-no-auto-board`

Changes and clarifications you asked for during planning. Each entry is dated and
records what changed in `plan.md` as a result.

<!-- Appended by the orchestrator on every "request changes" round at the plan gate,
     and on every UAT round that loops back to planning. Empty until then. -->

## 2026-08-01 — round 1: gate answers + a scope clarification that adds a third slice

**Your answers to the open decisions.**

| Fork | Answer |
|---|---|
| D2 — where `S` is legal | **C** — board **and** drill. Your correction stands: `updateDrill` *is* parsed by `TestBoardKeyHelpInSync` (it is the guard's second case), so the enumeration concern I raised only applied to option B (`updateActive`), not to C. |
| D3 — attach from the panel | **A — no attach.** Re-assign + close only, per your first message: *"reassign them to different item or close them thats all what was needed"*. |
| D4 — version | **A — 0.33.0.** |
| D1 — `board.py`'s fate | **Retire** — but *inferred*, not picked. See below. |

**The clarification — what you actually need (verbatim):**

> "we do not want to change sessions only on gogo done... we should something inside like
> gogo:session-update and check on which work item it is working now and update gogo board
> to show session on correct item thats all what we need and we should be able to run it at
> any time, sometimes im working on 1 item but meanwhile agents are runnign im creating new
> plans in this sessions, so then there are some plans without sessions assigned, so when I
> end work item currently working on session on board is still assigned to item in changelog
> and meanwhile im starting new plan in this same session, so then new work item on board
> doesnt show it has any sessions assigned becasue this session is still connected with
> changelog item that was done, so we should be able to run at any time gogo:session-update
> and then it should just move session assigment from changellog item to item it is
> currently working on"

**What changed in `plan.md` as a result:**

1. **New slice 3 + FR8-FR10** — a `/gogo:session-update [slug]` slash command + skill
   (markdown, in the plugin), runnable **at any time from inside a claude session**: it
   resolves its own host tmux session, determines the work item it is *currently* driving,
   derives the action with `bindAction`'s rule, and renames its **own** session onto that
   item. No pipeline-state write — the rename is the move.
2. **The problem statement was reframed.** The plan previously treated the retasked
   session as a cockpit-side repair (`R`). Your case is the **drift** case: the item you
   were on ships, you start a new plan *in the same session*, and the binding is left on a
   changelog item while the new item shows no session at all. That is now the plan's
   headline symptom, with the panel and `session-update` as the two complementary fixes.
3. **The "who knows what?" insight is now written down.** 0.32.0's D2 rejected auto-rebinding
   because *"there is no deterministic item-level evidence — `#{session_path}` gives the
   repo, nothing gives the work item"*. That is true **for the cockpit**. The claude
   *inside* the session does know. `/gogo:session-update` is the door that supplies exactly
   the missing evidence, which is why it can be explicit without guessing.
4. **The relationship to 0.32.0's rejected D2=C is cited, not contradicted.** D2=C was
   *automatic* writer-side renaming buried in a phase's prose (rejected because the
   analogous phase-entry write was skipped on all three of its live runs). This is the
   honest form of the same idea: **user-invoked, on demand** — nothing silently breaks when
   it is not run.
5. **D2=C applied** — `S` now opens the panel from the board **and** the drill;
   `drillKeysLine` + `cli/main.go`'s drill block join the sync surfaces (FR6).
6. **D3 applied** — attach is out; the Out-of-scope entry now records it as your explicit
   scope bound rather than a pending decision.
7. **Measured tmux facts added** (verified on this host, tmux 3.7b, in-pane — not inferred):
   a session **can** rename itself from inside (`display-message -p '#S'` → rc=0 rename →
   the new name is live immediately), and a rename onto a **live** duplicate name is
   **refused** with `rc=1` / `duplicate session: <name>` — so the skill must pre-check and
   apply the `-2`/`-3` collision suffix rather than assume success.
8. **New open decision D5** — whether `session-update` may rename a host session that is
   **not** already `gogo-*` (doing so enrolls a previously-unmanaged shell in gogo's
   session lifecycle, where `gogo sweep` can later reap it).
9. Tests gained the drift-case gherkin scenario end to end, plus the no-`$TMUX`,
   collision, and terminal-target cases. Charts gained `charts/session-update.mmd` and the
   `before/` set gained the drift as its own baseline sequence.
