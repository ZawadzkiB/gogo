package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/charmbracelet/lipgloss"
)

const boardWidth = 268

// View renders the current mode. On a project board (tabbed) the top-level render
// dispatches by the active tab; a lone repo has no tabs and renders today's single-
// repo board byte-for-byte (FR7).
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
		if m.global() {
			switch m.tab {
			case tabPlans:
				return m.viewTabBar() + "\n\n" + m.viewPlans()
			case tabConfig:
				return m.viewTabBar() + "\n\n" + m.viewConfig()
			}
		}
		return m.viewBoard()
	}
}

// viewTabBar renders the FR8 tab bar `board · plans · config` with the active tab
// highlighted. Project board only (the caller renders it just for m.global()).
func (m Model) viewTabBar() string {
	parts := make([]string, len(tabTitles))
	for i, t := range tabTitles {
		if tabID(i) == m.tab {
			parts[i] = tabActiveStyle.Render(t)
		} else {
			parts[i] = tabStyle.Render(t)
		}
	}
	return strings.Join(parts, dimStyle.Render("  ·  "))
}

// viewSourceChips renders the FR7 board source filter chips: `all` + one per source
// of the focused project, the active chip highlighted. "" (no chips) for a lone repo
// or a single-source project — so the single-repo board stays byte-for-byte.
func (m Model) viewSourceChips() string {
	chips := m.sourceChips()
	if len(chips) <= 1 {
		return ""
	}
	parts := make([]string, 0, len(chips))
	for _, c := range chips {
		label := c
		prefix := ""
		if c == "" {
			label = "all"
		} else {
			// Each source chip carries its colored origin dot (design 3a). The chip style's
			// Padding(0,1) supplies the space between the dot and the label.
			prefix = m.sourceDot(c)
		}
		if c == m.sourceChip {
			parts = append(parts, prefix+chipActiveStyle.Render(label))
		} else {
			parts = append(parts, prefix+chipStyle.Render(label))
		}
	}
	return dimStyle.Render("sources ") + strings.Join(parts, " ")
}

func (m Model) viewBoard() string {
	total := len(m.repo.Features)
	left := colTitleStyle.Render("gogo cockpit") + "  " + dimStyle.Render(fmt.Sprintf("%d features", total))
	// Project board only: a `M projects` note beside the feature count (FR7). Invisible
	// in single-repo mode, so that header is byte-for-byte unchanged.
	if m.global() {
		n := len(m.allProjects)
		left += dimStyle.Render(fmt.Sprintf("  ·  %d %s", n, plural(n, "project")))
	}
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
	// the "act now" signal now. A project board prepends the tab bar + the source
	// chips (both invisible on a lone repo → single-repo parity).
	var parts []string
	if m.global() {
		parts = append(parts, m.viewTabBar())
		parts = append(parts, header)
		if chips := m.viewSourceChips(); chips != "" {
			parts = append(parts, chips)
		}
	} else {
		parts = append(parts, header)
	}
	parts = append(parts, body, statusStyle(status), m.contextualFooter())
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
		parts = append(parts, m.changelogRow(col[j], rowW, j == focusedRow, hasLiveSession(col[j].Slug, m.sessions)))
	}
	if below := len(col) - end; below > 0 {
		parts = append(parts, faintStyle.Render(fmt.Sprintf("  ↓ %d more · enter to browse", below)))
	}
	return lipgloss.NewStyle().Width(colWidth).Render(strings.Join(parts, "\n"))
}

// changelogRow is one collapsed changelog entry (FR-6): a two-column cursor gutter
// (`▸ ` when focused, else blank), `✓ slug` (truncated) left, `MM-DD` (faint) right
// — no box.
//
// D3=A (cockpit-colors): on a PROJECT board (the row has a Source) the row LEADS with a
// SOURCE-colored origin dot (`● ✓ slug`), the design's fast origin cue, and the
// live-session cue moves to a TRAILING green `●` just before the date (`… ● MM-DD`), so
// origin reads left and liveness right. A SINGLE-REPO row (no source) keeps today's
// leading session dot — byte-for-byte unchanged (changelogRowSingle).
//
// A focused row renders as a full-width selection bar: plain inner text under a single
// focus fg+bg fill so the bar has no per-segment background holes.
func (m Model) changelogRow(f *contract.Feature, width int, focused, hasSession bool) string {
	date := shortDate(f.Completed)
	if date == "" {
		date = shortDate(f.Created)
	}
	dateW := len([]rune(date))
	if f.Source == "" {
		return changelogRowSingle(f, width, focused, hasSession, date, dateW)
	}

	// Project board: leading source dot + ✓ + slug, trailing green session dot + date.
	trailing := ""
	if hasSession {
		trailing = "● " // relocated live-session cue (D3=A)
	}
	// Reserve: cursor(2) + source-dot "● "(2) + "✓ "(2) + trailing + a 1-cell gap + date.
	slugMax := width - 2 - 2 - 2 - len([]rune(trailing)) - 1 - dateW
	if slugMax < 4 {
		slugMax = 4
	}
	slug := truncate(f.Slug, slugMax)
	leftPlain := "● " + "✓ " + slug
	rightPlain := trailing + date
	gap := width - 2 - lipgloss.Width(leftPlain) - lipgloss.Width(rightPlain) // -2 for the cursor gutter
	if gap < 1 {
		gap = 1
	}
	if focused {
		row := "▸ " + leftPlain + strings.Repeat(" ", gap) + rightPlain
		return changelogFocusStyle.Width(width).Render(row)
	}
	styledLeft := m.sourceDot(f.Source) + " " + secondaryStyle.Render("✓ ") + secondaryStyle.Render(slug)
	styledRight := ""
	if hasSession {
		styledRight = sessionStyle.Render("●") + " "
	}
	styledRight += faintStyle.Render(date)
	return "  " + styledLeft + strings.Repeat(" ", gap) + styledRight
}

// changelogRowSingle is the SINGLE-REPO changelog row (no source label) — byte-for-byte
// today's behaviour: `✓ slug` with the live-session green `●` prefixed just before the
// slug (`✓ ● slug`). Kept identical so a lone repo's board never gains a source dot.
func changelogRowSingle(f *contract.Feature, width int, focused, hasSession bool, date string, dateW int) string {
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

// sourceTag renders the compact per-card SOURCE tag shown ONLY on the project board
// (FR7): a colored dot + the source label, tinted with the source's configured color
// when one is set, else dim. It returns ("","") whenever f.Source == "" — i.e. always
// in single-repo mode — so the single-repo card is byte-for-byte unchanged. `plain`
// is the untinted form (used by the focused card, whose single fg/bg fill would punch
// a hole through a colored chip) and carries the rune width for the title truncation
// math.
func (m Model) sourceTag(f *contract.Feature) (styled, plain string) {
	if f.Source == "" {
		return "", ""
	}
	plain = "● " + f.Source
	// The source palette is never-blank (cockpit-colors FR2), so the tag always
	// carries its source's color — never the old grey "no color" fallback.
	return lipgloss.NewStyle().Foreground(m.sourceColor(f.Source)).Render(plain), plain
}

// fitSourceTag truncates a card's right-aligned source tag (styled+plain) so the
// composed NAME row — `mark + slug + dot`, a forced 1-col gap, then the tag — can never
// exceed the card text width and wrap (REV-006). It only kicks in when the slug budget
// would be floored (slugBudget < minSlug, i.e. the tag is so wide the slug can't get its
// minimum); otherwise the tag is returned untouched. A budget too small for even a
// 3-rune tag drops it entirely (the slug takes the whole row). A lone-repo card (empty
// tag) is a no-op — byte-for-byte unchanged. markW/dotW are the display widths of the
// (styled or plain) mark/dot prefixes already computed by the caller.
func (m Model) fitSourceTag(f *contract.Feature, styled, plain string, textW, markW, dotW int) (fStyled, fPlain string) {
	if plain == "" {
		return "", ""
	}
	const minSlug = 4 // matches truncate()'s floor — the widest a floored slug can be
	slugBudget := textW - markW - dotW - lipgloss.Width(plain) - 1
	if slugBudget >= minSlug {
		return styled, plain // the slug floor won't kick in → no wrap possible
	}
	// The slug is floored to minSlug, so the name row is wider than the reserved budget
	// assumed. Shrink the tag so markW + minSlug + dotW + 1(gap) + tag <= textW.
	maxTag := textW - markW - minSlug - dotW - 1
	if maxTag < 3 {
		return "", "" // no room for a meaningful tag → drop it
	}
	tp := truncateRunes(plain, maxTag)
	return m.styleSourceTag(f, tp), tp
}

// styleSourceTag re-applies a source's tag tint to an (already truncated) plain tag —
// the same coloring sourceTag uses — so a shrunk tag keeps its source color.
func (m Model) styleSourceTag(f *contract.Feature, plain string) string {
	if plain == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(m.sourceColor(f.Source)).Render(plain)
}

// truncateRunes truncates s to max runes with a trailing ellipsis (no minimum floor,
// unlike truncate) — used to shrink a source tag to an exact narrow budget.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// correlationChipsPlain is the plain (untinted) `⛓ plan-… ⛓ plan-…` chip string for
// a card's correlation membership (FR14) read straight from state.md, or "" when the
// ticket carries no correlation. Plural: one `⛓ plan-<id>` per membership,
// space-joined (a ticket in two plans shows two chips). The rune width feeds the
// card's fit math.
func correlationChipsPlain(f *contract.Feature) string {
	if len(f.Correlations) == 0 {
		return ""
	}
	parts := make([]string, len(f.Correlations))
	for i, id := range f.Correlations {
		parts[i] = "⛓ " + id
	}
	return strings.Join(parts, " ")
}

// correlationCountFallback is the compact chip a card shows when it is too narrow to
// render its full ⛓ plan-<id> chip(s) (TEST-002): `⛓ ×N` preserves the "belongs to N
// plans" signal instead of collapsing to a content-free `⛓ plan-…` ellipsis.
func correlationCountFallback(n int) string {
	return fmt.Sprintf("⛓ ×%d", n)
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

	// Three rows: name (+ mark, + live ● dot, + right-aligned SOURCE tag) · one-line
	// desc · status pill [+ agent chip]. Per the design (TURN-3a) the source tag rides
	// the NAME row, right-aligned — the description gets its own row below.
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
		// The tag rides the name row right-aligned; it is plain here (the focus fill
		// carries one fg/bg), and its width is reserved from the slug's budget. Right-
		// alignment pads to the card's TEXT area (width minus the style's Padding(0,1)),
		// so the padded line never overruns the frame and wraps.
		_, tag := m.sourceTag(f)
		textW := width - 2
		mw, dw := lipgloss.Width(mark), lipgloss.Width(dot)
		// Truncate the tag if a long source name at a narrow width would wrap the row
		// (REV-006). The focused card renders the tag plain (the focus fill owns fg/bg).
		_, tag = m.fitSourceTag(f, tag, tag, textW, mw, dw)
		slugBudget := textW - mw - dw
		if tag != "" {
			slugBudget -= lipgloss.Width(tag) + 1
		}
		nameLeft := mark + truncate(f.Slug, slugBudget) + dot
		if tag != "" {
			head = placeApart(nameLeft, tag, textW)
		} else {
			head = nameLeft
		}
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
		// Name row: mark + slug + live ● dot, with the (styled) SOURCE tag right-
		// aligned; its width is reserved from the slug's budget so nothing overruns.
		// Right-alignment pads to the card TEXT area (width minus the Padding(0,1)) so
		// the padded line stays inside the frame (no wrap).
		styled, plain := m.sourceTag(f)
		textW := width - 2
		mw, dw := lipgloss.Width(mark), lipgloss.Width(dot)
		// Truncate the tag if a long source name at a narrow width would wrap the row
		// (REV-006) — the composed name row must never exceed textW (the window height
		// math depends on it). Lone-repo (empty tag) is a no-op.
		styled, plain = m.fitSourceTag(f, styled, plain, textW, mw, dw)
		slugBudget := textW - mw - dw
		if plain != "" {
			slugBudget -= lipgloss.Width(plain) + 1
		}
		nameLeft := mark + slugStyle.Render(truncate(f.Slug, slugBudget)) + dot
		if plain != "" {
			head = placeApart(nameLeft, styled, textW)
		} else {
			head = nameLeft
		}
		titleLine = dimStyle.Render(truncate(title, width))
		badgeLine = pillStyleFor(f).Render(pillLabel(f))
		if agent != "" {
			badgeLine += "  " + sessionStyle.Render("● "+agent)
		}
	}

	// Correlation chip(s) (FR14): a member ticket paints its ⛓ plan-… chip(s) after
	// the status pill (plural — a ticket in two plans shows two chips), read straight
	// from state.md. Appended only when it FITS the card width, so it never wraps the
	// line (which would desync the window height math); a correlation-less card carries
	// nothing (byte-for-byte parity).
	if plain := correlationChipsPlain(f); plain != "" {
		if room := width - lipgloss.Width(badgeLine) - 1; room >= 8 {
			shown := plain
			// When the full ⛓ plan-<id> chip(s) don't fit, fall back to a compact
			// count (⛓ ×N) rather than an indistinguishable truncated `⛓ plan-…`
			// (TEST-002) — the card still says "belongs to N plans" at a narrow
			// width; full ids render at comfortable widths.
			if lipgloss.Width(plain) > room {
				shown = correlationCountFallback(len(f.Correlations))
			}
			shown = truncate(shown, room)
			if focused {
				badgeLine += " " + shown // plain — the focus fill carries one fg/bg
			} else {
				badgeLine += " " + correlationChipStyle.Render(shown)
			}
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
		full := "←→/h cols · ↑↓/jk cards · space select · enter drill · v view · w web · m move · d ship · a attach · l peek · x del · p source · tab plans/config · / filter · ? keys · q quit"
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
