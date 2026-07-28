# Decisions — feature `plans-board-kanban`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

## D1 — build scope (slice, or all at once)
- **Phase:** plan
- **Question:** Build the whole vision at once, or slice it?
- **Options:**
  - A. Slice 1 now (FR1-FR4), Slices 2/3 as separate follow-up plans.
  - B. One plan covering all of FR1-FR6, built + tested together, shipped as 0.26.0.
- **gogo recommended:** A.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-23)
**B — everything (FR1-FR6) is in scope for THIS one accepted plan/build**, shipping
as 0.26.0. The Changes checklist is organized into build phases (A = kanban +
re-sequence, B = done-archive, C = auto-pickup) for a sane build order, but all
three are implemented and tested together before ship. The "slice cut" framing is
dropped in favour of a "Build order" section.

## D2 — plans-tab keymap for the manual move
- **Phase:** plan
- **Question:** How does the user trigger a phase move on the plans kanban?
- **Options:**
  - A. A single board-style **`m` move** that advances the focused plan one column
    right, resolving mark-ready / go-spawn / project-UAT by current status.
  - B. Distinct keys — `r` mark-ready, `g` go, `D` done.
- **gogo recommended:** A.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-23)
**A — single board-style `m` move.** Today's plans-tab `r` (accept+spawn) and `D`
(accept UAT) **fold into `m`** — their tests are adapted. The CLI verbs
`gogo plan ready` / `gogo plan go` / `gogo plan done` are kept as the mirrors.

## D3 — [Phase B] where the done-archive / project changelog lives
- **Phase:** plan
- **Question:** When `MarkDone` archives a plan on active→done, what shape is the
  project-level changelog entry, and where under `~/.gogo/`?
- **Options:**
  - A. A **deterministic, LLM-free CLI record** + archive under
    `~/.gogo/projects/<name>/.gogo/changelog/<date>-<id>/`.
  - B. A **launched `claude -p` synthesis** like `/gogo:done`.
- **gogo recommended:** A.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-23)
**A — deterministic CLI record.** On active→done, `MarkDone` ALSO writes a
deterministic, LLM-free project-changelog entry under
`~/.gogo/projects/<name>/.gogo/changelog/<date>-<id>/` (path mirrors how
`plans.Dir` is built: `filepath.Join(projects.Dir(project), ".gogo", …)` —
verified against `cli/internal/projects/projects.go` + `cli/internal/plans/plans.go`).
The entry records: date, plan id, plan title, and the shipped member work items
(`source : slug`, plus a one-line pulled from each member's state/report **only if
cheaply available** from the already-loaded board contract, else just `source:slug`).
**No launched `claude -p` synthesis.** The plan **STAYS in the plans store as
`status=done`** (the kanban's 4th `done` column reads `plans.List`, so a moved-out
plan would vanish from the board) — "archive" means writing the **additional**
deterministic changelog record + flipping `done` in place (a COPY/record, never a
MOVE out of `plans/`). All writes stay under `~/.gogo/`.

## D4 — [Phase C] the auto-pickup firing mechanism
- **Phase:** plan
- **Question:** What fires the automatic plan→implementation pickup for a work item
  spawned into a skip-configured source? (Invariant: ALWAYS a launched
  `claude -p /gogo:go <slug>`, never a CLI state-flip.)
- **Options:**
  - A. An explicit `gogo plan pickup [<id>]` verb + a plans-tab key.
  - B. Auto-fire from the cockpit's reload/refresh path.
  - C. Skill-chained plan→go for skip sources.
- **gogo recommended:** A.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-23)
**B — auto-fire from the cockpit's reload path, with REQUIRED guards.** Fires ONLY
from the open cockpit's existing reload loop (the fsnotify-driven `reloadMsg`
handler in `cli/internal/tui/update.go`, after `m.reload()`; the 5s `sessionsMsg`
tick is a secondary refresh) — NOT a CLI daemon (a headless/scheduled caller can
reuse the same reconcile function as a later follow-up). Eligibility — ALL must
hold before it launches:
1. the member's **SOURCE has `planAcceptanceSkip`** set (`projects.SkipForSource` /
   `Source.PlanAcceptanceSkip`) — a non-skip source is NEVER auto-fired (waits for a
   manual `gogo go`);
2. the member's work item is at **`plan-accepted`** — the exact state a manual first
   `gogo go` acts on (`orchestrator.RunnableStatus` = `plan-accepted|implementing|
   reviewing|testing`; a skip-acceptance spawn auto-accepts the plan, so the member
   lands at `plan-accepted`. Auto-pickup keys on the **initial `plan-accepted`
   handoff** only, not a mid-pipeline status, so it never auto-resumes a parked/
   headless mid-run — verified against `cli/go.go cmdGo` + `orchestrator.go`);
3. it has **NO live session** (`liveSessionFor` / `SessionMatchesSlug` — the dedupe
   guard, exact match, never substring);
4. it has **not already been auto-fired this cockpit lifetime** — a fire-once
   in-memory set keyed by the composite `featureKey` (`Root\x00Slug`).

**The cap two-branch.** `concurrentWorkItems` is an **INTEGER on the SOURCE**
(`projects.Source.ConcurrentWorkItems`; default `1`, `0` = unlimited — verified in
`cli/internal/projects/projects.go`), NOT a bool and NOT a plan/project setting. The
free-slot test is per-SOURCE, scoped to the source's repo root — never global, never
per-plan: `active := orchestrator.ActiveWorkCount(m.repo, root, m.sessions, f.Slug)`
(distinct in-progress + live-session features whose `Root == root`, excluding the
member itself; `ActiveWorkSlugs`), `cap := orchestrator.CapForSource(capWatchSources,
root)`, under-cap iff `!orchestrator.CapExceeded(cap, active)`. This is the exact trio
the board's `capBounce` (`cli/internal/tui/move.go`) uses — reused, not reinvented.
Two members of one plan spawned into two DIFFERENT sources each get their own
per-source slot budget. Concretely: `ConcurrentWorkItems=1` with a run in flight →
AT cap (no fire); `ConcurrentWorkItems=2` with 1 running → UNDER cap (fires); `2/2`
→ AT cap.
- **Free slot** — the source is **UNDER** its cap (in the common `cap=1` case:
  nothing else running in that repo/source) → **AUTO-FIRE** `/gogo:go <slug>` (all
  the other eligibility guards above still apply). The composite key is added to the
  fire-once set on the ACTUAL launch.
- **Repo busy** — the source is **AT** its cap (a run already in flight there) → **DO
  NOT auto-fire.** Instead surface a **visible "needs manual trigger" cue on BOTH the
  eligible work-item card (work board) AND its plan card (plans kanban)** so the user
  sees it is ready and can start it themselves (`gogo go` / board `g`). This is an
  explicit signal, NOT a silent wait.

**Fire-once × cap (transient / auto-when-free — recommended, matches user intent):**
the fire-once set records only ACTUAL auto-launches; a **cap-skipped (busy) member is
NOT added**, so it stays eligible. While cap-blocked it shows the manual-trigger cue;
when a slot frees on a LATER reload it **auto-fires then** and the cue clears. (The
alternative — permanently-manual-once-blocked — is simpler but loses the "queue drains
itself when a slot frees" behaviour the user wants, so it is not chosen.)

The launch is ALWAYS `claude -p /gogo:go <slug>` (an attachable detached tmux
session, byte-for-byte a manual go via `launch.BuildIntent(ActionGo,…)` + the
board's launcher seam, carrying `--skip-acceptance`/`--skip-uat` per the source's
flags via `SkipForSource`/`SkipParams`); the CLI writes NO source pipeline state.
Auto-pickup only removes the **initial keypress** for skip sources — a genuine
mid-pipeline decision gate still PARKS the session at `waiting-for-user` and
surfaces on the board's "needs you", never waived.
