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
| `status` | `awaiting-plan-acceptance` \| `plan-accepted` \| `implementing` \| `reviewing` \| `testing` \| `waiting-for-user` \| `done` \| `shipped` \| `aborted` | mirrors `events.status` |
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
| **ready-to-ship** | not shipped, **and** a final report exists (`report/report.md`, or a legacy root `report.md`) |
| **in-progress** | no report, **and** `phase` is one of `implement` / `review` / `test` (or `status` is `implementing` / `reviewing` / `testing`) |
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
`gate-opened` · `gate-resolved` · `shipped`), `phase` (required enum: `plan` ·
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
