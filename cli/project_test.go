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

// TestCmdProjectAddAssignsColors (cockpit-colors FR2): `gogo project add` writes a
// non-blank palette color for the PROJECT and its source #1, both real palette swatches,
// persisted to config.json.
func TestCmdProjectAddAssignsColors(t *testing.T) {
	seedDataHome(t)
	root := gogoRepo(t)
	if code := cmdProject([]string{"add", root}); code != 0 {
		t.Fatalf("project add: exit %d", code)
	}
	p, _ := projects.Load(filepath.Base(root))
	if p.Color == "" {
		t.Error("project add left the project color blank (FR2)")
	}
	if _, ok := projects.LookupSwatch(p.Color); !ok {
		t.Errorf("project color %q is not a palette swatch", p.Color)
	}
	if len(p.Sources) != 1 || p.Sources[0].Color == "" {
		t.Fatalf("project add left source #1 colorless: %+v", p.Sources)
	}
	if _, ok := projects.LookupSwatch(p.Sources[0].Color); !ok {
		t.Errorf("source color %q is not a palette swatch", p.Sources[0].Color)
	}
}

// TestCmdSourceAddAssignsNextFreeColor (cockpit-colors FR2): a second source added to a
// project gets a DIFFERENT (next-free) palette color, and a re-add preserves it.
func TestCmdSourceAddAssignsNextFreeColor(t *testing.T) {
	seedDataHome(t)
	repo1 := gogoRepo(t)
	repo2 := gogoRepo(t)
	if code := cmdProject([]string{"add", repo1, "--name", "gogo"}); code != 0 {
		t.Fatalf("project add: exit %d", code)
	}
	if code := cmdSource([]string{"add", repo2}); code != 0 {
		t.Fatalf("source add: exit %d", code)
	}
	p, _ := projects.Load("gogo")
	if len(p.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(p.Sources))
	}
	c1, c2 := p.Sources[0].Color, p.Sources[1].Color
	if c1 == "" || c2 == "" {
		t.Fatalf("a source has no color: %+v", p.Sources)
	}
	if c1 == c2 {
		t.Errorf("both sources share color %q — AssignColor should skip taken", c1)
	}
	// Re-add repo2 → its color is preserved (never churned).
	if code := cmdSource([]string{"add", repo2}); code != 0 {
		t.Fatalf("source re-add: exit %d", code)
	}
	p, _ = projects.Load("gogo")
	if got := p.Sources[1].Color; got != c2 {
		t.Errorf("re-add churned the source color: %q, want preserved %q", got, c2)
	}
}

// TestCmdProjectAddBareNameEmpty (FR1, D=A): a bare NAME creates an EMPTY project —
// config.json with sources: [], the .knowledge/ seed + .gogo/plans/ scaffold, no repo
// and no .gogo/ required. This is the primary flow (`gogo project add sanoma`).
func TestCmdProjectAddBareNameEmpty(t *testing.T) {
	seedDataHome(t)
	if code := cmdProject([]string{"add", "sanoma"}); code != 0 {
		t.Fatalf("project add sanoma: exit %d, want 0", code)
	}
	list, _ := projects.List()
	if len(list) != 1 || list[0].Name != "sanoma" {
		t.Fatalf("after bare-name add: %+v, want one project named sanoma", list)
	}
	if len(list[0].Sources) != 0 {
		t.Errorf("empty project has %d sources, want 0", len(list[0].Sources))
	}
	// config.json carries sources: [] (not null).
	raw, err := os.ReadFile(filepath.Join(projects.Dir("sanoma"), "config.json"))
	if err != nil {
		t.Fatalf("config.json not written: %v", err)
	}
	if !strings.Contains(string(raw), `"sources": []`) {
		t.Errorf("empty project config.json should carry sources: []:\n%s", raw)
	}
	// FR2 scaffold: .knowledge/project-knowledge.md + .gogo/plans/.
	if _, err := os.Stat(filepath.Join(projects.KnowledgeDir("sanoma"), "project-knowledge.md")); err != nil {
		t.Errorf(".knowledge/project-knowledge.md not scaffolded: %v", err)
	}
	if info, err := os.Stat(filepath.Join(projects.Dir("sanoma"), ".gogo", "plans")); err != nil || !info.IsDir() {
		t.Errorf(".gogo/plans/ not scaffolded: %v", err)
	}
	// FR22 parity: the cockpit home is initialized by an empty add too.
	if !projects.Initialized() {
		t.Error("bare-name add did not initialize the cockpit home")
	}
}

// TestCmdProjectAddBareNameIdempotent (FR1): re-adding the same bare name is a friendly
// no-op (exit 0) that preserves the project and NEVER clobbers an edited knowledge file.
func TestCmdProjectAddBareNameIdempotent(t *testing.T) {
	seedDataHome(t)
	if code := cmdProject([]string{"add", "sanoma"}); code != 0 {
		t.Fatalf("first add: exit %d", code)
	}
	kf := filepath.Join(projects.KnowledgeDir("sanoma"), "project-knowledge.md")
	if err := os.WriteFile(kf, []byte("MY DOMAIN NOTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cmdProject([]string{"add", "sanoma"}); code != 0 {
		t.Fatalf("re-add: exit %d, want 0", code)
	}
	if list, _ := projects.List(); len(list) != 1 {
		t.Errorf("re-add created a duplicate: %d projects, want 1", len(list))
	}
	if raw, _ := os.ReadFile(kf); string(raw) != "MY DOMAIN NOTES" {
		t.Errorf("re-add clobbered the knowledge file: %q", raw)
	}
}

// TestCmdProjectAddBareNameWithSource (FR1): `--source <repo>` creates the named project
// AND links the repo as source #1 in one shot, still scaffolding the project knowledge.
func TestCmdProjectAddBareNameWithSource(t *testing.T) {
	seedDataHome(t)
	repo := gogoRepo(t)
	if code := cmdProject([]string{"add", "sanoma", "--source", repo}); code != 0 {
		t.Fatalf("add --source: exit %d, want 0", code)
	}
	p, _ := projects.Load("sanoma")
	if p.Name != "sanoma" || len(p.Sources) != 1 || p.Sources[0].Path != filepath.Clean(repo) {
		t.Fatalf("add --source built %+v, want sanoma with source %s", p, filepath.Clean(repo))
	}
	if p.Sources[0].ConcurrentWorkItems != projects.DefaultConcurrentWorkItems {
		t.Errorf("source cap = %d, want the default %d", p.Sources[0].ConcurrentWorkItems, projects.DefaultConcurrentWorkItems)
	}
	if _, err := os.Stat(filepath.Join(projects.KnowledgeDir("sanoma"), "project-knowledge.md")); err != nil {
		t.Errorf(".knowledge not scaffolded with --source: %v", err)
	}
}

// TestCmdProjectAddBadSourceRepo (FR1): `--source` pointing at a non-gogo dir errors and
// creates NOTHING (the bad repo is resolved before any project is written).
func TestCmdProjectAddBadSourceRepo(t *testing.T) {
	seedDataHome(t)
	bare := t.TempDir() // no .gogo/
	if code := cmdProject([]string{"add", "sanoma", "--source", bare}); code == 0 {
		t.Error("add --source <non-gogo>: exit 0, want non-zero")
	}
	if list, _ := projects.List(); len(list) != 0 {
		t.Errorf("a bad --source still wrote %d projects, want 0", len(list))
	}
}

// TestCmdProjectAddPathWithSourceRejected (FR1): combining a repo-PATH positional with
// --source is refused (the path is already source #1).
func TestCmdProjectAddPathWithSourceRejected(t *testing.T) {
	seedDataHome(t)
	repo := gogoRepo(t)
	other := gogoRepo(t)
	if code := cmdProject([]string{"add", repo, "--source", other}); code == 0 {
		t.Error("add <path> --source: exit 0, want a non-zero refusal")
	}
}

// TestCmdProjectAddDisambiguation (FR1): a token that LOOKS pathish (has a separator) is
// read as a PATH (→ the .gogo/ requirement), while a bare token that IS a real repo dir
// resolves to PATH mode (project+source), never a stray empty project.
func TestCmdProjectAddDisambiguation(t *testing.T) {
	seedDataHome(t)

	// (a) a pathish token → PATH mode → errors (no .gogo/), creates nothing.
	if code := cmdProject([]string{"add", "foo/bar"}); code == 0 {
		t.Error("add foo/bar (pathish): exit 0, want non-zero (PATH mode, no .gogo/)")
	}
	if list, _ := projects.List(); len(list) != 0 {
		t.Errorf("a pathish token wrote %d projects, want 0", len(list))
	}

	// (b) a bare token that resolves (in cwd) to a real repo dir → PATH mode.
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "myrepo", ".gogo", "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work) // so `myrepo` resolves relative to cwd
	if code := cmdProject([]string{"add", "myrepo"}); code != 0 {
		t.Fatalf("add myrepo (real repo dir): exit %d, want 0 (PATH mode)", code)
	}
	p, _ := projects.Load("myrepo")
	if len(p.Sources) != 1 {
		t.Errorf("a real-repo bare token should create source #1: %+v", p)
	}
}

// TestIsPathArg (FR1): the name-vs-path classifier — separators / ~ / . are PATHs, a
// plain token is a NAME.
func TestIsPathArg(t *testing.T) {
	paths := []string{"foo/bar", `foo\bar`, "~/repos/x", "./x", "..", ".", "/abs/path"}
	names := []string{"sanoma", "gogo", "my-project", "app123"}
	for _, p := range paths {
		if !isPathArg(p) {
			t.Errorf("isPathArg(%q) = false, want true (a PATH)", p)
		}
	}
	for _, n := range names {
		if isPathArg(n) {
			t.Errorf("isPathArg(%q) = true, want false (a NAME)", n)
		}
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
