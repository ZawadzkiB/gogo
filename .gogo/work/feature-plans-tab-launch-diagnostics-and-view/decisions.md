# Decisions — feature `plans-tab-launch-diagnostics-and-view`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

---

## D1 — What happens when a plan brief exceeds tmux's command-line limit
- **Phase:** plan
- **Question:** tmux 3.7b refuses a command line over **16 317 bytes** with `command too long`
  (bisected on this host). The user's real plan bodies build **16 337** and **20 213**-byte
  commands, so this is not hypothetical. When a launch is over budget, what should happen?
- **Options:**
  - A. **Fold to a pointer.** Drop the inlined body and pass an absolute pointer instead —
    *"read your brief at `<planPath>`, section `## Source briefs` → `### <source>`"*. The brief
    is **already** a file in `~/.gogo/`, and `AuthorPlanIntent` already names `planPath`, so
    nothing is lost. Under budget the command stays byte-for-byte today's.
    Trade-off: the launched session must actually read the file (it is instructed to, and the
    analyst reads the plan file anyway) — one extra indirection.
  - B. **Hard error naming the size.** Refuse with *"brief is 20 213 B, tmux accepts ~16 317 B —
    shorten it"*. Trade-off: honest and simple, but it blocks the user on a limit they did not
    create and cannot easily fix (the analyst wrote that brief).
  - C. **Truncate the goal to fit.** Trade-off: silently ships a mutilated brief. Rejected — a
    truncated spec is worse than a failed launch.
  - D. **Add a `--plan-file <abs path>` param to `/gogo:plan`.** Trade-off: the cleanest
    contract, but a **skill-contract change** requiring enum-sync across `gogo-plan`,
    `gogo-project-plan`, `README.md`, `docs/cli-contract.md` and `skills/gogo-cli` — a much
    larger blast radius for the same effect as A.
- **gogo recommends:** **A**, with B as the backstop if even the pointer form does not fit — it
  preserves the whole brief, needs no contract change, and is a no-op under budget.
- **Status:** RESOLVED — user accepted gogo's recommendation (2026-07-29)

## D2 — Where a plan's `w` page is written
- **Phase:** plan
- **Question:** `pages.WritePage(root, bundle)` writes `<root>/.gogo/resources/view/<name>.html`
  plus ~466 KB of viewer assets (measured). A project plan has no repo of its own. Which root?
- **Options:**
  - A. **The project home** — `~/.gogo/projects/<name>/`. Trade-off: honours the hard invariant
    ("the CLI writes only `~/.gogo/`, never a source repo's `.gogo/`") and matches where the plan
    itself lives. Costs one vendored renderer copy per project — exactly the footprint
    `non-functional-requirements.md` already sanctions ("one vendored renderer per project").
  - B. **The project's first source repo.** Trade-off: reuses an existing renderer copy, but
    **violates the invariant** by writing into a source's `.gogo/`.
- **gogo recommends:** **A** — B is not available; the invariant is a hard rule.
- **Status:** RESOLVED — user accepted gogo's recommendation (2026-07-29) *(recorded for the audit trail; A is effectively forced)*

## D3 — How the plan viewer returns from `modeViewer`
- **Phase:** plan
- **Question:** `updateViewer`'s `esc` sets `mode = modeDrill`, and `viewDrill` dereferences
  `m.drill` with **no nil guard** (`tui/view.go:840-842`). A plan `v` has no drill card, so a
  naive wiring panics. How should the return path work?
- **Options:**
  - A. **A `planViewing` flag mirroring the existing `peeking` pattern** — `updateViewer` checks
    it first and calls `closePlanView()` (→ `modeBoard`, `tab = tabPlans`). Trade-off: follows
    the established precedent exactly; adds one bool.
  - B. **A general `viewerReturn mode` field.** Trade-off: more general, but changes the
    established shape and touches the peek path too.
  - C. **Nil-guard `viewDrill` and let `esc` land there.** Trade-off: the user ends up on an
    empty drill screen they never asked for — a worse UX than the panic it replaces.
- **gogo recommends:** **A** — the `peeking` precedent is right there and the change is local.
  Add the `viewDrill` nil guard anyway as defence in depth.
- **Status:** RESOLVED — user accepted gogo's recommendation (2026-07-29)

## D4 — Making a plan-spawned work item's session attributable
- **Phase:** plan
- **Question:** A plan spawn names its session after the **plan title**
  (`gogo-plan-catalogue-side-…---normalise-…`), but the analyst derives its own feature slug
  (on disk: `feature-catalogue-ingestion`). `SessionMatchesSlug` returns **false** — verified
  against the live session on this host. So the work item shows no `●` dot, `a` attach says "no
  running session", `l` peek falls back to the log, and the cap under-counts it. FR1.7 (aligning
  the slug regexes + adding `ActionAuthor`/`ActionResume`) is a partial fix; the CLI **cannot**
  know the analyst's slug at launch time. Do we close the gap fully in this release?
- **Options:**
  - A. **Record the launched session name on the plan `Member`** (`plans.Member{Source, SlugHint,
    Session}`) and let the board consult it when attributing. Trade-off: a `~/.gogo/`-only store
    write (allowed — `plans` is deliberately write-capable), fixes `a`/`l`/`●`/cap for
    plan-spawned items. Costs a `plans` schema field and a `tui → plans` read in the liveness
    path.
  - B. **FR1.7 only** — align the regexes and the action list, and document the residual gap.
    Trade-off: small and safe, but a plan-spawned item still shows no live session whenever the
    analyst's slug differs from the title hint, which is the common case.
  - C. **Defer entirely.**
- **gogo recommends:** **B for this release, A as the immediate follow-up.** A is the right
  answer but it widens the blast radius into the `plans` schema mid-release; B is a strict
  improvement and unblocks the rest.
- **Status:** RESOLVED — user accepted gogo's recommendation (2026-07-29)

## D5 — The cross-repo same-slug cap over-count
- **Phase:** plan
- **Question:** `cap.go:23-26` documents a Phase-1 limitation: `ActiveWorkSlugs` tests liveness
  with `liveSession(f.Slug, sessions)` over the **global** tmux list, so an identically-named
  slug in another source over-counts and **wrongly blocks** a launch. Reproduced:
  `launching 'other' in /repos/b -> active=[shared-slug] blocked=true` while the session really
  belongs to `/repos/a`. Fix now or defer?
- **Options:**
  - A. **Fix now** via the root-scoped session registry
    (`<root>/.gogo/resources/cli/sessions/<slug>.json`, already read by `m.registry(root, slug)`),
    which proves a session belongs to that root. Trade-off: correct, but adds a per-feature file
    read to the cap path, which runs on every render and reload.
  - B. **Defer**, keeping the documented note, and re-scope it with D4's session recording (which
    would make attribution root-aware anyway).
- **gogo recommends:** **B** — it requires the *same slug in two different sources* to trigger,
  none of the user's three projects has that, and D4's follow-up subsumes it more cheaply.
- **Status:** RESOLVED — user accepted gogo's recommendation (2026-07-29)

## D6 — One release or three slices
- **Phase:** plan
- **Question:** Three legs is a large plan. Ship together or slice?
- **Options:**
  - A. **One release (recommended).** The legs share seams: Leg 1's typed error is what Leg 3's
    red status renders, Leg 3's target guard gates Leg 1's launch, and Leg 2 edits the same key
    switch and help lines Leg 3 touches. Splitting means three passes over `plans_tab.go` and
    three doc-sync rounds.
  - B. **Three slices, in this order** if the reviewer prefers smaller steps:
    1. **Slice 1 — diagnosability** (FR1.1, FR1.5, FR1.6, FR3.2). Pure win, no behaviour change;
       after this the next occurrence names itself.
    2. **Slice 2 — the causes** (FR1.2, FR1.3, FR1.4, FR1.7, FR3.1, FR3.3, FR3.4).
    3. **Slice 3 — plan view parity** (all of FR2). Fully independent of 1 and 2.
- **gogo recommends:** **A**. If sliced, Slice 1 alone already answers "the next failure must
  name itself".
- **Status:** RESOLVED — user accepted gogo's recommendation (2026-07-29)
