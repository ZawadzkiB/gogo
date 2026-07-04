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
| in (optional) | `charts/before/*.mmd` + `charts/before/manifest.json` (the plan-time "before" set, FR7) | `charts-manifest.schema.json` |
| in (optional) | `decisions.md` + the implement rounds | prose audit trail |
| out | `report/report.md`, the as-built UML set + `report/diagrams.html`, updated `.gogo/knowledge/*` | prose / diagrams |
| out | `report/manifest.json` (the as-built set index) | `charts-manifest.schema.json` |
| out (optional) | `report/before/*.mmd` + `report/before/manifest.json` (the before set copied in, FR8) | `charts-manifest.schema.json` |
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

**Append the phase-start event (telemetry).** As phase ⑤ begins, append one
compact JSON line to `.gogo/work/feature-<slug>/events.jsonl` per
`events.schema.json` (`${CLAUDE_PLUGIN_ROOT}/templates/contracts/`):
`{"ts":"<RFC3339>","event":"phase-started","phase":"report","status":"<state.md status at entry>","slug":"<slug>"}`.
Create the file if absent; **best-effort** — never fail the phase if the append
fails (append-only telemetry; `state.md` stays the human resume file). Note events
call this phase `report` even though `state.md` labels it `knowledge`. This skill
owns both `phase-started`/report and `phase-done`/report (below); the orchestrator
emits neither.

1. **Finalize the plan.** Update `.gogo/work/feature-<slug>/plan.md` to the as-built
   state (what actually shipped vs the original).
   - **Strict (green run):** set `state.md` phase=done, **status=awaiting-uat** (the
     UAT gate — the plan-gate symmetry, from 0.11.0: ⑤ no longer ends at `done`, it
     hands the work to the user to verify), resume=`awaiting UAT — verify the work;
     /gogo:done accepts, or describe issues to loop back`. Running `/gogo:done` is the
     acceptance and is what flips the feature to `shipped` (via `gogo-done`); UAT
     feedback instead re-plans the SAME item (see `gogo`'s UAT loop) and reruns ②→⑤.
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
   drew nothing).

   **Copy the plan-time "before" set into the bundle (FR8).** If `charts/before/`
   exists (the as-is baseline plan ① drew), copy its `*.mmd` **and** `manifest.json`
   into **`report/before/`** so the report bundle is **self-contained** — the archive
   `/gogo:done` ships and `/gogo:view` compare mode both read the before set locally,
   with no dependency on the `charts/` folder. In the copied
   `report/before/manifest.json`, **rewrite each `file` to point at the copied
   location** — `report/before/<kind>.mmd` (or the bare `<kind>.mmd` basename) —
   so the archived manifest doesn't dangle back into `charts/`. This is a
   path-string update only; the schema shape is unchanged (D5 forbids a *schema*
   change, not correcting a path). If there is **no** `charts/before/` (the feature
   predates this, or the plan drew none), note that and produce only the after set —
   the comparison below then just shows the after diagrams.

   **In lenient mode** draw only what the *available* artifacts
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
   - **Summary (TL;DR)** — the FINAL section (`## Summary (TL;DR)`, at the very
     end): a few bold-led lines closing the report — **what shipped**, the review
     and test **verdicts** (one line each), and a pointer to the **follow-ups**.
   - **Before / after comparison** (FR8) — if a `report/before/` set exists, add a
     comparison section: for each kind present in **both** the before and after sets,
     show the two diagrams **side by side** (fenced mermaid blocks — before then
     after) with a short prose **"what changed"**; call out any kind that was
     **added** (after only) or **removed** (before only). If there is no before set,
     say so in one line and show only the after set. This is **side-by-side + prose
     only** — do **not** compute a structural node-diff (decision D4=A).

   Link the audit-trail files (`../decisions.md`, `../review-NN.md`/`../review/issues.json`,
   `../test-NN.md`/`../test/issues.json`) rather than repeating them. This is the
   durable companion to `plan.md`, and the bundle `/gogo:done` archives to the changelog.

   **Write it like a readable article (FR3 — legibility, keep the sections above).**
   `report.md` is what `/gogo:view` and the changelog surface to a human, so author
   it to be *read*, not just recorded — phrasing/emphasis only, **not** new sections
   beyond the closing `## Summary (TL;DR)` (D4=A):
   - **Lead each section with a 1-2 sentence summary** (open the report with a crisp
     "what shipped and why") before the detail.
   - **Short, scannable sections** — tight paragraphs, lists, and tables over walls
     of text.
   - **Bold the decisions, outcomes, and key terms** so a skim surfaces them.
   - Plain language; define a term once, then reuse it.
   - **Close with `## Summary (TL;DR)`** — the final section: what shipped, the
     review/test verdicts (one line each), and a follow-ups pointer, so a skim of
     just the opening lead and this closing block conveys the whole report.
   The viewer renders this with article typography (readable measure, styled
   headings, a lead paragraph, visible emphasis).
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
   - **Never touch a `## Custom` section** (any mode). It is user-owned and copied
     1:1 — the same rule `/gogo:build` follows for `## gogo overrides`. Leave it
     byte-for-byte; only ever edit gogo-authored regions.
   - The only files you may write are under `.gogo/` (config in `.gogo/knowledge/`, work in `.gogo/work/`).
5. **Summarise to the user:** what was planned, what was implemented, what review
   found and how it resolved, what was tested (UI/CLI/API), and which knowledge
   docs you updated. Point them at `report/report.md` and `report/diagrams.html`.
   List any "consider upstreaming" suggestions and any follow-ups. The feature is
   now report-complete and sits at the **UAT gate** (`status: awaiting-uat`): tell
   the user to **verify the work**, then either run `/gogo:done` (which IS the
   acceptance — it ships to the changelog) **or** describe any issues/questions to
   loop back into planning (the orchestrator's UAT loop re-plans the SAME item via
   `uat.md` and reruns ②→⑤).

## validate-out (FR3)

Via `gogo-contracts`: validate the written `report/manifest.json` against
`charts-manifest.schema.json` (it IS schema-governed; the `.mmd`/`.html` viewer
are the unschematized prose/visual artifacts). If a before set was copied in,
validate `report/before/manifest.json` against the **same** schema too (it was
carried over verbatim — decision D5, no schema change). Then write `report/result.json`
(`phase: report`, `status: ok`, `inputs`, `outputs`, `validated_in: true`,
`validated_out: true`, `summary`). `report/report.md` is a prose artifact; the
result record is the machine state for chaining. Set `state.md` per Step 1: a
clean run → phase=done, **status=awaiting-uat**, resume=the UAT hint; a
lenient/past-broken run → leave `phase`/`status` at their real pre-report values
(never `done` and never `awaiting-uat`) and update only `resume:` with an honest
report-only + gaps note. In lenient mode, `result.json`'s `summary` should name
the gaps.

**Append the phase-done event (telemetry).** Beside that `state.md` write, append
one line to `.gogo/work/feature-<slug>/events.jsonl` (best-effort, per
`events.schema.json`) mirroring the status just written — clean green run →
`{"ts":"<RFC3339>","event":"phase-done","phase":"report","status":"awaiting-uat","slug":"<slug>"}`;
a lenient/past-broken run → the same with `status` set to the honest pre-report
value (never `done`, never `awaiting-uat`).
