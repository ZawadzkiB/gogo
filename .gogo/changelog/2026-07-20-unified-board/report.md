# unified all-projects board — 0.23.0

**`gogo global` is now ONE board across every project (design 3a).** Instead of showing one
project at a time with a `p` switcher, the global cockpit aggregates *all* registered projects into
a single board — each card and changelog row tagged with its **`●project ●source`** origin, filter
by project. This is where "both project + source" finally reads, and it makes the multi-project
cockpit look like the design. Ships **0.23.0**.

## What changed

- **Aggregate every project.** `contract.LoadWorkspace(projects)` merges each project's work items
  into one workspace (newest-first) and stamps a new in-memory `Feature.Project` (no `.gogo`
  contract-file change). The header `N features · M projects` now counts everything; `gogo global`
  (and bare `gogo` outside a repo) opens this unified view.
- **`●project ●source` origin everywhere.** Board cards and changelog rows carry both the project
  and source color dots. When a single-source project shares its repo's name (project == source),
  the tag **dedupes to one `● name`**; when they differ (a multi-source project) it shows both
  (`●gogo ●gogo-cli`). The ticket name is prioritized — the tag shrinks (names → dots → drop) so a
  long project/source name never crushes or wraps the slug.
- **Filter by project.** A `project` chip row (`all · ●gogo · ●very-nice-mermaid`), `p` cycles
  project; the `@name` free-text filter now matches a project **or** a source. Plans and config act
  on the focused project (the chip selection).
- **Fixed a real cross-project regression.** The concurrency cap and file-watch now span every
  project's sources, so a card in a non-focused project is still capped + watched.

## The safety fix (found in review)

On a unified board a feature's identity was its slug alone, but slugs are unique only *per-source*.
With two projects sharing a slug this could **launch a session in the wrong repo** (and the confirm
dialog showed no path), collide multi-select, and defeat the cross-project ship guard. Fixed: the
launch intent now carries the focused card's own repo `Root` (and the confirm dialog prints it),
selection is keyed by a composite `Root+Slug`, and the ship guard computes from the real selection.

## Decisions (D1–D5)

First-class `Feature.Project` field · `gogo global` unified with no flag · project chips are the
sole `p`-cycled row (source via dot + `@name`) · plans/config focus the selected project · card tag
shows both project + source **names** (D5=B, deduped when equal).

## Review / test

- **Review:** CHANGES-REQUESTED → resolved. 1 major (the same-slug wrong-repo launch + selection +
  ship-guard) + 1 nit, both fixed and re-verified; everything else clean.
- **Test:** PASS. Live tmux e2e against the real binary with isolated multi-project fixtures — the
  same-slug launch was confirmed to anchor at the right repo, the cross-project cap fired, and the
  single-repo/single-project fallbacks stayed byte-for-byte.
- **Gates:** `gofmt`/`go vet`/`go test -race ./...` green; `gogo --version` → 0.23.0.

## Follow-ups

Plans-tab grouped-by-project (deferred) · the three 0.22.0 color nits · `A` plan-authoring polish.

Full audit trail: `.gogo/work/feature-unified-board/`.
