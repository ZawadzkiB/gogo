package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedHome points the whole registry at a fresh t.TempDir() via the
// GOGO_CONFIG_HOME seam, so no test ever touches the real ~/.config.
func seedHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GOGO_CONFIG_HOME", dir)
	return dir
}

func TestPathHonorsGogoConfigHome(t *testing.T) {
	dir := seedHome(t)
	want := filepath.Join(dir, "projects.json")
	if got := Path(); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestHomePrefersGogoOverXDG(t *testing.T) {
	// GOGO_CONFIG_HOME wins over XDG_CONFIG_HOME.
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	t.Setenv("GOGO_CONFIG_HOME", "/seam")
	if got := Home(); got != "/seam" {
		t.Errorf("Home() = %q, want the GOGO_CONFIG_HOME seam", got)
	}
	// With no seam, XDG_CONFIG_HOME/gogo is used.
	os.Unsetenv("GOGO_CONFIG_HOME")
	if got := Home(); got != filepath.Join("/xdg", "gogo") {
		t.Errorf("Home() = %q, want %q", got, filepath.Join("/xdg", "gogo"))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	seedHome(t)
	in := &File{Projects: []Project{
		{Name: "alpha", Path: "/repos/alpha", Color: "#7aa8ff"},
		{Name: "beta", Path: "/repos/beta"},
	}}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Schema != Schema {
		t.Errorf("schema = %d, want %d", got.Schema, Schema)
	}
	if len(got.Projects) != 2 {
		t.Fatalf("projects = %d, want 2", len(got.Projects))
	}
	if got.Projects[0] != in.Projects[0] || got.Projects[1] != in.Projects[1] {
		t.Errorf("round-trip mismatch: %+v", got.Projects)
	}
}

// TestMaxConcurrentRoundTrips: the per-project cap survives Save→Load, and a set
// cap is written to disk (so the config screen / `gogo project add` persist it).
func TestMaxConcurrentRoundTrips(t *testing.T) {
	dir := seedHome(t)
	in := &File{Projects: []Project{
		{Name: "capped", Path: "/repos/capped", MaxConcurrent: 2},
		{Name: "uncapped", Path: "/repos/uncapped"}, // cap 0 = unlimited
	}}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Projects[0].MaxConcurrent != 2 {
		t.Errorf("capped project MaxConcurrent = %d, want 2", got.Projects[0].MaxConcurrent)
	}
	if got.Projects[1].MaxConcurrent != 0 {
		t.Errorf("uncapped project MaxConcurrent = %d, want 0 (unlimited)", got.Projects[1].MaxConcurrent)
	}
	// The set cap is serialized; the zero cap is elided (omitempty).
	raw, err := os.ReadFile(filepath.Join(dir, "projects.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"maxConcurrent": 2`) {
		t.Errorf("saved file does not carry the cap:\n%s", raw)
	}
}

// TestMaxConcurrentAbsentFieldIsZero: a Phase-1 registry file (no maxConcurrent
// key) loads with cap 0 — the fallback-preserving sentinel (existing entries stay
// unlimited, byte-for-byte as today).
func TestMaxConcurrentAbsentFieldIsZero(t *testing.T) {
	dir := seedHome(t)
	phase1 := `{"schema":1,"projects":[{"name":"legacy","path":"/repos/legacy","color":"#abc"}]}`
	if err := os.WriteFile(filepath.Join(dir, "projects.json"), []byte(phase1), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Projects) != 1 || got.Projects[0].MaxConcurrent != 0 {
		t.Errorf("Phase-1 file → %+v, want one project with MaxConcurrent 0", got.Projects)
	}
}

// TestZeroCapOmitEmptyKeepsShape: a zero-cap project serializes WITHOUT a
// maxConcurrent key, so a registry written after this change is byte-identical to
// the Phase-1 shape when no cap is set (omitempty guard).
func TestZeroCapOmitEmptyKeepsShape(t *testing.T) {
	dir := seedHome(t)
	if err := Save(&File{Projects: []Project{{Name: "z", Path: "/repos/z", Color: "#fff"}}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "projects.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "maxConcurrent") {
		t.Errorf("zero-cap project must omit maxConcurrent (omitempty):\n%s", raw)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	seedHome(t) // fresh dir — projects.json does not exist
	got, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file returned error: %v", err)
	}
	if len(got.Projects) != 0 {
		t.Errorf("missing file → %d projects, want 0", len(got.Projects))
	}
	if got.Schema != Schema {
		t.Errorf("missing file → schema %d, want %d", got.Schema, Schema)
	}
}

func TestLoadMalformedIsEmptyNoError(t *testing.T) {
	dir := seedHome(t)
	if err := os.WriteFile(filepath.Join(dir, "projects.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load on malformed file returned error (want graceful empty): %v", err)
	}
	if len(got.Projects) != 0 {
		t.Errorf("malformed file → %d projects, want 0", len(got.Projects))
	}
}

func TestAddDedupesByPath(t *testing.T) {
	seedHome(t)
	if added, err := Add(Project{Name: "a", Path: "/repos/a"}); err != nil || !added {
		t.Fatalf("first Add: added=%v err=%v", added, err)
	}
	// Same path, new name → replace in place, not append.
	if added, err := Add(Project{Name: "a-renamed", Path: "/repos/a", Color: "#fff"}); err != nil || added {
		t.Fatalf("dedupe Add: added=%v (want false) err=%v", added, err)
	}
	list, _ := List()
	if len(list) != 1 {
		t.Fatalf("after dedupe: %d projects, want 1", len(list))
	}
	if list[0].Name != "a-renamed" || list[0].Color != "#fff" {
		t.Errorf("dedupe did not update in place: %+v", list[0])
	}
	// A distinct path appends.
	if added, _ := Add(Project{Name: "b", Path: "/repos/b"}); !added {
		t.Errorf("distinct path Add: added=false, want true")
	}
	if list, _ := List(); len(list) != 2 {
		t.Errorf("after distinct Add: %d projects, want 2", len(list))
	}
}

func TestRemoveByNameAndByPath(t *testing.T) {
	seedHome(t)
	Add(Project{Name: "alpha", Path: "/repos/alpha"})
	Add(Project{Name: "beta", Path: "/repos/beta"})
	Add(Project{Name: "gamma", Path: "/repos/gamma"})

	// Remove by name.
	if removed, err := Remove("beta"); err != nil || !removed {
		t.Fatalf("Remove by name: removed=%v err=%v", removed, err)
	}
	// Remove by path.
	if removed, err := Remove("/repos/gamma"); err != nil || !removed {
		t.Fatalf("Remove by path: removed=%v err=%v", removed, err)
	}
	list, _ := List()
	if len(list) != 1 || list[0].Name != "alpha" {
		t.Fatalf("after removes: %+v, want [alpha]", list)
	}
	// A no-match remove is a graceful no-op.
	if removed, err := Remove("does-not-exist"); err != nil || removed {
		t.Errorf("Remove no-match: removed=%v err=%v (want false,nil)", removed, err)
	}
}
