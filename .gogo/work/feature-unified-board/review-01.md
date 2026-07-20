# Review — unified-board (round 1)

Fresh-eyes code review of the `unified-board` change (→ 0.23.0): `gogo global` now
opens ONE unified board across every registered project (design 3a). Reviewed against
`plan.md` (FR1–FR6) + `decisions.md` (D1–D4=A, D5=B).

- Diff: 17 files, +511/-183 · new `cli/internal/tui/unified_board_test.go`.
- Gates: `gofmt -l` clean · `go vet ./...` clean · `go test -race ./...` GREEN.
- Version: `plugin.json` + `cli/main.go` both at `0.23.0`.

## Verdict: CHANGES-REQUESTED

- Blockers: 0
- Majors: 1 (REV-001)
- Minors: 0
- Nits: 1 (REV-002)

One open major → CHANGES.

## Findings

### REV-001 · major · P1 · open — duplicate slug across projects mis-routes launches + collides selection

The highest-risk area the plan flagged is real. On the unified board a feature's
identity is its **slug alone**, but a slug is unique only per-source, not across the
merged workspace. Reproduced empirically (throwaway test, since removed):

1. **Silent wrong-repo launch.** `doLaunch` (`move.go:203-206`) re-resolves the launch
   root with `m.repo.Feature(intent.Slugs[0])`, which returns the **first** feature
   matching the slug (`contract.go:282-289`), not the focused card. Focusing project
   *beta*'s card (Root `/r/b`) and pressing `d` (ship) or `m` (go/accept) launches in
   project *alpha*'s repo (`/r/a`). The confirm dialog (`confirmSummary`) shows no repo
   path, so the user cannot catch it — a state-mutating `/gogo:done` / `/gogo:go` runs
   against the wrong repo/feature.
   - Observed: `focusedCard: project=beta root=/r/b` → `doLaunch would root at: project=alpha root=/r/a`.
2. **Selection collision.** `m.selected` is keyed by `f.Slug` (`model.go:176`,
   `update.go:245-248`, `view.go:611`); one `space` toggle marks **both** same-slug
   cards selected (observed: 2 cards), and the user cannot pick just one.
3. **Cross-project ship guard defeated.** `selectionSpansProjects` (`move.go:85-101`)
   resolves each slug via `m.repo.Feature` and collapses the pair to one feature, so it
   returns `false` — the plan's stated safety net ("cross-project merged ship stays
   refused by the existing `selectionSpansProjects` bounce") does **not** fire.
   - Observed: `selectedSlugs=[dup] selectionSpansProjects=false` (wanted `true`).

Also: `LoadWorkspace` does not dedup a repo that is a Source in ≥2 projects, so that
repo's features render as duplicate cards (same slug+root, different Project stamp).
The concurrency cap is **not** inflated — `orchestrator.ActiveWorkSlugs` dedups by slug
— but the board shows doubles.

The pattern predates 0.23.0 (a single-project multi-source board could collide too),
but the unified board makes collisions **likely**: independent projects commonly reuse
slug names, and a shared repo can be registered under two projects. No dup-slug test
exists.

**Fix (AGENT-FIXABLE).** Minimal first: root every launch from the focused card's own
resolved root rather than re-resolving by slug in `doLaunch`. Then make board identity
composite — key `m.selected` by e.g. `f.Root+"\x00"+f.Slug` and update `toggleSelect`,
`selectedSlugs`, the `view.go:611` selected check, and
`selectionSpansProjects`/`attemptAction`/`doLaunch` to resolve by that composite. Add a
two-projects-same-slug test asserting the focused-card ship roots correctly, one toggle
selects one card, and a spanning selection bounces. If the multi-select composite work
is deferred, manifestation 1 (the silent wrong-repo launch) must still be fixed now.

### REV-002 · nit · P3 · open — `spawnedFeature` matches by source-name only across the merged repo

`spawnedFeature` (`plans_tab.go:99-114`) scans the whole merged `m.repo.Features`
matching `f.Source == sourceName` (+ correlation id), not constrained to the focused
project. D3=A notes source labels collide across projects, so this relies purely on
plan-id uniqueness (`plan-<hash>`, effectively unique) to avoid cross-project mismatch —
very low risk today, but the query is no longer strictly project-scoped.

**Fix (AGENT-FIXABLE).** Also require `f.Project == m.project.Name` (or match against
the focused project's source roots) so a same-named source in another project can never
be picked up.

## What checks out (verified, no action)

- **FR1 aggregation.** `LoadWorkspace` loops `LoadProject`, stamps `f.Project` on every
  feature (Source/Root already stamped), merges + `sortFeaturesNewestFirst` (stable,
  Created-desc, slug tie-break) → deterministic newest-first matching single-project
  order. Empty/malformed project contributes nothing (no crash); covered by
  `TestLoadWorkspaceMergesProjects` / `...Empty`. `Feature.Project` is an in-memory
  overlay — never written to `state.md` (no `.gogo` contract-file change).
- **FR5 regression fix is real.** `capBounce` (`move.go:123`) and `watchDirs`
  (`watch.go`) both resolve `m.capWatchSources()` = `projects.AllSources(m.allProjects)`
  on the unified board, so a card whose source lives in a non-focused project is capped
  + watched. Confirmed by `TestUnifiedCapBounceSpansProjects` /
  `TestUnifiedWatchDirsSpansProjects`. `ActiveWorkSlugs` still counts by `f.Root`.
- **Fallbacks.** Single-repo `New(root)` → `unified=false`, `project=nil`; `projectChips`
  nil, `viewProjectChips` "", `originTag` returns `("","")` for a source-less feature
  (`LoadRepo` leaves `Source`/`Project` empty) → byte-for-byte single-repo card. Single
  registered project degrades cleanly (`TestUnifiedSingleProjectDegrades`). `NewCockpit`
  is crash-safe with zero projects.
- **Origin-tag fit (D5=B).** `originTag` dedups to one `● name` when `Project==Source`,
  two names when they differ; `fitOriginTag` reserves the slug floor (14) and shrinks the
  tag (names → dots → drop) never the slug; the composed name row cannot wrap (slug is
  truncated to the width left after the fitted tag). Changelog leads `● ● ✓ slug`, deduped
  to one dot when `Project==Source`/source-only. Covered by the new no-wrap /
  slug-readable / dedup / two-dot tests at widths 22–60.
- **Plans/config focus (D4).** Board chip and config switcher share `m.project` via one
  `focusProject`; `all` → `allProjects[0]`; `switchProject` sets `projectChip` to follow
  (`TestUnifiedConfigSwitcherSharesFocus`). Plans stay written to the focused project.
- **`@name` token (D3=A).** `matchFilter` now matches project OR source
  (`TestMatchFilterMatchesProjectOrSource`); single-repo literal-match parity preserved.
- **Standards.** Version bumped; no new verb (enum-sync untouched); no skill-bash change
  (`TestSkillsBashNoUnsafeRm` green); docs synced across `README.md`,
  `docs/cli-contract.md`, `skills/gogo-cli/SKILL.md` (p = project chip, `@name` project-or-source);
  CLI stays a deterministic LLM-free reader writing only `~/.gogo/`.
