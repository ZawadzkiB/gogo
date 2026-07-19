# UAT — feature `projects-sources-plans`

<!-- The UAT gate log — the plan-gate symmetry at the END of the pipeline.
  Phase ⑤ (report) no longer ends at `done`; it ends at status: awaiting-uat, and you verify the work.
  There are exactly two ways forward, and both are recorded here (append-only, newest round at the bottom):

  1. ACCEPT — running `/gogo:done` IS the acceptance (no extra confirmation question, mirroring how
     accepting a plan unlocks `/gogo:go`). `/gogo:done` first appends the one-line accept verdict below,
     then ships as usual.
  2. ISSUES / QUESTIONS — you describe what's wrong or ask a question instead of shipping. The
     orchestrator hands your input to `gogo-analyst`, which analyses it against the current plan.md +
     decisions.md + THE CODE (code = source of truth) and appends an issues round below; adjustments.md
     logs the plan delta and plan.md is updated. You RE-ACCEPT the adjusted plan, then `/gogo:go` reruns
     ②→⑤ — the SAME work item, never a new one — landing back at awaiting-uat.

  Each round is numbered sequentially (round N). state.md's `iterations:` line gains `uat=N`, counting the
  re-plan loop-backs.

  Round format — an ISSUES round (analyst-authored):

    ## UAT round N — <YYYY-MM-DD>
    **Input (verbatim):** <the user's UAT feedback, quoted exactly as given>
    **Analysis:** <the analyst's read of it against the current plan.md + decisions.md + the actual code>
    **Proposed plan delta:** <what changes in plan.md; adjustments.md logs the same delta>
    **Disposition (per point):**
      - <point> — fix-needed
      - <point> — works-as-designed (explain why the current behaviour is correct)
      - <point> — new-scope (out of this item; note where it goes)
    **Verdict:** re-planned — awaiting re-acceptance
      (then, once accepted:) re-accepted (user, <YYYY-MM-DD>) → /gogo:go reruns ②→⑤
      ^ the analyst writes the round up to "awaiting re-acceptance" and STOPS; the
        orchestrator appends this second line to THIS round's Verdict when the user
        re-accepts (the same step it bumps iterations uat=N and emits uat-failed).

  Round format — an ACCEPT round (via /gogo:done, no analyst round needed):

    ## UAT round N — accepted (user, <YYYY-MM-DD>) — via /gogo:done
    <optional one line: what the user verified>
-->

## UAT round 1 — 2026-07-18

**Input (verbatim):**
> "it still doesnt looks like on designs???" [screenshot: `./gogo` in `~/repos/gogo` showing the
> single-repo fallback board — no tab bar, no source chips]
>
> "you are right if I open per repo gogo it should just show its ticket and thats ok, I need to
> install gogo globaly, we some command like gogo global init so then this will be place where all
> projects lives"

**Analysis (against plan.md + the code):** Running `./gogo` inside `~/repos/gogo` hit
`chooseBoard` case 2 (a repo with no OWNING registered project → `tui.New(root)`, the byte-for-byte
single-repo board with no tabs). The tabbed `board·plans·config` cockpit only renders for a
registered project (verified live: rendering `~/repos/gogo` as a `projects.Project` shows the full
tabbed cockpit + source chips + knowledge explorer). The user's model is now explicit and CLEANER
than the plan's: there are **two modes** — (a) **repo-local:** `gogo` in a repo shows THAT repo's
tickets (the current single-repo board — correct, keep it); (b) **global cockpit:** a separately
**initialized** home (`~/.gogo/`) where all projects live, opened as its own view. The plan
conflated the two by auto-routing an in-project repo to the project cockpit (`chooseBoard` case 1)
and by having the tabbed UI appear implicitly on registration. The fix is an explicit
**`gogo global init`** setup + an explicit way to **open** the global cockpit, and making per-repo
`gogo` always the simple repo board.

**Command surface (user-confirmed via AskUserQuestion, 2026-07-18 → option A):**
- `gogo` **inside a repo** → that repo's board (single-repo, its tickets). Always.
- `gogo global` → the global tabbed cockpit (board·plans·config across projects), from anywhere.
- `gogo` **outside any repo** → the global cockpit.
- setup: `gogo global init` (creates `~/.gogo/` — the home where projects live) → `gogo project add <repo>`.

**Proposed plan delta:**
- **FR19 — `gogo global init`** (new): initialize the global cockpit home `~/.gogo/` (create the
  dir + `~/.gogo/config.json` global marker/config); idempotent; prints the location + next step
  (`gogo project add <repo>`). This is the explicit "turn on the global cockpit" entry.
- **FR20 — `gogo global`** (new): open the global tabbed cockpit from anywhere (even inside a repo).
  If the home isn't initialized → a friendly hint to run `gogo global init`. If initialized with 0
  projects → the cockpit's empty state hinting `gogo project add`.
- **FR21 — `chooseBoard` re-shaped to the two-mode model:** inside a repo → ALWAYS that repo's
  single board (DROP the case-1 "in-source-of-project → project board" auto-route — per-repo stays
  simple); outside any repo → the global cockpit when the home is initialized, else a hint to
  `gogo global init` or to cd into a repo. `gogo global` forces the cockpit from anywhere.
- **FR22 — global-home "initialized" marker:** `~/.gogo/config.json` (written by `global init`;
  also ensured by the first `gogo project add` so registering is forgiving). The global cockpit
  entry points key off it.
- Enum-sync the new `global` verb across the four sources; docs (`README.md`,
  `docs/cli-contract.md`, `skills/gogo-cli/SKILL.md`) describe the two modes; version stays 0.21.0.

**Disposition (per point):**
- Per-repo `gogo` shows only the repo's tickets — **works-as-designed** (the single-repo fallback
  is correct; the confusion was that the *global* cockpit had no explicit entry). Now made explicit.
- No global setup / cockpit only appears on registration — **fix-needed** → FR19/FR20/FR22.
- In-project repo auto-routed to the project cockpit — **fix-needed** → FR21 (drop case 1).
- The global cockpit as a UNIFIED all-projects board (design TURN 3a) vs the current per-project
  focus + `p` switcher — **new-scope** (noted as a fast-follow; this round keeps the built
  per-project-with-switcher cockpit, which `gogo global` opens).

**Verdict:** re-planned — awaiting re-acceptance
re-accepted (user, 2026-07-18, command surface confirmed = option A) → /gogo:go reruns ②→⑤
