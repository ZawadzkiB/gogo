package orchestrator_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
)

// fakeRunner stands in for spawning `claude -p`: it records every Invocation and,
// per its script, rewrites the feature's state.md to the status the real session
// would leave — so the manager's real read+classify path runs without claude.
type fakeRunner struct {
	root, slug  string
	writeStatus string // status to leave in state.md after the run ("" → leave as-is)
	res         launch.RunResult
	err         error
	calls       []orchestrator.Invocation
}

func (f *fakeRunner) Run(inv orchestrator.Invocation) (launch.RunResult, error) {
	f.calls = append(f.calls, inv)
	if f.writeStatus != "" {
		writeState(nil, f.root, f.slug, f.writeStatus)
	}
	res := f.res
	if res.SessionID == "" {
		res.SessionID = inv.SessionID
		if inv.Resume != "" {
			res.SessionID = inv.Resume
		}
	}
	return res, f.err
}

// writeState writes a minimal, grammar-valid state.md for a feature (t may be nil
// when called from the fake runner, where a failure just surfaces downstream).
func writeState(t *testing.T, root, slug, status string) {
	dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
	_ = os.MkdirAll(dir, 0o755)
	body := "# State — feature `" + slug + "`\n\n" +
		"- **feature:** unit-test scratch\n" +
		"- **phase:** implement\n" +
		"- **status:** " + status + "\n" +
		"- **created:** 2026-07-11\n" +
		"- **open-decision:** none\n"
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(body), 0o644); err != nil && t != nil {
		t.Fatalf("write state.md: %v", err)
	}
}

func newSession(root, slug string, runner *fakeRunner) (*orchestrator.Session, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return &orchestrator.Session{
		Root:   root,
		Slug:   slug,
		Kind:   "go",
		Reg:    orchestrator.LoadRegistry(root, slug),
		Runner: runner,
		Out:    buf,
		Killer: func(string) error { return nil },
		Lister: func() []string { return nil }, // hermetic: no tmux shell-out by default
	}, buf
}

// --- 1. launch-or-resume resolver (pure) -------------------------------------

func TestResolveInvocation(t *testing.T) {
	root := t.TempDir()
	// First run: empty registry → a fresh --session-id, no --resume.
	empty := orchestrator.LoadRegistry(root, "feat")
	inv := orchestrator.ResolveInvocation(empty, "go", "feat")
	if inv.SessionID == "" || inv.Resume != "" {
		t.Errorf("first run must be a fresh --session-id, got %+v", inv)
	}
	if inv.Command != "/gogo:go feat" {
		t.Errorf("go command = %q, want /gogo:go feat", inv.Command)
	}
	// Re-run: a tracked uuid → --resume it, no --session-id.
	reg := &orchestrator.Registry{Slug: "feat", Persistent: map[string]*orchestrator.PersistentSession{
		"go": {Kind: "go", UUID: "warm-123"},
	}}
	inv = orchestrator.ResolveInvocation(reg, "go", "feat")
	if inv.Resume != "warm-123" || inv.SessionID != "" {
		t.Errorf("re-run must --resume the warm uuid, got %+v", inv)
	}
	// plan leg uses its own command + its own tracked session.
	planInv := orchestrator.ResolveInvocation(reg, "plan", "feat")
	if planInv.Command != "/gogo:plan feat" || planInv.SessionID == "" {
		t.Errorf("plan leg = %+v, want a fresh /gogo:plan invocation", planInv)
	}
}

// --- 2. lock refusal / reclaim / takeover ------------------------------------

func seedLock(t *testing.T, root, slug string, owner orchestrator.Owner) {
	t.Helper()
	path := orchestrator.LockPath(root, slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(owner)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLockRefusesLiveOwner(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	seedLock(t, root, "feat", orchestrator.Owner{PID: 4242, Tmux: "gogo-go-feat", Kind: "go"})

	fake := &fakeRunner{root: root, slug: "feat", writeStatus: "awaiting-uat"}
	sess, buf := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return true } // owner is LIVE

	out, err := sess.LaunchOrResume()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultRefused {
		t.Fatalf("a live owner must refuse, got %+v", out)
	}
	if len(fake.calls) != 0 {
		t.Errorf("refusal must launch NOTHING, got %d calls", len(fake.calls))
	}
	if !strings.Contains(buf.String(), "already owned by a live session") {
		t.Errorf("refusal must name the live owner; output: %q", buf.String())
	}
}

func TestLockReclaimsStaleOwner(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	seedLock(t, root, "feat", orchestrator.Owner{PID: 999999, Kind: "go"}) // headless, no tmux

	fake := &fakeRunner{root: root, slug: "feat", writeStatus: "awaiting-uat"}
	sess, _ := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return false } // owner is DEAD → reclaim

	out, err := sess.LaunchOrResume()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultAwaitingUAT {
		t.Fatalf("a stale lock must be reclaimed + launched, got %+v", out)
	}
	if len(fake.calls) != 1 {
		t.Errorf("stale reclaim must launch exactly once, got %d", len(fake.calls))
	}
}

func TestLockTakeoverSeizesAndReaps(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	seedLock(t, root, "feat", orchestrator.Owner{PID: 4242, Tmux: "gogo-go-feat", Kind: "go"})

	fake := &fakeRunner{root: root, slug: "feat", writeStatus: "awaiting-uat"}
	sess, _ := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return true } // owner is LIVE
	sess.Takeover = true
	var killed []string
	sess.Killer = func(n string) error { killed = append(killed, n); return nil }

	out, err := sess.LaunchOrResume()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultAwaitingUAT || len(fake.calls) != 1 {
		t.Fatalf("--takeover must seize + launch once, got %+v / %d calls", out, len(fake.calls))
	}
	if len(killed) != 1 || killed[0] != "gogo-go-feat" {
		t.Errorf("--takeover must reap the prior owner's tmux, killed = %v", killed)
	}
}

func TestLockRefusesUntrackedBoardSession(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	// No lockfile at all — but a live gogo-* session holds the slug (a board-launched
	// racer that never wrote our lock). The lock must still refuse it (D1=C / FR6).
	fake := &fakeRunner{root: root, slug: "feat", writeStatus: "awaiting-uat"}
	sess, buf := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return true } // a live session exists for the slug
	sess.Lister = func() []string { return []string{"gogo-go-feat"} }

	out, err := sess.LaunchOrResume()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultRefused {
		t.Fatalf("a live untracked board session must refuse, got %+v", out)
	}
	if len(fake.calls) != 0 {
		t.Errorf("refusal must launch NOTHING, got %d calls", len(fake.calls))
	}
	if !strings.Contains(buf.String(), "untracked live session") {
		t.Errorf("refusal must name the untracked board session; output: %q", buf.String())
	}
	// The just-created lockfile is removed — we do not own the work.
	if _, err := os.Stat(orchestrator.LockPath(root, "feat")); !os.IsNotExist(err) {
		t.Errorf("a refused untracked acquire must leave no lockfile (err=%v)", err)
	}
}

func TestTakeoverReapsBoardSessionBySlug(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	fake := &fakeRunner{root: root, slug: "feat", writeStatus: "awaiting-uat"}
	sess, _ := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return true }
	sess.Takeover = true
	// A collision-suffixed / board session that the lockfile's base name would miss —
	// reap must go BY SLUG (REV-002).
	sess.Lister = func() []string { return []string{"gogo-go-feat-2", "gogo-done-other"} }
	var killed []string
	sess.Killer = func(n string) error { killed = append(killed, n); return nil }

	out, err := sess.LaunchOrResume()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultAwaitingUAT || len(fake.calls) != 1 {
		t.Fatalf("--takeover must seize + launch once, got %+v / %d calls", out, len(fake.calls))
	}
	if len(killed) != 1 || killed[0] != "gogo-go-feat-2" {
		t.Errorf("--takeover must reap the matching session by slug (not the unrelated one), killed = %v", killed)
	}
}

// --- 3. registry round-trip --------------------------------------------------

func TestRegistryRoundTrip(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	fake := &fakeRunner{root: root, slug: "feat", writeStatus: "waiting-for-user"}
	sess, _ := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return false }

	if _, err := sess.LaunchOrResume(); err != nil {
		t.Fatal(err)
	}
	firstUUID := fake.calls[0].SessionID
	if firstUUID == "" {
		t.Fatal("first run must assign a --session-id")
	}

	// Reload from disk: the persistent session's uuid survives, and a re-run resumes
	// the SAME uuid (warm), not a fresh one.
	reg := orchestrator.LoadRegistry(root, "feat")
	if reg.Get("go") == nil || reg.Get("go").UUID != firstUUID {
		t.Fatalf("registry did not persist the go session uuid %q: %+v", firstUUID, reg.Get("go"))
	}
	inv := orchestrator.ResolveInvocation(reg, "go", "feat")
	if inv.Resume != firstUUID {
		t.Errorf("re-run must --resume %q, got %+v", firstUUID, inv)
	}

	// A garbled registry degrades to fresh, never a crash.
	path := orchestrator.RegistryPath(root, "bad")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("{not json"), 0o644)
	if orchestrator.LoadRegistry(root, "bad").Get("go") != nil {
		t.Error("garbled registry should degrade to fresh")
	}
}

// --- 4. reap kills the tracked session + tmux --------------------------------

func TestReapKillsTrackedTmux(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "shipped")
	reg := &orchestrator.Registry{Slug: "feat", Persistent: map[string]*orchestrator.PersistentSession{
		"go": {Kind: "go", UUID: "u-1", Tmux: "gogo-go-feat", Status: orchestrator.SessRunning},
	}}
	if err := reg.Save(root); err != nil {
		t.Fatal(err)
	}
	seedLock(t, root, "feat", orchestrator.Owner{PID: 4242, Tmux: "gogo-go-feat"})

	var killed []string
	sess := &orchestrator.Session{
		Root: root, Slug: "feat", Kind: "go",
		Reg:    orchestrator.LoadRegistry(root, "feat"),
		Killer: func(n string) error { killed = append(killed, n); return nil },
		Out:    &bytes.Buffer{},
	}
	sess.Reap()

	if len(killed) != 1 || killed[0] != "gogo-go-feat" {
		t.Errorf("reap must KillSession the tracked tmux, killed = %v", killed)
	}
	reg2 := orchestrator.LoadRegistry(root, "feat")
	if reg2.Get("go") == nil || reg2.Get("go").Status != orchestrator.SessReaped {
		t.Errorf("reap must mark the registry reaped, got %+v", reg2.Get("go"))
	}
	if _, err := os.Stat(orchestrator.LockPath(root, "feat")); !os.IsNotExist(err) {
		t.Errorf("reap must release the lock, lockfile still present (err=%v)", err)
	}
}

// --- 5. orphan-sweep (with exact SessionMatchesSlug attribution, TEST-005) ----

func TestSweepReapsOrphansAndTerminal(t *testing.T) {
	root := t.TempDir()
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "a", Status: "testing"},
		{Slug: "b", Status: "shipped"},
		{Slug: "oauth", Status: "testing"},
		{Slug: "auth", Status: "testing"},
		{Slug: "waiting-card", Status: "implementing"},
	}}
	sessions := []string{
		"gogo-go-a",               // a is live/in-progress → spare
		"gogo-done-b",             // b is shipped → reap (kill-at-ship)
		"gogo-go-orphan",          // no feature "orphan" → reap
		"gogo-go-oauth",           // matches oauth (not auth) → spare
		"gogo-done-awaiting-card", // no feature "awaiting-card"; must NOT match "waiting-card" → reap
	}
	var killed []string
	sw := &orchestrator.Sweeper{
		Root: root, Repo: repo,
		List: func() []string { return sessions },
		Kill: func(n string) error { killed = append(killed, n); return nil },
		Out:  &bytes.Buffer{},
	}
	got := sw.Sweep()

	want := map[string]bool{"gogo-done-b": true, "gogo-go-orphan": true, "gogo-done-awaiting-card": true}
	if len(got) != len(want) {
		t.Fatalf("sweep killed %v, want exactly %v", got, keys(want))
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("sweep killed %q which should have been spared", s)
		}
	}
	// gogo-go-a and gogo-go-oauth (live, non-terminal) must be spared.
	for _, spared := range []string{"gogo-go-a", "gogo-go-oauth"} {
		if contains(killed, spared) {
			t.Errorf("sweep reaped %q but its feature is live/non-terminal (attribution bug)", spared)
		}
	}
}

func TestSweepDryRunKillsNothing(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{{Slug: "b", Status: "shipped"}}}
	var killed []string
	sw := &orchestrator.Sweeper{
		Root: t.TempDir(), Repo: repo, DryRun: true,
		List: func() []string { return []string{"gogo-done-b", "gogo-go-orphan"} },
		Kill: func(n string) error { killed = append(killed, n); return nil },
		Out:  &bytes.Buffer{},
	}
	got := sw.Sweep()
	if len(got) != 2 {
		t.Errorf("dry-run must report 2 would-reap, got %v", got)
	}
	if len(killed) != 0 {
		t.Errorf("dry-run must kill NOTHING, killed = %v", killed)
	}
}

// TestSweepSparesSelf proves the FR3 self-guard: `gogo sweep` never kills the
// session it is itself running in, even when that session's owning feature is
// terminal. This is what makes the /gogo:done ship-reap (which runs plain
// `gogo sweep` after flipping members to `shipped`) safe when /gogo:done is
// hosted in a board-launched gogo-done-<slug> session — it reaps the driving
// gogo-go-<slug> but spares its own host, so the ship is never truncated.
func TestSweepSparesSelf(t *testing.T) {
	root := t.TempDir()
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "x", Status: "shipped"}, // terminal → its sessions are reap candidates
	}}
	sessions := []string{
		"gogo-done-x", // == Self: the session hosting /gogo:done → SPARE (self-guard)
		"gogo-go-x",   // x's driving session, x is shipped → reap
	}
	var killed []string
	sw := &orchestrator.Sweeper{
		Root: root, Repo: repo,
		List: func() []string { return sessions },
		Kill: func(n string) error { killed = append(killed, n); return nil },
		Self: "gogo-done-x",
		Out:  &bytes.Buffer{},
	}
	got := sw.Sweep()

	if len(got) != 1 || got[0] != "gogo-go-x" {
		t.Fatalf("sweep killed %v, want exactly [gogo-go-x] (driving session reaped)", got)
	}
	if contains(killed, "gogo-done-x") {
		t.Errorf("sweep killed gogo-done-x — its own hosting session (self-guard breach)")
	}
}

// TestSweepTargetedOnlyNamedSlug proves the D4=B targeted mode (REV-002 fix): a
// slug-scoped sweep — what the /gogo:done ship-reap runs (`gogo sweep <slug>`) —
// reaps ONLY the named slug's sessions. A DIFFERENT terminal feature's session
// (e.g. another card's concurrent in-flight ship) and an orphan are BOTH spared,
// so a ship can never truncate another feature's ship. The self-guard still
// applies inside the targeted scan.
func TestSweepTargetedOnlyNamedSlug(t *testing.T) {
	root := t.TempDir()
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "x", Status: "shipped"}, // the slug being shipped → its sessions are in scope
		{Slug: "z", Status: "shipped"}, // a DIFFERENT terminal feature (concurrent ship) → out of scope
	}}
	sessions := []string{
		"gogo-go-x",   // x's driving session, x named + terminal → reap
		"gogo-done-x", // == Self: x's ship host → SPARE (self-guard)
		"gogo-done-z", // z's concurrent ship host, terminal but NOT named → SPARE (targeted)
		"gogo-go-orphan",
	}
	var killed []string
	sw := &orchestrator.Sweeper{
		Root: root, Repo: repo,
		List: func() []string { return sessions },
		Kill: func(n string) error { killed = append(killed, n); return nil },
		Self: "gogo-done-x",
		Only: []string{"x"},
		Out:  &bytes.Buffer{},
	}
	got := sw.Sweep()

	if len(got) != 1 || got[0] != "gogo-go-x" {
		t.Fatalf("targeted sweep killed %v, want exactly [gogo-go-x]", got)
	}
	for _, spared := range []string{"gogo-done-z", "gogo-go-orphan", "gogo-done-x"} {
		if contains(killed, spared) {
			t.Errorf("targeted `gogo sweep x` reaped %q — a session outside slug x (REV-002 breach)", spared)
		}
	}
}

// --- 6. exit classification --------------------------------------------------

func TestExitClassifyAwaitingUAT(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	fake := &fakeRunner{root: root, slug: "feat", writeStatus: "awaiting-uat"}
	sess, buf := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return false }

	out, err := sess.LaunchOrResume()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultAwaitingUAT || out.Status != "awaiting-uat" {
		t.Fatalf("green leg must classify awaiting-uat, got %+v", out)
	}
	if !strings.Contains(buf.String(), "/gogo:done feat") {
		t.Errorf("awaiting-uat surface must point at /gogo:done; output: %q", buf.String())
	}
	if ps := orchestrator.LoadRegistry(root, "feat").Get("go"); ps == nil || ps.Status != orchestrator.SessAwaitingUAT {
		t.Errorf("registry status must be awaiting-uat, got %+v", ps)
	}
	// The lock is released once the -p child exits.
	if _, err := os.Stat(orchestrator.LockPath(root, "feat")); !os.IsNotExist(err) {
		t.Errorf("headless leg must release the lock at exit (err=%v)", err)
	}
}

func TestExitClassifyWaitingForUser(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	fake := &fakeRunner{root: root, slug: "feat", writeStatus: "waiting-for-user"}
	sess, buf := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return false }

	out, err := sess.LaunchOrResume()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultParked {
		t.Fatalf("a gate exit must classify parked, got %+v", out)
	}
	if !strings.Contains(buf.String(), "waiting-for-user") || !strings.Contains(buf.String(), "resume the warm session") {
		t.Errorf("parked surface must explain the gate + resume; output: %q", buf.String())
	}
}

func TestExitIsErrorHalts(t *testing.T) {
	root := t.TempDir()
	writeState(t, root, "feat", "plan-accepted")
	fake := &fakeRunner{root: root, slug: "feat", res: launch.RunResult{IsError: true, CostUSD: 0.2}}
	sess, _ := newSession(root, "feat", fake)
	sess.Live = func(orchestrator.Owner, string) bool { return false }

	if _, err := sess.LaunchOrResume(); err == nil {
		t.Fatal("an is_error envelope must halt with an error, never a false green")
	}
	// REV-006 parity: the errored run's cost is still booked before the halt.
	reg := orchestrator.LoadRegistry(root, "feat")
	if reg.CostUSD < 0.2 {
		t.Errorf("errored run's cost must be booked, got $%.2f", reg.CostUSD)
	}
	// The lock is still released even on a halt.
	if _, err := os.Stat(orchestrator.LockPath(root, "feat")); !os.IsNotExist(err) {
		t.Errorf("halt must still release the lock (err=%v)", err)
	}
}

// --- status gate predicates --------------------------------------------------

func TestStatusGates(t *testing.T) {
	runnable := []string{"plan-accepted", "implementing", "reviewing", "testing"}
	notRunnable := []string{"awaiting-plan-acceptance", "awaiting-uat", "waiting-for-user", "done", "shipped", ""}
	for _, s := range runnable {
		if !orchestrator.RunnableStatus(s) {
			t.Errorf("%q should be runnable", s)
		}
	}
	for _, s := range notRunnable {
		if orchestrator.RunnableStatus(s) {
			t.Errorf("%q should NOT be runnable", s)
		}
	}
	// plan is permitted for a new/non-terminal feature, refused once shipped.
	for _, s := range []string{"", "awaiting-plan-acceptance", "plan-accepted", "waiting-for-user"} {
		if !orchestrator.PlannableStatus(s) {
			t.Errorf("%q should be plannable", s)
		}
	}
	for _, s := range []string{"shipped", "aborted", "done"} {
		if orchestrator.PlannableStatus(s) {
			t.Errorf("%q should NOT be plannable", s)
		}
		if !orchestrator.TerminalStatus(s) {
			t.Errorf("%q should be terminal", s)
		}
	}
}

// --- small helpers -----------------------------------------------------------

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
