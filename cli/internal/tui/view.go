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
	left := colTitleStyle.Render("gogo cockpit") + "  " + dimStyle.Render(fmt.Sprintf("%d features", total))
	if m.filter != "" {
		left += dimStyle.Render("  /" + m.filter)
	}
	header := placeApart(left, m.attentionSummary(), m.boardBodyWidth())

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

	// Header · columns · status · contextual footer. The needs-you strip is gone —
	// the header count (⏸ K need you) plus each gate card's left-border stripe carry
	// the "act now" signal now.
	parts := []string{header, body, statusStyle(status), m.contextualFooter()}
	return strings.Join(parts, "\n")
}

// placeApart lays a left and a right segment on one line `width` wide, padding
// the gap between them — the FR-1 header (identity | attention summary) and the
// FR-7 footer (card keys | [?] all keys) split.
func placeApart(left, right string, width int) string {
	if right == "" {
		return left
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// attentionSummary is the FR-1 right-aligned header cue: a red `⏸ K need you`
// pill (only when K>0), then a green `● S session` count (only when S>0).
func (m Model) attentionSummary() string {
	k := m.needsYouCount()
	s := len(m.sessions)
	var out string
	if k > 0 {
		out = pillRed.Render(fmt.Sprintf("%s %d need you", waitingMarker, k))
	}
	if s > 0 {
		if out != "" {
			out += "  "
		}
		out += sessionStyle.Render(fmt.Sprintf("● %d session", s))
	}
	return out
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
	if isChangelogCol(i) {
		return m.renderChangelogColumn(i, colWidth) // FR-6: a collapsed list, not cards
	}
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

// renderChangelogColumn renders the collapsed changelog (FR-6): plain
// `✓ slug … MM-DD` rows (no card boxes), windowed like the work columns (unit
// row heights — see cardHeights), overflow shown as `↓ N more · enter to browse`.
func (m Model) renderChangelogColumn(i, colWidth int) string {
	col := m.cols[i]
	if len(col) == 0 {
		parts := []string{m.columnHeader(i, ""), "", dimStyle.Render("(none)")}
		return lipgloss.NewStyle().Width(colWidth).Render(strings.Join(parts, "\n"))
	}
	rowW := colWidth - 2
	if rowW < 12 {
		rowW = 12
	}
	heights := m.cardHeights(i, rowW) // 1 per row for the changelog (cardHeights special-cases it)

	start, end := 0, len(col)
	if m.height > 0 {
		avail := m.colAvail()
		if avail < 1 {
			avail = 1
		}
		start = clamp(m.colOffset[i], 0, len(col)-1)
		end = fitEnd(heights, start, avail)
	}

	hint := ""
	if start > 0 || end < len(col) {
		hint = fmt.Sprintf("%d–%d", start+1, end)
	}
	// The focused row (only when the changelog column itself holds board focus) gets
	// the ▸ cursor + selection bar, so navigating the list has an in-list indicator,
	// not just the footer text.
	focusedRow := -1
	if i == m.colIdx {
		focusedRow = m.cardIdx[i]
	}
	parts := []string{m.columnHeader(i, hint), ""}
	if start > 0 {
		parts = append(parts, faintStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
	}
	for j := start; j < end; j++ {
		parts = append(parts, changelogRow(col[j], rowW, j == focusedRow, hasLiveSession(col[j].Slug, m.sessions)))
	}
	if below := len(col) - end; below > 0 {
		parts = append(parts, faintStyle.Render(fmt.Sprintf("  ↓ %d more · enter to browse", below)))
	}
	return lipgloss.NewStyle().Width(colWidth).Render(strings.Join(parts, "\n"))
}

// changelogRow is one collapsed changelog entry (FR-6): a two-column cursor gutter
// (`▸ ` when focused, else blank), `✓ slug` (truncated) left, `MM-DD` (faint) right
// — no box. When the shipped item has a live pipeline session (hasSession, FR-1) a
// green `●` is prefixed just before the slug (`✓ ● slug`), so the user can spot it
// and enter→drill→kill. A focused row renders as a full-width selection bar: plain
// inner text under a single focus fg+bg fill so the bar has no per-segment
// background holes (the same tactic the focused work card uses — the dot rides the
// fill there too, so on a focused row it is plain, not green).
func changelogRow(f *contract.Feature, width int, focused, hasSession bool) string {
	date := shortDate(f.Completed)
	if date == "" {
		date = shortDate(f.Created)
	}
	dateW := len([]rune(date))
	// Reserve the leading "● " (2 cells) when live, so the slug truncation and the
	// right-aligned date math both account for it and MM-DD stays put.
	dot := ""
	if hasSession {
		dot = "● "
	}
	slugMax := width - dateW - 5 - len([]rune(dot)) // "▸ " gutter + "✓ " + [dot] + a one-cell gap
	if slugMax < 4 {
		slugMax = 4
	}
	slug := truncate(f.Slug, slugMax)
	leftPlain := "✓ " + dot + slug                       // plain form for the width/gap math
	gap := width - 2 - lipgloss.Width(leftPlain) - dateW // -2 for the cursor gutter
	if gap < 1 {
		gap = 1
	}
	if focused {
		row := "▸ " + leftPlain + strings.Repeat(" ", gap) + date
		return changelogFocusStyle.Width(width).Render(row)
	}
	// Non-focused: the ● gets its own green sessionStyle; ✓/slug stay secondary.
	styledLeft := secondaryStyle.Render("✓ ")
	if hasSession {
		styledLeft += sessionStyle.Render("●") + " "
	}
	styledLeft += secondaryStyle.Render(slug)
	return "  " + styledLeft + strings.Repeat(" ", gap) + faintStyle.Render(date)
}

// shortDate reduces a YYYY-MM-DD date to MM-DD (the changelog's compact form);
// anything else passes through unchanged.
func shortDate(s string) string {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[5:10]
	}
	return s
}

// columnHeader renders a column's title + count (FR-2: an accent, underlined
// title + a trailing dim count — no `(N)` parentheses), a ▸ focus marker, and an
// optional scroll-position hint (only when the column overflows — no noise on
// short columns). The changelog count reads `N shipped` (FR-6).
func (m Model) columnHeader(i int, hint string) string {
	st := columnStyles[i]
	n := len(m.cols[i])
	countTxt := fmt.Sprintf("%d", n)
	if isChangelogCol(i) {
		countTxt = fmt.Sprintf("%d shipped", n)
	}
	marker := "  "
	if i == m.colIdx {
		marker = "▸ "
	}
	head := marker + st.header.Underline(true).Render(columnTitles[i]) + dimStyle.Render(" "+countTxt)
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

	title := f.Title
	if title == "" {
		title = f.Slug
	}

	// The live agent chip (FR-6, D1): a green `● <agent>` shown only when a session
	// is live on the card AND the card is not itself a user gate. An idle card, or a
	// gate card, shows the status pill alone.
	agent := ""
	if hasSession && !f.WaitingForInput() {
		agent = activeAgent(f)
	}

	// Three rows: name (+ mark, + live ● dot) · one-line desc · status pill [+ agent chip].
	var head, titleLine, badgeLine string
	if focused {
		// Plain inner text — the frame carries one foreground + background so the
		// highlight fills cleanly (no per-segment holes, incl. the pill's tint).
		mark := ""
		if selected {
			mark = "✓ "
		} else if f.Class == contract.ClassReadyToShip {
			mark = "○ "
		}
		dot := ""
		if hasSession {
			dot = " ●"
		}
		head = mark + truncate(f.Slug, width-len([]rune(mark))-len([]rune(dot))) + dot
		titleLine = truncate(title, width)
		// The frame carries one fg+bg fill, so a colored chip would punch a hole —
		// render it plain and reserve its rune width from the pill's truncation budget
		// (the pill gets the full width when there is no chip).
		if agent != "" {
			chip := "  ● " + agent
			badgeLine = truncate(pillLabel(f), width-len([]rune(chip))) + chip
		} else {
			badgeLine = truncate(pillLabel(f), width)
		}
	} else {
		mark := ""
		if selected {
			mark = selMarkStyle.Render("✓ ")
		} else if f.Class == contract.ClassReadyToShip {
			mark = dimStyle.Render("○ ")
		}
		dot := ""
		if hasSession {
			dot = " " + sessionStyle.Render("●")
		}
		head = mark + slugStyle.Render(truncate(f.Slug, width-4)) + dot
		titleLine = dimStyle.Render(truncate(title, width))
		badgeLine = pillStyleFor(f).Render(pillLabel(f))
		if agent != "" {
			badgeLine += "  " + sessionStyle.Render("● "+agent)
		}
	}

	body := strings.Join([]string{head, titleLine, badgeLine}, "\n")

	style := columnStyles[colIdx].card
	switch {
	case focused:
		style = columnStyles[colIdx].cardFocused
	case selected:
		style = columnStyles[colIdx].cardSelected
	}
	// FR-5 left accent stripe: recolor the heavy-┃ gate border, independent of
	// focus (a focused gate card keeps both the focus accent and the stripe).
	if col, ok := stripeAccent(f); ok {
		style = style.Border(gateBorder).BorderLeftForeground(col)
	}
	return style.Width(width).Render(body)
}

// contextualFooter is the FR-7 footer: the focused card's applicable action
// key-chips (a live card leading with a green ●) + a right-aligned `[?] all
// keys`. `?` (FR-10) swaps it for the full pre-redesign key list.
func (m Model) contextualFooter() string {
	if m.showAllKeys {
		full := "←→/h cols · ↑↓/jk cards · space select · enter drill · v view · w web · m move · d ship · a attach · l peek · x del · / filter · ? keys · q quit"
		return helpStyle.Render(full)
	}
	right := keyChipStyle.Render("[?] all keys")
	f := m.focusedCard()
	if f == nil {
		return placeApart(dimStyle.Render("no card focused"), right, m.boardBodyWidth())
	}
	lead := slugStyle.Render(f.Slug)
	if hasLiveSession(f.Slug, m.sessions) {
		lead = sessionStyle.Render("● ") + lead
	}
	left := lead + "  " + strings.Join(m.footerChips(f), " ")
	return placeApart(left, right, m.boardBodyWidth())
}

// footerChips are the action key-chips applicable to the focused card — the
// column's legal move first (accept / go / ship), then the always-available
// drill/view/web and, for a live card, peek/attach.
func (m Model) footerChips(f *contract.Feature) []string {
	chip := keyChipStyle.Render
	var c []string
	switch f.Class {
	case contract.ClassUnfinished:
		if f.Status == "awaiting-plan-acceptance" {
			c = append(c, chip("[m] accept"))
		} else {
			c = append(c, chip("[m] go"))
		}
	case contract.ClassInProgress:
		c = append(c, chip("[m] go"))
	case contract.ClassReadyToShip:
		c = append(c, chip("[d] ship"))
	}
	c = append(c, chip("[enter] drill"), chip("[v] view"))
	if hasLiveSession(f.Slug, m.sessions) {
		c = append(c, chip("[l] peek"), chip("[a] attach"))
	}
	c = append(c, chip("[w] web"))
	return c
}

// boardBodyWidth is the total rendered width of the 4 columns + their 3 one-cell
// separators — the width the header / footer / needs-you strip lay out across.
func (m Model) boardBodyWidth() int {
	return 4*m.boardColWidth() + 3
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
