# Adjustments — feature `plan-readiness-gate`

Log of changes / clarifications requested during planning (and, later, at the UAT
gate). Each entry: date · what changed · why. The plan above is kept current; this
is the running history.

## 2026-07-29 — scope: fold in the "building item sits in the plan column" case (Slice B)

Mid-plan, the coordinator supplied a second live sighting and asked for it to be folded in
rather than planned separately: the board's **plan** column showed `catalogue-ingestion
● dotai` at `plan-accepted` while a Claude session was actively editing `messages/pl.ts`,
writing component tests and running `npm run test:component`; the header read
`in progress 0` and `● 2 session`.

**Verified, and the unification holds.** Both sightings are the same defect —
**`state.md` records phase COMPLETION, not phase OCCUPANCY**, so the board can only
describe where work *has been*, never where it *is*:

- Sighting 1: the boundary write lands **too early relative to its own output**
  (`awaiting-plan-acceptance` before `plan.md` exists).
- Sighting 2: it lands **too late relative to the work it names** — `gogo-implement` writes
  `status: implementing` at **§④**, *after* §② does all the work and §③ validates out
  (same in `gogo-review/SKILL.md:96` and `gogo-test/SKILL.md:100`).

**Plan delta:**
- Restructured `plan.md` around the one root cause, with the FRs split into **Slice A**
  (plan readiness, FR1-FR8) and **Slice B** (phase occupancy, FR11-FR15), plus the
  cross-cutting FR9/FR10.
- Added **FR11** (phases write occupancy at entry), **FR12** (the cap counts a live `go`
  session rather than the file-derived class), **FR13** (`launch.SessionAction()`),
  **FR14** (`● building` chip + `activeAgent` from the session + a `gogo status` session
  marker), **FR15** (`· stalled` cue).
- Added decisions **D4** (where the Slice-B fix belongs — writer vs classifier vs display),
  **D5** (drop the cap's `Class` filter?), **D6** (cue vs column override).
- Added `charts/sequence.mmd` and `charts/before/sequence.mmd`; extended
  `charts/flow.mmd` and `charts/state.mmd`.

**Severity note added:** `orchestrator/cap.go:37` requires `Class == ClassInProgress`, so
an item building under a stale `plan-accepted` is **not counted** against its source cap —
a second build can start in the same repo and clobber the working tree, and the
per-**slug** owner lock does not cover it. That is the severity argument for Slice B, not
the cosmetics.

**Correction recorded:** the coordinator's note that "`launch.SessionMatchesSlug` already
discriminates by action … so the action is available" is only half right. The naming
convention encodes the action and the parse correctly lives in `launch`, but the function
collapses it to a bare `bool` (`launch.go:481-495`). Hence FR13 adds `SessionAction()` and
refactors `SessionMatchesSlug` to delegate, so there stays exactly one parser (TEST-005).
