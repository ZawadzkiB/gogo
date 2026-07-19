package projects

import (
	"os"
	"path/filepath"

	"github.com/ZawadzkiB/gogo/cli/internal/config"
)

// Migrate runs the one-shot, best-effort, NON-destructive migration from the
// legacy flat config.Project registry (~/.config/gogo/projects.json) into
// home-folder projects (~/.gogo/projects/<basename>/) — FR6, D4=A.
//
// It runs at most ONCE: it is skipped the moment ~/.gogo/projects/ exists (a prior
// migration, or a machine that already uses the new store), so it never re-runs
// and is idempotent (a second call is a no-op). It leaves the legacy files in
// place (non-destructive) and NEVER blocks startup — every failure is swallowed
// (best-effort). Each legacy repo entry becomes a single-source project named by
// its basename, the legacy per-project MaxConcurrent carried onto the source's
// ConcurrentWorkItems; two legacy repos sharing a basename fold into one project
// with two sources (deduped by path) rather than clobbering.
//
// Legacy drafts/epics → project plans is Phase C — NOT done here.
func Migrate() {
	// Guard: the store already exists → migration already ran (or was never
	// needed). This is the one-shot latch (D4) and the idempotence guarantee.
	if _, err := os.Stat(ProjectsDir()); err == nil {
		return
	}
	legacy, err := config.List()
	if err != nil || len(legacy) == 0 {
		return // nothing to migrate → leave the store uncreated
	}
	for _, lp := range legacy {
		name := lp.Name
		if name == "" {
			name = filepath.Base(lp.Path)
		}
		if !validName(name) {
			continue // unmappable name → skip (best-effort)
		}
		src := Source{
			Path:                lp.Path,
			Name:                filepath.Base(lp.Path),
			ConcurrentWorkItems: lp.MaxConcurrent,
			Color:               lp.Color,
		}
		// Load-or-init then append (dedupe by path) so same-basename repos fold
		// into one project instead of overwriting each other.
		p, _ := Load(name)
		if p.Name == "" {
			p.Name = name
		}
		if !hasSourcePath(p.Sources, src.Path) {
			p.Sources = append(p.Sources, src)
		}
		_ = Save(p) // best-effort; a write failure never blocks startup
	}
}

// hasSourcePath reports whether sources already carries a source at path.
func hasSourcePath(sources []Source, path string) bool {
	for _, s := range sources {
		if s.Path == path {
			return true
		}
	}
	return false
}
