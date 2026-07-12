package tui

import (
	"fmt"
	"path/filepath"
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
	body := lipgloss.JoinHorizontal(lipgloss.Top, interleaveSeparators(rendered)...)

	status := m.status
	if status == "" {
		status = m.boardStatusLine()
	}
	help := "←→/h cols · ↑↓/jk cards · space select · enter drill · v view · w web · m move · d ship · a attach · l peek · x del · / filter · q quit"
	return strings.Join([]string{header, body, statusStyle(status), helpStyle.Render(help)}, "\n")
}

// interleaveSeparators inserts a full-height vertical rule between the rendered
// columns so it is clear where each card sits (FR-B4). The separator is sized to
// the tallest column so it spans the whole board body; its one-cell width per
// gutter is already reserved out of the width budget (boardColWidth).
func interleaveSeparators(cols []string) []string {
	h := 0
	for _, c := range cols {
		if ch := lipgloss.Height(c); ch > h {
			h = ch
		}
	}
	sep := columnSeparator(h)
	out := make([]string, 0, len(cols)*2-1)
	for i, c := range cols {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, c)
	}
	return out
}

// columnSeparator builds a styled one-cell vertical rule `height` rows tall.
func columnSeparator(height int) string {
	if height < 1 {
		height = 1
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = "│"
	}
	return sepStyle.Render(strings.Join(lines, "\n"))
}

// boardStatusLine surfaces the attach hint for the focused card when it has a
// live session (TEST-006), else the running-sessions summary.
func (m Model) boardStatusLine() string {
	if f := m.focusedCard(); f != nil && hasLiveSession(f.Slug, m.sessions) {
		return sessionStyle.Render("● "+f.Slug) + " has a live session — " +
			slugStyle.Render("l") + " peek · " + slugStyle.Render("a") + " attach"
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
		badgeLine = truncate(cardBadgeText(f, b), width)
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
		bs := badgeStyleFor(f, columnStyles[colIdx].badge)
		badgeLine = bs.Render(truncate(cardBadgeText(f, b), width))
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

// cardBadgeText prepends the waiting cue to a card's badge when it is
// WaitingForInput() — the leading ⏸ marks a card that blocks on the user, shown
// on both focused and unfocused cards so the signal never depends on focus (FR-B2).
func cardBadgeText(f *contract.Feature, b string) string {
	if f.WaitingForInput() {
		return waitingMarker + " " + b
	}
	return b
}

// badgeStyleFor picks a waiting card's accent: uat purple for the UAT gate, the
// wait red for a decision gate AND the plan-acceptance gate (which carried no
// accent before — FR-B2); a flowing card keeps its column accent (base).
func badgeStyleFor(f *contract.Feature, base lipgloss.Style) lipgloss.Style {
	switch {
	case f.AwaitingUAT():
		return uatStyle
	case f.WaitingForInput():
		return waitStyle
	}
	return base
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

// viewDrill renders the rich card detail panel (Slice B — FR-B1/B2/B4) above the
// feature's file list: description / folder / status, the card's session rows
// (registry ⨯ live-tmux), a compact recent-events tail, then the openable files.
func (m Model) viewDrill() string {
	f := m.drill
	var b []string
	b = append(b, colTitleStyle.Render("card — "+f.Slug), "")

	// Detail (FR-B1): description / folder / status (enriched with phase + round).
	desc := f.Title
	if desc == "" {
		desc = "(no description)"
	}
	statusLine := f.Status
	if f.Phase != "" {
		statusLine += " · " + f.Phase
	}
	if r := f.RoundFor(f.Phase); r > 0 {
		statusLine += fmt.Sprintf(" r%d", r)
	}
	b = append(b,
		dimStyle.Render("description  ")+desc,
		dimStyle.Render("folder       ")+filepath.Base(f.Dir)+"/",
		dimStyle.Render("status       ")+statusLine,
	)

	// Sessions (FR-B2): tracked legs (live/stale) + untracked-live racers.
	b = append(b, "", colTitleStyle.Render("sessions"))
	if len(m.drillSessions) == 0 {
		b = append(b, dimStyle.Render("  no tracked sessions"))
	} else {
		for _, r := range m.drillSessions {
			b = append(b, "  "+renderSessionRow(r))
		}
	}

	// Recent events (FR-B4): the compact tail; the full timeline stays openable
	// via the events row in the file list below.
	b = append(b, "", colTitleStyle.Render("recent events"))
	if m.drillEventsTail == "" {
		b = append(b, dimStyle.Render("  no events recorded"))
	} else {
		for _, ln := range strings.Split(m.drillEventsTail, "\n") {
			b = append(b, "  "+dimStyle.Render(ln))
		}
	}

	// Files (existing openable list).
	b = append(b, "", colTitleStyle.Render("files"))
	if len(m.artifacts) == 0 {
		b = append(b, "  (no files)")
	} else {
		for i, a := range m.artifacts {
			cursor := "  "
			if i == m.artIdx {
				cursor = "▸ "
			}
			b = append(b, cursor+a.Label)
		}
	}

	// Surface the transient status line here too (TEST-001): a/K hints ("no running
	// session", "no live session to kill") and the kill-cancelled/succeeded/detach
	// confirmations set m.status, but viewDrill — unlike viewBoard — never rendered
	// it, so those actions looked like silent no-ops in the live TUI.
	if m.status != "" {
		b = append(b, "", statusStyle(m.status))
	}
	help := lipgloss.NewStyle().Faint(true).Render("↑↓ files · enter open · a attach · K kill · G glow · w web · esc back")
	b = append(b, "", help)
	return strings.Join(b, "\n")
}

// renderSessionRow formats one session panel line (FR-B2): a live/stale dot, the
// leg kind (or "untracked"), the live/stale flag, the registry lifecycle status,
// the tmux session name (the kill/attach target), and per-leg cost/turns when
// recorded. Styled via lipgloss — plain text under `go test` (no TTY), so the
// panel stays substring-assertable.
func renderSessionRow(r sessionRow) string {
	dot, live := dimStyle.Render("○"), dimStyle.Render("stale")
	if r.Live {
		dot, live = sessionStyle.Render("●"), sessionStyle.Render("live")
	}
	kind := r.Kind
	if !r.Tracked {
		kind = "untracked"
	}
	parts := []string{dot, fmt.Sprintf("%-9s", kind), live}
	if r.Status != "" {
		parts = append(parts, r.Status)
	}
	if r.Session != "" {
		parts = append(parts, slugStyle.Render(r.Session))
	} else {
		parts = append(parts, dimStyle.Render("(headless)"))
	}
	if r.NumTurns > 0 || r.CostUSD > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("$%.2f · %d turns", r.CostUSD, r.NumTurns)))
	}
	return strings.Join(parts, "  ")
}

func (m Model) viewViewer() string {
	title := colTitleStyle.Render(m.viewerTitle)
	help := helpStyle.Render("↑↓/jk line · space/b page · d/u ½page · g/G top/bottom · w web · esc back · (glow: G from the file list)")
	verb := "rendering"
	if m.peeking {
		// Read-only session-log peek (FR7): r re-captures, a escalates to attach.
		help = helpStyle.Render("↑↓/jk scroll · r re-capture · a attach · q back  (read-only log peek)")
		verb = "capturing"
	}
	if m.viewerLoading {
		body := m.spinner.View() + " " + verb + " " + m.viewerTitle + "…"
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
