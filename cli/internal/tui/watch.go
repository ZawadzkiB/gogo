package tui

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// watchDebounce coalesces a burst of fsnotify events into one reload.
const watchDebounce = 300 * time.Millisecond

// featureSubdirs are the per-feature folders that receive writes during the
// pipeline. fsnotify is non-recursive, so each must be watched explicitly.
var featureSubdirs = []string{"review", "test", "report", "charts", "implement"}

// reloadMsg tells the board to re-read the repo (debounced fsnotify signal).
type reloadMsg struct{}

// watcherReadyMsg hands the started watcher back to the model so reload() can
// re-arm it (REV-010) and quit can close it (REV-011).
type watcherReadyMsg struct{ ws *watchSet }

// waitForReload blocks until the watcher signals, then emits a reloadMsg. It is
// re-issued after each reload so the board keeps updating itself live.
func waitForReload(ch chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-ch
		return reloadMsg{}
	}
}

// watchPaths is the full set of directories the board should watch for a repo:
// .gogo/work, .gogo/changelog, every feature dir + its known subdirs, and every
// changelog entry dir. Non-recursive by design (fsnotify), so subdirs are
// enumerated explicitly; features/entries created mid-session are picked up on
// the next reconcile (REV-010).
func watchPaths(root string, repo *contract.Repo) []string {
	base := filepath.Join(root, ".gogo")
	paths := []string{
		filepath.Join(base, "work"),
		filepath.Join(base, "changelog"),
	}
	if repo != nil {
		for _, f := range repo.Features {
			paths = append(paths, f.Dir)
			for _, sub := range featureSubdirs {
				paths = append(paths, filepath.Join(f.Dir, sub))
			}
		}
		for _, c := range repo.Changelog {
			paths = append(paths, c.Dir)
		}
	}
	return paths
}

// watchDirs is the full directory set this board watches: the single root's tree in
// single-repo mode, or the UNION across every SOURCE root the board spans (each source's
// per-root path set from watchPaths, deduped). On the UNIFIED board that is EVERY
// project's sources (capWatchSources / projects.AllSources) — a card's source may live
// in a non-focused project, so watching only the focused project's sources left it
// unwatched (FR5); off the unified board it is the focused project's sources. fsnotify
// is non-recursive, so watchPaths enumerates the subdirs per root and this only unions them.
func (m Model) watchDirs() []string {
	if !m.global() {
		return watchPaths(m.root, m.repo)
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range m.capWatchSources() {
		for _, path := range watchPaths(s.Path, m.repo) {
			if !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
		}
	}
	return out
}

// watchSet is a long-lived fsnotify watcher plus the set of directories it is
// currently armed on. It is created once (startWatchCmd), re-armed on every
// reload (reconcile), and torn down on quit (close). All watched-set mutation
// happens on the Bubble Tea Update goroutine; the fsnotify goroutine only reads
// its channels — so no lock is needed on `watched`.
type watchSet struct {
	w         *fsnotify.Watcher
	ch        chan struct{} // reload signal, shared with waitForReload
	done      chan struct{} // closed on shutdown: stops the goroutine + guards sends
	watched   map[string]bool
	closeOnce sync.Once
}

// reconcile makes the armed set equal to want: it arms directories that have
// appeared (a feature born mid-session — REV-010) and unwatches directories
// that vanished (REV-011 graceful removal). fsnotify.Add is idempotent per
// path; a path that does not exist yet is skipped and retried next reconcile.
// Returns the directories newly armed on this call.
func (ws *watchSet) reconcile(want []string) []string {
	wantSet := make(map[string]bool, len(want))
	for _, p := range want {
		wantSet[p] = true
	}
	for p := range ws.watched {
		if !wantSet[p] {
			_ = ws.w.Remove(p) // deleted dir may already be gone — ignore
			delete(ws.watched, p)
		}
	}
	var added []string
	for _, p := range want {
		if ws.watched[p] {
			continue
		}
		if err := ws.w.Add(p); err != nil {
			continue // not yet on disk (e.g. an empty feature's subdirs)
		}
		ws.watched[p] = true
		added = append(added, p)
	}
	return added
}

// fire signals a reload without blocking, dropping the signal if one is already
// queued (buffered chan) or if we are shutting down. The done guard makes a late
// debounce-timer callback a safe no-op after close.
func (ws *watchSet) fire() {
	select {
	case <-ws.done:
		return
	default:
	}
	select {
	case ws.ch <- struct{}{}:
	default:
	}
}

// start runs the debounced event loop until the watcher closes or done fires.
func (ws *watchSet) start() {
	go func() {
		var timer *time.Timer
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()
		for {
			select {
			case <-ws.done:
				return
			case _, ok := <-ws.w.Events:
				if !ok {
					return
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(watchDebounce, ws.fire)
			case _, ok := <-ws.w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

// close stops the reload goroutine and closes the watcher exactly once. Safe to
// call on a nil set (tests that never start a watcher).
func (ws *watchSet) close() error {
	if ws == nil {
		return nil
	}
	var err error
	ws.closeOnce.Do(func() {
		close(ws.done)
		if ws.w != nil {
			err = ws.w.Close()
		}
	})
	return err
}

// startWatchCmd creates the watcher, arms it against the startup snapshot, and
// starts the debounced loop, handing the set back via watcherReadyMsg.
// Best-effort: any error just disables live refresh (manual reload still works).
func (m Model) startWatchCmd() tea.Cmd {
	dirs := m.watchDirs() // single root, or the union across all project roots
	ch := m.reloadCh
	return func() tea.Msg {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return nil
		}
		ws := &watchSet{
			w:       w,
			ch:      ch,
			done:    make(chan struct{}),
			watched: map[string]bool{},
		}
		ws.reconcile(dirs)
		ws.start()
		return watcherReadyMsg{ws: ws}
	}
}
