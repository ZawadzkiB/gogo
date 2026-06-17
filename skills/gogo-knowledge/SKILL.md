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
1. **Finalize the plan.** Update `.plans/feature-<slug>/plan.md` to the as-built
   state (what actually shipped vs the original). Set `state.md` phase=done,
   status=done, resume=none.
2. **Update gogo-owned knowledge — never the originals.** Walk `.gogo/knowledge/*`
   and apply drift learned this feature, respecting each file's `Mode`:
   - `Mode: owned` → edit freely (e.g. add a verified gotcha to
     `code-review-standards.md`; add a journey to `test-strategy.md`; record a new
     bar in `non-functional-requirements.md`).
   - `Mode: proxy` → edit **only** the summary / `## gogo overrides` section, with
     gogo-specific notes. **Do NOT edit the linked `Source:` files** (the
     project's CLAUDE.md, README, etc.). If a change really belongs upstream,
     don't rewrite it silently — add a note and **surface a suggestion** to the
     user: "Consider adding X to CLAUDE.md."
   - The only files you may write are under `.gogo/` and `.plans/`.
3. **Re-render charts** via `gogo-mermaid` if the design changed.
4. **Summarise to the user:** what was planned, what was implemented, what review
   found and how it resolved, what was tested (UI/CLI/API), and which knowledge
   docs you updated. List any "consider upstreaming" suggestions and any
   follow-ups.
