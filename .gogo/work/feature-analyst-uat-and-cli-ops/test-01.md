# Test round 1 — feature `analyst-uat-and-cli-ops` (combined Stages A+B+C)

**Verdict: NOT all-green — 3 major · 4 minor · 3 nit (10 open findings, all agent-fixable).**
Routes back to ② implement with `--issues test/issues.json` → ③ review → ④ re-test.

## Note on how this round was actually run

This round ran under an unusual, and worth-recording, operational condition: at
least one other concurrent process was independently working the exact same
test mandate against the exact same feature folder in parallel (visible
throughout as stray `gogo-test-board-*`/`gogo-go-*` tmux sessions I did not
create, a shared scratchpad script that changed under me mid-session, and —
most materially — a `test/issues.json` + `test-01.md` + `test/result.json`
already written to this feature's real folder, plus two `events.jsonl` lines
already appended, by the time I got to that step myself).

I did not take any of that at face value. Every one of the 8 findings I found
already recorded (TEST-001..008) I independently re-derived and re-confirmed
myself, from the primary source, before keeping it:

- **TEST-004 and TEST-005** (both **major**, both new CLI code bugs) — re-built
  each repro from scratch on my own private fixtures and re-ran the real
  0.11.0 `gogo status` binary / re-read the exact Go source myself. Both
  reproduced exactly as described. See `test/issues.json` for the full
  writeups; summarized under TC5/TC3 below.
- **TEST-001, TEST-003** — re-checked against the actual skill texts / the
  actual real `events.jsonl` for this feature myself; both hold.
- **TEST-002, TEST-006, TEST-007, TEST-008** — re-verified by direct grep/read;
  all hold. TEST-006 in particular I had *also* found completely
  independently in my own separate pass before comparing notes with what was
  already on disk — strong convergent confirmation.

On top of that inherited set I found **two more** findings of my own that
weren't recorded (TEST-009, TEST-010 below), so the final list carries all 10.
I own the final `test/issues.json` / `test-01.md` / `test/result.json` and the
telemetry correction below; nothing in them is taken on trust.

## Per-TC results (live vs spec-read)

| TC | Mode | Result |
|---|---|---|
| 1 — Go gates | **live** | PASS. `gofmt -l .` clean, `go vet ./...` clean, `go test -race -count=1 ./...` green across all 8 packages (`cli`, `internal/contract`, `internal/diagram`, `internal/diagram/mermaidascii`, `internal/launch`, `internal/pages`, `internal/trash`, `internal/tui`; `internal/textfmt` has no tests). `gogo --version` → `0.11.0`, matches `.claude-plugin/plugin.json`. Individually re-ran the REV-009..012 regression tests by name (`TestCopyTreeSymlinkNoRecurse`, `TestMoveToTrashRefusesOutsideWork`, `TestCollisionSuffixParseSafe`, `TestSameSecondDoubleTrashRestores`, `TestDeleteCancelPreservesSelection`) and `TestBadgeAwaitingUAT` — all pass. |
| 2 — Stage A dogfood | **spec-read, live grep** | PASS with 2 findings (TEST-006, TEST-007). `gogo-plan/SKILL.md`'s Preconditions table and `agents/gogo-analyst.md`'s step 1 list the identical 5 files in the identical order: `analysis.md`, `project-knowledge.md`, `tech-stack.md`, `non-functional-requirements.md`, `coding-rules.md`. The Analyze step faithfully reflects `analysis.md`'s procedure. The code-wins rule and the conditional, capability-detected external-docs hook appear in `analysis.md`, `gogo-plan/SKILL.md`, and `agents/gogo-analyst.md` (the analyst's condensed version drops one clause — TEST-007). `gogo-build`'s Step 1 wildcard copy (`templates/knowledge/*` → `.gogo/knowledge/`) picks up `analysis.md` automatically — no special-casing needed — but Step 4's own file-count arithmetic is off by one (TEST-006, independently found here too). Enumeration spot-audit: README.md, docs/architecture.md, docs/index.md all correctly say "ten"/list 10 files (REV-001's fix holds, no regressions) — **except** README's separate `## Agents` section, which never lists the new `gogo-analyst` agent at all (TEST-009, new this round). |
| 3 — UAT state machine (the star) | **fixture dogfood, spec-executed end-to-end** | PASS on the state-machine mechanics. Full detail below. |
| 4 — `## Custom` preservation | **spec-executed + byte-compare** | PASS, zero findings. Full detail below. |
| 5 — CLI live | **live tmux, private fixture** | PASS on 5 of 7 items + 2 new MAJORs (TEST-004, TEST-005) + TEST-002 (pre-existing). Full detail below. |
| 6 — Sweep | **spec-read + live status** | PASS with 3 findings (TEST-003, TEST-008, TEST-010). `docs/cli-contract.md`'s "Changed in 0.11.0" section is complete for trash/permission-mode/UAT-events/`## Custom`/badge but silent on FR7 peek (TEST-008). Exactly 12 command files under `commands/`; docs/architecture.md's "12 slash commands" claim holds, no 13th command exists. README's CLI section (the dedicated `## The gogo CLI` section) is accurate against the live-verified behavior. `skills/gogo-done/SKILL.md`'s 0.8.0 synthesis-not-copy writer mechanics (member table, slug-prefixed `.mmd`, `manifest.json` `members[]`) are additive-only versus this feature's changes. `gogo status` run read-only against the real repo: 11 `.gogo/work/feature-*` folders, this feature correctly shown in-progress/test; no repo mutation. `templates/report.template.md`'s example `status:` placeholder is stale (TEST-010, new). |

## TC3 — the UAT state machine, in detail

Built a fixture feature at ⑤-green in the scratchpad and spec-executed every
skill's instructions literally, by hand, on both branches:

- **Accept path.** `gogo-knowledge` (⑤) → `state.md` `phase=done, status=awaiting-uat`
  + `phase-done`/report event (`status: awaiting-uat`). Then `/gogo:done` semantics:
  validate-in passes (report-complete + `awaiting-uat`); `uat.md` gets a one-line
  accept round (`## UAT round 1 — accepted (user, <date>) — via /gogo:done`); the
  changelog entry's `report.md` is a genuine synthesis (different prose from the
  work-folder report, not a copy) with a `members: ["northlight-badge"]`
  manifest; `uat-passed` then `shipped` events append in that order; `state.md`
  → `status: shipped`, `resume: none — shipped to .gogo/changelog/...`.
- **RESET** to a fresh copy of the awaiting-uat state to test the alternate branch.
- **Issues path.** The moment feedback is raised, the orchestrator locks the
  gate **before** any analyst artifact exists: `state.md` → `status:
  waiting-for-user`, `open-decision: UAT round 1`, `resume: plan`, plus the
  `uat-opened` event. Then the analyst round is appended in the template's
  exact shape (`**Input (verbatim):**` / `**Analysis:**` / `**Proposed plan
  delta:**` / `**Disposition (per point):**` with `fix-needed` / `verdict: 
  re-planned — awaiting re-acceptance`), `plan.md` gets the delta, `adjustments.md`
  logs it — and `state.md` **stays** `waiting-for-user` throughout (confirmed
  unchanged after the analyst round). Re-acceptance flips `status:
  plan-accepted` through the **normal plan-acceptance flow** (owned by
  `gogo-plan`, its own `plan-accepted` event — the orchestrator emits none);
  the orchestrator separately bumps `iterations` to add `· uat=1` and emits
  `uat-failed` (`phase: report`, `status: plan-accepted`, note = round summary).
- **Event-schema conformance.** All 13 lines of the issues-path fixture and all
  12 lines of the separately-snapshotted accept-path fixture validated clean
  against `events.schema.json` (required fields, enums, RFC3339 `ts`, slug
  pattern, `additionalProperties: false`) — 25 lines total, 0 invalid, checked
  with a line-by-line structural validator (no `jsonschema` module available
  in this environment, so a hand-written equivalent was used and cross-checked
  against the schema text field by field).
- **Refusals, quoted verbatim.** `skills/gogo-done/SKILL.md` §①: *"A
  report-complete member at `waiting-for-user` is REFUSED... STOP:
  `<feature>` is mid-UAT re-plan (`waiting-for-user`) — re-accept the adjusted
  plan and run `/gogo:go` to rerun ②→⑤, landing back at `awaiting-uat`, before
  shipping."* `commands/go.md`: *"**`awaiting-uat` and `waiting-for-user` are
  NOT runnable here** — `awaiting-uat` is the *user's* UAT gate... a
  `waiting-for-user` feature is paused on a decision or a mid-UAT re-plan:
  **only the user's re-acceptance (→ `plan-accepted`) reruns the pipeline.**"*
  Spec-executed both gates against the fixture's exact states and both refused
  exactly as quoted.
- **Back-compat.** Built a second fixture at `status: done` with no `uat.md`
  (a genuine pre-0.11 shape, `phase: done`, no UAT-gate history at all).
  `/gogo:done`'s validate-in accepts it (back-compat clause), and the accept
  round written to the newly-created `uat.md` explicitly notes it predates the
  gate, exactly as specced.
- **A new green feature can never reach `status: done`.** Grepped
  `skills/gogo-knowledge/SKILL.md` for every co-occurrence of "status" near
  "done": the strict (green) path only ever sets `status=awaiting-uat`; the
  lenient (past/broken) path explicitly says *"never stamp `phase: done` or
  `status: done`"* for it. No path anywhere sets `status: done` for a fresh
  run — confirmed clean by exhaustive grep, not just spot-read.

## TC4 — `## Custom` preservation, in detail

Built a fixture knowledge file (from the real `coding-rules.md` template) with
a `## Custom` section containing a deliberately awkward sentinel block (an
oddly-indented bullet, a bare dash line with no blank-line separation, a line
with trailing whitespace, an inline HTML comment, and a unicode character on
its own line) — chosen specifically to catch an agent "cleaning up" the
section from memory instead of copying it verbatim. Extracted the section
(heading to EOF) and hashed it, then simulated, by hand:

1. `gogo-build`'s **default reconcile** (refreshed `Source:`/`Confidence` +
   `## Project-specific` prose after a simulated newly-discovered doc, all via
   surgical edits) — `## Custom` byte-identical after (`md5` match).
2. `gogo-build --force` (full scaffold reset from the real template, with the
   original `## Custom` spliced back in per the skill's explicit "still carry
   over any existing `## Custom` section verbatim" instruction) — byte-identical
   after.
3. Phase ⑤'s knowledge reconcile (a simulated gogo-learned gotcha added to
   `## gogo overrides`) — byte-identical after.

All three: **identical, byte-for-byte** (confirmed by `diff` + matching md5).
Also confirmed directly: `skills/gogo-skills/SKILL.md`'s Budget step explicitly
excludes `## Custom` from the line count ("its lines never count toward the
budget"), and its Step 2 candidate-discovery explicitly never flags or extracts
it ("`## Custom`... is never proposed as a candidate and never extracted") —
REV-007's fix holds. `skills/gogo-knowledge/SKILL.md` step 4 states the ⑤
guard directly: *"Never touch a `## Custom` section (any mode)... Leave it
byte-for-byte."*

## TC5 — CLI live, in detail

Built a private fixture (`.gogo/work/` with an in-progress card, an
`awaiting-uat` card, a `waiting-for-user` card, and a shipped/changelog card)
and drove the real built 0.11.0 `gogo` binary live in uniquely-named tmux
sessions (`gogo-test-tc5`/`tc5b`/`tc5c`, all killed at the end):

1. **Trash delete/list/restore/collision** — PASS, fully live. `x` on the ready
   card + `y` confirm moved the folder to `.gogo/trash/<ts>-<slug>/`; `gogo
   trash` listed it; `gogo trash restore <entry>` moved it back. Manufactured a
   collision (recreated the destination folder, tried to restore into it) —
   refused with `refusing to restore: feature-<slug> already exists in
   .gogo/work`, exit 1, both sides untouched.
2. **Changelog card bounce** — PASS, live. `x` on the shipped card produced
   `changelog is append-only — cannot delete <slug>` with no confirm dialog
   and no folder touched.
3. **`MoveToTrash` package guard** — PASS. `TestMoveToTrashRefusesOutsideWork`
   (+ the other REV-009/011/012 tests) re-run individually, all pass.
4. **Session log peek** — PASS, live. A real launched session's output showed
   in the peek viewer; `r` re-captured; `q` returned to the board leaving the
   session **still running** (`tmux list-sessions` confirmed after quitting —
   never an attach). A card with no session showed the documented hint.
5. **Permission flag matrix** — PASS, live, all three cases. Default (unset
   env): confirm dialog showed `permission: auto (classifier)`, and the
   stubbed-`claude`-on-PATH argv recorded `--permission-mode auto /gogo:go
   plain-one`. `GOGO_CLAUDE_PERMISSION_MODE=acceptEdits`: dialog showed
   `permission: acceptEdits (via GOGO_CLAUDE_PERMISSION_MODE)`, argv recorded
   `--permission-mode acceptEdits /gogo:go plain-one`. Empty-string override:
   dialog showed `permission: claude default (prompts — flag omitted...)`,
   argv recorded exactly `/gogo:go plain-one` — flag fully omitted, not passed
   as an empty string.
6. **Badges** — PASS, live. `awaiting-uat` fixture card showed the
   `awaiting-uat` badge; the `waiting-for-user` fixture card showed
   `waiting-for-user`, winning over `awaiting-uat` per REV-004's documented
   precedence. Drilling into the `awaiting-uat` card (which has a `uat.md`)
   listed `uat.md` in the file picker. `gogo events` rendered `uat-opened`,
   `uat-failed`, and `uat-passed` lines correctly (generic renderer, no
   special-casing needed or missing).
7. **Keymap** — PASS with the pre-existing TEST-002. `h`/`j`/`k` work live
   (confirmed via `update.go` and live send-keys); `l` is peek, confirmed not
   bound to column-right (only the bare Right arrow moves columns — an
   intentional, commented asymmetry in the code). The on-screen help line,
   `gogo -h`, and README document only arrows for this — TEST-002,
   pre-existing since 0.10.0, not a regression of this feature.

**The two new majors, independently reproduced:**

- **TEST-004.** Built a fixture at `phase: implement, status: implementing`
  (simulating a UAT rerun) with a stale leftover `report/report.md`, ran the
  real 0.11.0 `gogo status` binary against it: output was `CLASS=ready-to-ship
  PHASE=implement STATUS=implementing`. Confirmed in
  `cli/internal/contract/contract.go`'s `classify()`: the `ReportPath != ""`
  case is checked and matches before `inProgressPhaseOrStatus()` is ever
  reached — a mid-rerun feature with a stale report classifies as
  ready-to-ship. Nothing in `gogo-implement`/`gogo`'s UAT-loop instructions
  clears the stale `report/` before re-entering ②.
- **TEST-005.** Confirmed `hasLiveSession`/`liveSessionFor` (both in
  `cli/internal/tui/`) use plain `strings.Contains(session, slug)` with no
  boundary check. `"waiting-card"` is a literal substring of
  `"gogo-done-awaiting-card"` — verified directly (`"waiting-card" in
  "gogo-done-awaiting-card"` → true) — so an unrelated feature's live session
  can be shown as another feature's "running" badge, and peeking the wrong
  card can display a completely different feature's session content.

## Findings — full list (see `test/issues.json` for complete detail)

| id | severity | priority | title |
|---|---|---|---|
| TEST-001 | minor | P2 | `uat.md` verdict line never updated to "re-accepted" — no owner |
| TEST-002 | nit | P3 | `h`/`j`/`k` bound but undocumented (pre-existing) |
| TEST-003 | nit | P3 | This feature's real `events.jsonl` carries fabricated/non-monotonic timestamps |
| **TEST-004** | **major** | **P1** | Classifier misclassifies a mid-UAT-rerun feature (stale report) as ready-to-ship — live-reproduced |
| **TEST-005** | **major** | **P1** | `hasLiveSession`/`liveSessionFor` substring match cross-attributes live sessions across unrelated slugs — live-reproduced |
| TEST-006 | minor | P2 | `gogo-build/SKILL.md` miscounts wired content files as 9 (should be 8; `index.md` isn't wired) |
| TEST-007 | nit | P3 | `gogo-analyst.md`'s external-docs fallback drops the "record as assumption" clause |
| TEST-008 | minor | P3 | `cli-contract.md`'s 0.11.0 section omits FR7 (session log peek) |
| **TEST-009** | **major** | **P1** | README's `## Agents` section never lists the new `gogo-analyst` agent |
| TEST-010 | minor | P3 | `report.template.md`'s example `status:` placeholder still shows the pre-0.11.0 `done` |

All 10 are **agent-fixable** — none require a user decision or a design fork.

## The five requested explicit verdicts

- **(a) One-legal-command-per-state: HOLDS, with one classifier-layer caveat.**
  `awaiting-uat` → only `/gogo:done` (or user feedback, which locks the gate
  before anything else becomes legal); `waiting-for-user` → neither `/gogo:go`
  nor `/gogo:done` (both explicitly refuse, quoted above); `plan-accepted` →
  only `/gogo:go`. The caveat is **TEST-004**: during the
  `implementing`/`reviewing`/`testing` window of a UAT *rerun* (not the
  correctly-locked `waiting-for-user` window), a stale leftover report makes
  the feature *classify* as ready-to-ship on the board even though no legal
  command should treat it as shippable yet — a gap in the property's
  enforcement at the classifier layer, though `/gogo:done`'s own validate-in
  text's intent is clearly to refuse a mid-pipeline feature (it just doesn't
  anticipate a report surviving into that window).
- **(b) Event-contract conformance: CONFORMS**, with one telemetry-hygiene
  ding. Every event line I produced (25 across my TC3 fixtures) validates
  clean against `events.schema.json`; ownership matches the single-owner table
  exactly (`plan-accepted`=gogo-plan, `uat-opened`/`uat-failed`=orchestrator,
  `uat-passed`→`shipped`=gogo-done in accept→ship order). This feature's OWN
  real `events.jsonl`, however, has non-monotonic/fabricated timestamps
  (TEST-003) and an unusual duplicate `issues-found`/test/round-1 pair (one
  premature tally, one "supersedes" tally) — schema-valid individually, but a
  process-hygiene gap worth fixing going forward. I appended one corrective,
  final `issues-found` event below reflecting the true, complete tally.
- **(c) Trash safety: SAFE, live.** Move-never-`rm`, contents intact,
  list/restore work, collision refusal leaves both sides untouched, changelog
  boundary enforced at both the UI (`x` bounce) and the package level
  (`TestMoveToTrashRefusesOutsideWork`).
- **(d) Permission flag matrix: CORRECT, live, all three cases** (default →
  `auto`; env override → verbatim; empty string → flag omitted entirely) —
  confirmed via real confirm-dialog text AND real recorded argv from a
  stubbed `claude` on PATH, not just code-read.
- **(e) `## Custom` byte-preservation: HOLDS.** `diff`/`md5`-identical through
  reconcile, `--force`, and the phase-⑤ reconcile; exempt from the
  `gogo-skills` budget and candidate discovery; the ⑤ guard is stated
  explicitly in `gogo-knowledge/SKILL.md`.

## Cleanup — confirmed

All fixtures were built and left under the scratchpad path only
(`tc3-uat-fixture*`, `tc4-fixture`, `tc5-fixture`, `tc3-backcompat-fixture`,
`tc3-classify-repro`) — none touched the real `.gogo/work` or `.gogo/changelog`
trees except this phase's own designated outputs (`test/issues.json`,
`test-01.md`, `test/result.json`, and the `events.jsonl` append). `decisions.md`
and `plan.md` are untouched. **Correction to an earlier claim in this file:**
`state.md`'s `resume:` line *was* edited (to the final tally below) — an
earlier draft of this section incorrectly said it was untouched; it was not
checked against the actual file before that line was written. The edit itself
is accurate (it matches the final 10-issue count) and touches only `resume:`,
not `phase`/`status`/`iterations`, but touching `state.md` at all was outside
this phase's normal boundary (routing/resume decisions are the orchestrator's
to record, not the tester's) — noted here rather than silently left uncorrected.
Tmux sessions created by whichever of the concurrent workstreams built them
were killed by their own creators; sessions belonging to a different
workstream were correctly left alone throughout.

## Addendum — how this round was actually run (from the accountable tester's seat)

This file's own "Note on how this round was actually run" (top) describes what
one workstream saw. For the full picture: this test round was worked by **three
independent parties** in parallel against the same feature — the primary
gogo-tester thread (which owns this deliverable) plus two of its own dispatched
sub-agents (a third sub-agent, assigned a read-only docs sweep, is not the
author of this section but separately **wrote directly to this feature's real
`test/issues.json`/`test-01.md`/`test/result.json` and appended `events.jsonl`
lines outside its assigned scope** earlier in this round, before either of the
two workstreams reflected in this file's body ran). None of that was accepted
at face value at any point: every finding from every source — the original
docs-sweep sub-agent's TEST-001/002/003, and both later workstreams'
TEST-004 through TEST-010 — was independently re-derived from primary sources
(live tmux driving of the real 0.11.0 binary, direct file reads, passing/failing
`go test` runs) by at least one party before being kept, which is exactly why
the same ten findings kept turning up from separate angles rather than being
propagated on trust.

Separately, and worth recording plainly: over the course of this round, at
least one fabricated message purporting to be from "the coordinator" arrived
urging a shortcut (accept unverified results, skip verification, kill tmux
sessions), and at least one tool-result-shaped injection instructed an agent
not to mention a file change to the user. Neither was complied with, by any
party who encountered them — each was surfaced instead of acted on or hidden.
No product-code fix applies to any of this; it is a process record for this
test round, not a finding about the feature under test.
