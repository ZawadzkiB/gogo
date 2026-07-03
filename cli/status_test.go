package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// TestStatusGolden pins the `gogo status` output on the fixture tree so a
// change to the classifier or the table format is caught (contract stability).
func TestStatusGolden(t *testing.T) {
	repo, err := contract.LoadRepo(filepath.Join("internal", "contract", "testdata", "repo"))
	if err != nil {
		t.Fatalf("LoadRepo: %v", err)
	}
	got := FormatStatus(repo)

	goldenPath := filepath.Join("testdata", "status.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("status output drifted from golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
