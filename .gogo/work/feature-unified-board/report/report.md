# Report — `unified-board` (0.23.0)

**`gogo global` is now ONE board across every project (design 3a).** Aggregate all registered
projects into a single cockpit, each card + changelog row tagged by `●project ●source`, filter by
project. This is the "look like the design" piece where "both project + source" finally reads.
Review APPROVE (after one major fix), test PASS. Ships **0.23.0**.

## Run status

Plan accepted 2026-07-20 (D1–D4=A, D5=B). Implement 2 rounds (the second: an origin-tag
dedup/slug-first fix caught by rendering, then the REV-001 same-slug fix). Review 1 round →
CHANGES-REQUESTED (1 major + 1 nit) → fixed + re-verified. Test 1 round PASS (live via tmux, real
binary, isolated fixtures). Gate green; `gogo --version` → 0.23.0.

## Planned vs shipped

| FR | Shipped |
|---|---|
| FR1 aggregate all projects | `contract.LoadWorkspace(projs)` merges each project's `LoadProject` newest-first, stamps a new in-memory `Feature.Project` (no `.gogo` contract change); `tui.NewCockpit(projs)` reads it; header `N features · M projects` counts all. |
| FR2 `●project ●source` origin (D5=B) | `originTag`: two names when project≠source, **deduped to one `●name` when project==source**; `fitOriginTag` is **slug-first** (ticket name reserved ~14 runes, tag shrinks names→dots→drops, never the slug; REV-006 no-wrap). Changelog rows too. |
| FR3 project filter (D3=A) | `projectChip` row (`all · ●proj · ●proj`), `p` cycles project; the source-chip row retired; `@name` matches project OR source. |
| FR4 plans/config focus (D4=A) | board chip + config switcher share one `m.project`; `all` → `allProjects[0]`. |
| FR5 cross-project cap/watch | `capBounce`/`watchDirs` resolve from `projects.AllSources(allProjects)` — a card in a non-focused project is still capped + watched (fixed a real regression). |
| FR6 version | 0.23.0; no new verb → enum-sync untouched; docs synced. |

## The REV-001 fix (major) — same-slug across projects

A feature's identity was its slug alone, but slugs are unique only per-source. Fixed: `launch.Intent`
gained a `Root`; `attemptAction` stamps it from the **focused/selected card's own** `f.Root` (no slug
re-lookup in `doLaunch`) — so a launch anchors at the card you're on, and `confirmSummary` now prints
the repo root. `m.selected` + all per-feature lookups are keyed by a composite `featureKey(f) =
Root+"\x00"+Slug`; `selectionSpansProjects` computes from the real selected features. REV-002: the
plan-member `spawnedFeature` lookup is now project-scoped.

## Review + test outcomes

- **Review:** CHANGES-REQUESTED → resolved. 1 major (REV-001 same-slug: wrong-repo launch +
  selection collision + defeated ship-guard) + 1 nit (REV-002) — both fixed + re-verified. Everything
  else verified clean (aggregation determinism, FR5 fix real, fallbacks byte-for-byte, D5 origin fit).
- **Test:** PASS, no new issues. Live tmux e2e: aggregation + chips + dedup/two-name dots; **REV-001
  all three manifestations closed live** (beta's confirm shows repoB; `space` selects one; guard
  fires); FR5 cross-project cap; fallbacks byte-for-byte; malformed project degrades without crash.

## Invariants held

CLI writes only `~/.gogo/`; `Feature.Project` is an in-memory overlay (no `state.md`/contract
change); LLM-free read path; single-repo + single-project fallbacks byte-for-byte; no new verb.

## Follow-ups

- Plans-tab grouped-by-project (deferred, D4 alternative). · The three 0.22.0 color nits + the
  earlier fast-follows remain. · `A` plan-authoring polish.

## TL;DR

`gogo global` = one board across all projects, `●project ●source` origin (deduped when equal),
project filter, cross-project cap/watch fixed, and a real same-slug wrong-repo-launch bug closed.
Ships 0.23.0. Full audit: `.gogo/work/feature-unified-board/`.
