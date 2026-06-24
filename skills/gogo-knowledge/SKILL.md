---
name: gogo-knowledge
description: >-
  Phase ⑤ of the gogo pipeline — the success/report step. After tests pass,
  reconcile the plan to as-built and update the project's gogo-owned knowledge
  docs (never the proxied upstream files), then summarise. Also usable when the
  user says "update the docs / knowledge".
---

# gogo-knowledge — phase ⑤ (report & feedback loop)

Run after phase ④ is all-green — either as the final orchestrator step or
**standalone** via `/gogo:report <slug>`. This is the diagram's "update plan and
all knowledge docs" arrow back to the top.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `plan.md`, `state.md` | prose / human state |
| in (optional) | `review/issues.json`, `test/issues.json` | `issues-list.schema.json` |
| in (optional) | `charts/manifest.json` | `charts-manifest.schema.json` |
| out | `report.md`, refreshed `charts/`, updated `.gogo/knowledge/*` | prose / diagrams |
| out | `charts/manifest.json` (re-written to match the refreshed set) | `charts-manifest.schema.json` |
| out | `report/result.json` (per run) | `phase-result.schema.json` |

## ① validate-in (gate — FR2)

Via `gogo-contracts`: confirm tests are all-green (`state.md` shows test done /
no `open`/`new` issues in `test/issues.json`) and `plan.md` exists. If
`review/issues.json`, `test/issues.json`, or `charts/manifest.json` are present,
validate each against its schema before reporting on it. Not yet green / a
required input missing → **STOP** with a precise contract error (tell the user to
finish ④ test first). Don't report on an incomplete feature.

## Steps
1. **Finalize the plan.** Update `.gogo/plans/feature-<slug>/plan.md` to the as-built
   state (what actually shipped vs the original). Set `state.md` phase=done,
   status=done, resume=none.
2. **Draw the as-built diagram set** via `gogo-mermaid`. Diagram the *shipped code
   and behaviour* — never the gogo phases or the plan's task checklist (FR1→FR2→…
   is a to-do list, not a system diagram). Produce the ones that carry signal (skip
   any that would be trivial; if the feature was pure process — docs / tests /
   merge — draw nothing and note it), each as a fenced block (in `report.md`), a
   `.mmd` in `charts/`, and a refreshed `charts/diagrams.html`:
   - **Flow** — `flowchart` of the implemented change / data flow / the
     architecture it touches.
   - **Sequence** — `sequenceDiagram` of the key runtime interaction(s) the change
     introduces (caller → modules → store / API → back).
   - **Activity** (lifecycle / state / action flow) — `stateDiagram-v2` (or an
     activity-style flowchart) for any new state machine, status transitions, or
     user-action flow.
   - **Class** (structure / types, when useful) — `classDiagram` / component view
     of the new or changed types/modules and their relationships.

   Update the plan's diagrams where they still hold; add the as-built ones. Name
   files per kind (`flow.mmd` / `sequence.mmd` / `activity.mmd` / `class.mmd`); the
   manifest `kind` must be one of `{flow, sequence, class, activity}`. **Re-write
   `charts/manifest.json`** so its `diagrams[]` (kind/file/title) match the
   refreshed `.mmd` set on disk (empty `diagrams` + a `note` if you drew nothing).
3. **Write the final report.** Copy `${CLAUDE_PLUGIN_ROOT}/templates/report.template.md`
   → `.gogo/plans/feature-<slug>/report.md` and fill every section from the actual
   work: summary, planned-vs-shipped, the as-built changes table, decisions,
   review outcome, test outcome, the diagrams (link `charts/diagrams.html`),
   knowledge updates, and follow-ups. Link the audit-trail files
   (`decisions.md`, `review-NN.md`/`review/issues.json`, `test-NN.md`/`test/issues.json`)
   rather than repeating them. This is the durable companion to `plan.md`.
4. **Update gogo-owned knowledge — never the originals.** Walk `.gogo/knowledge/*`
   and apply drift learned this feature, respecting each file's `Mode`:
   - `Mode: owned` → edit freely (e.g. add a verified gotcha to
     `code-review-standards.md`; add a journey to `test-strategy.md`; record a new
     bar in `non-functional-requirements.md`).
   - `Mode: proxy` → edit **only** the summary / `## gogo overrides` section, with
     gogo-specific notes. **Do NOT edit the linked `Source:` files** (the
     project's CLAUDE.md, README, etc.). If a change really belongs upstream,
     don't rewrite it silently — add a note and **surface a suggestion** to the
     user: "Consider adding X to CLAUDE.md."
   - The only files you may write are under `.gogo/` (config in `.gogo/knowledge/`, work in `.gogo/plans/`).
5. **Summarise to the user:** what was planned, what was implemented, what review
   found and how it resolved, what was tested (UI/CLI/API), and which knowledge
   docs you updated. Point them at `report.md` and `charts/diagrams.html`. List any
   "consider upstreaming" suggestions and any follow-ups.

## validate-out (FR3)

Via `gogo-contracts`: validate the re-written `charts/manifest.json` against
`charts-manifest.schema.json` (it IS schema-governed; the `.mmd`/`.html` viewer
are the unschematized prose/visual artifacts). Then write `report/result.json`
(`phase: report`, `status: ok`, `inputs`, `outputs`, `validated_in: true`,
`validated_out: true`, `summary`). `report.md` is a prose artifact; the result
record is the machine state for chaining. Set `state.md` phase=done, status=done,
resume=none.
