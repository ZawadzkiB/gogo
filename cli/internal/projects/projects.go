// Package projects is the CLI-owned reader/writer for home-folder PROJECT
// entities — the corrected multi-source model that supersedes the flat
// config.Project registry. A PROJECT is a home-folder entity at
// ~/.gogo/projects/<name>/ (config.json + an optional .knowledge/ dir + a
// .gogo/plans/ dir) that links many SOURCES — repos (or monorepo services) that
// already carry their own .gogo/. One project → many sources.
//
// Like config, this package is deliberately WRITE-capable: a project entity is
// CLI-owned data, not pipeline state, so `gogo project add/rm` and
// `gogo source add/rm` mutate it. The hard "CLI reads, skills write" invariant is
// about a SOURCE's .gogo/ — which this package NEVER writes. It writes ONLY the
// gogo DATA home (~/.gogo/), and `Remove` deletes only a project's own home
// folder, never a source's .gogo/.
//
// Every read is defensive: a missing or malformed file degrades to an empty
// result with NO error (the single-repo fallback the board relies on), mirroring
// config.Load and the contract parsers.
package projects

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Schema is the current project-config schema version stamped into every saved
// config.json.
const Schema = 1

// configFile is the per-project entity file under the project's home folder.
const configFile = "config.json"

// homeConfigFile is the GLOBAL-home marker/config at ~/.gogo/config.json (distinct
// from the per-project config.json under projects/<name>/). Its mere existence
// marks the global cockpit as "initialized" — the entry-point key for `gogo` outside
// a repo and for `gogo global` (FR19/FR22).
const homeConfigFile = "config.json"

// DefaultConcurrentWorkItems is the per-SOURCE concurrency cap a freshly-added
// source defaults to (the successor of config.DefaultMaxConcurrent): a new source
// limits itself to one live in-progress work item. 0 = UNLIMITED — the
// fallback-preserving sentinel (an absent field unmarshals to 0), so an uncapped
// source stays byte-for-byte as the single-repo path behaves today.
const DefaultConcurrentWorkItems = 1

// Source is one repo (or monorepo service) linked into a project: an absolute
// path to the dir that contains .gogo/, a display name, the source's default
// branch, its per-source concurrency cap, and an optional card-tag color (hex).
type Source struct {
	Path string `json:"path"`
	Name string `json:"name"`
	// MainBranch is the source's default branch (detected git default, else main).
	MainBranch string `json:"mainBranch,omitempty"`
	// ConcurrentWorkItems caps how many of this source's features may be actively
	// worked at once — the launch guard (orchestrator.CapForSource / CapExceeded)
	// refuses a go over it. 0 = UNLIMITED (the sentinel that preserves the
	// single-repo fallback). `omitempty` keeps a zero-cap source's on-disk shape
	// minimal.
	ConcurrentWorkItems int `json:"concurrentWorkItems,omitempty"`
	// Color is the optional card-tag color (hex) the board tints this source's
	// cards with.
	Color string `json:"color,omitempty"`
	// PlanAcceptanceSkip, when true, opts this source OUT of the per-work-item
	// plan-acceptance gate (FR4): the CLI appends `--skip-acceptance` to the launched
	// `/gogo:go`, and the gogo skills auto-record the acceptance instead of stopping
	// for the user. Additive + optional (omitempty, schema stays 1), default false —
	// an absent field keeps the gate byte-for-byte.
	PlanAcceptanceSkip bool `json:"planAcceptanceSkip,omitempty"`
	// UatAcceptanceSkip, when true, opts this source OUT of the per-work-item UAT gate
	// (FR4): the CLI appends `--skip-uat` to the launched `/gogo:go`, and the gogo
	// skills auto-pass UAT (emit `uat-passed`, ship) instead of stopping at
	// `awaiting-uat`. Additive + optional (omitempty), default false. NOTE the FR3×FR4
	// orthogonality: this removes the per-WORK-ITEM UAT; the project-UAT still gates
	// the whole plan.
	UatAcceptanceSkip bool `json:"uatAcceptanceSkip,omitempty"`
}

// Project is a home-folder entity linking many sources. It is written to
// ~/.gogo/projects/<Name>/config.json.
type Project struct {
	Schema      int    `json:"schema"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Color is the optional project-level origin color (hex) — the design's
	// per-project label color, auto-assigned at `gogo project add`, editable in the
	// config tab. Additive + optional (omitempty, schema stays 1): an absent field
	// degrades to the deterministic palette fallback (ColorForIndex), never a crash.
	Color   string   `json:"color,omitempty"`
	Sources []Source `json:"sources"`
}

// Home resolves the gogo DATA home directory, honoring (in order):
//
//	$GOGO_DATA_HOME   the test seam — points straight at the data home
//	$HOME/.gogo       the user's stated default
//
// This mirrors config.Home()'s env-seam pattern but for the DATA home (D1=A):
// deliberately the literal ~/.gogo path, not an XDG/os.UserConfigDir divergence.
// The GOGO_DATA_HOME seam lets tests point the whole store at a t.TempDir() so
// they never touch the real ~/.gogo.
func Home() string {
	if h := os.Getenv("GOGO_DATA_HOME"); h != "" {
		return h
	}
	return filepath.Join(os.Getenv("HOME"), ".gogo")
}

// ProjectsDir is ~/.gogo/projects — the parent of every project's home folder.
func ProjectsDir() string { return filepath.Join(Home(), "projects") }

// HomeConfigPath is ~/.gogo/config.json — the global-home marker/config whose
// existence means the global cockpit is initialized (FR19/FR22).
func HomeConfigPath() string { return filepath.Join(Home(), homeConfigFile) }

// homeConfig is the global-home config written at ~/.gogo/config.json. Its mere
// presence marks the cockpit initialized; schema stamps the format so a later
// version can grow the file without a flag day.
type homeConfig struct {
	Schema int `json:"schema"`
}

// Initialized reports whether the global cockpit home has been set up — the
// ~/.gogo/config.json marker exists as a regular file. A missing/unreadable marker
// (or a dir in its place) → not initialized, never a crash. This is the entry-point
// key for `gogo` outside a repo and for `gogo global`.
func Initialized() bool {
	info, err := os.Stat(HomeConfigPath())
	return err == nil && !info.IsDir()
}

// EnsureHome initializes the global cockpit home if it is not already: it creates
// ~/.gogo/projects/ and writes the ~/.gogo/config.json marker. It is idempotent — a
// no-op returning created=false once the marker exists — and writes ONLY under
// ~/.gogo/ (the hard write-scope invariant). Returns created=true only when it wrote
// the marker on this call. Used by `gogo global init` (explicit setup) and by
// `gogo project add` (forgiving auto-init, FR22).
func EnsureHome() (created bool, err error) {
	if Initialized() {
		return false, nil
	}
	if err := os.MkdirAll(ProjectsDir(), 0o755); err != nil {
		return false, err
	}
	raw, err := json.MarshalIndent(homeConfig{Schema: Schema}, "", "  ")
	if err != nil {
		return false, err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(HomeConfigPath(), raw, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// Dir is a project's home folder ~/.gogo/projects/<name>.
func Dir(name string) string { return filepath.Join(ProjectsDir(), name) }

// configPath is <project-home>/config.json.
func configPath(name string) string { return filepath.Join(Dir(name), configFile) }

// validName reports whether name is a safe single path component to use as a
// project folder — the write-scope guard so a name carrying `/`, `\` or `..` can
// never escape ProjectsDir.
func validName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, `/\`) && filepath.Base(name) == name
}

// ValidName reports whether name is a safe single path component usable as a
// project folder (no `/`, `\`, `.`, or `..`) — the exported write-scope guard the
// CLI's dual-mode `gogo project add` uses to reject a bad bare NAME up front.
func ValidName(name string) bool { return validName(name) }

// hasConfig reports whether a project home folder actually holds a config.json
// (so List skips a stray dir with no entity).
func hasConfig(name string) bool {
	info, err := os.Stat(configPath(name))
	return err == nil && !info.IsDir()
}

// Exists reports whether a project with this name is registered (its config.json
// exists). A missing / invalid name → false. The CLI's bare-NAME `project add`
// uses it to dedupe/preserve an existing project instead of clobbering it.
func Exists(name string) bool { return validName(name) && hasConfig(name) }

// KnowledgeDir is a project's cross-repo knowledge dir
// ~/.gogo/projects/<name>/.knowledge — the PROJECT-level domain knowledge (how the
// sources connect), distinct from each SOURCE's own per-repo <repo>/.gogo/knowledge/.
func KnowledgeDir(name string) string { return filepath.Join(Dir(name), ".knowledge") }

// projectKnowledgeFile is the single seeded cross-repo knowledge file scaffolded at
// `gogo project add` (FR2).
const projectKnowledgeFile = "project-knowledge.md"

// EnsureProjectHome scaffolds a project's home-folder LAYOUT idempotently (FR2): the
// project dir, its cross-repo .knowledge/ dir seeded with project-knowledge.md, and
// its .gogo/plans/ dir. It writes ONLY under ~/.gogo/ and NEVER clobbers an existing
// knowledge file. It does NOT write config.json — the entity is persisted by
// Add/Save; this only ensures the surrounding dirs + seed exist, so calling it on an
// already-scaffolded (or knowledge-less legacy) project just re-ensures them, never a
// crash. An invalid name is refused (the write-scope guard).
func EnsureProjectHome(name string) error {
	if !validName(name) {
		return fmt.Errorf("projects: invalid project name %q", name)
	}
	for _, dir := range []string{
		Dir(name),
		KnowledgeDir(name),
		filepath.Join(Dir(name), ".gogo", "plans"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return SeedProjectKnowledge(name)
}

// SeedProjectKnowledge writes .knowledge/project-knowledge.md from a deterministic
// seeded template (headed: domain · how the sources connect · glossary · integration
// contracts), but ONLY when the file is absent — idempotent, so a user's own edits (or
// a hand-authored file) are NEVER clobbered. Writes ONLY under ~/.gogo/.
func SeedProjectKnowledge(name string) error {
	if !validName(name) {
		return fmt.Errorf("projects: invalid project name %q", name)
	}
	dir := KnowledgeDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, projectKnowledgeFile)
	if _, err := os.Stat(path); err == nil {
		return nil // already present → never clobber
	}
	return os.WriteFile(path, []byte(projectKnowledgeTemplate(name)), 0o644)
}

// projectKnowledgeTemplate renders the deterministic seed for a project's
// project-knowledge.md — the cross-repo DOMAIN brief the plan-author session reads so
// a whole-domain understanding flows into each spawned work item's goal. Headed,
// prompt-fillable, no LLM.
func projectKnowledgeTemplate(name string) string {
	return `# Project knowledge - ` + name + `

<!-- Cross-repo DOMAIN knowledge for this gogo project: how its SOURCES connect,
     the shared vocabulary, and the contracts between repos. This is distinct from
     each source's own per-repo .gogo/knowledge/. Fill it in by hand or via the
     plans-tab "A" plan-with-claude author, which reads this file before writing a
     brief. Delete a heading you do not need. -->

## Domain
<!-- What is ` + name + `? The product / business domain the sources serve, in a few
     lines — the context a plan should assume. -->

## How the sources connect
<!-- The repos (sources) in this project and how they relate: who calls whom, which
     is the front-end / API / worker / data store, the request or data flow between
     them. A cross-repo plan targets several of these at once. -->

## Glossary
<!-- Cross-cutting terms and their meaning across the sources (entities, roles,
     domain nouns) — so the same word means the same thing in every repo. -->

## Integration contracts
<!-- The shared interfaces the sources agree on: API endpoints, event/topic names,
     shared schemas or DTOs, auth/token flows. Changing one side is a cross-repo
     change — note what must move together. -->
`
}

// Load reads the project entity at ~/.gogo/projects/<name>/config.json. A missing,
// unreadable, or malformed file degrades to an EMPTY project (schema stamped, name
// set, no sources) with NO error — mirroring config.Load's defensive style. Only a
// genuinely successful parse returns real sources.
func Load(name string) (*Project, error) {
	empty := &Project{Schema: Schema, Name: name}
	if !validName(name) {
		return empty, nil
	}
	raw, err := os.ReadFile(configPath(name))
	if err != nil {
		return empty, nil // missing / unreadable → empty, never a crash
	}
	var p Project
	if err := json.Unmarshal(raw, &p); err != nil {
		return empty, nil // malformed JSON → empty, never an error
	}
	if p.Schema == 0 {
		p.Schema = Schema
	}
	if p.Name == "" {
		p.Name = name
	}
	return &p, nil
}

// Save writes p's entity under its home folder, creating the directory when
// absent. It always stamps the current schema and appends a trailing newline. A
// project with an invalid Name is refused (the write-scope guard).
func Save(p *Project) error {
	if p == nil {
		return fmt.Errorf("projects: cannot save a nil project")
	}
	if !validName(p.Name) {
		return fmt.Errorf("projects: invalid project name %q", p.Name)
	}
	p.Schema = Schema
	if err := os.MkdirAll(Dir(p.Name), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(configPath(p.Name), raw, 0o644)
}

// List returns every project under ~/.gogo/projects/ (name-sorted, defensive).
// A missing store, or a stray dir with no config.json, degrades to nothing —
// never a crash.
func List() ([]Project, error) {
	entries, err := os.ReadDir(ProjectsDir())
	if err != nil {
		return nil, nil // missing store → empty, never an error
	}
	var out []Project
	for _, e := range entries {
		if !e.IsDir() || !hasConfig(e.Name()) {
			continue
		}
		if p, _ := Load(e.Name()); p != nil {
			out = append(out, *p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Add registers project p by Name: an existing project with that Name is UPDATED
// in place (added=false), a fresh one is created (added=true). Mirrors
// config.Add's dedupe semantics, keyed on the project name (the home-folder key).
func Add(p Project) (added bool, err error) {
	if !validName(p.Name) {
		return false, fmt.Errorf("projects: invalid project name %q", p.Name)
	}
	existed := hasConfig(p.Name)
	if err := Save(&p); err != nil {
		return false, err
	}
	return !existed, nil
}

// AddSource appends s to the named project's sources, deduping by absolute Path:
// an existing source with the same path is UPDATED in place (added=false) rather
// than duplicated. The project is created if it does not yet exist.
func AddSource(name string, s Source) (added bool, err error) {
	p, _ := Load(name)
	if p.Name == "" {
		p.Name = name
	}
	for i := range p.Sources {
		if p.Sources[i].Path == s.Path {
			p.Sources[i] = s
			return false, Save(p)
		}
	}
	p.Sources = append(p.Sources, s)
	return true, Save(p)
}

// RemoveSource removes the FIRST source of the named project whose Path OR Name
// equals key. Returns removed=false (no error, no write) when nothing matched.
func RemoveSource(name, key string) (removed bool, err error) {
	p, _ := Load(name)
	kept := make([]Source, 0, len(p.Sources))
	for _, s := range p.Sources {
		if !removed && (s.Path == key || s.Name == key) {
			removed = true
			continue
		}
		kept = append(kept, s)
	}
	if !removed {
		return false, nil
	}
	p.Sources = kept
	return true, Save(p)
}

// Remove deletes a project's home folder (~/.gogo/projects/<name>/) and returns
// removed=false (no error) when it was absent. It NEVER touches a source's .gogo/
// — the hard invariant. Guarded: name must be a safe single component and the
// resolved dir must sit strictly under ProjectsDir (else it refuses, a no-op).
func Remove(name string) (removed bool, err error) {
	if !validName(name) {
		return false, nil
	}
	dir := Dir(name)
	rel, rerr := filepath.Rel(ProjectsDir(), dir)
	if rerr != nil || rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return false, nil // would escape the projects home → refuse
	}
	if _, serr := os.Stat(dir); serr != nil {
		return false, nil // not present → graceful no-op
	}
	if err := os.RemoveAll(dir); err != nil {
		return false, err
	}
	return true, nil
}

// SkipForSource resolves the per-source gate-skip flags of the SOURCE whose Path ==
// root (FR4), or (false, false) when root is not a registered source — the fallback
// that keeps an unregistered / single repo's gates byte-for-byte. Mirrors
// orchestrator.CapForSource: both `gogo go` launch paths (the CLI and the board)
// resolve their source from the projects store and share this one resolver so the
// two never drift.
func SkipForSource(sources []Source, root string) (planSkip, uatSkip bool) {
	for _, s := range sources {
		if s.Path == root {
			return s.PlanAcceptanceSkip, s.UatAcceptanceSkip
		}
	}
	return false, false
}

// AllSources flattens every project's sources into one slice — what the
// concurrency-cap resolver (orchestrator.CapForSource) reads to look up a repo
// root's per-source cap.
func AllSources(projs []Project) []Source {
	var out []Source
	for _, p := range projs {
		out = append(out, p.Sources...)
	}
	return out
}
