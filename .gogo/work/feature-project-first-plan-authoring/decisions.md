# Decisions — feature `project-first-plan-authoring`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

---

## D0 — Turn `plan-1948afcd` into work items: native spawn, or one scoped work item?

- **Phase:** plan
- **Question:** The existing project plan `~/.gogo/projects/gogo/.gogo/plans/plan-1948afcd.md`
  (`status: ready`, `targets: gogo`) already analyses this whole problem across four slices.
  gogo has a native mechanism for turning it into work: `gogo plan go <id>` (or `m` on the
  plans tab) fans out one work item per target, seeded with that source's `## Source briefs`
  section. Should this analysis ride that mechanism, or become a hand-scoped work item?
- **Options:**
  - A. **Fix visibility first, then use the native spawn** — the plan file is now correctly
    filed under the `gogo` project (whose single source *is* this repo), so it is already
    spawn-ready. One `m` press produces one work item carrying all four slices.
    Trade-off: the spawned item covers Slices A-D wholesale, including work the user has not
    prioritised, and the spawn cannot happen until the very visibility bug it describes is
    fixed — a chicken-and-egg.
  - B. **Scope one work item to the user's three priorities** (A1 project choice, B1 multi-line,
    B2 attachments) plus the plans-tab visibility bug, and leave Slices C and D in the plan
    file as backlog for a later spawn.
    Trade-off: the plan file stays partly unconsumed, so the backlog has to be remembered.
  - C. Duplicate `plan-1948afcd`'s content into several new work items by hand.
    Trade-off: silently reimplements what `gogo plan go` already does, and the copies drift
    from the source plan.
- **gogo recommends:** **B** — smallest coherent slice, matches the stated priorities, and
  keeps the native spawn mechanism intact for the remainder.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-29)
**B.** One work item covering A1 (force a project choice), B1 (multi-line goal entry), B2
(attachments) and the plans-tab visibility/reachability bug. Slices **C** (start work directly
/ reuse sessions) and **D** (add a source by browsing) are explicitly out of scope and stay in
`plan-1948afcd.md` as the backlog for a later spawn — that plan file keeps `status: ready` and
must not be consumed or rewritten by this work item. Option C was rejected as duplicating
`gogo plan go`; option A was considered and set aside in favour of the narrower scope.

---

## D1 — Fix the `projs[0]` default focus, or only make it visible and overridable?

- **Phase:** plan
- **Question:** `NewCockpit` sets `focus = &projs[0]` (`model.go:331-343`) and `projects.List()`
  is name-sorted (`projects.go:338`), so the focused project is alphabetical, not meaningful.
  Should the default itself change?
- **Options:**
  - A. **Leave the default; make it visible and overridable** — the plans tab renders its
    project (FR2.2), `p` switches it (FR2.1), and both mint forms ask when several projects
    exist (FR1.1). The default becomes harmless because it is never silent.
    Trade-off: a cold cockpit still opens on the alphabetically-first project.
  - B. **Make the default cwd-aware** — if `$PWD` is inside a registered source's repo, focus
    that project. Helps `gogo global` run from inside a repo.
    Trade-off: a behaviour change to the shared focus that the **board** also uses, with its
    own blast radius (project chip, cap guard, watch set), for a modest gain — and
    `chooseBoard` already routes an in-repo `gogo` to that repo's single board (`main.go:122`),
    so the win is limited to `gogo global` run from inside a repo.
  - C. Persist the last-focused project in `~/.gogo/config.json`.
    Trade-off: new persisted state, new migration surface, and a stale focus is its own
    confusion.
- **gogo recommends:** **A** — it fixes what the user actually hit (a silent wrong-project
  mint) with the smallest surface. B is a reasonable follow-up once A is in.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31, at plan acceptance via /gogo:accept)
**A.** Leave the `projs[0]` default; make it visible (FR2.2) and overridable (FR1.1, FR2.1).

---

## D2 — A per-project switcher on the plans tab, or an all-projects plans view?

- **Phase:** plan
- **Question:** The brief offers two shapes for reachability: "a project switcher/chip on that
  tab, or an all-projects plans view". Which?
- **Options:**
  - A. **Per-project switcher** — bind `p` to the config tab's existing `switchProject`
    (`config_tab.go:26-30`) and render the project in a header row.
    Trade-off: still one project at a time; no cross-project overview of plans.
  - B. **All-projects plans view** — `loadPlans()` loads every project's plans, cards carry a
    project dot (`projectDot` already exists), the kanban partitions as usual.
    Trade-off: **`plans.Plan` carries no project identity.** Every action site keys on
    `m.project.Name`: `plans.Delete` (`plans_tab.go:150`), `planMove`/`planMarkReady`/`planGo`/
    `planAcceptUAT`, `planCreateWorkItem`, `sourceByName`, `planAddTarget`. A merged list would
    act on the wrong project unless each loaded plan carries its origin and every site is
    re-keyed — a much larger change than the reported bug requires.
  - C. Both — `p` cycles `project-1 … project-N … all`, where `all` is the merged view.
    Trade-off: B's cost plus a mode where half the keys must be disabled.
- **gogo recommends:** **A** — it closes the reported bug exactly, reuses an existing shared
  mover, and adds zero risk to the action sites. B stays viable later once `Plan` (or a
  loaded-plan wrapper) carries its project.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31, at plan acceptance via /gogo:accept)
**A.** Per-project switcher (`p` → `switchProject`) + project header row. No all-projects view.

---

## D3 — Attached images: reference by path, or copy into the project store?

- **Phase:** plan
- **Question:** When a user attaches `/Users/me/Desktop/mockup.png` to a plan, does gogo record
  the path, or copy the bytes into `~/.gogo/projects/<name>/`?
- **Options:**
  - A. **Reference by path** (`~`-expanded, made absolute), validated at submit; URLs
    shape-checked but never fetched.
    Trade-off: an attachment in `/tmp` or a moved/deleted file breaks later. Mitigated by
    marking a now-missing path in the plan detail (FR4.5).
  - B. **Copy into the store** at `~/.gogo/projects/<name>/.gogo/attachments/<plan-id>/`.
    Trade-off: durable and self-contained, but adds a real lifecycle — name collisions,
    dedup, a delete cascade in `plans.Delete`, and bytes in the home store that the
    *Footprint* NFR asks us to keep slim.
  - C. Reference by default, **copy only when the path is under a temp dir**.
    Trade-off: B's lifecycle cost for a heuristic that will surprise someone.
- **gogo recommends:** **A** — the plan file is plain markdown where a path/URL is natural,
  Claude Code is multimodal and reads a local path directly, and it avoids inventing a
  storage lifecycle before anyone has asked for one. B is a clean follow-up if broken links
  turn out to hurt in practice.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31, at plan acceptance via /gogo:accept)
**A.** Reference by path (~-expanded, absolute), validated at submit; URLs shape-checked, never
fetched; a now-missing path is marked in the plan detail (FR4.5). No copying into the store.

---

## D4 — How are attachments stored, given the closed front-matter set?

- **Phase:** plan
- **Question:** `parsePlan` reads exactly `id|title|status|created|targets|members` and `render`
  writes exactly those (`plans.go:135-178`, `227-255`) — an unknown key is dropped on the next
  CLI write. And `parseList` splits on `,` (`plans.go:197-208`), so a stored value cannot
  contain a comma. Where do attachments live?
- **Options:**
  - A. **A typed `attachments:` front-matter key** (`Plan.Attachments []string` + parse +
    render), comma-separated like `targets:`, with a **comma rejected at submit**.
    Trade-off: a path containing a comma cannot be attached (rare; refused loudly, never
    silently corrupted).
  - B. **A `## Attachments` body section**, parsed back out like `BriefFor` parses
    `## Source briefs` (`plans.go:474-…`).
    Trade-off: no store-format change and it renders naturally in a markdown viewer, but the
    set becomes a function of prose parsing and a user editing the body can break it.
  - C. A sidecar JSON file beside the plan.
    Trade-off: a second file to keep in sync; the plan stops being one hand-editable markdown
    file, which is the store's whole design premise.
- **gogo recommends:** **A** — it matches the store's existing list fields exactly, round-trips
  for free, is what the launch decorator and the detail view read, and cannot be broken by
  prose edits. The comma limitation is explicit and enforced.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31, at plan acceptance via /gogo:accept)
**A.** Typed `attachments:` front-matter key (`Plan.Attachments []string` + parse + render),
comma rejected at submit (FR4.4).

---

## D5 — Should the multi-line keymap apply to every form, or only the two that have a Text field?

- **Phase:** plan
- **Question:** `form.WithKeyMap` is per-form. Only 2 of the 12 `huh.NewForm(` sites currently
  contain a `huh.NewText` field. Where does the rebound keymap get applied?
- **Options:**
  - A. **A `newForm()` wrapper used at all 12 sites.** The keymap only differs in its `Text`
    group, so the 10 `Input`/`Select`/`Confirm`-only forms are provably unchanged.
    Trade-off: a 12-line mechanical diff instead of 2.
  - B. **Apply it only at the two Text-bearing sites.**
    Trade-off: smaller diff now, but the next `Text` field anyone adds silently regresses to
    "enter submits" — precisely the enumeration-drift trap `coding-rules.md` names as this
    repo's top failure mode.
- **gogo recommends:** **A** — one construction site is the structural fix; B is a smaller
  diff that re-opens the bug on the next form.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31, at plan acceptance via /gogo:accept)
**A.** One `newForm()` wrapper with `gogoKeyMap()` at all 12 form sites (FR3.1, FR3.2).
