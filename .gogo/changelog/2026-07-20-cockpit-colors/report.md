# cockpit colors + home-dir board fix — 0.22.0

**An origin-at-a-glance cockpit, and `gogo` in the home dir now opens the global cockpit.**
A presentation + additive-config follow-up to 0.21.0: every project and source gets a default
color, rendered on every surface (board · filter chips · **changelog** · plans · plan-detail ·
config) to match the "Gogo Cockpit" design — so you can tell where a card comes from instantly.
Ships **0.22.0**.

## What changed

- **Home-dir board fix** — `gogo global init` creates `~/.gogo` (a literal `.gogo` dir), which
  the repo-detector previously mistook for a repo, so `gogo` run in your home folder showed an
  empty board. `chooseBoard` is now data-home-aware: a directory whose `.gogo` **is** the global
  data home falls through to the **global cockpit** instead. `gogo` in `~` (or any non-repo dir)
  now opens the cross-project cockpit; real repos still open their own board.
- **Default colors per project + per source** — a curated 8-swatch palette
  (`cli/internal/projects/palette.go`, design teal/pink/blue verbatim, alert-red avoided so a dot
  never reads as "needs you"). `gogo project add` / `gogo source add` auto-assign the next free
  color (deterministic, skip-taken, wraps past 8); a re-add preserves the existing color. New
  additive `Project.Color` alongside `Source.Color`; both editable in the config tab.
- **Colors everywhere** — a never-blank resolver paints source dots on board card name-rows,
  filter chips, plans-tab cards, and plan-detail target rows; the **changelog** column gains a
  leading **source-color dot** (the origin cue that was missing) with the live-session cue moved
  to a trailing green dot. The config tab shows project + source dots, a two-dot `●project
  ●source` origin cue in the switcher, and an editable **label color** (hex or swatch name).
- Colorless (pre-0.22) projects/sources and hand-typed/garbage hex all resolve to a stable,
  non-blank color — never a crash, never grey.

## Key outcomes

- Multi-project/multi-source work reads by color at a glance; the changelog finally shows which
  project/source each shipped item came from.
- Additive over 0.21.0: no new command verb, `state.md`/config schema unchanged (`Project.color`
  is `omitempty`); the single-repo board + changelog stay byte-for-byte.

## Decisions (D1–D5)

Data-home-aware `chooseBoard` (pure-testable seam). · Persist a swatch hex, render adaptive on a
match / direct for arbitrary hex / index-fallback when blank (never blank). · 8-swatch palette. ·
Changelog leading dot = source, live-session cue relocated to a trailing green dot. · Persisted
editable `Project.Color`. · Two dots `●P ●S` for multi-project surfaces.

## Review / test

- **Review:** APPROVE — 0 blockers/majors/minors, 3 cosmetic nits accepted (duplicated
  taken-colors helper; legacy fallback color can shift on reorder; config switcher row can wrap
  for a long name) — all fast-follows.
- **Test:** PASS, no new issues. The home-dir fix confirmed **e2e** with the real binary (home/
  child dirs → global cockpit; real repos → own board); color assign/preserve/wrap over ~22 CLI
  calls; full dot structure vs the design; garbage/hex/extreme-index edges degrade gracefully.
- **Gates:** `gofmt`/`go vet`/`go test -race ./...` green; `gogo --version` → 0.22.0.

## Follow-ups

Extract a shared taken-colors helper · stabilize the legacy fallback color (hash by name) ·
truncate the config switcher `●P ●S` row · the **unified all-projects board** (design 3a) stays a
separate deferred feature.

Full audit trail: `.gogo/work/feature-cockpit-colors/`.
