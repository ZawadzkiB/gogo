package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestVersionMirrorsPlugin pins the CLI --version to the plugin version (the stability
// statement: `cli/main.go Version` mirrors `.claude-plugin/plugin.json`), and to the
// current release train (0.25.0 — smart-project-plans). A version bump updates both,
// so this fails loudly if the two ever drift.
func TestVersionMirrorsPlugin(t *testing.T) {
	const want = "0.25.0"
	if Version != want {
		t.Errorf("cli Version = %q, want %q", Version, want)
	}

	raw, err := os.ReadFile(filepath.Join("..", ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}
	var plugin struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &plugin); err != nil {
		t.Fatalf("parse plugin.json: %v", err)
	}
	if plugin.Version != Version {
		t.Errorf("plugin.json version = %q, but cli Version = %q — they must mirror", plugin.Version, Version)
	}
}
