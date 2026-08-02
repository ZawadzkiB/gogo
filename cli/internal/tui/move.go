package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// launchDoneMsg carries the outcome of a launch back to the model, with its
// SEVERITY (FR3.2). The zero level is statusLevelOK, so every site that only sets
// `status` keeps today's dim voice byte-for-byte.
type launchDoneMsg struct {
	status string
	level  statusLevel
}

// attemptAction resolves the requested action into a launch intent or a bounce
// reason (the status-line hint). Pure — the unit-tested move-guard core.
//
//	ship=false (m): plan-pending → accept · other unfinished (plan-accepted) → go · in-progress → go(resume) · ready → done · shipped → bounce
//	ship=true  (d): ready(+selection) → done · everything else → bounce
func (m *Model) attemptAction(ship bool) (launch.Intent, bool, string) {
	return m.attemptActionForce(ship, false)
}

// attemptActionForce is attemptAction with the FR3.3 cap override: force=true
// skips the per-source concurrency-cap bounce (and only that guard - every other
// legality rule still applies) so `M` can start a second build in a busy source
// from inside the cockpit, instead of sending the user out to
// `gogo go <slug> --force`. The confirm still names the cap and the blocking
// slugs, so forcing stays a deliberate act.
func (m *Model) attemptActionForce(ship, force bool) (launch.Intent, bool, string) {
	sel := m.selectedFeatures()
	if len(sel) > 0 {
		// A ready selection ships as one (merged if >1) entry — but ONLY within a
		// single project. The merged /gogo:done launches in ONE repo (doLaunch anchors
		// it at the intent's Root), so a selection spanning >1 project would silently
		// run the other projects' slugs in the wrong repo. Cross-repo merged ship is
		// later-phase work (P4 correlation id); Phase 1 refuses it with a clear bounce
		// and launches nothing (REV-001). Computed from the actual selected features
		// (composite-keyed), never a slug re-lookup that collapses a same-slug pair.
		if selectionSpansProjects(sel) {
			return launch.Intent{}, false, "select ready cards from one project to ship together"
		}
		slugs := make([]string, len(sel))
		for i, f := range sel {
			slugs[i] = f.Slug
		}
		in := launch.BuildIntent(launch.ActionDone, slugs, "")
		in.Root = m.rootFor(sel[0]) // all share one root (the guard above)
		return in, true, ""
	}

	f := m.focusedCard()
	if f == nil {
		return launch.Intent{}, false, "no card focused"
	}

	if ship {
		if f.Class != contract.ClassReadyToShip {
			return launch.Intent{}, false, "only ready cards can ship (d) — " + f.Slug + " is " + f.Class
		}
		return m.intentFor(launch.ActionDone, f), true, ""
	}

	// TEST-002: a card paused at a genuine DECISION GATE is not launchable. Checked here,
	// before the class switch and OUTSIDE every `!force` condition, for exactly the reason
	// FR4a gives for the plan-readiness gate: `M` overrides the per-source cap "and only that
	// guard - every other legality rule still applies". A decision gate is a legality rule.
	//
	// Why this needs to be code. `waiting-for-user` is the status the orchestrator sets when it
	// stops and asks; the answer belongs in `decisions.md` and `/gogo:resume` folds it back in.
	// The board consulted WaitingForUser() for DISPLAY (the badge, the ⏸ count, the gate stripe)
	// but never for the MOVE, so `m` on a paused card opened a real `/gogo:go` confirmation and
	// the only thing standing between a keypress and a relaunch was a STOP instruction in the
	// spawned session's own prompt. That is prose enforcement - and the central evidence of this
	// very release is that prose enforcement fails (FR11's entry write: n=3, skipped every time).
	// A gate guarded only by an instruction to the thing being gated is not guarded.
	//
	// It pairs with the exit write's new scoping (REV-022): §④ never overwrites a gate status,
	// so a card that enters the gate STAYS there, and this keeps it un-launchable while it does.
	// Leaving the gate legitimately - `/gogo:resume` records the answer and moves the status on -
	// makes the card launchable again, so the gate is closed, not sealed.
	if f.WaitingForUser() {
		return launch.Intent{}, false, decisionGateBounce(f)
	}

	// FR4a MUST precede the accept route: it is the guard that stops `m`/`M` accepting a
	// plan that was never written. Round 06 hoisted the accept route above the class
	// switch and briefly jumped over this - the suite caught it immediately. Outside every
	// !force condition, so M cannot override it.
	if bounce := planReadinessBounce(f); bounce != "" {
		return launch.Intent{}, false, bounce
	}
	// A plan-pending card's legal `m` is ACCEPT, and it must be decided BEFORE the
	// runnable check because `awaiting-plan-acceptance` is deliberately not runnable.
	if f.Status == "awaiting-plan-acceptance" {
		return m.intentFor(launch.ActionAccept, f), false, ""
	}
	switch f.Class {
	case contract.ClassUnfinished:
		// A plan-pending card's legal `m` move is ACCEPT — route it to the launched
		// /gogo:accept (FR-C2), not the /gogo:go that bounces without plan-accepted
		// (the dead end this closes). Accept is uncapped (it doesn't drive build work).
		// Every other unfinished card (incl. an accepted plan awaiting its first build)
		// goes — behind the concurrency-cap guard. Branch on status, not class, because
		// both share ClassUnfinished.
		// REV-026: `/gogo:go` only runs a RUNNABLE status. This lives in BOTH go-producing
		// arms and NOT above the switch: ClassReadyToShip's legal status IS `awaiting-uat`
		// and its legal move is /gogo:done, so an above-switch check refuses a ship (the
		// suite caught exactly that). Round 05 guarded only ClassUnfinished - but
		// `classify()` derives ClassInProgress from the `phase:` line ALONE, so a card with
		// `phase: review` + `status: awaiting-uat` took the OTHER arm and returned ActionGo
		// with an EMPTY bounce while `gogo go` refused it. A lagging phase line is this
		// release's own subject matter, so that combination is reachable by construction.
		if !orchestrator.RunnableStatus(f.Status) {
			return launch.Intent{}, false, notRunnableBounce(f)
		}
		if bounce := m.capBounce(f); bounce != "" && !force {
			return launch.Intent{}, false, bounce
		}
		return m.intentFor(launch.ActionGo, f), false, ""
	case contract.ClassInProgress:
		// REV-026: `/gogo:go` only runs a RUNNABLE status. This lives in BOTH go-producing
		// arms and NOT above the switch: ClassReadyToShip's legal status IS `awaiting-uat`
		// and its legal move is /gogo:done, so an above-switch check refuses a ship (the
		// suite caught exactly that). Round 05 guarded only ClassUnfinished - but
		// `classify()` derives ClassInProgress from the `phase:` line ALONE, so a card with
		// `phase: review` + `status: awaiting-uat` took the OTHER arm and returned ActionGo
		// with an EMPTY bounce while `gogo go` refused it. A lagging phase line is this
		// release's own subject matter, so that combination is reachable by construction.
		if !orchestrator.RunnableStatus(f.Status) {
			return launch.Intent{}, false, notRunnableBounce(f)
		}
		if bounce := m.capBounce(f); bounce != "" && !force {
			return launch.Intent{}, false, bounce
		}
		return m.intentFor(launch.ActionGo, f), false, ""
	case contract.ClassReadyToShip:
		return m.intentFor(launch.ActionDone, f), true, ""
	case contract.ClassShipped:
		return launch.Intent{}, false, "already shipped — no move (illegal)"
	}
	return launch.Intent{}, false, "no legal move for " + f.Slug
}

// notRunnableBounce explains why a card whose status is not runnable cannot take `m`,
// naming the legal move instead of refusing bare (Diagnosability). It must not
// contradict cmdGo's runnableHint - see the REV-031 note inside.
func notRunnableBounce(f *contract.Feature) string {
	// REV-031: this and cmdGo answer the same question, so they must not contradict each
	// other. Round 06 aligned the wording to runnableHint's terminal arm - which is DEAD
	// CODE: cmdGo returns earlier for TerminalStatus (go.go:153) with its own sentence and
	// exit 0. Aligning to a string the CLI never prints, and calling an ABORTED feature
	// "already shipped", put a falsehood in front of the user. Both surfaces now key on
	// orchestrator.TerminalStatus and each says what is actually true of that status.
	switch f.Status {
	case "awaiting-uat":
		return f.Slug + " is at the UAT gate - run `/gogo:done " + f.Slug +
			"` to ship it, or give feedback to loop it back"
	case "aborted":
		return f.Slug + " was aborted - no move (illegal)"
	case "shipped", "done":
		return f.Slug + " is already shipped - no move (illegal)"
	case "":
		return f.Slug + " has no status on disk - run `gogo plan " + f.Slug +
			"` and accept a plan first"
	}
	return f.Slug + " is " + f.Status + " - not a runnable status; `/gogo:go` needs " +
		"plan-accepted, implementing, reviewing or testing"
}

// decisionGateBounce is the refusal for a card paused at a decision gate (TEST-002). Per the
// Diagnosability bar a refusal must carry its UNBLOCK, so it names the legal move
// (`/gogo:resume <slug>`) rather than only saying no - and it names the open decision's ID when
// `state.md` carries one, because "paused on D3" is actionable where "paused on a decision" is
// not. The wording deliberately mirrors cmdGo's runnableHint("waiting-for-user") so the board
// and the headless surface tell the user the same story.
//
// A mid-UAT re-plan parks on the same status with an `open-decision: UAT round N`, and the same
// answer applies: resolve it, then resume. Routed through statusBlocked by launchActionForce -
// nothing failed, the user is blocked.
// notRunnableBounce explains why a card whose status is not runnable cannot take `m`,
// naming the legal move. It mirrors cmdGo's runnableHint so board and CLI agree (REV-026).
func decisionGateBounce(f *contract.Feature) string {
	gate := "a decision"
	if d := strings.TrimSpace(f.OpenDecision); d != "" && !strings.EqualFold(d, "none") {
		gate = d
	}
	// REV-025: `waiting-for-user` covers TWO gates with different artifacts and different
	// exits. A plain decision is answered in decisions.md and folded back by /gogo:resume.
	// A UAT re-plan round lives in uat.md, and ONLY the user's re-acceptance
	// (-> plan-accepted) leaves it - /gogo:resume does not. Naming the wrong artifact sends
	// the user to a file that does not hold their question.
	//
	// Round 06 first branched on `strings.Contains(gate, "uat")`, which is wrong in BOTH
	// directions: an ordinary decision whose text merely mentions uat ("D4 - uat asked for a
	// different header") was sent to uat.md, and it disagreed with the card's own pill.
	// isUATReplan is the deterministic answer that already existed - it requires a DIGIT
	// after "uat" (the orchestrator writes "UAT round N"), and pillLabel/badge key on the
	// same predicate, so the bounce and the pill now agree by construction.
	where, how := "decisions.md", "run `/gogo:resume "+f.Slug+"`"
	if isUATReplan(f) {
		where, how = "uat.md", "re-accept the adjusted plan"
	}
	return f.Slug + " is paused on " + gate + " - answer it in " + where + ", then " + how +
		" to continue (a paused card is never relaunched by m or M)"
}

// planUnready is the PURE predicate half of planReadinessBounce: "is this card's plan
// unready", with no I/O and no message formatting. Render paths use it; keystroke paths that
// actually SHOW the sentence use planReadinessBounce.
//
// The split exists because footerChips runs on every View() - every keystroke, every fsnotify
// reload - and deciding a chip through planReadinessBounce meant opening and scanning plan.md
// once per frame just to test a boolean, then discarding the sentence it built. Worse, it made
// the rendered answer depend on disk state that can differ from the loaded model, which is the
// one-source-of-truth split this feature otherwise works to remove: f.PlanUnwritten is derived
// once per load by loadFeature, and that is the value the columns, the cap and the pill all
// use. TestPlanUnreadyAgreesWithTheBounce pins the two in step; TestFooterChipDoesNoDiskIO
// pins the render path to this one.
func planUnready(f *contract.Feature) bool {
	if f == nil {
		return false
	}
	return f.Authoring() || (f.Status == "plan-accepted" && f.PlanUnwritten)
}

// planReadinessBounce returns the status-line refusal when f's plan.md is not written,
// else "". TWO distinct gates, and the message says WHICH one refused (a refusal the user
// cannot explain is a bug even when the code is right):
//
//   - AUTHORING (FR4) - the status reads the plan-acceptance gate (or nothing) while the
//     plan does not exist, so there is nothing to accept. This is the card the board used
//     to offer for acceptance, launching /gogo:accept at a folder with no plan in it.
//   - ACCEPTED-BUT-UNWRITTEN (FR8) - the status says `plan-accepted` while the plan does
//     not exist, so there is nothing to build. This ADDS a requirement to the
//     unaccepted-plan invariant and removes none.
//
// Both name their unblock (`gogo plan <slug>`) and both name their NUMBER when they have
// one (`plan.md has 1 of the 2 sections a written plan needs`) - the exact reason, quoted
// from the one contract-side producer so this can never drift from `gogo go`'s refusal.
// Routed through statusBlocked (amber `⚠ `) by launchActionForce, never statusFailed:
// nothing failed, the user is blocked.
func planReadinessBounce(f *contract.Feature) string {
	if f == nil {
		return ""
	}
	switch {
	case f.Authoring():
		return "plan.md not written yet - " + f.Slug + " is still being authored (" +
			contract.PlanUnwrittenReason(f.Dir) + "); finish it with `gogo plan " + f.Slug + "`"
	case f.Status == "plan-accepted" && f.PlanUnwritten:
		return f.Slug + " is plan-accepted but its plan.md is not written (" +
			contract.PlanUnwrittenReason(f.Dir) + ") - nothing to build; re-plan it with `gogo plan " + f.Slug + "`"
	}
	return ""
}

// intentFor builds a single-card launch intent for f, stamping its OWN repo root (via
// rootFor) so the launch anchors at the FOCUSED card's source — never a slug re-lookup
// that could grab a same-slug card from another project on the unified board (REV-001).
// FR4: a go-launch also carries the SOURCE's gate-skip params (--skip-acceptance /
// --skip-uat) and its fast-mode param (--fast), resolved by the card's own root
// through the same shared resolvers the cap guard uses (so the board and `gogo go`
// never drift); they are visible in the launch confirmation. Absent flags → the
// command is byte-for-byte today's.
func (m *Model) intentFor(action launch.Action, f *contract.Feature) launch.Intent {
	in := launch.BuildIntent(action, []string{f.Slug}, "")
	in.Root = m.rootFor(f)
	if action == launch.ActionGo {
		planSkip, uatSkip := projects.SkipForSource(m.capWatchSources(), in.Root)
		in.Command += launch.SkipParams(planSkip, uatSkip)
		in.Command += launch.FastParam(projects.FastForSource(m.capWatchSources(), in.Root))
	}
	return in
}

// selectionSpansProjects reports whether the selected ready features resolve to more
// than one distinct repo root. A merged ship builds a SINGLE /gogo:done anchored at one
// root, so a selection crossing project roots would mis-root every other project's slug.
// Phase 1 refuses such a ship (REV-001); a per-root fan-out is later-phase work. It reads
// the ACTUAL selected features (composite-keyed by the caller), never a slug re-lookup —
// on the unified board a slug re-lookup collapses a same-slug cross-project pair into one
// and would let the guard silently pass. Single-repo mode never trips this (every feature
// shares one root), so its ship path is byte-for-byte unchanged.
func selectionSpansProjects(feats []*contract.Feature) bool {
	root, seen := "", false
	for _, f := range feats {
		if f == nil {
			continue
		}
		if !seen {
			root, seen = f.Root, true
			continue
		}
		if f.Root != root {
			return true
		}
	}
	return false
}

// capBounce returns a status-line bounce when launching a `go` on f would exceed
// its source's concurrency cap (FR4/FR5) — the board analog of cmdGo's capBlock.
// BOTH launch paths enforce the SAME orchestrator.CapExceeded rule (over the one
// shared pure helper, CapForSource) so they never drift: two live build sessions
// clobber a repo's shared working tree. It resolves the cap from EVERY project's
// SOURCES on the unified board (else the focused project's — capWatchSources, FR5), by
// the target feature's OWN root (rootFor), and counts that
// root's features with a live BUILD session EXCLUDING f itself (so resuming an
// already-active feature is never blocked; since 0.29.0 the file-derived class is no
// longer part of the count - see ActiveWorkSlugs). Empty when uncapped / unregistered - the
// byte-for-byte single-repo fallback (a lone repo has no sources, so CapForSource
// returns 0). Read-side only; it writes nothing and composes with the one-owner
// lock. Over the cap it drops the user to `gogo go --force` (the CLI escape hatch),
// matching the selectionSpansProjects bounce style.
func (m *Model) capBounce(f *contract.Feature) string {
	if f == nil {
		return ""
	}
	root := m.rootFor(f)
	// FR5: resolve the cap from EVERY project's sources on the unified board — a card's
	// source may live in a non-focused project — else the focused project's (byte-for-byte).
	cap := orchestrator.CapForSource(m.capWatchSources(), root)
	if cap <= 0 {
		return ""
	}
	active := orchestrator.ActiveWorkSlugs(m.repo, root, m.sessions, f.Slug)
	if !orchestrator.CapExceeded(cap, len(active)) {
		return ""
	}
	// FR3.4: say plainly what the cap counts - the scope was invisible, so a bounce
	// read as "gogo won't let me work" rather than "this ONE repo is busy".
	// 0.29.0 (FR12a): the RULE changed, so the sentence had to change with it - and it now
	// comes from orchestrator.CapRuleClause, the single source every surface quotes, because
	// the hand-copied version left three of four surfaces stating the old rule.
	// The unblock list names a sweep too (REV-008): since the cap counts a live build session
	// regardless of the feature's status, a blocker can be an already-SHIPPED item whose
	// session outlived a failed ship-reap - and for that one "ship one" is useless advice.
	// It must be the TARGETED `gogo sweep <slug>` (REV-011): a bare `gogo sweep` is
	// host-global and judges every `gogo-*` session on the machine against THIS repo's
	// features, so on a multi-source host it kills another source's in-flight build as an
	// "orphan", without confirmation. This bounce appears in the multi-source cockpit BY
	// CONSTRUCTION (the cap is per-source on the unified board), and it already has the
	// blocking slugs - so naming them costs nothing. One producer (CapSweepRemedy) shared
	// with cmdGo's capBlock, so the two refusals cannot drift.
	return fmt.Sprintf("cap %d reached in %s - already building %s (the cap %s); %s",
		cap, filepath.Base(root), strings.Join(active, ", "), orchestrator.CapRuleClause,
		orchestrator.CapRefusal(
			"press M to force",
			"ship one",
			orchestrator.CapSweepRemedy(active),
			"or run `gogo go "+f.Slug+" --force`",
		))
}

// launchAction runs the guard, then either bounces or opens the huh
// confirmation. NEVER launches without the confirmation.
func (m Model) launchAction(ship bool) (tea.Model, tea.Cmd) {
	return m.launchActionForce(ship, false)
}

// launchForce is the FR3.3 in-TUI cap override (`M`): the same guarded launch as
// `m`, minus the per-source cap bounce, still behind the huh confirmation - which
// now names the cap it is overriding and the slugs already building.
func (m Model) launchForce() (tea.Model, tea.Cmd) {
	return m.launchActionForce(false, true)
}

// launchActionForce is the shared body of `m` / `M`.
func (m Model) launchActionForce(ship, force bool) (tea.Model, tea.Cmd) {
	intent, isShip, bounce := m.attemptActionForce(ship, force)
	if bounce != "" {
		m.statusBlocked(bounce) // FR3.2: a refused move is BLOCKED, not failed
		return m, nil
	}
	if !m.hasClaude {
		m.statusBlocked("claude CLI not on PATH — cannot launch " + intent.Command)
		return m, nil
	}
	// Only claim to be forcing when a cap is ACTUALLY being overridden (REV-007/REV-010).
	// Ask the GUARD what the force overrode rather than enumerating the arms that do
	// not consult the cap: attemptActionForce answers the selection branch (a merged
	// /gogo:done) and the plan-acceptance branch (an uncapped /gogo:accept) before any
	// capBounce is reached, so an arm list is a thing to keep in sync and was already
	// wrong twice. Re-running the guard UNFORCED yields exactly the bounce the force
	// suppressed - "" for every arm that never consulted the cap - which is correct by
	// construction for all of them. It is a pure read, so the extra call costs nothing.
	// A confirm is the safety surface for a state-changing launch, so wrong text there
	// is worse than cosmetic.
	override := ""
	if force {
		if _, _, unforced := m.attemptActionForce(ship, false); unforced != "" {
			override = unforced
		}
	}
	return m, m.startFormOverriding(intent, isShip, override)
}

// startFormOverriding builds the launch-site confirmation (a release-name input
// first, for a merged ship of >=2) plus the FR3.3 override note: when `M` is
// skipping a cap bounce, the confirm names the cap and the blocking slugs
// (compressed by forcingNote) so the user sees exactly what they are
// overriding. An empty note renders the unforced confirm byte-for-byte.
//
// A `go` intent gets the THREE-OPTION SELECT (D1=B) — Launch / Launch --fast /
// Cancel — pre-highlighted on whichever mode the source's fastMode config
// implies (the seed reads the command intentFor already resolved, so config and
// seed agree by construction, FR1), with the EXACT command shown in the TITLE
// and re-evaluated live as the selection moves (REV-001: labels stay one line).
// Ship/accept intents keep the Launch/Cancel Confirm byte-for-byte (FR6: no
// fast option where it does not apply).
//
// It returns the command to hand back to Update: the form's Init batched with
// the modal layout (enterLaunchModal) — huh's protocol is async, so a caller
// must never discard it (TEST-001).
func (m *Model) startFormOverriding(intent launch.Intent, isShip bool, override string) tea.Cmd {
	m.pending = intent
	m.pendingShip = isShip
	// A fresh, heap-stable binding for this form's fields (see formBinding).
	//
	// CONFIRM-DEFAULT CONVENTION (the canonical statement; TEST-001). Every gogo
	// confirm seeds binding.confirm EXPLICITLY, and which value it seeds is a
	// deliberate safety rule, not a style choice:
	//
	//   - a FORWARD pipeline move (launch / spawn / accept - the `m` family, here and
	//     startPlanSpawnForm / startPlanDoneForm on the plans tab) seeds `true`, so the
	//     affirmative starts highlighted and a bare **Enter submits** the confirmation
	//     the user deliberately opened. Esc/Ctrl+C, or toggling to Cancel (n), aborts.
	//     The go Select is this same rule in its D1=B shape: the seeded option IS a
	//     launch, so a bare Enter still launches exactly the command shown.
	//   - a DESTRUCTIVE or irreversible action (startDeleteForm in delete.go,
	//     startKillForm in update.go) seeds `false`, so **Enter is safe** and the user
	//     must arrow over to pick Delete/Kill on purpose.
	//
	// The asymmetry IS the rule. Never "align" the two families for consistency: an
	// unseeded binding on a plans-tab spawn made the same keystroke that launches on the
	// board silently cancel there (TEST-001), and seeding `true` on a delete would make
	// a stray Enter destructive.
	m.binding = &formBinding{confirm: true, launchSite: true}

	var fields []huh.Field
	merged := isShip && len(intent.Slugs) >= 2
	if merged {
		m.binding.release = suggestRelease(intent.Slugs)
		fields = append(fields, huh.NewInput().
			Title("Release name for the merged entry").
			Description(strings.Join(intent.Slugs, " + ")).
			Value(&m.binding.release))
	}
	if intent.Action == launch.ActionGo {
		// Every option label is ONE line (REV-001): huh sizes the group one row per
		// option pre-wrap, then clamps the Select's viewport to those rows — so a
		// wrapped label pushes the TAIL option (Cancel) out of view at realistic
		// slugs/widths. The EXACT command lives in the TITLE instead, re-evaluated
		// live via TitleFunc as the selection moves (Select commits the bound value
		// on every cursor move; probe P3). Title and doLaunch build the command
		// through the SAME producer (launch.SetFastParam), so what launches is what
		// was shown (FR4) by construction. Options BEFORE Value: huh pre-selects by
		// scanning the options already set when Value binds. The title closure
		// captures the heap-stable binding, never the value-copied Model (TEST-001).
		m.binding.launchMode = goLaunchFull
		if launch.HasFastParam(intent.Command) {
			m.binding.launchMode = goLaunchFast
		}
		b, cmd, tail := m.binding, intent.Command, m.confirmWhere(intent)
		title := func() string {
			return "will run: claude \"" + launch.SetFastParam(cmd, b.launchMode == goLaunchFast) + "\"  " + tail
		}
		sel := huh.NewSelect[string]().
			Title(title()). // Title first, so title.val is never empty (no flicker)
			TitleFunc(title, &b.launchMode).
			Options(
				huh.NewOption("Launch", goLaunchFull),
				huh.NewOption("Launch --fast  (token-lean gogo-fast)", goLaunchFast),
				huh.NewOption("Cancel", goLaunchCancel),
			).
			Value(&b.launchMode)
		if override != "" {
			sel = sel.Description("FORCING past the source cap - " + forcingNote(override))
		}
		fields = append(fields, sel)
	} else {
		confirm := huh.NewConfirm().
			Title(m.confirmSummary(intent)).
			Affirmative("Launch").
			Negative("Cancel").
			Value(&m.binding.confirm)
		if override != "" {
			confirm = confirm.Description("FORCING past the source cap - " + override)
		}
		fields = append(fields, confirm)
	}

	return m.enterLaunchModal(newForm(huh.NewGroup(fields...)))
}

// enterLaunchModal is the launch confirm's form-entry step (FR9/FR10): it
// records the ORIGIN view the modal composites over and every cancel/finish
// returns to (recorded, never inferred — the formOrigin rule, D4=A), switches to
// form mode, and lays the form out at the MODAL's inner size by feeding it a
// WindowSizeMsg (P2 — the one path huh recomputes width AND height together;
// Form.WithWidth is measured-broken, P1). On a too-small or unsized terminal
// (modalFormSize ok == false) no size is fed, so the form lays out exactly as
// every full-screen form does today — the same one rule View()'s FR12 render
// fallback uses.
func (m *Model) enterLaunchModal(f *huh.Form) tea.Cmd {
	m.formOrigin = m.mode
	m.form = f
	m.mode = modeForm
	cmds := []tea.Cmd{f.Init()}
	if w, h, ok := modalFormSize(m.width, m.height); ok {
		fm, cmd := m.form.Update(tea.WindowSizeMsg{Width: w, Height: h})
		if ff, ok2 := fm.(*huh.Form); ok2 {
			m.form = ff
		}
		// huh's protocol is async — dropping a form's own command is the TEST-001
		// failure this codebase already paid for once. Batch, never discard.
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

// forcingNote compresses the full cap bounce into the confirm's FORCING line
// (REV-001, round 2): the bounce's remedy tail ("press M to force, ship one, …")
// advises a gate the user has ALREADY passed by pressing M, and the rule clause
// restates CapRuleClause — inside the confirm they only cost the rows the three
// Select options need at small terminals (at 60x12 the full note left ZERO rows
// for the options). What must survive is what is being OVERRIDDEN: the cap and
// the blocking slugs (the 0.28.0 value, pinned by the force-confirm tests). The
// full sentence still appears where it originated — the `m` bounce on the
// status line.
func forcingNote(override string) string {
	if i := strings.Index(override, " (the cap "); i >= 0 {
		return override[:i]
	}
	if i := strings.Index(override, "; press M to force"); i >= 0 {
		return override[:i]
	}
	return override
}

// modalLaunchConfirm reports whether the ACTIVE form is the board launch confirm
// (the startFormOverriding site) — the one form rendered as a centered modal
// over its origin view (D2=B; the other form sites keep the full-screen
// takeover for now). The marker lives on the heap-stable binding, so it dies
// with the form on every close path.
func (m Model) modalLaunchConfirm() bool {
	return m.mode == modeForm && m.form != nil && m.binding != nil && m.binding.launchSite
}

func (m *Model) confirmSummary(intent launch.Intent) string {
	return "will run: claude \"" + intent.Command + "\"  " + m.confirmWhere(intent)
}

// confirmWhere is the shared context tail of the launch confirms: where the
// command runs, at which repo root, under which permission mode.
func (m *Model) confirmWhere(intent launch.Intent) string {
	where := "tmux session " + intent.Session
	if !m.hasTmux {
		where = "background (claude -p + log)"
	}
	// Name the target REPO (intent.Root) so a mis-anchored launch is catchable at the
	// confirm — on the unified board a same-slug card in another project must never be
	// launched into the wrong repo unnoticed (REV-001). Empty root falls back silently
	// (single-repo mode, where the CLI roots the launch itself).
	at := ""
	if intent.Root != "" {
		at = "  at " + intent.Root
	}
	// FR8: state the effective permission mode the launch runs under.
	return "in " + where + at + "  · " + launch.PermissionSummary()
}

// doLaunch rebuilds the intent with the (possibly edited) release name and
// spawns Claude via the injected launcher. Returns a command that reports the
// outcome. The caller (updateForm) clears the consumed selection/pending state
// on the model it returns — this closure only captures the resolved intent.
func (m Model) doLaunch() tea.Cmd {
	intent := m.pending
	// The launch anchors at the intent's OWN Root — captured from the FOCUSED / selected
	// card when the intent was built (attemptAction). NEVER re-resolve the root by a slug
	// re-lookup: on the unified board a slug is unique only per-source, so the first
	// match could be a same-slug card in the WRONG project and silently launch there
	// (REV-001). In single-repo mode this Root == m.root, so the resolution is unchanged.
	root := intent.Root
	if m.pendingShip && len(intent.Slugs) >= 2 && m.binding != nil {
		rebuilt := launch.BuildIntent(launch.ActionDone, intent.Slugs, m.binding.release)
		rebuilt.Root = root // preserve the captured root across the release-name rebuild
		intent = rebuilt
	}
	if m.binding != nil && m.binding.launchMode != "" {
		// D1=B/FR4: apply the chosen launch mode through the SAME producer the option
		// labels were built from (SetFastParam), so what launched is what was shown by
		// construction. Command only — Session/Root/Slugs are untouched (FR4), and the
		// toggle is per-launch only: the source's config.json is never written (FR5).
		intent.Command = launch.SetFastParam(intent.Command, m.binding.launchMode == goLaunchFast)
	}
	if root == "" {
		// No target root captured (the feature vanished between confirm and launch, or a
		// bare intent). Never launch relative to the process cwd — bounce, launching
		// nothing (REV-004).
		return func() tea.Msg {
			return launchDoneMsg{status: "feature no longer present, nothing launched", level: statusLevelWarn}
		}
	}
	launcher := m.launcher
	return func() tea.Msg {
		res, err := launcher(root, intent)
		if err != nil {
			// FR1.1/FR3.2: err is now a *launch.TmuxError (or a *CommandTooLongError),
			// so this line carries tmux's OWN words - `command too long`, a duplicate
			// session name - instead of a bare `exit status 1`, and it renders RED.
			return launchDoneMsg{status: "launch failed: " + err.Error(), level: statusLevelErr}
		}
		if res.Mode == "tmux" {
			return launchDoneMsg{status: "launched " + res.Command + " → tmux " + res.Session + " (press a to attach)"}
		}
		return launchDoneMsg{status: "launched " + res.Command + " → background, log " + res.LogPath}
	}
}

// suggestRelease proposes a merged-entry name from a common theme across the
// slugs (the longest shared leading word run), else a generic fallback.
func suggestRelease(slugs []string) string {
	if len(slugs) == 0 {
		return "release"
	}
	if len(slugs) == 1 {
		return slugs[0]
	}
	parts := make([][]string, len(slugs))
	for i, s := range slugs {
		parts[i] = strings.Split(s, "-")
	}
	var common []string
	for i := 0; ; i++ {
		if i >= len(parts[0]) {
			break
		}
		word := parts[0][i]
		same := true
		for _, p := range parts[1:] {
			if i >= len(p) || p[i] != word {
				same = false
				break
			}
		}
		if !same {
			break
		}
		common = append(common, word)
	}
	if len(common) > 0 {
		return strings.Join(common, "-")
	}
	sorted := append([]string(nil), slugs...)
	sort.Strings(sorted)
	return sorted[0] + "-plus-" + strconv.Itoa(len(slugs)-1)
}
