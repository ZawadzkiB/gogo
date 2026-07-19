# Adjustments — feature `projects-sources-plans`

Log of changes / clarifications requested during planning (and mid-UAT re-plans).
Each entry: date · what changed · why.

## 2026-07-18 — acceptance gate: 9 decisions resolved (D1–D9)

- **D1 → A** data root `~/.gogo/` (`$GOGO_DATA_HOME` seam). **D2 → A** `config.json`.
  **D3 → A** spawn via `claude -p` + explicit `--correlation` param. **D4 → A** one-shot
  non-destructive auto-migration. **D5 → A** `project add`/`source add` semantics + four-way
  `chooseBoard`. **D6 → A** `tab` board→plans→config. (D1/D3/D4/D5/D6 = gogo's recommendation.)
- **D7 → A (user overrode rec B):** ship phases A–D as ONE `0.21.0` drop (not four minors);
  reset the working-tree `0.24.0`, bump to `0.21.0` once at the end.
- **D8 (user-added):** a plan is ONE entity with a **status lifecycle**
  `draft → ready → active → done` — "a draft is a plan in draft status; a plan becomes ready
  to implement." Inserted `ready` into FR12's `draft|active`; plans tab groups
  DRAFTS·READY·ACTIVE (FR10). *Why:* the user clarified drafts/epics aren't separate entities.
- **D9 (user-chosen):** keep `gogo draft` / `gogo epic` as thin CLI **aliases** into `gogo plan`
  (they map to plan statuses per D8). Adjusted FR17 (was: `plan` fully supersedes them).

No re-plan needed — these refine the accepted plan; state.md → `plan-accepted`. Implementation
proceeds A→B→C→D, built + tested in order, shipped as one 0.21.0 release.

## 2026-07-18 — UAT round 1: two-mode model + `gogo global init`

- The tabbed cockpit was hidden behind project registration (a lone repo hit the single-repo
  fallback). User clarified the model: **repo-local** `gogo` shows just that repo's tickets (keep
  the fallback), and the **global** multi-project cockpit is a separately-initialized home.
- Plan delta (FR19–FR22): add `gogo global init` (create `~/.gogo/` — the projects home + a
  `config.json` marker), `gogo global` (open the cockpit from anywhere), re-shape `chooseBoard`
  (in-repo → repo board ALWAYS, dropping the case-1 in-project auto-route; outside → global cockpit
  if initialized, else hint). Command surface user-confirmed (option A). Version stays 0.21.0.
- Deferred (new-scope): a UNIFIED all-projects board (design 3a) — `gogo global` keeps the built
  per-project-focus + `p` switcher cockpit for now.
