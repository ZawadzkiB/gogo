# Review — round 2 (Stage 2: FR4–FR7)

- **feature:** `workspace-changelog-viewer`
- **scope:** Stage 2 only — FR4 (report/ bundle), FR5 (richer report content),
  FR6 (use-case + chosen-by-diff UML), FR7 (`.gogo/changelog/` + `/gogo:done`)
- **reviewed:** new `commands/done.md`, `skills/gogo-done/SKILL.md`; modified
  `templates/contracts/charts-manifest.schema.json`, `skills/gogo-mermaid`,
  `skills/gogo-knowledge`, `templates/report.template.md`, `templates/state.template.md`,
  `skills/gogo/SKILL.md`, `docs/flow.md`, `docs/agents.md`, `docs/architecture.md`,
  `docs/commands.md`, `commands/report.md`, `README.md`
- **date:** 2026-06-30
- **deferred (not flagged):** plugin.json version bump and the command-count
  enumerations (commands.md groups intro, architecture.md plugin command-file list,
  README command list) — explicitly deferred to the FR11 cross-cutting sweep.

## Findings this round

| id | severity | priority | status | file:line | finding | proposed fix | tag |
|---|---|---|---|---|---|---|---|
| REV-003 | major | P1 | new | docs/contracts.md:55-56 · templates/contracts/README.md:55-56 | Chart-kind enum left out of sync with the FR6-extended schema: both contract descriptions still say `kind ∈ {flow, sequence, class, activity}` and `file (a .mmd under charts/)`, missing `use-case` and the `report/` location. The schema (the real contract) was updated; its two prose mirrors were not. | Add `use-case` to both enum lists; broaden the file note to `a .mmd under charts/ (or report/ for the ⑤ bundle)`; note ⑤ also emits `report/manifest.json` under the same schema. | AGENT-FIXABLE |
| REV-004 | nit | P3 | new | templates/contracts/charts-manifest.schema.json:5 | Schema top-level `description` still frames the manifest as only implement ② → `charts/`; the same schema now also governs `report/manifest.json` at ⑤ (validate-out validates it). The `file` property description was broadened, the top-level one was not. | Broaden the top-level `description` to mention the ⑤ `report/manifest.json` report bundle. | AGENT-FIXABLE |

## Prior findings (carried, verified)

| id | severity | status | note |
|---|---|---|---|
| REV-001 | nit | verified | architecture.md `resources/` comment alignment — fixed in round 2. |
| REV-002 | minor | verified | gogo-build partial-migration log no longer misreports a conflict as no-op — fixed in round 2. |

## What was checked and passed

- **FR6 schema (use-case):** enum is exactly `["flow","sequence","class","activity","use-case"]`,
  `additionalProperties:false` preserved at both object levels, `kind`/`file` descriptions
  define use-case and the report/ location; `gogo-mermaid` adds a real flowchart
  actor↔use-case pattern plus the "choose the kinds by what changed" table. No drift in
  `gogo-knowledge` / `gogo-mermaid` themselves (both list the 5-kind set). Drift exists only
  in the two contract-doc mirrors → REV-003.
- **FR4/FR5 report bundle:** `gogo-knowledge` ⑤ writes `report/report.md` + `report/<kind>.mmd`
  + `report/diagrams.html` + `report/manifest.json` (+ existing `report/result.json`); the
  viewer runtime path `../../../resources/mermaid.min.js` is correct (`report/` is the same
  depth as `charts/`). `report.md` now mandates Implementation + Decisions & rationale
  (choice + reason) sections; `templates/report.template.md` matches (Implementation section,
  Decisions & rationale table with Decision/Choice/Reason). Report `../` link rewrites
  (decisions/review/test) are correct for the deeper `report/` location.
- **report/manifest.json addition — judged SOUND, not over-reach.** It is the natural typed
  index of the bundle, structurally valid under the reused `charts-manifest.schema.json`
  (`slug`+`diagrams`, `file` pattern `\.mmd$` accepts report/ paths), and is in fact REQUIRED
  to keep the ⑤ validate-out gate meaningful: ⑤ no longer rewrites `charts/manifest.json`, so
  without `report/manifest.json` there'd be no ⑤-produced manifest to validate. Only clarity
  gap is the schema's top-level description → REV-004 (nit).
- **FR7 done/changelog:** `gogo-done` validate-in requires `report/report.md`; copies
  (never moves) report.md + the `.mmd` set + diagrams.html (+ manifest); date derived
  (report `completed:` → user arg → `date +%F`) and the `completed:` field really exists in
  `report.template.md:12`; idempotent (overwrites same dated dir); sets `state.md`
  `status: shipped`, `resume: none`; only writes under `.gogo/`. `commands/done.md` is thin
  (logic in the skill) with valid frontmatter + a validate-in gate. Flow note placing
  `/gogo:done` after ⑤ present in `skills/gogo/SKILL.md` and `docs/flow.md`.
- **Enumeration sync (in scope):** every feature-root `report.md` reference is now
  `report/report.md` / the `report/` bundle (gogo SKILL, state.template, architecture map +
  ⑤ line, agents.md, commands.md, README "What gets created"); `.gogo/changelog/` entry is in
  the file maps; `shipped` added to the state status enum. (The `charts/` folder descriptions
  in README/gogo SKILL still list the 4-kind implement set — correct, that's the ② set, not
  the ⑤ enum.)
- **Hard invariants:** `${CLAUDE_PLUGIN_ROOT}` used for template/asset paths; `gogo-done` is
  pure/portable and `.gogo/`-only; commands stay thin; no new dependencies; gates intact.

## Verdict

**CHANGES** — one major (REV-003) blocks. REV-004 is a nit. No blockers; FR4/FR5/FR7
are clean. Blocking id: **REV-003**.
