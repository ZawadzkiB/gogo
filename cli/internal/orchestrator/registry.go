package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Registry is the CLI-owned bookkeeping for one feature's orchestrated run (FR9).
// It lives under .gogo/resources/cli/sessions/<slug>.json — the CLI's sanctioned
// write root — and NEVER inside the feature folder, because the CLI must not mutate
// pipeline state (hard invariant + the frozen consumer contract). It records the
// warm developer session's uuid (so a re-launched `gogo run` resumes the SAME warm
// session instead of a cold rebuild), the loop position, and per-run cost telemetry.
// A missing or garbled registry degrades to a fresh run — never a crash.
type Registry struct {
	Slug     string        `json:"slug"`
	DevUUID  string        `json:"dev_uuid"`       // the warm developer session (stable across rounds)
	Round    int           `json:"round"`          // current implement↔review round
	Phase    string        `json:"phase"`          // last phase the loop entered
	CostUSD  float64       `json:"total_cost_usd"` // summed across every session (bounds check)
	Sessions []SessionInfo `json:"sessions"`       // per-run telemetry, append-only
}

// SessionInfo is one `claude -p` run's telemetry — the FR9 record and the source
// the later cost/telemetry surfacing (FR12) reads.
type SessionInfo struct {
	Kind       string  `json:"kind"` // implement | review | test | report
	UUID       string  `json:"uuid"`
	Round      int     `json:"round"`
	Resumed    bool    `json:"resumed"` // true = --resume (warm), false = --session-id (fresh)
	CostUSD    float64 `json:"cost_usd"`
	NumTurns   int     `json:"num_turns"`
	DurationMS int     `json:"duration_ms"`
}

// RegistryPath is <root>/.gogo/resources/cli/sessions/<slug>.json.
func RegistryPath(root, slug string) string {
	return filepath.Join(root, ".gogo", "resources", "cli", "sessions", slug+".json")
}

// LoadRegistry reads a feature's registry. A missing or unparseable file yields a
// fresh Registry (never an error): degrade to first run, never crash (FR9).
func LoadRegistry(root, slug string) *Registry {
	fresh := &Registry{Slug: slug}
	data, err := os.ReadFile(RegistryPath(root, slug))
	if err != nil {
		return fresh
	}
	var loaded Registry
	if json.Unmarshal(data, &loaded) != nil {
		return fresh // garbled → first run
	}
	loaded.Slug = slug
	return &loaded
}

// Save writes the registry under .gogo/resources/cli/sessions/. Best-effort: a
// write failure is returned to the caller but can never corrupt pipeline state
// (this is CLI-only bookkeeping outside the contract surface).
func (r *Registry) Save(root string) error {
	path := RegistryPath(root, r.Slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// record appends a session's telemetry and updates the running cost total.
func (r *Registry) record(s SessionInfo) {
	r.Sessions = append(r.Sessions, s)
	r.CostUSD += s.CostUSD
}
