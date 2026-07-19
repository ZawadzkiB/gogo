package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// gogoRepo makes a t.TempDir() look like a gogo source (has .gogo/) and returns
// its absolute path.
func gogoRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gogo", "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// seedDataHome points the projects store at a fresh t.TempDir() via the
// GOGO_DATA_HOME seam, so no test ever touches the real ~/.gogo.
func seedDataHome(t *testing.T) {
	t.Helper()
	t.Setenv("GOGO_DATA_HOME", t.TempDir())
}

// seedConfigHome points the legacy config stores (drafts / epics, still under
// ~/.config/gogo through Phase C) at a fresh t.TempDir() via the GOGO_CONFIG_HOME
// seam. Used by the draft / epic command tests.
func seedConfigHome(t *testing.T) {
	t.Helper()
	t.Setenv("GOGO_CONFIG_HOME", t.TempDir())
}

func TestCmdProjectAddListRm(t *testing.T) {
	seedDataHome(t)
	root := gogoRepo(t)
	name := filepath.Base(root)

	// add — happy path: creates a project with the repo as source #1.
	if code := cmdProject([]string{"add", root}); code != 0 {
		t.Fatalf("project add: exit %d, want 0", code)
	}
	list, _ := projects.List()
	if len(list) != 1 {
		t.Fatalf("after add: %d projects, want 1", len(list))
	}
	if list[0].Name != name || len(list[0].Sources) != 1 || list[0].Sources[0].Path != root {
		t.Errorf("added project = %+v, want {name=%s, source path=%s}", list[0], name, root)
	}
	// The default per-source cap is 1 (D5).
	if list[0].Sources[0].ConcurrentWorkItems != projects.DefaultConcurrentWorkItems {
		t.Errorf("source cap = %d, want %d", list[0].Sources[0].ConcurrentWorkItems, projects.DefaultConcurrentWorkItems)
	}

	// list — the table names the project + its source.
	out := FormatProjects(list)
	for _, want := range []string{"1 project", name, root} {
		if !strings.Contains(out, want) {
			t.Errorf("project list missing %q\n%s", want, out)
		}
	}

	// rm by name → removes the whole project.
	if code := cmdProject([]string{"rm", name}); code != 0 {
		t.Fatalf("project rm: exit %d, want 0", code)
	}
	if list, _ := projects.List(); len(list) != 0 {
		t.Errorf("after rm: %d projects, want 0", len(list))
	}
}

// TestCmdProjectAddName: --name overrides the basename default.
func TestCmdProjectAddName(t *testing.T) {
	seedDataHome(t)
	root := gogoRepo(t)
	if code := cmdProject([]string{"add", root, "--name", "custom"}); code != 0 {
		t.Fatalf("project add --name: exit %d, want 0", code)
	}
	list, _ := projects.List()
	if len(list) != 1 || list[0].Name != "custom" {
		t.Errorf("project = %+v, want name=custom", list)
	}
}

func TestCmdProjectAddBadPath(t *testing.T) {
	seedDataHome(t)
	bare := t.TempDir() // no .gogo/
	if code := cmdProject([]string{"add", bare}); code == 0 {
		t.Errorf("project add on a non-gogo dir: exit 0, want non-zero")
	}
	if list, _ := projects.List(); len(list) != 0 {
		t.Errorf("bad add still wrote %d projects, want 0", len(list))
	}
}

func TestCmdProjectAddMissingArg(t *testing.T) {
	seedDataHome(t)
	if code := cmdProject([]string{"add"}); code == 0 {
		t.Errorf("project add with no path: exit 0, want non-zero")
	}
}

func TestCmdProjectRmByNameAndNoMatch(t *testing.T) {
	seedDataHome(t)
	root := gogoRepo(t)
	if code := cmdProject([]string{"add", root}); code != 0 {
		t.Fatalf("setup add: exit %d", code)
	}
	// rm a no-match → non-zero.
	if code := cmdProject([]string{"rm", "nope"}); code == 0 {
		t.Errorf("rm no-match: exit 0, want non-zero")
	}
	// rm by name → 0.
	if code := cmdProject([]string{"rm", filepath.Base(root)}); code != 0 {
		t.Fatalf("rm by name: exit %d, want 0", code)
	}
}

func TestCmdProjectUnknownSubcommand(t *testing.T) {
	seedDataHome(t)
	if code := cmdProject([]string{"frobnicate"}); code == 0 {
		t.Errorf("unknown subcommand: exit 0, want non-zero")
	}
}

// TestCmdProjectReAddPreservesSettings: `gogo project add` on a repo already
// registered as source #1 is an idempotent no-op — it must NOT reset the source's
// customized cap (a re-add carries no flags to change it).
func TestCmdProjectReAddPreservesSettings(t *testing.T) {
	seedDataHome(t)
	root := gogoRepo(t)
	name := filepath.Base(root)

	if code := cmdProject([]string{"add", root}); code != 0 {
		t.Fatalf("add: exit %d", code)
	}
	// User customizes the source cap on disk (as the config screen would).
	if _, err := projects.AddSource(name, projects.Source{Path: root, Name: name, ConcurrentWorkItems: 3, Color: "#abc"}); err != nil {
		t.Fatal(err)
	}
	// Re-add the SAME repo → idempotent no-op that preserves cap 3 / color.
	if code := cmdProject([]string{"add", root}); code != 0 {
		t.Fatalf("re-add: exit %d", code)
	}
	list, _ := projects.List()
	if len(list) != 1 || len(list[0].Sources) != 1 {
		t.Fatalf("re-add changed the shape: %+v", list)
	}
	if s := list[0].Sources[0]; s.ConcurrentWorkItems != 3 || s.Color != "#abc" {
		t.Errorf("re-add clobbered the source: %+v, want cap=3 color=#abc", s)
	}
}

// TestCmdProjectAddAutoInitializesHome (FR22): `gogo project add` is forgiving —
// registering a project without an explicit `gogo global init` still initializes the
// global cockpit home (writes the ~/.gogo/config.json marker), so the cockpit
// becomes available.
func TestCmdProjectAddAutoInitializesHome(t *testing.T) {
	seedDataHome(t)
	root := gogoRepo(t)

	if projects.Initialized() {
		t.Fatal("fresh home should not be initialized before any add")
	}
	if code := cmdProject([]string{"add", root}); code != 0 {
		t.Fatalf("project add: exit %d, want 0", code)
	}
	if !projects.Initialized() {
		t.Error("project add did not auto-initialize the global cockpit home (FR22)")
	}
}

func TestCapLabel(t *testing.T) {
	if capLabel(0) != "∞" || capLabel(3) != "3" {
		t.Errorf("capLabel(0)=%q capLabel(3)=%q, want ∞ / 3", capLabel(0), capLabel(3))
	}
}

func TestFormatProjectsEmpty(t *testing.T) {
	out := FormatProjects(nil)
	if !strings.Contains(out, "0 project") || !strings.Contains(out, "gogo project add") {
		t.Errorf("empty projects table missing guidance:\n%s", out)
	}
}
