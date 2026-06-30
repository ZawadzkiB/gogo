# Adjustments — feature `skill-extraction`

Running log of changes / clarifications requested during planning.

## 2026-06-29 — Scope add: full "how gogo works" documentation
User: "we also need to update readme or make a separate readme — full
documentation of how gogo is working now, the reason why we split knowledge and
flow, what files are stored and where, etc." Added as **FR12** + checklist item
10: a dedicated `docs/architecture.md` (linked from README) covering the
flow-vs-knowledge split (generic flow ships with the plugin; per-project rules in
`.gogo/knowledge/`), the NEW knowledge-vs-on-demand-skills split + its determinism
rationale, and the complete file map (plugin side + project `.gogo/` side +
standalone `.claude/skills/`). Chose a separate doc over bloating README; README
gains a short "How it works" pointer. Folded into this feature so it ships
together (handled in the same implement pass via the running gogo-developer).

## 2026-06-29 — D1 reframed: per-candidate kind, not a global toggle
User clarified the real fork: each extraction is either a **knowledge skill**
(needed only here, by the gogo pipeline → `.gogo/skills/`) or a **standalone
skill** (a self-contained, reusable capability worth Claude Code auto-discovery →
`.claude/skills/`). So D1 becomes a **per-candidate classification** the command
proposes (`kind` + `destination`) and the user confirms at the gate — not one
global location. `.claude/skills/` is written **only** for a candidate the user
approves as standalone (never automatic); knowledge skills keep the `.gogo/`-only
invariant. Updated: Goal, FR4/FR6/FR9 + new FR (classification), Approach, D1,
and the flow diagram (added a "classify kind" step + two destinations).
Clarification re: skill discovery — `.gogo/skills/` is NOT auto-loaded by the
harness (only plugin `skills/`, `~/.claude/skills/`, project `.claude/skills/`
are); a knowledge skill is loaded by the gogo agent reading the parent's pointer.
