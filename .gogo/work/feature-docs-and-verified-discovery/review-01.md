# Review snapshot — round 01

- **Feature:** Hosted docs site + code-verified discovery (`docs-and-verified-discovery`)
- **Phase:** ③ review · **round:** 1 · **date:** 2026-06-30
- **Scope reviewed:** `docs/_config.yml`; `docs/{index,commands,flow,agents,discovery,contracts}.md`; `docs/architecture.md` (front matter); `.claude-plugin/plugin.json`; `README.md`; `commands/build.md`; `skills/gogo-build/SKILL.md`; `templates/knowledge/_discovered.md`. Cross-checked against the real `commands/`, `agents/`, `skills/`, `templates/contracts/`, and the orchestrator skill.

## Verdict: APPROVE

No blockers and no majors. Three minor findings (polish / drift); none block the hand-off to ④ test.

## What was verified clean (the high-signal checks)

- **Pages layout (D4).** `_config.yml` is at `docs/_config.yml` (deploy-from-`/docs`); there is **no** root `_config.yml` and **no** stale `exclude:` list; `url` + `baseurl: /gogo` resolve to `https://zawadzkib.github.io/gogo/`; `remote_theme: just-the-docs/just-the-docs`; mermaid enabled via `mermaid.version`. Correct.
- **Front matter.** Every `docs/*.md` (incl. `architecture.md`) has valid just-the-docs front matter; `nav_order` 1–7 are unique and sensible; no parent/child nesting to mis-wire.
- **Content accuracy.** Commands page lists exactly the 10 real `commands/*.md`; agents page I/O matches the phase skills + contracts; flow page matches `skills/gogo/SKILL.md`; contracts page matches all four schemas in `templates/contracts/`; discovery page matches the updated `gogo-build` incl. the Part B verify pass; architecture file map matches the real `templates/`, `hooks/`, `assets/`, `agents/`, `skills/`.
- **Links/anchors.** All internal links are relative `.md` (resolve via the always-on `jekyll-relative-links`); anchors `#knowledge--on-demand-skills`, `#knowledge-maintenance`, `#2-the-knowledge-vs-on-demand-skills-split` match their kramdown-generated targets; no absolute `/docs/...` links. Three mermaid fences present (flow, agents, discovery) per FR3.
- **Part B.** `gogo-build` Step 5 verify pass is pure (Glob/Grep/Read, only `.gogo/` written), code-wins, never edits the upstream `Source:` file (surfaces a stale-upstream suggestion), idempotent; step renumbering 1–7 is consistent (no dangling refs); file is 163 lines (under the 200 budget); `_discovered.md` + its template mirror the verified/corrected/unverifiable section and the "code is source of truth" principle.
- **Invariants/sync.** `${CLAUDE_PLUGIN_ROOT}` used for in-plugin paths; version bumped 0.3.0 → 0.4.0; command/agent/phase enumerations consistent across README/docs/skills; README "Documentation" link points at the correct Pages URL.

## Findings

| id | sev | pri | status | tag | file:line | finding | suggested fix |
|---|---|---|---|---|---|---|---|
| REV-001 | minor | P2 | new | AGENT-FIXABLE | `.gogo/knowledge/project-knowledge.md:80-81` | Self-knowledge still says the docs site has "`_config.yml` at the repo root" — contradicts the D4 reconciliation (config is `docs/_config.yml`, deploy-from-`/docs`; no root config on disk). The feature is literally about doc accuracy. | Change to "`docs/_config.yml` (Pages deploys from branch `main`, folder `/docs`)" so self-knowledge matches the real repo. |
| REV-002 | minor | P3 | new | AGENT-FIXABLE | `docs/agents.md:50-51,69-70` | The issues-list → developer edge is drawn twice: dotted `RISS/TISS -.->|fix mode| DEV` and solid `RISS/TISS ==> DEV`. Renders parallel duplicate arrows for one relationship and contradicts the diagram's own legend. | Keep only the dotted `-.->|fix mode|` edges; drop the solid `==> DEV` pair (lines 69-70). |
| REV-003 | minor | P2 | new | NEEDS-USER-DECISION | `docs/agents.md:11` vs `agents/gogo-developer.md:22`, `agents/gogo-reviewer.md:35-42` | agents.md cites `agents/*.md` as authoritative and uses the `issues.json` contract, but those role files still describe the older `review-NN.md`/`test-NN.md` flow (pre-contract-layer drift). Docs are correct to the live contract; the agent files lag. | Sync the agent role files to the `issues.json` contract, or consciously defer (the `agents/*.md` are outside this feature's stated file scope) — a scope call. |

## Route

Findings are all minor; none are `open`/`new` blockers or majors. Per the review router this is **clean for advancement**: REV-002 and REV-001 are quick agent-fixable polish that may be batched, and REV-003 is a scope decision for the user. No blocking re-implement is required before ④ test.
