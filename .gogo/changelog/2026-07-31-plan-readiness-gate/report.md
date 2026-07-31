# plan-readiness-gate - 0.29.0 (2026-07-31)

The board **narrated the past**. `state.md` was written at each phase's *exit*, so it was a
completion log, not an occupancy record - and that one root cause had two very different
faces. Cosmetically, a work item scaffolded from the template was **born** reading
`awaiting-plan-acceptance` with no `plan.md` on disk, so the board offered an unwritten plan
for acceptance and rendered the template's literal `<one-line title>` as the card's name.
Structurally - and this was the real defect - an item **mid-build** still read
`plan-accepted`, which failed `cap.go`'s class test and made it **invisible to the
per-source concurrency cap**. A second build could start in the same repo and clobber a
shared working tree, which is the exact thing the cap exists to prevent.

0.29.0 makes readiness **derived at read time** (no new status enum, no frozen-contract
change), removes the cap's class filter so a live build counts regardless of what the file
claims, and adds a `· state lags` cue for when `state.md` and `events.jsonl` disagree.

Review **APPROVE** (7 rounds, 36 findings). Test **PASS** (4 rounds, hands-on in real tmux).

## What changed

- **Plan readiness is derived, not declared.** `Feature.PlanUnwritten` (defect-positive, so
  a zero-value Feature keeps its pre-0.29 meaning byte-for-byte), `PlanSections`,
  `planWritten` (absent = unwritten, **unreadable = written** - never invent a defect), and
  `PlanUnwrittenReason` as the **one** clause every refusal quotes, so the board bounce, the
  CLI, the drill note and the footer chip cannot drift into three diagnoses of one fact.
- **The board refuses what it cannot honour.** `m` and `M` both bounce on a card whose plan
  is not written, and on a card paused at a decision gate. Both guards sit **outside** every
  `!force` condition - `M` overrides the cap and only the cap.
- **The cap counts reality.** `cap.go`'s `Class != ClassInProgress` filter is gone;
  `liveBuildSession` requires an `ActionGo` session, so an authoring or planning session
  never consumes a slot while a real build always does.
- **`· state lags`** - a deterministic cue when the newest event disagrees with the phase
  line. It fired on **this very work item** during its own review rounds.
- **Template placeholders never render.** `stripPlaceholder` blanks `<one-line title>`-style
  values, and `parseStateFile` became **block-comment aware** - the template's commented-out
  `correlation:` legend was being parsed as real data and painting a bogus `⛓ ×3` chip.
  That chip was in the user's original bug report, and an earlier analysis had
  mis-attributed it to the header's `⏸ N need you` pill.
- **One producer, many surfaces.** `selectableForShip` (the ship selection), `moveChip` (the
  footer's `m` chip) and `CapRuleClause` each replace two or more hand-kept copies of one
  rule. Every copy that existed had already drifted.

## Key outcomes

- **The working-tree clobber is closed at the reader.** A safety property no longer depends
  on an LLM writing a file on time - which matters, because the writer half provably does not.
- **A stale ship selection cannot resurrect.** It is pruned on every rebuild, not filtered at
  the read: filtering stopped it shipping but left it armed and invisible.
- **`gogo plan go` and the board agree.** Not-runnable statuses refuse on both surfaces, and
  the refusal names the legal move instead of refusing bare.

## Decisions (one-liners)

- **D1 = A** - derive readiness at read time; no new `status` enum value, no frozen-contract change.
- **D2 = A** - "written" means >= 2 `## ` headings (measured: the smallest real plan is 5,494 bytes with 8).
- **D3 = A** - `m` on an authoring card **bounces**; it does not resume authoring.
- **D4 = D** - fix the writer **and** add the reader-side cue.
- **D5 = A** - **drop the cap's `Class` filter.** The rule was always "a live build session"; the filter contradicted it.
- **D6 = A** - cue the disagreement, do not move the card's column.

## What we got wrong, and caught

Recorded because the corrections are the useful part:

- **FR11's prose half never worked.** Instructing the phase skills to write `state.md` at
  *entry* was skipped on **all three** of its live runs. It ships as *a hypothesis that has
  not yet paid off*; the release claim rests on the deterministic cue instead.
- **FR11 also caused a regression nobody asked for.** It dropped the phase/status **exit**
  write, so a skipped entry write left the line stale **indefinitely** rather than one phase
  late. Phase ⑤ caught it. The exit write is restored and both writers are kept - the
  redundancy is the design, because one of the two is an LLM following prose.
- **We briefly recommended a command that destroys other people's work.** The cap bounce
  told the user to run a bare `gogo sweep`, which is host-global: on a multi-source host it
  classifies another source's **in-flight build** as an orphan and kills it, unconfirmed.
  Now only the targeted `gogo sweep <slug>` can be emitted. *(Reasoned from the code and
  from `Sweeper`'s own contract - deliberately never executed against a live host.)*
- **We told an aborted feature it had "already shipped."** The board had been aligned to a
  `runnableHint` arm that `cmdGo` never reaches, so the wording matched dead code.
- **A CI blocker was caught before tagging.** Three new tests needed `claude` on PATH; the
  release workflow's `go test -race ./...` on a clean runner would have failed *after* the
  tag was pushed. A **hermetic** run is now a standing gate.

## Process finding

The `gogo` skill bounds implement↔review at **~3 rounds**. This ran to **7**. Rounds 1-4
found genuine defects; rounds 5-7 largely fixed defects introduced by earlier fixes,
including several by the orchestrator running ② in-context. The bound exists for a reason
and should have been treated as a stop-and-escalate signal rather than a suggestion.

## Method: fourteen ways a test lies

`.gogo/knowledge/test-strategy.md` now enumerates **fourteen** distinct variants of "an
assertion that looks like a check and isn't", found across 0.28.0 and 0.29.0 - several of
them *inside guards written to close an earlier one*. Among them: a mutation that does not
compile (`go build` does not type-check `_test.go` - use `go vet`); an insertion whose
replacement contains its anchor, defeating a naive landed-edit check; a structural guard
matching its own comment; two lipgloss styles that render identically under a TTY-less
terminal; an exclusivity assertion true **vacuously**, needing a reachability guard; a
guard-only mutation that can never fail while production code is correct, needing a
**two-part** mutation; a guard satisfied by its own producer's body; an anchor written from
memory that never lands and reports a false PASS; a shell `&&` after a pipe masking the exit
code; and aligning a message to an arm that an earlier `return` makes unreachable.

## Known limitations (carried forward)

- **FR11's entry write is advisory** (n=3, skipped every time). The exit write is what makes
  the line move; the cue is what makes a disagreement visible.
- **The cue's meaning is narrower than "the phase line is stale."** Arm B is ambiguous by
  construction: it reads *"`state.md` and `events.jsonl` disagree; one half of step 1 did not
  land"*. And **silence is not proof of health** - a later mid-phase event overwrites the
  `phase-done` arm A keys on.
- **A mid-write `state.md` can drop a legitimate ship selection** (REV-035). Judged the safer
  direction: degrade to *missing*, never to *wrong*.
- **`uatRound` takes the first integer anywhere after "uat"** (REV-036).
- **A bare `gogo sweep` still asks no confirmation**, and resolves owners against one repo's
  feature list. The real repair is cross-source resolution via `projects.AllSources`.
- **Orphaned doc comments have recurred four times.** An `go/ast` first-word doc guard is
  worth more than a fifth manual fix; one instance remains at `move.go:192-193`.
- **0.28.0's cross-repo same-slug cap OVER-count** is untouched and distinct from this
  release's UNDER-count - opposite directions, do not conflate.

Full audit trail: [.gogo/work/feature-plan-readiness-gate/](../../work/feature-plan-readiness-gate/)
(plan · decisions · adjustments · 7 review rounds · 4 test rounds · report bundle).
