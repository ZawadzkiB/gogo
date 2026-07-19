package contract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// writeFeature drops a minimal feature folder (state.md) under root's .gogo/work.
func writeFeature(t *testing.T, root, slug, created, phase, status string) {
	t.Helper()
	dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "- **feature:** " + slug + "\n" +
		"- **phase:** " + phase + "\n" +
		"- **status:** " + status + "\n" +
		"- **created:** " + created + "\n"
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRepoStampsRoot(t *testing.T) {
	root := t.TempDir()
	writeFeature(t, root, "one", "2026-07-10", "implement", "implementing")
	repo, err := LoadRepo(root)
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	if len(repo.Features) != 1 {
		t.Fatalf("features = %d, want 1", len(repo.Features))
	}
	if repo.Features[0].Root != root {
		t.Errorf("Root = %q, want %q", repo.Features[0].Root, root)
	}
	// Single-repo mode leaves Source empty (card tag invisible → fallback parity).
	if repo.Features[0].Source != "" {
		t.Errorf("single-repo Source = %q, want empty", repo.Features[0].Source)
	}
}

// TestLoadProjectAggregatesSources: LoadProject merges every source newest-first,
// stamps each feature's Source label + Root, and an empty/unreadable source
// contributes nothing (no crash) — the corrected multi-source board reader.
func TestLoadProjectAggregatesSources(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	bare := t.TempDir() // no .gogo/ at all → contributes nothing
	writeFeature(t, rootA, "alpha-old", "2026-07-01", "implement", "implementing")
	writeFeature(t, rootB, "beta-new", "2026-07-14", "plan", "plan-accepted")

	repo := LoadProject(projects.Project{
		Name: "gogo",
		Sources: []projects.Source{
			{Path: rootA, Name: "svc-a"},
			{Path: rootB, Name: "svc-b"},
			{Path: bare, Name: "svc-bare"},
		},
	})

	if len(repo.Features) != 2 {
		t.Fatalf("merged features = %d, want 2 (bare source skipped)", len(repo.Features))
	}
	// Newest-first across sources.
	if repo.Features[0].Slug != "beta-new" || repo.Features[1].Slug != "alpha-old" {
		t.Errorf("merge order = [%s, %s], want [beta-new, alpha-old]",
			repo.Features[0].Slug, repo.Features[1].Slug)
	}
	if repo.Root != "" {
		t.Errorf("project-board Root = %q, want empty (each feature carries its own)", repo.Root)
	}
	byslug := map[string]*Feature{}
	for _, f := range repo.Features {
		byslug[f.Slug] = f
	}
	if got := byslug["beta-new"]; got.Source != "svc-b" || got.Root != rootB {
		t.Errorf("beta-new stamped {Source:%q Root:%q}, want {svc-b %q}", got.Source, got.Root, rootB)
	}
	if got := byslug["alpha-old"]; got.Source != "svc-a" || got.Root != rootA {
		t.Errorf("alpha-old stamped {Source:%q Root:%q}, want {svc-a %q}", got.Source, got.Root, rootA)
	}
}

// TestLoadProjectDefaultsSourceNameToBasename: a source with no Name is tagged by
// its path basename.
func TestLoadProjectDefaultsSourceNameToBasename(t *testing.T) {
	root := t.TempDir()
	writeFeature(t, root, "solo", "2026-07-10", "implement", "implementing")
	repo := LoadProject(projects.Project{Sources: []projects.Source{{Path: root}}}) // no source Name
	if len(repo.Features) != 1 {
		t.Fatalf("features = %d, want 1", len(repo.Features))
	}
	if want := filepath.Base(root); repo.Features[0].Source != want {
		t.Errorf("Source = %q, want basename %q", repo.Features[0].Source, want)
	}
}

// TestLoadProjectEmpty: a project with no sources (or all unreadable) yields an
// empty board, never a crash.
func TestLoadProjectEmpty(t *testing.T) {
	if repo := LoadProject(projects.Project{Name: "empty"}); repo == nil || len(repo.Features) != 0 {
		t.Errorf("empty project → %v, want a non-nil board with no features", repo)
	}
}
