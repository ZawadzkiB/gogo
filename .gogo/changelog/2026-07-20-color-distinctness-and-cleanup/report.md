# distinct colors + em-dash sweep + nits — 0.22.1

A tidy-up patch on top of 0.22.0's cockpit colors. Ships **0.22.1**.

## What changed

- **Distinct colors for colorless projects/sources (the "all blue" fix).** A project/source with
  no stored color (e.g. registered before 0.22.0) now falls back to a **name-stable** swatch
  (`ColorForName` = hash of the name), instead of a position index. So `gogo` and
  `very-nice-mermaid` get *different* colors (teal vs green) with no re-register, and a color no
  longer shifts when the list reorders.
- **Em-dash → plain dash** across the CLI help text + user-facing docs (`cli/*.go` command help,
  `README.md`, `docs/cli-contract.md`, `skills/gogo-cli/SKILL.md`) — ~338 `—` replaced. Command
  tokens untouched, so the enum-sync guard stays green.
- **Review nits from 0.22.0:** one shared `projects.TakenColors` helper (dedup across
  `project add` / `source add` / the config form); the config project-switcher `●P ●S` row is now
  width-truncated so a long name can't wrap.
- **Removed** the 8 superseded, never-released P1–P4 work/changelog folders from the tree.

## Review / test

Mechanical patch, kept green throughout: `gofmt`/`go vet`/`go test -race ./...` all pass; the
enum-sync + no-unsafe-rm guards stay green; `gogo --version` → 0.22.1.

Full audit trail: `.gogo/work/feature-cockpit-colors/` (the 0.22.0 feature these nits came from).
