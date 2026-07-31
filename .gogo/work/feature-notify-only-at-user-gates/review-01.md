# Review — round 01 · `notify-only-at-user-gates`

- **Date:** 2026-07-31 · **Track:** review (phase ③) · **Round:** 1
- **Contract:** `review/issues.json` (this file is its rendered snapshot)
- **Scope:** working-tree diff on `main` — `hooks/notify.sh` (rewritten, +418/-39),
  `hooks/hooks.json`, `cli/notify_hook_test.go` (new), `cli/main.go`,
  `cli/version_test.go`, `README.md`, `docs/flow.md`, `.claude-plugin/plugin.json`
- **Reviewed against:** `plan.md` (FR1–FR8, D1/D2/D3), `code-review-standards.md`,
  `coding-rules.md`, `non-functional-requirements.md`, `charts/` (as-built)

## Verified green

| Gate | Result |
|---|---|
| `bash hooks/notify.sh --selftest` | `selftest: 33/33 passed`, exit 0 (bash 5.3 and /bin/bash 3.2) |
| `gofmt -l .` · `go vet ./...` | clean |
| `go test . -run 'TestNotifyHook\|TestVersionMirrorsPlugin'` | 3/3 PASS |
| Version bump (standards #2) | `plugin.json` + `cli/main.go` + `version_test.go` all 0.31.0 (D3) |
| Portability (NFR) | `jq` optional with a working `grep`/`sed` fallback (pinned directly); `curl`/`osascript`/`/dev/tty` each guarded; no bash-4 construct — `${var//…}`, `$'\t'`, `local` are all 3.2-safe |
| Write scope | reads only; writes nothing outside the selftest's own `mktemp -d` (guarded, scoped `find -delete`, no glob-`rm`) |
| Secrets | `GOGO_NTFY_TOPIC` never echoed; the FR6 trace prints type/level/class/verdict/gates/channels only |
| FR5 parser vs the Go authority | `gogo_state_status` agrees with `contract.parseStateFile` on all 30 real `state.md` files **and** on `templates/state.template.md` |
| Classifier ordering | the `*permission*` hedge sits **before** the silent list, and no silent type contains `permission` (`push_notification` is the near-miss) — verified, no silent type is swallowed and no silent type is falsely promoted |
| Selftest is not a copy | it drives the production `gogo_notify_decide`; mutations to `GOGO_GATE_STATUSES` (drop `awaiting-uat` → 30/33) and to the authoring carve-out (→ 31/33) both bite |

## Findings

### REV-001 · **major** · P1 · new · **NEEDS-USER-DECISION**
**Repo-wide gate scan re-enables the per-hand-off spam whenever ANY other work item is parked at a gate.**

`gogo_notify_gates()` (`hooks/notify.sh:125-145`) scans every `.gogo/work/*/state.md`,
so the gate-conditional branch fires for an open gate unrelated to the running
pipeline. Verified live in this repo — `feature-interactive-plan-review` sits at
`awaiting-plan-acceptance` with a written `plan.md` (8 `## ` sections), and:

```
$ printf '{"notification_type":"agent_completed","message":"gogo-reviewer finished"}' \
  | GOGO_NOTIFY_DEBUG=1 CLAUDE_PROJECT_DIR="$PWD" bash hooks/notify.sh
gogo-notify: type=agent_completed level=gates class=gate verdict=notify gates=1 channels=banner
```

So a full `/gogo:go` on *this* feature today emits one ping per subagent hand-off —
the exact reported symptom, and a direct falsification of the plan's acceptance
signal ("exactly one notification"). A tree with a parked gate is the normal state
of an active gogo repo. The plan foresaw it: A4 (dedupe) was parked with *"keep it
in the back pocket only if spam survives the classifier."* It survived.

**Fix (decision):** (a) scope the scan to the running session's slug — *recommended*,
keeps the acceptance signal literally true and adds no mutable state; (b) ping only
on a gate-set *change* (needs state under `.gogo/`, which the plan put out of scope);
(c) A4 dedupe/rate-limit; (d) accept and correct the acceptance signal.

### REV-002 · **major** · P1 · new · AGENT-FIXABLE
**osascript escaping is quote-only, so a backslash in the message injects AppleScript and executes arbitrary shell.**

`hooks/notify.sh:209` interpolates with `${msg//\"/\\\"}` — quotes escaped,
backslashes not. A message containing `\"` renders as `\\"`, which AppleScript reads
as *escaped backslash + closing delimiter*: the literal ends early, the rest is
executed as code, and a trailing `--` comments out the harness's own
` with title "…"`. Verified with a harmless probe using that exact escaping:

```
A: msg = x\" & (do shell script (ASCII character 105 & ASCII character 100)) --   -> rc=0   (ran `id`)
B: same with chars 122,122,122                                                    -> rc=1   err=[30:111: execution error: sh: zzz: command not found (127)]
```

B's error proves the shell command genuinely *ran*. No quote character is needed in
the payload to reach `do shell script`. The same hole exists on `$title` via the cwd
basename. Pre-existing shape, but the file is rewritten here and the plan
deliberately widens what reaches `msg`.

**Fix:** escape `\` before `"` for both fields, or drop interpolation entirely and
pass via argv (`osascript - "$msg" "$title" <<'AS' … on run argv … AS`) — no escaping
surface at all. Pin with a selftest case on the escaped string.

### REV-003 · **major** · P1 · new · AGENT-FIXABLE
**The FR8 anti-drift guard is blind in its primary direction.**

`cli/notify_hook_test.go:41-95` derives `want` by running the status enum through
`contract.WaitingForInput()` — but reads that **enum from
`templates/state.template.md`**, not from the contract. Verified by mutation: adding
`"awaiting-security-signoff"` to the `case "waiting-for-user", "awaiting-uat":` arm
of `contract.go:174`, leaving the hook and the template untouched, keeps
`TestNotifyHookGateStatusesMatchContract` reporting `ok`. `want` silently shrinks to
whatever the template lists, so the comparison is against a hand-maintained list —
undercutting the plan's stated justification ("provably identical rather than merely
intended to be") and the test's own docstring ("never hand-copied"). No other test in
`cli/` pins the template legend against the code.

**Fix:** source-scan `WaitingForInput`'s body for quoted status literals and fail if
any is missing from the parsed enum, with the usual "never pass vacuously" guard
(the `TestNewFormIsTheOnlyFormConstructionSite` precedent). Re-run the mutation.

### REV-004 · **major** · P1 · new · AGENT-FIXABLE
**Both FR5 parse-trap selftest cases pass for a weaker reason than claimed.**

`code-review-standards.md` #11(c) verbatim. The production code is **correct** in both
cases (probes confirm); the *fixtures* are vacuous. Verified by mutation on a copy:

| Mutation | Selftest |
|---|---|
| `split(line, a, …); print a[1]` → `print line` (drop the first-token split, `notify.sh:103-104`) | still **33/33**, exit 0 |
| delete the whole `incomment { … next }` rule (`notify.sh:91-94`) | still **33/33**, exit 0 |

Why: (1) the `implementing <!-- legend -->` fixture is defeated by the inline-comment
*splice*, not the split — no fixture carries a trailing token after a status value;
(2) `feature-b`'s commented example is **indented** (`'  - **status:** …'`,
`notify.sh:274`) and the awk pattern is column-anchored, so the anchor rejects it
whether or not comment tracking exists. The real TEST-001 shape is at column 0.

**Fix:** add a `- **status:** awaiting-uat (uat round 2)` gate fixture; move
`feature-b`'s commented example to column 0. Then re-run both mutations and confirm
each drops below 33 and exits 1.

### REV-005 · minor · P2 · new · AGENT-FIXABLE
**`gogo_state_status` is stricter than the parser it mirrors — an indented status line becomes a SILENT gate.**

`contract.stateLine` (`state.go:15`) is `^\s*-\s*\*\*([^:*]+):\*\*\s*(.*)$`; the awk
(`notify.sh:101-102`) demands a literal `^- **status:**`. Verified: both
`  - **status:** awaiting-uat` and `-  **status:** awaiting-uat` yield an **empty**
status, so the folder is skipped — while every CLI reader shows a genuine open gate.
A gate the user is never pinged about is precisely what D1 was written to avoid, on a
file written by an LLM following prose.

**Fix:** relax the awk match + `sub()` to `^[[:space:]]*-[[:space:]]*\*\*status:\*\*`.
Land together with REV-004(2), which makes the comment tracking load-bearing.

### REV-006 · minor · P2 · new · AGENT-FIXABLE
**`curl -d "$msg"` treats a leading `@` as a filename.**

`notify.sh:202`. Verified against a local listener: `-d "@/tmp/gogo-curl-probe.txt"`
POSTed `Content-Length: 20` / `SECRET-FILE-CONTENTS` — the file, not the string. A
message starting with `@` sends the wrong body, and if the text names a readable path
it ships that file's contents to the ntfy.sh topic.

**Fix:** `--data-raw "$msg"` (curl ≥ 7.43; macOS ships 8.7.1 — verified), or
`--data-binary @- <<<"$msg"` for maximum portability. Keep the `|| true`.

### REV-007 · minor · P2 · new · AGENT-FIXABLE
**Nothing tests the hook end to end.**

`--selftest` correctly drives the *production* `gogo_notify_decide` — but nothing
exercises `gogo_notify_main` (`notify.sh:221-241`): the stdin read, the subtle
`IFS=$'\t' read … <<EOF $(gogo_notify_decide …) EOF` record parse, the
verdict→send dispatch, the channel roll-up, or the exit-0 contract. Break that record
parse and `verdict` reads empty, the hook goes permanently silent, and both Go tests
plus all 33 selftest cases stay green (standards #11(b)). My hand-run above is its
only coverage.

**Fix:** add a `GOGO_NOTIFY_DRYRUN=1` seam in `gogo_notify_send` (print channels,
invoke nothing), then two selftest cases that pipe a payload through `bash "$0"` and
assert the debug trace (`verdict=notify` + non-`none` channels; `verdict=silent`) and
exit 0. Still sends nothing.

### REV-008 · minor · P3 · new · AGENT-FIXABLE
**`skills/gogo/SKILL.md:405` still carries the un-updated twin of the rewritten `docs/flow.md:208` sentence.**

Identical sentence, identical decision-gate step, untouched. It stays *true* there (a
decision gate writes `waiting-for-user`, so the scan fires) — hence minor, not a
stale-behaviour major — but standards #1 requires every surface in sync, and the
plan's own enumeration-sync grep (`plan.md:318-320`) omitted `skills/`, which is why
it was missed. A full re-grep found exactly this one uncovered hit;
`docs/architecture.md:137`, `project-knowledge.md:43` and `testing-tools.md:25` are
correctly deferred to the phase-⑤ reconcile.

**Fix:** mirror the flow.md clause; add `skills/` to the plan's grep list.

### REV-009 · nit · P3 · new · AGENT-FIXABLE
**The gate scan globs every `.gogo/work/*/` dir; `contract.LoadRepo` only counts `feature-*`.**

Verified: a `.gogo/work/scratchpad/state.md` reading `awaiting-uat` yields
`scratchpad · awaiting-uat`, a gate no CLI reader will ever show. Harmless today.
**Fix:** `for dir in "$root"/.gogo/work/feature-*/` — the existing `[ -d ]` guard
already absorbs the no-match literal.

## Plan fidelity

- **FR1–FR3, FR6, FR7** — implemented as specified; nothing unplanned crept in.
- **FR4/FR5** — implemented, but the gate predicate's *blast radius* (REV-001) makes
  the plan's acceptance signal unreachable in a repo with any parked gate, and the
  bash mirror is narrower than the authority in two spots (REV-005, REV-009).
- **FR8** — the guard exists and is wired, but is weaker than the plan claims in
  three independently-verified ways (REV-003, REV-004).
- **D1/D2/D3** — honoured (unknown types → gate-conditional; `idle_prompt` always
  notifies; 0.31.0 across all three files).
- **Charts** — `charts/flow.mmd` and `charts/sequence.mmd` match the as-built code;
  `manifest.json` is well-formed and its no-class/no-use-case rationale is sound.

## Verdict

**CHANGES** — 4 open majors (REV-001 needs a user decision; REV-002/003/004 are
agent-fixable), 4 minors, 1 nit.
