// Package config is the CLI-owned reader/writer for the GLOBAL gogo config —
// the multi-project registry at ~/.config/gogo/projects.json. Unlike the
// contract package (the read-only reader of the per-repo .gogo/ file surface),
// config is deliberately WRITE-capable: the registry is CLI-owned config, not
// pipeline state, so `gogo project add/list/rm` mutate it (the hard "CLI reads,
// skills write" invariant is about .gogo/, never ~/.config/gogo/). Every read is
// defensive — a missing or malformed file degrades to an empty registry (the
// single-repo fallback the board relies on), never a crash.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Schema is the current registry schema version stamped into every saved file.
const Schema = 1

// fileName is the registry file under the resolved config home.
const fileName = "projects.json"

// DefaultMaxConcurrent is the per-project concurrency cap a NEW project defaults
// to (D2): the config screen and `gogo project add` stamp it so a fresh registry
// entry limits itself to one live in-progress feature. Existing Phase-1 entries
// (written before this field) unmarshal MaxConcurrent to 0 (= unlimited), so they
// stay byte-for-byte as today — 0 is the fallback-preserving sentinel.
const DefaultMaxConcurrent = 1

// Project is one registered repo root: an absolute path to the dir that
// contains .gogo/, a display name, an optional card-tag color (hex), and an
// optional per-project concurrency cap (MaxConcurrent).
type Project struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Color is the optional card-tag color (hex) the aggregate board tints this
	// project's cards with.
	Color string `json:"color,omitempty"`
	// MaxConcurrent caps how many of this project's features may be actively
	// worked at once (in-progress phase with a live session) — the launch guard
	// (orchestrator.CapExceeded) refuses a go over it. 0 = UNLIMITED (the sentinel
	// that preserves today's behaviour for an absent field). `omitempty` keeps a
	// zero-cap entry's on-disk shape identical to a Phase-1 file.
	MaxConcurrent int `json:"maxConcurrent,omitempty"`
}

// File is the on-disk shape of ~/.config/gogo/projects.json: a schema version
// plus the ordered list of registered projects.
type File struct {
	Schema   int       `json:"schema"`
	Projects []Project `json:"projects"`
}

// Home resolves the gogo config home directory, honoring (in order):
//
//	$GOGO_CONFIG_HOME           the test seam — points straight at the dir
//	$XDG_CONFIG_HOME/gogo       the XDG base-dir spec
//	$HOME/.config/gogo          the design-exact default
//
// This is the design's literal ~/.config/gogo path on every OS — deliberately
// NOT os.UserConfigDir(), which returns ~/Library/Application Support on macOS
// (a divergence from the mockup). The GOGO_CONFIG_HOME seam lets tests point the
// whole registry at a t.TempDir() so they never touch the real ~/.config.
func Home() string {
	if h := os.Getenv("GOGO_CONFIG_HOME"); h != "" {
		return h
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gogo")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "gogo")
}

// Path is the absolute projects.json path under the resolved config home.
func Path() string { return filepath.Join(Home(), fileName) }

// Load reads the registry. A missing, unreadable, or malformed file degrades to
// an EMPTY registry (schema stamped, no projects) with NO error — the
// single-repo fallback, mirroring the contract parsers' defensive style. Only a
// genuinely successful parse returns real projects.
func Load() (*File, error) {
	empty := &File{Schema: Schema}
	raw, err := os.ReadFile(Path())
	if err != nil {
		return empty, nil // missing / unreadable → empty registry, never a crash
	}
	var parsed File
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return empty, nil // malformed JSON → empty registry, never an error
	}
	if parsed.Schema == 0 {
		parsed.Schema = Schema
	}
	return &parsed, nil
}

// Save writes the registry under the config home, creating the directory when
// absent. It always stamps the current schema and appends a trailing newline.
func Save(f *File) error {
	if f == nil {
		f = &File{}
	}
	f.Schema = Schema
	if err := os.MkdirAll(Home(), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(Path(), raw, 0o644)
}

// List returns the registered projects (empty when none / missing / malformed).
func List() ([]Project, error) {
	f, err := Load()
	if err != nil {
		return nil, err
	}
	return f.Projects, nil
}

// Add registers p, deduping by absolute Path: an existing entry with the same
// path is UPDATED in place (name/color refreshed) rather than duplicated.
// Returns added=false when it replaced an existing entry.
func Add(p Project) (added bool, err error) {
	f, err := Load()
	if err != nil {
		return false, err
	}
	for i := range f.Projects {
		if f.Projects[i].Path == p.Path {
			f.Projects[i] = p
			return false, Save(f)
		}
	}
	f.Projects = append(f.Projects, p)
	return true, Save(f)
}

// Remove deletes the FIRST project whose Name OR Path equals key. Returns
// removed=false (and no error, no write) when nothing matched.
func Remove(key string) (removed bool, err error) {
	f, err := Load()
	if err != nil {
		return false, err
	}
	kept := make([]Project, 0, len(f.Projects))
	for _, p := range f.Projects {
		if !removed && (p.Name == key || p.Path == key) {
			removed = true
			continue
		}
		kept = append(kept, p)
	}
	if !removed {
		return false, nil
	}
	f.Projects = kept
	return true, Save(f)
}
