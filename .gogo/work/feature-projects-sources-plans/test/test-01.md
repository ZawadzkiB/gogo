# Test round 01 — `projects-sources-plans`

Phase ④ hands-on e2e/UI test of the whole rework (projects → sources →
project-scoped plans → correlation-list-in-state.md → tabbed TUI), 0.21.0.
Tested against `plan.md` (FRs, Tests), `decisions.md` (D1–D9), and
`review/review-01.md` (the 7 fixed findings — re-verified live, not just re-read).

All testing ran against **isolated** `GOGO_DATA_HOME`/`GOGO_CONFIG_HOME` temp
dirs (scratchpad) — the real `~/.gogo` and `~/.config/gogo` were never touched.

## Gate (must stay green) — PASS
- `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` **PASS**
  (every package: `projects`, `plans`, `contract`, `tui`, `launch`, `orchestrator`,
  the top-level `cli` enum-sync + no-unsafe-rm guards).
- `/tmp/gogo-e2e --version` → `gogo 0.21.0` (built from `cd cli && go build -o
  /tmp/gogo-e2e .`).

## What I exercised hands-on

### CLI e2e (real binary, isolated dirs, ≥2 fake source repos)
- `project add <r1> --name acme` → `~/.gogo/projects/acme/config.json` shape
  confirmed exact: `{schema, name, sources:[{path,name,mainBranch,
  concurrentWorkItems}]}`.
- `source add <r2> --project acme` appends (config.json grows to 2 sources);
  `project list` renders both with branch/cap/path.
- `plan new "<title>" --project acme --desc …` writes `plan-<hash>.md`
  (front-matter `id/title/status/created` + free-text body), status `draft`;
  `plan list`/`plan show`/`plan ready` (draft→ready) all confirmed.
- `plan promote <id> <source>` with `claude` stripped from PATH → clean
  `"claude CLI not found on PATH — cannot launch …"`, exit 1, **no crash**, and
  **repo-one's `.gogo/work/` stayed byte-for-byte empty** (verified by directory
  listing before/after) — the CLI never writes a source's `.gogo/work/`.
  Also confirmed the plan's own status/members were untouched by the failed
  spawn (REV-005 held: no phantom active member / no phantom status flip).
- `plan promote` with a **real stubbed `claude` on PATH + real tmux**: the
  stub recorded argv confirms the exact contract — `--permission-mode auto` as
  separate argv elements, then the WHOLE `/gogo:plan <body> --correlation
  plan-XXXX` command as **one single trailing argv element**, even when the
  body contained embedded quotes, `$()`, `;`, `|`, a literal newline, and an
  embedded fake `--correlation plan-fake001` lookalike substring — no shell
  interpretation occurred (no `rm -rf /` executed), confirming injection-safety
  under a REAL spawned process, not just the unit-test fake launcher.
- `draft list`/`epic list` aliases: draft-status filtering and (REV-003)
  member-or-target filtering (any status) both confirmed live.
- `project rm`/`source rm`: both confirmed to leave the source repo's own
  `.gogo/` completely untouched (directory listing before/after); nonexistent
  name → clean error, exit 1.
- **Error paths, all clean (no crash):** ambiguous `source add` with 2 projects
  and no `--project` → clear stderr + exit 1; `project add <dir-without-.gogo>`
  → rejected, exit 1; a hand-corrupted `config.json` (`{not valid json!!`) →
  `project list` degrades that project to `(0 sources)`, exit 0, never a crash.
- **Path-escape adversarial:** `project rm "../evil"`, `project rm
  "acme/../../etc"`, `project add --name "evil/../escape"` all cleanly
  rejected (`validName` guard) — no escape from `ProjectsDir()`.
- **Idempotence/dedupe:** re-adding the same source path → "updated" not
  duplicated; re-running `project add` on an existing project name → directs
  to `gogo source add` instead of clobbering.
- **Migration (D4), both stages, live:** (1) a legacy flat
  `~/.config/gogo/projects.json` entry folds into
  `~/.gogo/projects/<basename>/config.json` (maxConcurrent →
  concurrentWorkItems) on first run, legacy file left in place, second run is
  a no-op (no duplication); (2) a legacy `epics.json` + `drafts/*.md` fold into
  project plans (epic → `active` plan with members, draft → `draft` plan with
  target resolved from the legacy registry) — re-ran the fold with the
  one-shot marker removed to force genuine re-entrancy and confirmed
  deterministic id-minting skipped re-creating either plan (true idempotence,
  not just the marker latch). **REV-007 confirmed live:** a genuinely fresh
  `GOGO_DATA_HOME` with no legacy stores and a no-op command (`project list`)
  leaves the data-home directory **uncreated** afterward.
- **Full plan command surface:** `new/list/show/add(target)/add(retroactive
  link, FR16)/rm(target)/rm(member)/ready/promote/delete` all exercised and
  behave as documented — **except `draft rm`, see TEST-001**.
- **Docs staleness re-check (REV-001):** grepped README.md and
  `skills/gogo-cli/SKILL.md` for every stale marker the review called out (dead
  `C`/`D`/`E`/`e` keys, `~/.config/gogo`, `draft edit`/`--to`, `epic add
  <repo>:<slug>`) — **zero matches**, and both docs now describe the shipped
  tabbed `~/.gogo` model accurately (spot-checked the `board · plans · config`
  keymap + `gogo project/source/plan` command lines).

### Correlation round-trip (the load-bearing flow) — hands-on, real files
Hand-wrote 4 real `state.md` fixtures under the two source repos (not synthetic
in-memory fixtures): `feature-x` → `[plan-7f3a]`, `feature-y` → `[plan-a,
plan-b]` (many-to-many), `feature-z` → no correlation line (parity), `feature-w`
→ `correlation: []` (empty-list parity). Rendered the REAL `acme` project board
through `NewProjectBoard` → `contract.LoadProject` → `View()` (a throwaway
`internal/tui/zz_e2e_probe_test.go`, deleted after this round) against these
on-disk files:
- feature-x renders `⛓ plan-7f3a`; feature-y renders **both** `⛓ plan-a` and
  `⛓ plan-b` (many-to-many holds); feature-z and feature-w render **no** `⛓`
  chip at all (byte-for-byte parity for absent/empty correlation) — asserted by
  locating each card's own text block and checking it, not just a
  whole-board substring search.
- `#plan-7f3a` filter narrowed the in-progress column to exactly `[x]` against
  the real fixture (not a synthetic `contract.Feature`).
- This corroborates (rather than duplicates) the already-thorough pinned unit
  suite: `internal/contract/correlation_test.go`
  (`TestParseStateCorrelationRoundTrip` — one/many/bare/absent/empty-list/
  malformed-unclosed-bracket/trailing-comment, all correct) and
  `TestLoadProjectReadsCorrelationEndToEnd`; `internal/tui/correlation_test.go`
  (chip rendering + `#plan` filter AND-ing with `@source`, unknown-token
  literal-match parity).

### TUI (via Update/View, real fixture data — no browser, this is a Go CLI/TUI)
- **Tab bar** (`board · plans · config`) renders on the project board; **plans
  tab** groups DRAFTS/READY/ACTIVE correctly against the real acme fixture
  (one `ready` plan, one `draft` plan) with `⛓ plan-XXXX` chips and `draft ·
  edited <ago>`.
- **Config tab**: project switcher lists `acme` (+ the malformed `broken`
  project at `0 sources`, confirming graceful degrade reaches the TUI too, not
  just the CLI); sources pane lists both `repo-one`/`repo-two`; knowledge
  explorer surfaces the seeded `tech-stack.md` (21 B) for the focused source.
- **Single-repo fallback** (repo-three, a source whose owning project was
  removed mid-run, leaving it a genuine lone repo): rendered via `New(root)`
  and confirmed **zero leak** — no tab bar (`board · plans`/`plans · config`
  substrings absent), no source tag, no chips, no project count.
- **Review-fixed nits, both confirmed held via the pinned regression tests**
  (`go test -race` green): `TestRenderCardNarrowLongSourceTagNoWrap` (REV-006 —
  source tag never wraps the name row at a 20-char narrow card) and
  `TestPlanListSingleCursor` (single `▸`, never doubled `▸ ▸`, in both the
  plans list and plan detail).
- **`A` plan-with-claude / REV-002 fix**, verified via the existing pinned
  seam tests (`internal/tui/plans_tab_test.go`): fires the launcher exactly
  once, `Action == ActionAuthor` (a PLAIN session, never a `/gogo:plan` slash
  command), no `--correlation` flag, anchored at the project's first SOURCE
  root (never the `~/.gogo` project home unless sourceless), and a no-claude
  box leaves it fully inert (no phantom draft minted, no launcher call).

### Invariant probes
- Grepped every write call site (`os.WriteFile`/`os.MkdirAll`/`os.RemoveAll`/
  `os.Remove`) across `internal/projects`, `internal/plans`, `internal/tui`,
  and the top-level `cli/*.go` command files: **every one** is scoped under
  `projects.Dir(...)`/`ProjectsDir()`/`plans.Dir(...)` (i.e. `~/.gogo/…`); the
  top-level `main.go`/`project.go`/`source.go`/`plan.go` command files contain
  **zero** direct filesystem writes — everything routes through the
  `projects`/`plans` packages. `internal/launch/launch.go`'s only writes are
  the tmux/background-log path (`.gogo/resources/cli/logs/`, the explicitly
  permitted per-source location) — never `.gogo/work/`.
- `projects.Remove` code-read + live path-escape probes (above) confirm it
  stays strictly under `ProjectsDir()`.
- Migration non-destructive/idempotent/no-genuine-no-op-creation — all three
  confirmed live (above).
- `gogo status` (pre-existing, unrelated command) sanity-checked inside a
  project source repo — unaffected by the rework, still works.

### Exploration (breaking it)
- Repo path with a space (`repo with space`) → accepted end-to-end
  (`project add --name "space proj"`), config.json + JSON round-trip clean.
- Project reduced to **zero sources** (`source rm` its only source) →
  `project list` shows `(0 sources)` cleanly; `plan new` against it still
  works (a plan can predate its targets); `plan promote` against an
  unregistered source name → clean rejection, exit 1.
- A registered source whose directory **vanishes from disk** (`rm -rf` after
  registering) → `project list` still renders it (path is data, not
  re-verified live) with no crash; the reader-side degrade for a now-missing
  `.gogo/` is already covered by the pinned `TestLoadProjectAggregatesSources`
  bare-tempdir case (same code path, same defensive behaviour).

## Findings — 3 new (0 blocker · 0 major · 1 minor · 2 nit), 0 regressions on the 7 review-fixed items

| id | sev | pri | title | tag |
|----|-----|-----|-------|-----|
| TEST-001 | minor | P2 | `gogo draft rm <id>` documented as "delete a draft" but forwards to plan-rm-target (needs `<source>`) — always fails as documented; the real (working) verb is the undocumented `draft delete` | AGENT-FIXABLE |
| TEST-002 | nit | P3 | Multiple correlation chips truncate to an indistinguishable `⛓ plan-…` at narrow card widths (~120-col terminal / 4 cols) — full ids render fine at comfortable widths (200 cols) | AGENT-FIXABLE |
| TEST-003 | nit | P3 | A plan body containing a literal `--correlation plan-XXXX`-shaped substring is ambiguous for the gogo-plan skill's NL correlation-capture step (CLI-side injection-safety itself is solid, confirmed live under real tmux) | AGENT-FIXABLE |

Full descriptions + concrete repro + proposed fixes: `test/issues.json`.

**Re-verified all 7 `review-01.md` findings hold (no regressions):** REV-001
(docs accurate, zero stale-marker greps), REV-002 (`A` mechanism — plain
session, no scaffold, fire-once), REV-003 (`epic list` any-status filter),
REV-004 (softened comment — the `plan new` shadow caveat itself still
reproduces exactly as the review described, unchanged, not a regression),
REV-005 (failed promote leaves no phantom member/status), REV-006 (no
name-row wrap), REV-007 (no-op leaves `~/.gogo` uncreated).

## Verdict: **ISSUES-FOUND** (non-blocking)

**Done-bar:** build ✅ + unit/integration ✅ (`go test -race ./...` green) +
e2e hands-on ✅ (CLI, correlation round-trip, TUI render, invariants, migration,
adversarial exploration — all run, nothing blocked) → the hands-on portion of
the done-bar **is met**; no check was blocked or skipped (`claude`/`tmux` were
both exercised: the no-claude path via a stripped PATH, the real-spawn path
via a stubbed `claude` binary + the real `tmux` present on this host — no
silent skip was needed).

0 blockers, 0 majors — nothing here need block a ship. TEST-001 is worth
folding in before shipping (it's a genuinely broken documented command, same
class as the already-fixed REV-001, just missed there since it lives in
`draft.go`'s own `--help` text rather than README/SKILL.md). TEST-002/TEST-003
are cosmetic/defense-in-depth polish, fine to fold in the same pass or defer.
