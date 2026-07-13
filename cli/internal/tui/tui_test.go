package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

const fixtureRoot = "../contract/testdata/repo"

func newModel(t *testing.T) Model {
	t.Helper()
	m := New(fixtureRoot)
	// give it a size so View + viewport are exercised
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	return nm.(Model)
}

func send(m Model, msg tea.Msg) Model {
	nm, _ := m.Update(msg)
	return nm.(Model)
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// right moves one column right via the arrow key. Column-right is arrow-only
// since `l` became the log-peek key (FR7); `h`/`j`/`k` stay as vim aliases.
func right(m Model) Model { return send(m, tea.KeyMsg{Type: tea.KeyRight}) }

func TestBoardColumnsPopulated(t *testing.T) {
	m := newModel(t)
	// plan=3 (unfinished, aborted, malformed), in-progress=1, ready=2, changelog=3
	want := [4]int{3, 1, 2, 3}
	for i := range want {
		if len(m.cols[i]) != want[i] {
			t.Errorf("column %d (%s) = %d, want %d", i, columnTitles[i], len(m.cols[i]), want[i])
		}
	}
}

func TestBoardViewRenders(t *testing.T) {
	m := newModel(t)
	out := m.View()
	// FR-2 restyles the column header (underlined title + trailing dim count, no
	// (N) parens); FR-6 makes the changelog count read "N shipped". FR-4/FR-8 add
	// the phase dots + the needs-you strip — pinned here so the redesign stays
	// visibly present (a token diff would fail this, not a palette port).
	for _, want := range []string{
		"cockpit", "plan 3", "in progress 1", "ready 2", "changelog 3 shipped",
		"①②③④⑤", "⏸ NEEDS YOU (1)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("board view missing %q", want)
		}
	}
}

func TestNavigation(t *testing.T) {
	m := newModel(t)
	if m.colIdx != 0 {
		t.Fatalf("start colIdx = %d", m.colIdx)
	}
	m = right(m) // right (arrow — `l` is now peek)
	m = right(m)
	if m.colIdx != 2 {
		t.Errorf("after 2×right colIdx = %d, want 2", m.colIdx)
	}
	m = send(m, runes("j")) // down within ready (2 cards)
	if m.cardIdx[2] != 1 {
		t.Errorf("after down cardIdx = %d, want 1", m.cardIdx[2])
	}
	// clamp at edges
	m = send(m, runes("j"))
	if m.cardIdx[2] != 1 {
		t.Errorf("down should clamp at 1, got %d", m.cardIdx[2])
	}
	m = send(m, runes("h"))
	m = send(m, runes("h"))
	m = send(m, runes("h"))
	if m.colIdx != 0 {
		t.Errorf("left should clamp at 0, got %d", m.colIdx)
	}
}

func TestFilter(t *testing.T) {
	m := newModel(t)
	m = send(m, runes("/"))
	if !m.filtering {
		t.Fatal("/ should start filtering")
	}
	for _, r := range "shipped" {
		m = send(m, runes(string(r)))
	}
	// Only the 3 shipped features match "shipped" in their slug.
	total := 0
	for i := range m.cols {
		total += len(m.cols[i])
	}
	if total != 3 {
		t.Errorf("filtered total = %d, want 3", total)
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.filter != "" || m.filtering {
		t.Errorf("esc should clear filter")
	}
}

func TestSelectionOnlyReady(t *testing.T) {
	m := newModel(t)
	// focus column 0 (plan) — space must bounce, not select.
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	if len(m.selectedSlugs()) != 0 {
		t.Errorf("selected a plan card: %v", m.selectedSlugs())
	}
	if !strings.Contains(m.status, "select only ready") {
		t.Errorf("no bounce status, got %q", m.status)
	}
	// move to ready column, select a card.
	m = right(m)
	m = right(m)
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	if len(m.selectedSlugs()) != 1 {
		t.Errorf("ready selection = %v, want 1", m.selectedSlugs())
	}
}

func TestAttemptActionGuards(t *testing.T) {
	m := newModel(t)

	// plan column card → go
	in, ship, bounce := m.attemptAction(false)
	if bounce != "" || ship || in.Action != launch.ActionGo {
		t.Errorf("plan m: intent=%+v ship=%v bounce=%q", in, ship, bounce)
	}

	// in-progress card → go (resume)
	m.colIdx = 1
	in, _, bounce = m.attemptAction(false)
	if bounce != "" || in.Action != launch.ActionGo {
		t.Errorf("in-progress m: intent=%+v bounce=%q", in, bounce)
	}

	// ready card → done
	m.colIdx = 2
	in, ship, bounce = m.attemptAction(false)
	if bounce != "" || !ship || in.Action != launch.ActionDone {
		t.Errorf("ready m: intent=%+v ship=%v bounce=%q", in, ship, bounce)
	}

	// shipped card with m → bounce (illegal)
	m.colIdx = 3
	_, _, bounce = m.attemptAction(false)
	if bounce == "" {
		t.Errorf("shipped m should bounce")
	}

	// plan card with d (ship) → bounce
	m.colIdx = 0
	_, _, bounce = m.attemptAction(true)
	if !strings.Contains(bounce, "only ready cards can ship") {
		t.Errorf("plan d bounce = %q", bounce)
	}
}

func TestAttemptActionMergedShip(t *testing.T) {
	m := newModel(t)
	m.selected["ready"] = true
	m.selected["legacy-ready"] = true
	in, ship, bounce := m.attemptAction(false)
	if bounce != "" || !ship {
		t.Fatalf("merged ship bounce=%q ship=%v", bounce, ship)
	}
	if in.Command != "/gogo:done legacy-ready+ready" {
		t.Errorf("merged command = %q (sorted a+b)", in.Command)
	}
}

func TestDrillInAndArtifacts(t *testing.T) {
	m := newModel(t)
	// focus the in-progress feature (has the richest file set).
	m.colIdx = 1
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeDrill {
		t.Fatalf("enter did not drill in, mode=%d", m.mode)
	}
	if m.drill.Slug != "inprogress" {
		t.Fatalf("drilled into %q", m.drill.Slug)
	}
	labels := map[string]bool{}
	for _, a := range m.artifacts {
		labels[a.Label] = true
	}
	for _, want := range []string{"plan.md", "review/issues.json", "events (timeline)", "charts/flow.mmd"} {
		if !labels[want] {
			t.Errorf("drill missing artifact %q", want)
		}
	}
	// esc back to board
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeBoard {
		t.Errorf("esc did not return to board")
	}
}

func TestOpenEventsArtifactTimeline(t *testing.T) {
	m := newModel(t)
	m.colIdx = 1
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // drill
	// navigate to the events artifact and open it.
	for i, a := range m.artifacts {
		if a.Kind == contract.KindEvents {
			m.artIdx = i
		}
	}
	// Opening is async now (TEST-003): the render runs in a tea.Cmd, so pump the
	// command graph back through Update to deliver the viewerContentMsg.
	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeViewer {
		t.Fatalf("did not open viewer, mode=%d", m.mode)
	}
	if m.viewerLoading {
		t.Fatalf("viewer still loading after the render pump settled")
	}
	out := m.viewport.View()
	if !strings.Contains(out, "phase-started") && !strings.Contains(out, "round-opened") {
		t.Errorf("timeline not rendered:\n%s", out)
	}
}

// TEST-003: opening a view is async — the model enters modeViewer + a loading
// state IMMEDIATELY (no blocking render inside Update), the spinner is running,
// and the real content arrives via a later viewerContentMsg. Reopening the same
// file at the same width is a cache hit (no loading state, no command).
func TestViewerAsyncLoadingThenContent(t *testing.T) {
	m := newModel(t)
	m.colIdx = 1
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // drill (plan.md is artifact 0)

	// Enter opens the viewer synchronously into a LOADING state and returns a
	// command (the async render). Update itself must not block on the render.
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.mode != modeViewer {
		t.Fatalf("enter did not open viewer, mode=%d", m.mode)
	}
	if !m.viewerLoading {
		t.Fatalf("viewer did not enter the loading state before the async render")
	}
	if cmd == nil {
		t.Fatalf("open returned no async render command")
	}
	if got := m.View(); !strings.Contains(got, "rendering") {
		t.Errorf("loading view missing the rendering hint:\n%s", got)
	}

	// Pump the render command graph → content arrives, loading clears.
	m = drive(t, m, cmd)
	if m.viewerLoading {
		t.Fatalf("still loading after the render settled")
	}
	if key := cacheKey(m.curArtifact, m.width); m.renderCache[key] == "" {
		t.Fatalf("render was not cached for %q", key)
	}

	// Back out, reopen the same artifact at the same width → cache hit: no
	// loading state and no command returned (instant reopen).
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc}) // back to drill
	_, cmd2 := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd2 != nil {
		t.Errorf("reopen at same width should be a cache hit (nil cmd), got a command")
	}
	nm3, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if nm3.(Model).viewerLoading {
		t.Errorf("reopen at same width should not re-enter the loading state")
	}
}

// TEST-004: `v` opens the column's DEFAULT file. ready → the report; plan →
// plan.md; changelog → the entry's report.md; in progress → the file list; a
// column whose default file is missing → the file list.
func TestQuickViewDefaultsPerColumn(t *testing.T) {
	cases := []struct {
		name       string
		colIdx     int
		focusSlug  string // when set, focus this card before v
		wantMode   mode
		wantSuffix string // viewerTitle when a file opens
	}{
		{"ready opens the report", 2, "", modeViewer, "report.md"},
		{"plan opens plan.md", 0, "", modeViewer, "plan.md"},
		{"changelog opens the entry report", 3, "shipped-by-folder", modeViewer, "report.md"},
		{"in progress stays on the file list", 1, "", modeDrill, ""},
		{"shipped-by-status (no report) → file list", 3, "shipped-status", modeDrill, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(t)
			m.colIdx = tc.colIdx
			if tc.focusSlug != "" {
				for i, f := range m.cols[tc.colIdx] {
					if f.Slug == tc.focusSlug {
						m.cardIdx[tc.colIdx] = i
					}
				}
			}
			m = keyPress(t, m, runes("v"))
			if m.mode != tc.wantMode {
				t.Fatalf("col %d: mode=%d want %d (title=%q)", tc.colIdx, m.mode, tc.wantMode, m.viewerTitle)
			}
			if tc.wantSuffix != "" && !strings.HasSuffix(m.viewerTitle, tc.wantSuffix) {
				t.Errorf("col %d: opened %q, want a *%s", tc.colIdx, m.viewerTitle, tc.wantSuffix)
			}
		})
	}
}

// TEST-006 (redesigned FR-1/FR-3): a card whose gogo-* tmux session is alive
// shows a bare green ● dot beside its name, the header attention summary counts
// it as "● S session", and the board status line surfaces the attach hint.
func TestSessionIndicatorOnCard(t *testing.T) {
	m := newModel(t)
	m.colIdx = 1
	f := m.focusedCard()
	if f == nil {
		t.Fatal("no in-progress card to test")
	}
	m.sessions = []string{"gogo-go-" + f.Slug}
	out := m.View()
	// FR-1: the header attention summary counts the live session.
	if !strings.Contains(out, "● 1 session") {
		t.Errorf("header attention summary missing the ● session count:\n%s", out)
	}
	// The card carries a bare live ● dot next to its name (the "● session" text
	// moved to the header — FR-3).
	if !strings.Contains(out, f.Slug+" ●") {
		t.Errorf("card missing the bare live ● dot:\n%s", out)
	}
	if !strings.Contains(out, "attach") {
		t.Errorf("board status line did not surface the attach hint")
	}

	// No live session → no ● anywhere (card dot, header count, footer lead).
	m.sessions = nil
	if strings.Contains(m.View(), "●") {
		t.Errorf("● shown with no live session:\n%s", m.View())
	}
}

func TestRenderIssuesTable(t *testing.T) {
	list, _ := contract.ReadIssues(filepath.Join(fixtureRoot, ".gogo", "work", "feature-inprogress", "review", "issues.json"))
	out := renderIssues(list)
	for _, want := range []string{"REV-001", "REV-002", "open", "fixed", "track review"} {
		if !strings.Contains(out, want) {
			t.Errorf("issues table missing %q", want)
		}
	}
}

func TestRenderTimeline(t *testing.T) {
	evs := contract.ReadEvents(filepath.Join(fixtureRoot, ".gogo", "work", "feature-inprogress", "events.jsonl"))
	out := renderTimeline(evs)
	if !strings.Contains(out, "phase-started") || !strings.Contains(out, "r1") || !strings.Contains(out, "2 blockers") {
		t.Errorf("timeline wrong:\n%s", out)
	}
	if renderTimeline(nil) != "no events recorded" {
		t.Errorf("empty timeline wrong")
	}
}

func TestBadge(t *testing.T) {
	f := &contract.Feature{Slug: "x", Status: "waiting-for-user"}
	if got := badge(f); got != "waiting-for-user" {
		t.Errorf("waiting badge = %q", got)
	}
	// A live session is NOT a status: the badge stays the card's true phase
	// (running-vs-status decoupling — liveness rides the separate ● dot).
	f2 := &contract.Feature{Slug: "runme", Phase: "implement"}
	if got := badge(f2); got != "implement" {
		t.Errorf("badge = %q, want implement (session is a separate signal)", got)
	}
	f3 := &contract.Feature{Slug: "y", LatestEvent: &contract.Event{Phase: "review", Round: 2, HasRound: true}}
	if got := badge(f3); got != "review r2" {
		t.Errorf("event badge = %q", got)
	}
	f4 := &contract.Feature{Slug: "z", Phase: "plan", Status: "plan-accepted"}
	if got := badge(f4); got != "plan-accepted" {
		t.Errorf("state badge = %q", got)
	}

	// REV-008 live repro: events say implement r3 but state.md is in review — the
	// badge must follow state.md (its own column), NOT the stale event. The round
	// comes from state.md's iterations (review=2), so the badge is "review r2".
	f5 := &contract.Feature{
		Slug:       "stale",
		Phase:      "review",
		Status:     "reviewing",
		Iterations: "plan=2 · implement=4 · review=2 · test=0",
		LatestEvent: &contract.Event{
			Phase: "implement", Round: 3, HasRound: true,
		},
	}
	if got := badge(f5); got != "review r2" {
		t.Errorf("stale-event badge = %q, want review r2 (state.md wins)", got)
	}
	if got := badge(f5); strings.Contains(got, "implement") {
		t.Errorf("badge must not show the stale implement phase: %q", got)
	}

	// Agreement: latest event's phase matches state.md's phase — enrich with the
	// event's round.
	f6 := &contract.Feature{
		Slug: "agree", Phase: "review", Status: "reviewing",
		LatestEvent: &contract.Event{Phase: "review", Round: 2, HasRound: true},
	}
	if got := badge(f6); got != "review r2" {
		t.Errorf("agreeing-event badge = %q, want review r2", got)
	}

	// knowledge↔report mapping: state.md's "knowledge" phase agrees with an
	// events "report" line — they are the same phase, so the round applies.
	f7 := &contract.Feature{
		Slug: "kn", Phase: "knowledge", Status: "reporting",
		LatestEvent: &contract.Event{Phase: "report", Round: 1, HasRound: true},
	}
	if got := badge(f7); got != "knowledge r1" {
		t.Errorf("knowledge/report badge = %q, want knowledge r1", got)
	}
}

func TestSuggestRelease(t *testing.T) {
	if got := suggestRelease([]string{"viewer-bundles", "viewer-menu"}); got != "viewer" {
		t.Errorf("common-theme suggest = %q, want viewer", got)
	}
	if got := suggestRelease([]string{"alpha", "beta"}); got != "alpha-plus-1" {
		t.Errorf("no-common suggest = %q", got)
	}
	if got := suggestRelease([]string{"solo"}); got != "solo" {
		t.Errorf("single suggest = %q", got)
	}
}

func TestQuitKey(t *testing.T) {
	m := newModel(t)
	_, cmd := m.Update(runes("q"))
	if cmd == nil {
		t.Errorf("q should return a quit command")
	}
}

func newTestWatchSet(t *testing.T) *watchSet {
	t.Helper()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	ws := &watchSet{
		w:       w,
		ch:      make(chan struct{}, 1),
		done:    make(chan struct{}),
		watched: map[string]bool{},
	}
	t.Cleanup(func() { _ = ws.close() })
	return ws
}

// REV-010: a feature dir created AFTER startup must be armed on the next
// reconcile, so its later writes keep the board live. Deterministic — asserts
// the path-set logic (the precondition for a second reload), no fsnotify timing.
func TestWatchReconcileArmsNewFeature(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, ".gogo", "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ws := newTestWatchSet(t)

	// Startup snapshot: no features yet — only .gogo/work exists on disk.
	repo0, _ := contract.LoadRepo(root)
	ws.reconcile(watchPaths(root, repo0))
	if !ws.watched[workDir] {
		t.Fatalf(".gogo/work not armed at startup: %v", ws.watched)
	}
	newFeatureDir := filepath.Join(workDir, "feature-born-in-session")
	if ws.watched[newFeatureDir] {
		t.Fatalf("feature dir armed before it exists")
	}

	// A plan→implement launch creates the feature folder mid-session.
	if err := os.MkdirAll(newFeatureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newFeatureDir, "state.md"), []byte("- **phase:** implement\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-arm on reload: the new feature dir is now watched, and it is reported as
	// newly added (proving the re-arm actually fired for it).
	repo1, _ := contract.LoadRepo(root)
	added := ws.reconcile(watchPaths(root, repo1))
	if !ws.watched[newFeatureDir] {
		t.Errorf("new feature dir not armed after reconcile: %v", ws.watched)
	}
	if !containsPath(added, newFeatureDir) {
		t.Errorf("reconcile did not report the new dir as added: %v", added)
	}

	// Idempotent: a reconcile with no changes arms nothing new.
	if again := ws.reconcile(watchPaths(root, repo1)); len(again) != 0 {
		t.Errorf("re-reconcile added %v, want none (idempotent)", again)
	}

	// Graceful removal: a vanished dir is dropped from the watched set.
	if err := os.RemoveAll(newFeatureDir); err != nil {
		t.Fatal(err)
	}
	repo2, _ := contract.LoadRepo(root)
	ws.reconcile(watchPaths(root, repo2))
	if ws.watched[newFeatureDir] {
		t.Errorf("removed feature dir still watched: %v", ws.watched)
	}
}

// REV-011: close stops the goroutine and guards the buffered send — a late
// debounce callback (fire) after close must be a no-op, never a send-on-closed
// panic, and close must be idempotent.
func TestWatchSetCloseGuardsSend(t *testing.T) {
	ws := newTestWatchSet(t)
	ws.start()

	if err := ws.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Second close is a safe no-op (sync.Once).
	if err := ws.close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
	// A stray timer callback after shutdown must not panic.
	ws.fire()

	// A nil watchSet closes cleanly (the tests-never-start-a-watcher path).
	var nilWS *watchSet
	if err := nilWS.close(); err != nil {
		t.Errorf("nil close: %v", err)
	}
}

func containsPath(ps []string, want string) bool {
	for _, p := range ps {
		if p == want {
			return true
		}
	}
	return false
}

// --- FR5 launch-form lifecycle (TEST-001 message routing / TEST-002 stale selection) ---

// recordingLauncher is the injected fake launcher — it records every intent it
// is asked to spawn instead of shelling out to tmux/claude.
type recordingLauncher struct{ calls []launch.Intent }

func (r *recordingLauncher) launch(_ string, in launch.Intent) (launch.Result, error) {
	r.calls = append(r.calls, in)
	return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
}

// launchable wires a model with the fake launcher and claude "present", so the
// m/d launch forms open and can be driven to completion without a tty.
func launchable(t *testing.T) (Model, *recordingLauncher) {
	t.Helper()
	m := newModel(t)
	rl := &recordingLauncher{}
	m.hasClaude = true
	m.launcher = rl.launch
	return m, rl
}

// drive pumps a command graph back through Update exactly as the Bubble Tea
// runtime would — running each command, expanding tea.Batch, and re-feeding the
// resulting message — until the queue drains. This IS the async round-trip
// TEST-001 was about: huh advances between fields and submits via its own
// nextFieldMsg/nextGroupMsg, which must reach the form via Update.
func drive(t *testing.T, m Model, cmds ...tea.Cmd) Model {
	t.Helper()
	queue := append([]tea.Cmd(nil), cmds...)
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 2000 {
			t.Fatalf("form command pump did not settle in 2000 steps (mode=%d)", m.mode)
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch tm := c().(type) {
		case nil:
			continue
		case tea.BatchMsg:
			queue = append(queue, tm...)
		case launchDoneMsg:
			// The launch command already invoked the (fake) launcher; re-feeding
			// it would shell out to tmux via ListSessions.
			continue
		default:
			nm, next := m.Update(tm)
			m = nm.(Model)
			if next != nil {
				queue = append(queue, next)
			}
		}
	}
	return m
}

// keyPress sends one key, then pumps every async command it spawns.
func keyPress(t *testing.T, m Model, key tea.Msg) Model {
	t.Helper()
	nm, cmd := m.Update(key)
	return drive(t, nm.(Model), cmd)
}

// TEST-001 regression: a single-card ready ship (d) must be completable with y.
// Drives the FULL huh lifecycle through Update — pressing y emits an async
// NextField→nextGroup→StateCompleted chain that the old KeyMsg-only routing
// dropped, leaving the form unsubmittable. Asserts the launcher fired once.
func TestFormSingleConfirmLaunches(t *testing.T) {
	m, rl := launchable(t)
	m.colIdx = 2 // ready column, focus "ready"

	nm, _ := m.Update(runes("d"))
	m = nm.(Model)
	if m.mode != modeForm {
		t.Fatalf("d did not open a confirm form, mode=%d", m.mode)
	}

	// Enter on the default-affirmative confirm submits → huh emits an async
	// NextField→nextGroup→StateCompleted chain the old KeyMsg-only routing
	// dropped. The pump proves those messages now reach the form.
	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if len(rl.calls) != 1 {
		t.Fatalf("launcher called %d times, want exactly 1", len(rl.calls))
	}
	if got := rl.calls[0]; got.Action != launch.ActionDone || got.Command != "/gogo:done ready" {
		t.Errorf("launched %+v, want done /gogo:done ready", got)
	}
	if m.mode != modeBoard {
		t.Errorf("did not return to board after launch, mode=%d", m.mode)
	}
}

// TEST-001 regression for the MERGED release-name form: two fields (input then
// confirm). Enter advances past the input (proving inter-field navigation works
// live), then y submits. The launcher must fire once with the injection-safe
// single-argv merged command.
func TestFormMergedReleaseLaunches(t *testing.T) {
	m, rl := launchable(t)
	m.selected["ready"] = true
	m.selected["legacy-ready"] = true

	nm, _ := m.Update(runes("d"))
	m = nm.(Model)
	if m.mode != modeForm {
		t.Fatalf("merged d did not open a form, mode=%d", m.mode)
	}

	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // advance input → confirm
	if m.mode != modeForm {
		t.Fatalf("form completed while only advancing the release-name input (mode=%d)", m.mode)
	}
	if len(rl.calls) != 0 {
		t.Fatalf("advancing a field launched prematurely: %+v", rl.calls)
	}

	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // confirm (Launch) → submit

	if len(rl.calls) != 1 {
		t.Fatalf("launcher called %d times, want 1", len(rl.calls))
	}
	if got := rl.calls[0].Command; got != "/gogo:done legacy-ready+ready" {
		t.Errorf("merged launch command = %q, want /gogo:done legacy-ready+ready", got)
	}
	if len(m.selectedSlugs()) != 0 {
		t.Errorf("selection not cleared after launch: %v", m.selectedSlugs())
	}
}

// TEST-002 regression: aborting a form (ctrl+c) must NOT launch, must clear the
// ready selection, and must not let a later, unrelated action resurrect the
// abandoned target list.
func TestFormAbortClearsSelectionNoRelaunch(t *testing.T) {
	m, rl := launchable(t)
	m.selected["ready"] = true
	m.selected["legacy-ready"] = true

	nm, _ := m.Update(runes("d"))
	m = nm.(Model)
	if m.mode != modeForm {
		t.Fatalf("d did not open the merged form")
	}

	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyCtrlC}) // abort

	if len(rl.calls) != 0 {
		t.Fatalf("abort still launched: %+v", rl.calls)
	}
	if m.mode != modeBoard {
		t.Errorf("abort did not return to board, mode=%d", m.mode)
	}
	if m.status != "cancelled" {
		t.Errorf("abort status = %q, want cancelled", m.status)
	}
	if len(m.selectedSlugs()) != 0 {
		t.Fatalf("abort left a stale selection %v — a later m/d would re-ship it", m.selectedSlugs())
	}

	// The core TEST-002 guarantee: a fresh action on a DIFFERENT card rebuilds
	// its target from the current focus, never the abandoned selection.
	m.colIdx = 0 // a plan card → m should propose a go, not the stale done
	in, ship, bounce := m.attemptAction(false)
	if bounce != "" || ship || in.Action != launch.ActionGo {
		t.Errorf("after abort, m on a plan card resurrected the stale ship: intent=%+v ship=%v", in, ship)
	}
}

// TEST-002: Esc must also abort (huh binds only ctrl+c to Quit) — cleanly, with
// no launch and a cleared selection.
func TestFormEscAborts(t *testing.T) {
	m, rl := launchable(t)
	m.selected["ready"] = true
	m.selected["legacy-ready"] = true

	nm, _ := m.Update(runes("d"))
	m = nm.(Model)
	if m.mode != modeForm {
		t.Fatalf("d did not open a form")
	}

	m = keyPress(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if len(rl.calls) != 0 {
		t.Fatalf("esc still launched: %+v", rl.calls)
	}
	if m.mode != modeBoard || m.status != "cancelled" {
		t.Errorf("esc did not cancel to board: mode=%d status=%q", m.mode, m.status)
	}
	if len(m.selectedSlugs()) != 0 {
		t.Errorf("esc-abort left a stale selection: %v", m.selectedSlugs())
	}
}

// Toggling the confirm to Cancel (n / Reject) and completing is NOT a launch —
// it returns to the board cancelled and clears the selection.
func TestFormRejectCompletesWithoutLaunch(t *testing.T) {
	m, rl := launchable(t)
	m.colIdx = 2

	nm, _ := m.Update(runes("d"))
	m = nm.(Model)

	m = keyPress(t, m, runes("n")) // reject → sets confirm false, completes

	if len(rl.calls) != 0 {
		t.Fatalf("reject still launched: %+v", rl.calls)
	}
	if m.mode != modeBoard || m.status != "cancelled" {
		t.Errorf("reject-completion state wrong: mode=%d status=%q", m.mode, m.status)
	}
}
