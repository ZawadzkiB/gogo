package main

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/trash"
)

func TestFormatTrashEmpty(t *testing.T) {
	out := FormatTrash(nil)
	if !strings.Contains(out, "0 entries") || !strings.Contains(out, "empty") {
		t.Errorf("empty trash listing = %q", out)
	}
}

func TestFormatTrashListing(t *testing.T) {
	entries := []trash.Entry{
		{Base: "20260704T024500Z-my-slug", When: "2026-07-04 02:45:00", Slug: "my-slug", Phase: "review", Status: "reviewing"},
	}
	out := FormatTrash(entries)
	for _, want := range []string{"1 entry", "my-slug", "review/reviewing", "20260704T024500Z-my-slug", "restore"} {
		if !strings.Contains(out, want) {
			t.Errorf("trash listing missing %q:\n%s", want, out)
		}
	}
}
