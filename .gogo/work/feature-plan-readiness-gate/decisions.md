# Decisions — feature `plan-readiness-gate`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

## D1 — The shape of the fix: derive readiness, add a status, or fix the ordering

- **Phase:** plan
- **Question:** How should the pipeline know that a work item is still being authored and
  must not be offered for acceptance?
- **Options:**
  - **A. Derive it (recommended).** Treat *"status says `awaiting-plan-acceptance` but
    `plan.md` is absent or a stub"* as authoring, computed at every read
    (`contract.planWritten` → `Feature.PlanUnwritten` → `Feature.Authoring()`).
    — **No new enum value**, so `docs/cli-contract.md` §2 (the frozen `status` enum), §3
    (the four classes + class→column mapping) and `events.schema.json` are untouched.
    Enforced in **Go**, so no agent behaviour can violate it. Crash-safe by construction —
    nothing to un-set. Fixes existing half-written folders retroactively, no migration.
    — Trade-off: it is a **presence-derived** rule, which brushes against `coding-rules.md`
    **TEST-004** ("state rules gate on `status`, never on artifact presence"). See the
    reasoning note below.
  - **B. A new `authoring` / `drafting` status** that `gogo-plan` sets on scaffold and
    clears when `plan.md` lands.
    — Reads cleanly and, checked site by site, **degrades safely by default**: `classify`
    → `ClassUnfinished`; `WaitingForInput` → false; `RunnableStatus` → false;
    `PlannableStatus` → true; `TerminalStatus` → false; `autoPickupReady` → false;
    `readyToShipStatus` → false; `badge` → falls through to the raw status.
    — **But it does not fix the observed failure.** An LLM analyst does not `cp` the
    template — it *writes* `state.md`, and `skills/gogo-plan/SKILL.md` step 5 explicitly
    instructs it to "Set `state.md`: … **status=awaiting-plan-acceptance**". So the safety
    would rest on the analyst honouring a two-phase write in the right order — **the exact
    discipline that just failed**. Prose guarding prose.
    — Cost: `status` is a **frozen-contract enum**. 14 sites name
    `awaiting-plan-acceptance` today: `cli/go.go`, `cli/internal/contract/contract.go`,
    `cli/internal/orchestrator/orchestrator.go`, `cli/internal/tui/{model,move,view}.go`,
    `commands/accept.md`, `docs/cli-contract.md`, `docs/commands.md`, `README.md`,
    `skills/gogo-accept/SKILL.md`, `skills/gogo-plan/SKILL.md`,
    `templates/contracts/events.schema.json`, `templates/state.template.md`.
  - **C. Ordering only** — make `gogo-plan` write `state.md` last (or write a stub-marked
    state first, then finalize).
    — **This is already the documented order**: `skills/gogo-plan/SKILL.md` writes
    `plan.md` at **step 3** and `state.md` at **step 5**. The observed analyst ran them out
    of order anyway. Prose ordering is advisory to an LLM and cannot be a gate. Useful as a
    **belt** (this plan restates it as a hard rule under FR10), never as the fix.
  - **D. A + B together** — derive it *and* add the status.
    — Belt-and-braces, but it buys nothing over A (A already covers every B case) while
    paying B's full frozen-enum cost.
- **gogo recommends:** **A** — it is the only option enforced by something other than an
  LLM instruction, it is the cheapest (no frozen-contract change), and it is the only one
  that repairs folders that are already broken.
- **Status:** OPEN

> **Reasoning note on TEST-004.** The recorded rule reacted to a **stale `report/`**
> surviving a UAT rerun: the artifact was **present** while the state had moved on, so
> presence **over-claimed**. This check is the mirror image and is safe for two reasons:
> (1) `plan.md` is **monotonic** — `docs/cli-contract.md:338` lists it *Guaranteed (from
> plan ①)*, phase ⑤ updates it to as-built and nothing in the tree deletes it, so its
> **absence** can only mean "never written", never "stale"; and (2) the check only ever
> **narrows** — it can turn acceptable into not-acceptable, never promote a class or unlock
> an action. If A is chosen, `coding-rules.md` should be refined at phase ⑤ to state the
> sanctioned exception: *gate on `status`; a presence check may only ever **REFUSE**, never
> **PROMOTE**, and only on a **monotonic** artifact.*

## D2 — How strict is the "the plan is written" check?

- **Phase:** plan
- **Question:** What exactly counts as a written (non-stub) `plan.md`? The requirement is
  "missing **or still a stub**", so mere existence is not enough — but the rule must never
  reject a real plan.
- **Options:**
  - **A. Structural (recommended)** — exists **and** has **≥ 2 `## ` headings**.
    Measured over the 25 existing plans in `.gogo/work/`: the fewest headings any real plan
    carries is **8**. A 4× margin, size-independent, so a legitimately terse plan is never
    rejected for brevity. A scaffold stub has 0-1.
  - **B. Existence only** — simplest, zero false negatives, but does **not** satisfy the
    "or still a stub" requirement.
  - **C. Byte floor** — exists and ≥ ~1 KB. The smallest real plan is **5,494 bytes**, so a
    floor works too, but it is an arbitrary number that ages badly and would reject a
    deliberately short plan for a one-line change.
  - **D. Require the `## Summary (TL;DR)` closing section** (which `gogo-plan` mandates as
    the final section, so its presence proves the plan finished).
    — Rejected on evidence: **only 15 of 25** existing plans carry it (it post-dates most
    of them), so it would mark 10 real plans as stubs.
- **gogo recommends:** **A** — the only option that satisfies "or still a stub" with a
  measured, generous margin and no arbitrary constant.
- **Status:** OPEN

## D3 — What does the board's `m` do on an authoring card?

- **Phase:** plan
- **Question:** `m` is "advance this card". On an authoring card it must not accept. Should
  it do nothing, or offer to finish the authoring?
- **Options:**
  - **A. Bounce with a reason (recommended)** — status line:
    `plan.md not written yet — <slug> is still being authored (finish it with `gogo plan
    <slug>`)`. Launches nothing. Simplest thing that fully satisfies FR4, and it names the
    recovery so a dead-analyst card is never a dead end.
  - **B. Route `m` to `/gogo:plan <slug>` when no session is live** — use the existing
    `liveSessionFor(f.Slug, m.sessions)` to tell a **live** analyst (bounce: "still being
    authored — press `a` to attach") from a **crashed** one (launch `ActionPlan` to resume
    authoring). ~6 lines, reuses machinery that already exists, and turns the crash case
    into a one-keypress recovery.
    — Trade-off: `m` on a plan-column card would launch **three** different commands
    depending on hidden state (plan / accept / go), which is harder to explain in the
    footer and in `README.md`.
  - **C. Bounce, and add a separate key** for "resume authoring".
    — Rejected: a new key for a rare case, and the key space is already dense.
- **gogo recommends:** **A** for v1 — least surprise, fully satisfies the FR, and B stays
  available as a small follow-up once the gate itself is proven. Choose **B** if you want
  the crashed-analyst card recoverable without leaving the board.
  **Note:** **FR13** adds `launch.SessionAction()` for Slice B anyway, so the live-vs-crashed
  discrimination B needs is now free. If you want B, this is the cheap moment to take it.
- **Status:** OPEN

## D4 — Where the silent-build fix belongs: the writer, the classifier, or the display

- **Phase:** plan
- **Question:** An item mid-`/gogo:go` reads `plan-accepted` on disk, so it sits in the
  **plan** column while a session is actively building it (observed live: `catalogue-ingestion
  ● dotai`, header `in progress 0` / `● 2 session`). Where does the fix go?
- **Options:**
  - **A. Fix the writer (recommended).** Phases write their occupancy status at **entry**,
    not their completion status at exit — `gogo-implement` §④ currently runs *after* §②
    does all the work and §③ validates out, so `status: implementing` literally means
    "implementing just **finished**" (same in `gogo-review/SKILL.md:96` and
    `gogo-test/SKILL.md:100`). Writing `implementing` as the first act after validate-in
    makes the on-disk truth match reality.
    — Fixes **every** consumer at once: TUI columns, `gogo status`, `orchestrator` cap,
    `pages`, and any headless reader. **No change to `classify()`, no change to the frozen
    §3 table, no new coupling.**
    — Trade-off: it is an **LLM-prose change**, the same class of unenforceable instruction
    that already failed in Slice A. Hence it is paired with **D5-A** for the one safety
    property, and **FR14**'s cue for the residual launch-to-first-write window.
  - **B. Session-aware `classify`** (pass `sessions` into the classifier).
    — Rejected on evidence: `launch.ListSessions()` returns `nil` when tmux is absent
    (`launch.go:537-540`) and **tmux is a soft dep** by the portability NFR. The core
    classifier would give **different answers for the same tree** depending on the host,
    and `docs/cli-contract.md` §3 is the **frozen, authoritative** table quoted verbatim
    from `skills/gogo-status/SKILL.md`. A deterministic file-surface reader must stay a
    function of the files.
  - **C. TUI-level display override** (file-derived class, session-derived column).
    — Rejected: `cli/status.go` calls `ListSessions()` **zero** times, so `gogo status`
    would keep lying; and decisively **the cap reads `f.Class`** (`cap.go:37`), so the
    working-tree-clobbering bug would survive untouched. It fixes the cosmetics and leaves
    the safety bug.
  - **D. A + a display cue (this plan's shape).** A as the fix, plus an amber `● building`
    chip when a live `go` session contradicts the file (FR14) and a `· stalled` cue when a
    working status has no session (FR15) — so the *residual* window is visible rather than
    silently wrong, without splitting the column truth across surfaces.
- **gogo recommends:** **D** (A + the cue). A is the honest fix and the mirror image of
  Slice A's ordering rule; the cue costs ~15 lines and covers the seconds-to-a-minute
  window A cannot remove.
- **Status:** OPEN

> **The unification the coordinator hypothesised holds, and it is the strongest version of
> this plan.** Both sightings are the same defect: **`state.md` records phase COMPLETION,
> not phase OCCUPANCY**, so the board can only describe where work *has been*, never where
> it *is*. Sighting 1 is that boundary write landing **too early relative to its own
> output** (`awaiting-plan-acceptance` before `plan.md` exists); sighting 2 is it landing
> **too late relative to the work it names** (`plan-accepted` while the build runs). The
> telemetry has the same shape — `gogo-implement` §④ appends **both** `phase-started` and
> `phase-done` in one burst at the end, so `events.jsonl` is a post-hoc log, not a live
> stream. That is why the user's guess ("it will switch on first review when events get
> updated") is essentially right.

## D5 — Should the concurrency cap keep its `Class` filter?

- **Phase:** plan
- **Question:** `orchestrator.ActiveWorkSlugs` (`cap.go:37`) skips any feature whose
  `Class != ClassInProgress`, then requires a live session. An item building under a stale
  `plan-accepted` fails the first test, so it is **invisible to the cap** — a second
  `gogo go` in the same repo is allowed and two sessions edit one working tree. The
  per-**slug** owner lock (`.gogo/resources/cli/locks/<slug>.lock`) does not cover this;
  the same under-count also frees a slot for the reload auto-pickup (`pickup.go:51-55`).
- **Options:**
  - **A. Drop the `Class` filter; count a live `go` session in the root (recommended).**
    The cap's job is *don't let two builds clobber one working tree*, and that question is
    answered exactly by "is there a live build session here". The class test is a redundant
    filter that lies precisely when the writer lags. Makes the one **safety** guard
    deterministic and independent of LLM discipline. Requires **FR13**
    (`launch.SessionAction`) so a `gogo-plan-<slug>` authoring session is **not** counted —
    otherwise Slice B would paper over Slice A.
  - **B. Keep the filter and rely on D4-A.** Smaller diff, but the safety property then
    depends on a phase skill writing a file on time — and a killed session between launch
    and the first write reopens the hole.
- **gogo recommends:** **A** — a guard that exists to prevent working-tree corruption
  should not be contingent on an LLM's write ordering. B is acceptable only if you want
  the absolute minimum diff.
- **Status:** OPEN

## D6 — On a live-vs-file disagreement, cue the card or move it?

- **Phase:** plan
- **Question:** In the window between `gogo go` launching and the phase's first `state.md`
  write, the card's file-derived column is stale. Cue it, or override the column?
- **Options:**
  - **A. Cue only (recommended).** The card keeps its file-derived column and gains an
    amber `● building` chip plus a status-line note. **One source of truth** for placement
    across the TUI, `gogo status`, `pages` and the cap. With D4-A the window is seconds.
  - **B. Override the column** — move a card with a live `go` session into **in progress**
    regardless of its file state.
    — The column is then always visually right, but the TUI structurally disagrees with
    `gogo status` (no sessions) and with any headless reader — the split D4-C was rejected
    for. Also creates a card that "moves back" when the session ends before a write.
- **gogo recommends:** **A** — pick **B** only if a momentarily-wrong column is
  unacceptable to you, accepting that two surfaces will then disagree by design.
- **Status:** OPEN

---

## Findings recorded here (not forks — no call needed)

- **The `⏸ x3` badge is the header pill, not a card badge.** Grepping every `⏸`/`×`
  producer leaves exactly two: `view.go:150` `⏸ K need you` (fed by `needsYouCount()`,
  which counts `WaitingForInput()` across all four columns) and `view.go:603` `⛓ ×N` (a
  card chip needing a `correlation:` line — and `~/.gogo/projects/*/.gogo/plans/` does not
  exist on this machine, so no work item carries one). So `⏸ x3` = `⏸ 3 need you`. It is
  meaningful **only** for items genuinely at a gate; an authoring item wrongly inflates it,
  which **FR2 fixes for free**. No decision needed.
- **`cli/internal/contract/files.go:42` is not the classifier and is not the bug.** It is
  the drill-in artifact list, and `add` already wraps `fileExists` (`files.go:36-40`). The
  real gap is `classify()` / `loadFeature()` in `contract.go`.
- **A placeholder `created:` sorts the broken card to the top.**
  `sortFeaturesNewestFirst` compares raw strings and `'<'` (0x3C) > `'2'` (0x32), so
  `<YYYY-MM-DD>` beats every real date. Covered by FR6.
- **Pre-existing drift, out of scope:** `templates/contracts/events.schema.json`'s `status`
  description lists the "known values" without `awaiting-uat`. Harmless (the field is a
  free string by design). Noted for a later sweep.
- **`launch.SessionMatchesSlug` does not expose the action.** It loops all four actions and
  returns a bare `bool` (`launch.go:481-495`). The convention encodes the action and the
  parse correctly lives in `launch` (TEST-005), but no function returns it — hence **FR13**
  adds `SessionAction()` and refactors `SessionMatchesSlug` to delegate, keeping exactly
  one parser.
- **A card being BUILT displays `● analyst`.** `renderCard` (`view.go:622-625`) computes
  the agent chip as `activeAgent(f)`, which maps `f.Phase` — still `plan` — to `"analyst"`
  (`model.go:929-953`). So the observed card was wrong in its column, its status pill *and*
  its agent chip, all from the same stale phase line. Covered by FR14.
- **`cli/status.go` has zero session awareness** (no `ListSessions()` call), which is the
  decisive evidence in **D4** against a display-layer-only fix.
- **The bug reproduces cross-project** (the observed item is in the `dotai` project, not
  `gogo`), consistent with a defect in the shared skills + `cli/` reader rather than one
  repo's state files.

_No decisions resolved yet._
