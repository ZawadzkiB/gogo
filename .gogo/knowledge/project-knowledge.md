# Project knowledge

**Purpose:** what this project is — architecture, domains, and the key decisions
the plan phase must respect.

<!-- gogo:meta
Mode: proxy
Source: [ ../../README.md ]
Confidence: high
Generated-by: /gogo:build
-->
> Architecture, domains, key decisions. Source of truth: **../../README.md**
> (the published plugin docs). This file distils it for the pipeline.

## What this project is
**gogo** is a Claude Code **plugin**: a portable, knowledge-grounded development
pipeline — **plan → implement → review → test → report**. The *flow* ships with
the plugin; the *rules* are per-project markdown in `.gogo/knowledge/`. "Same
pipeline everywhere; the behaviour is configuration."

## Architecture
Three layers, all plain markdown (+ a little bash and one vendored JS):

- **`commands/*.md`** — ultra-thin entry points. Orchestration: `build|plan|go|
  accept|status|resume`. **Standalone phase commands** (since 0.2.0): `implement|review|
  test|report` — each runnable alone, each a typed function (validate-in → work →
  validate-out). **Knowledge maintenance** (since 0.3.0): `skills` — audit the
  knowledge line budget + extract on-demand skills. No flow logic; each just
  invokes a skill.
- **`skills/*/SKILL.md`** — the operating manuals (the real logic):
  - `gogo` — the orchestrator (phases, loops, decision gates, feature-folder state).
  - `gogo-plan` ①, `gogo-implement` ②, `gogo-review` ③, `gogo-test` ④, `gogo-knowledge` ⑤.
  - `gogo-contracts` — the pipeline's "type system": JSON-Schema registry +
    portable two-tier validate-in/out gate (since 0.2.0).
  - `gogo-build` — wire `.gogo/knowledge/` from project docs.
  - `gogo-skills` — audit the knowledge line budget; extract bloat into on-demand
    skills (`knowledge` → `.gogo/skills/`, `standalone` → `.claude/skills/`).
  - `gogo-mermaid` — portable diagrams (vendored mermaid, offline viewer).
- **`templates/contracts/*.schema.json`** — the artifact contracts that cross
  phase boundaries: `issues-list`, `phase-result`, `pipeline`, `charts-manifest`.
- **`agents/*.md`** — specialist subagents the orchestrator delegates to:
  `gogo` (orchestrator, hands-off), `gogo-developer` ②, `gogo-reviewer` ③, `gogo-tester` ④.
- **`hooks/`** — `config-check.sh` (SessionStart reminder), `notify.sh`
  (Notification → ntfy/macOS/bell). **`assets/mermaid/`** — vendored UMD build +
  offline `viewer.template.html`. **`.mcp.json`** — bundled Playwright MCP.

## Domains & glossary
- **Knowledge file** — a `.gogo/knowledge/*.md` config file, `proxy` (links the
  project's real doc) or `owned` (gogo authored it). Read at specific phases.
- **Feature folder** — `.gogo/work/feature-<slug>/`: `plan.md` (the contract),
  `adjustments.md`, `state.md`, `decisions.md`, `review-NN.md`, `test-NN.md`,
  `report.md`, `charts/`. The pipeline's memory + audit trail.
- **Phase / gate / loop** — five fixed phases; decision gates pause for the user;
  implement↔review↔test loop until clean (bounded ~3 rounds per finding).
- **Contract / validate-gate** (since 0.2.0) — a typed artifact (`issues.json`,
  `charts/manifest.json`, `result.json`, `pipeline.json`) governed by a JSON Schema
  in `templates/contracts/`. Each phase runs **validate-in** (required inputs exist,
  parse, conform) and **validate-out** (its output conforms) via `gogo-contracts` —
  portable: `jq`/validator if present, else the agent checks against the schema.

## Key decisions (constraints the pipeline must respect)
- **Generic flow, per-project config** — never bake project specifics into the flow.
- **Portability** — core loop needs **no external deps**; mermaid is vendored
  (offline); Playwright/`mmdc`/`jq` are optional and degrade gracefully.
- **Only ever write under `.gogo/`** — never edit a proxied upstream file.
- **Hard gate** — never implement an unaccepted plan.
- **Idempotent build** — re-runs preserve `## gogo overrides` and `Mode: owned`.

## gogo overrides
<!-- gogo-specific notes not in the linked source. Preserved across re-runs. -->
- The repo IS the plugin source; `${CLAUDE_PLUGIN_ROOT}` references resolve to it.
- Installed via marketplace `gogo` → GitHub `ZawadzkiB/gogo`; version in
  `.claude-plugin/plugin.json` must be bumped for installs to detect updates.
- **Knowledge vs on-demand skills (since 0.3.0):** always-read `.gogo/knowledge/*`
  is held to a line budget (OK `<200` / WARN `200-400` / OVER `>400`) so workers
  stay deterministic; `/gogo:skills` extracts bloat into on-demand skills. The
  `.gogo/`-only write rule has **one user-gated exception**: an approved
  `standalone` skill written to `.claude/skills/`. Full model: `docs/architecture.md`.
- **Hosted docs + code-verified discovery (since 0.4.0):** a GitHub Pages docs
  site (Jekyll + `just-the-docs` remote theme, GitHub-built, no local build) lives
  under `docs/` and deploys from branch `main` folder `/docs` (config at
  `docs/_config.yml`) — published at `https://zawadzkib.github.io/gogo/`;
  **code/skills stay authoritative**, the site is generated from them. `/gogo:build` now ends with a **verify-against-code**
  pass: high-signal claims (stack, build/run/test commands, test framework, entry
  points) are cross-checked against the code and **code wins** on conflict
  (correct the gogo summary, never the upstream), recorded in `_discovered.md`.
- **Workspace + changelog + viewer (since 0.5.0):** the feature workspace is
  **`.gogo/work/`** (was `.gogo/plans/`) and the vendored mermaid runtime lives at
  **`.gogo/resources/`** (shared; `/gogo:build` Step 0 auto-migrates legacy layouts).
  Report ⑤ writes a **`report/` bundle** (report.md + a diff-chosen UML set incl.
  the **`use-case`** kind + offline `diagrams.html`). **`/gogo:done`** copies a
  feature's report bundle into the append-only **`.gogo/changelog/<date>-<slug>/`**;
  **`/gogo:view`** opens an offline page with the summary + custom pan/zoom/drag
  diagrams (renderer vendored at `.gogo/resources/viewer/`). `/gogo:report` has a
  **lenient mode** to document past/broken runs. Command set is now **12**.
- **Interactive diagrams + before/after compare (since 0.6.0):** `/gogo:view`'s
  renderer is now **xplan-style** — mermaid lays out, its SVG is parsed into a
  `{nodes,edges}` model and re-rendered as custom node cards with an owned edge
  layer; **drag a node and its edges re-route live**, plus zoom/fit/minimap and a
  **persisted layout** (localStorage + an Export button → `<name>.layout.json`).
  Non-flowchart kinds fall back to the pan/zoom canvas. Plan ① now draws an as-is
  **`charts/before/`** baseline; report ⑤ copies it to **`report/before/`** and adds
  a **before/after** side-by-side comparison; `/gogo:done` **prints a `file://`
  viewer link**. Renderer modules: `assets/viewer/{geometry,viewport,mermaid-parse,
  render,interactive}.js`.
- **View menu + plan bundles + `/gogo:done` work board (since 0.7.0):**
  `/gogo:view` (no arg) now shows a grouped **Work** (each feature's plan + report)
  / **Changelog** (shipped reports) `AskUserQuestion` picker; **plans are viewable
  bundles** rendered **in place** from `plan.md` + `charts/` (D1=A), and plans/
  reports are authored **article-style** (lead summary, bold key parts). `/gogo:done`
  (no slug) classifies all `.gogo/work/*` via the shared **`gogo-status`** work-index
  (shipped / ready-to-ship / in-progress / unfinished) and opens an **interactive
  terminal kanban** — vendored `python3` curses `assets/kanban/board.py` in a tmux
  pane that ships on drop — or, when `python3`/`tmux`/tty are absent (**soft deps**),
  the status-table + `AskUserQuestion` multi-select fallback; shipping stays
  single-sourced. Command set still **12**; version **0.7.0**.
- **Merged + synthesized changelog entries (since 0.8.0):** a changelog entry is a
  **written synthesis, never a copy** — for merged releases AND ordinary single
  ships (supersedes the 0.5.0 "copies the report bundle" behaviour above).
  `/gogo:done` can ship several related features as ONE merged entry at
  `.gogo/changelog/<date>-<name>/`: board/multi-select picks ≥2 → one
  separate-vs-merged gate, or the direct `slug1+slug2` arg pre-answers it; the
  release name is suggested + confirmed (D2), date = newest member. Entries carry
  a **slim set** — synthesized `report.md` + slug-prefixed `.mmd` + `manifest.json`
  with an additive optional **`members[]`** (charts-manifest schema) + `before/`;
  **no `diagrams.html` copy** (`/gogo:view` builds the page from source).
  `gogo-status` classifies a member as shipped via `members[]` even though the
  entry dir is named after the release; `board.py` untouched; the full audit trail
  stays in `.gogo/work/` (linked). Command set still **12**; version **0.8.0**.
- **Board cockpit — action keys + filter + intent protocol v2 (since 0.9.0):**
  the `/gogo:done` board is the **pipeline cockpit** — one mode, action keys
  (`v` view · `s` ship · `m` ship-merged · `g` go/resume · `/` live filter ·
  `q` cancel) with per-class guards. Every action is a **single-shot schema-v2
  intent** `{"schema":2, "action", "items"}` written to **`board-intent.json`**
  (renamed from `ship-result.json`; legacy `{"ship":[...]}` still parsed as
  `action: ship`); `gogo-done` executes the intent and **relaunches the board**
  (re-classifying in between) — only `go`/`cancel` end the loop; `board.py` stays
  a **no-mutation selector** with the 0/1/2 exit contract, now **crash-safe**
  (any TUI failure → exit 2 + one-line stderr, routed to the fallback, never
  misread as a cancel). validate-in relaxed: the cockpit opens whenever **any**
  `.gogo/work/feature-*` exists (only zero features stops). The chat fallback
  stays ship-focused (`/gogo:view` + `/gogo:go` cover the rest). Command set
  still **12**; version **0.9.0**.
- **The `gogo` CLI + events telemetry (since 0.10.0):** the repo now also ships a
  **Go binary** — `cli/` (Go 1.25, Charm stack: bubbletea/bubbles/lipgloss/
  glamour/huh + goldmark + fsnotify) — a **deterministic cockpit** that opens the
  4-column board in milliseconds by parsing the contract files directly (**no LLM
  in the read path**): drill-in terminal viewers (glamour md, issues tables,
  events timelines, ASCII flowcharts via an internal renderer), a native `w` web
  page build, and column moves that **launch Claude** (`/gogo:go`, `/gogo:done`)
  in attachable tmux sessions `gogo-<action>-<slug>` — the CLI never mutates
  pipeline state. Every feature gains **`events.jsonl`** (append-only telemetry,
  `templates/contracts/events.schema.json`, RFC3339): **phase skills own their
  lifecycle events, the orchestrator emits only gate events** — each transition
  exactly once, beside every state.md write; a missing file is never an error.
  The frozen consumer spec is **`docs/cli-contract.md`**. Subcommands
  status/view/events; `gogo --version` mirrors plugin.json. Command set still
  **12** (the CLI is a binary, not a 13th command); version **0.10.0**.
- **Planning analyst + the UAT gate + CLI ops (since 0.11.0):** phase ① is now
  delegated to a **fifth agent — `gogo-analyst`** — driven by **`analysis.md`**,
  the **10th knowledge file** (the ordered analysis procedure, knowledge files
  named per phase, **code = source of truth**, a capability-detected
  external-docs hook); every command states the **orchestrator-first** chain
  (command → orchestrator → specialist agent). Report ⑤'s green path now ends
  at **`status: awaiting-uat`** — the **UAT gate**: running `/gogo:done` IS the
  acceptance (recorded as the `uat.md` verdict line; `uat-passed` → `shipped`);
  issues instead **lock the item** (`waiting-for-user` + `uat-opened` — done AND
  go both refuse) while `gogo-analyst` re-plans the **SAME work item** (`uat.md`
  round + plan delta), and only user re-acceptance (`plan-accepted`, then
  `uat=N` + `uat-failed`) reruns ②→⑤. Classifier: **ready-to-ship = a final
  report AND `awaiting-uat`/legacy `done`** (additive clarification — a rerun's
  stale report is in-progress). Every knowledge file may carry a user-owned
  **`## Custom`** section — preserved 1:1 by build re-runs, the ⑤ reconcile,
  and `/gogo:skills` (exempt from budget + extraction). CLI 0.11.0: `x` delete
  → recoverable **`.gogo/trash/<ts>-<slug>/`** + `gogo trash` list/restore
  (changelog un-deletable at UI + package level; the CLI's ONE write outside
  `.gogo/resources/`), `l` read-only session log peek (capture-pane, never an
  attach), launches carry **`--permission-mode auto`**
  (`GOGO_CLAUDE_PERMISSION_MODE` tri-state: unset→auto · set→verbatim ·
  empty→omit), `awaiting-uat` badge on ready cards (`waiting-for-user` wins
  mid-UAT). Agents **5**; knowledge files **10**; command set still **12**;
  version **0.11.0**.
- **In-context implement + the CLI process-orchestrator (0.12.0 → 0.13.0):** there
  are now **TWO orchestrators** over the one shared core (phase skills + typed
  contracts). **0.12.0** — the in-chat `/gogo:go` orchestrator **runs ② implement
  in its own context** (warm across the fix loop, no re-spawn/re-read); it delegates
  only the fresh-eyes phases ③ review + ④ test (① plan stays `gogo-analyst`).
  **0.13.0** — a **Go CLI orchestrator**, `gogo run [<slug>]` (`cli/internal/orchestrator`),
  drives ②→③→④(→⑤) by spawning each phase as a `claude -p` session: the **developer
  session kept warm across fix rounds via `claude -p --resume <session-id>`** (pre-assigned
  UUIDs), **review/test spawned fresh** (new UUID, no resume). The Go loop is a dumb
  deterministic sequencer — judgment + gates stay with the claude phase-sessions + the
  human (via the attach path / `gogo run --attach`); routing is **one shared rule**
  (`contract.Route`, track-aware: review batches minors per `gogo-review` §④, test routes
  on any per `gogo-test` §④). Bounds (round budget + cost ceiling) **gate**, never abort;
  a **CLI-owned session registry** lives at `.gogo/resources/cli/sessions/<slug>.json` (the
  CLI still never mutates pipeline state). Needs `--in-session` on `/gogo:implement` so
  `--resume` continues the real worker. The in-chat path stays the simple, dependency-free
  default; `gogo run` is the opt-in power path (needs tmux+claude). This is **Slice 1** of
  roadmap #11; multi-model (gemini/codex/opencode) is a later slice behind an agent-type
  seam. Slash command set still **12** (`gogo run` is a **CLI-binary subcommand**, not a
  13th slash command — like `status`/`view`/`events`/`trash`); version **0.13.0**.
- **Unattended ops + input signals + board accept (0.14.0):** three linked fixes.
  **(A)** gogo's own mechanical `/gogo:done` bash (changelog assembly + board
  stale-file cleanup) is rewritten to **guarded scoped-`find … -delete`** (no
  glob-`rm`, no bare-variable `rm`) so it stops tripping Claude Code's "dangerous rm"
  permission classifier; a Go lint (`cli` suite, `TestSkillsBashNoUnsafeRm`) fails if
  an unsafe shape reappears. **(B)** one `contract.Feature.WaitingForInput()`
  predicate (union of `awaiting-plan-acceptance` + `waiting-for-user` + `awaiting-uat`)
  is surfaced in three read-only display sites: a ⏸ card cue on the TUI board
  (incl. plan-pending, which had none), a dedicated **WAIT** column in `gogo status`,
  and **vertical separators** between the four board columns. **(C)** a new
  **`launch.ActionAccept`** routes the board's `m` on an `awaiting-plan-acceptance`
  card to a thin launched **`/gogo:accept <slug>`** (skill `gogo-accept`) that records
  acceptance via gogo-plan's existing recording — closing the dead end where `m`
  bounced into a `/gogo:go` that refuses (the CLI still never mutates pipeline state).
  Slash command set now **13** (adds `accept`); frozen contract stays additive
  (presentation-only); version **0.14.0**.
- **Persistent-session CLI orchestrator (0.15.0):** the Go CLI orchestrator is
  reworked from the 0.13.0 **per-phase loop** into a **session-lifecycle manager**
  over the one skill. `gogo go <slug>` / `gogo plan <slug>` **launch-or-`--resume`
  ONE persistent `claude -p` session** running the existing `/gogo:go` / `/gogo:plan`
  skill (implement in-context + `Task` review/test + report); the CLI runs **no phase
  loop and no routing in Go** — the single routing rule lives in the skill, so the
  drift-bug class is gone. The **Go per-phase loop + `contract.Route`** (and the dead
  `contract.ReadResult`) are **deleted**. Two incident fixes land with it: a
  **one-owner-per-work-item lock** (`.gogo/resources/cli/locks/<slug>.lock`: PID + tmux
  liveness cross-check via exact `SessionMatchesSlug`, atomic `O_EXCL` acquire,
  **refuse-by-default** / `--takeover` / stale-reclaim, and refusal over a live
  *untracked* board session), and an **extended session registry + reaper** — per-leg
  (`go`|`plan`) `PersistentSession` state/telemetry at `.gogo/resources/cli/sessions/`,
  plus **`gogo sweep`** (kill orphaned/terminal-feature `gogo-*` sessions, `--dry-run`)
  and opportunistic kill-at-ship; the `--attach` path never sets `remain-on-exit` (no
  orphan by construction). `gogo run` becomes a **deprecated alias** for `gogo go`; a
  slug **write-scope guard** (`validSlug`) keeps a `..` slug from escaping
  `.gogo/resources/`. Exit is classified from `state.md` (awaiting-uat → run
  `/gogo:done`; waiting-for-user → parked; `is_error` → halt). The CLI is still a
  deterministic, LLM-free reader that never mutates pipeline state; the frozen contract
  stays additive (§0.15.0). Slash command set unchanged at **13** (`gogo go`/`plan`/`sweep`
  are CLI-binary subcommands); version **0.15.0**.
- **Cockpit redesign — 1b + 1c board restyle (0.18.0):** the `gogo` TUI board
  (`cli/internal/tui/`) is restyled to the Claude-Design **1b + 1c** mockup —
  **presentation-only**, over the **same `contract.Repo`** (no contract change, no new
  pipeline state; the CLI stays a deterministic, LLM-free reader). New per-card + board
  elements: a **header attention summary** (`⏸ K need you` pill when K>0 · `● S session`),
  **status pills** (`pillLabel`/`pillStyleFor` transform; `badge()` stays canonical),
  **phase dots `①②③④⑤`** and a **segmented bar** — both rendering ONE shared
  `phaseProgress(f) [5]phaseState` vector (dots on cards, bar on the strip), a left
  **gate stripe** (heavy `┃` `gateBorder`, red plan/decision · purple uat, focus-independent),
  **underlined column headers** (no `(N)`), a **collapsed changelog** list (`✓ slug … MM-DD`),
  a **contextual footer** (focused card's key-chips + `[?] all keys`), and a top **needs-you
  inbox strip** (`⏸ NEEDS YOU (N)`, one row per `WaitingForInput()` gate; gates ALSO stay in
  their columns — a shortcut, not a move) with **`1..9` number-key answering** (jump-focus +
  read plan/report) and a `?` full-help toggle; the strip **degrades to a one-line summary**
  on a short terminal so the board never overflows (`colAvail` subtracts the strip height).
  Slash command set unchanged; version **0.18.0** (bumped in `plugin.json` + `cli/main.go`).
- **Running-vs-status decoupling + UAT re-plan label (0.19.0):** presentation-only follow-up
  on the redesign. `badge()` no longer treats a live session as `"running"` — session
  liveness is a **separate** signal (the green `●` name-row dot + the header `● N session`
  count), so the status pill always shows the true `state.md` status. A `shipped` card whose
  just-finished `gogo-done-<slug>` host still lingers reads **`shipped`** (not `running`) — the
  documented self-reap limitation becomes cosmetically harmless — and an in-flight card reads
  its phase (`review r2`) beside the `●` dot. `badge`/`pillLabel`/`pillStyleFor` drop the
  now-unused `sessions` param; the dead `pillGreen`/`greenTint` styles are removed. A mid-UAT
  re-plan (`waiting-for-user` carrying a `UAT round N` open-decision) reads **`⏸ re-planning ·
  UAT N`** (a `uat re-plan` gate), distinct from a generic decision fork. No contract/classifier
  change (docs/cli-contract.md §"Changed in 0.19.0"); version **0.19.0**.
- **Lean cockpit cards (0.20.0):** presentation-only follow-up that makes the `gogo` TUI board
  (`cli/internal/tui/`) leaner and more legible — each card says plainly WHAT state it is in and
  WHO is on it, with the left border as the sole "act now" cue. **Drops** the 0.18.0 `⏸ NEEDS YOU
  (N)` inbox strip (and its whole support cast: `renderNeedsYouStrip`/`stripDegraded`/
  `numberedGates`/`stripHeight`, `gates()`/`gateFor`, `stripBoxStyle`/`stripBg`/`waitStyle`), the
  per-card `①②③④⑤` **phase dots** (the `phaseProgress [5]phaseState` vector + `phaseDots`/
  `phaseBar` + `phaseIndex*`/`phaseStyleFor`/`phaseGlyphs`/phase styles), and the **`1..9`
  gate number-key** answering (`jumpToGate`/`gateNumberKey`). **Adds** a green `● <agent>` chip on
  a card's status row — shown **only** when a live session is on it AND it is not a user gate
  (`hasLiveSession && !WaitingForInput`) — via `activeAgent(f)` (phase→agent: plan→analyst ·
  implement→developer · review→reviewer · test→tester · knowledge|report→reporter; `reporter`
  is a display label, no agent file). The header `⏸ K need you` count moves from `len(gates())`
  to a new `needsYouCount()`; the heavy `┃` left border (`stripeAccent`) is unchanged and becomes
  THE gate cue; `colAvail()` = `height - 5` (no strip height). Still presentation-only over the
  **same `contract.Repo`** (no contract/classifier/skill/state change); version **0.20.0**.
- **Per-session attach/kill pickers + changelog live-session dot (0.20.0):** presentation/
  interaction-only follow-up (`cli/internal/tui/`) making lingering pipeline sessions **visible
  and individually actionable**. **FR-1:** collapsed changelog rows show a green `●` before the
  slug when the shipped item has a live session — `changelogRow` gains a `hasSession bool`,
  `renderChangelogColumn` passes `hasLiveSession(slug, m.sessions)`. **FR-2:** `attachFeature`
  (shared by board `a` + drill `a`) branches on the live-session count — `0` hint, `1`
  `attachSession(s)` direct (unchanged UX, sets an `"attaching <session>"` status observable),
  `≥2` `startAttachPicker` → a `huh.NewSelect[string]` (one per session + Cancel). **FR-3:** drill
  `K` branches `1` (the existing `huh.NewConfirm`, unchanged — D2) vs `≥2` `startKillPicker` →
  Select of one per session + `"all N sessions"` + Cancel; `finishKill` resolves targets from
  `binding.selected` (`""`=Confirm-path all-or-none · `killAll` · `killCancel` · else the one
  exact session). Both pickers bind through the heap-stable `*formBinding.selected` (TEST-001) and
  route through `updateForm` (new `pendingAttach → finishAttach` branch; `formPreservesSelection` +
  `cancelForm` cover the picker origin mode + selection preservation). Attribution stays EXACT via
  `launch.SessionMatchesSlug` (TEST-005 — never substring). Picker sentinels
  (`killAll`/`killCancel`/`attachCancel`) are non-empty leading-space consts that can never equal
  a real `gogo-*` session, so `selected == ""` is an unambiguous Confirm-path discriminator. Pure,
  substring-assertable (no TTY); same `contract.Repo`, no state mutation (killing tmux is not a
  state write); version **0.20.0**.

## Custom
<!-- Yours. gogo never rewrites this section: `/gogo:build` re-runs and the report-phase
     reconcile copy it 1:1 (byte-for-byte), exactly like `## gogo overrides`. Put any
     project notes gogo should read but never touch here — safe to edit or delete. -->
