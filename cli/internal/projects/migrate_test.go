package projects

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/config"
)

// seedHomes points BOTH the legacy config home (GOGO_CONFIG_HOME) and the new
// data home (GOGO_DATA_HOME) at fresh temp dirs, and returns them.
func seedHomes(t *testing.T) (configHome, dataHome string) {
	t.Helper()
	configHome = t.TempDir()
	dataHome = t.TempDir()
	t.Setenv("GOGO_CONFIG_HOME", configHome)
	t.Setenv("GOGO_DATA_HOME", dataHome)
	return configHome, dataHome
}

// TestMigrateConvertsLegacyRegistry: a legacy flat projects.json is converted into
// single-source home-folder projects, carrying the per-project MaxConcurrent onto
// the source's ConcurrentWorkItems.
func TestMigrateConvertsLegacyRegistry(t *testing.T) {
	seedHomes(t)
	// Seed the legacy flat registry.
	if _, err := config.Add(config.Project{Name: "gogo", Path: "/repos/gogo", MaxConcurrent: 2, Color: "#abc"}); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Add(config.Project{Name: "svc", Path: "/repos/svc"}); err != nil {
		t.Fatal(err)
	}

	Migrate()

	list, _ := List()
	if len(list) != 2 {
		t.Fatalf("after migrate: %d projects, want 2", len(list))
	}
	byname := map[string]Project{}
	for _, p := range list {
		byname[p.Name] = p
	}
	g, ok := byname["gogo"]
	if !ok || len(g.Sources) != 1 {
		t.Fatalf("migrated gogo = %+v, want one source", g)
	}
	if s := g.Sources[0]; s.Path != "/repos/gogo" || s.ConcurrentWorkItems != 2 || s.Color != "#abc" {
		t.Errorf("migrated source = %+v, want {path=/repos/gogo cap=2 color=#abc}", s)
	}
	if s := byname["svc"].Sources[0]; s.ConcurrentWorkItems != 0 {
		t.Errorf("uncapped legacy → cap %d, want 0", s.ConcurrentWorkItems)
	}
}

// TestMigrateIdempotentAndNonDestructive: a second Migrate is a no-op (the store
// already exists), and the legacy projects.json is LEFT IN PLACE (non-destructive).
func TestMigrateIdempotentAndNonDestructive(t *testing.T) {
	configHome, _ := seedHomes(t)
	config.Add(config.Project{Name: "gogo", Path: "/repos/gogo", MaxConcurrent: 1})

	Migrate()
	first, _ := List()

	// Mutate the store, then Migrate again → must NOT re-run (idempotent latch).
	Add(Project{Name: "manual", Sources: []Source{{Path: "/repos/manual"}}})
	Migrate()
	second, _ := List()
	if len(second) != len(first)+1 {
		t.Errorf("second Migrate re-ran: %d projects, want %d (idempotent no-op after the manual add)",
			len(second), len(first)+1)
	}

	// Non-destructive: the legacy registry file still exists.
	if _, err := os.Stat(filepath.Join(configHome, "projects.json")); err != nil {
		t.Errorf("legacy projects.json was removed (want non-destructive): %v", err)
	}
}

// TestMigrateNoLegacyIsNoop: with no legacy registry, Migrate creates nothing.
func TestMigrateNoLegacyIsNoop(t *testing.T) {
	_, dataHome := seedHomes(t)
	Migrate()
	if _, err := os.Stat(filepath.Join(dataHome, "projects")); !os.IsNotExist(err) {
		t.Errorf("Migrate with no legacy created the store: %v", err)
	}
	if list, _ := List(); len(list) != 0 {
		t.Errorf("Migrate with no legacy → %d projects, want 0", len(list))
	}
}

// TestMigrateFoldsSameBasename: two legacy repos sharing a basename fold into one
// project with two sources (deduped by path), never clobbering.
func TestMigrateFoldsSameBasename(t *testing.T) {
	seedHomes(t)
	config.Add(config.Project{Name: "", Path: "/repos/one/app", MaxConcurrent: 1})
	config.Add(config.Project{Name: "", Path: "/repos/two/app"})
	Migrate()
	list, _ := List()
	if len(list) != 1 || list[0].Name != "app" {
		t.Fatalf("folded projects = %+v, want one named app", list)
	}
	if len(list[0].Sources) != 2 {
		t.Errorf("folded sources = %d, want 2", len(list[0].Sources))
	}
}
