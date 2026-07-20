package projects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedDataHome points the whole store at a fresh t.TempDir() via the
// GOGO_DATA_HOME seam, so no test ever touches the real ~/.gogo.
func seedDataHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GOGO_DATA_HOME", dir)
	return dir
}

func TestHomeHonorsGogoDataHome(t *testing.T) {
	t.Setenv("GOGO_DATA_HOME", "/seam")
	if got := Home(); got != "/seam" {
		t.Errorf("Home() = %q, want the GOGO_DATA_HOME seam", got)
	}
	os.Unsetenv("GOGO_DATA_HOME")
	t.Setenv("HOME", "/home/u")
	if got := Home(); got != filepath.Join("/home/u", ".gogo") {
		t.Errorf("Home() = %q, want ~/.gogo", got)
	}
}

// TestEnsureHomeAndInitialized: the global-home marker (~/.gogo/config.json) drives
// Initialized(); EnsureHome() writes it (creating ~/.gogo/projects/) and is
// idempotent — created=true the first time, created=false thereafter, never leaving
// the home uninitialized. FR19/FR22.
func TestEnsureHomeAndInitialized(t *testing.T) {
	seedDataHome(t)

	if Initialized() {
		t.Fatal("a fresh home should not be initialized")
	}
	created, err := EnsureHome()
	if err != nil || !created {
		t.Fatalf("first EnsureHome: created=%v err=%v, want true/nil", created, err)
	}
	if !Initialized() {
		t.Error("EnsureHome did not mark the home initialized")
	}
	if _, err := os.Stat(HomeConfigPath()); err != nil {
		t.Errorf("EnsureHome did not write %s: %v", HomeConfigPath(), err)
	}
	if info, err := os.Stat(ProjectsDir()); err != nil || !info.IsDir() {
		t.Errorf("EnsureHome did not create %s: err=%v", ProjectsDir(), err)
	}

	// Idempotent: a second call is a no-op (created=false), still initialized.
	created, err = EnsureHome()
	if err != nil || created {
		t.Errorf("second EnsureHome: created=%v err=%v, want false/nil", created, err)
	}
	if !Initialized() {
		t.Error("second EnsureHome un-initialized the home")
	}
}

func TestDirLayout(t *testing.T) {
	dir := seedDataHome(t)
	if got, want := ProjectsDir(), filepath.Join(dir, "projects"); got != want {
		t.Errorf("ProjectsDir() = %q, want %q", got, want)
	}
	if got, want := configPath("gogo"), filepath.Join(dir, "projects", "gogo", "config.json"); got != want {
		t.Errorf("configPath = %q, want %q", got, want)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	seedDataHome(t)
	in := &Project{
		Name:        "gogo",
		Description: "the cockpit",
		Sources: []Source{
			{Path: "/repos/gogo", Name: "gogo", MainBranch: "main", ConcurrentWorkItems: 2, Color: "#7aa8ff"},
			{Path: "/repos/svc", Name: "svc"},
		},
	}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load("gogo")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Schema != Schema {
		t.Errorf("schema = %d, want %d", got.Schema, Schema)
	}
	if got.Name != "gogo" || got.Description != "the cockpit" || len(got.Sources) != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Sources[0] != in.Sources[0] || got.Sources[1] != in.Sources[1] {
		t.Errorf("source round-trip mismatch: %+v", got.Sources)
	}
}

func TestLoadMissingIsEmptyNoError(t *testing.T) {
	seedDataHome(t)
	got, err := Load("ghost")
	if err != nil {
		t.Fatalf("Load on missing returned error: %v", err)
	}
	if got.Name != "ghost" || len(got.Sources) != 0 {
		t.Errorf("missing project → %+v, want empty named ghost", got)
	}
}

func TestLoadMalformedIsEmptyNoError(t *testing.T) {
	dir := seedDataHome(t)
	pdir := filepath.Join(dir, "projects", "bad")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "config.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load("bad")
	if err != nil {
		t.Fatalf("Load on malformed returned error (want graceful empty): %v", err)
	}
	if len(got.Sources) != 0 {
		t.Errorf("malformed → %d sources, want 0", len(got.Sources))
	}
}

func TestListMissingStoreIsEmpty(t *testing.T) {
	seedDataHome(t) // fresh dir — no projects/ yet
	got, err := List()
	if err != nil {
		t.Fatalf("List on missing store returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("missing store → %d projects, want 0", len(got))
	}
}

func TestListNameSortedSkipsStrayDirs(t *testing.T) {
	dir := seedDataHome(t)
	Add(Project{Name: "beta", Sources: []Source{{Path: "/b"}}})
	Add(Project{Name: "alpha", Sources: []Source{{Path: "/a"}}})
	// A stray dir with NO config.json must be skipped.
	if err := os.MkdirAll(filepath.Join(dir, "projects", "stray"), 0o755); err != nil {
		t.Fatal(err)
	}
	list, _ := List()
	if len(list) != 2 || list[0].Name != "alpha" || list[1].Name != "beta" {
		t.Fatalf("List = %+v, want [alpha beta] (stray skipped, name-sorted)", list)
	}
}

// TestAddSourceDedupesByPath: appending a source with an existing path updates it
// in place rather than duplicating.
func TestAddSourceDedupesByPath(t *testing.T) {
	seedDataHome(t)
	if added, err := AddSource("gogo", Source{Path: "/repos/a", Name: "a"}); err != nil || !added {
		t.Fatalf("first AddSource: added=%v err=%v", added, err)
	}
	// Same path, new name + cap → replace in place, not append.
	if added, err := AddSource("gogo", Source{Path: "/repos/a", Name: "a2", ConcurrentWorkItems: 3}); err != nil || added {
		t.Fatalf("dedupe AddSource: added=%v (want false) err=%v", added, err)
	}
	p, _ := Load("gogo")
	if len(p.Sources) != 1 {
		t.Fatalf("after dedupe: %d sources, want 1", len(p.Sources))
	}
	if p.Sources[0].Name != "a2" || p.Sources[0].ConcurrentWorkItems != 3 {
		t.Errorf("dedupe did not update in place: %+v", p.Sources[0])
	}
	// A distinct path appends.
	if added, _ := AddSource("gogo", Source{Path: "/repos/b", Name: "b"}); !added {
		t.Errorf("distinct path AddSource: added=false, want true")
	}
	if p, _ := Load("gogo"); len(p.Sources) != 2 {
		t.Errorf("after distinct AddSource: %d sources, want 2", len(p.Sources))
	}
}

func TestRemoveSourceByNameAndPath(t *testing.T) {
	seedDataHome(t)
	AddSource("p", Source{Path: "/repos/a", Name: "alpha"})
	AddSource("p", Source{Path: "/repos/b", Name: "beta"})
	AddSource("p", Source{Path: "/repos/c", Name: "gamma"})

	if removed, err := RemoveSource("p", "beta"); err != nil || !removed { // by name
		t.Fatalf("RemoveSource by name: removed=%v err=%v", removed, err)
	}
	if removed, err := RemoveSource("p", "/repos/c"); err != nil || !removed { // by path
		t.Fatalf("RemoveSource by path: removed=%v err=%v", removed, err)
	}
	p, _ := Load("p")
	if len(p.Sources) != 1 || p.Sources[0].Name != "alpha" {
		t.Fatalf("after removes: %+v, want [alpha]", p.Sources)
	}
	if removed, err := RemoveSource("p", "nope"); err != nil || removed {
		t.Errorf("RemoveSource no-match: removed=%v err=%v (want false,nil)", removed, err)
	}
}

// TestAddDedupesByName: Add on an existing project name updates in place
// (added=false), a new name creates (added=true).
func TestAddDedupesByName(t *testing.T) {
	seedDataHome(t)
	if added, err := Add(Project{Name: "gogo", Sources: []Source{{Path: "/a"}}}); err != nil || !added {
		t.Fatalf("first Add: added=%v err=%v", added, err)
	}
	if added, err := Add(Project{Name: "gogo", Sources: []Source{{Path: "/b"}}}); err != nil || added {
		t.Fatalf("re-Add same name: added=%v (want false) err=%v", added, err)
	}
	if list, _ := List(); len(list) != 1 {
		t.Errorf("dedupe by name: %d projects, want 1", len(list))
	}
}

// TestRemoveDeletesHomeFolderOnly: Remove deletes the project's home folder and
// is a graceful no-op when absent.
func TestRemoveDeletesHomeFolder(t *testing.T) {
	dir := seedDataHome(t)
	Add(Project{Name: "gogo", Sources: []Source{{Path: "/a"}}})
	if _, err := os.Stat(filepath.Join(dir, "projects", "gogo")); err != nil {
		t.Fatalf("home folder not created: %v", err)
	}
	if removed, err := Remove("gogo"); err != nil || !removed {
		t.Fatalf("Remove: removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "projects", "gogo")); !os.IsNotExist(err) {
		t.Errorf("home folder survived Remove: %v", err)
	}
	if removed, err := Remove("gogo"); err != nil || removed {
		t.Errorf("Remove absent: removed=%v err=%v (want false,nil)", removed, err)
	}
}

// TestInvalidNameGuards: names with path separators / traversal are refused by
// Save (write-scope guard) and never escape the store.
func TestInvalidNameGuards(t *testing.T) {
	seedDataHome(t)
	for _, bad := range []string{"", ".", "..", "a/b", "../evil", `a\b`} {
		if err := Save(&Project{Name: bad}); err == nil {
			t.Errorf("Save(name=%q) = nil, want an error (invalid name)", bad)
		}
		if removed, _ := Remove(bad); removed {
			t.Errorf("Remove(name=%q) removed something, want a refused no-op", bad)
		}
	}
}

func TestAllSourcesFlattens(t *testing.T) {
	projs := []Project{
		{Name: "a", Sources: []Source{{Path: "/1"}, {Path: "/2"}}},
		{Name: "b", Sources: []Source{{Path: "/3"}}},
	}
	got := AllSources(projs)
	if len(got) != 3 {
		t.Fatalf("AllSources = %d, want 3", len(got))
	}
	paths := got[0].Path + "," + got[1].Path + "," + got[2].Path
	if !strings.Contains(paths, "/1") || !strings.Contains(paths, "/3") {
		t.Errorf("AllSources missing paths: %v", got)
	}
}

// TestEnsureProjectHomeScaffolds (FR2): EnsureProjectHome creates the project dir,
// its .knowledge/ (seeded with project-knowledge.md) and its .gogo/plans/ dir — writing
// ONLY under ~/.gogo/. It is idempotent and NEVER clobbers an edited knowledge file.
func TestEnsureProjectHomeScaffolds(t *testing.T) {
	seedDataHome(t)
	if err := EnsureProjectHome("sanoma"); err != nil {
		t.Fatalf("EnsureProjectHome: %v", err)
	}
	kf := filepath.Join(KnowledgeDir("sanoma"), "project-knowledge.md")
	raw, err := os.ReadFile(kf)
	if err != nil {
		t.Fatalf("seed knowledge not written: %v", err)
	}
	// The seeded template carries the project name + the four headed sections.
	for _, want := range []string{"Project knowledge - sanoma", "## Domain", "How the sources connect", "## Glossary", "Integration contracts"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("seeded template missing %q:\n%s", want, raw)
		}
	}
	// .gogo/plans/ scaffolded for the CLI-owned plan store.
	if info, err := os.Stat(filepath.Join(Dir("sanoma"), ".gogo", "plans")); err != nil || !info.IsDir() {
		t.Errorf(".gogo/plans/ not scaffolded: err=%v", err)
	}
	// Idempotent + non-clobber: edit the file, re-run, the edit survives.
	if err := os.WriteFile(kf, []byte("MY EDITS"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureProjectHome("sanoma"); err != nil {
		t.Fatalf("second EnsureProjectHome: %v", err)
	}
	if raw, _ := os.ReadFile(kf); string(raw) != "MY EDITS" {
		t.Errorf("EnsureProjectHome clobbered an edited knowledge file: %q", raw)
	}
}

// TestSeedProjectKnowledgeIdempotent (FR2): SeedProjectKnowledge writes the template
// only when absent; a second call is a no-op that preserves a hand-authored file.
func TestSeedProjectKnowledgeIdempotent(t *testing.T) {
	seedDataHome(t)
	if err := SeedProjectKnowledge("app"); err != nil {
		t.Fatal(err)
	}
	kf := filepath.Join(KnowledgeDir("app"), "project-knowledge.md")
	if err := os.WriteFile(kf, []byte("hand written"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SeedProjectKnowledge("app"); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(kf); string(raw) != "hand written" {
		t.Errorf("SeedProjectKnowledge clobbered an existing file: %q", raw)
	}
}

// TestScaffoldRefusesBadName: EnsureProjectHome / SeedProjectKnowledge refuse an
// unsafe name (the write-scope guard) so a `..`/separator name can never escape the store.
func TestScaffoldRefusesBadName(t *testing.T) {
	seedDataHome(t)
	for _, bad := range []string{"", ".", "..", "a/b", "../evil", `a\b`} {
		if err := EnsureProjectHome(bad); err == nil {
			t.Errorf("EnsureProjectHome(%q) = nil, want an error (invalid name)", bad)
		}
		if err := SeedProjectKnowledge(bad); err == nil {
			t.Errorf("SeedProjectKnowledge(%q) = nil, want an error (invalid name)", bad)
		}
	}
}

// TestExistsReportsRegistered: Exists is true only once a project's config.json exists;
// a bad name is always false.
func TestExistsReportsRegistered(t *testing.T) {
	seedDataHome(t)
	if Exists("ghost") {
		t.Error("Exists(ghost) = true before any add")
	}
	if _, err := Add(Project{Name: "app", Sources: []Source{{Path: "/a"}}}); err != nil {
		t.Fatal(err)
	}
	if !Exists("app") {
		t.Error("Exists(app) = false after Add")
	}
	if Exists("../evil") {
		t.Error("Exists on a traversal name = true, want false")
	}
}

// TestSkipForSource (FR4, REV-001): the per-source gate-skip flags resolve by exact Path
// match — a flagged source returns its (planSkip, uatSkip); an unflagged source or an
// UNREGISTERED root returns (false, false), the fallback that keeps an unregistered /
// single repo's gates byte-for-byte. Mirrors CapForSource's resolve-by-path discipline so
// both `gogo go` launch paths share this one resolver.
func TestSkipForSource(t *testing.T) {
	sources := []Source{
		{Name: "flagged", Path: "/repos/flagged", PlanAcceptanceSkip: true, UatAcceptanceSkip: true},
		{Name: "plan-only", Path: "/repos/plan-only", PlanAcceptanceSkip: true},
		{Name: "uat-only", Path: "/repos/uat-only", UatAcceptanceSkip: true},
		{Name: "plain", Path: "/repos/plain"},
	}
	cases := []struct {
		root              string
		wantPlan, wantUAT bool
	}{
		{"/repos/flagged", true, true},
		{"/repos/plan-only", true, false},
		{"/repos/uat-only", false, true},
		{"/repos/plain", false, false},
		{"/repos/unregistered", false, false},
	}
	for _, c := range cases {
		gotPlan, gotUAT := SkipForSource(sources, c.root)
		if gotPlan != c.wantPlan || gotUAT != c.wantUAT {
			t.Errorf("SkipForSource(%s) = (%v,%v), want (%v,%v)", c.root, gotPlan, gotUAT, c.wantPlan, c.wantUAT)
		}
	}
	// nil sources (single-repo, no store) → never a skip (byte-for-byte fallback).
	if p, u := SkipForSource(nil, "/repos/anything"); p || u {
		t.Errorf("SkipForSource(nil) = (%v,%v), want (false,false)", p, u)
	}
}

// TestSkipFlagsOmitemptyRoundTrip (FR4, REV-001): the two additive gate-skip flags
// round-trip through config.json when set, and a FALSE flag OMITS its JSON key (omitempty)
// so an opted-in-nowhere source keeps the on-disk shape byte-for-byte — schema stays 1.
func TestSkipFlagsOmitemptyRoundTrip(t *testing.T) {
	dir := seedDataHome(t)
	// Both flags false → neither key on disk.
	if err := Save(&Project{Name: "off", Sources: []Source{{Path: "/a", Name: "a"}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "projects", "off", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "planAcceptanceSkip") || strings.Contains(string(raw), "uatAcceptanceSkip") {
		t.Errorf("a false skip flag must omit its JSON key (omitempty):\n%s", raw)
	}
	// Both flags true → round-trip, schema unchanged.
	in := &Project{Name: "on", Sources: []Source{{Path: "/a", Name: "a", PlanAcceptanceSkip: true, UatAcceptanceSkip: true}}}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	got, _ := Load("on")
	if got.Schema != Schema {
		t.Errorf("schema = %d, want %d (additive fields must not bump it)", got.Schema, Schema)
	}
	if !got.Sources[0].PlanAcceptanceSkip || !got.Sources[0].UatAcceptanceSkip {
		t.Errorf("skip flags did not round-trip: %+v", got.Sources[0])
	}
}

// TestZeroCapOmitsField: a zero (unlimited) cap serializes WITHOUT the
// concurrentWorkItems key (omitempty), keeping the on-disk shape minimal.
func TestZeroCapOmitsField(t *testing.T) {
	dir := seedDataHome(t)
	if err := Save(&Project{Name: "p", Sources: []Source{{Path: "/a", Name: "a"}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "projects", "p", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "concurrentWorkItems") {
		t.Errorf("zero-cap source must omit concurrentWorkItems (omitempty):\n%s", raw)
	}
}
