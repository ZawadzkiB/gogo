package plans

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// TestMigrateLegacyFoldsDraftsAndEpics: legacy drafts + epics fold into project
// plans where the target source is resolvable; the run is idempotent (a 2nd run is
// a no-op) and non-destructive (the legacy files remain).
func TestMigrateLegacyFoldsDraftsAndEpics(t *testing.T) {
	t.Setenv("GOGO_DATA_HOME", t.TempDir())
	configHome := t.TempDir()
	t.Setenv("GOGO_CONFIG_HOME", configHome)

	// A home project "web" with a source at /repos/web (the migration target).
	if _, err := projects.Add(projects.Project{
		Name:    "web",
		Sources: []projects.Source{{Path: "/repos/web", Name: "web"}},
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	// Legacy epics.json with a member whose repo resolves to the web source.
	epicsJSON := `{"schema":1,"epics":[{"id":"epic-aaa","title":"Token migration","description":"move the store","created":"2026-07-14T00:00:00Z","members":[{"repo":"/repos/web","slugHint":"login"}]}]}`
	if err := os.WriteFile(filepath.Join(configHome, "epics.json"), []byte(epicsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// A legacy draft targeting the web repo by path.
	draftsDir := filepath.Join(configHome, "drafts")
	if err := os.MkdirAll(draftsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	draftMD := "---\ntitle: Retry budget\ncreated: 2026-07-15T00:00:00Z\ntarget: /repos/web\n---\n\nadd a retry budget everywhere\n"
	if err := os.WriteFile(filepath.Join(draftsDir, "retry-budget.md"), []byte(draftMD), 0o644); err != nil {
		t.Fatal(err)
	}

	MigrateLegacy()

	list, _ := List("web")
	if len(list) != 2 {
		t.Fatalf("after migrate: %d plans, want 2 (one epic + one draft)", len(list))
	}
	byTitle := map[string]Plan{}
	for _, p := range list {
		byTitle[p.Title] = p
	}
	epic := byTitle["Token migration"]
	if epic.Status != StatusActive || len(epic.Members) != 1 || epic.Members[0] != (Member{Source: "web", SlugHint: "login"}) {
		t.Errorf("migrated epic = %+v, want active with member web:login", epic)
	}
	draft := byTitle["Retry budget"]
	if draft.Status != StatusDraft || draft.Description != "add a retry budget everywhere" {
		t.Errorf("migrated draft = %+v, want a draft with the body", draft)
	}

	// Idempotent: a second run adds nothing.
	MigrateLegacy()
	if list, _ := List("web"); len(list) != 2 {
		t.Errorf("second migrate produced %d plans, want still 2 (idempotent)", len(list))
	}

	// Non-destructive: the legacy files are left in place.
	if _, err := os.Stat(filepath.Join(configHome, "epics.json")); err != nil {
		t.Error("legacy epics.json was removed — migration must be non-destructive")
	}
	if _, err := os.Stat(filepath.Join(draftsDir, "retry-budget.md")); err != nil {
		t.Error("legacy draft was removed — migration must be non-destructive")
	}
}

// TestMigrateLegacyNoOpLeavesDataHomeUncreated pins REV-007: on a machine that never
// used gogo — no ~/.gogo data home yet, and no legacy ~/.config/gogo drafts/epics —
// MigrateLegacy must NOT create the data home (nor the migrated marker) just to latch a
// no-op run. A surprise ~/.gogo appearing on a user's first `gogo` invocation is the
// papercut this guards.
func TestMigrateLegacyNoOpLeavesDataHomeUncreated(t *testing.T) {
	// A data home path that does NOT exist yet (t.TempDir exists, so point one level in).
	dataHome := filepath.Join(t.TempDir(), "gogo-data")
	t.Setenv("GOGO_DATA_HOME", dataHome)
	// An empty config home — no epics.json, no drafts/ (nothing to migrate).
	t.Setenv("GOGO_CONFIG_HOME", t.TempDir())

	MigrateLegacy()

	if _, err := os.Stat(dataHome); !os.IsNotExist(err) {
		t.Errorf("MigrateLegacy created the data home %q on a no-op run (err=%v), want it left uncreated", dataHome, err)
	}
	if _, err := os.Stat(filepath.Join(dataHome, migratedMarker)); !os.IsNotExist(err) {
		t.Errorf("MigrateLegacy wrote the migrated marker on a no-op run, want none")
	}
}

// TestMigrateLegacyUnresolvableSkipped: a draft/epic whose target does not resolve
// to any project is skipped (no home), never guessed into the wrong project.
func TestMigrateLegacyUnresolvableSkipped(t *testing.T) {
	t.Setenv("GOGO_DATA_HOME", t.TempDir())
	configHome := t.TempDir()
	t.Setenv("GOGO_CONFIG_HOME", configHome)
	projects.Add(projects.Project{Name: "web", Sources: []projects.Source{{Path: "/repos/web", Name: "web"}}})

	// An epic whose member repo matches NO source.
	epicsJSON := `{"schema":1,"epics":[{"id":"epic-x","title":"Orphan","created":"2026-07-14T00:00:00Z","members":[{"repo":"/repos/ghost","slugHint":"x"}]}]}`
	os.WriteFile(filepath.Join(configHome, "epics.json"), []byte(epicsJSON), 0o644)

	MigrateLegacy()
	if list, _ := List("web"); len(list) != 0 {
		t.Errorf("unresolvable epic migrated into web (%d plans), want 0", len(list))
	}
}
