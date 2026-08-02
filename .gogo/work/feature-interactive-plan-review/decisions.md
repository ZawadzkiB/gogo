# Decisions — feature `interactive-plan-review`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

<!-- Template for each decision — copy and fill:

## D1 — <short title>
- **Phase:** <plan | implement | review | test>
- **Question:** <the fork, stated plainly>
- **Options:**
  - A. <option> — <trade-off>
  - B. <option> — <trade-off>
- **gogo recommends:** <A / B> — <one-line why>
- **Status:** OPEN        # OPEN | RESOLVED

### RESOLVED (user, <YYYY-MM-DD>)
<the decision, in the user's terms>
-->

## D1 — How do comments get from the offline page back to disk?

- **Phase:** plan
- **Question:** The plan page opens over `file://` with no network and no server
  (`grep -rn "net/http" cli/` returns **nothing** — the product has no HTTP surface at
  all). JavaScript on that page cannot write a file. So how does "Send review" become
  `plan-comments.json` on disk, where the analyst can read it? This is the single
  decision the whole review loop hangs on.
- **Options:**
  - A. **Offline download + drop-in.** The page keeps live state in `localStorage` and
    **Send review** triggers a `Blob` download of `<slug>-plan-comments.json` (the exact
    mechanism the existing layout-export button already uses). The analyst looks for the
    file in the feature folder first, then read-only in `~/Downloads`, and copies it in.
    — Zero dependencies, works from any built page in any project, honours the
    portability bar; costs one manual (or one auto-detected) file move.
  - B. **Local HTTP server in the Go CLI.** `gogo view --serve <slug>` binds
    `127.0.0.1:<port>`, serves the page, and accepts a `POST` that writes the JSON
    straight into the feature folder. — Frictionless, real "send" semantics; but it adds
    an HTTP surface to a product that has none, only works for CLI users (the
    `/gogo:view` skill path is left behind), and makes the CLI write a work-folder file
    (a new exception to "the CLI never mutates pipeline state" — today the only one is
    `.gogo/trash/`).
  - C. **File System Access API** (`showSaveFilePicker`) — write back to the original
    path from the browser. — Chromium-only and unreliable over `file://`; fails the
    portability bar as a primary mechanism.
- **gogo recommends:** **A now, B as the first follow-up work item.** A is the only
  option that works from a page any project can open by double-clicking, and it reuses a
  download path that already exists in the viewer. B is genuinely better UX and should be
  planned immediately after — but as an **opt-in power path**, exactly like `gogo go`
  sits beside in-chat `/gogo:go`. C can be layered on later as a progressive enhancement
  where the browser supports it.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
"We use it locally — just save it on disk: the work folder for local gogo plans, the
project folder for global plans." Direct save is the primary mechanism (File System
Access API, one-time folder grant, handle persisted); the Blob download + printed
destination is the automatic fallback where the API is unavailable. The local server
(`gogo view --serve`) stays the named follow-up, not part of this item.

## D2 — Where does the detailed plan live?

- **Phase:** plan
- **Question:** The brief asks for two files combined into one plan. `plan.md`'s path is
  a **frozen contract**: five skills read it, `contract.PlanSections` gates on it, four
  accept paths refuse without it, and `docs/cli-contract.md` §1 calls it *Guaranteed
  (from plan ①)* and monotonic. Where does the second document go?
- **Options:**
  - A. **`plan.md` (overview) + optional sibling `plan-detail.md`.** — Purely additive;
    every existing work item and every existing reader keeps working byte-for-byte;
    `PlanSections` stays keyed on `plan.md`. Costs: one more file in five enumerations.
  - B. **`plan/overview.md` + `plan/detail.md`.** — Tidier on disk, and it **breaks the
    contract path**. `skills/gogo-view/SKILL.md` forbids it in writing ("its path is the
    contract every phase reads"), and it would force a migration of ~30 existing folders.
  - C. **One `plan.md` with an overview part and a detail part + anchors.** — Simplest,
    zero contract impact; but it does not deliver the ask (two files), and the terminal
    view / `git diff` still show one wall of text.
- **gogo recommends:** **A** — it delivers the two-document ask at zero contract cost and
  keeps the "additive only" record the frozen contract has held since 0.10.0.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — `plan.md` (overview) + optional sibling `plan-detail.md`; contract path untouched.

## D3 — Where does the portable planning tool live, and what is it called?

- **Phase:** plan
- **Question:** The brief wants the planning mechanism as "a separate tool gogo just
  calls", in its own folder, portable later as a standalone skill. Claude Code
  auto-discovers skills from `skills/<name>/` only, and a skill copied to another
  project's `.claude/skills/` has **no `${CLAUDE_PLUGIN_ROOT}`** — so a portable tool
  must address its own files relative to its skill base directory.
- **Options:**
  - A. **`skills/xplan/`** — a self-contained skill dir: `SKILL.md` + `templates/` +
    `contracts/` + `assets/`, invocable as `gogo:xplan`, copyable verbatim into
    `.claude/skills/xplan/`. No manifest change, no new discovery mechanism, and gogo's
    own skills call it by name.
  - B. **`tools/xplan/`** — a non-skill folder the gogo skills read by path. — Clean
    separation from the `gogo-*` skill family, but it is **not invocable**, and it can
    only be addressed through `${CLAUDE_PLUGIN_ROOT}`, which is exactly what a ported copy
    would not have.
  - C. **A separate repo / marketplace plugin now.** — Maximum portability; gogo could not
    dogfood it in one release, and every change would need two repos in lock-step.
- **gogo recommends:** **A**, name **`xplan`** (alternatives if you dislike it:
  `planwright`, `plan-review`, `planboard`). Deliberately **not** `gogo-`prefixed — the
  prefix would have to be renamed at port-out. Plugin skills are namespaced anyway
  (`gogo:xplan`), so an unprefixed name collides with nothing.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A for the home (`skills/` self-contained folder); the user named it **`xplan`**
(renamed from the originally proposed `planpad` across the plan and charts) — aligning
the portable planning tool with their standalone xplan direction.

## D4 — Diagram comments: per-diagram, per-element, or coordinate pins?

- **Phase:** plan
- **Question:** The brief asks for comments on diagrams. What can a comment actually
  point at? Verified in the vendored runtime: **flowchart** nodes are real DOM elements
  carrying their id (`assets/vnm/vnm-browser.js:7701` — `card.dataset.id = nd.id` on
  `div.vnm-node`), so they are addressable with **no renderer change**. A **sequence**
  diagram is not: `buildSequencePayload` returns `svg: renderSequenceSvg(...)`, a static
  SVG in a pan/zoom shell. Class/state paths do query `g[data-id]`, but that is
  unverified.
- **Options:**
  - A. **Per-diagram threads only.** — Works for every kind, zero risk, and loses the
    "comment on *that box*" precision the brief is really asking for.
  - B. **Per-diagram always + per-node where the DOM exposes an id.** — Full precision on
    the flowchart family, which is exactly how gogo authors **architecture, component,
    actor and use-case** diagrams (`gogo-mermaid` renders use-case as an actor↔use-case
    *flowchart*). Sequence degrades to a per-diagram thread; class/state get per-node if
    the probe at implement time confirms it.
  - C. **Coordinate pins** (click anywhere, store world x/y). — Universal, including
    static SVGs; but gogo **persists dragged node positions** (`localStorage`
    `gogo-view:layout:<key>` + the layout sidecar), so a pin's meaning drifts the first
    time anyone rearranges the diagram.
- **gogo recommends:** **B** — precision where it is free and correct, honest degradation
  where the runtime cannot support it. C is a trap given that node positions are
  deliberately mutable.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
B — per-diagram always, per-node where the DOM exposes `data-id`.

## D5 — How does a comment survive a plan edit?

- **Phase:** plan
- **Question:** The brief says: if a comment can no longer be mapped to its line, keep it
  in a global section at the bottom. That is the fallback — what is the *primary*
  anchoring strategy, and who computes it?
- **Options:**
  - A. **Line number only.** — Trivial, and wrong after the first inserted paragraph.
  - B. **Line number + `sha256` of the normalized line + the quoted text (+ neighbour
    hashes).** Resolution order: exact hash at the recorded line → the same hash found
    uniquely elsewhere (re-anchor, badge it *moved*) → **orphan** (global section, badge
    *outdated*, original quote preserved). Computed **in the page** at render time, as a
    pure function of (document, comments) — so **neither page builder writes anything**
    and the CLI keeps its no-mutation invariant.
  - C. **Anchor to the enclosing `##` heading + an offset within it.** — Survives edits
    elsewhere in the document, but a reworded heading orphans a whole section's comments
    at once.
- **gogo recommends:** **B** — it is what PR tools do, it degrades exactly into the
  behaviour the user asked for, and computing it in the page keeps both builders and the
  CLI read-only.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
B — line + hash + quote; exact → unique re-anchor → orphan, resolved in-page.

## D6 — One comments file per plan, or one per document?

- **Phase:** plan
- **Question:** Comments exist against the overview, the detail, and the diagrams. How
  many JSON files?
- **Options:**
  - A. **One `plan-comments.json` per plan**, every thread carrying a `doc` field
    (`plan.md` | `plan-detail.md` | `diagram:<stem>`). — One file to send, one file to
    drop in, one file for the analyst to read; the page filters by `doc`.
  - B. **One file per document** (`plan.comments.json`, `plan-detail.comments.json`,
    …). — Smaller diffs per file; but the round trip becomes N downloads and N drop-ins,
    and the analyst has to enumerate them.
- **gogo recommends:** **A** — the round trip is the expensive part of this design; making
  it one artifact is worth more than smaller diffs. Project plans get the same single file
  at `~/.gogo/projects/<name>/.gogo/plans/<plan-id>.comments.json`.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — one `plan-comments.json` per plan, threads keyed by `doc`.

## D7 — How is "answer my comments" triggered?

- **Phase:** plan
- **Question:** After the user sends a review, something has to make the analyst read the
  JSON and reply. New command, or reuse an existing entry point?
- **Options:**
  - A. **No new command.** `gogo-plan`'s revise path gains a step: *if
    `plan-comments.json` has open threads, answer them first, then revise*. The existing
    re-entries (`/gogo:plan <slug>`, `/gogo:resume`, the UAT loop) already mean "fold in
    the user's input". — Zero new surface; costs a little discoverability ("how do I send
    it back?" is answered by a line printed on the page).
  - B. **A new `/gogo:comments <slug>`.** — Explicit and discoverable; grows the slash
    command set 13 → 14 and drags four enumerations with it (README, docs, the skill, the
    command file), which is exactly the drift trap `coding-rules.md` warns about.
- **gogo recommends:** **A** — "commands stay ultra-thin, skills own the logic" is the
  repo's own rule, and the re-entry paths already exist. If discoverability proves to be
  the real problem, adding the command later is cheap and additive.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — no new slash command; the `/gogo:plan <slug>` revise path answers open threads.

## D8 — Do open comments block plan acceptance?

- **Phase:** plan
- **Question:** Once a plan can carry open comment threads, should the acceptance gate
  refuse while any are open?
- **Options:**
  - A. **No — advisory.** The four accept paths **show** the open-thread count
    (`3 open comments`) and accept anyway. Acceptance stays the user's explicit act.
  - B. **Yes — a hard refusal** while `status: open` threads exist, mirroring the
    plan-readiness refusal shipped in 0.29.0.
  - C. **Yes, but only for threads the AI has not yet answered** — a middle position.
- **gogo recommends:** **A.** 0.29.0's own lesson is that a refusal must be a **legality**
  rule on a monotonic fact ("this plan was never written"), not an opinion about
  readiness. An unresolved comment is frequently just a note the user chose to leave; a
  gate that refuses on it would be a new way to get stuck, and `M` deliberately cannot
  force past legality rules. Show the count, do not block.
- **Status:** RESOLVED

### RESOLVED (user, 2026-07-31)
A — advisory only: accept paths show the open-comment count, never refuse on it.
