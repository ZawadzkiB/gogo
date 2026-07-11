package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ZawadzkiB/gogo/cli/internal/launch"
)

// The one-owner-per-work-item lock (D1=C, FR5/FR6). The incident that motivated
// this feature was a chat session and a board-launched tmux session silently
// double-driving the SAME feature. A pure lockfile goes stale on a crash; a pure
// live-session scan misses a headless `-p` run that never opened a tmux session.
// So the lock is BOTH: a lockfile recording the owner (PID + uuid + tmux + host +
// started-at), whose liveness is cross-checked against signal-0 AND a matching
// live `gogo-*` tmux session. Either signal alive → live (refuse / --takeover);
// both dead → stale and silently reclaimable.

// Owner records who holds a work-item lock — enough to cross-check liveness and
// to tell the user WHO holds it when a launch is refused.
type Owner struct {
	PID       int    `json:"pid"`
	UUID      string `json:"uuid"`           // the claude session uuid this owner drives
	Tmux      string `json:"tmux,omitempty"` // tmux session name (--attach), else "" (headless -p)
	Host      string `json:"host"`
	Kind      string `json:"kind"`       // go | plan
	StartedAt string `json:"started_at"` // RFC3339
}

// Lock is a held owner lock over one work item. Release removes the lockfile.
type Lock struct {
	Path  string
	Owner Owner
}

// LivenessFn reports whether a recorded owner is still live. Injectable so tests
// assert refuse / reclaim / takeover with no real process or tmux.
type LivenessFn func(o Owner, slug string) bool

// DefaultLiveness is the production cross-check (FR6): an owner is live iff its
// PID answers signal-0 OR a matching `gogo-*` tmux session is running (exact
// SessionMatchesSlug parse — never substring, TEST-005). Either signal alive →
// live; both dead → stale/reclaimable. This is what catches the incident's
// board-launched tmux racer (whose PID we never owned).
func DefaultLiveness(o Owner, slug string) bool {
	if launch.PidAlive(o.PID) {
		return true
	}
	for _, s := range launch.ListSessions() {
		if launch.SessionMatchesSlug(s, slug) {
			return true
		}
	}
	return false
}

// LockPath is <root>/.gogo/resources/cli/locks/<slug>.lock.
func LockPath(root, slug string) string {
	return filepath.Join(root, ".gogo", "resources", "cli", "locks", slug+".lock")
}

// Acquire tries to take the owner lock for slug. The outcomes:
//   - a live prior owner (or a live UNTRACKED gogo-* session — the board racer)
//     and takeover=false → REFUSE: returns (nil, true, &prior, nil) and writes
//     nothing durable (the caller must not launch).
//   - a stale prior owner (both liveness signals dead) → RECLAIM: overwrites the
//     lockfile and returns (lock, false, &prior, nil) — prior is returned so the
//     caller can best-effort reap any leftover session.
//   - a live prior/untracked owner and takeover=true → SEIZE: writes the lock and
//     returns (lock, false, &prior, nil) — the caller reaps the displaced prior.
//   - genuinely free → returns (lock, false, nil, nil).
//
// The fresh-slug create is ATOMIC (O_CREATE|O_EXCL): of two simultaneous first
// launches only one wins the create, closing the double-launch race (REV-003).
// Even after a clean create, a live gogo-* session for the slug with NO lockfile
// (a board-launched racer never wrote one) is still an owner — Acquire refuses to
// it too, which is the incident this lock exists to catch (D1=C / FR6). live is the
// injectable cross-check (nil → DefaultLiveness). Acquire never launches and never
// kills — the reap decision stays with the orchestrator that owns the seams.
func Acquire(root, slug string, me Owner, takeover bool, live LivenessFn) (*Lock, bool, *Owner, error) {
	if live == nil {
		live = DefaultLiveness
	}
	path := LockPath(root, slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, false, nil, err
	}

	// 1. Atomic exclusive create — the common fresh case + the race closer.
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); err == nil {
		writeOwner(f, me)
		_ = f.Close()
		// A live untracked session (board-launched, no lockfile) still owns the slug.
		if live(Owner{}, slug) {
			if !takeover {
				_ = os.Remove(path) // we don't own the work — release our just-made lock
				return nil, true, untrackedOwner(), nil
			}
			return &Lock{Path: path, Owner: me}, false, untrackedOwner(), nil // caller reaps by slug
		}
		return &Lock{Path: path, Owner: me}, false, nil, nil
	} else if !os.IsExist(err) {
		return nil, false, nil, err // a real create error (perm, etc.)
	}

	// 2. A lockfile already exists → read its owner + evaluate liveness.
	var prior *Owner
	if data, err := os.ReadFile(path); err == nil {
		var p Owner
		if json.Unmarshal(data, &p) == nil && (p.PID != 0 || p.UUID != "" || p.Tmux != "") {
			prior = &p
		}
	}
	if prior != nil && live(*prior, slug) && !takeover {
		return nil, true, prior, nil // refuse: a live owner already holds it
	}
	// stale (reclaim) or live+takeover (seize) → overwrite + grant.
	if err := writeLock(path, me); err != nil {
		return nil, false, prior, err
	}
	return &Lock{Path: path, Owner: me}, false, prior, nil
}

// SetTmux updates the held lock's recorded tmux session name and rewrites the
// lockfile, so the owner reflects the REAL (post-collision) session name. The
// --attach path calls it after LaunchPersistent picks a unique name, so a later
// reclaim/takeover reaps the right session and the refusal hint points at the
// right pane (REV-002).
func (l *Lock) SetTmux(name string) error {
	if l == nil {
		return nil
	}
	l.Owner.Tmux = name
	return writeLock(l.Path, l.Owner)
}

// Release removes the lockfile (best-effort; a missing file is not an error).
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	err := os.Remove(l.Path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// untrackedOwner is the synthesized prior for a live gogo-* session that holds no
// lockfile (a board-launched racer) — enough to drive the refusal message + the
// reap-by-slug path; the real session name is discovered by SessionMatchesSlug.
func untrackedOwner() *Owner { return &Owner{Kind: "untracked"} }

// releaseLock removes a feature's lockfile without a held Lock handle — used by
// the reaper and sweep. Best-effort.
func releaseLock(root, slug string) error {
	err := os.Remove(LockPath(root, slug))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func writeLock(path string, me Owner) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(me, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// writeOwner marshals an owner into an already-open (O_EXCL) lockfile — the atomic
// create path in Acquire, where the file must not be re-opened.
func writeOwner(f *os.File, me Owner) {
	if data, err := json.MarshalIndent(me, "", "  "); err == nil {
		_, _ = f.Write(data)
	}
}
