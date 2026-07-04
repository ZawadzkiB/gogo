package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- pure windowing math (TEST-014) -----------------------------------------

// TestFitEndVaryingHeights: N cards of VARYING heights in H rows → the correct
// visible set, with the ↑/↓ indicator rows reserved out of the budget.
func TestFitEndVaryingHeights(t *testing.T) {
	h := []int{3, 2, 4, 3, 2} // total 14
	cases := []struct {
		name               string
		start, avail, want int
	}{
		{"everything fits", 0, 14, 5},
		{"one hidden below reserves a bottom row", 0, 13, 4},
		{"packs whole cards only", 0, 7, 2},
		{"start>0 reserves a top row too", 2, 6, 3},
	}
	for _, c := range cases {
		if got := fitEnd(h, c.start, c.avail); got != c.want {
			t.Errorf("%s: fitEnd(%v,%d,%d)=%d want %d", c.name, h, c.start, c.avail, got, c.want)
		}
	}

	// A tall card among short ones still windows correctly.
	if got := fitEnd([]int{2, 2, 6, 2}, 0, 8); got != 2 {
		t.Errorf("tall-card fitEnd = %d, want 2", got)
	}
}

// TestScrollWindowCursorFollow: the focused column scrolls-into-view — moving
// below the window shifts it down one card, moving above snaps up, and the last
// card is reachable and fully visible. Non-focused columns keep their offset.
func TestScrollWindowCursorFollow(t *testing.T) {
	h := []int{5, 5, 5, 5, 5, 5, 5, 5, 5} // 9 cards, avail 15 → 2 whole cards + a ↓
	const avail = 15

	// Cursor at top: offset stays 0.
	if got := scrollWindow(h, 0, 0, avail, true); got != 0 {
		t.Errorf("cur=0 offset=%d, want 0", got)
	}
	// Move to the first card past the window bottom → scroll down one.
	if got := scrollWindow(h, 2, 0, avail, true); got != 1 {
		t.Errorf("cur=2 offset=%d, want 1 (window follows down)", got)
	}
	// Jump to the last card → offset lands so the last card sits fully at the
	// bottom (no ↓ indicator, ↑ shown).
	if got := scrollWindow(h, 8, 1, avail, true); got != 7 {
		t.Errorf("cur=8 offset=%d, want 7 (last card fully visible)", got)
	}
	end := fitEnd(h, 7, avail)
	if end != 9 {
		t.Errorf("window from 7 ends at %d, want 9 (last card visible, no bottom overflow)", end)
	}
	// Move back to the top from a scrolled window → snaps up to the cursor.
	if got := scrollWindow(h, 0, 7, avail, true); got != 0 {
		t.Errorf("cur=0 from offset 7 = %d, want 0 (scroll up)", got)
	}

	// A NON-focused column keeps its offset (only re-clamped into range).
	if got := scrollWindow(h, 8, 5, avail, false); got != 5 {
		t.Errorf("non-focused kept offset = %d, want 5 (independent scroll)", got)
	}
	if got := scrollWindow(h, 0, 99, avail, false); got != 7 {
		t.Errorf("over-scrolled non-focused offset = %d, want clamp to maxStart 7", got)
	}
}

// TestLastFitStart: the maximum sane offset places the last card at the bottom.
func TestLastFitStart(t *testing.T) {
	h := []int{5, 5, 5, 5, 5, 5, 5, 5, 5}
	if got := lastFitStart(h, len(h)-1, 15); got != 7 {
		t.Errorf("lastFitStart = %d, want 7", got)
	}
	if got := lastFitStart([]int{4}, 0, 15); got != 0 {
		t.Errorf("single-card lastFitStart = %d, want 0", got)
	}
}

// TestTinyHeightDegradation: a terminal too short for even one card still shows
// the focused card + indicators — never a negative slice index, never a panic.
func TestTinyHeightDegradation(t *testing.T) {
	h := []int{5, 5, 5}
	if got := fitEnd(h, 0, 2); got != 1 {
		t.Errorf("tiny fitEnd = %d, want 1 (at least one card)", got)
	}
	// Focused on the LAST card in a 2-row terminal: window degrades to just it.
	got := scrollWindow(h, 2, 0, 2, true)
	if got != 2 {
		t.Errorf("tiny scrollWindow offset = %d, want 2 (focused card shown)", got)
	}
	if e := fitEnd(h, got, 2); e <= got || e > len(h) {
		t.Errorf("tiny window [%d,%d) invalid", got, e)
	}
}

// TestEmptyColumnWindow: an empty column pins offset 0 and an empty window.
func TestEmptyColumnWindow(t *testing.T) {
	if got := scrollWindow(nil, 0, 3, 10, true); got != 0 {
		t.Errorf("empty scrollWindow = %d, want 0", got)
	}
	if got := fitEnd(nil, 0, 10); got != 0 {
		t.Errorf("empty fitEnd = %d, want 0", got)
	}
}

// --- model / View integration (TEST-014) ------------------------------------

// smallBoard focuses the changelog column on a short terminal so its 3 fixture
// cards (5 rows each) must window: colAvail(13)=8 → one whole card + a ↓.
func smallBoard(t *testing.T) Model {
	t.Helper()
	m := newModel(t)
	m = send(m, tea.WindowSizeMsg{Width: 200, Height: 13})
	m = right(m) // → in progress (arrow — `l` is now peek)
	m = right(m) // → ready
	m = right(m) // → changelog (focused)
	if m.colIdx != 3 {
		t.Fatalf("did not focus changelog, colIdx=%d", m.colIdx)
	}
	return m
}

// TestColumnWindowIndicatorsHiddenWhenFits: on a tall terminal every card fits,
// so a column shows NO overflow arrows and NO position hint (no noise).
func TestColumnWindowIndicatorsHiddenWhenFits(t *testing.T) {
	m := newModel(t) // height 40 → colAvail 35, all fixture columns fit
	out := m.renderColumn(3, m.boardColWidth())
	for _, noise := range []string{"↑", "↓", "–"} {
		if strings.Contains(out, noise) {
			t.Errorf("short column showed overflow noise %q:\n%s", noise, out)
		}
	}
}

// TestColumnWindowIndicatorsAndHint: when a column overflows it shows the ↓/↑
// "N more" indicators AND a header position hint reflecting the visible range.
func TestColumnWindowIndicatorsAndHint(t *testing.T) {
	m := newModel(t)
	m.height = 13 // colAvail = 8 → one 5-row card fits
	m.colIdx = 3

	m.colOffset[3] = 0
	top := m.renderColumn(3, m.boardColWidth())
	if !strings.Contains(top, "↓ 2 more") {
		t.Errorf("top window missing ↓ indicator:\n%s", top)
	}
	if !strings.Contains(top, "1–1") {
		t.Errorf("top window missing position hint 1–1:\n%s", top)
	}
	if strings.Contains(top, "↑") {
		t.Errorf("top window wrongly showed a ↑ indicator:\n%s", top)
	}

	m.colOffset[3] = 2
	bot := m.renderColumn(3, m.boardColWidth())
	if !strings.Contains(bot, "↑ 2 more") {
		t.Errorf("bottom window missing ↑ indicator:\n%s", bot)
	}
	if !strings.Contains(bot, "3–3") {
		t.Errorf("bottom window missing position hint 3–3:\n%s", bot)
	}
	if strings.Contains(bot, "↓") {
		t.Errorf("bottom window wrongly showed a ↓ indicator:\n%s", bot)
	}
}

// TestBoardWindowScrollIntoView: navigating down a windowed column keeps the
// FOCUSED card fully visible at every step, hides the cards that don't fit, and
// makes the last card reachable (the ↑ indicator appears once scrolled).
func TestBoardWindowScrollIntoView(t *testing.T) {
	m := smallBoard(t)
	col := m.cols[3]
	if len(col) < 3 {
		t.Fatalf("changelog fixture has %d cards, need ≥3", len(col))
	}

	// At the top: the focused (first) card is visible, the last is hidden below.
	out := m.View()
	if !strings.Contains(out, col[0].Slug) {
		t.Errorf("focused first card %q not visible:\n%s", col[0].Slug, out)
	}
	if strings.Contains(out, col[len(col)-1].Slug) {
		t.Errorf("last card %q should be windowed OUT at the top", col[len(col)-1].Slug)
	}
	if !strings.Contains(out, "↓") {
		t.Errorf("top of a windowed column should show a ↓ indicator")
	}

	// Walk to the bottom; the focused card must stay visible the whole way.
	for j := 1; j < len(col); j++ {
		m = send(m, runes("j"))
		if f := m.focusedCard(); f == nil || !strings.Contains(m.View(), f.Slug) {
			t.Fatalf("focused card %v scrolled out of view at step %d", f, j)
		}
	}
	// At the bottom the last card is visible and the ↑ indicator has appeared.
	out = m.View()
	if !strings.Contains(out, col[len(col)-1].Slug) {
		t.Errorf("last card %q not reachable/visible:\n%s", col[len(col)-1].Slug, out)
	}
	if !strings.Contains(out, "↑") {
		t.Errorf("scrolled-down column should show a ↑ indicator")
	}
}

// TestFilterResetClampsOffset: a filter that shrinks a scrolled column re-clamps
// its offset into range (here down to 0 for a now single-card column).
func TestFilterResetClampsOffset(t *testing.T) {
	m := smallBoard(t)
	// Scroll the changelog to the bottom.
	for j := 1; j < len(m.cols[3]); j++ {
		m = send(m, runes("j"))
	}
	if m.colOffset[3] == 0 {
		t.Fatalf("precondition: changelog should be scrolled (offset>0)")
	}

	// Filter down to a single changelog card.
	m = send(m, runes("/"))
	for _, r := range "shipped-by-folder" {
		m = send(m, runes(string(r)))
	}
	if len(m.cols[3]) != 1 {
		t.Fatalf("filter left %d changelog cards, want 1", len(m.cols[3]))
	}
	if m.colOffset[3] != 0 {
		t.Errorf("filter did not clamp the scroll offset: colOffset[3]=%d, want 0", m.colOffset[3])
	}
	// The lone card renders with no overflow noise.
	if out := m.renderColumn(3, m.boardColWidth()); strings.Contains(out, "↑") || strings.Contains(out, "↓") {
		t.Errorf("single-card column still shows overflow indicators:\n%s", out)
	}
}

// TestRefocusKeepsSlugUnderCursor: after a reload (indices can shift) the cursor
// follows the focused slug if it survives, else clamps — the precondition for
// reflow keeping it visible.
func TestRefocusKeepsSlugUnderCursor(t *testing.T) {
	m := newModel(t)
	m.colIdx = 3
	col := m.cols[3]
	if len(col) < 3 {
		t.Fatalf("need ≥3 changelog cards")
	}
	target := col[len(col)-1].Slug

	m.cardIdx[3] = 0
	m.refocus(target)
	if got := m.focusedCard(); got == nil || got.Slug != target {
		t.Errorf("refocus did not follow the slug: focused=%v want %q", got, target)
	}

	// A vanished slug clamps into range (no panic, valid index).
	m.cardIdx[3] = len(col) - 1
	m.refocus("does-not-exist")
	if m.cardIdx[3] < 0 || m.cardIdx[3] >= len(m.cols[3]) {
		t.Errorf("refocus left an out-of-range cardIdx=%d", m.cardIdx[3])
	}
}

// TestReloadReflowNoPanic: a reloadMsg re-reads the repo, refocuses, and
// re-windows without panicking and with valid offsets.
func TestReloadReflowNoPanic(t *testing.T) {
	m := smallBoard(t)
	for j := 1; j < len(m.cols[3]); j++ {
		m = send(m, runes("j"))
	}
	nm, _ := m.Update(reloadMsg{})
	m = nm.(Model)
	for i := 0; i < 4; i++ {
		if n := len(m.cols[i]); n > 0 && (m.colOffset[i] < 0 || m.colOffset[i] >= n) {
			t.Errorf("col %d offset %d out of range (n=%d) after reload", i, m.colOffset[i], n)
		}
	}
	// The focused card is still visible after the reload.
	if f := m.focusedCard(); f == nil || !strings.Contains(m.View(), f.Slug) {
		t.Errorf("focused card not visible after reload: %v", f)
	}
}
