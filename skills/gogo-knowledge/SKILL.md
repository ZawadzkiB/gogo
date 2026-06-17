---
name: gogo-knowledge
description: >-
  Phase ⑤ of the gogo pipeline — the success/report step. After tests pass,
  reconcile the plan to as-built and update the project's gogo-owned knowledge
  docs (never the proxied upstream files), then summarise. Also usable when the
  user says "update the docs / knowledge".
---

# gogo-knowledge — phase ⑤ (report & feedback loop)

Run after phase ④ is all-green. This is the diagram's "update plan and all
knowledge docs" arrow back to the top.

## Steps
1. **Finalize the plan.** Update `.gogo/plans/feature-<slug>/plan.md` to the as-built
   state (what actually shipped vs the original). Set `state.md` phase=done,
   status=done, resume=none.
2. **Draw the as-built diagram set** via `gogo-mermaid`. Diagram *what actually
   shipped*, not just the plan — produce the ones that carry signal (skip any that
   would be trivial), each as a fenced block (in `report.md`), a `.mmd` in
   `charts/`, and a refreshed `charts/diagrams.html`:
   - **Flow** — `flowchart` of the implemented change / data flow / the
     architecture it touches.
   - **Sequence** — `sequenceDiagram` of the key runtime interaction(s) the change
     introduces (caller → modules → store / API → back).
   - **Actions / lifecycle** — `stateDiagram-v2` (or an activity-style flowchart)
     for any new state machine, status transitions, or user-action flow.
   - **Structure** (when useful) — `classDiagram` / component view of the new or
     changed types/modules and their relationships.

   Update the plan's diagrams where they still hold; add the as-built ones.
3. **Write the final report.** Copy `${CLAUDE_PLUGIN_ROOT}/templates/report.template.md`
   → `.gogo/plans/feature-<slug>/report.md` and fill every section from the actual
   work: summary, planned-vs-shipped, the as-built changes table, decisions,
   review outcome, test outcome, the diagrams (link `charts/diagrams.html`),
   knowledge updates, and follow-ups. Link the audit-trail files
   (`decisions.md`, `review-NN.md`, `test-NN.md`) rather than repeating them. This
   is the durable companion to `plan.md`.
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
