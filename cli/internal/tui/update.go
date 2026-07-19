package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// viewerContentMsg carries an async render (TEST-003) back to the model.
type viewerContentMsg struct {
	key     string // cacheKey(artifact, width) it was rendered for
	content string
}

// sessionsMsg carries a refreshed tmux session set (TEST-006 ticker).
type sessionsMsg struct{ sessions []string }

// sessionRefresh is the modest ticker that keeps the card session dots fresh
// even when no fsnotify write fires. tmux is listed off the UI goroutine.
const sessionRefresh = 5 * time.Second

func sessionTick() tea.Cmd {
	return tea.Tick(sessionRefresh, func(time.Time) tea.Msg {
		return sessionsMsg{sessions: launch.ListSessions()}
	})
}

// Update is the pure state transition. It dispatches by mode; the decision
// logic (selection, move guards) lives in small methods the tests call.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		prevWidth := m.width
		m.width, m.height = msg.Width, msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = maxInt(msg.Height-3, 1)
		m.viewerReady = true
		// A resize changes how many cards fit per column — re-window (TEST-014).
		m.reflowColumns()
		// A live form lays itself out from the window size too.
		if m.mode == modeForm {
			return m.updateForm(msg)
		}
		// Re-render an open viewer ONLY when the width actually changed — the
		// render cache is width-keyed, so a same-width resize is a no-op (TEST-003).
		if m.mode == modeViewer && msg.Width != prevWidth && m.curArtifact.Path != "" {
			return m, m.openArtifact(m.curArtifact)
		}
		return m, nil

	case watcherReadyMsg:
		m.watch = msg.ws
		return m, nil

	case viewerContentMsg:
		// Cache the finished render; apply it only if it is still what the viewer
		// is showing (the user may have moved on while it rendered).
		m.renderCache[msg.key] = msg.content
		if m.mode == modeViewer && cacheKey(m.curArtifact, m.width) == msg.key {
			m.viewport.SetContent(msg.content)
			m.viewport.GotoTop()
			m.viewerLoading = false
		}
		return m, nil

	case sessionsMsg:
		m.sessions = msg.sessions
		return m, sessionTick()

	case peekContentMsg:
		// A finished peek capture (FR7). Apply it only if the viewer is still the
		// peek that requested it (the user may have moved on).
		if m.mode == modeViewer && m.peeking && msg.slug == m.peekSlug {
			m.viewport.SetContent(msg.content)
			m.viewport.GotoTop()
			m.viewerLoading = false
		}
		return m, nil

	case spinner.TickMsg:
		if m.mode == modeViewer && m.viewerLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case reloadMsg:
		// Keep the focused slug under the cursor across the reload (indices can
		// shift as features appear/vanish), then re-window so it stays visible.
		slug := ""
		if f := m.focusedCard(); f != nil {
			slug = f.Slug
		}
		m.reload()
		m.refocus(slug)
		m.reflowColumns()
		// Re-arm the watch so features/entries born mid-session keep the board
		// live (REV-010); reconcile also drops any that vanished.
		if m.watch != nil {
			m.watch.reconcile(m.watchDirs())
		}
		return m, waitForReload(m.reloadCh)

	case launchDoneMsg:
		m.status = msg.status
		m.sessions = launch.ListSessions()
		// A plan spawn records its member + active flip only on a SUCCESSFUL launch
		// (REV-005), inside the fired cmd — so re-read the project's plans here to catch
		// the Model up to that store write. A no-op on a single-repo board (no plans).
		m.loadPlans()
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeViewer:
			return m.updateViewer(msg)
		case modeDrill:
			return m.updateDrill(msg)
		default:
			return m.updateActive(msg)
		}
	}

	// Any OTHER message type. A live huh form MUST see every one of these: it
	// advances between fields and submits via its OWN async messages
	// (nextFieldMsg/nextGroupMsg, focus/blur, blink ticks) that round-trip
	// through the Bubble Tea runtime. Dropping them here — the pre-fix behaviour,
	// which only forwarded tea.KeyMsg — left every form permanently unsubmittable
	// (TEST-001); only the synchronous ctrl+c abort ever escaped.
	if m.mode == modeForm {
		return m.updateForm(msg)
	}
	if m.mode == modeViewer {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// updateActive is the tabbed dispatch (FR8/D6): it owns the keys that persist
// across every tab (q quit · ? help · tab/shift+tab cycle · / filter) and then
// routes the remaining keys to the active tab's handler. On a lone repo (no tabs)
// the tab keys are inert and it always lands on the board — byte-for-byte fallback.
func (m Model) updateActive(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.updateFilter(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, m.quit()
	case "?":
		// FR-10: toggle the full key list (the pre-redesign long help line).
		m.showAllKeys = !m.showAllKeys
		return m, nil
	case "tab":
		if m.global() {
			m.cycleTab(1)
		}
		return m, nil
	case "shift+tab":
		if m.global() {
			m.cycleTab(-1)
		}
		return m, nil
	}
	switch m.tab {
	case tabPlans:
		return m.updatePlans(msg)
	case tabConfig:
		return m.updateConfig(msg)
	default:
		return m.updateBoard(msg)
	}
}

func (m Model) updateBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		m.colIdx = clamp(m.colIdx-1, 0, 3)
	case "right":
		// `l` is the log-peek key (FR7), so column-right is the arrow only here;
		// `h` stays a left alias. Slightly asymmetric on purpose — peek won `l`.
		m.colIdx = clamp(m.colIdx+1, 0, 3)
	case "up", "k":
		m.cardIdx[m.colIdx] = clamp(m.cardIdx[m.colIdx]-1, 0, len(m.cols[m.colIdx])-1)
	case "down", "j":
		m.cardIdx[m.colIdx] = clamp(m.cardIdx[m.colIdx]+1, 0, len(m.cols[m.colIdx])-1)
	case " ", "space":
		m.toggleSelect()
	case "/":
		m.filtering = true
		m.status = "filter: type to narrow · enter keeps · esc clears"
	case "p":
		// FR7: cycle the board's source filter chip (all → source-1 → … → all). A
		// no-op on a lone repo / a project with a single source.
		m.cycleChip(1)
	case "enter":
		if f := m.focusedCard(); f != nil {
			m.openDrill(f)
		}
	case "v":
		if f := m.focusedCard(); f != nil {
			return m, m.quickView(f)
		}
	case "w":
		return m, m.buildPageCmd()
	case "m":
		return m.launchAction(false)
	case "d":
		return m.launchAction(true)
	case "a":
		return m.attachFocused()
	case "l":
		return m.peekFocused()
	case "x":
		return m.deleteFocused()
	}
	// Cursor/column moves change which card is focused — re-window so it stays
	// fully visible (scroll-into-view), and refresh each column's offset (TEST-014).
	m.reflowColumns()
	return m, nil
}

// toggleSelect flips selection — ONLY for ready-to-ship cards (space guard).
func (m *Model) toggleSelect() {
	f := m.focusedCard()
	if f == nil {
		return
	}
	if f.Class != contract.ClassReadyToShip {
		m.status = "select only ready cards (space) — this card is " + f.Class
		return
	}
	if m.selected[f.Slug] {
		delete(m.selected, f.Slug)
	} else {
		m.selected[f.Slug] = true
	}
	m.status = ""
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filtering = false
		m.status = ""
	case "esc":
		m.filtering = false
		m.filter = ""
		m.rebuild()
		m.status = ""
	case "backspace":
		if m.filter != "" {
			m.filter = m.filter[:len(m.filter)-1]
			m.rebuild()
		}
	default:
		if len(msg.Runes) > 0 {
			m.filter += string(msg.Runes)
			m.rebuild()
		}
	}
	// A filter change re-partitions the columns — re-clamp every scroll offset
	// into range (empty column → 0) so the window never points past the end.
	m.reflowColumns()
	return m, nil
}

func (m Model) updateDrill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "left", "h":
		m.mode = modeBoard
	case "up", "k":
		m.artIdx = clamp(m.artIdx-1, 0, len(m.artifacts)-1)
	case "down", "j":
		m.artIdx = clamp(m.artIdx+1, 0, len(m.artifacts)-1)
	case "enter", "v", "right", "l":
		if m.artIdx < len(m.artifacts) {
			return m, m.openArtifact(m.artifacts[m.artIdx])
		}
	case "a":
		// FR-B3: attach the drilled card's live session (same path as the board `a`).
		return m.attachFeature(m.drill)
	case "K":
		// FR-B3: kill the drilled card's live session(s) behind a confirm. Capital
		// K — `k` stays up-nav (D2).
		return m.killDrill()
	case "w":
		return m, m.buildPageCmd()
	case "G":
		// glow (soft dep) lives on the FILE LIST context (TEST-010): it opens the
		// HIGHLIGHTED file in the full glow pager. Freeing `G` here lets the open
		// VIEWER use g/G for top/bottom without a conflict.
		return m.openInGlow()
	}
	return m, nil
}

// updateViewer wires the viewport paging keys (TEST-010). Every one of these
// only moves the viewport's yOffset over its CACHED, already-rendered lines —
// no per-keypress re-render (bubbles/viewport slices m.lines; the width-keyed
// render cache is only touched on an actual width change). Key-conflict
// resolution: `space` and `d`/`u` act as paging ONLY in this viewer context
// (on the board, `space`=select and `d`=ship are untouched); `g`/`G` are
// top/bottom here, while `G`=glow stays on the file-list context.
func (m Model) updateViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "left", "h":
		if m.peeking {
			return m.closePeek(), nil // peek launched from the board → back to board
		}
		m.mode = modeDrill
		return m, nil
	case "r":
		// FR7: re-capture the session/log while peeking (no-op in a normal viewer).
		if m.peeking {
			m.viewerLoading = true
			return m, tea.Batch(m.capturePeekCmd(), m.spinner.Tick)
		}
	case "a":
		// FR7: escalate a read-only peek to a full attach.
		if m.peeking {
			return m.attachFromPeek()
		}
	case "w":
		return m, m.buildPageCmd()
	case "g":
		m.viewport.GotoTop()
		return m, nil
	case "G":
		m.viewport.GotoBottom()
		return m, nil
	case "pgdown", " ", "space", "f":
		m.viewport.PageDown()
		return m, nil
	case "pgup", "b":
		m.viewport.PageUp()
		return m, nil
	case "d", "ctrl+d":
		m.viewport.HalfPageDown()
		return m, nil
	case "u", "ctrl+u":
		m.viewport.HalfPageUp()
		return m, nil
	}
	// Line scroll (j/k, ↑/↓) via the viewport's own keymap — also cache-only.
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// updateForm drives the active huh form. It takes any tea.Msg (not just keys)
// because huh's field-advance/submit protocol is async — see Update's fallthrough.
func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.form == nil {
		m.mode = modeBoard
		return m, nil
	}
	// Esc aborts. huh binds only ctrl+c to Quit, so without this Esc would do
	// nothing and a launch could feel un-escapable; ctrl+c still flows through
	// the form and lands in StateAborted below.
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyEsc {
		return m.cancelForm(m.formPreservesSelection()), nil
	}
	fm, cmd := m.form.Update(msg)
	if f, ok := fm.(*huh.Form); ok {
		m.form = f
	}
	switch m.form.State {
	case huh.StateCompleted:
		// A delete confirm (FR6) is its own completion path — move to trash or cancel.
		if m.pendingDelete != nil {
			return m.finishDelete()
		}
		// A drill-in kill confirm/picker (FR-B3/FR-3) is its own completion path too.
		if m.pendingKill != nil {
			return m.finishKill()
		}
		// The attach picker (FR-2) completes to finishAttach — mutually exclusive
		// with the delete/kill paths above and the ship path below.
		if m.pendingAttach != nil {
			return m.finishAttach()
		}
		// A config-tab per-source add/edit/remove form (FR9) completes to
		// finishSourceForm — its own path (stays on the config tab), mutually
		// exclusive with every launch/kill/attach path above and the ship path below.
		if m.pendingSource != nil {
			return m.finishSourceForm()
		}
		// A plans-tab new-plan form (FR10 `n`) completes to finishPlanForm — its own
		// path (stays on the plans tab), mutually exclusive with every path above.
		if m.pendingPlan {
			return m.finishPlanForm()
		}
		if m.binding == nil || !m.binding.confirm {
			// Completed on "Cancel" — same as an abort. Only a SHIP form reaches
			// here (delete/kill forms are handled above), so the pending targets are
			// nil and the ready-ship selection is (correctly) cleared.
			return m.cancelForm(m.formPreservesSelection()), nil
		}
		// Build the launch command NOW (it captures pending + the edited release
		// name), then clear the consumed launch state on the model we return so a
		// later m/d can never re-ship this selection (TEST-002).
		launchCmd := m.doLaunch()
		m.selected = map[string]bool{}
		m.pending = launch.Intent{}
		m.pendingShip = false
		m.binding = nil
		m.form = nil
		m.mode = modeBoard
		return m, launchCmd
	case huh.StateAborted:
		return m.cancelForm(m.formPreservesSelection()), nil
	}
	return m, cmd
}

// formPreservesSelection reports whether the active form is a delete, kill, attach,
// or config-tab source form — all unrelated to the ready-ship selection, so
// cancelling them must NOT wipe the user's multi-selection (only a SHIP form's
// cancel does). REV-012.
func (m Model) formPreservesSelection() bool {
	return m.pendingDelete != nil || m.pendingKill != nil || m.pendingAttach != nil || m.pendingSource != nil || m.pendingPlan
}

// cancelForm returns to the board and clears the in-flight form state. For a SHIP
// form (preserveSelection=false) it ALSO drops the pending intent AND the
// ready-card selection, so a stale, unconfirmed launch target can never be
// silently re-shipped by a later, unrelated m/d press (TEST-002). For a DELETE
// form (preserveSelection=true) the ready-ship selection is unrelated to the
// delete, so it survives — an Esc-abort of a delete now matches the Cancel-button
// (finishDelete) path instead of wiping the user's multi-selection (REV-012).
func (m Model) cancelForm(preserveSelection bool) Model {
	// A kill confirm/picker was launched FROM the drill card, and an attach picker
	// from either the board or the drill; cancelling one (Esc / abort) returns to
	// the picker's ORIGIN mode — matching the Cancel-option paths (finishKill /
	// finishAttach), not bouncing the user back to the board (REV-001). Every other
	// form (ship / delete) cancels to the board as before.
	returnMode := modeBoard
	switch {
	case m.pendingKill != nil && m.drill != nil:
		returnMode = modeDrill
	case m.pendingAttach != nil && m.pickerFromDrill:
		returnMode = modeDrill
	}
	// A config-tab per-source form was opened while the config TAB was active;
	// cancelling it returns to the tab's normal state (modeBoard renders the active
	// tab — m.tab stays tabConfig), never a modal screen (those are gone).
	m.status = "cancelled"
	if !preserveSelection {
		m.selected = map[string]bool{}
		m.pending = launch.Intent{}
		m.pendingShip = false
	}
	m.pendingDelete = nil
	m.pendingKill = nil
	m.pendingAttach = nil
	m.pendingSource = nil
	m.pendingPlan = false
	m.binding = nil
	m.form = nil
	m.mode = returnMode
	return m
}

// quit closes the fsnotify watcher (stopping its reload goroutine — REV-011)
// before asking Bubble Tea to exit, so nothing leaks past the board session.
func (m Model) quit() tea.Cmd {
	_ = m.watch.close() // nil-safe when the watcher never started
	return tea.Quit
}

// attachFocused suspends the TUI and attaches to the focused board card's tmux
// session, when one exists.
func (m Model) attachFocused() (tea.Model, tea.Cmd) {
	return m.attachFeature(m.focusedCard())
}

// attachFeature suspends the TUI and attaches to f's live tmux session
// (tea.ExecProcess). Shared by the board `a` (focused card) and the drill-in `a`
// (the drilled card, FR-B3) — one attach path, no duplication. It branches on the
// live-session count (FR-2): 0 → a hint, 1 → attach directly (current UX), ≥2 →
// an attach picker so the user chooses WHICH session (today's code grabbed the
// first exact match blindly).
func (m Model) attachFeature(f *contract.Feature) (tea.Model, tea.Cmd) {
	if f == nil {
		return m, nil
	}
	if !m.hasTmux {
		m.status = "tmux not installed — nothing to attach"
		return m, nil
	}
	sessions := liveSessionsFor(f.Slug, m.sessions)
	switch len(sessions) {
	case 0:
		m.status = "no running session for " + f.Slug
		return m, nil
	case 1:
		return m.attachSession(sessions[0])
	default:
		m.startAttachPicker(f, sessions)
		return m, m.form.Init()
	}
}

// attachSession suspends the TUI and attaches to exactly the named tmux session
// (tea.ExecProcess). It sets a synchronous "attaching <session>" status so the
// CHOSEN session is substring-assertable in tests — attach has no killer-style
// seam, so the status line is the observable (the same "status line is the
// observable" pattern viewDrill already relies on).
func (m Model) attachSession(session string) (tea.Model, tea.Cmd) {
	m.status = "attaching " + session
	c := exec.Command("tmux", launch.AttachArgs(session)...)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return launchDoneMsg{status: "detached from " + session}
	})
}

// startAttachPicker opens the FR-2 attach picker (≥2 live sessions): one option
// per session (value = session name) plus a Cancel option (attachCancel sentinel).
// It records whether the picker was opened from the drill so a cancel restores the
// right mode, and binds the choice through the heap-stable *formBinding.selected
// (TEST-001), exactly like the ship/kill forms.
func (m *Model) startAttachPicker(f *contract.Feature, sessions []string) {
	m.pendingAttach = sessions
	m.pickerFromDrill = m.mode == modeDrill
	m.binding = &formBinding{}
	opts := make([]huh.Option[string], 0, len(sessions)+1)
	for _, s := range sessions {
		opts = append(opts, huh.NewOption(s, s))
	}
	opts = append(opts, huh.NewOption("Cancel", attachCancel))
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Attach which session for " + f.Slug + "?").
			Description("choose a live session to attach (tmux) — pipeline state is untouched").
			Options(opts...).
			Value(&m.binding.selected),
	))
	m.mode = modeForm
}

// finishAttach runs after the attach picker completes. A session name → attach it
// via attachSession; the Cancel sentinel (or an empty selection) → cancel back to
// the picker's origin mode (drill vs board).
func (m Model) finishAttach() (tea.Model, tea.Cmd) {
	sel := ""
	if m.binding != nil {
		sel = m.binding.selected
	}
	originMode := modeBoard
	if m.pickerFromDrill {
		originMode = modeDrill
	}
	m.pendingAttach = nil
	m.binding = nil
	m.form = nil
	m.mode = originMode
	if sel == "" || sel == attachCancel {
		m.status = "cancelled"
		return m, nil
	}
	return m.attachSession(sel)
}

// killDrill (K in the drill — FR-B3) kills the drilled card's LIVE tmux
// session(s) behind an explicit confirm. It targets only real live sessions
// (exact-match attribution) and never touches pipeline state — killing a session
// is not a state write.
func (m Model) killDrill() (tea.Model, tea.Cmd) {
	f := m.drill
	if f == nil {
		return m, nil
	}
	if !m.hasTmux {
		m.status = "tmux not installed — no session to kill"
		return m, nil
	}
	sessions := liveSessionsFor(f.Slug, m.sessions)
	switch len(sessions) {
	case 0:
		m.status = "no live session to kill for " + f.Slug
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

// startKillForm opens the kill confirm. Defaults to Cancel (confirm=false) so
// Enter is safe — the user must deliberately pick Kill.
func (m *Model) startKillForm(f *contract.Feature, sessions []string) {
	m.pendingKill = sessions
	m.binding = &formBinding{confirm: false}
	noun := "session"
	if len(sessions) != 1 {
		noun = "sessions"
	}
	title := "Kill " + f.Slug + "'s live " + noun + "?"
	desc := "kills " + strings.Join(sessions, ", ") + " (tmux) — the pipeline state is untouched"
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Description(desc).
			Affirmative("Kill").
			Negative("Cancel").
			Value(&m.binding.confirm),
	))
	m.mode = modeForm
}

// startKillPicker opens the FR-3 kill picker (≥2 live sessions): one option per
// session (value = its exact name), an "all N sessions" option (killAll sentinel),
// and a Cancel option (killCancel sentinel). It reuses m.pendingKill — so
// updateForm's StateCompleted already routes completion to finishKill — and binds
// the choice through the heap-stable *formBinding.selected (TEST-001). The single
// session a picker selects is still killed by its EXACT name (never a substring
// sibling — TEST-005), because the option value IS that exact session string.
func (m *Model) startKillPicker(f *contract.Feature, sessions []string) {
	m.pendingKill = sessions
	m.binding = &formBinding{}
	opts := make([]huh.Option[string], 0, len(sessions)+2)
	for _, s := range sessions {
		opts = append(opts, huh.NewOption(s, s))
	}
	opts = append(opts,
		huh.NewOption(fmt.Sprintf("all %d sessions", len(sessions)), killAll),
		huh.NewOption("Cancel", killCancel),
	)
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Kill which session for " + f.Slug + "?").
			Description(fmt.Sprintf("kill one stray session, or all %d (tmux) — the pipeline state is untouched", len(sessions))).
			Options(opts...).
			Value(&m.binding.selected),
	))
	m.mode = modeForm
}

// finishKill runs after a completed kill form OR kill picker. It resolves the kill
// target(s) from binding.selected and kills each via the killer seam (the EXACT
// name — never a substring sibling, TEST-005), then refreshes the drill panel in
// place; a cancelled one just returns to the drill:
//   - selected == ""        → the single-session Confirm path (no picker ran): kill
//     every pendingKill iff binding.confirm, else none.
//   - selected == killAll   → kill every pendingKill session.
//   - selected == killCancel → cancel (no kill).
//   - otherwise             → selected is one exact session name; kill only it.
//
// The picker's Cancel/all use distinct NON-EMPTY sentinels, so the Confirm path
// (selected == "") is never ambiguous — no extra discriminator field needed.
func (m Model) finishKill() (tea.Model, tea.Cmd) {
	sessions := m.pendingKill
	sel := ""
	if m.binding != nil {
		sel = m.binding.selected
	}
	var targets []string
	switch sel {
	case "":
		if m.binding != nil && m.binding.confirm {
			targets = sessions
		}
	case killAll:
		targets = sessions
	case killCancel:
		// cancel — no targets
	default:
		targets = []string{sel} // the one exact session the user picked
	}
	m.pendingKill = nil
	m.binding = nil
	m.form = nil
	m.mode = modeDrill
	if len(targets) == 0 {
		m.status = "cancelled"
		return m, nil
	}
	killed, failed := 0, 0
	for _, s := range targets {
		if err := m.killer(s); err != nil {
			failed++
		} else {
			killed++
		}
	}
	m.sessions = launch.ListSessions()
	if m.drill != nil {
		m.loadDrillCard(m.drill) // refresh live/stale rows after the kill
	}
	m.status = fmt.Sprintf("killed %d %s", killed, plural(killed, "session"))
	if failed > 0 {
		m.status += fmt.Sprintf(", %d failed", failed)
	}
	return m, nil
}

// liveSessionFor returns the running gogo-* tmux session launched for slug, or
// "". It matches the session's sanitized-slug component EXACTLY (launch.
// SessionMatchesSlug), never by substring — so one feature's session is never
// misattributed to another whose slug is a substring of it (TEST-005).
func liveSessionFor(slug string, sessions []string) string {
	for _, s := range sessions {
		if launch.SessionMatchesSlug(s, slug) {
			return s
		}
	}
	return ""
}

// liveSessionsFor returns ALL running gogo-* tmux sessions launched for slug
// (exact SessionMatchesSlug, TEST-005) — the kill targets for the drill `K`.
func liveSessionsFor(slug string, sessions []string) []string {
	var out []string
	for _, s := range sessions {
		if launch.SessionMatchesSlug(s, slug) {
			out = append(out, s)
		}
	}
	return out
}

// plural returns noun or noun+"s" for a count.
func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
