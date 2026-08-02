package tui

// Session-binding ops (P start · K kill · R re-assign). A gogo tmux session is
// bound to a work item by exactly one thing — its NAME — minted once at launch
// and re-derived by every reader on every tick. These three card-anchored ops
// make that binding editable from the cockpit: `P` mints a correctly-named
// authoring session (launch → attach), `K` prunes sessions from where the user
// sees them (board + drill), and `R` renames a live session onto the item it is
// really driving — one `tmux rename-session`, after which the dot, the cues, the
// cap, the lock and the sweeper all correct themselves. None of them writes
// pipeline state: renaming/killing a tmux session is not a state write.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// bindAction derives the ACTION component a session adopted onto f takes (D4=A):
// a runnable target (plan-accepted / implementing / reviewing / testing) mints a
// BUILD session (gogo-go-<slug>) — which the per-source concurrency cap counts —
// anything else an authoring one (gogo-plan-<slug>). Derived from the target's
// status, never preserved from the old name: the incident's surviving pane is
// typically a gogo-done-<A> (the ship's own host), and preserving `done` would
// re-bind it as gogo-done-<B>, leaving a real build invisible to the cap — the
// working-tree-clobber guard.
func bindAction(f *contract.Feature) launch.Action {
	if f != nil && orchestrator.RunnableStatus(f.Status) {
		return launch.ActionGo
	}
	return launch.ActionPlan
}

// boundFeature returns the first loaded feature the session name attributes to
// (exact gogo-<action>-<label>[-N] convention parse — never substring, TEST-005),
// or nil: the session is bound to nothing this board shows.
func boundFeature(name string, features []*contract.Feature) *contract.Feature {
	for _, f := range features {
		if f != nil && launch.SessionMatchesSlug(name, f.Slug) {
			return f
		}
	}
	return nil
}

// unboundSessions returns the live gogo-* sessions that are anchored inside one
// of the roots this board shows AND attribute to no feature here (FR4) — the
// `gogo-plan-<plan title>` plans-tab spawn case, and a retasked session before it
// is adopted. A session anchored in a repo this board does NOT show is excluded:
// it belongs to another board, so counting it would nag the wrong user.
func unboundSessions(metas []launch.SessionMeta, features []*contract.Feature, roots map[string]bool) []launch.SessionMeta {
	var out []launch.SessionMeta
	for _, meta := range metas {
		inRoot := false
		for root := range roots {
			if pathWithinRoot(meta.Path, root) {
				inRoot = true
				break
			}
		}
		if !inRoot {
			continue
		}
		if boundFeature(meta.Name, features) == nil {
			out = append(out, meta)
		}
	}
	return out
}

// pathWithinRoot reports whether path is root or inside it (cleaned compare) —
// the session_path anchor test the adopt refusal and the unbound count share. A
// claude anchored at another repo cannot write this item, and a false ● is worse
// than none.
func pathWithinRoot(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	p, r := filepath.Clean(path), filepath.Clean(root)
	return p == r || strings.HasPrefix(p, r+string(filepath.Separator))
}

// boardRoots is the set of repo roots this board shows (cleaned): the single
// root, every loaded feature's own root, and every registered source path (a
// source with zero work items still anchors sessions).
func (m Model) boardRoots() map[string]bool {
	roots := map[string]bool{}
	if m.root != "" {
		roots[filepath.Clean(m.root)] = true
	}
	for _, f := range m.repo.Features {
		if f != nil && f.Root != "" {
			roots[filepath.Clean(f.Root)] = true
		}
	}
	for _, s := range m.capWatchSources() {
		if s.Path != "" {
			roots[filepath.Clean(s.Path)] = true
		}
	}
	return roots
}

// unboundHere is the board's unbound-session set (FR4) — the idle status line
// counts it. Pure in-memory (cached sessionMeta ⨯ loaded features), no disk IO
// on the render path.
func (m Model) unboundHere() []launch.SessionMeta {
	return unboundSessions(m.sessionMeta, m.repo.Features, m.boardRoots())
}

// metaFor returns the cached SessionMeta for a live session name, or nil.
func (m Model) metaFor(name string) *launch.SessionMeta {
	for i := range m.sessionMeta {
		if m.sessionMeta[i].Name == name {
			return &m.sessionMeta[i]
		}
	}
	return nil
}

// sessionAge renders a session's age compactly for the adopt picker rows
// ("now" / "35m" / "2h" / "3d"); "" for an unknown created time. Cosmetic, and
// computed once at picker-open (a keystroke), never per frame.
func sessionAge(created time.Time) string {
	if created.IsZero() {
		return ""
	}
	d := time.Since(created)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// --- P: plan this item in an attached tmux session (FR1) ----------------------

// planFeature (P on a card, board + drill) starts — or joins — the item's
// authoring session: a confirm, then `claude "/gogo:plan <slug>"` in a detached
// tmux session anchored at the card's OWN root, then the TUI suspends and
// attaches (the plans-tab `A` shape). The session name is gogo-plan-<slug> — the
// slug is real, so the binding is exact by construction. Uncapped (planning
// never touches the working tree — the same rule `gogo plan` uses) and
// lock-free, exactly like every other board launch.
func (m Model) planFeature(f *contract.Feature) (tea.Model, tea.Cmd) {
	if f == nil {
		return m, nil
	}
	// PlannableStatus, not an inlined !TerminalStatus (REV-004): `gogo plan` gates
	// on the same named producer, so the two doors to the same act can never drift.
	if !orchestrator.PlannableStatus(f.Status) {
		m.statusBlocked(f.Slug + " is " + f.Status + " — nothing to plan")
		return m, nil
	}
	if !m.hasClaude {
		m.statusBlocked("claude CLI not on PATH — cannot start a plan session")
		return m, nil
	}
	// An existing live authoring session for this slug: ATTACH it instead of
	// minting a second (FR1) — the exact-name convention parse tells authoring
	// (gogo-plan-<slug>) from build (gogo-go-<slug>).
	for _, s := range m.sessions {
		if a, ok := launch.SessionAction(s, f.Slug); ok && a == launch.ActionPlan {
			return m.attachSession(s)
		}
	}
	m.startPlanSessionForm(f)
	return m, m.form.Init()
}

// startPlanSessionForm opens the `P` launch confirm (forward pipeline move →
// Enter submits, the confirm-default convention in move.go's
// startFormOverriding), recording the origin mode so a cancel restores it.
func (m *Model) startPlanSessionForm(f *contract.Feature) {
	in := launch.BuildIntent(launch.ActionPlan, []string{f.Slug}, "")
	in.Root = m.rootFor(f)
	m.pending = in
	m.pendingPlanSession = f
	m.formOrigin = m.mode
	m.binding = &formBinding{confirm: true}
	m.form = newForm(huh.NewGroup(
		huh.NewConfirm().
			Title(m.confirmSummary(in)).
			Description("plans " + f.Slug + " in an attached tmux session — uncapped (planning never touches the working tree)").
			Affirmative("Launch").
			Negative("Cancel").
			Value(&m.binding.confirm),
	))
	m.mode = modeForm
}

// finishPlanSession runs after a completed P confirm: fire the launcher EXACTLY
// once, then hand the created session to Update as a planAuthorLaunchedMsg — the
// shared launch→attach shape — so a tmux launch attaches the user in and the
// no-tmux fallback surfaces the backgrounded run's log path in the status line.
func (m Model) finishPlanSession() (tea.Model, tea.Cmd) {
	f := m.pendingPlanSession
	in := m.pending
	confirmed := m.binding != nil && m.binding.confirm
	m.pendingPlanSession = nil
	m.pending = launch.Intent{}
	m.binding = nil
	m.form = nil
	m.mode = m.formOrigin
	if !confirmed || f == nil {
		m.status = "cancelled"
		return m, nil
	}
	if in.Root == "" {
		// Never launch relative to the process cwd — bounce, launching nothing (the
		// same no-cwd-launch rule doLaunch enforces in move.go).
		m.statusBlocked("no repo root for " + f.Slug + " — nothing launched")
		return m, nil
	}
	root := in.Root
	launcher := m.launcher
	return m, func() tea.Msg {
		res, err := launcher(root, in)
		if err != nil {
			return launchDoneMsg{status: "plan session failed: " + err.Error(), level: statusLevelErr}
		}
		return planAuthorLaunchedMsg{session: res.Session, logPath: res.LogPath}
	}
}

// --- K: kill a selected session, from the board too (FR2) ---------------------

// killFeature (K on the board or in the drill) kills f's live session(s) behind
// the existing explicit confirm/picker — promoted out of the drill so a
// lingering session is killable from where the user sees it, including a
// shipped changelog card. Behaviour is the drill's, unchanged: 0 → a blocked
// hint; 1 → the Enter-safe confirm; ≥2 → the one / all N / cancel picker,
// killing by EXACT name. No pipeline state is written.
func (m Model) killFeature(f *contract.Feature) (tea.Model, tea.Cmd) {
	if f == nil {
		return m, nil
	}
	if !m.hasTmux {
		m.statusBlocked("tmux not installed — no session to kill")
		return m, nil
	}
	sessions := liveSessionsFor(f.Slug, m.sessions)
	switch len(sessions) {
	case 0:
		m.statusBlocked("no live session to kill for " + f.Slug)
		return m, nil
	case 1:
		// Exactly one session → keep the existing single-confirm UX (D2).
		m.startKillForm(f, sessions)
	default:
		// ≥2 sessions → a picker: one, or "all N", or Cancel (FR-3).
		m.startKillPicker(f, sessions)
	}
	return m, m.form.Init()
}

// --- R: re-assign a live session onto this work item (FR3) --------------------

// adoptFeature (R on a card, board + drill) opens a Select of EVERY live gogo-*
// session — each row reading name · bound item | unbound · repo · age — plus
// Cancel. The choice is the confirmation (the kill picker's shape); the chosen
// session is renamed to gogo-<derived action>-<slug> and every reader corrects
// itself on the next session tick. Refusals all name their reason (FR3).
func (m Model) adoptFeature(f *contract.Feature) (tea.Model, tea.Cmd) {
	if f == nil {
		return m, nil
	}
	if orchestrator.TerminalStatus(f.Status) {
		// The target must be drivable. The shipped card's own lingering session is
		// re-assigned FROM the card it should drive — name that move (Diagnosability:
		// a refusal carries its unblock).
		m.statusBlocked(f.Slug + " is " + f.Status + " — nothing to drive; press R on the card the session should drive (this one's session appears in that picker), or K to kill it")
		return m, nil
	}
	if !m.hasTmux {
		m.statusBlocked("tmux not installed — no session to re-assign")
		return m, nil
	}
	if len(m.sessionMeta) == 0 {
		m.statusBlocked("no live gogo-* session to re-assign")
		return m, nil
	}
	m.startAdoptPicker(f)
	return m, m.form.Init()
}

// startAdoptPicker opens the R picker over every live gogo-* session. The
// description SHOWS the resulting name (D4), so the user sees that adopting a
// building item mints a BUILD session — which counts against the source cap.
func (m *Model) startAdoptPicker(f *contract.Feature) {
	m.pendingAdopt = f
	m.formOrigin = m.mode
	m.binding = &formBinding{}
	newName := launch.SessionNameFor(bindAction(f), f.Slug)
	opts := make([]huh.Option[string], 0, len(m.sessionMeta)+1)
	for _, meta := range m.sessionMeta {
		opts = append(opts, huh.NewOption(m.adoptRow(meta), meta.Name))
	}
	opts = append(opts, huh.NewOption("Cancel", adoptCancel))
	m.form = newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Re-assign which session onto " + f.Slug + "?").
			Description("renames the chosen session to " + newName + " — every reader (dot, cues, cap, lock, sweep) follows the name; no pipeline state is written").
			Options(opts...).
			Value(&m.binding.selected),
	))
	m.mode = modeForm
}

// adoptRow renders one adopt-picker row: name · bound: <item> (<status>) |
// unbound · <repo> · <age> [· attached]. `attached` (a word, not a colour — the
// colourless-terminal rule) marks a session a human client is on RIGHT NOW, so
// renaming it out from under them is a deliberate act, not an accident (REV-005).
func (m Model) adoptRow(meta launch.SessionMeta) string {
	bound := "unbound"
	if f := boundFeature(meta.Name, m.repo.Features); f != nil {
		bound = "bound: " + f.Slug
		if f.Status != "" {
			bound += " (" + f.Status + ")"
		}
	}
	row := meta.Name + " · " + bound
	if meta.Path != "" {
		row += " · " + filepath.Base(meta.Path)
	}
	if age := sessionAge(meta.Created); age != "" {
		row += " · " + age
	}
	if meta.Attached {
		row += " · attached"
	}
	return row
}

// finishAdopt runs after the adopt picker completes: resolve the choice, then
// hand (session, target) to the shared reassign core.
func (m Model) finishAdopt() (tea.Model, tea.Cmd) {
	target := m.pendingAdopt
	sel := ""
	if m.binding != nil {
		sel = m.binding.selected
	}
	m.pendingAdopt = nil
	m.binding = nil
	m.form = nil
	m.mode = m.formOrigin
	if sel == "" || sel == adoptCancel || target == nil {
		m.status = "cancelled"
		return m, nil
	}
	return m.reassign(sel, target), nil
}

// reassign is the ONE producer of the re-assign act (FR4.4 — a user-visible rule
// stated twice must be one constant): the refusal chain (already bound · foreign
// anchor), the derived-action rename via the renamer seam, and the status
// message. Both doors call it — the card-anchored `R` (finishAdopt, session
// picked) and the sessions panel's `R` (finishReassignSession, target picked).
// Nothing else is written (D3=A): no pipeline state, no registry rewrite — the
// old card's tracked leg truthfully renders stale.
func (m Model) reassign(session string, target *contract.Feature) Model {
	if launch.SessionMatchesSlug(session, target.Slug) {
		m.statusBlocked(session + " is already bound to " + target.Slug + " — nothing to move")
		return m
	}
	root := m.rootFor(target)
	if meta := m.metaFor(session); meta != nil && meta.Path != "" && !pathWithinRoot(meta.Path, root) {
		m.statusBlocked("that session is anchored at " + meta.Path + ", but " + target.Slug +
			" lives in " + root + " — a claude working elsewhere cannot drive this item")
		return m
	}
	base := launch.SessionNameFor(bindAction(target), target.Slug)
	newName, err := m.renamer(session, base)
	if err != nil {
		m.statusFailed("re-assign failed: " + err.Error())
		return m
	}
	m.refreshSessions()
	m.clampSessIdx()
	if m.drill != nil {
		m.loadDrillCard(m.drill) // refresh live/stale rows after the move
	}
	m.setStatus(statusLevelOK, "re-assigned "+session+" → "+newName+" — "+target.Slug+"'s readers follow the new name")
	return m
}

// --- S: the sessions panel (0.33.0 FR3-FR5) ----------------------------------

// openSessions (S on the board or in the drill) opens the sessions panel —
// modeSessions, a full-screen live list of EVERY gogo-* session from the cached
// meta. It ALWAYS opens (FR3.2): an empty panel names why it is empty, so the
// key never reads as a dead no-op. It records the opening mode so esc/q return
// exactly there (D2=C).
func (m Model) openSessions() (tea.Model, tea.Cmd) {
	m.sessionsOrigin = m.mode
	m.mode = modeSessions
	m.clampSessIdx()
	m.status = ""
	return m, nil
}

// focusedSession returns the panel's cursored session meta, or nil on an empty
// list.
func (m Model) focusedSession() *launch.SessionMeta {
	if len(m.sessionMeta) == 0 {
		return nil
	}
	return &m.sessionMeta[clamp(m.sessIdx, 0, len(m.sessionMeta)-1)]
}

// clampSessIdx keeps the panel cursor in range — the list is LIVE (the 5s
// session tick refreshes it in every mode), so rows can vanish under the cursor.
func (m *Model) clampSessIdx() {
	m.sessIdx = clamp(m.sessIdx, 0, len(m.sessionMeta)-1)
}

// hasDrivableFeature reports whether any loaded work item is non-terminal —
// the panel `R`'s zero-target guard shares this with the picker's own filter,
// so the refusal and the row set can never disagree.
func (m Model) hasDrivableFeature() bool {
	for _, f := range m.repo.Features {
		if f != nil && !orchestrator.TerminalStatus(f.Status) {
			return true
		}
	}
	return false
}

// startReassignPicker opens the panel's `R` target Select (FR4.1): every
// DRIVABLE (non-terminal) work item, each row reading
// `<slug> · <status> · → <resulting name>` — the name bindAction derives, so the
// user sees that adopting onto a runnable item mints a cap-counted build session.
// All drivable targets are shown, never silently filtered (FR4.2): a refusal
// explains, a missing row puzzles.
func (m *Model) startReassignPicker(session string) {
	m.pendingReassign = session
	m.formOrigin = m.mode
	m.binding = &formBinding{}
	var opts []huh.Option[string]
	for _, f := range m.repo.Features {
		if f == nil || orchestrator.TerminalStatus(f.Status) {
			continue
		}
		status := f.Status
		if status == "" {
			status = "no status"
		}
		row := f.Slug + " · " + status + " · → " + launch.SessionNameFor(bindAction(f), f.Slug)
		opts = append(opts, huh.NewOption(row, featureKey(f)))
	}
	opts = append(opts, huh.NewOption("Cancel", adoptCancel))
	m.form = newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Re-assign " + session + " onto which work item?").
			Description("renames the session — every reader (dot, cues, cap, lock, sweep) follows the name; no pipeline state is written").
			Options(opts...).
			Value(&m.binding.selected),
	))
	m.mode = modeForm
}

// finishReassignSession runs after the panel's `R` picker completes: resolve the
// chosen feature by its workspace-unique key, then hand (session, target) to the
// shared reassign core. The user stays in the panel (formOrigin).
func (m Model) finishReassignSession() (tea.Model, tea.Cmd) {
	session := m.pendingReassign
	sel := ""
	if m.binding != nil {
		sel = m.binding.selected
	}
	m.pendingReassign = ""
	m.binding = nil
	m.form = nil
	m.mode = m.formOrigin
	if sel == "" || sel == adoptCancel || session == "" {
		m.status = "cancelled"
		return m, nil
	}
	var target *contract.Feature
	for _, f := range m.repo.Features {
		if featureKey(f) == sel {
			target = f
			break
		}
	}
	if target == nil {
		m.statusBlocked("that work item is no longer present — nothing renamed")
		return m, nil
	}
	return m.reassign(session, target), nil
}

// startKillSessionForm opens the panel's `K` destructive confirm for the exact
// named session — Cancel default (the confirm-default convention: destructive
// actions must not submit on a bare Enter).
func (m *Model) startKillSessionForm(session string) {
	m.pendingKillSession = session
	m.formOrigin = m.mode
	m.binding = &formBinding{confirm: false}
	m.form = newForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Close session " + session + "?").
			Description("kills the tmux session (exact name) — the pipeline state is untouched").
			Affirmative("Kill").
			Negative("Cancel").
			Value(&m.binding.confirm),
	))
	m.mode = modeForm
}

// finishKillSession runs after the panel's `K` confirm: kill exactly the named
// session via the killer seam, refresh, clamp the cursor, stay in the panel. The
// status line carries tmux's own words on failure.
func (m Model) finishKillSession() (tea.Model, tea.Cmd) {
	session := m.pendingKillSession
	confirmed := m.binding != nil && m.binding.confirm
	m.pendingKillSession = ""
	m.binding = nil
	m.form = nil
	m.mode = m.formOrigin
	if !confirmed || session == "" {
		m.status = "cancelled"
		return m, nil
	}
	if err := m.killer(session); err != nil {
		m.statusFailed("close failed: " + err.Error())
		return m, nil
	}
	m.refreshSessions()
	m.clampSessIdx()
	m.setStatus(statusLevelOK, "closed "+session)
	return m, nil
}
