package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
)

// SweepTTL is the age past which a `gogo-*` tmux session is treated as an orphan
// even if it still attributes to a live, non-terminal feature — a backstop for a
// wedged session that never exits. Applied to the registry's started_at when
// parseable; best-effort (a session with no registry timestamp is judged by the
// owner rules alone).
const SweepTTL = 24 * time.Hour

// Sweeper reaps stale/orphaned persistent sessions (FR9). It kills (1) `gogo-*`
// tmux sessions whose owning feature is terminal (the kill-at-ship backstop) and
// (2) orphans — a live `gogo-*` session with no live, non-terminal owning feature.
// Attribution is by exact launch.SessionMatchesSlug (never substring, TEST-005).
type Sweeper struct {
	Root   string
	Repo   *contract.Repo
	List   func() []string         // list live gogo-* tmux sessions; nil → launch.ListSessions
	Kill   func(name string) error // kill a tmux session; nil → launch.KillSession
	Out    io.Writer
	DryRun bool
}

func (sw *Sweeper) defaults() {
	if sw.List == nil {
		sw.List = launch.ListSessions
	}
	if sw.Kill == nil {
		sw.Kill = launch.KillSession
	}
}

// Sweep reaps the sessions per the FR9 rules and returns the session names it
// killed (or, with DryRun, would kill). It also best-effort cleans up stale
// lockfiles + marks terminal features' registries reaped, but those are not part
// of the returned set (which is the tmux sessions — the incident's actual leak).
func (sw *Sweeper) Sweep() []string {
	sw.defaults()
	var killed []string
	for _, sess := range sw.List() {
		feat := sw.owningFeature(sess)
		reap, why := sw.shouldReap(sess, feat)
		if !reap {
			continue
		}
		killed = append(killed, sess)
		if sw.DryRun {
			sw.printf("would reap %s (%s)\n", sess, why)
			continue
		}
		if err := sw.Kill(sess); err != nil {
			sw.printf("reap %s failed: %v\n", sess, err)
		} else {
			sw.printf("reaped %s (%s)\n", sess, why)
		}
	}
	if !sw.DryRun {
		sw.cleanupTerminalRegistries()
		sw.cleanupStaleLocks()
	}
	if len(killed) == 0 {
		sw.printf("nothing to reap — no orphaned or terminal-feature sessions.\n")
	}
	return killed
}

// owningFeature returns the feature a session attributes to (exact slug parse), or
// nil when no feature matches (an orphan).
func (sw *Sweeper) owningFeature(sess string) *contract.Feature {
	if sw.Repo == nil {
		return nil
	}
	for _, f := range sw.Repo.Features {
		if launch.SessionMatchesSlug(sess, f.Slug) {
			return f
		}
	}
	return nil
}

// shouldReap applies the FR9 rules: no owning feature → orphan; terminal owning
// feature → kill-at-ship backstop; a session past SweepTTL → wedged-session
// backstop. A live, non-terminal owning feature within TTL is spared.
func (sw *Sweeper) shouldReap(sess string, feat *contract.Feature) (bool, string) {
	if feat == nil {
		return true, "orphan — no owning feature"
	}
	if TerminalStatus(feat.Status) {
		return true, "owning feature " + feat.Slug + " is " + feat.Status
	}
	if sw.overTTL(feat.Slug) {
		return true, "older than the " + SweepTTL.String() + " sweep TTL"
	}
	return false, ""
}

// overTTL reports whether the feature's tracked persistent session started longer
// than SweepTTL ago (best-effort: no registry / unparseable timestamp → false).
func (sw *Sweeper) overTTL(slug string) bool {
	reg := LoadRegistry(sw.Root, slug)
	for _, ps := range reg.Persistent {
		if ps == nil || ps.StartedAt == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, ps.StartedAt)
		if err != nil {
			continue
		}
		if time.Since(t) > SweepTTL {
			return true
		}
	}
	return false
}

// cleanupTerminalRegistries marks a terminal feature's tracked sessions reaped and
// releases its lock (the registry/lock side of kill-at-ship; the tmux kill already
// happened in the session scan).
func (sw *Sweeper) cleanupTerminalRegistries() {
	if sw.Repo == nil {
		return
	}
	for _, f := range sw.Repo.Features {
		if !TerminalStatus(f.Status) {
			continue
		}
		reg := LoadRegistry(sw.Root, f.Slug)
		changed := false
		for _, ps := range reg.Persistent {
			if ps != nil && ps.Status != SessReaped {
				ps.Status = SessReaped
				ps.UpdatedAt = now()
				changed = true
			}
		}
		if changed {
			_ = reg.Save(sw.Root)
		}
		_ = releaseLock(sw.Root, f.Slug)
	}
}

// cleanupStaleLocks removes lockfiles whose owning feature is terminal or whose
// owner is no longer live (self-healing stale locks — D1's third promise).
func (sw *Sweeper) cleanupStaleLocks() {
	dir := filepath.Join(sw.Root, ".gogo", "resources", "cli", "locks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".lock" {
			continue
		}
		slug := e.Name()[:len(e.Name())-len(".lock")]
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var o Owner
		if json.Unmarshal(data, &o) != nil {
			continue
		}
		feat := sw.featureFor(slug)
		terminal := feat != nil && TerminalStatus(feat.Status)
		if terminal || !DefaultLiveness(o, slug) {
			_ = releaseLock(sw.Root, slug)
		}
	}
}

func (sw *Sweeper) featureFor(slug string) *contract.Feature {
	if sw.Repo == nil {
		return nil
	}
	return sw.Repo.Feature(slug)
}

func (sw *Sweeper) printf(format string, a ...any) {
	if sw.Out != nil {
		fmt.Fprintf(sw.Out, format, a...)
	}
}
