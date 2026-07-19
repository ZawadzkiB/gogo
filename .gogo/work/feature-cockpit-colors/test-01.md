# Test round 1 — `cockpit-colors` (→ 0.22.0)

Fresh-eyes hands-on test (phase ④), against `plan.md` (FR1–FR5), `decisions.md`
(D1–D5=A), and `review/review-01.md` (APPROVE; 3 P3 nits). Isolation used
throughout: `GOGO_DATA_HOME`/`GOGO_CONFIG_HOME` pointed at scratch temp dirs for
every CLI/TUI invocation — the real `~/.gogo` was never touched.

## Gate (`cd cli`)

- `gofmt -l .` — clean
- `go vet ./...` — clean
- `go test -race -count=1 ./...` — all packages green, incl.
  `TestCLICommandEnumerationInSync` and `TestSkillsBashNoUnsafeRm` (force-run
  standalone too)
- `go build -o /tmp/gogo-colors .` — builds clean
- `/tmp/gogo-colors --version` → `gogo 0.22.0` (matches `.claude-plugin/plugin.json`)

**Gate: PASS.**

## 1 — FR1 home-dir bug fix, real e2e (tmux-driven, real binary — not just the unit test)

Simulated a home where the data home *is* the `~/.gogo` dir: `TMPHOME=<scratch>/fr1-home`,
`GOGO_DATA_HOME=$TMPHOME/.gogo`, `gogo global init`, then `gogo project add
<scratch>/fr1-realrepo --name proja` (a separate real repo elsewhere, with its own
`.gogo/work`). Then launched the real `/tmp/gogo-colors` binary (no args) detached in a
throwaway tmux session from four cwds and asserted the rendered pane (all sessions
cleaned up after):

| cwd | expected | observed |
|---|---|---|
| `$TMPHOME` (== the data home's parent) | global cockpit (tabs, project count, source chips) | `board · plans · config` tabs, `1 project`, `● fr1-realrepo` chip — **global cockpit** |
| `$TMPHOME/childdir` (no own `.gogo/`) | global cockpit | same as above — **global cockpit** |
| `$TMPHOME/fr1-realrepo` (own `.gogo/work`) | that repo's single board | no tab bar, no project count, `0 features` — **single board** |
| `$TMPHOME/fr1-realrepo/childdir` (child of a real repo) | that repo's single board (FindRoot walks up) | same as above — **single board** |

Before this fix, cwd 1/2 would have hit `chooseBoard`'s unconditional first branch
(`rootFound` from `FindRoot` finding `~/.gogo`) and opened an **empty single-repo
board** (`tui.New(~)` reading a nonexistent `~/.gogo/work/`) — confirmed by code
reading (`main.go` pre-fix diff, `plan.md` "Context" section) since the fix removes
exactly that branch for `root/.gogo == dataHome`. The pure `chooseBoard` seam test
(`cli/board_test.go: TestChooseBoardHomeDirFallsThrough`, already durable/pre-existing
from implement) drives the same four branches with fakes — this round adds the *live*
binary confirmation via tmux on top of it, per the task's "real repo elsewhere /
child-of-real-repo" contrast requirement.

**Result: PASS.** FR1 works end-to-end with the real binary in all four scenarios.

## 2 — Colors CLI e2e

Isolated data home; ran:
```
gogo project add <repo1> --name a          # → a.color=#58a6ff, repo1.color=#58a6ff
gogo project add <repo2> --name b          # → b.color=#35c9b5, repo2.color=#35c9b5
gogo source  add <repo3> --project a       # → repo3.color=#4fc3e0 (skips both taken)
gogo source  add <repo3> --project a       # re-add → color stays #4fc3e0 (preserved)
```
- Every `config.json` write carries a non-empty `color` for the project and its
  source(s) — confirmed by reading the JSON directly.
- Re-adding an existing source (`gogo source add` on the same path) preserves its
  color (`existingSourceColor`) rather than reassigning — confirmed (`#4fc3e0`
  unchanged after the re-add prints `updated ...`).
- **Wrap at >8 (no blank/no crash):** added 9 more sources to project `a` (11 total)
  and 9 more projects (10 total, `a`..`j`) — every color non-blank, the palette (8
  swatches) visibly wraps (`repo9`/`a`↔`repo1` both `#58a6ff`, `repo10`/`b`↔`repo2`
  both `#35c9b5`, …), no crash, no exit-code failure across ~22 CLI invocations.
- **Note (informational, matches D2/D2.1 by design, not a bug):** the project-color
  pool and the source-color pool are gathered/assigned *independently*
  (`takenColors()` returns separate `projColors`/`srcColors`), and both pools advance
  in lockstep on `project add` (one project-color pick + one source-color pick per
  call). So a freshly-created project's **own first source** ends up the identical
  swatch as the **project itself** (`a`=`#58a6ff` and `repo1`=`#58a6ff`; `b`=`#35c9b5`
  and `repo2`=`#35c9b5`) for every single-source project — the config switcher's `●P
  ●S` two-dot combo is a same-color pair in the single-source-per-project case,
  slightly undercutting "reads project P, source S at a glance" for the *most common*
  topology. This is presentational and does not violate any FR/decision text verbatim
  (D5 only mandates two dots, not that they differ), so I'm **not** filing it as an
  issue — flagging it here for visibility since it's adjacent to the REV-002/REV-003
  nits below and future readers may notice it live.

**Result: PASS.**

## 3 — Design fidelity (throwaway `zz_colors_probe_test.go` in `cli/internal/tui`, run then deleted)

Built a multi-source project (`alpha`: `s1` explicit palette color, `s2` **legacy
colorless** i.e. pre-0.22, `s3` **hand-typed arbitrary hex**) + a second project
(`beta`), a shipped feature and a shipped-with-live-session feature, and asserted in
one pass:

- Filter chips: `● s1`, `● s2`, `● s3` — a dot per source. **Pass.**
- Board card name row: `● s1` present. **Pass.**
- Changelog: non-live shipped row = exactly 1 dot (leading source dot); live-session
  shipped row = exactly 2 dots (leading source + trailing session, D3); single-repo
  changelog row = 0 leading source dots (byte-for-byte). **Pass** (all three).
- Config left column: project rows carry ≥2 dots (`●P ●S` combo, D5); source rows
  carry a dot each for `s1`/`s2`/`s3`. **Pass.**
- Config right pane: `label color` line contains the swatch name (`blue` for `s1`'s
  explicit color) and the blank/legacy `s2` is flagged `(default)`. **Pass.**
- Every source in the fixture (`s1`/`s2`/`s3`) resolves to a **distinct** non-blank
  color. **Pass.**
- The legacy colorless source (`s2`) resolves to the **same** color across two
  independent model rebuilds from the same store (determinism/stability). **Pass.**

Also ran `TestPlanWithClaudePersistsDraftBeforeLaunch` (item 5, A-persistence) —
**PASS**, and it is meaningful: it asserts the draft plan file
(`plans.Path("app", id)`) exists on disk (via `plans.List`/`os.Stat`) *before* the
launcher cmd has fired (`launched == 0` at that point), then that the returned `tea.Cmd`
resolves to `launchDoneMsg` and the launcher fires exactly once — pins the exact "author
session launched but the plans dir was empty" report this test was added for.

**Result: PASS** (7/7 dot-structure assertions + the A-persistence check).

## 4 — Edge/adversarial

- **Hand-typed arbitrary hex** (`s3 = "#123abc"`, not a palette swatch): renders via
  `colorFor` as a direct `lipgloss.Color`, non-blank, no crash. **Pass** (covered in
  item 3's fixture, and by the pre-existing `TestColorForNeverBlank`).
- **Garbage/invalid color strings** (`"not-a-color"`, `""`, `"#"`, `"####"`,
  `"rgb(1,2,3)"`, an emoji string, a 500-char string) passed through `colorFor` +
  rendered with `lipgloss.NewStyle().Foreground(...).Render("●")`: never nil, never
  panics, never empty output. **Pass.**
- **`ColorForIndex` at 0 / -1 / -1000 / 100000 / -100000:** always returns a non-blank
  hex that round-trips through `LookupSwatch` (i.e. is a real palette swatch — the
  modulo-wrap handles negatives correctly). **Pass.**
- **Empty color on a project/source:** covered by the legacy `s2` case in item 3 and
  by `AssignColor`/`ColorForIndex` fallback tests — never blank. **Pass.**
- **Two-dot combo at narrow width:** built a project with a long name
  (`a-very-long-project-name-indeed`) at `Width=50` (config left pane `half=25`) and
  rendered `viewConfig()`. **Reproduced REV-003 concretely**: the row hard-wraps
  *mid-word* (`a-very-long-project-` / `name-indeed` on two physical lines) because
  `viewConfig` applies `lipgloss.NewStyle().Width(half).Render(left)` over the whole
  un-truncated left column, and `projectOriginDots`/the row string carries no
  `fitSourceTag`-style truncation. No crash, no data loss, no misalignment beyond the
  hard line-break — cosmetic, matching review's non-blocking classification, but this
  concretely **contradicts** review's mitigating note "unlikely to wrap in practice":
  it does wrap, for a realistic long project name at a realistic narrow terminal.
  Recorded here for visibility; not filed as a new issue since REV-003 already tracks
  it (see "Review nits" below).

**Result: PASS** (all edge cases degrade gracefully; REV-003 concretely reproduced,
non-blocking).

## Review nits — verified they stay contained (review-01.md)

| id | review's characterization | this round's finding |
|---|---|---|
| REV-001 | "gather taken colors" duplicated across 3 call sites (DRY smell, not a defect) | Exercised all 3 call sites via CLI + config-tab-adjacent code reading during items 1–4; no drift observed — each site independently avoids collisions correctly. **Contained**, no behavioral symptom found. |
| REV-002 | legacy colorless source's fallback shifts with slice position (cosmetic) | **Concretely reproduced** with a dedicated fixture (`TestZZLegacyFallbackCanCollideWithSiblingExplicit`, throwaway): a project with an explicit-teal `s1` (index 0) and a blank `s2` (index 1) → `s2`'s positional fallback (`ColorForIndex(1)`) is *also* teal — the two dots render **identical**. Non-blank, non-crashing (matches review's disposition); only reachable via hand-edited/legacy `config.json` data, never via the CLI happy path (which dedupes against the whole store). **Contained.** |
| REV-003 | config switcher row not width-truncated (low risk, "unlikely to wrap in practice") | **Concretely reproduced** (see Edge/adversarial above) — it does wrap for a long name at width 50. Cosmetic only, no crash/data loss. **Contained**, but review's "unlikely" mitigation is optimistic; worth a note if this round's finding reaches the user. |

All three nits stay **non-blocking** — verified, not escalated.

## New issues found this round

**None.** `test/issues.json` round 1: `issues: []`.

## Hands-on checks — none blocked

Go toolchain, `tmux`, and the real `/tmp/gogo-colors` binary were all available and
used. Every planned hands-on check (FR1 real-binary e2e via tmux, CLI color e2e,
design-fidelity render, edge/adversarial render, A-persistence) ran to completion.
No emulator/device/dev-server dependency applies to this Go CLI/TUI feature. **No
blocked checks — no user-decision gate required.**

## Cleanup

- Deleted the throwaway `cli/internal/tui/zz_colors_probe_test.go` after use — `git
  status`/`gofmt -l`/`go vet`/`go test -race -count=1 ./...` all clean afterward.
- Killed every scratch tmux session created for this round (`gogo-test-fr1-*`,
  `gogo-test-fr1c-*`, `gogo-test-fr1r-*`, `gogo-test-fr1rc-*`); left the two
  pre-existing host sessions (`gogo-author-untitled-plan`,
  `gogo-author-untitled-plan-2`) untouched (not created by this test round, and
  `gogo sweep` was never invoked — no whole-board reaper risk per the
  host-global-sweep test-strategy caution).
- Scratch fixtures written only under the session scratchpad
  (`fr1-home/`, `fr1-realrepo/`, `colors-home/`, `colors-repos/`) — never under the
  real `~/.gogo` or `~/.config/gogo`.

## Verdict

**PASS** — done-bar met: build + unit + e2e all green, every relevant hands-on check
ran (none blocked), FR1's bug fix is confirmed working end-to-end with the real
binary (contrasted against a real repo and its child dir, which correctly keep their
own single board), the color model is distinct/never-blank/wrap-safe in the CLI happy
path, the design's dot structure (chips, board, changelog, plans, config) matches
FR1–FR5 exactly, and all three of review's P3 nits stay contained (two concretely
reproduced, none escalate). **No open/new issues — ready to advance to ⑤ report.**
