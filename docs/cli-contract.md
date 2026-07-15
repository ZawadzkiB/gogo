---
title: CLI contract
nav_order: 8
---

# CLI contract — the frozen file surface a deterministic reader depends on

This is the **consumer contract** for the files gogo's pipeline writes under
`.gogo/`. It is the surface a **deterministic external reader** — the `gogo` CLI
(the Go/Bubble Tea cockpit, `cli/`) — parses with **no LLM in the read path**:
folder layout, `state.md` line grammar, the work-index classifier, the typed
JSON artifacts and their schemas, the changelog entry shape, and the new
`events.jsonl` telemetry stream.

> **Source of truth is still the code.** The plugin's `skills/*/SKILL.md` and
> `templates/contracts/*` produce these files; this doc *freezes* what a reader
> may rely on. Where this doc and a skill disagree, the skill wins and this doc
> is the bug. See also [Contracts](contracts.md) for the pipeline's internal
> type system (the same schemas, seen from the producer side).

## Stability statement

This is the contract the `gogo` CLI builds against. **It is versioned with the
plugin** (`.claude-plugin/plugin.json` `version`): a breaking change to any file
name, `state.md` line, schema field, or classifier rule below is a plugin
version bump, and the CLI's own `--version` mirrors the plugin version. Additive
changes (a new optional field, a new event kind) are backward-compatible and a
reader must ignore what it does not recognize. **A missing optional file or a
malformed line is degradation, never a crash** — parse defensively.

The `events.schema.json` `additionalProperties: false` is **producer**
self-validation at the current version — it is **not** a check a consumer should
run per line. Forward-compatibility relies on the **consumer** parsing leniently
(skipping unknown/invalid lines and ignoring fields it does not recognize, as Go's
`encoding/json` does), **not** on re-validating each line against this pinned
schema — a strict validation of a future v2 line (new field) would wrongly reject
it and drop a real transition.

## Command surface (enumeration-sync anchor)

The CLI's command surface is defined in `cli/main.go` (the runtime truth) and
enumerated in **four** places that must stay in lockstep — this contract,
`cli/main.go` help, `README.md` `## The gogo CLI`, and the on-demand companion
reference `skills/gogo-cli/SKILL.md`. Any change to the surface updates **all
four**; the `cli` test `TestCLICommandEnumerationInSync` greps them against
`main.go`'s dispatch so a missing or renamed command can't drift silently.

| Command | Read (this contract) vs launch/other |
|---|---|
| `gogo` | opens the interactive board over the `.gogo/` surface (pure read). |
| `gogo go [<slug>]` · `gogo plan <slug>` | persistent-session launch verbs — delegated launches (CLI never mutates pipeline state; see *Changed in 0.15.0*). |
| `gogo sweep [--dry-run] [<slug>...]` | reaps `gogo-*` sessions (whole-board, or targeted to the given slug(s)); CLI-owned bookkeeping only. |
| `gogo status` | prints the work-index classifier table (pure read). |
| `gogo view <target>` | reads a plan/report bundle → terminal or `--web` HTML (pure read). |
| `gogo events <slug>` | reads `events.jsonl` → timeline (pure read). |
| `gogo trash [restore <entry>]` | lists / restores `.gogo/trash/` entries (file moves, recoverable). |
| `gogo run [<slug>]` | DEPRECATED alias for `gogo go`. |
| `gogo --version` | prints the version (mirrors this contract's plugin version). |

### Changed in 0.11.0 (all additive — no key removed or renamed)

The **UAT gate** and the `## Custom` knowledge convention extend the surface without
breaking any existing consumer:

- **`state.md` `status` enum** gains `awaiting-uat` (§2) — where phase ⑤ now leaves a
  feature (was `done`). A pre-0.11 feature still on `done` is unchanged; the classifier
  treats both as **ready-to-ship** (§3), and the four classes/columns are unchanged.
  **Ready-to-ship clarification (§3):** ready-to-ship now gates on a report **and** a
  ship-gate `status` (`awaiting-uat` or legacy `done`) — not report-presence alone — so a
  UAT rerun's **stale** `report/` (status `implementing` / `plan-accepted` /
  `waiting-for-user`) classifies **in-progress**, never ready-to-ship. Additive
  clarification of an existing rule; the four classes/columns are unchanged.
- **`events.jsonl` `event` enum** gains `uat-opened`, `uat-passed`, `uat-failed` (§5) —
  the UAT loop's telemetry. A lenient consumer already skips unknown events, so older
  readers are unaffected.
- **`uat.md`** — a new optional work-folder file (§1), the UAT gate log (prose, not
  schematized).
- **`## Custom` sections in `.gogo/knowledge/*`** — a user-owned, copied-1:1 knowledge
  convention (like `## gogo overrides`); `/gogo:build` and phase ⑤ preserve it verbatim.
  **Not part of the CLI read path** (the CLI does not parse knowledge files) — listed
  here only because it ships in the same versioned plugin bump.
- **`.gogo/trash/`** — a new top-level dir (sibling of `work/` and `changelog/`) the CLI
  **writes** when a board card is deleted (`x`): the work folder moves to
  `.gogo/trash/<compact-ts>-<slug>/` (recoverable — never `rm`). This is the CLI's **one
  write outside `.gogo/resources/`**. `gogo trash` lists it; `gogo trash restore <entry>`
  moves it back to `.gogo/work/feature-<slug>/` (refused if that name already exists). The
  timestamp is a filesystem-safe compact UTC form (`20060102T150405Z`, no `:`), and since
  it carries no `-`, the first `-` in an entry name splits the timestamp from the slug.
  Changelog-column (shipped) cards are the append-only archive and are **never** deletable.
- **Board-launched sessions run in an auto (classifier) permission mode.** A `/gogo:go` or
  `/gogo:done` launch passes `claude --permission-mode auto` (verified against
  `claude --help`) so the skills' safe file steps do not nag inside an unwatched session —
  NOT a full bypass. `GOGO_CLAUDE_PERMISSION_MODE` overrides the value verbatim (any
  `claude --permission-mode` value); the **empty string** omits the flag entirely (claude
  prompts normally). The flag + value are always separate argv elements (a slug never
  reaches a shell). Presentation/launch concern only — not part of the file read contract.
- **Board session-log peek (`l`).** The board's `l` key opens a **read-only** snapshot of a
  card's live session pane (`tmux capture-pane -p`, never an attach — `a` still attaches);
  with no live tmux session it falls back to tailing that card's background `claude -p` log
  under `.gogo/resources/cli/logs/`. Presentation/launch concern only — reads no `.gogo/`
  contract file and writes nothing (like the permission-mode bullet, listed here only
  because it ships in the same versioned plugin bump).
- **Process-orchestrator session registry (`gogo run`).** The CLI process-orchestrator
  (`gogo run`) keeps per-feature bookkeeping — the warm developer session's uuid, the loop
  round, and per-session cost/turns telemetry — at **`.gogo/resources/cli/sessions/<slug>.json`**.
  This is **CLI-owned state, NOT part of the pipeline contract**: it lives under the CLI's
  sanctioned `.gogo/resources/` write root (never in `.gogo/work/feature-*/`, which the CLI
  must not mutate), its shape may change between CLI versions, and a missing or garbled file
  degrades to a fresh run. The orchestrator only *reads* the pipeline's typed contracts
  (`state.md`, `*/issues.json`, `*/result.json`) to route — it writes no pipeline state.

A reader that ignores what it does not recognize (the stability rule above) keeps working
against a 0.11.0 tree with no changes.

### Changed in 0.14.0 (all additive — no key removed or renamed)

The **"waiting for input" signal** and the **board accept action** extend the surface
without touching the file read contract — the status enum (§2), the four classes, and
the class→column mapping (§3) are **unchanged**:

- **"Waiting for input" is a presentation union, not a new state or class.** A CLI may
  read the raw `status` and flag a feature parked at any of the **three genuine user
  gates** — `awaiting-plan-acceptance`, `waiting-for-user`, `awaiting-uat` (the
  `contract.Feature.WaitingForInput()` predicate) — with a distinct card cue (a ⏸ marker,
  including on an `awaiting-plan-acceptance` card, which carried none before) and a
  dedicated **`WAIT`** column in the `gogo status` table. This is the same footing as the
  0.11.0 `awaiting-uat` badge: an **additive, optional presentation concern**; each card
  still sits in its existing phase column and the classifier still emits only the four
  classes.
- **Board column separators.** The board draws a one-cell vertical rule between the four
  columns (plan · in progress · ready · changelog). Layout-only; the four columns and the
  class→column mapping are unchanged.
- **`/gogo:accept` — a new delegated-launch board action.** The board's `m` on an
  `awaiting-plan-acceptance` card now launches `claude "/gogo:accept <slug>"` (the new
  `launch.ActionAccept`) instead of the `/gogo:go` that refuses at that gate — a thin
  session that records acceptance through gogo-plan's existing recording. Like `/gogo:go`
  and `/gogo:done`, it is a **delegated launch: the CLI never mutates pipeline state**;
  the launched session performs the `state.md` write. Presentation/launch concern only —
  no `.gogo/` file-read-contract change.

### Changed in 0.15.0 (all additive — no key removed or renamed; all CLI-owned, not pipeline state)

The CLI's process-orchestrator is reworked from a Go per-phase loop into a **persistent-
session lifecycle manager**: `gogo go <slug>` / `gogo plan <slug>` **launch-or-`--resume`
ONE persistent `claude -p` session** running the existing `/gogo:go` / `/gogo:plan` skill
(implement warm in-context + review/test as nested `Task` subagents + report). The CLI runs
**no phase loop and no routing in Go** — the single routing rule lives in the skill. All of
the following live under the CLI's sanctioned `.gogo/resources/` write root, **never** in
`.gogo/work/feature-*/`, and the `state.md` status enum (§2), the four classes, and the
class→column mapping (§3) are **unchanged**. A reader that ignores what it does not
recognize keeps working against a 0.14.0 tree with no changes.

- **The one-owner lock (`.gogo/resources/cli/locks/<slug>.lock`).** Before launching/resuming,
  `gogo go`/`gogo plan` acquire an exclusive owner lock for the slug (JSON: owner PID, session
  uuid, tmux name if any, host, started-at). "Live" is cross-checked against **both** signal-0
  on the PID **and** a matching live `gogo-*` tmux session (exact `SessionMatchesSlug` parse —
  never substring). A live owner is **refused** by default (or seized with `--takeover`, reaping
  the prior); a **stale** lock (both signals dead) is silently reclaimed. CLI-owned state; the
  lock is released when the invocation's `-p` child exits (headless) or at reap (`--attach`).
- **The extended session registry (`.gogo/resources/cli/sessions/<slug>.json`).** The 0.11.0
  registry gains a `persistent` block keyed by leg kind (`go` | `plan`) recording the persistent
  session's uuid (so a re-launch `--resume`s the SAME warm session), tmux name, last PID, a
  lifecycle status (`running` | `parked` | `awaiting-uat` | `shipped` | `reaped`), timestamps, and
  per-leg cost/turns telemetry. Shape may change between CLI versions; a missing/garbled/legacy
  (`gogo run`, `dev_uuid`) file degrades to a fresh run — never a crash.
- **`gogo go` / `gogo plan` — the persistent-session launch verbs.** `gogo go` enforces the SAME
  acceptance gate `/gogo:go` uses (`plan-accepted` / mid-pipeline; refuses `awaiting-uat` /
  `waiting-for-user` / terminal). On the `-p` child's exit it reads `state.md` and surfaces the
  leg's outcome (`awaiting-uat` → run `/gogo:done`; `waiting-for-user` → the parked gate + resume
  hint; an `is_error` envelope → halt). `--attach` runs an interactive claude in an attachable
  tmux session (no `remain-on-exit`; reaped at close). These are **delegated launches: the CLI
  never mutates pipeline state** — the launched session performs every `state.md`/contract write.
- **`gogo sweep` — the orphan-reaper + kill-at-ship backstop.** Reaps `gogo-*` tmux sessions
  whose owning feature is terminal, and orphans (a live `gogo-*` session with no live, non-terminal
  owning feature), plus a TTL backstop; `--dry-run` lists without killing. `gogo go`/`gogo plan`
  also reap opportunistically when they see the target feature is terminal. Attribution is by exact
  `SessionMatchesSlug` (never substring).
- **`gogo run` is now a deprecated alias** that prints a notice and forwards to `gogo go`.

### Changed in 0.17.0 (all additive — no key removed or renamed; immediate kill-at-ship, D5=B)

Kill-at-ship becomes **immediate** rather than next-sweep/next-launch-only, and the board's
interactive launcher stops leaking dead panes. No command or flag is added; a reader sees
only truthful live-session state sooner.

- **`gogo sweep` gains an optional slug argument (`gogo sweep [<slug>...]`) — targeted mode.**
  With no slug it is the unchanged **whole-board** manual cleanup (orphans + every terminal
  feature + TTL). With one or more slugs it is **targeted**: the reap and the lock/registry
  cleanup are restricted to sessions/features attributing (exact `SessionMatchesSlug` parse)
  to the named slug(s). A slug that fails the kebab-case validator is rejected. Additive — a
  reader that ignores the argument sees the prior behavior.
- **`/gogo:done` reaps its driving session at ship — targeted to the shipped slug(s).** After
  it flips each member's `state.md` to `shipped`, `/gogo:done` runs a **best-effort**
  `gogo sweep <member-slug>...` (guarded on `command -v gogo`; a missing CLI / absent tmux /
  sweep error is silently skipped and the ship still completes). Because the members are
  already terminal, that reaps their `gogo-go-<slug>` / `gogo-plan-<slug>` driving sessions
  immediately — so a just-shipped card shows no phantom "● session running" badge. Passing the
  slug(s) (not a bare `gogo sweep`) keeps a ship from truncating a **different** feature's
  concurrent `/gogo:done`. The whole-board `gogo sweep` / opportunistic next-launch reap
  remains the backstop.
- **`gogo sweep` spares the session it runs in (self-guard).** The sweeper never reaps the
  tmux session hosting it (resolved from `tmux display-message -p '#S'` when `$TMUX` is set),
  so the ship-reap above cannot truncate a `/gogo:done` running inside a board-launched
  `gogo-done-<slug>` session — and `gogo sweep` is safe to invoke from any session context.
  (The shipped card's own `gogo-done-<slug>` host therefore lingers until the user quits it or
  a later sweep — a known limitation of self-reaping. This is now **cosmetically harmless**:
  the board's status pill is decoupled from session liveness, so the shipped card reads
  `shipped` regardless of the lingering pane — never a "running" status. See the
  running-vs-status decoupling in § 2.) All other attribution/TTL/orphan rules (§ 0.15.0) are
  unchanged.
- **The board's interactive `Launch()` no longer sets `remain-on-exit`.** A board-launched
  `gogo-*` session now closes when claude exits (parking at a gate keeps claude — and the
  pane — alive), exactly like the `--attach` / headless `-p` paths, so a finished launch
  leaves no dead pane and `ListSessions()` (the badge source) stays truthful.

### Changed in 0.19.0 (all additive — presentation only; no key removed or renamed)

- **`running` is a session-liveness signal, NOT a status.** The card's status pill always
  shows the true pipeline state from `state.md` (`shipped`, `awaiting-uat`, `review r2`, …);
  whether a `gogo-*` tmux session is live is a **separate** cue — the green `●` name-row dot,
  the header `● N session` count, and the sessions status line. A live session therefore no
  longer masquerades as a status, so a `shipped` card whose just-finished `gogo-done-<slug>`
  host lingers reads `shipped` (not `running`), and an in-flight card reads its phase (e.g.
  `review r2`) with the `●` dot beside its name. Purely how the reader renders existing
  `state.md` + `ListSessions()` data — the status enum (§2) and classes (§3) are unchanged.
- **A mid-UAT re-plan reads as `re-planning · UAT N`.** A `waiting-for-user` card whose
  `open-decision` is `UAT round N` (the re-plan lock, `skills/gogo/SKILL.md`) is labelled a
  `uat re-plan` gate — distinct from a generic decision fork — so the board says the analyst
  is revising the plan rather than looking like a stuck decision. Derived from the already-read
  `status` + `open-decision`; no new state.

### Changed in 0.20.0 (all additive — presentation/interaction only; no key removed or renamed)

- **Lean board.** The board drops the needs-you strip and the per-card phase dots; each card is
  `name (+ ● live dot) · description · status pill [+ ● <agent> chip]`, where the green
  `● <agent>` chip (analyst/developer/reviewer/tester/reporter, from the current phase) shows
  **only** while a live session is actively working the card (not a user gate). The heavy `┃`
  left-border stripe (red gate / purple UAT) is the sole per-card "act now" cue; the header keeps
  the `⏸ K need you` · `● N session` counts.
- **Per-session attach/kill + changelog dot.** The drill's attach (`a`) and kill (`K`) choose a
  SINGLE session via a picker when a card has ≥2 live sessions (`K` also offers "all N"); the
  collapsed changelog shows a `●` on rows whose shipped item still holds a live session. All of
  this reads the SAME file surface + `ListSessions()` — no state-enum, class, or read-path change.

## 1. The `.gogo/` layout a consumer reads

Two roots matter: **work** (one folder per feature, the live pipeline state +
audit trail) and **changelog** (append-only shipped-release history).

### `.gogo/work/feature-<slug>/` — one per feature

`<slug>` is kebab-case with no `feature-` prefix. Files by lifecycle phase —
**guaranteed** ones exist from the moment named; **optional** ones appear only
once that phase has run:

| Path | Meaning | Guaranteed? |
|---|---|---|
| `plan.md` | The accepted plan — the prose contract + the feature's functional requirements. A leading `Status: **accepted** (...)` line once accepted. | **Guaranteed** (from plan ①) |
| `state.md` | Current phase / status / iteration counters / resume hint. The human resume file; its bolded lines are the contract (§2). | **Guaranteed** (from plan ①) |
| `decisions.md` | Open/closed forks that needed the user + gogo's recommendation + the resolution. | **Guaranteed** (from plan ①) |
| `adjustments.md` | Running log of user-requested changes/clarifications during planning. | **Guaranteed** (from plan ①) |
| `uat.md` | The UAT gate log (0.11.0): one round per user check after ⑤ — a `/gogo:done` accept line, or an analyst-authored issues round (verbatim input + analysis + plan delta + disposition + verdict) when feedback loops back. Prose, not schematized. | Optional (from the UAT gate; absent pre-0.11) |
| `charts/` | Plan's intended-design diagrams: `*.mmd` + `manifest.json` + offline `diagrams.html` + `before/` (the plan-time as-is baseline). Implement ② overwrites with the as-built flow/sequence/class/activity set. | Optional (absent for a pure-process feature) |
| `events.jsonl` | Append-only telemetry — one JSON object per line, appended at every phase/status transition (§5). | **Optional** (new in 0.10.0; absent on older features) |
| `review/issues.json` | The living, typed review findings (§4). | Optional (from review ③) |
| `review-NN.md` | Rendered human snapshot of review round `NN` (audit view, not the contract). | Optional (from review ③) |
| `review/result.json` | Per-run review record (§4). | Optional (from review ③) |
| `test/issues.json` | The living, typed test findings (same schema as review). | Optional (from test ④) |
| `test-NN.md` | Rendered human snapshot of test round `NN`. | Optional (from test ④) |
| `test/result.json` | Per-run test record. | Optional (from test ④) |
| `implement/result.json` | Per-run implement record. | Optional (from implement ②) |
| `report/` | As-built bundle from report ⑤: `report.md`, the UML `*.mmd` set, `manifest.json`, `diagrams.html`, `before/` (plan-time set copied in), `result.json`. | Optional (from report ⑤) |
| `pipeline.json` | Feature-level index of what each phase last produced (Stage B orchestration; may be absent). | Optional |

**Reader rules.** Presence signals progress — e.g. `report/report.md` means the
feature is report-complete (§3). Never assume a file exists; treat absence as
"that phase hasn't run." A feature is **report-complete** iff
`report/report.md` (new bundle) or a legacy root `report.md` exists.

### `.gogo/changelog/<YYYY-MM-DD>-<name>/` — append-only shipped history

Written by `/gogo:done`. `<name>` is the **slug** for a single-feature entry or a
**release name** for a merged entry; the date is the newest member's
`completed:`. Shape in §6.

### `.gogo/trash/<compact-ts>-<slug>/` — deleted work (0.11.0, recoverable)

Written by the CLI when a board card is deleted (`x`): the whole
`.gogo/work/feature-<slug>/` folder is **moved** here (never `rm`), so a delete is
reversible. `<compact-ts>` is a filesystem-safe UTC timestamp (`20060102T150405Z`) with no
`:` and no `-`, so the first `-` in the entry name separates the timestamp from the slug.
`gogo trash` lists entries (when, slug, the trashed `state.md`'s phase/status, entry
handle); `gogo trash restore <entry>` moves the folder back to
`.gogo/work/feature-<slug>/` (refused if that name already exists). This is the CLI's only
write outside `.gogo/resources/`. Changelog entries are append-only and never trashed.

## 2. `state.md` line grammar

`state.md` opens with an HTML-comment block (a file-list legend) that a reader
**ignores**. The contract is the set of **bolded key lines**, each exactly:

```
- **<key>:** <value>            <!-- optional trailing HTML comment -->
```

A parser keys on the `- **<key>:**` prefix and takes the value up to end-of-line
or a trailing `<!-- ... -->` comment (trim it). Only these keys are contract;
ignore anything else and tolerate extra bolded lines a future version adds.

| Key | Value | Notes |
|---|---|---|
| `feature` | one-line title | free text |
| `phase` | `plan` \| `implement` \| `review` \| `test` \| `knowledge` \| `done` | the fifth phase is `knowledge` here (skill name); events call it `report` (§5) |
| `status` | `awaiting-plan-acceptance` \| `plan-accepted` \| `implementing` \| `reviewing` \| `testing` \| `waiting-for-user` \| `awaiting-uat` \| `done` \| `shipped` \| `aborted` | mirrors `events.status`; `awaiting-uat` (added 0.11.0) is where ⑤ now leaves a feature — the UAT gate; a legacy `done` predates it |
| `created` | `YYYY-MM-DD` | |
| `completed` | `YYYY-MM-DD` | optional; present on shipped/done features — the source `/gogo:done` reads to date a changelog entry |
| `branch` | git branch \| `n/a` | |
| `iterations` | `plan=N · implement=N · review=N · test=N[ · report=N]` | `·`-separated `key=N` pairs; parse leniently (extra keys, parenthetical notes like `review=2 (APPROVE)` occur) |
| `resume` | `<phase> — <next action>` \| `none` | the human resume hint; free text after the phase token |
| `open-decision` | `<decisions.md anchor>` \| `none` | a trailing parenthetical (`none (D1=A …)`) may summarize resolved forks |
| `stage` | free text (e.g. `A of B`) | optional; multi-stage features only |

Parse defensively: a value may carry a trailing `<!-- … -->` or a `(…)` note;
strip those. `phase`/`status` are the two enums a reader can rely on.

## 3. The work-index classifier → the four board columns

Every `.gogo/work/feature-*/` classifies into exactly one of four classes. This
is the **authoritative table**, quoted verbatim from
`skills/gogo-status/SKILL.md` (the reusable classifier the CLI ports to Go).
**First matching rule wins, top to bottom:**

| Class | Rule |
|---|---|
| **shipped** | `state.md` `status: shipped`, **or** a `.gogo/changelog/*-<slug>/` entry with a `report.md` exists for this slug, **or** this slug appears in any `.gogo/changelog/*/manifest.json` `members` array (a merged release entry named after the release) |
| **ready-to-ship** | not shipped, a final report exists (`report/report.md`, or a legacy root `report.md`), **and** `status` is a ship gate — `awaiting-uat` (0.11.0) **or** a legacy `done` (pre-0.11). A **stale** report left by a UAT rerun (status `implementing` / `plan-accepted` / `waiting-for-user`) does **not** qualify; it falls through to **in-progress** |
| **in-progress** | `phase` is one of `implement` / `review` / `test` (or `status` is `implementing` / `reviewing` / `testing`) — including a UAT rerun re-implementing the same feature **with a stale `report/` still on disk** |
| **unfinished** | anything else — early/`plan` phase, planned but not built, no report |

Notes carried from the classifier: a feature that has a report **and** a matching
changelog entry (by folder slug **or** by `manifest.json` `members`) is
**shipped** (changelog wins over ready-to-ship); an `aborted` feature reports as
**unfinished**. The `members[]` match is essential — a merged entry's folder is
named after the release, so its member slugs are only discoverable through
`manifest.json` `members`.

**Class → board column** (the CLI's four columns):

| Class | Column |
|---|---|
| `unfinished` | **plan** |
| `in-progress` | **in progress** |
| `ready-to-ship` | **ready** |
| `shipped` | **changelog** |

The classifier's in-memory record shape (`slug`, `title`, `phase`, `status`,
`class`, `report_path`, `changelog_path`, `iterations`, `resume`) is documented
in `skills/gogo-status/SKILL.md`; it is computed on demand, not a file on disk.

**`awaiting-uat` → still `ready-to-ship` (0.11.0).** Phase ⑤ now leaves a feature at
`status: awaiting-uat` (the UAT gate) instead of `done`, and such a feature always has a
report — so it classifies **ready-to-ship**; **the four classes and columns are unchanged**
(frozen-contract additive). A CLI may read the raw `status` and paint an **`awaiting-uat`
badge** on a ready card to flag the pending user sign-off — an additive, optional
presentation concern (the 0.11.0 CLI); the classifier still emits only the four classes.

**Ready-to-ship gates on the ship-gate status, not report-presence alone (0.11.0
clarification).** The UAT loop re-runs ②→⑤ on the **same** feature and does not clear the
prior `report/`, so between re-acceptance and the next ⑤ a mid-pipeline feature
(`implementing` / `plan-accepted` / `waiting-for-user`) still carries a **pre-feedback**
report on disk. Ready-to-ship therefore requires a report **and** a ship-gate `status`
(`awaiting-uat`, or a legacy `done`); a stale-report rerun classifies **in-progress** and
is not shippable from the board until ⑤ lands again. This is an **additive clarification**
of an existing rule (report-presence was always meant to signal a *completed* pass), not a
new class — the four classes/columns are unchanged.

## 4. The typed JSON artifacts

Each schema-governed file below is validated by its producer against a JSON
Schema in `templates/contracts/`. A reader may rely on those shapes.

| Artifact | Schema | What it carries |
|---|---|---|
| `review/issues.json`, `test/issues.json` | `templates/contracts/issues-list.schema.json` | `{ slug, track, round, updated?, issues[] }`; each issue has `id, title, description, proposed_solution, severity, priority, status, origin, found_in_round, fixed_in_round?, fix_summary?`. One living file per track, updated in place across rounds. |
| `charts/manifest.json`, `report/manifest.json` | `templates/contracts/charts-manifest.schema.json` | `{ slug, updated?, note?, diagrams[], members? }`; each diagram `{ kind ∈ {flow,sequence,class,activity,use-case}, file (`.mmd`), title }`. A changelog `manifest.json` adds `members[]` (§6). |
| `*/result.json` (`implement`, `review`, `test`, `report`) | `templates/contracts/phase-result.schema.json` | `{ slug, phase, status ∈ {ok,blocked,waiting-for-user}, round?, inputs[], outputs[], validated_in, validated_out, open_issues?, summary }` — the per-run record. |
| `events.jsonl` | `templates/contracts/events.schema.json` | JSON **Lines** telemetry (§5). |

Read the `.mmd` diagram sources directly (they are Mermaid text, not schematized);
the `manifest.json` `diagrams[]` tells a reader each one's `kind` and `title`.

## 5. `events.jsonl` — the live-progress stream

New in 0.10.0. `events.jsonl` is **JSON Lines**: one compact JSON object **per
line**, terminated by a newline — **not** a JSON array; parse it line by line and
**skip a malformed line** rather than failing. Each object conforms to
`templates/contracts/events.schema.json`:

```json
{"ts":"2026-07-03T09:00:00Z","event":"phase-started","phase":"implement","status":"implementing","slug":"cli-cockpit-and-events"}
{"ts":"2026-07-03T10:15:30Z","event":"round-opened","phase":"review","status":"reviewing","round":1,"slug":"cli-cockpit-and-events"}
{"ts":"2026-07-03T10:42:11Z","event":"issues-found","phase":"review","status":"reviewing","round":1,"note":"2 blockers, 1 minor","slug":"cli-cockpit-and-events"}
```

Fields: `ts` (**RFC3339** — a strict ISO-8601 profile, UTC, e.g.
`2026-07-03T14:05:00Z`; pinned to `time.RFC3339` so a Go reader can parse it, and
`format: date-time` in the schema; required), `event` (required enum: `phase-started` ·
`plan-accepted` · `phase-done` · `round-opened` · `issues-found` · `fix-round` ·
`gate-opened` · `gate-resolved` · `uat-opened` · `uat-passed` · `uat-failed` ·
`shipped`), `phase` (required enum: `plan` ·
`implement` · `review` · `test` · `report` · `done`), `status` (required — mirrors
`state.md` status), `round` (optional integer), `note` (optional line), `slug`
(optional — self-describes a copied-out line).

**Emission guarantee — one owner per event.** A line is appended **beside** every
`state.md` phase/status transition — never instead of it (state.md stays the human
resume file). **Each transition is emitted exactly once, by its owning skill:** the
**phase skills** own every phase-lifecycle event (they must — `/gogo:implement`,
`/gogo:review`, … also run standalone), and the **orchestrator owns only the two
gate events**. There is no double emission — no event is written by two owners.

| Event (`event`/`phase`) | Owner | Emitted at (moment) |
|---|---|---|
| `phase-started`/plan | `gogo-plan` | the feature folder + `state.md` are created |
| `plan-accepted`/plan | `gogo-plan` | the user accepts the plan (**terminal** for plan — no `phase-done`/plan) |
| `phase-started`/implement | `gogo-implement` | a plain build run sets `state.md`→implementing |
| `fix-round`/implement | `gogo-implement` | a `--issues` re-entry to fix findings (+`round`) |
| `phase-done`/implement | `gogo-implement` | `implement/result.json` is written `ok` (this run hands off to review) |
| `round-opened`/review | `gogo-review` | review round `NN` opens (+`round`) |
| `issues-found`/review | `gogo-review` | that round has `open`/`new` findings (count in `note`) |
| `phase-done`/review | `gogo-review` | a round ends **clean** (advancing to test) |
| `round-opened`/test | `gogo-test` | test round `NN` opens (+`round`) |
| `issues-found`/test | `gogo-test` | that round has `open`/`new` findings |
| `phase-done`/test | `gogo-test` | the feature is **all-green** (advancing to report) |
| `phase-started`/report | `gogo-knowledge` (⑤) | report ⑤ begins |
| `phase-done`/report | `gogo-knowledge` (⑤) | the report bundle is written + `state.md` set |
| `gate-opened` | `gogo` (orchestrator) | a decision gate opens (`waiting-for-user`) |
| `gate-resolved` | `gogo` (orchestrator) | the user answers and the phase resumes |
| `uat-opened` | `gogo` (orchestrator) | the user routes awaiting-uat feedback to the analyst (the UAT loop opens; `phase: report`) |
| `uat-failed` | `gogo` (orchestrator) | a re-planned UAT round is re-accepted and `/gogo:go` is about to rerun ②→⑤ (round summary in `note`; `phase: report`) |
| `uat-passed` | `gogo-done` | the UAT gate is accepted by `/gogo:done`, emitted just before `shipped` (`phase: done`) |
| `shipped`/done | `gogo-done` | a member's changelog entry is archived (**terminal** for done — no `phase-done`/done; changelog path / members in `note`) |

The two gate events carry the *resume* phase in `phase`, mapped to the **events**
vocabulary: a gate opened during the fifth phase emits `report`, never `knowledge`
(the events `phase` enum has no `knowledge`).

**Reader rules.** Telemetry is **best-effort**: an emitter never fails its phase
if the append fails, so the stream may have gaps. A **missing** `events.jsonl` is
never an error (older features predate the contract) — fall back to `state.md`
for the current phase; `events.jsonl` adds only the *timeline and rounds*
state.md cannot carry. `ts` gives ordering; the last event is the most recent
transition. Note the `knowledge` (state.md) vs `report` (events) naming for the
fifth phase.

**Producer discipline for `ts`.** An emitter must stamp `ts` with the **real
current UTC time at the moment of emission** (`date -u +%Y-%m-%dT%H:%M:%SZ`) —
never an estimated, rounded, or back-dated time — so a feature's stream stays
**monotonic in append order**. The file is **append-only**: never rewrite a
historical line. Because emission is best-effort a reader must not *assume*
monotonicity — if `ts` is ever non-monotonic, **file (append) order is
authoritative**, not `ts` sort order.

## 6. Changelog entry shape

A `/gogo:done` entry under `.gogo/changelog/<YYYY-MM-DD>-<name>/` is a **slim,
high-level synthesis**, never a copy of the work `report/` bundle:

| Item | Notes |
|---|---|
| `report.md` | A **synthesized** high-level entry (what shipped, key outcomes, one-line decisions, review/test verdict; a member table + per-member section when merged) with links back to each `.gogo/work/feature-<slug>/`. Written, never `cp`'d. |
| `<slug>-<name>.mmd` | The diagram set, **slug-prefixed** so a merged entry keeps a flat layout (a single entry is the same shape with one member). |
| `manifest.json` | `charts-manifest.schema.json` shape with a **`members[]`** array — `[<slug>]` for a single entry, `[slug1, slug2, …]` for a merged release. `members[]` is how the classifier (§3) resolves a merged entry's members to **shipped**. |
| `before/<slug>-<name>.mmd` | Optional — the plan-time "before" set, merged and slug-prefixed, for the viewer's before/after compare. |

**`members[]` only since 0.8.0.** A changelog `manifest.json` is *guaranteed* to
carry `members[]` only for entries written by the current `/gogo:done` (0.8.0+).
**All entries currently on disk predate the writer and omit it** — so a consumer
must **not** assume `members[]` is present. When it is absent, fall back to the
**folder-name slug match** from §3 (the classifier already does exactly this): a
single-feature entry's folder is `<date>-<slug>`, so the member slug is recoverable
from the path. (Symmetric with the `diagrams.html` caveat below.)

**No `diagrams.html`.** Current entries deliberately drop the static viewer page
(`/gogo:view` builds the interactive page from `report.md` + the `.mmd` set on
demand). A reader must **not** depend on `diagrams.html` in a changelog entry
(older, pre-0.8.0 entries on disk may still carry one — ignore it).
