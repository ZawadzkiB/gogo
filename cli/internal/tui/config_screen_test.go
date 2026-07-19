package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
)

// seedConfigHome points the legacy registry / epics store at a fresh t.TempDir() via
// the GOGO_CONFIG_HOME seam so no test ever touches the real ~/.config.
func seedConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GOGO_CONFIG_HOME", dir)
	return dir
}

// seedDataHome points the projects store (~/.gogo/) at a fresh t.TempDir() via the
// GOGO_DATA_HOME seam so no config-tab test ever touches the real ~/.gogo.
func seedDataHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GOGO_DATA_HOME", dir)
	return dir
}

// gogoRepoDir makes a fresh t.TempDir() look like a gogo source (has .gogo/) so the
// per-source form path validation (dir contains .gogo/, like `gogo source add`)
// accepts it. Returns the absolute path.
func gogoRepoDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gogo", "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// configTab drives a fresh project board to the config tab (tab → plans → config).
func configTab(m Model) Model {
	m = send(m, tea.KeyMsg{Type: tea.KeyTab}) // board → plans
	m = send(m, tea.KeyMsg{Type: tea.KeyTab}) // plans → config
	return m
}

// TestConfigTabReached: tab/shift+tab reach (and leave) the config tab; the config
// tab renders the focused project's sources.
func TestConfigTabReached(t *testing.T) {
	seedDataHome(t)
	p := projects.Project{Name: "app", Sources: []projects.Source{{Name: "svc", Path: "/repos/svc"}}}
	m := sizedWorkspace(t, &contract.Repo{}, p)

	m = configTab(m)
	if m.tab != tabConfig {
		t.Fatalf("tab×2 did not reach the config tab, tab=%d", m.tab)
	}
	if !strings.Contains(m.View(), "config — sources") {
		t.Errorf("config tab did not render:\n%s", m.View())
	}
	// shift+tab returns toward the board.
	m = send(m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.tab != tabPlans {
		t.Errorf("shift+tab from config went to tab=%d, want plans", m.tab)
	}
}

// TestConfigTabSourceNav: ↑↓/jk move the per-source cursor, clamped to range.
func TestConfigTabSourceNav(t *testing.T) {
	seedDataHome(t)
	p := projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "a", Path: "/r/a"}, {Name: "b", Path: "/r/b"}, {Name: "c", Path: "/r/c"},
	}}
	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	if m.sourceIdx != 0 {
		t.Fatalf("start sourceIdx = %d, want 0", m.sourceIdx)
	}
	m = send(m, runes("j"))
	m = send(m, runes("j"))
	if m.sourceIdx != 2 {
		t.Errorf("after 2×j sourceIdx = %d, want 2", m.sourceIdx)
	}
	m = send(m, runes("j")) // clamp at the end
	if m.sourceIdx != 2 {
		t.Errorf("sourceIdx over-ran the end: %d, want 2", m.sourceIdx)
	}
	m = send(m, runes("k"))
	if m.sourceIdx != 1 {
		t.Errorf("after k sourceIdx = %d, want 1", m.sourceIdx)
	}
}

// TestConfigTabAddSource: `a` opens an add form (pendingSource.op == "add", cap
// defaulted to 1); a completed add validates the path + cap and persists the source
// to the project's config.json (asserted via projects.Load under GOGO_DATA_HOME).
func TestConfigTabAddSource(t *testing.T) {
	seedDataHome(t)
	repo := gogoRepoDir(t)
	p := projects.Project{Name: "app"}
	if err := projects.Save(&p); err != nil {
		t.Fatal(err)
	}

	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	m = send(m, runes("a"))
	if m.mode != modeForm || m.pendingSource == nil || m.pendingSource.op != "add" {
		t.Fatalf("a did not open an add form (mode=%d pending=%v)", m.mode, m.pendingSource)
	}
	if m.pendingSource.project != "app" {
		t.Errorf("add form targets project %q, want app", m.pendingSource.project)
	}
	if m.binding.srcCap != "1" {
		t.Errorf("add form cap default = %q, want \"1\"", m.binding.srcCap)
	}
	// Fill the heap-stable binding (what finishSourceForm reads, TEST-001) and complete.
	m.binding.srcPath = repo
	m.binding.srcName = "svc"
	m.binding.srcCap = "3"
	nm, _ := m.finishSourceForm()
	m = nm.(Model)

	if m.tab != tabConfig || m.mode != modeBoard {
		t.Errorf("finishSourceForm did not return to the config tab (tab=%d mode=%d)", m.tab, m.mode)
	}
	got, _ := projects.Load("app")
	if len(got.Sources) != 1 {
		t.Fatalf("after add: %d sources, want 1", len(got.Sources))
	}
	if got.Sources[0].Path != filepath.Clean(repo) || got.Sources[0].Name != "svc" || got.Sources[0].ConcurrentWorkItems != 3 {
		t.Errorf("added source = %+v, want {name=svc path=%s cap=3}", got.Sources[0], filepath.Clean(repo))
	}
}

// TestConfigTabAddRejectsNonGogoPath: a path without a .gogo/ dir is refused (like
// `gogo source add`) and nothing is written.
func TestConfigTabAddRejectsNonGogoPath(t *testing.T) {
	seedDataHome(t)
	bare := t.TempDir() // no .gogo/
	p := projects.Project{Name: "app"}
	projects.Save(&p)

	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	m = send(m, runes("a"))
	m.binding.srcPath = bare
	nm, _ := m.finishSourceForm()
	m = nm.(Model)

	if !strings.Contains(m.status, "no .gogo/") {
		t.Errorf("status = %q, want a 'no .gogo/' refusal", m.status)
	}
	if got, _ := projects.Load("app"); len(got.Sources) != 0 {
		t.Errorf("a rejected add still wrote %d sources, want 0", len(got.Sources))
	}
}

// TestConfigTabEditCapPersists: `e` seeds the form from the focused source; a
// completed edit updates the per-source cap in place and persists to config.json
// (the FR9 per-source cap edit).
func TestConfigTabEditCapPersists(t *testing.T) {
	seedDataHome(t)
	repo := gogoRepoDir(t)
	p := projects.Project{Name: "app", Sources: []projects.Source{{Name: "svc", Path: repo, ConcurrentWorkItems: 1}}}
	projects.Save(&p)

	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	m = send(m, runes("e"))
	if m.mode != modeForm || m.pendingSource == nil || m.pendingSource.op != "edit" {
		t.Fatalf("e did not open an edit form (mode=%d pending=%v)", m.mode, m.pendingSource)
	}
	if m.pendingSource.origPath != repo || m.binding.srcCap != "1" {
		t.Fatalf("edit form not seeded from the source: origPath=%q cap=%q", m.pendingSource.origPath, m.binding.srcCap)
	}
	m.binding.srcCap = "5"
	nm, _ := m.finishSourceForm()
	m = nm.(Model)

	got, _ := projects.Load("app")
	if len(got.Sources) != 1 {
		t.Fatalf("after edit: %d sources, want 1 (edit must not duplicate)", len(got.Sources))
	}
	if got.Sources[0].ConcurrentWorkItems != 5 {
		t.Errorf("edited cap = %d, want 5", got.Sources[0].ConcurrentWorkItems)
	}
}

// TestConfigTabRemoveSource: `x` → confirm → the source is removed. Driven through
// huh's real StateCompleted pump (keyPress y), exercising the updateForm →
// pendingSource → finishSourceForm routing branch end-to-end.
func TestConfigTabRemoveSource(t *testing.T) {
	seedDataHome(t)
	p := projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "keep", Path: "/r/keep"}, {Name: "drop", Path: "/r/drop"},
	}}
	projects.Save(&p)

	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	m = send(m, runes("j")) // focus "drop"
	m = send(m, runes("x"))
	if m.mode != modeForm || m.pendingSource == nil || m.pendingSource.op != "remove" {
		t.Fatalf("x did not open a remove confirm (mode=%d pending=%v)", m.mode, m.pendingSource)
	}
	m = keyPress(t, m, runes("y")) // confirm Remove and complete through the pump

	if m.tab != tabConfig || m.mode != modeBoard {
		t.Errorf("remove did not return to the config tab (tab=%d mode=%d)", m.tab, m.mode)
	}
	got, _ := projects.Load("app")
	if len(got.Sources) != 1 || got.Sources[0].Name != "keep" {
		t.Errorf("after remove: %+v, want [keep]", got.Sources)
	}
}

// TestConfigFormCancelReturnsToConfigTab: cancelling a per-source form (Esc) returns
// to the config tab, not away from it.
func TestConfigFormCancelReturnsToConfigTab(t *testing.T) {
	seedDataHome(t)
	p := projects.Project{Name: "app", Sources: []projects.Source{{Name: "svc", Path: "/r/svc"}}}
	projects.Save(&p)

	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	m = send(m, runes("a"))
	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.tab != tabConfig || m.mode != modeBoard {
		t.Errorf("esc-cancel left tab=%d mode=%d, want the config tab", m.tab, m.mode)
	}
	if m.pendingSource != nil {
		t.Errorf("pendingSource not cleared after cancel: %v", m.pendingSource)
	}
}

// TestConfigTabProjectSwitcher: `p` cycles the focused home project across the store.
func TestConfigTabProjectSwitcher(t *testing.T) {
	seedDataHome(t)
	a := projects.Project{Name: "alpha", Sources: []projects.Source{{Name: "a", Path: "/r/a"}}}
	b := projects.Project{Name: "beta", Sources: []projects.Source{{Name: "b", Path: "/r/b"}}}
	projects.Save(&a)
	projects.Save(&b)

	m := NewProjectBoard(a)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = configTab(nm.(Model))
	if len(m.allProjects) != 2 {
		t.Fatalf("config tab loaded %d projects, want 2", len(m.allProjects))
	}
	start := m.project.Name
	m = send(m, runes("p"))
	if m.project.Name == start {
		t.Errorf("p did not switch the focused project (still %q)", m.project.Name)
	}
}

// TestConfigProjectColorEdit (cockpit-colors FR4): `c` opens the project label-color
// form (pendingProject set); a completed edit accepting a swatch NAME persists the
// resolved hex to Project.Color and re-tints the board (refreshProject).
func TestConfigProjectColorEdit(t *testing.T) {
	seedDataHome(t)
	p := projects.Project{Name: "app", Sources: []projects.Source{{Name: "svc", Path: "/r/svc"}}}
	if err := projects.Save(&p); err != nil {
		t.Fatal(err)
	}

	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	m = send(m, runes("c"))
	if m.mode != modeForm || m.pendingProject == nil || m.pendingProject.name != "app" {
		t.Fatalf("c did not open the project-color form (mode=%d pending=%v)", m.mode, m.pendingProject)
	}
	// Accept a swatch NAME through the heap-stable binding (TEST-001).
	m.binding.projColor = "teal"
	nm, _ := m.finishProjectColorForm()
	m = nm.(Model)

	if m.tab != tabConfig || m.mode != modeBoard {
		t.Errorf("project-color edit did not return to the config tab (tab=%d mode=%d)", m.tab, m.mode)
	}
	got, _ := projects.Load("app")
	if got.Color != "#35c9b5" {
		t.Errorf("project color = %q, want the teal swatch hex #35c9b5", got.Color)
	}
	// The live board re-tinted (refreshProject rebuilt projectColors from the store).
	if m.projectColors["app"] != "#35c9b5" {
		t.Errorf("board projectColors not re-tinted live: %q", m.projectColors["app"])
	}
}

// TestConfigSourceColorSwatchNameOrHex (cockpit-colors FR4): the source `e` form's Label
// color field accepts a swatch NAME (resolved to its hex) or a raw hex, persisted to
// Source.Color.
func TestConfigSourceColorSwatchNameOrHex(t *testing.T) {
	seedDataHome(t)
	repo := gogoRepoDir(t)
	p := projects.Project{Name: "app", Sources: []projects.Source{{Name: "svc", Path: repo, ConcurrentWorkItems: 1}}}
	projects.Save(&p)

	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	m = send(m, runes("e"))
	if m.pendingSource == nil || m.pendingSource.op != "edit" {
		t.Fatalf("e did not open an edit form (pending=%v)", m.pendingSource)
	}
	m.binding.srcColor = "pink" // a swatch name
	nm, _ := m.finishSourceForm()
	m = nm.(Model)
	if got, _ := projects.Load("app"); got.Sources[0].Color != "#eb7bb5" {
		t.Errorf("source color = %q, want the pink swatch hex #eb7bb5", got.Sources[0].Color)
	}
}

// TestConfigAddAssignsBlankColor (cockpit-colors FR4): a blank Label color on ADD gets an
// auto-assigned palette swatch (a new source is never colorless).
func TestConfigAddAssignsBlankColor(t *testing.T) {
	seedDataHome(t)
	repo := gogoRepoDir(t)
	p := projects.Project{Name: "app"}
	projects.Save(&p)

	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	m = send(m, runes("a"))
	m.binding.srcPath = repo
	m.binding.srcName = "svc"
	// srcColor left blank → auto-assign on finish.
	nm, _ := m.finishSourceForm()
	m = nm.(Model)
	got, _ := projects.Load("app")
	if len(got.Sources) != 1 || got.Sources[0].Color == "" {
		t.Fatalf("blank color on add was not auto-assigned: %+v", got.Sources)
	}
	if _, ok := projects.LookupSwatch(got.Sources[0].Color); !ok {
		t.Errorf("auto-assigned color %q is not a palette swatch", got.Sources[0].Color)
	}
}

// TestConfigTabRendersColorDots (cockpit-colors FR4): the config left column shows a dot
// per project and per source, and the right pane shows the "label color · <name>" field
// naming the swatch.
func TestConfigTabRendersColorDots(t *testing.T) {
	seedDataHome(t)
	p := projects.Project{Name: "app", Color: "#58a6ff", Sources: []projects.Source{
		{Name: "svc", Path: "/r/svc", Color: "#35c9b5"},
	}}
	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))
	out := m.View()
	for _, want := range []string{"● svc", "label color", "teal"} {
		if !strings.Contains(out, want) {
			t.Errorf("config tab missing %q:\n%s", want, out)
		}
	}
}

// TestViewConfigTab: the config tab renders the sources with their caps (`cap N` /
// `cap ∞`) and the knowledge explorer lists the focused source's .gogo/knowledge/
// files + sizes.
func TestViewConfigTab(t *testing.T) {
	seedDataHome(t)
	repo := gogoRepoDir(t)
	if err := os.MkdirAll(filepath.Join(repo, ".gogo", "knowledge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gogo", "knowledge", "coding-rules.md"), []byte("hello knowledge"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "capped", Path: repo, ConcurrentWorkItems: 2},
		{Name: "unlimited", Path: "/r/unlimited"},
	}}
	m := configTab(sizedWorkspace(t, &contract.Repo{}, p))

	out := m.View()
	for _, want := range []string{"config — sources", "capped", "cap 2", "unlimited", "cap ∞", "knowledge", "coding-rules.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("config tab view missing %q:\n%s", want, out)
		}
	}
}
