# Review round 05 — plan-readiness-gate (ships as 0.29.0)

**Track:** review (③) · **Round:** 5 · **Date:** 2026-07-30 · **Reviewer:** fresh eyes, did not write this code

Scope per the coordinator: the two phase-④ round-3 fixes only — **TEST-002** (the decision-gate
move guard) and **TEST-003** (the wrap-insensitive standards guard) — plus verification of
round 04's four findings and the process spot-checks. Rounds 01-04's 24 findings were not
re-reviewed except to confirm REV-021/022/023 landed and REV-024 did not.

**Method note.** Every mutation and injection below was run in a **sandbox copy** of the tree
(`/tmp/gogo-rev05`, `cp -a`), never in the repo — this reviewer has no Edit tool by design and
touched no product file. Each run used `-count=1`. The sandbox was restored and re-verified
green after every experiment.

## Baseline (re-verified independently, not taken on trust)

| Gate | Result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | green, **12** tested packages (+1 with no test files) |
| hermetic `go test -count=1 ./...` (minimal `PATH`, no `claude`) | green |
| `gogo --version` / `plugin.json` | `gogo 0.29.0` / `"version": "0.29.0"` |

Matches the coordinator's baseline.

## 1. TEST-002 — the decision-gate move guard

**Placement is right.** `cli/internal/tui/move.go:93` sits before the class switch and outside
every `!force` condition, in FR4a's established shape. The bounce
(`decisionGateBounce`, move.go:146) names the decision **by ID**, points at `decisions.md`, names
a move, and is routed through `statusBlocked` (amber `⚠`, warn level) — blocked, not failed.
It renders in full at 60/80/100/120 columns (no truncation on the status line).

**`M` cannot override it — and the catch is stronger than the developer claimed.** Mutating the
check to `if f.WaitingForUser() && !force` in the sandbox:

- `TestDecisionGateIsNotLaunchable/M_(force)` FAILS ("M (force) on a paused card produced intent
  {Action:go … Command:/gogo:go demo} instead of a bounce"), and
- the parent test's keystroke loop ALSO fails, at three further assertions (`M` opened a launch
  confirm; refusal level 0 want warn; `View()` missing the warn glyph and the unblock).

The developer reported that only the `M_(force)` subtest catches it. That is **too modest**: the
guard is caught by two independent assertion sites. I re-ran the same mutation **hermetically**
(minimal `PATH`, so `hasClaude` is false and the confirm never opens) and it is still caught
twice — the subtest plus the `View()` assertion, whose status line then reads `⚠ claude CLI not
on PATH — cannot launch /gogo:go demo`. So the catch does not depend on the developer's machine
having `claude` installed.

Three further sampled mutations, all caught by the same test, restored green after each: removing
the check entirely; dropping the unblock from the message; dropping the decision-ID branch.

**Other paths to an `ActionGo`/`ActionAccept` intent — checked, all closed:**

| Path | Guard | Verdict |
|---|---|---|
| headless `gogo go` | `!orchestrator.RunnableStatus(f.Status)` (go.go:159) | refuses |
| reload auto-pickup | `f.Status != "plan-accepted"` (pickup.go, `autoPickupReady`) | refuses |
| plans-tab `m` (planGo) | spawns only `unspawnedTargets` (never touches an existing item) | n/a |
| board `d` (ship) on a paused card | `f.Class != ClassReadyToShip` — and `readyToShipStatus` (contract.go:473) excludes `waiting-for-user` | refuses |
| drill mode | has no launch key at all | n/a |
| `Session.intent()` (orchestrator) | only reachable through cmdGo, downstream of RunnableStatus | refuses |
| board `m` with a **selection** (merged ship) | **runs before every guard** | see **REV-027** |

**The gate is closed, not sealed — re-derived, not taken from the test.** The test's escape arm
only flips `Status` to `implementing` and asserts `ActionGo` returns, which is true by
construction. The question that matters is whether anything legitimately moves the status on:
`commands/resume.md` has `/gogo:resume` clear `open-decision` and re-enter at `state.md`'s
`resume:` phase, and the phase skill's §② step 1 then writes the working status. Critically, if
that entry write is skipped (n=3 in this very release), REV-022's scoping is **route-scoped, not
disk-scoped** — "write `phase`/`status` here only when the round loops back to ② or advances" —
so the resumed round's §④ exit write still moves the line. The gate therefore cannot strand an
item, with or without the LLM honouring the entry write. **No blocker here.**

**Interaction with REV-022 — the two agree, they do not fight.** §④ never overwrites a gate
status, so a card that enters the gate stays there; move.go refuses to launch while it does; a
round that CONTINUES writes the working status, which reopens the card. And the display half
holds: `TestDecisionGateKeepsItsDisplaySignals` pins `WaitingForInput`, the stripe, the `⏸`
count and the card cue, so a refused card still explains itself.

**Three gaps found in the same family** — all minor, all filed: **REV-025** (the board and the
headless surface give different unblocks for the same status, and the comment claims they
mirror), **REV-026** (the guard keys on one status literal, so `awaiting-uat` is still
launchable when the card does not classify ready-to-ship), **REV-027** (a stale ready-ship
selection still launches `/gogo:done` on a gated card, which the new bounce's own text says
never happens).

## 2. TEST-003 — the wrap-insensitive guard, and whether the control pair is sound

`skills_lint_test.go:159` normalises with `strings.Join(strings.Fields(body), " ")` before both
`Contains` checks. **The control pair is sound, and I reproduced both legs:**

| Leg | Result |
|---|---|
| hardened guard + forbidden phrase reintroduced WRAPPED across a line break | **1 failure**, the expected message |
| raw-matching guard + the same wrapped phrase | **0 failures** (PASS) |
| hardened guard, phrase absent (clean baseline) | green |

That triple is a complete detector score: **sensitivity** (fires on the injected defect),
**attribution** (the reverted leg is silent, so the failure is caused by the normalisation and
not by some unrelated assertion), **specificity** (green when the defect is absent). It is
strictly better here than `test-strategy.md`'s rule 7, which answers a different question
(does something ELSE bite) and would have scored this change unverifiable. Judgement: **valid,
and it belongs in `test-strategy.md`** — filed as **REV-029** (nit), including the heading and
the "ten known shapes" cross-reference in `code-review-standards.md` check #12 that move with it.

One observation, not filed: the sibling guard `TestPhaseSkillsWriteOccupancyAtEntryAndExit` was
NOT normalised, but all three of its checks are **positive** (`if !Contains → fail`), so a
markdown reflow makes it fail loudly rather than pass silently. Fail-closed is safe; only the
negative check had the silent-miss shape TEST-003 fixed. No finding.

## 3. Round-04 findings — verification

| Id | Verdict |
|---|---|
| REV-021 (major) | **verified** — NFR:31-37, project-knowledge:443-447 and code-review-standards check #13 all now describe entry AND exit |
| REV-022 (minor) | **verified** — the scoping sentence is in all three phase skills, pinned by a green guard, and agrees with TEST-002's check (see §1) |
| REV-023 (minor) | **verified** — the hand-off blink is explained in `skills/gogo-cli/SKILL.md:169-174` and `README.md:378-380` |
| REV-024 (minor) | **back to `open`** — the defect is still on disk by design. Confirmed at `report/report.md:436` AND `:439` (two rows, not one). ⑤'s to fix on re-reconcile; **recorded, not fixed here** |

## 4. Process spot-checks the coordinator asked for

- **`events.jsonl` is monotonic.** All 21 lines parsed and compared pairwise: no timestamp
  precedes its predecessor. The newest is `phase-started`/`review` at `19:55:53Z`, after
  `phase-done`/`implement` at `19:53:31Z`. The implement counts reconcile too: one initial
  `phase-started` + `fix-round` 1-7 + 8 × `phase-done` matches `iterations: implement=8`.
- **No verification in this round rests on a remembered anchor.** I re-derived the two claims
  that matter (the `!force` mutation and the TEST-003 control pair) from scratch against the
  file's actual bytes, plus three sampled mutations — 5 of the developer's reported 14 + the
  control pair. All reproduced exactly as reported. The one false pass they self-reported was
  caught by their own landed-edit check and is recorded in `adjustments.md`; treated as a
  warning, not a result, as instructed.
- **Write scope.** The round's code is pure read-side: the guard and `decisionGateBounce` do no
  I/O, the new tests use `t.TempDir()` and read-only fixtures, and nothing under `~/.gogo/`
  was touched (mtimes predate today). No new writes outside `.gogo/`.
- **Em dashes.** The round's new code, comments, messages and `adjustments.md` body use plain
  `-`. The only `—` added is the `## 2026-07-30 — …` section heading in `adjustments.md`,
  matching that file's six existing headings. Not filed.
- **Plan fidelity.** No FR was under- or over-built this round: the only new production symbol
  is `decisionGateBounce` plus the four-line guard, both traceable to TEST-002; TEST-003 is a
  test-only hardening. The as-built **record**, however, was not updated — see **REV-028**.

## Findings this round

| Id | Sev | Pri | Status | Finding | Fix |
|---|---|---|---|---|---|
| REV-025 | minor | P2 | new | `decisionGateBounce` (move.go:146-153) says "answer it in decisions.md, then run `/gogo:resume <slug>`"; `runnableHint` (go.go:447-448) says "resolve it and re-accept (→ plan-accepted) first" — two unblocks for one status, while the comment claims they mirror. For a mid-UAT re-plan (`open-decision: UAT round N`) the round lives in `uat.md` and only re-acceptance leaves the gate (skills/gogo/SKILL.md:262-293), so the message names the wrong artifact and an unsanctioned move. | **AGENT-FIXABLE** — one producer both surfaces quote, branch the wording on the gate flavour, pin the call sites, correct the comment |
| REV-026 | minor | P2 | new | The guard keys on the literal `waiting-for-user`, not on `orchestrator.RunnableStatus`. Verified: an `awaiting-uat` item with no `report/report.md` classifies `unfinished` through the real loader and `m`/`M` return `/gogo:go <slug>` with an EMPTY bounce (same for a stale `phase: test` + `awaiting-uat`). Lower reachability than TEST-002, hence minor. | **AGENT-FIXABLE** — refuse `!RunnableStatus(f.Status)` before an ActionGo intent, keeping the accept route and the ready-card ship route, and extend the test |
| REV-027 | minor | P2 | new | The selection branch (move.go:42-60) runs before every guard, and `m.selected` is never cleared on reload while the tick is class-filtered (view.go:613). Verified: select a ready card, move it to a UAT re-plan lock, and `m` returns `/gogo:done <slug>` with no bounce and no visible selection — contradicting the new bounce's "a paused card is never relaunched by m or M". | **AGENT-FIXABLE** — have `selectedFeatures()` return only still-ready cards (one predicate with the renderer), or bounce naming the card; add the regression test |
| REV-028 | minor | P2 | new | `plan.md` still carries the reverted exit-write claim in three places (`:21-24`, `:40`, `:805`, plus a quoted `:690`), including a Changes-checklist INSTRUCTION that would re-write the superseded rule into the knowledge base; and its fold-in bullet (`:32`) omits TEST-002/TEST-003. REV-021's sweep did not cover `.gogo/work/**`; REV-024 covers only the report — and ⑤ regenerates the report FROM the plan. | **AGENT-FIXABLE** — correct the three statements to "entry AND again at exit" (same wording as REV-024's), extend the fold-in bullet, re-grep `.gogo/work/**` |
| REV-029 | nit | P3 | new | The control-pair scoring technique (validated above) is not in `test-strategy.md`; rule 7 currently sends the next agent down a method that would score a guard-hardening unverifiable. Third distinct technique this feature has needed; the other two became rules 2 and 9. | **AGENT-FIXABLE** — add it as the eleventh entry, bump the "TEN variants" heading and the "ten known shapes" cross-reference in check #12 |
| REV-024 | minor | P3 | open | Carried, **not** fixed here by instruction: `report/report.md:436` and `:439` still state the superseded entry-only rule. | **AGENT-FIXABLE at ⑤'s re-reconcile** |

**Counts:** 0 open blockers · 0 open majors · 5 open/new minors · 1 open nit · 23 verified.

## Verdict — APPROVE

Both fixes do what they claim. TEST-002's guard is correctly placed, force-proof (proven by
mutation, twice over, including hermetically), consistent with REV-022's scoped exit write in
both directions, and leaves no `ActionGo`/`ActionAccept` path open. TEST-003's hardening is
real and its control-pair score is a valid method. Round 04's three fixes are verified.

Nothing found this round meets the blocker or major bar, so this does not loop back — but
REV-025/026/027 are all in TEST-002's own family (one status literal instead of the shared
rule; one branch that still runs ahead of the guard; two surfaces telling different stories),
and REV-028 must land before ⑤ re-reconciles or the corrected rule gets copied back out of a
stale `plan.md`. A cheap follow-up round would close all four.
