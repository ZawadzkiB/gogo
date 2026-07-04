package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ArtifactKind drives how the drill-in viewer renders a file.
type ArtifactKind string

const (
	KindMarkdown ArtifactKind = "markdown" // glamour viewport
	KindIssues   ArtifactKind = "issues"   // readable table
	KindEvents   ArtifactKind = "events"   // timeline
	KindMermaid  ArtifactKind = "mermaid"  // ASCII render / source fallback
)

// Artifact is one drill-in list row: a real file that exists for a feature.
type Artifact struct {
	Label string
	Path  string // absolute
	Kind  ArtifactKind
}

var roundFile = regexp.MustCompile(`^(review|test)-(\d+)\.md$`)

// Artifacts lists the feature's readable files, in a stable drill-in order:
// plan, report, decisions, adjustments, review/test snapshots, issues,
// events, then the chart/report diagram sources. Only files that exist are
// included (docs/cli-contract.md: presence signals progress).
func Artifacts(f *Feature) []Artifact {
	var out []Artifact
	add := func(label, path string, kind ArtifactKind) {
		if fileExists(path) {
			out = append(out, Artifact{Label: label, Path: path, Kind: kind})
		}
	}

	add("plan.md", filepath.Join(f.Dir, "plan.md"), KindMarkdown)
	// report: new bundle wins over legacy root.
	if bundle := filepath.Join(f.Dir, "report", "report.md"); fileExists(bundle) {
		add("report/report.md", bundle, KindMarkdown)
	} else {
		add("report.md", filepath.Join(f.Dir, "report.md"), KindMarkdown)
	}
	add("decisions.md", filepath.Join(f.Dir, "decisions.md"), KindMarkdown)
	add("adjustments.md", filepath.Join(f.Dir, "adjustments.md"), KindMarkdown)
	// uat.md — the UAT gate log (0.11.0); glamour-rendered like the other prose.
	add("uat.md", filepath.Join(f.Dir, "uat.md"), KindMarkdown)

	for _, rf := range roundSnapshots(f.Dir) {
		add(rf, filepath.Join(f.Dir, rf), KindMarkdown)
	}

	add("review/issues.json", filepath.Join(f.Dir, "review", "issues.json"), KindIssues)
	add("test/issues.json", filepath.Join(f.Dir, "test", "issues.json"), KindIssues)

	if evPath := filepath.Join(f.Dir, "events.jsonl"); fileExists(evPath) {
		out = append(out, Artifact{Label: "events (timeline)", Path: evPath, Kind: KindEvents})
	}

	for _, m := range ListDiagrams(filepath.Join(f.Dir, "charts")) {
		out = append(out, Artifact{Label: "charts/" + filepath.Base(m), Path: m, Kind: KindMermaid})
	}
	for _, m := range ListDiagrams(filepath.Join(f.Dir, "report")) {
		out = append(out, Artifact{Label: "report/" + filepath.Base(m), Path: m, Kind: KindMermaid})
	}
	return out
}

// roundSnapshots returns review-NN.md / test-NN.md names in the folder, sorted.
func roundSnapshots(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if roundFile.MatchString(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		// review before test, then by round number embedded in the name.
		ti, ni := roundKey(out[i])
		tj, nj := roundKey(out[j])
		if ti != tj {
			return ti < tj
		}
		return ni < nj
	})
	return out
}

func roundKey(name string) (string, int) {
	m := roundFile.FindStringSubmatch(name)
	if m == nil {
		return name, 0
	}
	n := 0
	for _, r := range m[2] {
		n = n*10 + int(r-'0')
	}
	return m[1], n
}

// SlugFromArg strips a leading "feature-" and any ":plan"/":report" suffix,
// returning the bare slug and the requested kind ("", "plan" or "report").
func SlugFromArg(arg string) (slug, kind string) {
	arg = strings.TrimPrefix(arg, "feature-")
	if i := strings.IndexByte(arg, ':'); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}
