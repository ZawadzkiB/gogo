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

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
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
	Slug string
	Dir  string // absolute path to the feature folder
	// Root is the repo root (the dir containing .gogo/) this feature was loaded
	// from — stamped by LoadRepo on every feature. In the aggregate project board
	// (LoadProject) it is what makes a per-feature action target the RIGHT source
	// (rootFor); in single-repo mode it simply equals the board's one root.
	Root string
	// Source is the label the board tags this feature's card with — the SOURCE it
	// was loaded from (LoadProject stamps it per source). Empty in single-repo mode —
	// so the card tag is invisible there (byte-for-byte fallback parity). Renamed from
	// Project as part of the corrected project→sources model (a card is tagged by its
	// source, not a flat repo-project).
	Source string
	// Project is the home PROJECT this feature's source belongs to — stamped once by
	// LoadWorkspace when it merges every project's sources into the unified cockpit
	// board (0.23.0). "" in single-repo mode AND in the single-project LoadProject view
	// (only the multi-project workspace merge knows the project), so the two-dot
	// `●project ●source` origin cue degrades to a single source dot there (byte-for-byte
	// parity). Presentation-only: it drives the card/changelog origin tag + the project
	// chip filter, never a .gogo write or a pipeline-state mutation.
	Project string
	// Correlations are the plan ids (plan-<hash>) this work item belongs to — read
	// DIRECTLY from state.md's additive optional `correlation:` list by the parser
	// (FR13/L1), NOT a CLI-side overlay. Many-to-many: a ticket in two plans carries
	// both ids. nil when the state.md carries no correlation line (the byte-for-byte
	// pre-correlation fallback — no `⛓` chip, no `#plan-…` filter effect). This
	// supersedes the removed epics-store overlay: correlation now lives in state.md.
	Correlations  []string
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

// AwaitingUAT reports whether the feature is parked at the UAT gate — where
// phase ⑤ now leaves a report-complete feature in 0.11.0 (status awaiting-uat).
// Such a feature classifies ready-to-ship (§3); the CLI paints an awaiting-uat
// badge on its ready card. During a mid-UAT re-plan the status flips off
// awaiting-uat (to waiting-for-user, then implementing/…) — so the feature drops
// OUT of ready-to-ship back to in-progress (readyToShipStatus, TEST-004), and
// its badge takes waiting-for-user priority (docs/cli-contract.md §2/§3).
func (f *Feature) AwaitingUAT() bool { return f.Status == "awaiting-uat" }

// Shipped reports whether the work item has reached its terminal shipped state,
// keyed on the state.md STATUS (`shipped`, or the legacy `done`) — NOT on artifact
// presence (a changelog entry outlives a mid-UAT re-plan, so gating on it would lie;
// TEST-004). The project-UAT gate (FR3) reads this to decide whether every member of
// a plan is shipped before the plan can be accepted.
func (f *Feature) Shipped() bool { return f.Status == "shipped" || f.Status == "done" }

// WaitingForInput reports whether the feature is parked at a genuine USER gate —
// the union of the three statuses that block on the user: awaiting-plan-acceptance
// (the plan-acceptance gate), waiting-for-user (a decision gate / mid-UAT re-plan
// lock), and awaiting-uat (the UAT gate). Every other status flows unattended
// (plan-accepted, implementing / reviewing / testing, shipped / done / aborted).
// This is the single predicate the display layer reads to mark which cards need
// the user vs which flow (docs/cli-contract.md §2/§3). WaitingForUser() and
// AwaitingUAT() stay for the badge precedence each carries; WaitingForInput() is
// the additive presentation union (unattended-ops-input-signals, FR-B1).
func (f *Feature) WaitingForInput() bool {
	switch f.Status {
	case "awaiting-plan-acceptance", "waiting-for-user", "awaiting-uat":
		return true
	}
	return false
}

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
		f.Root = root // every feature carries its own root (single-repo == global here)
		classify(f, r.Changelog)
		r.Features = append(r.Features, f)
	}
	sortFeaturesNewestFirst(r.Features)
	return r, nil
}

// LoadProject builds a PROJECT board view (the corrected multi-source model): it
// calls the per-repo LoadRepo ONCE per source (no fork of the reader), stamps each
// Feature with its source label (Root is already set by LoadRepo == the source
// path), and merges every source's features + changelog into ONE newest-first
// *Repo with an empty Root (each Feature carries its own). A source path with no
// readable .gogo/work yields an empty LoadRepo and contributes nothing (skipped
// gracefully, never a crash) — the defensive reader style. This is the sole
// aggregate board reader, keyed on a home project's sources.
func LoadProject(p projects.Project) *Repo {
	agg := &Repo{}
	for _, s := range p.Sources {
		repo, err := LoadRepo(s.Path)
		if err != nil || repo == nil {
			continue
		}
		label := s.Name
		if label == "" {
			label = filepath.Base(s.Path)
		}
		for _, f := range repo.Features {
			f.Source = label // Root already stamped by LoadRepo (== s.Path)
			agg.Features = append(agg.Features, f)
		}
		agg.Changelog = append(agg.Changelog, repo.Changelog...)
	}
	sortFeaturesNewestFirst(agg.Features)
	return agg
}

// LoadWorkspace builds the UNIFIED cockpit board across EVERY registered project (the
// multi-project aggregate, 0.23.0): it calls the existing per-project LoadProject(p)
// once per project — no fork of the reader, since LoadProject already stamps each
// Feature's Source + Root — then stamps f.Project = p.Name on every returned feature
// and merges all features + changelog into ONE newest-first *Repo with an empty Root
// (each Feature carries its own project + source + root — everything the tags, the
// project filter, and rootFor need). A project with no readable sources contributes
// nothing (skipped gracefully, never a crash) — the defensive reader style. This is
// presentation/aggregation ONLY: no .gogo contract-file change, no pipeline-state
// mutation, no LLM in the read path.
func LoadWorkspace(projs []projects.Project) *Repo {
	agg := &Repo{}
	for _, p := range projs {
		one := LoadProject(p)
		if one == nil {
			continue
		}
		for _, f := range one.Features {
			f.Project = p.Name // Source + Root already stamped by LoadProject/LoadRepo
			agg.Features = append(agg.Features, f)
		}
		agg.Changelog = append(agg.Changelog, one.Changelog...)
	}
	sortFeaturesNewestFirst(agg.Features)
	return agg
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
// first-matching-rule-wins, and fills ReportPath / ChangelogPath. ready-to-ship
// requires a report AND a ship-gate status (awaiting-uat or legacy done) — a
// stale report left behind by a UAT rerun does not qualify (readyToShipStatus).
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
	case f.ReportPath != "" && readyToShipStatus(f.Status):
		f.Class = ClassReadyToShip
	case inProgressPhaseOrStatus(f.Phase, f.Status):
		f.Class = ClassInProgress
	default:
		f.Class = ClassUnfinished
	}
}

// readyToShipStatus reports whether a report-complete feature is genuinely
// parked at the ship gate — status awaiting-uat (0.11.0's phase-⑤ landing) or a
// legacy done (pre-0.11). A report on disk alone no longer qualifies: a UAT
// rerun re-runs ②→⑤ on the SAME feature without clearing the prior report/, so
// during that window a mid-pipeline feature (status implementing / plan-accepted
// / waiting-for-user) carries a STALE report — and must NOT show as ready-to-ship.
// The in-progress rule catches it instead (docs/cli-contract.md §3, TEST-004).
func readyToShipStatus(status string) bool {
	return status == "awaiting-uat" || status == "done"
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
