package tui

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
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

// TestPlanNewCapturesDescription pins the 0.25.1 description gap: the `n` quick-draft form
// captures a DESCRIPTION (not only a title), plans.New persists it, and the plan detail
// RENDERS it (no longer the "no description" placeholder).
func TestPlanNewCapturesDescription(t *testing.T) {
	seedDataHome(t)
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("svc", "/r/svc")))
	m = tab(m) // → plans

	m = send(m, runes("n"))
	if m.mode != modeForm || !m.pendingPlan {
		t.Fatalf("n did not open the new-plan form (mode=%d pendingPlan=%v)", m.mode, m.pendingPlan)
	}
	// Fill the heap-stable binding (TEST-001): title + the new description field.
	m.binding.planTitle = "Auth rework"
	m.binding.planDesc = "Rework the auth flow across web and api."
	nm, _ := m.finishPlanForm()
	m = nm.(Model)

	got, _ := plans.List("app")
	if len(got) != 1 || strings.TrimSpace(got[0].Description) != "Rework the auth flow across web and api." {
		t.Fatalf("n did not persist the description: %+v", got)
	}
	// The plan detail renders the actual description (not the placeholder).
	m.planIdx = 0
	det := send(m, tea.KeyMsg{Type: tea.KeyEnter}).View()
	if !strings.Contains(det, "Rework the auth flow across web and api.") {
		t.Errorf("plan detail did not render the description:\n%s", det)
	}
	if strings.Contains(det, "no description") {
		t.Errorf("plan detail still shows the no-description placeholder despite a set description:\n%s", det)
	}
}

// authWithClaude drives the plans-tab `A` goal form to completion (0.25.1): press A (opens
// the form, mints NOTHING yet), fill the heap-stable binding (TEST-001), complete via
// finishPlanWithClaude, run the returned launch cmd, and feed the resulting message back
// through Update so the attach (or the no-tmux headless status) lands. Returns the final
// model, the launch cmd's message, and the launch cmd (nil when goal was empty/cancelled).
func authWithClaude(t *testing.T, m Model, goal, title string) (Model, tea.Msg) {
	t.Helper()
	nm, cmd := m.Update(runes("A"))
	m = nm.(Model)
	if !m.pendingPlanWithClaude || m.mode != modeForm {
		t.Fatalf("A did not open the goal form (mode=%d pending=%v)", m.mode, m.pendingPlanWithClaude)
	}
	if cmd == nil {
		t.Fatal("A returned no form-init cmd")
	}
	m.binding.planGoal = goal
	m.binding.planTitle = title
	fm, lcmd := m.finishPlanWithClaude()
	m = fm.(Model)
	if lcmd == nil {
		return m, nil // empty goal / cancel — nothing launched
	}
	msg := lcmd() // fires the launcher exactly once
	nm2, _ := m.Update(msg)
	return nm2.(Model), msg
}

// TestPlanWithClaudeOpensGoalForm pins the 0.25.1 bug-1 fix: `A` opens a GOAL form and
// mints NOTHING up front (no blank "Untitled plan"), and does NOT fire the launcher until
// the goal is submitted.
func TestPlanWithClaudeOpensGoalForm(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/repos/web")))
	m.hasClaude = true
	m.tab = tabPlans
	fired := false
	m.launcher = func(string, launch.Intent) (launch.Result, error) { fired = true; return launch.Result{}, nil }

	nm, cmd := m.Update(runes("A"))
	m = nm.(Model)
	if !m.pendingPlanWithClaude || m.mode != modeForm {
		t.Fatalf("A did not open the goal form (mode=%d pending=%v)", m.mode, m.pendingPlanWithClaude)
	}
	if cmd == nil {
		t.Error("A returned no form-init cmd")
	}
	// The form is open — NOTHING minted and NOTHING launched yet (no blank draft).
	if list, _ := plans.List("app"); len(list) != 0 {
		t.Errorf("A minted a plan before the goal was given: %+v", list)
	}
	if fired {
		t.Error("A fired the launcher before the goal was submitted")
	}
	if out := m.View(); !strings.Contains(out, "goal") {
		t.Errorf("goal form did not render a goal prompt:\n%s", out)
	}
}

// TestPlanWithClaudeSubmitMintsSeedsAndAttaches pins the 0.25.1 fix end-to-end: submitting
// the goal form mints a draft whose DESCRIPTION IS THE GOAL (never blank), fires the
// launcher EXACTLY ONCE with a PLAIN AuthorPlanIntent that NAMES the goal (and the plan
// path / correlation / knowledge / source paths, no --correlation flag), anchored at the
// first source root — and, because the launcher returned a session name, ATTACHES the user
// into it (the "attaching <session>" observable). No real claude/tmux spawn.
func TestPlanWithClaudeSubmitMintsSeedsAndAttaches(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/repos/web")))
	m.hasClaude = true
	m.tab = tabPlans

	var calls int
	var gotRoot string
	var gotIntent launch.Intent
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		calls++
		gotRoot, gotIntent = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	const goal = "Migrate the shared token store to the new auth service"
	m, _ = authWithClaude(t, m, goal, "") // blank title → derived from the goal

	if calls != 1 {
		t.Fatalf("launcher fired %d times, want exactly 1", calls)
	}

	// Exactly one DRAFT minted, its description == the goal (never blank), title derived.
	list, _ := plans.List("app")
	if len(list) != 1 || list[0].Status != plans.StatusDraft {
		t.Fatalf("A did not mint exactly one draft plan: %+v", list)
	}
	p := list[0]
	if strings.TrimSpace(p.Description) != goal {
		t.Errorf("plan description = %q, want the goal %q (never blank)", p.Description, goal)
	}
	if p.Title == "" || p.Title == "Untitled plan" {
		t.Errorf("plan title = %q, want a title derived from the goal (not blank/Untitled)", p.Title)
	}

	// The seed is a PLAIN prompt naming the goal — never a slash command / --correlation flag.
	if gotIntent.Action != launch.ActionAuthor {
		t.Errorf("intent action = %v, want ActionAuthor", gotIntent.Action)
	}
	if strings.HasPrefix(gotIntent.Command, "/") {
		t.Errorf("command = %q, must be a plain prompt (not a slash command)", gotIntent.Command)
	}
	if strings.Contains(gotIntent.Command, "--correlation") {
		t.Errorf("command = %q, must NOT carry a --correlation flag", gotIntent.Command)
	}
	// The GOAL is NAMED in the prompt (bug-2: the analyst plans FOR THE GOAL).
	if !strings.Contains(gotIntent.Command, goal) {
		t.Errorf("command = %q, want it to NAME the user's goal %q", gotIntent.Command, goal)
	}
	for _, want := range []string{plans.Path("app", p.ID), p.ID, projects.KnowledgeDir("app"), "gogo-project-plan", "/repos/web"} {
		if !strings.Contains(gotIntent.Command, want) {
			t.Errorf("command = %q, want it to reference %q", gotIntent.Command, want)
		}
	}
	// Anchored at the first source root (trusted repo), never the ~/.gogo home.
	if gotRoot != "/repos/web" {
		t.Errorf("anchored at %q, want the first source root /repos/web", gotRoot)
	}
	// The whole prompt is ONE trailing argv element (injection-safe even with the goal's spaces).
	if argv := launch.TmuxNewSessionArgs(gotRoot, gotIntent); argv[len(argv)-1] != gotIntent.Command {
		t.Errorf("author prompt was split across argv: last element = %q", argv[len(argv)-1])
	}
	// Bug-2 fix: a session name came back → the TUI ATTACHED the user in (status observable).
	if !strings.Contains(m.status, "attaching "+gotIntent.Session) {
		t.Errorf("status = %q, want it to attach the analyst session %q", m.status, gotIntent.Session)
	}
}

// TestPlanWithClaudeCancelMintsNothing pins the 0.25.1 cancel path: an empty goal (or an
// Esc-cancel) mints NO plan and fires NO launcher — no half-authored blank draft is left.
func TestPlanWithClaudeCancelMintsNothing(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/repos/web")))
	m.hasClaude = true
	m.tab = tabPlans
	fired := false
	m.launcher = func(string, launch.Intent) (launch.Result, error) { fired = true; return launch.Result{}, nil }

	// Submit with an empty goal → treated as cancel (nothing minted, nothing launched).
	m, msg := authWithClaude(t, m, "", "")
	if msg != nil {
		t.Errorf("empty-goal submit scheduled a launch (%v), want none", msg)
	}
	if fired {
		t.Error("empty-goal submit fired the launcher")
	}
	if list, _ := plans.List("app"); len(list) != 0 {
		t.Errorf("empty-goal submit minted a plan: %+v", list)
	}

	// Esc-cancel of the open form also mints nothing.
	nm, _ := m.Update(runes("A"))
	m = nm.(Model)
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.pendingPlanWithClaude {
		t.Error("Esc did not clear the pending A form")
	}
	if list, _ := plans.List("app"); len(list) != 0 {
		t.Errorf("Esc-cancel minted a plan: %+v", list)
	}
}

// TestPlanWithClaudeNoTmuxHeadless pins the no-tmux fallback (0.25.1): when the launcher
// returns no session name (backgrounded `claude -p`), the TUI does NOT attempt an attach —
// it surfaces the headless status so the user knows the analyst is running in the background.
func TestPlanWithClaudeNoTmuxHeadless(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/repos/web")))
	m.hasClaude = true
	m.tab = tabPlans
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		// No tmux → background mode: no Session name.
		return launch.Result{Mode: "background", LogPath: "/tmp/author.log", Command: in.Command}, nil
	}

	m, _ = authWithClaude(t, m, "Add a rate limiter across the gateway sources", "")
	if strings.Contains(m.status, "attaching") {
		t.Errorf("no-tmux path attempted an attach: %q", m.status)
	}
	if !strings.Contains(m.status, "no tmux") || !strings.Contains(m.status, "headless") {
		t.Errorf("status = %q, want the no-tmux headless note", m.status)
	}
	// REV-006: the headless status NAMES the background log path so a stalled/failed run
	// is inspectable (the pre-0.25.1 `A` surfaced it; the patch had dropped it).
	if !strings.Contains(m.status, "/tmp/author.log") {
		t.Errorf("status = %q, want it to name the background log path /tmp/author.log", m.status)
	}
	// The draft is still minted (the analyst has a file to write).
	if list, _ := plans.List("app"); len(list) != 1 {
		t.Errorf("no-tmux path did not mint the draft: %+v", list)
	}
}

// TestPlanWithClaudeNoTmuxNoSourceSurfacesAnchorNote pins REV-008: on the no-tmux + no-source
// path the headless status restores the dropped anchor heads-up ("runs in the project home;
// approve it if Claude prompts") so a backgrounded run that could stall on a first-run trust
// prompt in the untrusted ~/.gogo home warns the user — alongside the REV-006 log path.
func TestPlanWithClaudeNoTmuxNoSourceSurfacesAnchorNote(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("solo")) // no sources → falls back to the project home
	m.hasClaude = true
	m.tab = tabPlans
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		return launch.Result{Mode: "background", LogPath: "/tmp/author.log", Command: in.Command}, nil
	}

	m, _ = authWithClaude(t, m, "Stand up the project scaffolding", "")
	if !strings.Contains(m.status, "project home") || !strings.Contains(m.status, "approve it if Claude prompts") {
		t.Errorf("status = %q, want the no-source anchor heads-up (project home / trust prompt)", m.status)
	}
	if !strings.Contains(m.status, "/tmp/author.log") {
		t.Errorf("status = %q, want it to still name the background log path", m.status)
	}
}

// TestPlanWithClaudeFallsBackToProjectHome pins the rare no-sources case: a project with
// zero sources has no trusted repo to anchor at, so the author session falls back to the
// project home — still a plain authoring intent, no /gogo:plan, no source `.gogo/work/`.
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

	authWithClaude(t, m, "Stand up the project scaffolding", "")
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

// TestPlansTabDerivedAwaitingProjectUAT (FR3): a plan whose every member work item is
// shipped derives the display status `awaiting-project-uat` on its card + detail
// (distinct from a still-building `active`), while a plan with an unshipped member keeps
// showing `active`.
func TestPlansTabDerivedAwaitingProjectUAT(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Cross-repo migration", "")
	plans.AddMember("app", p.ID, plans.Member{Source: "web", SlugHint: "cross-repo-migration"})
	plans.SetStatus("app", p.ID, plans.StatusActive)

	// The web member is SHIPPED (state.md status: shipped) → the plan derives the gate.
	shipped := &contract.Repo{Features: []*contract.Feature{
		{Slug: "cross-web", Title: "Web side", Source: "web", Root: "/r/web",
			Class: contract.ClassShipped, Status: "shipped", Correlations: []string{p.ID}},
	}}
	m := sizedWorkspace(t, shipped, proj("app", src("web", "/r/web")))
	m = tab(m) // → plans
	if out := m.View(); !strings.Contains(out, plans.StatusAwaitingProjectUAT) {
		t.Errorf("plans list did not derive awaiting-project-uat for an all-shipped plan:\n%s", out)
	}
	if det := send(m, tea.KeyMsg{Type: tea.KeyEnter}).View(); !strings.Contains(det, plans.StatusAwaitingProjectUAT) {
		t.Errorf("plan detail did not derive awaiting-project-uat:\n%s", det)
	}

	// A member still building keeps the plan at `active` (no derived gate).
	building := &contract.Repo{Features: []*contract.Feature{
		{Slug: "cross-web", Title: "Web side", Source: "web", Root: "/r/web",
			Class: contract.ClassInProgress, Phase: "implement", Status: "implementing", Correlations: []string{p.ID}},
	}}
	m2 := sizedWorkspace(t, building, proj("app", src("web", "/r/web")))
	m2 = tab(m2)
	if det := send(m2, tea.KeyMsg{Type: tea.KeyEnter}).View(); strings.Contains(det, plans.StatusAwaitingProjectUAT) {
		t.Errorf("plan detail derived the UAT gate with an unshipped member:\n%s", det)
	}
}

// TestPlansTabAcceptProjectUAT (FR3, `D`): the TUI project-UAT accept mirrors `gogo plan
// done`. It REFUSES (a status naming the unshipped member, no confirm, plan stays active)
// while a member is unshipped, and — once every member ships — `D` opens the accept
// confirm whose completion records the accept (MarkDone: a `## Project UAT` round + the
// persisted `done`).
func TestPlansTabAcceptProjectUAT(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Cross-repo migration", "")
	plans.AddMember("app", p.ID, plans.Member{Source: "web", SlugHint: "cross-repo-migration"})
	plans.SetStatus("app", p.ID, plans.StatusActive)

	// Refuse: the web member is not shipped yet.
	building := &contract.Repo{Features: []*contract.Feature{
		{Slug: "cross-web", Title: "Web side", Source: "web", Root: "/r/web",
			Class: contract.ClassInProgress, Phase: "implement", Status: "implementing", Correlations: []string{p.ID}},
	}}
	m := sizedWorkspace(t, building, proj("app", src("web", "/r/web")))
	m = tab(m) // → plans
	m = send(m, runes("D"))
	if m.pendingPlanDone != nil {
		t.Fatalf("D opened a confirm despite an unshipped member (pending=%v)", m.pendingPlanDone)
	}
	if !strings.Contains(m.status, "not shipped") {
		t.Errorf("refuse status = %q, want a 'not shipped' message naming the member", m.status)
	}
	if got, _ := plans.Get("app", p.ID); got.Status != plans.StatusActive {
		t.Errorf("plan flipped despite the members-shipped guard: %s", got.Status)
	}

	// Accept: the member ships → D opens the confirm → completing it records the accept.
	shipped := &contract.Repo{Features: []*contract.Feature{
		{Slug: "cross-web", Title: "Web side", Source: "web", Root: "/r/web",
			Class: contract.ClassShipped, Status: "shipped", Correlations: []string{p.ID}},
	}}
	m2 := sizedWorkspace(t, shipped, proj("app", src("web", "/r/web")))
	m2 = tab(m2)
	m2 = send(m2, runes("D"))
	if m2.pendingPlanDone == nil || m2.mode != modeForm {
		t.Fatalf("D did not open the project-UAT accept confirm (mode=%d pending=%v)", m2.mode, m2.pendingPlanDone)
	}
	// Confirm through the heap-stable binding (TEST-001) and complete.
	m2.binding.confirm = true
	fm, _ := m2.finishPlanDone()
	m2 = fm.(Model)
	got, _ := plans.Get("app", p.ID)
	if got.Status != plans.StatusDone {
		t.Fatalf("after D-accept the plan is %s, want done", got.Status)
	}
	if !strings.Contains(got.Description, "Project UAT") {
		t.Errorf("MarkDone did not append a project-UAT round:\n%s", got.Description)
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

// TestPlansTabAcceptSpawnsPerTarget pins the 0.25.0 FR2 auto-spawn (`r` accept): a plan
// with 3 analyst-chosen targets opens a confirm listing them, and on accept fires the
// launcher ONCE per target — each `/gogo:plan` carrying that target's per-source BRIEF as
// the goal + the plan correlation id, the plan-acceptance-skip source getting
// `--skip-acceptance` — records 3 members, and flips the plan `active`.
func TestPlansTabAcceptSpawnsPerTarget(t *testing.T) {
	seedDataHome(t)
	body := `## Goal
Roll out the new token flow.

## Source briefs
### web
Rewire the web token client.

### api
Add the token endpoint.

### worker
Rotate tokens on schedule.`
	p, _ := plans.New("app", "Rollout", body)
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "api")
	plans.AddTarget("app", p.ID, "worker")

	project := projects.Project{Name: "app", Sources: []projects.Source{
		{Name: "web", Path: "/r/web"},
		{Name: "api", Path: "/r/api", PlanAcceptanceSkip: true}, // opted OUT of the plan-acceptance gate
		{Name: "worker", Path: "/r/worker"},
	}}
	m := NewWorkspace(&contract.Repo{}, project)
	m.hasClaude = true
	m.tab = tabPlans

	var calls int
	cmds := map[string]string{}
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		calls++
		cmds[root] = in.Command
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	// r opens the accept+spawn confirm listing the 3 un-spawned targets.
	nm, _ := m.Update(runes("r"))
	m = nm.(Model)
	if m.pendingPlanSpawn == nil || m.mode != modeForm {
		t.Fatalf("r did not open the accept+spawn confirm (mode=%d pending=%v)", m.mode, m.pendingPlanSpawn)
	}
	if len(m.pendingPlanSpawn.targets) != 3 {
		t.Fatalf("confirm targets = %v, want the 3 un-spawned sources", m.pendingPlanSpawn.targets)
	}

	// Confirm through the heap-stable binding (TEST-001), then run the fan-out cmd.
	m.binding.confirm = true
	fm, cmd := m.finishPlanSpawn()
	m = fm.(Model)
	if cmd == nil {
		t.Fatal("finishPlanSpawn returned a nil cmd on confirm")
	}
	if _, ok := cmd().(launchDoneMsg); !ok {
		t.Fatal("spawn cmd did not resolve to a launchDoneMsg")
	}

	if calls != 3 {
		t.Fatalf("launcher fired %d times, want exactly 3 (one per un-spawned target)", calls)
	}
	// Each spawn rooted at its source, carrying the plan correlation id + its OWN brief.
	for root, brief := range map[string]string{
		"/r/web":    "Rewire the web token client",
		"/r/api":    "Add the token endpoint",
		"/r/worker": "Rotate tokens on schedule",
	} {
		got := cmds[root]
		if got == "" {
			t.Errorf("no spawn rooted at %s", root)
			continue
		}
		if !strings.Contains(got, brief) {
			t.Errorf("spawn at %s missing its per-source brief %q: %q", root, brief, got)
		}
		if !strings.Contains(got, "--correlation "+p.ID) {
			t.Errorf("spawn at %s missing --correlation %s: %q", root, p.ID, got)
		}
	}
	// A brief is per-source: web's spawn must NOT carry api's brief text.
	if strings.Contains(cmds["/r/web"], "Add the token endpoint") {
		t.Errorf("web spawn leaked api's brief: %q", cmds["/r/web"])
	}
	// The plan-acceptance-skip source (api) carries --skip-acceptance; the plain ones don't.
	if !strings.Contains(cmds["/r/api"], "--skip-acceptance") {
		t.Errorf("api (planAcceptanceSkip) spawn missing --skip-acceptance: %q", cmds["/r/api"])
	}
	if strings.Contains(cmds["/r/web"], "--skip-acceptance") {
		t.Errorf("web (no skip) spawn wrongly carries --skip-acceptance: %q", cmds["/r/web"])
	}
	// 3 members recorded + plan flipped active (store-only writes to ~/.gogo/).
	got, _ := plans.Get("app", p.ID)
	if got.Status != plans.StatusActive || len(got.Members) != 3 {
		t.Errorf("after accept plan = %+v, want active with 3 members", got)
	}
}

// TestPlansTabAcceptTargetlessJustMarksReady pins the additive fallback (FR3): a plan
// with NO analyst-chosen targets keeps today's plain `r` → MarkReady with ZERO launches
// (no confirm, no spawn).
func TestPlansTabAcceptTargetlessJustMarksReady(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Solo idea", "just an idea")

	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.tab = tabPlans
	fired := false
	m.launcher = func(string, launch.Intent) (launch.Result, error) { fired = true; return launch.Result{}, nil }

	nm, _ := m.Update(runes("r"))
	m = nm.(Model)
	if m.pendingPlanSpawn != nil {
		t.Fatalf("targetless r opened a spawn confirm (%v)", m.pendingPlanSpawn)
	}
	if fired {
		t.Error("targetless r fired the launcher (want zero launches)")
	}
	if got, _ := plans.Get("app", p.ID); got.Status != plans.StatusReady {
		t.Errorf("targetless r: status = %q, want ready (plain MarkReady)", got.Status)
	}
}

// TestPlansTabAcceptSkipsAlreadySpawned pins the idempotency (D3=a): a re-`r` on a plan
// whose `web` target was already spawned confirms + fans out ONLY the still-un-spawned
// `api`, never re-launching web.
func TestPlansTabAcceptSkipsAlreadySpawned(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Rollout", "body")
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "api")
	plans.AddMember("app", p.ID, plans.Member{Source: "web", SlugHint: "rollout"}) // web already spawned
	plans.SetStatus("app", p.ID, plans.StatusActive)

	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web"), src("api", "/r/api")))
	m.hasClaude = true
	m.tab = tabPlans

	var calls int
	var roots []string
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		calls++
		roots = append(roots, root)
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	nm, _ := m.Update(runes("r"))
	m = nm.(Model)
	if m.pendingPlanSpawn == nil {
		t.Fatalf("r did not open a confirm for the remaining un-spawned target")
	}
	if len(m.pendingPlanSpawn.targets) != 1 || m.pendingPlanSpawn.targets[0] != "api" {
		t.Fatalf("confirm targets = %v, want only [api] (web already spawned)", m.pendingPlanSpawn.targets)
	}
	m.binding.confirm = true
	fm, cmd := m.finishPlanSpawn()
	m = fm.(Model)
	cmd()
	if calls != 1 || len(roots) != 1 || roots[0] != "/r/api" {
		t.Fatalf("fired %d times %v, want exactly 1 into /r/api (web skipped)", calls, roots)
	}
	if got, _ := plans.Get("app", p.ID); len(got.Members) != 2 {
		t.Errorf("after accept members = %d, want 2 (web + api)", len(got.Members))
	}
}

// TestPlansTabAcceptLaunchErrorRecordsNoMember pins REV-005: a spawn whose launch fails
// records NO member (never a phantom active member the store would over-report).
func TestPlansTabAcceptLaunchErrorRecordsNoMember(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Rollout", "body")
	plans.AddTarget("app", p.ID, "web")

	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.tab = tabPlans
	m.launcher = func(string, launch.Intent) (launch.Result, error) {
		return launch.Result{}, errors.New("boom")
	}

	nm, _ := m.Update(runes("r"))
	m = nm.(Model)
	if m.pendingPlanSpawn == nil {
		t.Fatal("r did not open the accept+spawn confirm")
	}
	m.binding.confirm = true
	fm, cmd := m.finishPlanSpawn()
	m = fm.(Model)
	cmd()
	if got, _ := plans.Get("app", p.ID); len(got.Members) != 0 {
		t.Errorf("a failed launch recorded a phantom member: %+v", got.Members)
	}
}

// TestPlansTabAcceptSpawnFormMessageDriven pins REV-004: the accept+spawn confirm
// completes through a REAL huh form message (not a direct finishPlanSpawn call),
// exercising the shipped updateForm → pendingPlanSpawn → finishPlanSpawn dispatch line
// (and its cancel branch) end-to-end. Confirm (y) fans out one launch per target and
// lands back on the plans tab; a separate cancel (n) fires nothing and leaves the plan
// un-spawned.
func TestPlansTabAcceptSpawnFormMessageDriven(t *testing.T) {
	seedDataHome(t)

	// A fresh project per sub-case, so the plans-list cursor deterministically lands on
	// the one plan under test (an isolated store, not a shared one whose ordering shifts).
	setup := func(project string) (Model, string, *int) {
		p, _ := plans.New(project, "Rollout", "roll it out")
		plans.AddTarget(project, p.ID, "web")
		plans.AddTarget(project, p.ID, "api")
		m := NewWorkspace(&contract.Repo{}, proj(project, src("web", "/r/web"), src("api", "/r/api")))
		m.hasClaude = true
		m.tab = tabPlans
		calls := 0
		m.launcher = func(string, launch.Intent) (launch.Result, error) {
			calls++
			return launch.Result{Mode: "tmux"}, nil
		}
		return m, p.ID, &calls
	}

	// Confirm (y): huh's async completion message routes through updateForm to the fan-out.
	m, id, calls := setup("confirmapp")
	m = send(m, runes("r"))
	if m.pendingPlanSpawn == nil || m.mode != modeForm {
		t.Fatalf("r did not open the accept+spawn confirm (mode=%d pending=%v)", m.mode, m.pendingPlanSpawn)
	}
	m = keyPress(t, m, runes("y")) // affirmative → huh completes → finishPlanSpawn fan-out
	if *calls != 2 {
		t.Fatalf("message-driven confirm fired the launcher %d times, want 2 (one per target)", *calls)
	}
	if m.mode != modeBoard || m.tab != tabPlans {
		t.Errorf("after confirm mode=%d tab=%d, want back on the plans tab", m.mode, m.tab)
	}
	if m.pendingPlanSpawn != nil {
		t.Errorf("pendingPlanSpawn not cleared after completion: %v", m.pendingPlanSpawn)
	}
	if got, _ := plans.Get("confirmapp", id); got.Status != plans.StatusActive || len(got.Members) != 2 {
		t.Errorf("after confirm plan = %+v, want active with 2 members", got)
	}

	// Cancel (n): the negative completion routes to finishPlanSpawn's cancel branch —
	// zero launches, plan left un-spawned, back on the plans tab.
	m2, id2, calls2 := setup("cancelapp")
	m2 = send(m2, runes("r"))
	if m2.pendingPlanSpawn == nil {
		t.Fatalf("r did not open the confirm for the cancel case")
	}
	m2 = keyPress(t, m2, runes("n")) // negative → huh completes → cancel branch
	if *calls2 != 0 {
		t.Errorf("cancel fired the launcher %d times, want 0", *calls2)
	}
	if m2.mode != modeBoard || m2.tab != tabPlans {
		t.Errorf("after cancel mode=%d tab=%d, want back on the plans tab", m2.mode, m2.tab)
	}
	if m2.pendingPlanSpawn != nil {
		t.Errorf("pendingPlanSpawn not cleared after cancel: %v", m2.pendingPlanSpawn)
	}
	if got, _ := plans.Get("cancelapp", id2); len(got.Members) != 0 || got.Status == plans.StatusActive {
		t.Errorf("cancel spawned/activated the plan: %+v", got)
	}
}

// TestDeriveTitle pins REV-005: deriveTitle truncates a long goal on RUNE boundaries, so a
// multibyte first line with no late ASCII space never ships an INVALID-UTF-8 title (the old
// byte-slice sheared a rune → mojibake). Every derived title must stay valid UTF-8 and a sane
// length, while short/ASCII goals are unchanged and the word-safe cut is preserved.
func TestDeriveTitle(t *testing.T) {
	cases := []struct {
		name string
		goal string
		want string // "" when we only assert the invariants (valid UTF-8 + length)
	}{
		{"short ascii passes through", "Add a rate limiter", "Add a rate limiter"},
		{"blank goal falls back", "\n  \n", "Untitled plan"},
		{"first non-blank line wins", "\n\nMigrate the token store\nmore", "Migrate the token store"},
		{"long ascii cuts on a word boundary", "Migrate the shared token store to the brand new auth service layer", "Migrate the shared token store to the brand new…"},
		// A >50-rune space-free multibyte first line (Japanese) — the REV-005 mojibake case.
		{"long space-free multibyte stays valid utf8", "これはとても長い日本語のゴールでスペースがまったくないので途中で切れてしまうかもしれないテストケースです", ""},
		// Multibyte with Polish diacritics past 50 runes.
		{"long polish stays valid utf8", "Zaimplementuj ograniczanie przepustowości żądań na wszystkich źródłach bramki API projektu", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveTitle(tc.goal)
			if !utf8.ValidString(got) {
				t.Fatalf("deriveTitle(%q) = %q — NOT valid UTF-8 (mojibake)", tc.goal, got)
			}
			if n := utf8.RuneCountInString(got); n == 0 || n > 51 { // ≤50 runes + one ellipsis
				t.Errorf("deriveTitle(%q) = %q has %d runes, want 1..51", tc.goal, got, n)
			}
			if tc.want != "" && got != tc.want {
				t.Errorf("deriveTitle(%q) = %q, want %q", tc.goal, got, tc.want)
			}
		})
	}
}

// pumpNoBlink drains cmds through m.Update, feeding back each resulting message, but DROPS any
// command that blocks past a short deadline. A huh TEXT/INPUT field arms a cursor BlinkCmd that
// waits on a timer channel (bubbles/cursor) — the blink-free `drive` helper deadlocks on it,
// so this pump is the `A` goal-form (text-input) equivalent. Dropping a blink never feeds its
// BlinkMsg back, so it does not re-arm; the useful async messages (huh field-advance /
// group-completion, the launch cmd) resolve immediately and drive the form to StateCompleted.
func pumpNoBlink(t *testing.T, m Model, cmds ...tea.Cmd) Model {
	t.Helper()
	queue := append([]tea.Cmd(nil), cmds...)
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 6000 {
			t.Fatalf("no-blink form pump did not settle in 6000 steps (mode=%d)", m.mode)
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }() // a blink cmd blocks here on a timer — we abandon it below
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-time.After(40 * time.Millisecond):
			continue // blocking cursor-blink / tick cmd — not needed to reach completion
		}
		switch tm := msg.(type) {
		case nil:
			continue
		case tea.BatchMsg:
			queue = append(queue, tm...)
		case launchDoneMsg:
			continue // already invoked the (fake) launcher — re-feeding would list tmux
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

// TestPlanWithClaudeSubmitFormMessageDriven pins REV-009 (the REV-004 class for the `A` path):
// the shipped updateForm → pendingPlanWithClaude → finishPlanWithClaude dispatch is exercised
// end-to-end through a REAL huh goal-form completion, NOT a direct finishPlanWithClaude call.
// It TYPES the goal + title into the focused fields and drives the group to completion via huh's
// own field-advance / group messages (huh.NextField/PrevField), so the plan is minted from the
// typed goal, the launcher fires exactly once, and the headless message lands (no-tmux launcher
// so no ExecProcess attach fires during the pump).
func TestPlanWithClaudeSubmitFormMessageDriven(t *testing.T) {
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/repos/web")))
	m.hasClaude = true
	m.tab = tabPlans
	calls := 0
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		calls++
		// No tmux → background: no Session name, so Update surfaces the headless status
		// instead of an ExecProcess attach (which the pump would otherwise try to spawn).
		return launch.Result{Mode: "background", LogPath: "/tmp/author.log", Command: in.Command}, nil
	}

	const goal = "Migrate the shared token store to the new auth service"
	const title = "Token migration"

	// Open the goal form. huh's field-advance FOCUSES the newly selected field, so hop to the
	// title field and back to focus the goal Text field, then TYPE the goal into it.
	m = send(m, runes("A"))
	if !m.pendingPlanWithClaude || m.mode != modeForm {
		t.Fatalf("A did not open the goal form (mode=%d pending=%v)", m.mode, m.pendingPlanWithClaude)
	}
	m = send(m, huh.NextField()) // → title Input focused
	m = send(m, huh.PrevField()) // → goal Text focused
	m = send(m, runes(goal))     // type the goal into the focused textarea
	m = send(m, huh.NextField()) // blur goal (writes the binding) → title Input focused
	m = send(m, runes(title))    // type the title

	// Advance off the LAST field → huh emits its group-completion message; draining it routes
	// through the shipped updateForm StateCompleted dispatch to finishPlanWithClaude.
	nm, cmd := m.Update(huh.NextField())
	m = pumpNoBlink(t, nm.(Model), cmd)

	// The shipped dispatch routed through finishPlanWithClaude: one launch, plan minted.
	if calls != 1 {
		t.Fatalf("message-driven submit fired the launcher %d times, want exactly 1", calls)
	}
	if m.pendingPlanWithClaude {
		t.Errorf("pendingPlanWithClaude not cleared after completion")
	}
	if m.mode != modeBoard || m.tab != tabPlans {
		t.Errorf("after submit mode=%d tab=%d, want back on the plans tab", m.mode, m.tab)
	}
	list, _ := plans.List("app")
	if len(list) != 1 {
		t.Fatalf("message-driven submit did not mint exactly one plan: %+v", list)
	}
	p := list[0]
	if strings.TrimSpace(p.Description) != goal {
		t.Errorf("plan description = %q, want the TYPED goal %q", p.Description, goal)
	}
	if p.Title != title {
		t.Errorf("plan title = %q, want the typed title %q", p.Title, title)
	}
	// The attach/headless message landed (no-tmux launcher → the REV-006 headless status).
	if !strings.Contains(m.status, "headless") {
		t.Errorf("status = %q, want the headless message produced by the shipped dispatch", m.status)
	}
}
