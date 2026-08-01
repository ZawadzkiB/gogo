# Review round 02 — `session-binding-ops` (verify pass after fix round 2)

**Scope:** the uncommitted working tree. Round 1's full change (13 modified + 4 new files)
re-checked where the fixes landed, plus a fresh-eyes sweep of the **fix-round diff only** —
`cli/internal/tui/key_help_sync_test.go`, `view.go`, `session_ops.go`, `session_ops_test.go`,
`plan.md`, `README.md`, `skills/gogo-cli/SKILL.md`, `charts/flow.mmd` (identified by mtime
against `review-01.md`, since nothing is committed).
**Reviewed against:** `plan.md` (accepted 2026-08-01, FR4 carrying the marked REV-003
implement-time correction) · `code-review-standards.md` · `coding-rules.md` ·
`non-functional-requirements.md`.

**Gates (re-run by the reviewer, not taken on trust):**
`cd cli && gofmt -l .` clean · `go vet ./...` clean · `go test -race -count=1 ./...` green
(13 packages; `-count=1` deliberately, so nothing rides on a cached result).

**Method:** every "fixed" claim was re-proven by **mutation on a scratch copy** of the repo
(`rsync` to the scratchpad — product code was never edited). A fix only flips to `verified`
if reverting it makes the suite go red for the stated reason.

---

## Verification of round-1 findings

| id | severity | round-1 status | **round-2 status** | proof |
|---|---|---|---|---|
| REV-001 | major | fixed | **verified** | both anti-vacuity floors bite |
| REV-002 | minor | fixed | **verified** | two mutations bite (shipped clause + 1-session arm) |
| REV-003 | minor | fixed | **verified** | chip mutation bites; doc sweep complete |
| REV-004 | nit | fixed | **verified** | producer called at the named site |
| REV-005 | nit | fixed | **verified** | marker mutation bites |

### REV-001 — anti-vacuity floors — **VERIFIED**

`key_help_sync_test.go:108-115` now carries both floors, inside the case loop and ahead of
every per-key assertion.

- Round-1's original mutation (`switchKeys(t, "update.go", c.fn+"NoSuchFunc")`) now **fails**:
  `parsed only 0 keys from updateBoard ([]) - parser drift?`
- Second mutation (emptying the `boardAllKeysLine` const) **fails**:
  `updateBoard: empty documented-key set (tui=0 cli=31) - help line or main.go block drift?`
- Margin **measured, not assumed**: `switchKeys` returns **23** keys for `updateBoard` and
  **18** for `updateDrill`, so the floor of 8 has real headroom in both directions — it
  cannot start failing spuriously. (The fix comment's "19 cases / 9 cases" counts *case
  clauses*, not keys; the conclusion it draws is right.)

### REV-002 — `K` on a shipped card, and the board's 1-session arm — **VERIFIED**

New subtest at `session_ops_test.go:202-228`. It bites on **both** claims, which is what the
finding asked for:

- Mutation A — re-insert the `TerminalStatus` refusal at the top of `killFeature`: **fails**
  (`board K on a shipped card did not open the kill confirm`). That failure incidentally
  proves the fixture card really is terminal, so the subtest is not passing for a weaker
  reason (standard #11(c)).
- Mutation B — route `killFeature`'s `case 1` to `startKillPicker` instead of the Enter-safe
  `startKillForm`: **fails** (`killer calls = [], want exactly [gogo-done-shipped-status]`).
  So the 1-session confirm arm is genuinely *pinned*, not merely traversed.
- The assertion names the exact session string, as the proposed fix required.

### REV-003 — `[K]`-only on shipped cards — **VERIFIED** (option A, end to end)

`view.go:883-888` renders `[K] kill` alone on the `f.Shipped() && hasLiveSession(...)` arm.

- Mutation (restoring `chip("[R] re-assign")` to that arm) **fails**:
  `shipped card advertises [R] re-assign, a key that always refuses there`. The chip cannot
  creep back.
- Doc sweep is complete and greppable — **no surface still promises `R` on a shipped card**:
  - `plan.md` FR4 carries an explicit `(Implement-time correction, REV-003: …)` note, so the
    contract records the FR3/FR4 tension instead of the build silently deviating from it;
  - `README.md` (changelog bullet) and `skills/gogo-cli/SKILL.md` both now read
    "`K` kills it from the board; `R` on the card it should **DRIVE** adopts it";
  - `charts/flow.mmd`'s `CHIPS` node reads `shipped+session -> [K] only`;
  - `docs/cli-contract.md` and `cli/main.go` never made the claim; `docs/index.md` carries no
    key table, so standard #1's "all of `docs/*.md`" sweep has nothing further to hit.

### REV-004 — the named producer — **VERIFIED** (at the site the finding named)

`session_ops.go:165` is now `if !orchestrator.PlannableStatus(f.Status)`, the same producer
`cli/go.go:318` calls, with a comment recording why. `adoptFeature` correctly kept
`orchestrator.TerminalStatus` (`session_ops.go:281`) — "nothing to DRIVE" stays a separate
rule, exactly as the finding asked. The *optional* structural pin was not added, and the
second call site of the same rule is still un-migrated → tracked as **REV-006** below.

### REV-005 — the `attached` marker — **VERIFIED**

`adoptRow` (`session_ops.go:342-344`) appends `" · attached"`, and
`TestAdoptRenamesToTheDerivedAction` asserts the rendered picker carries it.

- Mutation (delete the two-line append) **fails**: `picker row does not mark the attached
  session` — the assertion bites rather than matching incidental text.
- It is a **word, not a colour**, so it survives a colourless TTY and stays assertable in
  `View()` (NFR *Diagnosability*).
- Checked deliberately: the picker's option **value** is still the bare `meta.Name`
  (`huh.NewOption(m.adoptRow(meta), meta.Name)`). A label suffix leaking into the value would
  have made `rename-session -t "=<old>"` target a session that does not exist.

---

## Fresh-eyes sweep of the fix-round diff

Checked and **cleared** (so they are not on the findings list):

- **No render-path regression.** `boardStatusLine` → `unboundHere()` → `boardRoots()` →
  `capWatchSources()` resolves from in-memory `m.allProjects` / `m.project.Sources`
  (`model.go:527`) — no disk IO per frame, so the comment's "pure in-memory" claim is true and
  the CLI's millisecond read-path bar holds.
- **The chip switch matches FR4 exactly** — stalled → `[R] adopt`, shipped+session → `[K] kill`,
  non-terminal → `[P] plan`. Chips are symptom-driven by design, so the absence of `[K]` on a
  *live, non-shipped* card is the plan's intent, not an omission.
- **`plan.md` edit is legitimate.** Post-acceptance edits to the contract are normally a red
  flag; this one is a single, explicitly marked correction that preserves the original wording
  and states why it changed. That is better than the round-1 suggestion (a note in the phase-⑤
  report) because the contract itself stops disagreeing with the build.
- **Nothing unplanned crept in.** The fix round touched only the four code/test files the five
  findings named, plus their doc surfaces. No new dependency, no `.gogo/` write, no pipeline
  state written by any of the three ops, no host-global destructive remedy in any message
  (TEST-007), version still 0.32.0 in both `plugin.json` and `cli/main.go`.

---

## New findings (round 2)

| id | severity | priority | status | fixability | title |
|---|---|---|---|---|---|
| REV-006 | nit | P3 | new | AGENT-FIXABLE | the `[P] plan` **chip** still inlines `!TerminalStatus` while its handler now uses `PlannableStatus` |
| REV-007 | nit | P3 | new | AGENT-FIXABLE | a `(REV-004)` comment tag points at the wrong finding |

### REV-006 — the producer fix stopped one site short (nit, P3, AGENT-FIXABLE)

The REV-004 fix moved the **handler** onto the named producer but left the **chip that
advertises the same key** on the old inlined predicate:

```go
// cli/internal/tui/session_ops.go:165  (fixed in round 2)
if !orchestrator.PlannableStatus(f.Status) { … "nothing to plan" }

// cli/internal/tui/view.go:889        (still the inlined copy)
if !orchestrator.TerminalStatus(f.Status) { c = append(c, chip("[P] plan")) }
```

Before the fix the two sites were at least *consistently* wrong; now one user-visible rule
("can this card be planned?") is derived from two different predicates. That is the shape
code-review-standards #12 / coding-rules TEST-006 tell reviewers to flag — *"a constant whose
call sites are unpinned, so a surface can stop calling the producer and hand-write fresh copy
with the whole suite green"*. If `PlannableStatus` ever gains the clause REV-004 itself used
as its example (also refusing `waiting-for-user` while a re-plan lock is held), the footer
would offer `[P] plan` on a card where `P` bounces — a chip that lies, the same class as
REV-003, one file over.

**Latent only:** `PlannableStatus(s)` is literally `!TerminalStatus(s)` today
(`orchestrator.go:123-125`), so there is no user-visible bug and the suite is green.

**Fix:** one line — `if orchestrator.PlannableStatus(f.Status)` at `view.go:889`. Optionally
add the variant-8 wiring pin REV-004 left optional: a table test asserting `footerChips`
offers `[P] plan` for exactly the statuses `planFeature` accepts. Leave the `TerminalStatus` /
`Shipped()` uses in the chip switch and in `adoptFeature` alone — those are different rules.

### REV-007 — a comment tag points at the wrong finding (nit, P3, AGENT-FIXABLE)

`session_ops.go:224` reads
`// Never launch relative to the process cwd (REV-004) — bounce, launching nothing.`
on the `if in.Root == ""` guard in `finishPlanSession`. **REV-004 is the PlannableStatus
finding**, which has nothing to do with the launch root; the correctly-tagged REV-004 comment
is 60 lines above at `session_ops.go:163`. This repo uses `REV-NNN` comment tags as a
first-class audit convention (~20 across `cli/`), so a wrong id sends the next reader — and
the phase-⑤ report writer — to an unrelated finding. Verified the tag matches nothing: no
round-1 finding mentions the launch root, and no archived feature's review owns that text.

The guard **itself is correct and worth keeping** — refusing to launch relative to the process
cwd rather than guessing is the NFR's *"degrade to MISSING over WRONG"*. It is defensive:
`m.rootFor(f)` falls back to `m.root` (`model.go:724-729`), so an empty root is unreachable in
normal cockpit use and the branch is untested — acceptable for a belt-and-braces bounce.

**Fix:** drop the `(REV-004)` (the sentence explains itself) or replace it with the marker that
actually applies. Do not renumber the correct tag at `session_ops.go:163`.

---

## Verdict

**APPROVE** — all five round-1 findings are **verified fixed by mutation**, including the
`major` (REV-001), and the gates are green on a fresh `-count=1 -race` run. Two new **nits**
(REV-006, REV-007) are open; per the routing rule (no open/new blockers or majors) they do not
hold the round, and neither is a user decision — both are one-line, agent-fixable and can ride
along with any later round or be batched into phase ⑤.

Round is **clean → advance to ④ test.** The plan's phase-④ row is the right place for what is
still unproven here: the three ops against **real tmux on this host** (`P` opening an attached
pane, a hand-retasked session adopted with `R` clearing `· stalled` and moving the dot/chip/cap,
`K` on a two-session card), plus confirming the live unbound `gogo-plan-catalogue-…` session is
reported rather than invisible.
