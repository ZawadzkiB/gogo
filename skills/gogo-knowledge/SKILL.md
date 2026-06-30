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

Standalone, `/gogo:report <slug>` also (re)generates a report for a **past,
broken, or incomplete run** — see the lenient mode in validate-in below.

## Inputs (declared) and outputs

| Direction | Artifact | Contract |
|---|---|---|
| in (required) | `plan.md`, `state.md` | prose / human state |
| in (optional) | `review/issues.json`, `test/issues.json` | `issues-list.schema.json` |
| in (optional) | `charts/manifest.json` | `charts-manifest.schema.json` |
| in (optional) | `decisions.md` + the implement rounds | prose audit trail |
| out | `report/report.md`, the as-built UML set + `report/diagrams.html`, updated `.gogo/knowledge/*` | prose / diagrams |
| out | `report/manifest.json` (the as-built set index) | `charts-manifest.schema.json` |
| out | `report/result.json` (per run) | `phase-result.schema.json` |

## ① validate-in (gate — FR2) — strict (pipeline) vs lenient (standalone)

Phase ⑤ has two gates; they differ **only here**, in what they require to run.

- **Strict (in-pipeline).** When the orchestrator calls ⑤ right after a green ④:
  via `gogo-contracts`, confirm tests are all-green (`state.md` shows test done /
  no `open`/`new` issues in `test/issues.json`) and `plan.md` exists. If
  `review/issues.json`, `test/issues.json`, or `charts/manifest.json` are present,
  validate each against its schema before reporting on it. Not yet green / a
  required input missing → **STOP** with a precise contract error (tell the user
  to finish ④ test first). Don't certify an incomplete feature as a clean release.
- **Lenient (standalone — past/broken/incomplete runs).** When invoked explicitly
  via `/gogo:report <slug>` on a feature that is **not** cleanly green, do **not**
  refuse. Synthesize a **best-effort** `report/report.md` from whatever artifacts
  exist in `.gogo/work/<slug>/` — `plan.md`, `decisions.md`, `review/issues.json`
  (+ `review-NN.md`), `test/issues.json` (+ `test-NN.md`), `state.md`, `charts/`,
  any `implement/result.json` — and **clearly mark what completed vs what is
  missing/open** (the **Run status / gaps** section, Step 3). Validate any typed
  artifact that is present against its schema, but a missing or red one is
  **reported as a gap, not a STOP**.

**The one true prerequisite (both modes): `plan.md` must exist.** If even
`plan.md` is missing → **STOP** (there is no contract to report against).

## Steps
1. **Finalize the plan.** Update `.gogo/work/feature-<slug>/plan.md` to the as-built
   state (what actually shipped vs the original).
   - **Strict (green run):** set `state.md` phase=done, status=done, resume=none.
   - **Lenient (past/broken run):** don't pretend a clean release. Write the report
     (so `/gogo:done` and `/gogo:view` can find it), but keep `state.md` honest by
     **leaving `phase` and `status` at their real pre-report values** — a broken run
     is NOT `done`, so never stamp `phase: done` or `status: done` for it (that would
     make the phase/status pair lie). Update only `resume:` to record that a
     report-only was written and the gaps remain (e.g. `resume: report-only written
     — N open issues remain; see report Run status / gaps`). The report's **Run
     status / gaps** section is the source of truth for completeness.
2. **Draw the as-built UML set** via `gogo-mermaid`, into the feature's **`report/`**
   subfolder (not `charts/`). Diagram the *shipped code and behaviour* — never the
   gogo phases or the plan's task checklist (FR1→FR2→… is a to-do list, not a system
   diagram). **Choose the kinds by what the diff changed** (per gogo-mermaid's
   "choose the kinds by what changed" rule); produce only the ones that carry signal
   (skip any that would be trivial; if the feature was pure process — docs / tests /
   merge — draw nothing and note it), each as a fenced block (in `report/report.md`),
   a `.mmd` in `report/`, and a refreshed `report/diagrams.html`:
   - **Class** — new/changed types, modules, relationships → `classDiagram`.
   - **Sequence** — a new runtime interaction (caller → modules → store / API → back)
     → `sequenceDiagram`.
   - **Activity** — new states, status transitions, or an action flow →
     `stateDiagram-v2` (or an activity-style flowchart).
   - **Use-case** — a new user-facing capability → a flowchart actor↔use-case graph.
   - **Flow** — the implemented change / data flow / the architecture it touches →
     `flowchart`.

   Update the plan's diagrams where they still hold; add the as-built ones. Name
   files per kind under `report/` (`flow.mmd` / `sequence.mmd` / `activity.mmd` /
   `class.mmd` / `use-case.mmd`); the manifest `kind` must be one of
   `{flow, sequence, class, activity, use-case}`. The `report/diagrams.html` viewer
   loads the shared runtime at `../../../resources/mermaid.min.js` (`report/` is the
   same depth as `charts/`). **Write `report/manifest.json`** so its `diagrams[]`
   (kind/file/title) match the `.mmd` set on disk (empty `diagrams` + a `note` if you
   drew nothing). **In lenient mode** draw only what the *available* artifacts
   support (e.g. reuse the plan's `charts/` and whatever shipped) and **skip any kind
   you can't derive** from a partial run — an empty/partial set is fine; note it.
3. **Write the final report.** Copy `${CLAUDE_PLUGIN_ROOT}/templates/report.template.md`
   → `.gogo/work/feature-<slug>/report/report.md` (NOT the feature root) and fill
   every section from the actual work:
   - **Run status / gaps** — which phases ran (plan / implement / review / test /
     report) and which didn't, plus any still-open or unverified issues. For a clean
     green run this is one line ("all phases completed; no open issues"). **In
     lenient mode this section is required and must be honest**: enumerate what's
     missing/incomplete and list every open `review/issues.json` / `test/issues.json`
     finding so the reader knows the report is best-effort, not a clean release.
   - **Summary** and **planned-vs-shipped**.
   - **Implementation** — what was *actually built* (the real as-built changes, the
     approach taken), plus the as-built changes table. (Lenient: describe what
     exists; mark anything unverified.)
   - **Decisions & rationale** — reconcile `decisions.md` with the choices made during
     the implement rounds; for **each** decision record both the **choice** and the
     **reason** it was made.
   - **Review outcome**, **test outcome**, the **diagrams** (link `./diagrams.html`,
     same folder), **knowledge updates**, and **follow-ups**.

   Link the audit-trail files (`../decisions.md`, `../review-NN.md`/`../review/issues.json`,
   `../test-NN.md`/`../test/issues.json`) rather than repeating them. This is the
   durable companion to `plan.md`, and the bundle `/gogo:done` archives to the changelog.
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
   - The only files you may write are under `.gogo/` (config in `.gogo/knowledge/`, work in `.gogo/work/`).
5. **Summarise to the user:** what was planned, what was implemented, what review
   found and how it resolved, what was tested (UI/CLI/API), and which knowledge
   docs you updated. Point them at `report/report.md` and `report/diagrams.html`.
   List any "consider upstreaming" suggestions and any follow-ups. The feature is
   now report-complete — the user can run `/gogo:done` to ship it to the changelog.

## validate-out (FR3)

Via `gogo-contracts`: validate the written `report/manifest.json` against
`charts-manifest.schema.json` (it IS schema-governed; the `.mmd`/`.html` viewer
are the unschematized prose/visual artifacts). Then write `report/result.json`
(`phase: report`, `status: ok`, `inputs`, `outputs`, `validated_in: true`,
`validated_out: true`, `summary`). `report/report.md` is a prose artifact; the
result record is the machine state for chaining. Set `state.md` per Step 1: a
clean run → phase=done, status=done, resume=none; a lenient/past-broken run →
leave `phase`/`status` at their real pre-report values (never `done`) and update
only `resume:` with an honest report-only + gaps note. In lenient mode,
`result.json`'s `summary` should name the gaps.
