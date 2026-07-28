package plans

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMarkDoneWritesChangelogEntry pins plans-board FR5 (D3=A): accepting a plan's
// project-UAT (MarkDone) writes a deterministic, LLM-free changelog entry under
// ~/.gogo/projects/<name>/.gogo/changelog/<date>-<id>/entry.md that records the plan id +
// title + each member work item, AND the plan STAYS in the store as `done` so the kanban's
// 4th column still shows it. No source .gogo/ is touched.
func TestMarkDoneWritesChangelogEntry(t *testing.T) {
	seedDataHome(t)
	p, _ := New("app", "Cross-repo token migration", "roll it out")
	AddMember("app", p.ID, Member{Source: "web", SlugHint: "web-token"})
	AddMember("app", p.ID, Member{Source: "api", SlugHint: "api-token"})
	SetStatus("app", p.ID, StatusActive)

	done, err := MarkDone("app", p.ID)
	if err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	if done.Status != StatusDone {
		t.Fatalf("MarkDone left status %q, want done", done.Status)
	}

	// The plan STAYS in the store as done (kanban done-column parity).
	list, _ := List("app")
	var found *Plan
	for i := range list {
		if list[i].ID == p.ID {
			found = &list[i]
		}
	}
	if found == nil || found.Status != StatusDone {
		t.Fatalf("plan not in store as done after MarkDone: %+v", list)
	}

	// A changelog entry was written under the project home, naming id + title + members.
	dir := ChangelogDir("app")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read changelog dir %s: %v", dir, err)
	}
	var entryDir string
	for _, e := range entries {
		if e.IsDir() && strings.Contains(e.Name(), p.ID) {
			entryDir = filepath.Join(dir, e.Name())
		}
	}
	if entryDir == "" {
		t.Fatalf("no changelog entry dir for %s under %s (got %v)", p.ID, dir, entries)
	}
	raw, err := os.ReadFile(filepath.Join(entryDir, "entry.md"))
	if err != nil {
		t.Fatalf("read entry.md: %v", err)
	}
	body := string(raw)
	for _, want := range []string{p.ID, "Cross-repo token migration", "web : web-token", "api : api-token"} {
		if !strings.Contains(body, want) {
			t.Errorf("changelog entry missing %q:\n%s", want, body)
		}
	}
}

// TestWriteChangelogEntryBareFallback pins the FR5 BDD case the success-path test doesn't
// reach: when a member's status/report line is NOT cheaply available from the loaded
// contract (repo is nil — e.g. loadProjectRepo's best-effort project read came back empty),
// writeChangelogEntry never blocks or errors — it records the member as the bare advisory
// `source:slug` (mem.Source + " : " + mem.SlugHint`, no enrichment, no status parenthetical).
func TestWriteChangelogEntryBareFallback(t *testing.T) {
	seedDataHome(t)
	p := Plan{ID: "plan-bare1", Title: "Bare fallback plan",
		Members: []Member{{Source: "web", SlugHint: "web-thing"}, {Source: "api", SlugHint: "api-thing"}}}

	if err := writeChangelogEntry("app", p, "2026-07-23", nil /* repo unavailable */); err != nil {
		t.Fatalf("writeChangelogEntry with a nil repo: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(ChangelogDir("app"), "2026-07-23-plan-bare1", "entry.md"))
	if err != nil {
		t.Fatalf("read entry.md: %v", err)
	}
	body := string(raw)
	for _, want := range []string{"web : web-thing", "api : api-thing"} {
		if !strings.Contains(body, want) {
			t.Errorf("entry.md missing bare fallback line %q:\n%s", want, body)
		}
	}
	// No enrichment happened — the member lines are the exact bare form, no trailing
	// "(<status>)" parenthetical from a resolved feature.
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "- ") && strings.Contains(line, "(") {
			t.Errorf("member line carries an enrichment parenthetical despite repo==nil: %q", line)
		}
	}
}

// TestChangelogDirUnderProjectHome pins the path convention (FR5): the changelog dir mirrors
// the plans dir — both live under the project's home (~/.gogo/), never a source's .gogo/.
func TestChangelogDirUnderProjectHome(t *testing.T) {
	seedDataHome(t)
	got := ChangelogDir("app")
	if !strings.HasSuffix(got, filepath.Join("app", ".gogo", "changelog")) {
		t.Errorf("ChangelogDir = %q, want it under the project home <...>/app/.gogo/changelog", got)
	}
	// It shares the project-home root with the plans dir (both are ~/.gogo/, never a source).
	if filepath.Dir(got) != filepath.Dir(Dir("app")) {
		t.Errorf("changelog dir %q and plans dir %q must share the project's .gogo/ parent", got, Dir("app"))
	}
}
