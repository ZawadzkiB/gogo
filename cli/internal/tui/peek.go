package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	tea "github.com/charmbracelet/bubbletea"
)

// peekContentMsg carries a captured session snapshot (or a log tail) back to the
// model. It is NOT cached (a peek is dynamic — `r` always re-captures), so it
// bypasses the width-keyed render cache the artifact viewer uses.
type peekContentMsg struct {
	slug    string
	content string
}

// peekFocused (l on the board — FR7) opens a READ-ONLY viewer over the focused
// card's live session: a tmux capture-pane snapshot when a gogo-* session is
// running, else the background -p log tail, else a hint. Never an attach.
func (m Model) peekFocused() (tea.Model, tea.Cmd) {
	f := m.focusedCard()
	if f == nil {
		m.status = "no card to peek"
		return m, nil
	}
	if session := liveSessionFor(f.Slug, m.sessions); session != "" {
		return m.startPeek(f.Slug, session, "", "peek — "+session)
	}
	if logPath := launch.BackgroundLogFor(m.root, f.Slug); logPath != "" {
		return m.startPeek(f.Slug, "", logPath, "peek — "+filepath.Base(logPath))
	}
	m.status = "no running session or log for " + f.Slug + " — launch it (m / d) first"
	return m, nil
}

// startPeek enters the viewer in peek mode and fires the first capture.
func (m Model) startPeek(slug, session, logPath, title string) (tea.Model, tea.Cmd) {
	m.peeking = true
	m.peekSlug = slug
	m.peekSession = session
	m.peekLog = logPath
	m.viewerTitle = title
	m.mode = modeViewer
	m.viewerLoading = true
	m.viewport.GotoTop()
	return m, tea.Batch(m.capturePeekCmd(), m.spinner.Tick)
}

// capturePeekCmd runs the capture (or log tail) off the UI goroutine — the
// closure captures the peek target so it is pure and safe to run concurrently.
func (m Model) capturePeekCmd() tea.Cmd {
	slug, session, logPath, capture := m.peekSlug, m.peekSession, m.peekLog, m.capturer
	return func() tea.Msg {
		return peekContentMsg{slug: slug, content: capturePeek(session, logPath, capture)}
	}
}

// capturePeek resolves the peek content: a live tmux pane snapshot, else the
// tail of a background log. Every failure degrades to a readable line — never a
// panic (the viewer just shows the message).
func capturePeek(session, logPath string, capture func(string, int) (string, error)) string {
	if session != "" {
		out, err := capture(session, launch.PeekLines)
		if err != nil {
			return "capture failed: " + err.Error()
		}
		if strings.TrimSpace(out) == "" {
			return "(session pane is empty)"
		}
		return out
	}
	if logPath != "" {
		raw, err := os.ReadFile(logPath)
		if err != nil {
			return "cannot read log: " + err.Error()
		}
		tail := tailLines(string(raw), launch.PeekLines)
		if strings.TrimSpace(tail) == "" {
			return "(log is empty)"
		}
		return tail
	}
	return "no session or log to peek"
}

// closePeek clears peek state and returns to the board (peek is launched from
// the board, so q/esc land back there — not on the drill file list).
func (m Model) closePeek() Model {
	m.peeking = false
	m.peekSession = ""
	m.peekLog = ""
	m.peekSlug = ""
	m.mode = modeBoard
	return m
}

// attachFromPeek switches from a read-only peek to a full attach (a, when the
// peek is over a live tmux session).
func (m Model) attachFromPeek() (tea.Model, tea.Cmd) {
	if m.peekSession == "" {
		m.status = "background log peek — no live tmux session to attach"
		return m, nil
	}
	session := m.peekSession
	m = m.closePeek()
	c := exec.Command("tmux", launch.AttachArgs(session)...)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return launchDoneMsg{status: "detached from " + session}
	})
}

// tailLines returns the last n lines of s — the background-log analogue of tmux
// capture-pane's `-S -<n>` scrollback window.
func tailLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
