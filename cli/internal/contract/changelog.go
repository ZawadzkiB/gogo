package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var datePrefix = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-(.+)$`)

// loadChangelog reads every .gogo/changelog/<date>-<name>/ entry: its folder
// name (date + name), whether a report.md is present, and the manifest
// members[] array when the entry carries one (0.8.0+). Absent changelog dir →
// empty slice.
func loadChangelog(dir string) []*ChangelogEntry {
	entries, _ := os.ReadDir(dir)
	var out []*ChangelogEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		ce := &ChangelogEntry{
			Dir:       full,
			Base:      e.Name(),
			HasReport: fileExists(filepath.Join(full, "report.md")),
		}
		if m := datePrefix.FindStringSubmatch(e.Name()); m != nil {
			ce.Date = m[1]
			ce.Name = m[2]
		} else {
			ce.Name = e.Name()
		}
		ce.Members = readManifestMembers(filepath.Join(full, "manifest.json"))
		out = append(out, ce)
	}
	return out
}

// readManifestMembers extracts only the members[] array from a changelog
// manifest.json, tolerating absence (pre-0.8.0 entries omit it) and malformed
// JSON. The classifier's folder-slug fallback covers the empty case.
func readManifestMembers(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m struct {
		Members []string `json:"members"`
	}
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	// Defensively normalise: drop blanks / whitespace.
	var out []string
	for _, s := range m.Members {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
