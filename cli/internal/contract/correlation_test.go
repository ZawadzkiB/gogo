package contract

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// writeStateWithCorrelation drops a state.md carrying the given correlation LINE
// verbatim (line == "" writes no correlation line at all) under a feature folder.
func writeStateWithCorrelation(t *testing.T, root, slug, correlationLine string) {
	t.Helper()
	dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "- **feature:** " + slug + "\n" +
		"- **phase:** implement\n" +
		"- **status:** implementing\n" +
		"- **created:** 2026-07-18\n"
	if correlationLine != "" {
		body += correlationLine + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestParseStateCorrelationRoundTrip pins FR13: parseStateFile lifts the additive,
// optional `correlation:` list onto Feature.Correlations for one/many, leaves it
// nil when the line is absent (byte-for-byte pre-correlation parity), and recovers
// gracefully from a malformed value — never a crash.
func TestParseStateCorrelationRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{"one", "- **correlation:** [plan-7f3a]", []string{"plan-7f3a"}},
		{"many", "- **correlation:** [plan-7f3a, plan-9c2e]", []string{"plan-7f3a", "plan-9c2e"}},
		{"bare single (no brackets)", "- **correlation:** plan-7f3a", []string{"plan-7f3a"}},
		{"absent", "", nil},
		{"empty list", "- **correlation:** []", nil},
		{"malformed unclosed bracket", "- **correlation:** [plan-7f3a", []string{"plan-7f3a"}},
		{"trailing comment stripped", "- **correlation:** [plan-7f3a]  <!-- optional -->", []string{"plan-7f3a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeStateWithCorrelation(t, root, "feat", c.line)
			f := parseStateFile(filepath.Join(root, ".gogo", "work", "feature-feat", "state.md"))
			if !reflect.DeepEqual(f.Correlations, c.want) {
				t.Errorf("Correlations = %v, want %v", f.Correlations, c.want)
			}
		})
	}
}

// TestLoadProjectReadsCorrelationEndToEnd pins the reader side of the round-trip:
// a source work item stamped with `correlation: [plan-XXXX]` surfaces on the loaded
// feature's Correlations after LoadProject — the board reads the chip straight from
// state.md, no CLI-side overlay.
func TestLoadProjectReadsCorrelationEndToEnd(t *testing.T) {
	root := t.TempDir()
	writeStateWithCorrelation(t, root, "login", "- **correlation:** [plan-abcd1234]")

	repo := LoadProject(projects.Project{Name: "web", Sources: []projects.Source{{Path: root, Name: "web"}}})
	if len(repo.Features) != 1 {
		t.Fatalf("features = %d, want 1", len(repo.Features))
	}
	f := repo.Features[0]
	if !reflect.DeepEqual(f.Correlations, []string{"plan-abcd1234"}) {
		t.Errorf("Correlations = %v, want [plan-abcd1234]", f.Correlations)
	}
}
