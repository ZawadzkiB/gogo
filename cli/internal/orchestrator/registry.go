package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Persistent-session lifecycle statuses (FR7). A session is `running` while its
// `-p` child (or attach tmux) is live, `parked` when it exited at a decision gate,
// `awaiting-uat` when it exited green at the UAT gate, `shipped` once its feature
// is terminal, and `reaped` once the reaper killed it.
const (
	SessRunning     = "running"
	SessParked      = "parked"
	SessAwaitingUAT = "awaiting-uat"
	SessShipped     = "shipped"
	SessReaped      = "reaped"
)

// Registry is the CLI-owned bookkeeping for one feature's persistent sessions
// (FR7). It lives under .gogo/resources/cli/sessions/<slug>.json — the CLI's
// sanctioned write root — and NEVER inside the feature folder, because the CLI
// must not mutate pipeline state (hard invariant + the frozen consumer contract).
// It tracks the feature's persistent session(s) keyed by leg kind (`go` | `plan`)
// so a re-launched `gogo go`/`gogo plan` --resumes the SAME warm session instead
// of a cold restart, plus per-run cost/turns telemetry. A missing or garbled
// registry degrades to a fresh run — never a crash (unchanged invariant).
type Registry struct {
	Slug       string                        `json:"slug"`
	Persistent map[string]*PersistentSession `json:"persistent,omitempty"` // keyed by kind: "go" | "plan"
	CostUSD    float64                       `json:"total_cost_usd"`       // summed across every recorded run
	Sessions   []SessionInfo                 `json:"sessions"`             // per-run telemetry, append-only
}

// PersistentSession is the tracked lifecycle of one feature leg's persistent
// `claude` session (FR7): the stable uuid re-launches --resume, the tmux name is
// the reap target (attach mode), the status drives the exit surface + kill-at-ship,
// and the cost/turns telemetry backs a later cost-surfacing slice.
type PersistentSession struct {
	Kind      string  `json:"kind"`           // go | plan (the leg this session drives)
	UUID      string  `json:"uuid"`           // claude --session-id / --resume uuid (stable across resumes)
	Tmux      string  `json:"tmux,omitempty"` // tmux session name when launched with --attach, else ""
	PID       int     `json:"pid,omitempty"`  // last driver PID
	Status    string  `json:"status"`         // running | parked | awaiting-uat | shipped | reaped
	StartedAt string  `json:"started_at"`     // RFC3339, first launch
	UpdatedAt string  `json:"updated_at"`     // RFC3339, last transition
	CostUSD   float64 `json:"cost_usd"`       // summed cost for this leg
	NumTurns  int     `json:"num_turns"`      // summed turns for this leg
}

// SessionInfo is one `claude -p` run's telemetry — the append-only FR7 record a
// later cost/telemetry-surfacing slice reads.
type SessionInfo struct {
	Kind       string  `json:"kind"` // go | plan
	UUID       string  `json:"uuid"`
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
// fresh Registry (never an error): degrade to first run, never crash (FR7). A
// legacy `gogo run` registry (dev_uuid / round / phase keys, no `persistent`
// block) loads with an empty Persistent map — its old fields are ignored and the
// first `gogo go` starts a fresh persistent session.
func LoadRegistry(root, slug string) *Registry {
	fresh := &Registry{Slug: slug, Persistent: map[string]*PersistentSession{}}
	data, err := os.ReadFile(RegistryPath(root, slug))
	if err != nil {
		return fresh
	}
	var loaded Registry
	if json.Unmarshal(data, &loaded) != nil {
		return fresh // garbled → first run
	}
	loaded.Slug = slug
	if loaded.Persistent == nil {
		loaded.Persistent = map[string]*PersistentSession{}
	}
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

// Get returns the tracked persistent session for a leg kind, or nil.
func (r *Registry) Get(kind string) *PersistentSession {
	if r.Persistent == nil {
		return nil
	}
	return r.Persistent[kind]
}

// Ensure returns the persistent session for a leg kind, creating it if absent.
func (r *Registry) Ensure(kind string) *PersistentSession {
	if r.Persistent == nil {
		r.Persistent = map[string]*PersistentSession{}
	}
	ps := r.Persistent[kind]
	if ps == nil {
		ps = &PersistentSession{Kind: kind}
		r.Persistent[kind] = ps
	}
	return ps
}

// record appends a run's telemetry and updates the running cost total.
func (r *Registry) record(s SessionInfo) {
	r.Sessions = append(r.Sessions, s)
	r.CostUSD += s.CostUSD
}
