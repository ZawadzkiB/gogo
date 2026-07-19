package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	tea "github.com/charmbracelet/bubbletea"
)

func tab(m Model) Model      { return send(m, tea.KeyMsg{Type: tea.KeyTab}) }
func shiftTab(m Model) Model { return send(m, tea.KeyMsg{Type: tea.KeyShiftTab}) }

// TestTabCycling (FR8/D6): tab cycles board → plans → config → board and shift+tab
// runs it in reverse, on a project board.
func TestTabCycling(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("svc", "/r/svc")))
	if m.tab != tabBoard {
		t.Fatalf("start tab = %d, want board", m.tab)
	}
	m = tab(m)
	if m.tab != tabPlans {
		t.Errorf("tab×1 = %d, want plans", m.tab)
	}
	m = tab(m)
	if m.tab != tabConfig {
		t.Errorf("tab×2 = %d, want config", m.tab)
	}
	m = tab(m)
	if m.tab != tabBoard {
		t.Errorf("tab×3 = %d, want board (wrapped)", m.tab)
	}
	// shift+tab wraps backward to config.
	m = shiftTab(m)
	if m.tab != tabConfig {
		t.Errorf("shift+tab from board = %d, want config", m.tab)
	}
}

// TestTabBarRendersAcrossTabs: the tab bar shows all three labels on every tab of a
// project board.
func TestTabBarRendersAcrossTabs(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("svc", "/r/svc")))
	for _, want := range []string{"board", "plans", "config"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("board tab: tab bar missing %q", want)
		}
	}
	m = tab(m) // plans
	if out := m.View(); !strings.Contains(out, "board") || !strings.Contains(out, "config") {
		t.Errorf("plans tab: tab bar missing labels:\n%s", out)
	}
}

// TestPlansTabScaffold (FR10): the plans tab renders the titled skeleton with the
// three lifecycle section headers Phase C fills.
func TestPlansTabScaffold(t *testing.T) {
	m := tab(sizedWorkspace(t, &contract.Repo{}, proj("app", src("svc", "/r/svc"))))
	if m.tab != tabPlans {
		t.Fatalf("tab did not reach plans, tab=%d", m.tab)
	}
	out := m.View()
	for _, want := range []string{"plans", "DRAFTS", "READY", "ACTIVE"} {
		if !strings.Contains(out, want) {
			t.Errorf("plans scaffold missing %q:\n%s", want, out)
		}
	}
}

// TestSourceChipNarrows (FR7): `p` cycles the board source chip and rebuild narrows
// the columns to that one source; the chips render on the board.
func TestSourceChipNarrows(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Title: "A", Source: "projA", Root: "/r/a", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "b", Title: "B", Source: "projB", Root: "/r/b", Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	m := sizedWorkspace(t, repo, proj("app", src("projA", "/r/a"), src("projB", "/r/b")))

	// Both sources visible → both plan cards.
	if len(m.cols[0]) != 2 {
		t.Fatalf("all-chip plan column = %d, want 2", len(m.cols[0]))
	}
	if !strings.Contains(m.View(), "sources") {
		t.Errorf("board missing the source chips row:\n%s", m.View())
	}

	// p → narrow to projA (the first source chip after "all").
	m = send(m, runes("p"))
	if m.sourceChip != "projA" {
		t.Fatalf("after p sourceChip = %q, want projA", m.sourceChip)
	}
	if len(m.cols[0]) != 1 || m.cols[0][0].Slug != "a" {
		t.Errorf("narrowed plan column = %v, want just [a]", m.cols[0])
	}

	// p → projB, then p → back to all (empty chip).
	m = send(m, runes("p"))
	if m.sourceChip != "projB" || len(m.cols[0]) != 1 || m.cols[0][0].Slug != "b" {
		t.Errorf("after 2×p: chip=%q cols=%v, want projB → [b]", m.sourceChip, m.cols[0])
	}
	m = send(m, runes("p"))
	if m.sourceChip != "" || len(m.cols[0]) != 2 {
		t.Errorf("after 3×p: chip=%q cols=%d, want all → 2", m.sourceChip, len(m.cols[0]))
	}
}

// TestSingleRepoParity is the FR7 hard invariant: a lone repo with no home project
// renders the single-repo board byte-for-byte — NO tab bar, NO source chips, NO
// project-count note, and NO source tags leak in. The tab key is inert.
func TestSingleRepoParity(t *testing.T) {
	m := newModel(t) // New(fixtureRoot) — a lone repo, m.root != ""
	if m.global() {
		t.Fatalf("single-repo board must not be global")
	}
	out := m.View()
	// The tab bar's non-board labels must be absent (no tabs on a lone repo).
	for _, leak := range []string{"plans", "config", "sources ", "· 0 project", "· 1 project"} {
		if strings.Contains(out, leak) {
			t.Errorf("single-repo board leaked tabbed chrome %q:\n%s", leak, out)
		}
	}
	// The tab / shift+tab keys are inert on a lone repo.
	m = tab(m)
	if m.tab != tabBoard {
		t.Errorf("tab moved the tab on a lone repo (tab=%d), want inert board", m.tab)
	}
	// No source tag prefix on any card (features carry no Source in single-repo mode).
	if strings.Contains(out, "● proj") {
		t.Errorf("single-repo card leaked a source tag:\n%s", out)
	}
	// p (source chip cycle) is a no-op on a lone repo.
	m = send(m, runes("p"))
	if m.sourceChip != "" {
		t.Errorf("p set a source chip on a lone repo: %q", m.sourceChip)
	}
}
