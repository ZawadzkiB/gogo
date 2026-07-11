package orchestrator_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
)

// scriptedPhase is what a fake phase run "produces" when the loop invokes it: the
// contract files it writes (open issue count / result status) and the cost it books.
type scriptedPhase struct {
	open     int
	severity string // finding severity for review/test; "" → "major"
	needsDec bool
	status   string // result.json status; "" → "ok"
	noOutput bool   // simulate a phase that wrote NO contract files
	isError  bool   // simulate a `claude -p` run finishing with is_error=true
	cost     float64
}

// fakeRunner stands in for spawning `claude -p`: it records every Invocation and,
// per the script, writes the contract files the real phase session would, so the
// loop's real read+route path is exercised without spawning claude.
type fakeRunner struct {
	t          *testing.T
	root, slug string
	script     []scriptedPhase
	calls      []orchestrator.Invocation
}

func (f *fakeRunner) Run(inv orchestrator.Invocation) (launch.RunResult, error) {
	i := len(f.calls)
	f.calls = append(f.calls, inv)
	if i >= len(f.script) {
		f.t.Fatalf("phase call #%d unexpected (script has %d entries): %+v", i, len(f.script), inv)
	}
	sp := f.script[i]
	status := sp.status
	if status == "" {
		status = "ok"
	}
	fdir := filepath.Join(f.root, ".gogo", "work", "feature-"+f.slug)
	if !sp.noOutput {
		switch inv.Kind {
		case "implement":
			writeResultFile(f.t, fdir, "implement", status, 0)
		case "review", "test":
			writeIssuesFile(f.t, fdir, inv.Kind, sp.open, sp.severity, sp.needsDec)
			writeResultFile(f.t, fdir, inv.Kind, status, sp.open)
		}
	}
	uuid := inv.SessionID
	if inv.Resume != "" {
		uuid = inv.Resume
	}
	return launch.RunResult{CostUSD: sp.cost, SessionID: uuid, IsError: sp.isError}, nil
}

func writeResultFile(t *testing.T, fdir, track, status string, open int) {
	t.Helper()
	dir := filepath.Join(fdir, track)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"slug":"s","phase":%q,"status":%q,"inputs":[],"outputs":[],"validated_in":true,"validated_out":true,"open_issues":%d,"summary":"x"}`, track, status, open)
	if err := os.WriteFile(filepath.Join(dir, "result.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeIssuesFile(t *testing.T, fdir, track string, open int, severity string, needsDec bool) {
	t.Helper()
	if severity == "" {
		severity = "major"
	}
	dir := filepath.Join(fdir, track)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var items []string
	for i := 0; i < open; i++ {
		ps := "apply the fix"
		if needsDec && i == 0 {
			ps = "NEEDS-USER-DECISION: choose an approach"
		}
		items = append(items, fmt.Sprintf(`{"id":"REV-%03d","title":"t","description":"d","proposed_solution":%q,"severity":%q,"priority":"P1","status":"open","origin":%q,"found_in_round":1}`, i+1, ps, severity, track))
	}
	body := fmt.Sprintf(`{"slug":"s","track":%q,"round":1,"issues":[%s]}`, track, strings.Join(items, ","))
	if err := os.WriteFile(filepath.Join(dir, "issues.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newOrch(t *testing.T, fake *fakeRunner, maxRounds int, ceiling float64) (*orchestrator.Orchestrator, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	o := &orchestrator.Orchestrator{
		Root:        fake.root,
		Slug:        fake.slug,
		Runner:      fake,
		Reg:         orchestrator.LoadRegistry(fake.root, fake.slug),
		Out:         buf,
		MaxRounds:   maxRounds,
		CostCeiling: ceiling,
	}
	return o, buf
}

// TestHappyPath: clean review + clean test → report → awaiting-uat. Asserts the
// dev session is fresh (--session-id, no --resume), review/test are fresh, and
// report is a one-shot with no session flags.
func TestHappyPath(t *testing.T) {
	fake := &fakeRunner{t: t, root: t.TempDir(), slug: "feat",
		script: []scriptedPhase{{}, {open: 0}, {open: 0}, {}}}
	o, _ := newOrch(t, fake, 3, 0)

	out, err := o.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultAwaitingUAT {
		t.Fatalf("outcome = %+v, want awaiting-uat", out)
	}
	kinds := callKinds(fake.calls)
	if got := strings.Join(kinds, ","); got != "implement,review,test,report" {
		t.Fatalf("phase order = %s", got)
	}
	if fake.calls[0].SessionID == "" || fake.calls[0].Resume != "" {
		t.Errorf("initial dev build must be a fresh --session-id, not a resume: %+v", fake.calls[0])
	}
	if fake.calls[1].SessionID == "" || fake.calls[1].Resume != "" {
		t.Errorf("review must be a fresh session: %+v", fake.calls[1])
	}
	report := fake.calls[3]
	if report.SessionID != "" || report.Resume != "" {
		t.Errorf("report must be a one-shot (no session flags): %+v", report)
	}
}

// TestWarmResumeOnFix: review round 1 has an open fixable finding → the dev is
// RESUMED (same uuid) to fix it, and round-2 review gets a NEW fresh uuid.
func TestWarmResumeOnFix(t *testing.T) {
	fake := &fakeRunner{t: t, root: t.TempDir(), slug: "feat",
		script: []scriptedPhase{{}, {open: 1}, {}, {open: 0}, {open: 0}, {}}}
	o, _ := newOrch(t, fake, 3, 0)

	out, err := o.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultAwaitingUAT {
		t.Fatalf("outcome = %+v, want awaiting-uat", out)
	}
	// After the warm fix, review runs AGAIN (fresh eyes verify the fix) before test.
	if got := strings.Join(callKinds(fake.calls), ","); got != "implement,review,implement,review,test,report" {
		t.Fatalf("phase order = %s", got)
	}
	dev := fake.calls[0].SessionID
	fix := fake.calls[2]
	if fix.Kind != "implement" || fix.Resume != dev || fix.SessionID != "" {
		t.Errorf("the fix must RESUME the warm dev (uuid %s), not start fresh: %+v", dev, fix)
	}
	rev1, rev2 := fake.calls[1], fake.calls[3]
	if rev1.SessionID == rev2.SessionID {
		t.Errorf("re-review must use a NEW fresh uuid (fresh eyes); both were %s", rev1.SessionID)
	}
	if rev2.Resume != "" {
		t.Errorf("review is never resumed: %+v", rev2)
	}
}

// TestGateOnNeedsUserDecision: a needs-user-decision finding pauses the loop and
// spawns no further phases.
func TestGateOnNeedsUserDecision(t *testing.T) {
	fake := &fakeRunner{t: t, root: t.TempDir(), slug: "feat",
		script: []scriptedPhase{{}, {open: 1, needsDec: true}}}
	o, buf := newOrch(t, fake, 3, 0)

	out, err := o.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultGated {
		t.Fatalf("outcome = %+v, want gated", out)
	}
	if len(fake.calls) != 2 {
		t.Errorf("must pause after review (no test/report): got %d calls", len(fake.calls))
	}
	if !strings.Contains(buf.String(), "paused") {
		t.Errorf("gate must notify the user; output: %q", buf.String())
	}
}

// TestRoundBoundGates: a finding that never resolves gates after MaxRounds fixes.
func TestRoundBoundGates(t *testing.T) {
	fake := &fakeRunner{t: t, root: t.TempDir(), slug: "feat",
		script: []scriptedPhase{{}, {open: 1}, {}, {open: 1}, {}, {open: 1}}}
	o, _ := newOrch(t, fake, 2, 0)

	out, err := o.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultGated || !strings.Contains(out.Gate, "fix-round budget") {
		t.Fatalf("outcome = %+v, want gated on the round bound", out)
	}
	// build + (review, fix)×2 + a final review = 6 calls, then the 3rd fix gates.
	if len(fake.calls) != 6 {
		t.Errorf("expected 6 calls before the bound gates, got %d", len(fake.calls))
	}
}

// TestCostCeilingGates: crossing the cost ceiling gates rather than looping on.
func TestCostCeilingGates(t *testing.T) {
	fake := &fakeRunner{t: t, root: t.TempDir(), slug: "feat",
		script: []scriptedPhase{{cost: 0.1}, {open: 1, cost: 0.1}, {cost: 0.1}}}
	o, _ := newOrch(t, fake, 5, 0.25) // high round cap so the ceiling bites first

	out, err := o.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultGated || !strings.Contains(out.Gate, "cost ceiling") {
		t.Fatalf("outcome = %+v, want gated on the cost ceiling", out)
	}
}

// TestPhaseErrorHalts: a `claude -p` run that finishes with is_error must halt the
// run (not march on a failed phase as if green) — REV-002.
func TestPhaseErrorHalts(t *testing.T) {
	fake := &fakeRunner{t: t, root: t.TempDir(), slug: "feat",
		script: []scriptedPhase{{}, {isError: true, cost: 0.2}}}
	o, _ := newOrch(t, fake, 3, 0)

	if _, err := o.Run(); err == nil {
		t.Fatal("a phase reporting is_error must halt the run with an error")
	}
	if len(fake.calls) != 2 {
		t.Errorf("must halt at the failed review, got %d calls", len(fake.calls))
	}
	// The failed run's cost is still booked, not discarded (REV-006).
	if o.Reg.CostUSD < 0.2 {
		t.Errorf("errored run's cost must be booked before the halt, got $%.2f", o.Reg.CostUSD)
	}
}

// TestNoOutputGates: a review that writes no result.json/issues.json must gate, not
// be treated as clean-and-advance — REV-002.
func TestNoOutputGates(t *testing.T) {
	fake := &fakeRunner{t: t, root: t.TempDir(), slug: "feat",
		script: []scriptedPhase{{}, {noOutput: true}}}
	o, buf := newOrch(t, fake, 3, 0)

	out, err := o.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultGated {
		t.Fatalf("a review with no output must gate, not advance: %+v", out)
	}
	if len(fake.calls) != 2 {
		t.Errorf("must not advance to test/report: got %d calls", len(fake.calls))
	}
	if !strings.Contains(buf.String(), "no result.json") {
		t.Errorf("gate message should explain the missing output: %q", buf.String())
	}
}

// TestPreflightCostGateNoSpend: a re-run whose loaded cost already exceeds the
// ceiling gates immediately, spawning NOTHING (no money sink) — REV-003.
func TestPreflightCostGateNoSpend(t *testing.T) {
	root := t.TempDir()
	pre := &orchestrator.Registry{Slug: "feat", DevUUID: "dev-x", CostUSD: 99.0}
	if err := pre.Save(root); err != nil {
		t.Fatal(err)
	}
	fake := &fakeRunner{t: t, root: root, slug: "feat", script: nil}
	o, _ := newOrch(t, fake, 3, 1.0) // ceiling 1.0, already spent 99

	out, err := o.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultGated || !strings.Contains(out.Gate, "cost ceiling") {
		t.Fatalf("must pre-flight gate on the loaded cost, got %+v", out)
	}
	if len(fake.calls) != 0 {
		t.Errorf("pre-flight cost gate must spawn NOTHING, got %d calls", len(fake.calls))
	}
}

// TestReviewBatchesMinor: a lone open MINOR on review is batched (gogo-review §④) —
// advance to test, never a re-implement round — REV-001.
func TestReviewBatchesMinor(t *testing.T) {
	fake := &fakeRunner{t: t, root: t.TempDir(), slug: "feat",
		script: []scriptedPhase{{}, {open: 1, severity: "minor"}, {open: 0}, {}}}
	o, _ := newOrch(t, fake, 3, 0)

	out, err := o.Run()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != orchestrator.ResultAwaitingUAT {
		t.Fatalf("outcome = %+v, want awaiting-uat", out)
	}
	if got := strings.Join(callKinds(fake.calls), ","); got != "implement,review,test,report" {
		t.Fatalf("an open minor must be batched (no re-implement); order = %s", got)
	}
}

func callKinds(calls []orchestrator.Invocation) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.Kind
	}
	return out
}

func TestRegistryRoundTrip(t *testing.T) {
	root := t.TempDir()
	reg := &orchestrator.Registry{Slug: "feat", DevUUID: "dev-123", Round: 2, CostUSD: 1.5}
	if err := reg.Save(root); err != nil {
		t.Fatal(err)
	}
	got := orchestrator.LoadRegistry(root, "feat")
	if got.DevUUID != "dev-123" || got.Round != 2 {
		t.Errorf("reload = %+v, want dev-123/round 2", got)
	}
	// A missing registry degrades to a fresh (empty) one, never a crash.
	fresh := orchestrator.LoadRegistry(root, "nope")
	if fresh.DevUUID != "" {
		t.Errorf("missing registry should be fresh, got %+v", fresh)
	}
	// A garbled registry also degrades to fresh.
	path := orchestrator.RegistryPath(root, "bad")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("{not json"), 0o644)
	if orchestrator.LoadRegistry(root, "bad").DevUUID != "" {
		t.Errorf("garbled registry should degrade to fresh")
	}
}

func TestRunnableStatus(t *testing.T) {
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
}
