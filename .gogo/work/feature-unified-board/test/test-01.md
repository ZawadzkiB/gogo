# Test — unified-board (round 1)

Fresh-eyes e2e/hands-on test of the `unified-board` change (→ 0.23.0): `gogo global`
opens ONE unified board across every registered project. Tested against `plan.md`
(FR1-FR6), `decisions.md` (D1-D4=A, D5=B), and `review-01.md` (REV-001 major + REV-002
nit — both claimed fixed; this round verifies they STAY fixed, live).

All isolation via `GOGO_DATA_HOME` / `GOGO_CONFIG_HOME` pointed at scratch dirs under
`/private/tmp/.../scratchpad/e2e-unified-board/`; the real `~/.gogo` was never touched
(confirmed: `~/.gogo/projects/` still holds only the pre-existing `gogo` entry, no
`alpha`/`beta`/`solo`/`ghost`/`repoA`/`repoB` test projects leaked in). No real `claude`
was ever invoked (a stub on PATH recorded zero calls); every launch confirm was
cancelled before submission.

## Verdict: PASS

- New issues found: 0
- Gate: `gofmt -l .` clean · `go vet ./...` clean · `go test -race ./...` GREEN (all
  packages, including the durable `cli/internal/tui/unified_board_test.go` suite added
  by implement).
- Version: `/tmp/gogo-unified --version` → `gogo 0.23.0`; `.claude-plugin/plugin.json`
  matches.
- Done-bar (build + unit + e2e green + hands-on done): **MET**. No blocked hands-on
  checks — tmux and a real terminal were available on this host, so the interactive TUI
  was driven live end-to-end, not just at the model/`Update`/`View` level.

## 1. Build + gate

```
cd cli && go build -o /tmp/gogo-unified .   # OK
/tmp/gogo-unified --version                  # gogo 0.23.0
gofmt -l .                                   # clean
go vet ./...                                 # clean
go test -race ./...                          # ok, all packages
```

## 2. Unified aggregation — CLI e2e, real binary, live tty (tmux)

Built two isolated fixture "projects" on disk (`repoA` project **alpha**, `repoB`
project **beta**), each with `.gogo/work/feature-*` items spanning all four classes
(plan-pending, in-progress, ready-to-ship, shipped) plus a real `.gogo/changelog/`
entry, and a third empty project (**gamma** / `repoC`, 0 work items). Registered them
for real: `gogo global init` -> `gogo project add repoA --name alpha` -> `gogo project
add repoB --name beta` -> `gogo project add repoC --name gamma`. Then drove the real
built binary (`gogo global`) inside a throwaway `tmux` session (`gogo-test-unified-*`),
sending real keystrokes and capturing real rendered panes (`tmux capture-pane`).

Confirmed live (screen captures retained under the scratch dir for this session):
- **Header aggregates everything.** `gogo cockpit  10 features · 3 projects` — 10 =
  5+5 real features across alpha+beta (gamma contributes 0); unaffected by the project
  chip (by design, FR1: "N now counts every project's features" — the total, not the
  chip-narrowed view). Live `fsnotify` reload was also exercised: adding a fixture
  feature file on disk mid-session updated the header/columns within ~1s with no
  restart.
- **Project chip row present** — `project  all  ● alpha  ● beta  ● gamma`.
- **`p` cycles project + narrows**, verified by content (not just counts, since the
  fixtures were deliberately symmetric): pressing `p` from "all" landed on alpha (only
  `●alpha ●repoA` cards visible, beta's cards gone), one more press landed on beta
  (content-verified: `auth`/`payments`/`notify`/`cli` all tagged `●beta ●repoB`,
  changelog narrowed to `shipped-y` only), one more on gamma (all four columns empty,
  `0/0/0/0`, no crash on the empty project), one more back to "all" (`2/2/4/2`, the sum).
- **Origin dots, two-name form (D5=B).** Every card and changelog row carries
  `●project ●source` with both names spelled out when Project != Source (used
  `--name alpha`/`--name beta` so the project name differs from the source's repo-
  basename label) — e.g. `billing  ●alpha ●repoA`, changelog `● ● ✓ shipped-x  07-02`.
- **Dedup case (Project == Source).** Re-registered a second isolated pair WITHOUT
  `--name` (project name defaults to the repo basename == the source label too) and
  confirmed live: the tag collapses to ONE dot + name (`billing  ● repoA`, changelog
  `● ✓ shipped-y`), never the doubled `●repoA ●repoA`.
- **`@name` filter matches project OR source (D3=A).** `@alpha` (project token) and
  `@repoB` (source token) each correctly narrowed the board live while the chip stayed
  on "all" — the D3 drift-fix (the token used to match only `Source`) holds in the real
  TUI, not just the unit test.

## 3. REV-001 same-slug SAFETY — verified CLOSED, live, e2e

Both `repoA` and `repoB` carry a `feature-cli` (same slug, different projects, both
ready-to-ship). Drove this live with a `claude` stub prepended on PATH (so `hasClaude`
is real) and cancelled every confirm before submission — the stub's call log stayed
empty (`0` invocations) for the whole session, confirmed by its absence on disk.

1. **Wrong-repo launch — FIXED.** Focused project-**beta**'s `cli` card (the SECOND
   `cli` in the merged, newest-first feature list; the pre-fix bug would resolve by a
   `m.repo.Feature(slug)` re-lookup that returns the FIRST match, i.e. alpha's) and
   pressed `d`. The live confirm dialog read:
   `will run: claude "/gogo:done cli"  in tmux session gogo-done-cli  at
   .../scratchpad/e2e-unified-board/repoB` — correctly anchored at **beta's own root**
   (repoB), not alpha's. Cancelled with `n` + confirm — the stub was never called
   (confirmed via the empty call log).
2. **Selection collision — FIXED.** `space` on beta's `cli` card selected it alone
   (`✓ cli ●beta ●repoB`); alpha's `cli` card in the same column stayed unselected
   (`○ cli ●alpha ●repoA`) — the composite `Root\x00Slug` key keeps the two same-slug
   cards independently selectable, confirmed on the live render, not just the model.
3. **Cross-project ship guard — FIXED, fires live.** Selected BOTH same-slug cards
   (alpha's and beta's `cli`, now both showing `✓`) and pressed `d`. The board bounced
   with `select ready cards from one project to ship together` and opened no confirm,
   no launch — the guard resolves from the actual selected features (composite-keyed),
   not a slug re-lookup that would have collapsed the pair.

This closes REV-001's three manifestations end-to-end against the real binary and a
live terminal, matching the durable regression coverage already in
`cli/internal/tui/unified_board_test.go` (`TestUnifiedSameSlugAcrossProjects`) — this
round independently reproduced the same three assertions live rather than trusting the
unit test alone.

## 4. FR5 cross-project cap + watch — verified live

Focus was **alpha** (`m.project`, the default with chip=`all`); added a second
in-progress feature (`reports`) to **beta** and a real throwaway tmux session
`gogo-go-reports` (killed immediately after this check) so `beta`'s source (cap 1, the
default) was "over cap." Navigated to **beta's `payments`** card (a non-focused-project
card, visible because chip=`all`) and pressed `m`. The live status line read:

```
cap 1 reached — already building reports; ship one or run `gogo go payments --force`
```

This is the exact FR5 regression the plan called out: the capped card's source (beta)
is NOT the focused project (alpha), and `capBounce` still fired — proving it resolves
`projects.AllSources(m.allProjects)`, not just the focused project's sources. Matches
`TestUnifiedCapBounceSpansProjects`, reproduced live.

`watchDirs` spanning every project (not just focused) is covered by the durable
`TestUnifiedWatchDirsSpansProjects`; independently, the live `fsnotify` reload observed
in section 2 (a fixture file added to `repoA` while focus/chip pointed elsewhere) is
consistent with the watch set including non-focused sources.

## 5. Fallbacks — byte-for-byte, verified live

- **`gogo` INSIDE a repo** (`cd repoA && gogo`, isolated env): single-repo board, header
  `gogo cockpit  5 features` (no `· M projects` suffix), **no** tab bar (`board · plans
  · config` absent entirely), **no** project chip row, cards carry **zero** tags
  (`login`, `billing`, `search`, `cli` — no `●` at all), changelog row `✓ shipped-x
  07-02` with no leading dot. Byte-for-byte the pre-0.23 single-repo shape.
- **A single registered project**: `gogo global` with only `solo` (repoA) registered ->
  unified board opens, header `5 features · 1 project`, chip row collapses to
  `all  ● solo`, `p` cycles it cleanly (no crash, no regression), cards still carry the
  two-name tag (`●solo ●repoA`).
- **No projects / uninitialized**: three variants tried, all exit 1 with a helpful
  stderr line and no crash — not-initialized (`run \`gogo global init\``), initialized
  + 0 projects (`add one with \`gogo project add <repo>\``), and bare `gogo` outside any
  repo with no global home at all (`run \`gogo global init\`, or cd into a gogo repo`).
- **Malformed project — no crash.** Corrupted one project's `config.json` to invalid
  JSON (`beta` -> `{not valid json!!`) and pointed another project's only source at a
  path that no longer exists on disk (`ghost`). `gogo project list` degraded the
  malformed project to `(0 sources)` and printed the rest normally (exit 0); the live
  TUI opened without crashing, showed all 3 registered project names in the chip row
  (`alpha`/`beta`/`ghost`), rendered only `alpha`'s 5 real features, and `p`-cycling
  into `beta`/`ghost` rendered clean empty columns (`0/0/0/0`) with the tmux session
  still alive afterward (no panic).

## 6. Design fidelity + edges — verified live

- **Long project/source names, narrow terminal (90 cols).** Registered a project named
  `backend-platform-services` alongside `beta`. At 90 columns (4-column board, ~20-char
  cards) the origin tag degraded progressively: full names when there's room, truncated
  (`●b… ●rep…`) under pressure, dots-only (`●●`) at the tightest, but the **slug never
  wrapped onto a second line** in the name row at any point observed (`auth`, `login`,
  `cli` all stayed single-line) — matches `TestUnifiedTwoNameTagNoWrap` /
  `TestUnifiedTagSlugStaysReadable`. (Description-row text wrapping at that width is
  expected/unrelated — e.g. "Fixture reports" wrapping to two lines is the card body,
  not the name-row tag.)
- **Plans/config act on the focused project (D4).** Config tab showed
  `▸ ● ● backend-platform-services` focused by default (chip=`all` -> `allProjects[0]`).
  Pressed `p` in the config tab -> focus moved to `beta` (`▸ ● ● beta`, source panel
  switched to `repoB`). Tabbing back to the board confirmed the **board's project chip
  followed** — column counts changed to beta's subset (`1/2/2/1`, matching beta's
  fixture exactly) even though the chip's own keypress was never sent on the board tab
  itself — the shared `m.project` focus (D4) holds live, not just in
  `TestUnifiedConfigSwitcherSharesFocus`.
- **A project with 0 work items** (`gamma`/`repoC`): registered, appears in the chip
  row, `p`-cycles to it cleanly, renders 4 empty columns, no crash — covered in
  sections 2 and 5.

## What was NOT re-tested here (already durable + unchanged)

Per-widths 22-60 no-wrap/dedup/slug-floor matrix, the `matchFilter` truth table, and the
`chooseBoard` pure-branch tests are already exhaustively covered by the durable Go
suite (`unified_board_test.go`, `workspace_test.go`, `stagec_test.go`) which is part of
the gate re-run in section 1 — not duplicated live beyond the spot-checks above.

## No new e2e test files added

Every check in this round was either (a) driven against the REAL binary + a live tty
(tmux), using disposable on-disk fixtures under the scratchpad (never touching
`cli/`), or (b) re-confirmed the already-durable `cli/internal/tui/unified_board_test.go`
/ `workspace_test.go` coverage the implement round wrote. No throwaway `zz_*_test.go`
was created (none was needed — the live tty path covered every item), so there is
nothing to delete. The tree is gofmt-clean and the gate is green as re-run in section 1.
