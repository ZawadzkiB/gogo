---
description: Set up or refresh gogo for this project — discover docs and wire the knowledge config (idempotent; re-run anytime).
argument-hint: "[--force]"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Skill
---

Run the `gogo-build` skill for this project.

Arguments: $ARGUMENTS  (may contain `--force` to reset the knowledge files to fresh scaffolds).

Follow the skill: scaffold `.gogo/knowledge/` from the plugin templates if absent,
discover the project's existing docs (Claude / Copilot / Cursor / Windsurf / Codex
configs, README, manifests, test/CI configs) at every depth — including nested
monorepo packages — plus a sweep of all project markdown and in-code doc comments,
and wire each knowledge file as a proxy that links the real source — or synthesize
it from the codebase when none exists. On a re-run, **reconcile**: pick up newly-added docs and refresh summaries
while **preserving** every `## gogo overrides` section and every `Mode: owned`
file. Regenerate `_discovered.md`. Then report what was created vs kept, what was
linked, and tell the user they can now run `/gogo:plan "<goal>"`.
