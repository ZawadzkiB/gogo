# Review — `immediate-kill-at-ship` · round 01

Fresh-eyes review (phase ③) of the scoped patch, against `plan.md` (FR1–FR5,
D1=A/D2=A/D3=A), `code-review-standards.md`, `coding-rules.md`, and
`non-functional-requirements.md`. Gates re-run locally: `gofmt -l .` clean ·
`go vet ./...` clean · `TestSweepSparesSelf` PASS · `TestSkillsBashNoUnsafeRm` PASS.

## Scope reviewed
`cli/internal/orchestrator/sweep.go` (Self seam + skip-self) · `orchestrator_test.go`
(`TestSweepSparesSelf`) · `cli/internal/launch/launch.go` (`CurrentSession()`,
dropped `remain-on-exit`, doc comments) · `cli/go.go` (`cmdSweep` wiring) ·
`skills/gogo-done/SKILL.md` (step 6 ship-reap + Degradation bullet) ·
`docs/cli-contract.md` (additive 0.17.0 block) · version bump (plugin.json +
main.go → 0.17.0). Out-of-scope cockpit-cards hunks were ignored.

## Verified solid (no finding)
- **FR3 self-guard is correct.** `Sweep()` skips `sess == sw.Self` *before*
  `shouldReap` (sweep.go:53). `Self==""` (no `$TMUX`) never matches a real session
  (`ListSessions` only returns non-empty `gogo-*` names) — reaping is unaffected off-tmux.
  **Collision suffix is safe:** `CurrentSession()` and `ListSessions()` both source
  the *actual* live name from tmux, so a renamed host (`gogo-done-x-2`) still matches
  the guard by exact string equality — no residual self-kill via the suffix.
  `TestSweepSparesSelf` genuinely exercises the guard (would fail without it).
- **FR1/FR2 reap is best-effort + classifier-safe.** `command -v gogo … && gogo sweep … || true`
  can never return non-zero (short-circuit + trailing `|| true`), robust under `set -e`;
  no `rm` (guard test green). Ordering is correct: step 5 flips `state.md → shipped`
  and a fresh `gogo` process re-reads it from disk before reaping — the "order matters"
  note is accurate and load-bearing. The step writes nothing under `.gogo/`.
- **FR4 drop is safe; no dead-pane path left.** `Launch()` is the *only* board
  launcher (`m.launcher` default, model.go:141) — no other production caller. A gate
  keeps interactive claude (and its pane) alive; `remain-on-exit` only ever kept a
  *dead* pane. Grep confirms no production path still sets it. Doc comments now match.
- **Injection safety.** `CurrentSession()` is single-argv, no shell, no user input
  (`exec.Command("tmux","display-message","-p","#S")`); `$TMUX`/`HasTmux()` guards correct.
- **Contract/enumeration sync.** `docs/cli-contract.md` 0.17.0 block is additive,
  correctly placed, heading level matches siblings; versions paired at 0.17.0; the
  `main.go`/`README` sweep enumerations remain accurate (not contradicted).

## Findings

### REV-001 · minor · P3 · NEEDS-USER-DECISION · status: new
**Shipped card keeps a live "running" badge from its own lingering `gogo-done-<slug>`
session (acceptance signal not literally met).** `/gogo:done` runs inside an
*interactive* board-launched `gogo-done-<slug>` session, which the FR3 self-guard
(correctly) spares. Interactive claude idles at its prompt after Return, so that
session stays live; `badge()` → `SessionMatchesSlug("gogo-done-<slug>","<slug>")` is
true, so the shipped card renders "running" until the user quits (FR4 then closes the
pane) or an out-of-band sweep reaps the now-terminal feature. The DRIVING
`gogo-go-<slug>` reap (the headline win) is delivered, but the plan's acceptance
signal ("no live `gogo-*` session for the shipped slug") is not literally met for the
hosting done session. Inherent to self-reaping — not a code defect.
*Fix:* soften the plan's acceptance wording to the driving session, or (follow-up)
suppress the badge on a shipped card whose only live session is its own done host.
Do **not** self-reap the host.

### REV-002 · minor · P2 · NEEDS-USER-DECISION · status: new
**A ship's best-effort sweep can truncate a DIFFERENT feature's concurrent
`/gogo:done`.** The self-guard is exact-name only. Plain `gogo sweep` still reaps
every live `gogo-*` session whose owning feature is terminal — including a *concurrent*
`gogo-done-z` when feature `z` has already flipped to `shipped` (step 5) but is still
finishing its own steps 6–7. Verified: `gogo-done-z != Self`, `owningFeature=z`,
`TerminalStatus(z)=true` → `Kill(gogo-done-z)`. The per-feature lock does not prevent
two done sessions running at once. Impact is bounded (z is already durably shipped;
only its best-effort viewer + Return tail are lost), but this is a **new** hazard this
feature introduces — the plan/D1=A "collateral is safe by definition" premise has a
transient-done-session exception. *Fix:* spare all live `gogo-done-*` sessions, or
adopt the recorded D1=B targeted `gogo sweep <slug…>`; at minimum correct the
"safe by definition" claim. Route to the user (scope change beyond accepted D1=A).

## Verdict

**APPROVE** — no open blockers or majors. FR1–FR5 are implemented correctly and the
required `cli/` gates are green. The two findings are minor, inherent design tensions
tagged NEEDS-USER-DECISION (plan-wording / a narrow, low-impact concurrency edge);
neither gates the ship. Route them to the user for a wording/scope call rather than a
code loop.
