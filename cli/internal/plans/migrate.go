package plans

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/config"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// migratedMarker is the one-shot latch written under the DATA home once the legacy
// drafts/epics fold has run, so MigrateLegacy never re-scans on later startups.
const migratedMarker = ".legacy-plans-migrated"

// MigrateLegacy runs the one-shot, best-effort, NON-destructive fold of the legacy
// GLOBAL stores — `~/.config/gogo/drafts/<slug>.md` (P3) and
// `~/.config/gogo/epics.json` (P4) — into PROJECT-scoped plans (FR6 extension, D4).
// It is the Phase-C completion of projects.Migrate (which folds the flat
// projects.json into home-folder projects): this runs AFTER it, so the projects a
// legacy draft/epic targets already exist.
//
// It runs at most ONCE (guarded by a marker under the DATA home) and is idempotent
// (each plan id is minted deterministically from the legacy title + its stored
// created stamp, so a re-run mints the same id and skips an already-migrated plan).
// It leaves the legacy files in place (non-destructive) and NEVER blocks startup —
// every failure is swallowed (best-effort). A draft/epic whose target source is NOT
// resolvable to a known project is skipped (no home for it), never guessed.
//
//   - a legacy EPIC → an `active` plan (it already has members) in the project owning
//     its member repos; members mapped to (source-name, slugHint).
//   - a legacy DRAFT → a `draft` plan in the project resolved from its advisory
//     target (a legacy registry name/path → source path → owning project).
func MigrateLegacy() {
	dataHome := projects.Home()
	marker := filepath.Join(dataHome, migratedMarker)
	if _, err := os.Stat(marker); err == nil {
		return // already migrated (or a machine that never had legacy stores)
	}

	projs, _ := projects.List()
	if len(projs) > 0 { // nothing to map onto if there are no projects yet
		migrateLegacyEpics(projs)
		migrateLegacyDrafts(projs)
	}

	// Only create the data home + latch when there is a REASON to (REV-007): either the
	// data home already exists (so stamping the marker is not a surprise creation), or a
	// legacy store is actually present (there was something to fold). A machine that
	// never used gogo — no ~/.gogo, no legacy ~/.config/gogo drafts/epics — is left
	// UNTOUCHED: no surprise ~/.gogo creation on a first `gogo` invocation. The one-shot
	// re-scan on such a box is cheap (a couple of stats) until real content appears.
	if !dirExists(dataHome) && !hasLegacyStores() {
		return
	}
	// Stamp the latch (best-effort; a write failure just means we retry next startup).
	_ = os.MkdirAll(dataHome, 0o755)
	_ = os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// hasLegacyStores reports whether a legacy ~/.config/gogo store MigrateLegacy folds is
// present — the drafts dir or epics.json. The guard that keeps a no-op run from
// creating ~/.gogo on a machine that never used gogo.
func hasLegacyStores() bool {
	if _, err := os.Stat(filepath.Join(config.Home(), "epics.json")); err == nil {
		return true
	}
	if info, err := os.Stat(filepath.Join(config.Home(), "drafts")); err == nil && info.IsDir() {
		return true
	}
	return false
}

// migrateLegacyEpics folds ~/.config/gogo/epics.json into active project plans.
func migrateLegacyEpics(projs []projects.Project) {
	raw, err := os.ReadFile(filepath.Join(config.Home(), "epics.json"))
	if err != nil {
		return
	}
	var file struct {
		Epics []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Created     string `json:"created"`
			Members     []struct {
				Repo     string `json:"repo"`
				SlugHint string `json:"slugHint"`
			} `json:"members"`
		} `json:"epics"`
	}
	if json.Unmarshal(raw, &file) != nil {
		return
	}
	for _, e := range file.Epics {
		// Map every member repo to a (project, source) — the epic lands in the FIRST
		// project any member resolves into (a cross-project epic is folded into one).
		project := ""
		var members []Member
		targets := map[string]bool{}
		var order []string
		for _, m := range e.Members {
			pName, sName, ok := resolveSourceByPath(projs, m.Repo)
			if !ok {
				continue
			}
			if project == "" {
				project = pName
			}
			if pName != project {
				continue // keep the epic within one project (best-effort)
			}
			members = append(members, Member{Source: sName, SlugHint: m.SlugHint})
			if !targets[sName] {
				targets[sName] = true
				order = append(order, sName)
			}
		}
		if project == "" {
			continue // no member resolved to a known project → skip (no home)
		}
		id := MintID(e.Title, parseStamp(e.Created))
		if _, exists := Get(project, id); exists {
			continue // idempotent — already migrated
		}
		_ = Save(project, Plan{
			ID:          id,
			Title:       e.Title,
			Description: e.Description,
			Status:      StatusActive, // an epic already has members
			Targets:     order,
			Members:     members,
			Created:     e.Created,
		})
	}
}

// migrateLegacyDrafts folds ~/.config/gogo/drafts/<slug>.md into draft project plans.
func migrateLegacyDrafts(projs []projects.Project) {
	dir := filepath.Join(config.Home(), "drafts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".md") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, ent.Name()))
		if rerr != nil {
			continue
		}
		title, created, target, body := parseLegacyDraft(raw)
		project, ok := resolveDraftProject(projs, target)
		if !ok {
			continue // no resolvable target → skip (no home)
		}
		id := MintID(title, parseStamp(created))
		if _, exists := Get(project, id); exists {
			continue // idempotent
		}
		_ = Save(project, Plan{
			ID:          id,
			Title:       title,
			Description: body,
			Status:      StatusDraft,
			Created:     created,
		})
	}
}

// parseLegacyDraft pulls the title/created/target front-matter + body from a legacy
// draft .md (the P3 shape), tolerating a missing fence (all body).
func parseLegacyDraft(raw []byte) (title, created, target, body string) {
	lines := strings.Split(string(raw), "\n")
	i := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == frontFence {
		close := -1
		for j := 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == frontFence {
				close = j
				break
			}
		}
		if close >= 0 {
			for _, ln := range lines[1:close] {
				k, v, ok := strings.Cut(ln, ":")
				if !ok {
					continue
				}
				switch strings.ToLower(strings.TrimSpace(k)) {
				case "title":
					title = strings.TrimSpace(v)
				case "created":
					created = strings.TrimSpace(v)
				case "target":
					target = strings.TrimSpace(v)
				}
			}
			i = close + 1
		}
	}
	body = strings.Trim(strings.Join(lines[i:], "\n"), "\n")
	return title, created, target, body
}

// resolveDraftProject resolves a legacy draft's advisory target (a flat-registry
// name OR path) to a home project: match the legacy registry entry by name/path,
// take its path, and find the project owning a source at that path. An empty target
// falls back to the sole project (there is only one place it can go).
func resolveDraftProject(projs []projects.Project, target string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		if len(projs) == 1 {
			return projs[0].Name, true
		}
		return "", false
	}
	// Resolve the target against the legacy flat registry (name or path) to a path.
	path := target
	if legacy, _ := config.List(); len(legacy) > 0 {
		for _, lp := range legacy {
			if lp.Name == target || lp.Path == target {
				path = lp.Path
				break
			}
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = filepath.Clean(abs)
	}
	if pName, _, ok := resolveSourceByPath(projs, path); ok {
		return pName, true
	}
	return "", false
}

// resolveSourceByPath finds the (project, source) owning repoPath (cleaned-path
// match) across the home projects.
func resolveSourceByPath(projs []projects.Project, repoPath string) (project, source string, ok bool) {
	want := repoPath
	if abs, err := filepath.Abs(repoPath); err == nil {
		want = filepath.Clean(abs)
	}
	for _, p := range projs {
		for _, s := range p.Sources {
			sp := s.Path
			if abs, err := filepath.Abs(sp); err == nil {
				sp = filepath.Clean(abs)
			}
			if sp == want {
				name := s.Name
				if name == "" {
					name = filepath.Base(s.Path)
				}
				return p.Name, name, true
			}
		}
	}
	return "", "", false
}

// parseStamp parses an RFC3339 created stamp for deterministic id minting; an
// unparseable/empty stamp falls back to the zero time (still deterministic).
func parseStamp(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}
