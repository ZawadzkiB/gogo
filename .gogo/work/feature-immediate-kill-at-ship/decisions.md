# Decisions — feature `immediate-kill-at-ship`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

## D1 — The `/gogo:done` reap mechanism
- **Phase:** plan
- **Question:** How does `/gogo:done` reap the session it just shipped? It is a
  markdown skill run by a claude session, so a bash step is the natural fit — but
  which reaper does it call?
- **Options:**
  - A. **Skill bash step → plain `gogo sweep`** (+ the FR3 self-guard). The state
    flip is `shipped`-first, so the just-shipped session is already terminal and
    plain sweep reaps it via the exact-convention parse. Zero new CLI surface.
    Trade-off: reaps *all* terminal/orphan sessions on a ship, not just this slug's —
    but that is safe by definition (an orphan has no live owner; a terminal feature
    keeps no live session) and is the backstop sweep exists for.
  - B. **A targeted `gogo sweep <slug...>`** variant reaping only the named slugs'
    sessions. Surgical, a precise match to the FR wording, trivially handles a merged
    ship (`gogo sweep a b c`). Trade-off: adds a CLI slug argument + relaxes the
    current "sweep takes no slug" parse + new tests — a precision the ship path does
    not need (state is already terminal), and it *still* needs the self-guard.
  - C. **A Go `gogo done` wrapper** that ships and reaps. Trade-off: pulls `done`
    into the CLI — the explicitly-deferred gate/state-CLI slice (D5=C). Over-scoped.
- **gogo recommends:** **A** — simplest that fully works: no new command, no new
  arg, reuses `Sweeper`/`SessionMatchesSlug` wholesale; the **self-guard (D... FR3)**,
  not a slug filter, is what makes the call safe. B is recorded as the alternative if
  you prefer a ship to touch *only* its own sessions.
- **Status:** RESOLVED → A (user, 2026-07-12; see RESOLVED block at end)

## D2 — The board `remain-on-exit` fate
- **Phase:** plan
- **Question:** The board's interactive `Launch()` sets `remain-on-exit on`
  (`launch.go:459`) so a pane lingers after claude exits — a dead pane that still
  reads "● session running." What do we do with it?
- **Options:**
  - A. **Drop `remain-on-exit`** — a board-launched session closes when claude
    exits, matching `LaunchPersistent` and the headless `-p` path (no leak by
    construction). Badge is truthful *immediately*. Trade-off: you lose the
    post-exit scrollback of a *finished* session (mitigated: attach/peek while it is
    live; the durable trail is `.gogo/work/` + events + report). A gate keeps claude
    alive, so this never affects answering gates — `remain-on-exit` only ever kept a
    *dead* pane.
  - B. **Keep it, but reap dead panes on sweep** — preserves the post-exit read.
    Trade-off: the badge lies until the next sweep runs, and it needs new tmux
    `pane_dead` detection. Worse for the actual complaint (a *phantom running badge*).
  - C. **Keep it; rely on ship-reap + `gogo sweep` only.** Trade-off: does **not**
    fix an `m`-launched *non-terminal-but-finished* feature — its dead pane is spared
    by `shouldReap` (live, non-terminal owner) until the 24h TTL or a ship. The leak
    persists for up to a day.
- **gogo recommends:** **A** — both the simplest change (delete one `set-option`
  line + fix two comments) and the most correct for the badge. It brings the board's
  own launches in line with the `-p`/`--attach` paths the incident already proved
  leak-free.
- **Status:** RESOLVED → A (user, 2026-07-12; see RESOLVED block at end)

## D3 — Best-effort reap (never fail a ship)
- **Phase:** plan
- **Question:** What happens if the reap can't run — no `gogo` on PATH, no tmux, or
  `gogo sweep` errors?
- **Options:**
  - A. **Best-effort + silent skip** — guard the step
    (`command -v gogo >/dev/null 2>&1 && gogo sweep >/dev/null 2>&1 || true`); a
    missing CLI / tmux / a reap error never fails the ship. The existing
    `gogo sweep` / next-launch reap stays the backstop.
  - B. **Warn (but still ship)** — same, plus a one-line note when the reap is
    skipped so the user knows a manual `gogo sweep` may be wanted.
- **gogo recommends:** **A** (with the option to fold in B's one-liner) — the
  portability contract is non-negotiable: the core loop needs no external deps and a
  reap failure must never block a ship. This is not really contentious; recorded for
  completeness.
- **Status:** RESOLVED → A (user, 2026-07-12; see RESOLVED block at end)

## D4 — Ship-reap scope: accept the plain-`gogo sweep` edge cases, or tighten to a targeted sweep?
- **Phase:** review (surfaced by REV-001 + REV-002; fresh-eyes verdict was APPROVE, all gates green)
- **Question:** Review APPROVED the build (FR1-FR5 correct, `gofmt`/`vet` clean, `TestSweepSparesSelf` + `TestSkillsBashNoUnsafeRm` green). Two minor edge cases of the accepted D1=A "plain `gogo sweep`" ship-reap surfaced:
  - **REV-001 (P3, inherent):** the shipped card can briefly keep a live "● running" badge from its OWN `gogo-done-<slug>` host session. The FR3 self-guard (correctly) spares that session so `/gogo:done` never truncates itself; interactive claude idles after Return, so the host stays live until the user quits it (FR4 then closes the pane) or a later sweep reaps the now-terminal feature. The DRIVING `gogo-go-<slug>` reap - the headline win - IS delivered. Inherent to self-reaping; the plan already deliberately spares the host (see plan "The self-kill hazard").
  - **REV-002 (P2, new hazard):** a ship's plain `gogo sweep` can truncate a DIFFERENT feature's concurrent `/gogo:done`. The self-guard is exact-name-only, so a concurrent `gogo-done-z` (z already flipped `shipped` in step 5, still finishing steps 6-7) is reaped by x's ship-sweep. Bounded - z is already durably shipped; only its best-effort viewer page (rebuildable via `/gogo:view`) + Return summary are lost - but a NEW race, contradicting the D1=A "collateral is safe by definition" premise (a terminal feature's done-host is transiently live).
- **Options:**
  - A. **Accept D1=A + document both.** Ship 0.17.0 as-is; record REV-001 as works-as-designed and REV-002 as a documented known-limitation (rare: needs two overlapping different-slug `/gogo:done` runs; bounded: no data loss/corruption). Note targeted-sweep (D1=B) as the clean future fix in the report + cli-contract. Zero new code; keeps the plan's deliberate minimalism.
  - B. **Implement D1=B now (targeted `gogo sweep <slug>`).** Re-plan to add a slug-scoped sweep the ship-reap calls, so a ship only reaps its OWN slug's sessions and can never truncate another feature's in-flight ship. Fixes REV-002. Cost: expands the accepted scope (new CLI arg + parse relaxation + tests) the plan explicitly deferred; loops back to ① re-plan -> re-accept. (Does not change REV-001 - the host lingers by design either way.)
- **gogo recommends:** **A** - REV-002 is low-probability + bounded-impact, and the plan already weighed D1=B and chose D1=A for minimalism ("not needed"). Accept + document now; keep D1=B recorded as the surgical fix if concurrent multi-ship ever becomes common. REV-001 is genuinely works-as-designed.
- **Status:** RESOLVED → B (user, 2026-07-12; see RESOLVED block at end)

### RESOLVED (user, 2026-07-12) — D4 → B (targeted ship-reap), overriding D1
The user chose **B** over gogo's recommendation, with the guiding principle "a ship should
touch ONLY its own card's session, never sweep the whole board":
- **REV-002 → FIX (this supersedes the plan's D1=A).** The `/gogo:done` ship-reap must reap
  **only the shipped slug(s)'** sessions - a **targeted `gogo sweep <slug>...`**, not a
  board-wide sweep. This is **D1 = B**, moved from the plan's Out-of-scope into scope.
  Plain `gogo sweep` (no slug) stays as the **manual, whole-board** orphan/terminal cleanup
  the user runs themselves. Net: a ship can never truncate another feature's in-flight ship.
- **REV-001 → ACCEPT (minor, works-as-designed).** The ship's own host window lingers a
  moment because the reap can't kill the session it runs inside (the FR3 self-guard); it
  closes when the user quits it (FR4) or on the next sweep. Cosmetic; not worth complicating.
- **Resolution routing:** re-implement in-context (targeted sweep + Only-filter seam), re-run
  gates, then a fresh re-review before advancing to ④ test. Plan + adjustments updated.

<!-- Template for each decision — copy and fill:

## D<n> — <short title>
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

---

## RESOLVED (user, 2026-07-12) — plan-acceptance gate

Accepted as-is, all three recommendations:

- **D1 → A** — `/gogo:done` reaps via a best-effort bash step calling **plain `gogo sweep`** (state is `shipped`-first → the session is already terminal; reuses `Sweeper`/`SessionMatchesSlug`; the FR3 self-guard makes it safe). No new command/arg.
- **D2 → A** — **drop `remain-on-exit`** on the board's `Launch()` so a finished pane closes (matches the `-p`/`--attach` paths); badge truthful immediately.
- **D3 → A** — best-effort reap, silent skip; never fails a ship (`gogo sweep` / next-launch reap stays the backstop).

`state.md` → `plan-accepted`. BUILD HELD until `cockpit-cards-and-cli-awareness` ships (one-owner on `cli/`).
