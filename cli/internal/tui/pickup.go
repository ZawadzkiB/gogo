package tui

import (
	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
)

// --- reload-driven auto-pickup (plans-board FR6, D4=B) ---------------------------------
//
// When a plan `go` spawns a work item into a source the user opted OUT of the plan gate
// (Source.PlanAcceptanceSkip), that item should move plan→implementation WITHOUT a manual
// keypress. The pickup is ALWAYS a launched `claude -p /gogo:go <slug>` (an attachable
// session, byte-for-byte a manual go — the CLI never flips a source's pipeline state).
//
// It fires from the cockpit's fsnotify reload (the reloadMsg handler): a spawn writes the
// source's .gogo/work/, the watch fires a reload, and this reconcile launches every
// eligible item into a FREE slot. Guards: skip-source + plan-accepted + no live session +
// under the source's INTEGER cap + fire-once. A cap-blocked (repo-busy) item is NOT fired
// and NOT recorded — it shows a "needs manual trigger" cue instead, and auto-fires on a
// later reload once a slot frees (the queue drains itself). A genuine mid-pipeline gate
// still parks the launched session at waiting-for-user (auto-pickup only skips the initial
// keypress, never a real fork).

// autoPickupReady reports whether f is a plan member spawned into a skip-configured source
// that is sitting at `plan-accepted` with no live session — the auto-pickup candidate
// BEFORE the cap check. Both the fire path (autoPickupCmds) and the blocked cue
// (autoPickupBlocked) share it so they never disagree. A non-member (no correlation), a
// non-skip source, a mid-pipeline status, or a live session all disqualify it.
func (m Model) autoPickupReady(f *contract.Feature) bool {
	if f == nil || len(f.Correlations) == 0 {
		return false // only plan-spawned members auto-pick-up (they carry a correlation)
	}
	if f.Status != "plan-accepted" {
		return false // the exact initial-handoff state a first manual `gogo go` acts on
	}
	// FR8 on the ONE launch path with no human in the loop. An unwritten plan.md is not a
	// cap problem to wait out, it is ILLEGAL: there is nothing to build. The board's `m`/`M`
	// and `gogo go` both refuse this exact card, so without this check the UNATTENDED path
	// would be the most permissive one - a `claude -p /gogo:go <slug> --skip-acceptance`
	// fired at a folder with no plan in it, which is precisely the aggravating case this
	// feature exists to close (an analyst writing state.md before plan.md, in a source whose
	// planAcceptanceSkip auto-clears the gate). Returning false here also routes the card to
	// the plan-not-written cue instead of the misleading "trigger manually", because
	// autoPickupBlocked shares this helper.
	if f.PlanUnwritten {
		return false
	}
	planSkip, _ := projects.SkipForSource(m.capWatchSources(), f.Root)
	if !planSkip {
		return false // the source did NOT opt out of the plan gate — it waits for a manual go
	}
	return liveSessionFor(f.Slug, m.sessions) == "" // no live session — the dedupe guard
}

// autoPickupFreeSlot reports whether f's SOURCE is under its concurrency cap — the second
// half of eligibility. The cap is the source's INTEGER ConcurrentWorkItems, and the active
// count is scoped to the source's own Root (per-source, never global): a cap of 2 with one
// job running still has a free slot; 1/1 or 2/2 does not. cap 0 (unlimited) is always free.
func (m Model) autoPickupFreeSlot(f *contract.Feature) bool {
	cap := orchestrator.CapForSource(m.capWatchSources(), f.Root)
	active := orchestrator.ActiveWorkCount(m.repo, f.Root, m.sessions, f.Slug)
	return !orchestrator.CapExceeded(cap, active)
}

// autoPickupBlocked reports whether f is an auto-pickup candidate BLOCKED by its source's
// cap (repo busy) — the render-time predicate that drives the "needs manual trigger" cue on
// the work-board card + the plan card. Transient: it stores nothing, so the cue clears the
// moment a slot frees and the item auto-fires.
func (m Model) autoPickupBlocked(f *contract.Feature) bool {
	return m.autoPickupReady(f) && !m.autoPickupFreeSlot(f)
}

// autoPickupResultMsg carries an auto-pickup launch outcome back to Update. On a FAILED
// launch it names the fire-once key so the handler can UN-record it — a launcher error must
// not strand the member permanently fired-but-never-launched (REV-001); the next reload
// then retries it. A successful launch keeps the key recorded (fire-once holds).
type autoPickupResultMsg struct {
	key    string // the composite featureKey the reconcile recorded
	ok     bool   // the launch succeeded
	status string // the status-line message
}

// autoPickupCmds is the reload-driven auto-pickup reconcile: for each eligible member
// (skip-source, plan-accepted, no session) whose source has a FREE slot and that has NOT
// already been auto-fired, it builds a `/gogo:go` launch (byte-for-byte a manual go, via
// intentFor(ActionGo) — carrying the source's SkipParams) and records the composite
// featureKey in the fire-once set so a CONCURRENT reload (before the launch resolves) never
// double-launches it. A cap-blocked candidate is NOT recorded (its slot may free later → it
// auto-fires then), and a FAILED launch un-records the key via autoPickupResultMsg so the
// next reload retries (REV-001). The CLI writes NO source state. Returns the launch cmds
// (the caller batches them with waitForReload); empty unless claude is on PATH. Pointer
// receiver — it mutates the fire-once set on the Model the reload handler returns.
func (m *Model) autoPickupCmds() []tea.Cmd {
	if !m.hasClaude || m.repo == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, f := range m.repo.Features {
		if f == nil {
			continue
		}
		key := featureKey(f)
		if m.autoPickedUp[key] {
			continue // already auto-fired this cockpit lifetime (fire-once)
		}
		if !m.autoPickupReady(f) || !m.autoPickupFreeSlot(f) {
			continue // not eligible, or its source is at cap (a cap-skip is NOT recorded)
		}
		intent := m.intentFor(launch.ActionGo, f)
		if m.autoPickedUp == nil {
			m.autoPickedUp = map[string]bool{}
		}
		m.autoPickedUp[key] = true // record synchronously (dedupe a concurrent reload)
		cmds = append(cmds, autoPickupLaunch(m.launcher, intent, key))
	}
	return cmds
}

// autoPickupLaunch fires an auto-pickup's /gogo:go intent through the launcher seam and
// reports the outcome (carrying the fire-once key so a failure can be un-recorded). The
// SAME seam as a manual go, so the session is attachable + identical (a test's fake
// launcher observes the fired ActionGo intent + its root/skip).
func autoPickupLaunch(launcher func(string, launch.Intent) (launch.Result, error), intent launch.Intent, key string) tea.Cmd {
	root := intent.Root
	return func() tea.Msg {
		if root == "" {
			return autoPickupResultMsg{key: key, ok: false, status: "auto-pickup skipped - no root for " + intent.Command}
		}
		res, err := launcher(root, intent)
		if err != nil {
			return autoPickupResultMsg{key: key, ok: false, status: "auto-pickup failed: " + err.Error()}
		}
		where := res.Session
		if where == "" {
			where = res.LogPath
		}
		return autoPickupResultMsg{key: key, ok: true, status: "auto-picked up " + intent.Command + " (" + where + ")"}
	}
}

// planPickupCue returns the "needs manual trigger" cue for a PLAN card (plans-board FR6)
// when ANY of the plan's member work items is auto-pickup-blocked by its source's cap — the
// plan-level mirror of the work-board card cue, so a busy-blocked member is visible on both
// surfaces. "" when nothing is blocked. `plain` drops the tint for the focused card's single
// fg/bg fill.
func (m Model) planPickupCue(p plans.Plan, plain bool) string {
	blocked := false
	for _, t := range p.Targets {
		if f := m.spawnedFeature(t, p.ID); f != nil && m.autoPickupBlocked(f) {
			blocked = true
			break
		}
	}
	if !blocked {
		return ""
	}
	label := waitingMarker + " trigger manually"
	if plain {
		return label
	}
	return pillRed.Render(label)
}
