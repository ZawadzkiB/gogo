package contract

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFeatureFastMode pins the additive `mode: fast` state.md marker (0.34.0): the
// lenient parser lifts it into Extra, FastMode() reads it, and an absent or unknown
// value is simply false — a full-pipeline or pre-0.34 state.md reads byte-for-byte
// as before. Display-only: nothing here feeds classification.
func TestFeatureFastMode(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"fast", "- **mode:** fast", true},
		{"absent", "", false},
		{"unknown value", "- **mode:** turbo", false},
		{"trailing comment stripped", "- **mode:** fast  <!-- gogo-fast run -->", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "- **feature:** feat\n- **phase:** implement\n- **status:** implementing\n"
			if c.line != "" {
				body += c.line + "\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			f := parseStateFile(filepath.Join(dir, "state.md"))
			if got := f.FastMode(); got != c.want {
				t.Errorf("FastMode() = %v, want %v (line %q)", got, c.want, c.line)
			}
		})
	}

	// A synthetic Feature (nil Extra) must read false, never panic.
	if (&Feature{}).FastMode() {
		t.Error("synthetic Feature{} reads FastMode true")
	}
}
