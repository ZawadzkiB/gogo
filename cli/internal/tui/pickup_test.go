package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// member builds a plan-member work item feature (a correlation-stamped card) for the
// auto-pickup tests.
func member(slug, source, root, status string) *contract.Feature {
	return &contract.Feature{
		Slug: slug, Title: slug, Source: source, Root: root, Project: "app",
		Status: status, Class: contract.ClassUnfinished, Correlations: []string{"plan-x"},
	}
}

// pickupModel builds a unified-board Model over the given features + one project whose
// single "web" source has the given skip flag + cap, with a fake launcher recording the
// fired intents. hasClaude is forced on and sessions are injectable.
func pickupModel(t *testing.T, skip bool, cap int, sessions []string, feats ...*contract.Feature) (Model, *[]launch.Intent) {
	t.Helper()
	proj := projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/r/web", PlanAcceptanceSkip: skip, ConcurrentWorkItems: cap},
	}}
	m := NewWorkspaceAll(&contract.Repo{Features: feats}, []projects.Project{proj})
	m.hasClaude = true
	m.sessions = sessions
	fired := &[]launch.Intent{}
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		*fired = append(*fired, in)
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}
	return m, fired
}

// TestAutoPickupFiresIntoFreeSlot pins plans-board FR6 case (a): a skip-source member at
// plan-accepted, no live session, its source under cap → autoPickupCmds fires exactly one
// `/gogo:go` (byte-for-byte a manual go: right root + the source's --skip-acceptance) and
// records the fire-once key.
func TestAutoPickupFiresIntoFreeSlot(t *testing.T) {
	seedDataHome(t)
	f := member("web-token", "web", "/r/web", "plan-accepted")
	m, fired := pickupModel(t, true /*skip*/, 1 /*cap*/, nil, f)

	cmds := m.autoPickupCmds()
	if len(cmds) != 1 {
		t.Fatalf("autoPickupCmds returned %d cmds, want 1 (one eligible member into a free slot)", len(cmds))
	}
	res, ok := cmds[0]().(autoPickupResultMsg)
	if !ok || !res.ok {
		t.Fatalf("auto-pickup cmd did not resolve to a successful autoPickupResultMsg: %#v (ok=%v)", res, ok)
	}
	if len(*fired) != 1 {
		t.Fatalf("launcher fired %d times, want exactly 1", len(*fired))
	}
	in := (*fired)[0]
	if in.Action != launch.ActionGo || in.Root != "/r/web" {
		t.Errorf("fired intent = %+v, want ActionGo at /r/web", in)
	}
	if !strings.Contains(in.Command, "/gogo:go web-token") {
		t.Errorf("fired command = %q, want /gogo:go web-token", in.Command)
	}
	if !strings.Contains(in.Command, "--skip-acceptance") {
		t.Errorf("fired command = %q, want the skip source's --skip-acceptance", in.Command)
	}
	if !m.autoPickedUp[featureKey(f)] {
		t.Error("auto-pickup did not record the fire-once key")
	}
}

// TestAutoPickupFireOnce pins case (b): once a member has auto-fired, a second reconcile
// does NOT relaunch it (the fire-once set).
func TestAutoPickupFireOnce(t *testing.T) {
	seedDataHome(t)
	f := member("web-token", "web", "/r/web", "plan-accepted")
	m, fired := pickupModel(t, true, 1, nil, f)

	cmds := m.autoPickupCmds() // first reconcile: one cmd + records the fire-once key
	for _, c := range cmds {
		c() // run it (fires the fake launcher)
	}
	if len(*fired) != 1 {
		t.Fatalf("first reconcile fired %d, want 1", len(*fired))
	}
	// A second reconcile on the SAME model (fire-once set carries the key) fires nothing.
	if cmds2 := m.autoPickupCmds(); len(cmds2) != 0 {
		t.Errorf("second reconcile returned %d cmds, want 0 (fire-once)", len(cmds2))
	}
}

// TestAutoPickupSkipsNonSkipSource pins case (c): a member whose source did NOT opt out of
// the plan gate (PlanAcceptanceSkip=false) is never auto-fired — it waits for a manual go.
func TestAutoPickupSkipsNonSkipSource(t *testing.T) {
	seedDataHome(t)
	f := member("web-token", "web", "/r/web", "plan-accepted")
	m, _ := pickupModel(t, false /*NOT skip*/, 1, nil, f)

	if cmds := m.autoPickupCmds(); len(cmds) != 0 {
		t.Errorf("a non-skip source member was auto-fired (%d cmds), want 0", len(cmds))
	}
	if m.autoPickupReady(f) {
		t.Error("autoPickupReady true for a non-skip source, want false")
	}
}

// TestAutoPickupRefusesAnUnwrittenPlan pins FR8 on the ONE launch path with no human in the
// loop (REV-003). The board's `m`/`M` and `gogo go` both refuse a plan-accepted card whose
// plan.md is absent or a stub; before this guard the reload auto-pickup did not, so the
// UNATTENDED path was the most permissive one - it fired `claude -p /gogo:go <slug>
// --skip-acceptance` at a folder with no plan in it. That is reachable by exactly the failure
// this feature exists to fix: an analyst writing state.md before plan.md in a source whose
// planAcceptanceSkip auto-clears the plan gate.
//
// It asserts BOTH arms, so the guard is proven to bite only on the defect: zero launches for
// the unwritten member, exactly one for an otherwise-identical written one.
func TestAutoPickupRefusesAnUnwrittenPlan(t *testing.T) {
	seedDataHome(t)
	unwritten := member("web-token", "web", "/r/web", "plan-accepted")
	unwritten.PlanUnwritten = true
	m, fired := pickupModel(t, true /*skip*/, 1 /*cap*/, nil, unwritten)

	if m.autoPickupReady(unwritten) {
		t.Error("autoPickupReady = true for a plan-accepted member with an UNWRITTEN plan.md - " +
			"the unattended path must be at least as strict as the board's m")
	}
	cmds := m.autoPickupCmds()
	if len(cmds) != 0 {
		t.Fatalf("autoPickupCmds fired %d cmd(s) for an unwritten plan, want 0", len(cmds))
	}
	// Nothing recorded either, so a later reload cannot "resume" the illegal launch.
	if m.autoPickedUp[featureKey(unwritten)] {
		t.Error("the unwritten member was recorded in the fire-once set")
	}
	if len(*fired) != 0 {
		t.Errorf("the launcher fired %d time(s): %+v - an unattended build was launched at a folder with no plan", len(*fired), *fired)
	}
	// It is ILLEGAL, not cap-blocked: the misleading "trigger manually" cue must NOT show,
	// because pressing m would refuse too. The card carries the plan-not-written cue instead.
	if m.autoPickupBlocked(unwritten) {
		t.Error("autoPickupBlocked = true - an unwritten plan is not a busy repo, and telling the " +
			"user to trigger it manually would send them into a refusal")
	}
	if bounce := planReadinessBounce(unwritten); bounce == "" {
		t.Error("planReadinessBounce is empty for the same card - the two paths disagree")
	}

	// The other arm: an identical member WITH a written plan still fires exactly once, so the
	// new guard cannot be passing because auto-pickup stopped working.
	written := member("web-token", "web", "/r/web", "plan-accepted")
	m2, fired2 := pickupModel(t, true, 1, nil, written)
	cmds2 := m2.autoPickupCmds()
	if len(cmds2) != 1 {
		t.Fatalf("a WRITTEN-plan member fired %d cmd(s), want 1 - the guard is over-refusing", len(cmds2))
	}
	if res, ok := cmds2[0]().(autoPickupResultMsg); !ok || !res.ok {
		t.Fatalf("the written-plan auto-pickup did not resolve successfully: %#v", res)
	}
	if len(*fired2) != 1 || !strings.Contains((*fired2)[0].Command, "/gogo:go web-token") {
		t.Errorf("launcher fired %+v, want exactly one /gogo:go web-token", *fired2)
	}
}

// TestAutoPickupAtCapShowsCue pins case (d): a skip-source member whose source is AT its cap
// (ConcurrentWorkItems=1 with a busy sibling) is NOT fired, and the "needs manual trigger"
// cue renders on both its work-board card and its plan card.
func TestAutoPickupAtCapShowsCue(t *testing.T) {
	seedDataHome(t)
	eligible := member("web-token", "web", "/r/web", "plan-accepted")
	// A busy sibling at the same source: in-progress + a live session → occupies the 1 slot.
	busy := &contract.Feature{Slug: "busy-item", Source: "web", Root: "/r/web", Project: "app",
		Status: "implementing", Class: contract.ClassInProgress}
	m, _ := pickupModel(t, true, 1 /*cap*/, []string{"gogo-go-busy-item"}, eligible, busy)

	if cmds := m.autoPickupCmds(); len(cmds) != 0 {
		t.Errorf("at-cap member was auto-fired (%d cmds), want 0 (repo busy)", len(cmds))
	}
	if !m.autoPickupBlocked(eligible) {
		t.Fatal("autoPickupBlocked false for an at-cap eligible member, want true")
	}
	// The cue renders on the work-board card.
	if card := m.renderCard(0, eligible, false, 60); !strings.Contains(card, "trigger manually") {
		t.Errorf("work-board card missing the manual-trigger cue:\n%s", card)
	}
	// The cue renders on the plan card (a plan targeting web with this member spawned).
	p := plans.Plan{ID: "plan-x", Title: "Rollout", Status: plans.StatusActive, Targets: []string{"web"}}
	if pc := m.planPickupCue(p, false); pc == "" || !strings.Contains(pc, "trigger manually") {
		t.Errorf("planPickupCue = %q, want the manual-trigger cue", pc)
	}
	// A cap-skip is NOT recorded, so once a slot frees it can still fire.
	if m.autoPickedUp[featureKey(eligible)] {
		t.Error("a cap-blocked member was wrongly recorded in the fire-once set")
	}
}

// TestAutoPickupUnderCapWithOneFreeSlot pins case (e): ConcurrentWorkItems=2 with one busy
// sibling still has a free slot → the eligible member fires (the cap is the source's INTEGER,
// per-source, so 1-of-2 busy is under cap).
func TestAutoPickupUnderCapWithOneFreeSlot(t *testing.T) {
	seedDataHome(t)
	eligible := member("web-token", "web", "/r/web", "plan-accepted")
	busy := &contract.Feature{Slug: "busy-item", Source: "web", Root: "/r/web", Project: "app",
		Status: "implementing", Class: contract.ClassInProgress}
	m, fired := pickupModel(t, true, 2 /*cap=2*/, []string{"gogo-go-busy-item"}, eligible, busy)

	cmds := m.autoPickupCmds()
	if len(cmds) != 1 {
		t.Fatalf("cap=2 with one free slot returned %d cmds, want 1 (under cap)", len(cmds))
	}
	cmds[0]()
	if len(*fired) != 1 {
		t.Fatalf("launcher fired %d times, want 1", len(*fired))
	}
}

// TestAutoPickupTransientUnblocksOnFreedSlot pins the temporal half of case (e) that
// TestAutoPickupAtCapShowsCue and TestAutoPickupUnderCapWithOneFreeSlot each prove only a
// static half of: a member blocked at cap (cue shown, NOT fire-once-recorded) then, on a
// LATER reload once its source's busy sibling's session ends and a slot frees, auto-fires
// and the cue clears — the queue drains itself without ever having been permanently marked
// manual-only.
func TestAutoPickupTransientUnblocksOnFreedSlot(t *testing.T) {
	seedDataHome(t)
	eligible := member("web-token", "web", "/r/web", "plan-accepted")
	busy := &contract.Feature{Slug: "busy-item", Source: "web", Root: "/r/web", Project: "app",
		Status: "implementing", Class: contract.ClassInProgress}
	m, fired := pickupModel(t, true, 1 /*cap=1*/, []string{"gogo-go-busy-item"}, eligible, busy)

	// Reload #1: the sibling's live session occupies the only slot → blocked, cued, and NOT
	// recorded in the fire-once set (so it stays eligible for a later reload).
	if cmds := m.autoPickupCmds(); len(cmds) != 0 {
		t.Fatalf("reload #1: %d cmds fired, want 0 (source at cap)", len(cmds))
	}
	if !m.autoPickupBlocked(eligible) {
		t.Fatal("reload #1: autoPickupBlocked false for the at-cap member, want true")
	}
	if card := m.renderCard(0, eligible, false, 60); !strings.Contains(card, "trigger manually") {
		t.Errorf("reload #1: work-board card missing the manual-trigger cue:\n%s", card)
	}
	if m.autoPickedUp[featureKey(eligible)] {
		t.Fatal("reload #1: the blocked member was wrongly recorded in the fire-once set")
	}

	// The busy sibling's session ends (its job finished) — a slot frees.
	m.sessions = nil

	// Reload #2 (later): now under cap → it auto-fires, and the cue clears.
	cmds := m.autoPickupCmds()
	if len(cmds) != 1 {
		t.Fatalf("reload #2: %d cmds fired, want 1 (slot freed, transient auto-fire)", len(cmds))
	}
	cmds[0]()
	if len(*fired) != 1 {
		t.Fatalf("reload #2: launcher fired %d times, want 1", len(*fired))
	}
	if m.autoPickupBlocked(eligible) {
		t.Error("reload #2: autoPickupBlocked still true after the slot freed, want the cue to clear")
	}
}

// TestAutoPickupNotDoubleFiredWithLiveSession pins case (f): a skip-source member that
// already has its OWN live session is not auto-fired (the dedupe guard).
func TestAutoPickupNotDoubleFiredWithLiveSession(t *testing.T) {
	seedDataHome(t)
	f := member("web-token", "web", "/r/web", "plan-accepted")
	// The member itself already has a live go session.
	m, _ := pickupModel(t, true, 1, []string{"gogo-go-web-token"}, f)

	if cmds := m.autoPickupCmds(); len(cmds) != 0 {
		t.Errorf("a member with a live session was auto-fired (%d cmds), want 0", len(cmds))
	}
}

// TestAutoPickupIgnoresRealGate pins case (g): auto-pickup keys ONLY on the initial
// plan-accepted handoff — a member parked mid-pipeline at waiting-for-user (a real decision
// gate) is NOT auto-fired, so a genuine gate is never plowed through by the pickup.
func TestAutoPickupIgnoresRealGate(t *testing.T) {
	seedDataHome(t)
	parked := member("web-token", "web", "/r/web", "waiting-for-user")
	m, _ := pickupModel(t, true, 1, nil, parked)

	if cmds := m.autoPickupCmds(); len(cmds) != 0 {
		t.Errorf("a waiting-for-user member was auto-fired (%d cmds), want 0 (real gate not waived)", len(cmds))
	}
	if m.autoPickupReady(parked) {
		t.Error("autoPickupReady true for a parked (waiting-for-user) member, want false")
	}
}

// TestAutoPickupNoClaudeIsInert: with no claude on PATH, the reconcile is inert (never
// launches — a pickup is always a launched `claude -p`).
func TestAutoPickupNoClaudeIsInert(t *testing.T) {
	seedDataHome(t)
	f := member("web-token", "web", "/r/web", "plan-accepted")
	m, _ := pickupModel(t, true, 1, nil, f)
	m.hasClaude = false // no claude on PATH → auto-pickup is inert (never launches)

	if cmds := m.autoPickupCmds(); len(cmds) != 0 {
		t.Errorf("auto-pickup fired with no claude on PATH (%d cmds), want 0", len(cmds))
	}
}

// TestAutoPickupLaunchErrorRetries pins REV-001: a FAILED auto-pickup launch un-records the
// fire-once key (via autoPickupResultMsg{ok:false} through Update) so the NEXT reload retries
// it — a launcher error must not strand the member permanently fired-but-never-launched.
func TestAutoPickupLaunchErrorRetries(t *testing.T) {
	seedDataHome(t)
	f := member("web-token", "web", "/r/web", "plan-accepted")
	m, _ := pickupModel(t, true, 1, nil, f)
	m.launcher = func(string, launch.Intent) (launch.Result, error) {
		return launch.Result{}, errors.New("claude not found")
	}

	cmds := m.autoPickupCmds()
	if len(cmds) != 1 {
		t.Fatalf("reconcile returned %d cmds, want 1", len(cmds))
	}
	if !m.autoPickedUp[featureKey(f)] {
		t.Fatal("reconcile did not record the fire-once key synchronously")
	}
	// The launch resolves to a FAILED result; feeding it through Update un-records the key.
	msg := cmds[0]()
	res, ok := msg.(autoPickupResultMsg)
	if !ok || res.ok {
		t.Fatalf("failed launch did not resolve to autoPickupResultMsg{ok:false}: %#v", msg)
	}
	nm, _ := m.Update(res)
	m = nm.(Model)
	if m.autoPickedUp[featureKey(f)] {
		t.Error("a failed launch left the fire-once key recorded (would strand the member — REV-001)")
	}
	// The next reconcile retries it (the key is gone).
	if cmds2 := m.autoPickupCmds(); len(cmds2) != 1 {
		t.Errorf("after a failed launch the next reconcile returned %d cmds, want 1 (retry)", len(cmds2))
	}
}

// TestAutoPickupFiresOnReload pins the wiring (REV-002): a reloadMsg driven through Update
// runs the reconcile on the reload path — it re-reads the on-disk plan-accepted skip-source
// member, records the fire-once key, and returns a non-nil batch (the pickup launch(es) +
// waitForReload), rather than the reconcile only ever being called directly.
func TestAutoPickupFiresOnReload(t *testing.T) {
	seedDataHome(t)
	srcRoot := t.TempDir()
	writePlanAcceptedMember(t, srcRoot, "web-token", "plan-x")
	proj := projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: srcRoot, PlanAcceptanceSkip: true, ConcurrentWorkItems: 1},
	}}
	m := NewCockpit([]projects.Project{proj}) // reads disk (contract.LoadWorkspace)
	m.hasClaude = true
	m.sessions = nil
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	nm, cmd := m.Update(reloadMsg{})
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("reloadMsg returned no cmd (expected the pickup batch + waitForReload)")
	}
	// The reconcile ran ON THE RELOAD PATH and recorded a fire-once key for the eligible
	// on-disk member — proof the wiring (reloadMsg → autoPickupCmds) is connected.
	if len(m.autoPickedUp) != 1 {
		t.Fatalf("reloadMsg recorded %d auto-pickup keys, want 1 (reconcile not wired to the reload path?)", len(m.autoPickedUp))
	}
}

// writePlanAcceptedMember writes an on-disk plan-accepted work item (state.md + a WRITTEN
// plan.md) carrying the plan correlation, so the contract reader classifies it as an eligible
// auto-pickup member.
//
// The plan.md matters: since 0.29.0 an auto-pickup candidate must have a written plan
// (FR8/REV-003), and this fixture stands in for a member whose plan the source's
// planAcceptanceSkip already accepted - so a folder with no plan in it would not be a
// realistic fixture, it would be the defect.
func writePlanAcceptedMember(t *testing.T, root, slug, planID string) {
	t.Helper()
	dir := filepath.Join(root, ".gogo", "work", "feature-"+slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "- **feature:** " + slug + "\n" +
		"- **phase:** plan\n" +
		"- **status:** plan-accepted\n" +
		"- **created:** 2026-07-23\n" +
		"- **correlation:** [" + planID + "]\n"
	if err := os.WriteFile(filepath.Join(dir, "state.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := "# Plan - " + slug + "\n\nStatus: **accepted** (user, 2026-07-23)\n\n" +
		"## Goal\n\nauto-pickup fixture member.\n\n## Changes checklist\n\n- nothing (fixture only).\n"
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
}
