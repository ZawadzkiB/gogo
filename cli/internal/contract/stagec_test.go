package contract

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUATArtifactInFileList: uat.md (the 0.11.0 UAT gate log) shows up in the
// drill-in file list, glamour-rendered like the other prose docs.
func TestUATArtifactInFileList(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"plan.md", "uat.md", "decisions.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f := &Feature{Slug: "x", Dir: dir}
	var uat *Artifact
	for i := range Artifacts(f) {
		if a := Artifacts(f)[i]; a.Label == "uat.md" {
			uat = &a
		}
	}
	if uat == nil {
		t.Fatalf("uat.md not in the drill-in file list")
	}
	if uat.Kind != KindMarkdown {
		t.Errorf("uat.md kind = %q, want markdown", uat.Kind)
	}

	// Absent uat.md → not listed (presence signals progress).
	bare := t.TempDir()
	if err := os.WriteFile(filepath.Join(bare, "plan.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, a := range Artifacts(&Feature{Dir: bare}) {
		if a.Label == "uat.md" {
			t.Errorf("uat.md listed when the file is absent")
		}
	}
}

// TestUATEventsParseLeniently: the three new uat-* events (enum-only additions)
// parse and are kept — the consumer is lenient about the event vocabulary, so a
// 0.11.0 reader keeps every UAT transition.
func TestUATEventsParseLeniently(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	lines := `{"ts":"2026-07-04T10:00:00Z","event":"uat-opened","phase":"report","status":"waiting-for-user","slug":"x"}
{"ts":"2026-07-04T11:00:00Z","event":"uat-failed","phase":"report","status":"plan-accepted","round":1,"note":"3 issues","slug":"x"}
{"ts":"2026-07-04T12:00:00Z","event":"uat-passed","phase":"done","status":"shipped","slug":"x"}
`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	evs := ReadEvents(path)
	if len(evs) != 3 {
		t.Fatalf("parsed %d uat events, want 3", len(evs))
	}
	want := []string{"uat-opened", "uat-failed", "uat-passed"}
	for i, w := range want {
		if evs[i].Event != w {
			t.Errorf("event[%d] = %q, want %q", i, evs[i].Event, w)
		}
	}
	if !evs[1].HasRound || evs[1].Round != 1 {
		t.Errorf("uat-failed round not parsed: %+v", evs[1])
	}
}
