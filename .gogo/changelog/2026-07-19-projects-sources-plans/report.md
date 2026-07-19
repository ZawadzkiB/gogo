# projects · sources · plans — 0.21.0

**The gogo cockpit re-architected onto a three-level model, and made global.** A **project**
is now a first-class home-folder entity (`~/.gogo/projects/<name>/`) that links many **sources**
(repos/services with their own `.gogo/`), owns project-scoped **plans**, and spawns **work items**
into sources — each stamped with the plan's **correlation id** in its `state.md`. Delivered as a
tabbed TUI (**board · plans · config**) with an explicit **global cockpit** (`gogo global init` /
`gogo global`) distinct from the per-repo board. Ships **0.21.0**, superseding the never-committed
0.21–0.24 flat-model line (released baseline was 0.20.1).

## What changed

- **A home-folder project model** — `cli/internal/projects`: `~/.gogo/projects/<name>/config.json`
  links `sources[] {path, name, mainBranch, concurrentWorkItems, color}` (the concurrency cap is
  now **per-source**). `gogo project add/list/rm` + `gogo source add/rm` manage it; every read is
  defensive (missing/malformed → empty). A `$GOGO_DATA_HOME` seam; one-shot, non-destructive
  migration from the legacy `~/.config/gogo/` stores.
- **Project-scoped plans with a status lifecycle** — `cli/internal/plans`: a plan is ONE entity
  (`plan-<hash>` id = the correlation id) with `status: draft → ready → active → done`, collapsing
  the old "drafts" + "epics" into a single concept. `gogo plan new/list/show/add/rm/ready/promote/
  delete`; `gogo draft`/`gogo epic` kept as thin aliases (a draft = a plan in `draft`; an epic = a
  plan with members).
- **Correlation in `state.md`** — the reader parses an additive, optional `correlation:` **list**
  (`[plan-XXXX, …]`) onto `Feature.Correlations`; the board paints `⛓ plan-XXXX` chips and a
  `#plan-XXXX` filter. Absent field → byte-for-byte the old behaviour. A ticket may belong to
  several plans (many-to-many).
- **Spawn = launch, never write** — plan detail's `c create work item` (and `gogo plan promote`)
  launch `claude -p gogo:plan --correlation plan-XXXX` at the source root; the **skill** writes the
  source's `.gogo/work/` and stamps the correlation. The CLI writes only `~/.gogo/…` — never a
  source's `.gogo/`. Verified live (real process, injection-safe single argv element).
- **Tabbed TUI** — `board · plans · config` (`tab`/`shift+tab`), source-tagged cards + source
  filter chips, a plans tab (grouped ACTIVE·READY·DRAFTS + plan detail with target sources), and a
  config tab (project switcher → per-source settings → knowledge explorer). Retires the old modal
  `C`/`D`/`E` screens.
- **Two-mode invocation (UAT round 1)** — `gogo` **in a repo** → that repo's board (unchanged);
  **`gogo global`** / `gogo` **outside a repo** → the cross-project cockpit; set up once with
  **`gogo global init`** (`~/.gogo/` — the home where projects live). `A` on the plans tab authors a
  project plan via a plain `claude` session.

## Key outcomes

- The cockpit is now genuinely multi-project/multi-repo, driven by the "Gogo Cockpit" design, while
  the per-repo experience and the whole `plan→…→report` pipeline are unchanged.
- **Additive, non-breaking** over the released 0.20.1: in a repo `gogo` runs the identical
  `FindRoot → tui.New(root)` path; all released verbs preserved; five new verbs added; the `.gogo/`
  contract gains only the optional `correlation:` list.

## Decisions (one-liners)

- Correlation lives in `state.md` as a list (reversing the earlier CLI-store choice). · Rework the
  P1–P4 infra, don't restart. · Data root `~/.gogo/` (`$GOGO_DATA_HOME` seam). · Plans are one
  entity with a status lifecycle; `draft`/`epic` are aliases. · Spawn via `claude -p` +
  explicit `--correlation` (never advisory prose). · Ship A–D as one `0.21.0`. · Two-mode
  invocation with an explicit `gogo global init`.

## Review / test

- **Review:** CHANGES-REQUESTED → **all 7 findings fixed** (2 major incl. the `A`-trigger redesign
  + doc reconciliation; 3 minor; 2 nit). Every hard invariant held.
- **Test:** ISSUES-FOUND non-blocking → **all 3 fixed** (1 minor + 2 nit). The spawn contract was
  verified as a live process; the single-repo fallback is byte-for-byte.
- **Gates:** `gofmt`/`go vet`/`go test -race ./...` green; `gogo --version` → 0.21.0.

## Follow-ups

- A **unified all-projects board** (design 3a) — `gogo global` currently opens the per-project
  cockpit with a `p` switcher. · **P5 opt-in worktrees** still queued.

Full audit trail: `.gogo/work/feature-projects-sources-plans/` (plan · decisions · uat · review ·
test · report + diagrams).
