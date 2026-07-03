package tui

import (
	"fmt"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/charmbracelet/lipgloss"
)

const boardWidth = 268

// View renders the current mode.
func (m Model) View() string {
	switch m.mode {
	case modeDrill:
		return m.viewDrill()
	case modeViewer:
		return m.viewViewer()
	case modeForm:
		if m.form != nil {
			return "\n" + m.form.View() + "\n"
		}
		return ""
	default:
		return m.viewBoard()
	}
}

func (m Model) viewBoard() string {
	total := len(m.repo.Features)
	header := colTitleStyle.Render(fmt.Sprintf("gogo cockpit — %d features", total))
	if m.filter != "" {
		header += dimStyle.Render("  /" + m.filter)
	}

	colWidth := m.boardColWidth()

	var rendered []string
	for i := 0; i < 4; i++ {
		rendered = append(rendered, m.renderColumn(i, colWidth))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)

	status := m.status
	if status == "" {
		status = m.boardStatusLine()
	}
	help := "←→ cols · ↑↓ cards · space select · enter drill · v view · w web · m move · d ship · a attach · / filter · q quit"
	return strings.Join([]string{header, body, statusStyle(status), helpStyle.Render(help)}, "\n")
}

// boardStatusLine surfaces the attach hint for the focused card when it has a
// live session (TEST-006), else the running-sessions summary.
func (m Model) boardStatusLine() string {
	if f := m.focusedCard(); f != nil && hasLiveSession(f.Slug, m.sessions) {
		return sessionStyle.Render("● "+f.Slug) + " has a live session — press " +
			slugStyle.Render("a") + " to attach"
	}
	return m.sessionsLine()
}

func (m Model) renderColumn(i, colWidth int) string {
	col := m.cols[i]

	if len(col) == 0 {
		parts := []string{m.columnHeader(i, ""), "", dimStyle.Render("(none)")}
		return lipgloss.NewStyle().Width(colWidth).Render(strings.Join(parts, "\n"))
	}

	// Card frame width: colWidth minus the rounded border (2) + padding (2).
	cardW := colWidth - 4
	if cardW < 14 {
		cardW = 14
	}

	// Render + measure every card, then window to only what fits (TEST-014).
	cards := make([]string, len(col))
	heights := make([]int, len(col))
	for j, f := range col {
		focused := i == m.colIdx && j == m.cardIdx[i]
		cards[j] = m.renderCard(i, f, focused, cardW)
		heights[j] = lipgloss.Height(cards[j])
	}

	start, end := 0, len(col)
	if m.height > 0 {
		avail := m.colAvail()
		if avail < 1 {
			avail = 1
		}
		start = clamp(m.colOffset[i], 0, len(col)-1)
		end = fitEnd(heights, start, avail)
	}

	// A position hint (e.g. "3–5") in the header, only when the column overflows.
	hint := ""
	if start > 0 || end < len(col) {
		hint = fmt.Sprintf("%d–%d", start+1, end)
	}

	parts := []string{m.columnHeader(i, hint), ""}
	if above := start; above > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("  ↑ %d more", above)))
	}
	parts = append(parts, cards[start:end]...)
	if below := len(col) - end; below > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
	}
	return lipgloss.NewStyle().Width(colWidth).Render(strings.Join(parts, "\n"))
}

// columnHeader renders a column's title + count, the ▸ focus marker, and an
// optional scroll-position hint (shown only when the column overflows — no noise
// on short columns). The hint tells the user where the visible window sits in
// the full list (TEST-014 position indicator).
func (m Model) columnHeader(i int, hint string) string {
	st := columnStyles[i]
	n := len(m.cols[i])
	var head string
	if i == m.colIdx {
		head = st.header.Render("▸ "+columnTitles[i]) + st.header.Render(fmt.Sprintf(" (%d)", n))
	} else {
		head = "  " + st.header.Render(fmt.Sprintf("%s (%d)", columnTitles[i], n))
	}
	if hint != "" {
		head += dimStyle.Render(" · " + hint)
	}
	return head
}

// renderCard draws one feature as a bordered card. The focused card gets a
// full-card highlight (accent border + subtle background, TEST-007); a
// selected-for-ship card gets the select accent border + a ✓; a card with a
// live tmux session shows an unmissable green ● session marker (TEST-006).
func (m Model) renderCard(colIdx int, f *contract.Feature, focused bool, width int) string {
	selected := f.Class == contract.ClassReadyToShip && m.selected[f.Slug]
	hasSession := hasLiveSession(f.Slug, m.sessions)

	slug := truncate(f.Slug, width)
	title := f.Title
	if title == "" {
		title = f.Slug
	}
	b := badge(f, m.sessions)

	var head, titleLine, badgeLine string
	if focused {
		// Plain inner text — the frame carries one foreground + background so the
		// highlight fills cleanly (no per-segment background holes).
		mark := ""
		if selected {
			mark = "✓ "
		} else if f.Class == contract.ClassReadyToShip {
			mark = "○ "
		}
		dot := ""
		if hasSession {
			dot = "  ● session"
		}
		head = mark + truncate(slug, width-len([]rune(mark))-len([]rune(dot))) + dot
		titleLine = truncate(title, width)
		badgeLine = truncate(b, width)
	} else {
		mark := ""
		if selected {
			mark = selMarkStyle.Render("✓ ")
		} else if f.Class == contract.ClassReadyToShip {
			mark = dimStyle.Render("○ ")
		}
		dot := ""
		if hasSession {
			dot = "  " + sessionStyle.Render("● session")
		}
		head = mark + slugStyle.Render(truncate(slug, width-4)) + dot
		titleLine = dimStyle.Render(truncate(title, width))
		bs := columnStyles[colIdx].badge
		if f.WaitingForUser() {
			bs = waitStyle
		}
		badgeLine = bs.Render(truncate(b, width))
	}

	body := strings.Join([]string{head, titleLine, badgeLine}, "\n")
	switch {
	case focused:
		return columnStyles[colIdx].cardFocused.Width(width).Render(body)
	case selected:
		return columnStyles[colIdx].cardSelected.Width(width).Render(body)
	default:
		return columnStyles[colIdx].card.Width(width).Render(body)
	}
}

func (m Model) sessionsLine() string {
	if len(m.sessions) == 0 {
		hints := []string{}
		if !m.hasTmux {
			hints = append(hints, "tmux: no (background -p fallback)")
		}
		if !m.hasClaude {
			hints = append(hints, "claude: not found")
		}
		if len(hints) == 0 {
			return "no running sessions"
		}
		return strings.Join(hints, " · ")
	}
	return "sessions: " + strings.Join(m.sessions, " · ")
}

func (m Model) viewDrill() string {
	title := colTitleStyle.Render("files — " + m.drill.Slug)
	var lines []string
	for i, a := range m.artifacts {
		cursor := "  "
		if i == m.artIdx {
			cursor = "▸ "
		}
		lines = append(lines, cursor+a.Label)
	}
	if len(lines) == 0 {
		lines = append(lines, "  (no files)")
	}
	help := lipgloss.NewStyle().Faint(true).Render("↑↓ files · enter open · G glow · w web · esc back")
	return strings.Join([]string{title, "", strings.Join(lines, "\n"), "", help}, "\n")
}

func (m Model) viewViewer() string {
	title := colTitleStyle.Render(m.viewerTitle)
	help := helpStyle.Render("↑↓/jk line · space/b page · d/u ½page · g/G top/bottom · w web · esc back · (glow: G from the file list)")
	if m.viewerLoading {
		body := m.spinner.View() + " rendering " + m.viewerTitle + "…"
		return strings.Join([]string{title, "", dimStyle.Render(body), help}, "\n")
	}
	return strings.Join([]string{title, m.viewport.View(), help}, "\n")
}

func statusStyle(s string) string {
	return lipgloss.NewStyle().Faint(true).Render(s)
}

func truncate(s string, max int) string {
	if max < 4 {
		max = 4
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
