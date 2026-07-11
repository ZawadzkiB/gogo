# Decisions — feature `unattended-ops-input-signals`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

## D1 — Slice A fix strategy: safe-bash rewrite vs move changelog assembly into the Go CLI
- **Phase:** plan
- **Question:** How do we stop `/gogo:done`'s mechanical file steps from tripping the "dangerous rm" permission classifier?
- **Options:**
  - A. **Guarded scoped-`find` rewrite** of the skill bash (no glob-rm, no bare-variable `rm`; guard `$var` non-empty + under `.gogo/`, then `find <dir> … -delete`). Small, portable, self-contained.
  - B. **Move changelog assembly (the file copy/delete) into the Go CLI** — prompt-free by construction. Bigger; a new CLI write surface into `.gogo/changelog`, and a hard binary dep in a path that must degrade with no external deps.
- **gogo recommends:** A — smallest change that removes both classifier triggers, keeps the skill dependency-free, preserves the exact idempotent behaviour. B splits the LLM-authored synthesis from the mechanical copy awkwardly and adds a mutation surface.
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D2 — Board display: a new "waiting" column vs a per-card cue in the existing column
- **Phase:** plan
- **Question:** Where does the "waiting for input" signal live on the board?
- **Options:**
  - A. **A per-card cue** (marker + accent) on the card in its existing phase column.
  - B. **A dedicated fifth "waiting" column** cards move into.
- **gogo recommends:** A — a fifth column breaks the frozen §3 class→column 1:1 mapping and the four-column width/windowing math, and a waiting item legitimately still belongs to its phase column (a plan awaiting acceptance is still "plan"). A is additive and truthful.
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D3 — Does `awaiting-plan-acceptance` count as waiting-for-input?
- **Phase:** plan
- **Question:** Include the pre-acceptance plan state (which sits in the plan column) in `WaitingForInput()`?
- **Options:**
  - A. **Yes** — it's a genuine user gate (the plan-acceptance gate); flag it like the other two.
  - B. **No** — treat only `waiting-for-user` + `awaiting-uat` as waiting; leave plan-pending as ordinary "plan".
- **gogo recommends:** A — an unaccepted plan is blocked on exactly the user; today it shows no cue at all, which is the gap. It is the third genuine gate in the enumeration.
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D4 — How to render column borders in lipgloss without breaking the width math
- **Phase:** plan
- **Question:** What form do the column separators take?
- **Options:**
  - A. **A 1-cell styled vertical separator** joined between columns in `viewBoard` (re-derive `boardColWidth` for the 3 gutters).
  - B. **A full box/left border per column** via each column's lipgloss style.
  - C. Keep JoinHorizontal but give each column a right-`Border` (all but the last).
- **gogo recommends:** A — least disruption to the width budget and to the per-column windowing + focus highlight; B/C interact with the focus-background fill and the measured card heights. Re-derive the 4-column width from `m.width` minus the separator cells.
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D5 — `gogo status`: a dedicated waiting column vs a marker on the STATUS cell
- **Phase:** plan
- **Question:** How does the status table surface waiting-for-input?
- **Options:**
  - A. **A dedicated compact column** (e.g. a leading `WAIT`/`!` column) — greppable, golden-stable.
  - B. **A marker appended to the STATUS cell** (no new column).
- **gogo recommends:** A — a dedicated column is unambiguous and stable to grep/golden; the STATUS text already carries the raw state, so a separate signal reads cleanly. Costs a `status.golden` regen (expected).
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D6 — Where the Slice-A lint guard lives: a Go test in `cli/` vs a standalone shell `--selftest`
- **Phase:** plan
- **Question:** How is the "no unsafe skill-bash" regression guard run?
- **Options:**
  - A. **A Go test in the existing `cli/` suite** reading `../../skills/*/SKILL.md` — runs in the standard `go test -race ./...` gate, one command, already CI-wired.
  - B. **A standalone shell script** (pure POSIX, `--selftest`, documented exit code) that greps the skills — matches the vendored-executable convention, no path-coupling from `cli/` up to `skills/`.
- **gogo recommends:** A — it lands in the gate the coding-rules already require (`go test -race ./...`), so it can't be forgotten; the `../../skills` read is a small, explicit coupling. (B is a fine fallback if the team prefers to keep `cli/` unaware of `skills/`.)
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D7 — Slice C accept mechanism: a thin launched `/gogo:accept` vs a direct board state-flip
- **Phase:** plan
- **Question:** How does a board "accept plan" action clear the `awaiting-plan-acceptance` gate?
- **Options:**
  - A. **A thin new `/gogo:accept <slug>` command + skill the board LAUNCHES** (new `launch.ActionAccept` → attachable tmux / `claude -p`). The session presents the plan, then records acceptance through **gogo-plan's existing recording** (`state.md` → `plan-accepted`, the `Status: **accepted**` line, `open-decision` cleared, the single-owner `plan-accepted` event). The CLI writes no pipeline state.
  - B. **The board flips `state.md` → `plan-accepted` directly** after a huh confirm (deterministic single-line transition; no new command). Simpler, but **relaxes the "CLI never mutates pipeline state" invariant** (today the CLI's only writes are `.gogo/resources/` + the trash move), skips the plan-eyeball a session gives, and needs the `plan-accepted` event emitted from the CLI — a second owner of a single-owner event.
- **gogo recommends:** A — it keeps the frozen invariant intact (every state change is a delegated launch, exactly like `/gogo:go` and `/gogo:done`), reuses the one acceptance recording, and lets the user eyeball the plan in-session before the flip. B is only viable if the team explicitly amends the invariant. (Sub-choice within A: a dedicated `/gogo:accept` command — recommended, single-responsibility, costs the 13th command + enumeration-sync — over an accept *mode* bolted onto `/gogo:resume`/`/gogo:go`, which muddies those commands' contracts.)
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D8 — Slice C board key: reuse `m` on a plan-pending card vs a dedicated `A`/accept key
- **Phase:** plan
- **Question:** Which board key triggers accept?
- **Options:**
  - A. **Reuse `m`** — teach `move.go attemptAction` that an `awaiting-plan-acceptance` card's legal move is accept (it dead-ends into `/gogo:go` today). No new key.
  - B. **A dedicated `A` (accept) key** — explicit, more discoverable; adds a key + help/README/docs surface.
- **gogo recommends:** A — `m` already means "the legal move for this card," and a plan-pending card's only legal move is accept; routing it there removes the dead end with zero new key surface. B is more discoverable but duplicates a verb `m` should own. (If discoverability wins, `A` is a clean fallback.)
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D9 — Slice C: accept-only vs accept-then-go (chain into `/gogo:go`)
- **Phase:** plan
- **Question:** After recording acceptance, does `/gogo:accept` stop, or run the pipeline?
- **Options:**
  - A. **Accept-only** — record `plan-accepted` and stop; the board's `m`→`/gogo:go` on the now-`plan-accepted` card is the natural second step.
  - B. **Accept-then-go** — chain straight into `/gogo:go` so one keypress accepts + runs.
- **gogo recommends:** A — keeps `/gogo:accept` single-responsibility and mirrors the chat flow (acceptance and `/gogo:go` are distinct steps; the user can still eyeball/decide between them). B is a convenience that couples two gates and removes the pause between "I accept" and "go build it." (B is easy to add later if the two-step proves annoying.)
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

## D10 — Slice C: is a prior `v` view a prerequisite for accept?
- **Phase:** plan
- **Question:** Should the accept action be enabled only after/with viewing the plan?
- **Options:**
  - A. **No prerequisite** — accept is always available on an `awaiting-plan-acceptance` card; the launched `/gogo:accept` session presents the plan itself (the eyeball is built into accept).
  - B. **Require a prior view** — accept only enabled after the card has been `v`-viewed this session.
- **gogo recommends:** A — the acceptance session already shows the plan before recording, so viewing is intrinsic; gating a keypress on hidden per-session view state adds modality the board otherwise avoids. B guards nothing A doesn't already cover.
- **Status:** RESOLVED → A (user, 2026-07-11; see RESOLVED block at end)

---

## RESOLVED (user, 2026-07-11) — plan-acceptance gate

The user accepted the revised 3-slice plan **as-is**, taking the recommended option on every fork:

- **D1 → A** — Slice A = guarded scoped-`find` bash rewrite (not move-to-Go).
- **D2 → A** — board waiting signal = a per-card cue in the existing column (not a 5th column).
- **D3 → A (yes)** — `awaiting-plan-acceptance` counts as waiting-for-input.
- **D4 → A** — column borders = a 1-cell vertical separator with re-derived width.
- **D5 → A** — `gogo status` = a dedicated WAIT column (golden regen).
- **D6 → A** — Slice-A lint guard = a Go test in `cli/` reading `../../skills/*.md`.
- **D7 → A** — Slice C accept = a thin launched `/gogo:accept` (keeps the "CLI never mutates state" invariant; adds the 13th command).
- **D8 → A** — board key = reuse `m` on a plan-pending card.
- **D9 → A** — accept-only (accept pauses; `m`→`/gogo:go` is the second step).
- **D10 → A** — no prior-`v`-view prerequisite (the accept session shows the plan).

`state.md` → `plan-accepted`; `/gogo:go` unlocked to build Slice A → B → C.

---

## D11 — Slice A's live proof: run a real prompt-free `/gogo:done`, or accept the unattended evidence?
- **Phase:** test (resume at ④)
- **Question:** The plan's Slice A acceptance signal is a hands-on `/gogo:done` run that advances through changelog assembly + viewer build with **zero permission prompts**. It needs a live, interactive claude session the tester can't safely spawn unattended (TEST-001). How do we clear it?
- **Options:**
  - A. **Accept the unattended evidence + skip the live run.** The regression lint `TestSkillsBashNoUnsafeRm` (run `-count=1`, green) proves **no** glob-`rm` / bare-variable-`rm` shapes remain — which is exactly what the "dangerous rm" classifier keys on — and the tester independently re-verified the rewritten idiom in an isolated harness (byte-identical filesystem on empty-refusal, idempotency, `before/` whole-clear, FR-A2 cleanup). The real prompt-free `/gogo:done` then gets confirmed **organically** the next time you ship a feature via `/gogo:done` (including this one, after UAT) — if a prompt ever appears you'll see it immediately.
  - B. **You run it live now.** I pause; you run `/gogo:done` on a report-complete feature (scratch or real), watch for prompts through assembly + viewer build, and report back; I re-run test round 2 to mark TEST-001 verified before ⑤.
- **gogo recommends:** A — the classifier-safety is proven structurally (the lint is the durable guard the plan itself named as the CI-runnable proof) and behaviour is harness-verified; the live run is confirmatory and will happen naturally at ship time. B is the stronger proof if you'd rather see it now.
- **Status:** OPEN (awaiting user)

## D12 — Slice C's live proof: run a real board `m`-accept follow-through, or accept the unattended evidence?
- **Phase:** test (resume at ④)
- **Question:** The plan's Slice C acceptance signal is a hands-on board accept: `m` on an `awaiting-plan-acceptance` card launches `/gogo:accept` in an attachable session, and after you confirm **inside** that session the card flips to `plan-accepted`. It needs a live claude session the tester can't safely drive to completion unattended (TEST-002). How do we clear it?
- **Options:**
  - A. **Accept the unattended evidence + skip the live follow-through.** The move-guard, session attribution, and intent are unit-proven (`TestAcceptMoveGuard`, `TestAcceptSessionAttribution`, `TestBuildIntentAccept`, all green) and the tester drove the **real** board live via tmux up to the confirm dialog — it showed `will run: claude "/gogo:accept planpending" …` on the plan-pending card while a `plan-accepted` card still routed to `/gogo:go` (proving the status-branch). The only unverified part is the launched session's internal follow-through, which **reuses gogo-plan's already-exercised acceptance recording**.
  - B. **You run it live now.** I pause; you open `gogo`, press `m` on a genuine `awaiting-plan-acceptance` card, let the `/gogo:accept <slug>` session complete, and confirm `state.md`→`plan-accepted` + the `Status: **accepted**` line + exactly one new `plan-accepted` event; I re-run test round 2 to mark TEST-002 verified before ⑤.
- **gogo recommends:** A — routing + launch are proven end-to-end to the confirm, and the follow-through delegates to gogo-plan's battle-tested recording (no second acceptance path). B is available if you want to see the full flip live (note: there's no other plan-pending feature right now, so you'd seed a scratch one).
- **Status:** OPEN (awaiting user)

---

### RESOLVED (user, 2026-07-11) — D11 + D12: skip both live checks, advance to ⑤

The user chose **"Skip both, go to report"** at the test-phase user-decision gate:

- **D11 → A** — skip the live prompt-free `/gogo:done` run (TEST-001). Accepted the unattended evidence as sufficient: the regression lint `TestSkillsBashNoUnsafeRm` (green, `-count=1`) proves no unsafe `rm`/glob shapes remain — the exact thing the "dangerous rm" classifier keys on — plus the tester's isolated harness re-verification (empty-refusal byte-identical, idempotency, `before/` whole-clear, FR-A2 cleanup). The real prompt-free `/gogo:done` will be confirmed organically when this feature (or any) is shipped via `/gogo:done` after UAT.
- **D12 → A** — skip the live board-accept follow-through (TEST-002). Accepted the unattended evidence: the move-guard / session-attribution / intent unit tests (all green) plus the tester's live-tmux drive to the real confirm dialog (`will run: claude "/gogo:accept planpending"`, with a `plan-accepted` card still routing to `/gogo:go`). The launched session's internal flip reuses gogo-plan's already-exercised acceptance recording.

`test/issues.json` TEST-001/TEST-002 → `wontfix` (user-skipped, evidence accepted). Test is all-green (every relevant hands-on check run or user-skipped). Resume at ④ → advance to ⑤ report.
