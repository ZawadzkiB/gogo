package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// viewerWithLongContent puts the model into the viewer with a tall, already
// cached render so the paging keys have somewhere to scroll. Scrolling touches
// ONLY the viewport (its cached lines) — never a re-render.
func viewerWithLongContent(t *testing.T) Model {
	t.Helper()
	m := newModel(t)
	m.mode = modeViewer
	m.viewport.Width = 40
	m.viewport.Height = 6
	var b strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "line %d of the cached render\n", i)
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoTop()
	return m
}

// TEST-010: the viewer paging keys move the viewport over its cached content —
// half page (d/u), full page (space/b/pgdn/pgup), and top/bottom (g/G).
func TestViewerPagingKeys(t *testing.T) {
	m := viewerWithLongContent(t)

	// d = half page down (in the VIEWER context — not a ship).
	m = send(m, runes("d"))
	half := m.viewport.YOffset
	if half <= 0 {
		t.Fatalf("d in the viewer did not half-page down (yOffset=%d)", half)
	}

	// space = full page down — further than a half page.
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	if m.viewport.YOffset <= half {
		t.Errorf("space did not page down past the half page (%d <= %d)", m.viewport.YOffset, half)
	}

	// G = bottom, g = top.
	m = send(m, runes("G"))
	if !m.viewport.AtBottom() {
		t.Errorf("G did not jump to the bottom (yOffset=%d)", m.viewport.YOffset)
	}
	m = send(m, runes("g"))
	if m.viewport.YOffset != 0 {
		t.Errorf("g did not jump to the top (yOffset=%d)", m.viewport.YOffset)
	}

	// pgdown pages down; u half-pages back up.
	m = send(m, tea.KeyMsg{Type: tea.KeyPgDown})
	down := m.viewport.YOffset
	if down == 0 {
		t.Errorf("pgdown did not page down")
	}
	m = send(m, runes("u"))
	if m.viewport.YOffset >= down {
		t.Errorf("u did not half-page up (%d >= %d)", m.viewport.YOffset, down)
	}

	// j/k line scroll is still wired (forwarded to the viewport keymap).
	m = send(m, runes("g")) // top
	m = send(m, runes("j"))
	if m.viewport.YOffset != 1 {
		t.Errorf("j did not line-scroll down one (yOffset=%d)", m.viewport.YOffset)
	}
}

// TEST-010 (perf): paging must NOT re-render — no artifact command is spawned
// and the width-keyed render cache is untouched by a scroll keypress.
func TestViewerPagingDoesNotRerender(t *testing.T) {
	m := viewerWithLongContent(t)
	m.renderCache["sentinel"] = "cached"
	before := len(m.renderCache)
	for _, k := range []tea.Msg{runes("d"), runes("u"), runes("G"), runes("g"), tea.KeyMsg{Type: tea.KeySpace}} {
		nm, cmd := m.Update(k)
		m = nm.(Model)
		if cmd != nil {
			t.Errorf("paging key %v spawned a command (a re-render?) — scroll must be cache-only", k)
		}
	}
	if len(m.renderCache) != before || m.renderCache["sentinel"] != "cached" {
		t.Errorf("paging mutated the render cache — content was re-rendered")
	}
}

// TEST-010 key-conflict resolution: `d` and `space` keep their BOARD meaning
// (ship / select) — the viewer paging bindings only apply in the viewer.
func TestBoardShipAndSelectUnaffected(t *testing.T) {
	// d on a ready board card still opens the ship form.
	m, _ := launchable(t)
	m.colIdx = 2 // ready column
	nm, _ := m.Update(runes("d"))
	m = nm.(Model)
	if m.mode != modeForm {
		t.Errorf("board d no longer ships (mode=%d)", m.mode)
	}

	// space on a ready board card still selects (does not page).
	m2 := newModel(t)
	m2.colIdx = 2
	m2 = send(m2, tea.KeyMsg{Type: tea.KeySpace})
	if len(m2.selectedSlugs()) != 1 {
		t.Errorf("board space no longer selects: %v", m2.selectedSlugs())
	}
}

// TEST-010 key-conflict resolution for `G`: glow lives on the FILE LIST context
// now; in the open viewer `G` is bottom (not glow).
func TestGlowContextIsFileListNotViewer(t *testing.T) {
	// File list: G routes to glow (uninstalled → a hint), proving the binding moved.
	m := newModel(t)
	m.hasGlow = false
	m.colIdx = 1
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // drill = file list
	if m.mode != modeDrill {
		t.Fatalf("did not drill into the file list (mode=%d)", m.mode)
	}
	nm, _ := m.Update(runes("G"))
	m = nm.(Model)
	if !strings.Contains(m.status, "glow not installed") {
		t.Errorf("G in the file list did not route to glow: status=%q", m.status)
	}

	// Viewer: G scrolls to the bottom, it does NOT open glow.
	vm := viewerWithLongContent(t)
	vm.hasGlow = false
	vm = send(vm, runes("G"))
	if strings.Contains(vm.status, "glow not installed") {
		t.Errorf("G in the viewer wrongly routed to glow instead of scrolling")
	}
	if !vm.viewport.AtBottom() {
		t.Errorf("G in the viewer did not jump to the bottom (yOffset=%d)", vm.viewport.YOffset)
	}
}
