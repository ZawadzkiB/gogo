package tui

import "github.com/charmbracelet/lipgloss"

// Per-column vertical windowing (TEST-014). Columns used to render every card
// top-down, so a tall column (e.g. changelog with 9 shipped cards) overflowed
// the terminal and cards were cut off. Each column now keeps a scroll offset and
// renders only the WHOLE cards that fit, with ↑/↓ overflow indicators and a
// position hint; the focused column scrolls-into-view so the cursor is always
// fully visible. Card heights are MEASURED (lipgloss boxes are multi-line and a
// badge can add a line), never assumed fixed.

// --- board layout math -------------------------------------------------------

// boardColWidth is the per-column outer width (4 columns across the board). The
// board also draws 3 one-cell vertical separators between the columns (FR-B4),
// so those gutter cells are reserved out of the width budget before dividing by 4.
func (m Model) boardColWidth() int {
	const gutters = 3 // one-cell separators between the 4 columns
	colWidth := (boardWidth - 6 - gutters) / 4
	if m.width > 0 {
		colWidth = (m.width - 6 - gutters) / 4
	}
	if colWidth < 20 {
		colWidth = 20
	}
	return colWidth
}

// cardWidth is the inner card frame width (column minus its border + padding).
func (m Model) cardWidth() int {
	w := m.boardColWidth() - 4
	if w < 14 {
		w = 14
	}
	return w
}

// colAvail is the vertical budget (terminal rows) a single column has for its
// cards + overflow indicators: the whole height minus the board chrome (header,
// status, footer = 3 rows), the column's own head + blank line (2 rows), and the
// needs-you strip's height (1c) so the strip + board both fit rather than
// overflow (D3). A degraded strip contributes only its one summary line.
func (m Model) colAvail() int {
	return m.height - 5 - m.stripHeight()
}

// cardHeights measures each card's rendered height. Heights are measured (not
// assumed fixed) so a card that is a line taller still windows correctly. The
// collapsed changelog (FR-6) is a plain list, so its rows are one line each.
func (m Model) cardHeights(i, cardW int) []int {
	col := m.cols[i]
	hs := make([]int, len(col))
	if isChangelogCol(i) {
		for j := range hs {
			hs[j] = 1
		}
		return hs
	}
	for j, f := range col {
		focused := i == m.colIdx && j == m.cardIdx[i]
		hs[j] = lipgloss.Height(m.renderCard(i, f, focused, cardW))
	}
	return hs
}

// reflowColumns re-clamps each column's scroll offset so every window stays
// valid and the FOCUSED column keeps its focused card fully visible
// (scroll-into-view). It runs on every board-affecting transition: navigation,
// filter change, fsnotify reload, and window resize. Non-focused columns keep
// their offset (independent scrolling), only pulled back into a valid range.
func (m *Model) reflowColumns() {
	if m.height <= 0 {
		return // no size yet — View renders everything; offsets stay 0
	}
	avail := m.colAvail()
	if avail < 1 {
		avail = 1
	}
	cardW := m.cardWidth()
	for i := 0; i < 4; i++ {
		n := len(m.cols[i])
		if n == 0 {
			m.colOffset[i] = 0
			continue
		}
		heights := m.cardHeights(i, cardW)
		cur := clamp(m.cardIdx[i], 0, n-1)
		m.colOffset[i] = scrollWindow(heights, cur, m.colOffset[i], avail, i == m.colIdx)
	}
}

// --- pure windowing ----------------------------------------------------------

// fitEnd returns the exclusive end index of the visible window [start, end):
// as many WHOLE cards from `start` as fit in `avail` rows, reserving one row for
// the "↑ N more" indicator when start>0 and one for the "↓ N more" indicator
// when cards remain hidden below. At least one card is always shown, so a
// terminal too short for even a single card still displays the focused card
// (tiny-terminal degradation — never a negative slice index, never a panic).
func fitEnd(heights []int, start, avail int) int {
	n := len(heights)
	if start < 0 {
		start = 0
	}
	if start >= n {
		return n
	}
	budget := avail
	if start > 0 {
		budget-- // "↑ N more" indicator row
	}
	// Optimistic pass: assume the rest fits (no bottom indicator needed).
	total, overflow := 0, false
	for i := start; i < n; i++ {
		total += heights[i]
		if total > budget {
			overflow = true
			break
		}
	}
	if !overflow {
		return n
	}
	// Cards remain hidden below → reserve the "↓ N more" indicator row too.
	budget--
	end, used := start, 0
	for i := start; i < n; i++ {
		if used+heights[i] > budget {
			break
		}
		used += heights[i]
		end = i + 1
	}
	if end == start {
		end = start + 1 // always show at least the top card of the window
	}
	return end
}

// lastFitStart is the topmost start index for which the window ending at `last`
// (the bottom-most card) still fits — i.e. the maximum sane scroll offset. Used
// to pull an over-scrolled offset back so no empty space trails the last card.
func lastFitStart(heights []int, last, avail int) int {
	if last < 0 {
		return 0
	}
	start, used := last, heights[last]
	for i := last - 1; i >= 0; i-- {
		budget := avail
		if i > 0 {
			budget-- // a "↑ N more" indicator would sit above card i
		}
		if used+heights[i] > budget {
			break
		}
		used += heights[i]
		start = i
	}
	return start
}

// scrollWindow returns the clamped scroll offset for a column. Non-focused
// columns keep their offset (only re-clamped into a valid range). The focused
// column additionally scrolls-into-view: if the focused card sits above the
// window it scrolls up to it; if below, it scrolls down one card at a time
// until the focused card is fully visible.
func scrollWindow(heights []int, cur, offset, avail int, focused bool) int {
	n := len(heights)
	if n == 0 {
		return 0
	}
	maxStart := lastFitStart(heights, n-1, avail)
	start := clamp(offset, 0, maxStart)
	if !focused {
		return start
	}
	if cur < start {
		start = cur // focused card above the window — scroll up to it
	}
	for start < cur && cur >= fitEnd(heights, start, avail) {
		start++ // focused card below the window — scroll down one card
	}
	return clamp(start, 0, n-1)
}
