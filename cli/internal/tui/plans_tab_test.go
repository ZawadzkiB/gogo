package tui

import (
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
)

// TestPlansTabListRendersGrouped: the plans tab lists the project's plans grouped by
// status (ACTIVE · READY · DRAFTS), each with its ⛓ plan-XXXX chip, and the header
// counts.
func TestPlansTabListRendersGrouped(t *testing.T) {
	seedDataHome(t)
	active, _ := plans.New("app", "Shipping epic", "")
	plans.SetStatus("app", active.ID, plans.StatusActive)
	ready, _ := plans.New("app", "Ready plan", "")
	plans.MarkReady("app", ready.ID)
	draft, _ := plans.New("app", "A draft idea", "")

	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("svc", "/r/svc")))
	m = tab(m) // → plans
	if m.tab != tabPlans {
		t.Fatalf("did not reach plans tab")
	}
	out := m.View()
	for _, want := range []string{
		"ACTIVE", "READY", "DRAFTS",
		"1 active · 1 ready · 1 drafts",
		"Shipping epic", "Ready plan", "A draft idea",
		"⛓ " + active.ID, "⛓ " + draft.ID,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plans list missing %q:\n%s", want, out)
		}
	}
}

// TestPlanDetailRendersTargetSources: opening a plan (enter) shows the breadcrumb,
// the ⛓ chip, and a TARGET SOURCES row per target with the ＋ create-work-item
// affordance when nothing is spawned yet.
func TestPlanDetailRendersTargetSources(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Wire up auth", "seed the auth flow")
	plans.AddTarget("app", p.ID, "web")

	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	m = tab(m)                                  // → plans
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // enter — open the focused plan's detail
	if m.planDetail == nil {
		t.Fatalf("enter did not open the plan detail")
	}
	out := m.View()
	for _, want := range []string{"plans / Wire up auth", "⛓ " + p.ID, "seed the auth flow", "TARGET SOURCES", "web", "create work item"} {
		if !strings.Contains(out, want) {
			t.Errorf("plan detail missing %q:\n%s", want, out)
		}
	}
}

// TestPlanDetailCreateWorkItemFiresLauncherOnce pins FR11/the load-bearing spawn:
// `c create work item` on a target source builds launch.PlanIntent(plan.Title, body,
// plan.ID) — carrying `--correlation plan-XXXX` — and fires the launcher seam EXACTLY
// ONCE, anchored at the SOURCE root. The CLI writes nothing under the source's .gogo/.
func TestPlanDetailCreateWorkItemFiresLauncherOnce(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("web", "Token migration", "move the shared token store")
	plans.AddTarget("web", p.ID, "web")

	m := NewWorkspace(&contract.Repo{}, proj("web", src("web", "/repos/web")))
	m.hasClaude = true // deterministic — the spawn guard must not bounce on a CI box without claude
	var calls int
	var gotRoot string
	var gotIntent launch.Intent
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		calls++
		gotRoot, gotIntent = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	// Open the plans tab + the plan detail, focused on the one target source.
	m.tab = tabPlans
	detail := p
	detail.Targets = []string{"web"}
	m.planDetail = &detail
	m.planSourceIdx = 0

	nm, cmd := m.Update(runes("c"))
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("`c` returned a nil cmd — no launch was scheduled")
	}
	// Executing the returned cmd is what fires the launcher (fire-exactly-once).
	if _, ok := cmd().(launchDoneMsg); !ok {
		t.Fatal("create-work-item cmd did not resolve to a launchDoneMsg")
	}
	if calls != 1 {
		t.Fatalf("launcher fired %d times, want exactly 1", calls)
	}
	if gotRoot != "/repos/web" {
		t.Errorf("spawned in %q, want the source root /repos/web", gotRoot)
	}
	if !strings.HasPrefix(gotIntent.Command, "/gogo:plan move the shared token store") {
		t.Errorf("command = %q, want the plan body seeded whole", gotIntent.Command)
	}
	if !strings.HasSuffix(gotIntent.Command, "--correlation "+p.ID) {
		t.Errorf("command = %q, want the --correlation %s param", gotIntent.Command, p.ID)
	}
	// The spawn recorded an advisory member + advanced the plan to active (store only).
	if got, _ := plans.Get("web", p.ID); got.Status != plans.StatusActive || len(got.Members) != 1 {
		t.Errorf("after spawn plan = %+v, want active with one member", got)
	}
}

// TestPlanListNewAndDelete: `n` opens the title form (create draft) and `x` deletes
// the focused plan — the CLI-owned store mutates, the board is untouched.
func TestPlanListNewAndDelete(t *testing.T) {
	seedDataHome(t)
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("svc", "/r/svc")))
	m = tab(m) // → plans

	// n → the title form under modeForm.
	m = send(m, runes("n"))
	if m.mode != modeForm || !m.pendingPlan {
		t.Fatalf("n did not open the new-plan form (mode=%d pendingPlan=%v)", m.mode, m.pendingPlan)
	}
	// Fill the heap-stable binding (what finishPlanForm reads, TEST-001) and complete
	// directly (bypassing huh's own field pump, like the config-form tests).
	m.binding.planTitle = "Fresh idea"
	nm, _ := m.finishPlanForm()
	m = nm.(Model)
	if list, _ := plans.List("app"); len(list) != 1 || list[0].Title != "Fresh idea" {
		t.Fatalf("after n+complete: plans = %v, want one 'Fresh idea' draft", list)
	}

	// x → delete the focused plan.
	m.planIdx = 0
	m = send(m, runes("x"))
	if list, _ := plans.List("app"); len(list) != 0 {
		t.Errorf("after x: %d plans, want 0 (deleted)", len(list))
	}
}

// TestPlanWithClaudeMintsAndFires pins the FR-D `A` plan-with-claude authoring
// trigger (REV-002): it MINTS a fresh draft plan up front (so its plan-<hash> exists)
// and fires the launcher seam EXACTLY ONCE with a PLAIN authoring intent — a prompt
// referencing the CLI-owned plan-file path + the correlation id, and explicitly NOT a
// /gogo:plan slash command and NOT a --correlation flag (that skill would scaffold a
// source work item). The session is anchored at the project's FIRST SOURCE root (a
// trusted repo), never the untrusted ~/.gogo/ home; no real claude/tmux spawn.
func TestPlanWithClaudeMintsAndFires(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/repos/web")))
	m.hasClaude = true // deterministic — the spawn guard must not bounce on a CI box without claude
	m.tab = tabPlans

	var calls int
	var gotRoot string
	var gotIntent launch.Intent
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		calls++
		gotRoot, gotIntent = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	if list, _ := plans.List("app"); len(list) != 0 {
		t.Fatalf("expected no plans before A, got %d", len(list))
	}

	nm, cmd := m.Update(runes("A"))
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("`A` returned a nil cmd — no authoring session was scheduled")
	}
	// Executing the returned cmd is what fires the launcher (fire-exactly-once).
	if _, ok := cmd().(launchDoneMsg); !ok {
		t.Fatal("plan-with-claude cmd did not resolve to a launchDoneMsg")
	}
	if calls != 1 {
		t.Fatalf("launcher fired %d times, want exactly 1", calls)
	}

	// A fresh DRAFT plan was minted up front.
	list, _ := plans.List("app")
	if len(list) != 1 || list[0].Status != plans.StatusDraft {
		t.Fatalf("A did not mint exactly one draft plan: %+v", list)
	}
	id := list[0].ID

	if gotIntent.Action != launch.ActionAuthor {
		t.Errorf("intent action = %v, want ActionAuthor (a plain authoring session)", gotIntent.Action)
	}
	// A PLAIN authoring prompt — NOT a slash-command launch, and NOT a --correlation flag.
	if strings.HasPrefix(gotIntent.Command, "/") {
		t.Errorf("command = %q, must be a plain prompt, not a slash-command launch (e.g. /gogo:plan scaffolds a source work item)", gotIntent.Command)
	}
	if strings.Contains(gotIntent.Command, "--correlation") {
		t.Errorf("command = %q, must NOT carry a --correlation flag (a plain session, not a spawn)", gotIntent.Command)
	}
	// It references the plan file path + the correlation id (in prose).
	if !strings.Contains(gotIntent.Command, plans.Path("app", id)) {
		t.Errorf("command = %q, want it to reference the plan file path %q", gotIntent.Command, plans.Path("app", id))
	}
	if !strings.Contains(gotIntent.Command, id) {
		t.Errorf("command = %q, want it to reference the correlation id %s", gotIntent.Command, id)
	}
	// Anchored at the project's FIRST SOURCE root (trusted repo), never the ~/.gogo home.
	if gotRoot != "/repos/web" {
		t.Errorf("anchored at %q, want the first source root /repos/web (never the ~/.gogo home)", gotRoot)
	}
	if gotRoot == projects.Dir("app") {
		t.Errorf("anchored at the untrusted project home %q — must anchor at a source root", gotRoot)
	}
}

// TestPlanWithClaudeFallsBackToProjectHome pins the rare no-sources case (REV-002): a
// project with zero sources has no trusted repo to anchor at, so the author session
// falls back to the project home (with a note) — still a plain authoring intent, no
// /gogo:plan, no source `.gogo/work/`.
func TestPlanWithClaudeFallsBackToProjectHome(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("solo")) // no sources
	m.hasClaude = true
	m.tab = tabPlans

	var gotRoot string
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		gotRoot = root
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	nm, cmd := m.Update(runes("A"))
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("`A` returned a nil cmd on a source-less project")
	}
	if _, ok := cmd().(launchDoneMsg); !ok {
		t.Fatal("plan-with-claude cmd did not resolve to a launchDoneMsg")
	}
	if gotRoot != projects.Dir("solo") {
		t.Errorf("anchored at %q, want the project home %q (no source to anchor at)", gotRoot, projects.Dir("solo"))
	}
}

// TestPlanWithClaudeNoClaudeIsInert: with no claude on PATH, `A` neither mints a plan
// nor fires the launcher — it just surfaces a hint (so a half-authored empty draft is
// never left behind on a box that cannot launch the session).
func TestPlanWithClaudeNoClaudeIsInert(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/repos/web")))
	m.hasClaude = false
	m.tab = tabPlans
	fired := false
	m.launcher = func(string, launch.Intent) (launch.Result, error) { fired = true; return launch.Result{}, nil }

	nm, cmd := m.Update(runes("A"))
	m = nm.(Model)
	if cmd != nil {
		if _, ok := cmd().(launchDoneMsg); ok {
			// draining the cmd must not have launched
		}
	}
	if fired {
		t.Error("A fired the launcher with no claude on PATH")
	}
	if list, _ := plans.List("app"); len(list) != 0 {
		t.Errorf("A minted a plan with no claude available: %+v", list)
	}
	if !strings.Contains(m.status, "claude") {
		t.Errorf("status = %q, want a claude-not-found hint", m.status)
	}
}

// TestPlanCardPerSourceDotStates pins the FR10/FR11 per-source dot polish: an ACTIVE
// plan's card shows one dot per target source — a colored ● once a source is spawned,
// a dim `·` until then — and the plan detail spells out `· not created` (+ the ＋
// create affordance) on the un-spawned source's row while the spawned source shows its
// work-item slug.
func TestPlanCardPerSourceDotStates(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Rollout", "")
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "api")
	plans.SetStatus("app", p.ID, plans.StatusActive)

	// web is spawned (a work item carries the plan id); api is not created yet.
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "ship-web", Title: "Ship web", Source: "web", Root: "/r/web",
			Class: contract.ClassInProgress, Phase: "implement", Status: "implementing",
			Correlations: []string{p.ID}},
	}}
	m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web"), src("api", "/r/api")))
	m = tab(m) // → plans

	list := m.View()
	if !strings.Contains(list, "1 of 2 work items") {
		t.Errorf("plan card missing the 1-of-2 spawned count:\n%s", list)
	}
	// Per-source dots: web spawned (●) then api not-created (·).
	if !strings.Contains(list, "● ·") {
		t.Errorf("plan card missing the per-source dot strip (● spawned · not-created):\n%s", list)
	}

	det := send(m, tea.KeyMsg{Type: tea.KeyEnter}).View()
	if !strings.Contains(det, "ship-web") {
		t.Errorf("spawned web row missing its work-item slug:\n%s", det)
	}
	if !strings.Contains(det, "not created") || !strings.Contains(det, "create work item") {
		t.Errorf("un-spawned api row missing the `not created` / create affordance:\n%s", det)
	}
}

// TestPlanListSingleCursor pins the Phase-C nit fix: the focused plan carries a SINGLE
// focus indicator (the list cursor `▸`), never the doubled `▸ ▸` the always-on glyph
// used to produce — and the plan-detail target rows are single-cursor too.
func TestPlanListSingleCursor(t *testing.T) {
	seedDataHome(t)
	a, _ := plans.New("app", "Active one", "")
	plans.AddTarget("app", a.ID, "web")
	plans.SetStatus("app", a.ID, plans.StatusActive)

	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	m = tab(m) // → plans
	out := m.View()
	if strings.Contains(out, "▸ ▸") {
		t.Errorf("plans list doubled the cursor (`▸ ▸`):\n%s", out)
	}
	if !strings.Contains(out, "▸") {
		t.Errorf("plans list lost its focus cursor entirely:\n%s", out)
	}

	det := send(m, tea.KeyMsg{Type: tea.KeyEnter}).View()
	if strings.Contains(det, "▸ ▸") {
		t.Errorf("plan detail doubled the target-row cursor (`▸ ▸`):\n%s", det)
	}
}
