// Package contract is the deterministic reader for the gogo file surface
// documented in docs/cli-contract.md. It parses state.md, the work-index
// classifier, changelog entries, the typed JSON artifacts, events.jsonl and
// the mermaid chart sets — with NO LLM in the read path. Every parser is
// defensive: a missing optional file or a malformed line is degradation, never
// a panic.
package contract

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Work-index classes (docs/cli-contract.md §3, verbatim rule order).
const (
	ClassShipped     = "shipped"
	ClassReadyToShip = "ready-to-ship"
	ClassInProgress  = "in-progress"
	ClassUnfinished  = "unfinished"
)

// Board columns (class → column, §3).
const (
	ColPlan       = "plan"
	ColInProgress = "in progress"
	ColReady      = "ready"
	ColChangelog  = "changelog"
)

// Column maps a work-index class to its board column.
func Column(class string) string {
	switch class {
	case ClassUnfinished:
		return ColPlan
	case ClassInProgress:
		return ColInProgress
	case ClassReadyToShip:
		return ColReady
	case ClassShipped:
		return ColChangelog
	default:
		return ColPlan
	}
}

// Feature is one .gogo/work/feature-<slug>/ folder, parsed + classified. This
// mirrors the work-index record shape documented in skills/gogo-status.
type Feature struct {
	Slug          string
	Dir           string // absolute path to the feature folder
	Title         string
	Phase         string
	Status        string
	Created       string
	Completed     string
	Branch        string
	Iterations    string
	Resume        string
	OpenDecision  string
	Stage         string
	Class         string
	ReportPath    string // absolute path to report.md that classifies it, or ""
	ChangelogPath string // absolute path to the changelog entry that ships it, or ""
	Extra         map[string]string
	LatestEvent   *Event // the most recent events.jsonl line, or nil
}

// Column is the board column this feature belongs to.
func (f *Feature) Column() string { return Column(f.Class) }

// WaitingForUser reports whether the feature is parked on a decision gate.
func (f *Feature) WaitingForUser() bool { return f.Status == "waiting-for-user" }

// EventsPhase maps a state.md phase name to the events.jsonl phase vocabulary.
// The two agree everywhere except the fifth phase, which state.md labels
// "knowledge" and events.jsonl labels "report" (docs/cli-contract.md §2/§5).
// Used to compare state.md's current phase against the latest event's phase.
func EventsPhase(statePhase string) string {
	if statePhase == "knowledge" {
		return "report"
	}
	return statePhase
}

// RoundFor returns the iteration count recorded on state.md's `iterations` line
// for the given phase — e.g. "plan=2 · implement=4 · review=2 · test=0" yields 2
// for "review". Returns 0 when the phase is absent or unparseable. This lets a
// consumer derive a round badge from state.md alone when the events stream is
// behind (an expected telemetry gap).
func (f *Feature) RoundFor(phase string) int {
	if phase == "" || f.Iterations == "" {
		return 0
	}
	for _, tok := range strings.FieldsFunc(f.Iterations, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '·'
	}) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok || strings.TrimSpace(k) != phase {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return 0
}

// ChangelogEntry is one .gogo/changelog/<date>-<name>/ folder.
type ChangelogEntry struct {
	Dir       string // absolute
	Base      string // "<date>-<name>" folder name
	Date      string // YYYY-MM-DD (may be "")
	Name      string // the slug (single) or release name (merged)
	Members   []string
	HasReport bool
}

// Repo is a loaded view of a project's .gogo/ tree.
type Repo struct {
	Root      string // dir that contains .gogo/
	GogoDir   string // <root>/.gogo
	Features  []*Feature
	Changelog []*ChangelogEntry
}

// FindRoot walks up from start looking for a directory that contains a .gogo/
// folder. Returns that directory, or an error if none is found.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, ".gogo")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// LoadRepo reads every feature + changelog entry under root/.gogo and returns
// the classified, newest-first work index. It never fails on a malformed
// individual file; only a missing .gogo/work tree yields an empty repo.
func LoadRepo(root string) (*Repo, error) {
	r := &Repo{Root: root, GogoDir: filepath.Join(root, ".gogo")}
	r.Changelog = loadChangelog(filepath.Join(r.GogoDir, "changelog"))

	workDir := filepath.Join(r.GogoDir, "work")
	entries, _ := os.ReadDir(workDir)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "feature-") {
			continue
		}
		slug := strings.TrimPrefix(e.Name(), "feature-")
		f := loadFeature(filepath.Join(workDir, e.Name()), slug)
		classify(f, r.Changelog)
		r.Features = append(r.Features, f)
	}
	sortFeaturesNewestFirst(r.Features)
	return r, nil
}

// Feature returns the loaded feature for a slug, or nil.
func (r *Repo) Feature(slug string) *Feature {
	for _, f := range r.Features {
		if f.Slug == slug {
			return f
		}
	}
	return nil
}

// loadFeature parses one feature folder (state.md + latest event).
func loadFeature(dir, slug string) *Feature {
	f := parseStateFile(filepath.Join(dir, "state.md"))
	f.Dir = dir
	f.Slug = slug
	if evs := ReadEvents(filepath.Join(dir, "events.jsonl")); len(evs) > 0 {
		last := evs[len(evs)-1]
		f.LatestEvent = &last
	}
	return f
}

// classify applies the work-index classifier (docs/cli-contract.md §3),
// first-matching-rule-wins, and fills ReportPath / ChangelogPath.
func classify(f *Feature, cl []*ChangelogEntry) {
	// A changelog entry ships this slug when its folder is <date>-<slug> with a
	// report.md, OR the slug appears in a manifest members[] array.
	for _, e := range cl {
		if (e.Name == f.Slug && e.HasReport) || containsStr(e.Members, f.Slug) {
			f.ChangelogPath = e.Dir
			break
		}
	}
	f.ReportPath = detectReport(f.Dir)

	switch {
	case f.Status == "shipped" || f.ChangelogPath != "":
		f.Class = ClassShipped
	case f.ReportPath != "":
		f.Class = ClassReadyToShip
	case inProgressPhaseOrStatus(f.Phase, f.Status):
		f.Class = ClassInProgress
	default:
		f.Class = ClassUnfinished
	}
}

// detectReport returns the absolute report.md path that makes a feature
// report-complete: the new report/report.md bundle, else a legacy root
// report.md. Empty when neither exists.
func detectReport(dir string) string {
	bundle := filepath.Join(dir, "report", "report.md")
	if fileExists(bundle) {
		return bundle
	}
	legacy := filepath.Join(dir, "report.md")
	if fileExists(legacy) {
		return legacy
	}
	return ""
}

func inProgressPhaseOrStatus(phase, status string) bool {
	switch phase {
	case "implement", "review", "test":
		return true
	}
	switch status {
	case "implementing", "reviewing", "testing":
		return true
	}
	return false
}

func sortFeaturesNewestFirst(fs []*Feature) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Created != fs[j].Created {
			return fs[i].Created > fs[j].Created // YYYY-MM-DD lexicographic == chronological
		}
		return fs[i].Slug < fs[j].Slug
	})
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
