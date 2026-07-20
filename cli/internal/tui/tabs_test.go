package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
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

// TestProjectChipNarrows (FR3, D3=A): `p` cycles the board PROJECT chip and rebuild
// narrows the columns to that one project; the chip row renders on the unified board;
// and per D4 the chip moves the shared focus (m.project), defaulting back to
// allProjects[0] on "all".
func TestProjectChipNarrows(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Title: "A", Project: "alpha", Source: "web", Root: "/r/a", Class: contract.ClassUnfinished, Status: "plan-accepted"},
		{Slug: "b", Title: "B", Project: "beta", Source: "api", Root: "/r/b", Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	projs := []projects.Project{proj("alpha", src("web", "/r/a")), proj("beta", src("api", "/r/b"))}
	m := sizedWorkspaceAll(t, repo, projs)

	// Both projects visible → both plan cards.
	if len(m.cols[0]) != 2 {
		t.Fatalf("all-chip plan column = %d, want 2", len(m.cols[0]))
	}
	if !strings.Contains(m.View(), "project") {
		t.Errorf("board missing the project chips row:\n%s", m.View())
	}

	// p → narrow to alpha (the first project chip after "all") + move the focus (D4).
	m = send(m, runes("p"))
	if m.projectChip != "alpha" {
		t.Fatalf("after p projectChip = %q, want alpha", m.projectChip)
	}
	if len(m.cols[0]) != 1 || m.cols[0][0].Slug != "a" {
		t.Errorf("narrowed plan column = %v, want just [a]", m.cols[0])
	}
	if m.project == nil || m.project.Name != "alpha" {
		t.Errorf("p did not move the shared focus to alpha: %v", m.project)
	}

	// p → beta, then p → back to all (empty chip → focus defaults to allProjects[0]).
	m = send(m, runes("p"))
	if m.projectChip != "beta" || len(m.cols[0]) != 1 || m.cols[0][0].Slug != "b" {
		t.Errorf("after 2×p: chip=%q cols=%v, want beta → [b]", m.projectChip, m.cols[0])
	}
	m = send(m, runes("p"))
	if m.projectChip != "" || len(m.cols[0]) != 2 {
		t.Errorf("after 3×p: chip=%q cols=%d, want all → 2", m.projectChip, len(m.cols[0]))
	}
	if m.project == nil || m.project.Name != "alpha" {
		t.Errorf("all-chip did not default the focus to allProjects[0] (alpha): %v", m.project)
	}
}

// TestSingleRepoParity is the FR5 hard invariant: a lone repo with no home project
// renders the single-repo board byte-for-byte — NO tab bar, NO project chips, NO
// project-count note, and NO origin tags leak in. The tab key is inert.
func TestSingleRepoParity(t *testing.T) {
	m := newModel(t) // New(fixtureRoot) — a lone repo, m.root != ""
	if m.global() {
		t.Fatalf("single-repo board must not be global")
	}
	out := m.View()
	// The tab bar's non-board labels + the project chip row must be absent (no tabs /
	// chips on a lone repo).
	for _, leak := range []string{"plans", "config", "project ", "· 0 project", "· 1 project"} {
		if strings.Contains(out, leak) {
			t.Errorf("single-repo board leaked tabbed chrome %q:\n%s", leak, out)
		}
	}
	// The tab / shift+tab keys are inert on a lone repo.
	m = tab(m)
	if m.tab != tabBoard {
		t.Errorf("tab moved the tab on a lone repo (tab=%d), want inert board", m.tab)
	}
	// No origin tag prefix on any card (features carry no Project/Source in single-repo).
	if strings.Contains(out, "● proj") {
		t.Errorf("single-repo card leaked an origin tag:\n%s", out)
	}
	// p (project chip cycle) is a no-op on a lone repo.
	m = send(m, runes("p"))
	if m.projectChip != "" {
		t.Errorf("p set a project chip on a lone repo: %q", m.projectChip)
	}
}
