package tui

import (
	"os/exec"
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
			m.watch.reconcile(watchPaths(m.root, m.repo))
		}
		return m, waitForReload(m.reloadCh)

	case launchDoneMsg:
		m.status = msg.status
		m.sessions = launch.ListSessions()
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
			return m.updateBoard(msg)
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

func (m Model) updateBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.updateFilter(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, m.quit()
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
		return m.cancelForm(m.pendingDelete != nil), nil
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
		if m.binding == nil || !m.binding.confirm {
			// Completed on "Cancel" — same as an abort. Only a SHIP form reaches
			// here (a delete form is handled above via finishDelete), so
			// pendingDelete is nil and the selection is (correctly) cleared.
			return m.cancelForm(m.pendingDelete != nil), nil
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
		return m.cancelForm(m.pendingDelete != nil), nil
	}
	return m, cmd
}

// cancelForm returns to the board and clears the in-flight form state. For a SHIP
// form (preserveSelection=false) it ALSO drops the pending intent AND the
// ready-card selection, so a stale, unconfirmed launch target can never be
// silently re-shipped by a later, unrelated m/d press (TEST-002). For a DELETE
// form (preserveSelection=true) the ready-ship selection is unrelated to the
// delete, so it survives — an Esc-abort of a delete now matches the Cancel-button
// (finishDelete) path instead of wiping the user's multi-selection (REV-012).
func (m Model) cancelForm(preserveSelection bool) Model {
	m.mode = modeBoard
	m.status = "cancelled"
	if !preserveSelection {
		m.selected = map[string]bool{}
		m.pending = launch.Intent{}
		m.pendingShip = false
	}
	m.pendingDelete = nil
	m.binding = nil
	m.form = nil
	return m
}

// quit closes the fsnotify watcher (stopping its reload goroutine — REV-011)
// before asking Bubble Tea to exit, so nothing leaks past the board session.
func (m Model) quit() tea.Cmd {
	_ = m.watch.close() // nil-safe when the watcher never started
	return tea.Quit
}

// attachFocused suspends the TUI and attaches to the focused card's tmux
// session (tea.ExecProcess), when one exists.
func (m Model) attachFocused() (tea.Model, tea.Cmd) {
	f := m.focusedCard()
	if f == nil {
		return m, nil
	}
	if !m.hasTmux {
		m.status = "tmux not installed — nothing to attach"
		return m, nil
	}
	session := liveSessionFor(f.Slug, m.sessions)
	if session == "" {
		m.status = "no running session for " + f.Slug
		return m, nil
	}
	c := exec.Command("tmux", launch.AttachArgs(session)...)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return launchDoneMsg{status: "detached from " + session}
	})
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
