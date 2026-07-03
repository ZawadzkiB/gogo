package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Issue is one entry of a review/test issues.json (issues-list.schema.json).
type Issue struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	ProposedSolution string `json:"proposed_solution"`
	Severity         string `json:"severity"`
	Priority         string `json:"priority"`
	Status           string `json:"status"`
	Origin           string `json:"origin"`
	FoundInRound     int    `json:"found_in_round"`
	FixedInRound     int    `json:"fixed_in_round"`
	FixSummary       string `json:"fix_summary"`
}

// IssuesList is a whole review/issues.json or test/issues.json file.
type IssuesList struct {
	Slug    string  `json:"slug"`
	Track   string  `json:"track"`
	Round   int     `json:"round"`
	Updated string  `json:"updated"`
	Issues  []Issue `json:"issues"`
}

// ReadIssues parses a review/test issues.json. Absent file → (nil, nil) so a
// caller can treat "no findings" as normal.
func ReadIssues(path string) (*IssuesList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list IssuesList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// Diagram is one entry of a charts/report manifest.json diagrams[] array.
type Diagram struct {
	Kind  string `json:"kind"`
	File  string `json:"file"`
	Title string `json:"title"`
}

// Manifest is a charts/manifest.json or report/manifest.json
// (charts-manifest.schema.json), plus the changelog members[] extension.
type Manifest struct {
	Slug     string    `json:"slug"`
	Updated  string    `json:"updated"`
	Note     string    `json:"note"`
	Diagrams []Diagram `json:"diagrams"`
	Members  []string  `json:"members"`
}

// TitleFor returns the manifest title for a diagram basename (the .mmd file
// name without extension), matched against each diagram's File basename. Empty
// when the manifest has no matching entry.
func (m *Manifest) TitleFor(basename string) string {
	if m == nil {
		return ""
	}
	for _, d := range m.Diagrams {
		if strings.TrimSuffix(filepath.Base(d.File), filepath.Ext(d.File)) == basename {
			return d.Title
		}
	}
	return ""
}

// ReadManifest parses a manifest.json. Absent file → (nil, nil).
func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ListDiagrams returns the .mmd files in a bundle directory, sorted by name,
// skipping the non-diagram files (manifest.json, diagrams.html). Absolute
// paths. Missing dir → nil.
func ListDiagrams(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".mmd") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}
