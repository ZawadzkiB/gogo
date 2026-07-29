package tui

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/pages"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
)

// --- 0.28.0: plan v/w viewers, target resolution, status severity, key guard ---

// planWith seeds a project plan in a temp data home and returns a sized plans-tab
// Model focused on the plan's kanban column.
func planWith(t *testing.T, project, title, body string, col int, srcs ...projects.Source) (Model, plans.Plan) {
	t.Helper()
	seedDataHome(t)
	p, err := plans.New(project, title, body)
	if err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	m := sizedWorkspace(t, &contract.Repo{}, proj(project, srcs...))
	m = tab(m) // → plans
	m.planColIdx = col
	return m, p
}

// TestPlansTabQuickView (FR2.1): `v` on a focused plan enters the terminal viewer
// with the PLAN's markdown path as the current artifact - the same glamour article
// renderer the work board's `v` uses, no new renderer.
func TestPlansTabQuickView(t *testing.T) {
	m, p := planWith(t, "app", "Wire up auth", "seed the auth flow", 0, src("web", "/r/web"))

	m = send(m, runes("v"))
	if m.mode != modeViewer {
		t.Fatalf("v did not enter the viewer (mode=%d)", m.mode)
	}
	if !m.planViewing {
		t.Error("v did not mark planViewing - esc would land on modeDrill and nil-deref m.drill")
	}
	want := plans.Path("app", p.ID)
	if m.curArtifact.Path != want {
		t.Errorf("curArtifact.Path = %q, want the plan file %q", m.curArtifact.Path, want)
	}
	if m.curArtifact.Kind != contract.KindMarkdown {
		t.Errorf("curArtifact.Kind = %q, want markdown", m.curArtifact.Kind)
	}
	// The plan detail's `v` opens the SAME artifact (both switches are wired).
	m2, p2 := planWith(t, "app", "Detail view", "body", 0, src("web", "/r/web"))
	m2 = send(m2, tea.KeyMsg{Type: tea.KeyEnter}) // open the detail
	if m2.planDetail == nil {
		t.Fatal("enter did not open the plan detail")
	}
	m2 = send(m2, runes("v"))
	if m2.mode != modeViewer || m2.curArtifact.Path != plans.Path("app", p2.ID) {
		t.Errorf("detail v = mode %d, artifact %q", m2.mode, m2.curArtifact.Path)
	}
}

// TestPlansTabViewEscReturnsToPlansTab pins FR2.2 - the regression this guards is a
// PANIC: updateViewer's esc used to set mode = modeDrill unconditionally, and
// viewDrill dereferences m.drill with no nil guard (view.go), so a plan `v` + esc
// would crash the cockpit. esc must land on the plans tab, and rendering after it
// must not panic.
func TestPlansTabViewEscReturnsToPlansTab(t *testing.T) {
	m, _ := planWith(t, "app", "Escape hatch", "body", 0, src("web", "/r/web"))

	m = send(m, runes("v"))
	if m.mode != modeViewer {
		t.Fatalf("v did not enter the viewer")
	}
	if m.drill != nil {
		t.Fatal("fixture invalid: a plan view must have NO drilled card (that is the whole hazard)")
	}

	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeBoard || m.tab != tabPlans {
		t.Errorf("esc landed on mode=%d tab=%d, want the plans tab (modeBoard/tabPlans)", m.mode, m.tab)
	}
	if m.planViewing {
		t.Error("esc left planViewing set - a later board viewer would return to the wrong place")
	}
	// The render after the return must not panic (the actual crash surface).
	if out := m.View(); !strings.Contains(out, "Escape hatch") {
		t.Errorf("after esc the plans tab did not render the plan:\n%s", out)
	}

	// Defence in depth (D3): even a direct modeDrill render with no drilled card is
	// now survivable rather than a nil-deref.
	bare := m
	bare.mode = modeDrill
	bare.drill = nil
	if out := bare.View(); !strings.Contains(out, "no card selected") {
		t.Errorf("viewDrill with a nil drill = %q, want the graceful panel", out)
	}
}

// TestPlansTabWebPageWritesUnderGogoHome pins FR2.3/FR2.4 + D2=A: `w` writes the
// plan's page under the PROJECT HOME (~/.gogo/projects/<name>/), and touches NO
// source repo - asserted against a real temp source dir that must stay empty.
func TestPlansTabWebPageWritesUnderGogoHome(t *testing.T) {
	home := seedDataHome(t)
	sourceRoot := t.TempDir() // a stand-in source repo that must NOT be written
	p, err := plans.New("app", "Catalogue side", "## Goal\nnormalise, store, embed.")
	if err != nil {
		t.Fatal(err)
	}
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", sourceRoot)))
	m = tab(m)

	m2, cmd := m.Update(runes("w"))
	m = m2.(Model)
	if cmd == nil {
		t.Fatal("w returned a nil cmd - no page build was scheduled")
	}
	msg, ok := cmd().(launchDoneMsg)
	if !ok {
		t.Fatalf("w cmd resolved to %T, want a launchDoneMsg", cmd())
	}
	if !strings.HasPrefix(msg.status, "page: ") {
		t.Fatalf("page build did not succeed: %q", msg.status)
	}
	page := strings.TrimPrefix(msg.status, "page: ")

	wantDir := filepath.Join(projects.Dir("app"), ".gogo", "resources", "view")
	if filepath.Dir(page) != wantDir {
		t.Errorf("page written to %q, want it under the project home %q", page, wantDir)
	}
	if filepath.Base(page) != p.ID+".html" {
		t.Errorf("page basename = %q, want %s.html", filepath.Base(page), p.ID)
	}
	if !strings.HasPrefix(page, home) {
		t.Errorf("page %q escaped the gogo data home %q", page, home)
	}
	if _, err := os.Stat(page); err != nil {
		t.Errorf("page was not actually written: %v", err)
	}
	// The hard invariant: the source repo is untouched.
	if _, err := os.Stat(filepath.Join(sourceRoot, ".gogo")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the source repo at %s was written to (stat err = %v) - the CLI must only write ~/.gogo/", sourceRoot, err)
	}
}

// TestPlanBundleDegradesWithoutCharts pins the seam reuse behind FR2.3: a project
// plan is ONE markdown file with no charts/, and pages.BuildHTML must render its
// summary anyway (empty DiagramDir/BeforeDir/ManifestPath) rather than erroring -
// which is why `w` needs no new page builder.
func TestPlanBundleDegradesWithoutCharts(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Chartless plan", "## Goal\nA distinctive summary sentence.")

	b := planBundleFor("app", p)
	if b.DiagramDir != "" || b.BeforeDir != "" || b.ManifestPath != "" {
		t.Errorf("plan bundle = %+v, want no diagram inputs (a plan has no charts/)", b)
	}
	if b.MarkdownPath != plans.Path("app", p.ID) {
		t.Errorf("bundle markdown = %q, want the plan file", b.MarkdownPath)
	}
	html, err := pages.BuildHTML(b)
	if err != nil {
		t.Fatalf("BuildHTML on a chart-less plan returned an error: %v", err)
	}
	if !strings.Contains(html, "A distinctive summary sentence") {
		t.Error("the built page does not render the plan's summary")
	}
}

// TestPlansTabUnknownTargetRefusedBeforeConfirm pins FR3.1 against the user's live
// shape (project `dotai`, plan `targets: gogo` where gogo is another project's
// source): NO confirm opens, the status names the target AND the project, and the
// launcher is never called. Before, this opened a confirm promising a spawn and
// then silently dropped it.
func TestPlansTabUnknownTargetRefusedBeforeConfirm(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("dotai", "Catalogue ingestion", "brief")
	plans.AddTarget("dotai", p.ID, "gogo") // a source of a DIFFERENT project
	plans.MarkReady("dotai", p.ID)

	m := NewWorkspace(&contract.Repo{}, proj("dotai", src("dotai-app", "/r/dotai")))
	m.hasClaude = true
	m.tab = tabPlans
	m.planColIdx = 1 // the ready column
	fired := 0
	m.launcher = func(string, launch.Intent) (launch.Result, error) {
		fired++
		return launch.Result{}, nil
	}

	nm, _ := m.Update(runes("m"))
	m = nm.(Model)

	if m.mode == modeForm || m.pendingPlanSpawn != nil {
		t.Fatalf("a confirm opened for an unspawnable plan (mode=%d pending=%v)", m.mode, m.pendingPlanSpawn)
	}
	if fired != 0 {
		t.Errorf("launcher fired %d times, want 0", fired)
	}
	for _, want := range []string{"gogo", "dotai", "config tab"} {
		if !strings.Contains(m.status, want) {
			t.Errorf("status %q does not name %q", m.status, want)
		}
	}
	if m.statusLevel != statusLevelWarn {
		t.Errorf("statusLevel = %v, want warn (blocked, not failed)", m.statusLevel)
	}
	// The refusal is VISIBLE (a render assertion, not just a model field).
	m.width, m.height = 200, 40
	if out := m.View(); !strings.Contains(out, statusWarnMarker) || !strings.Contains(out, "not a source of project dotai") {
		t.Errorf("the refusal is not rendered on the plans tab:\n%s", out)
	}
	// The plan is untouched - no phantom member, still ready.
	if got, _ := plans.Get("dotai", p.ID); got.Status != plans.StatusReady || len(got.Members) != 0 {
		t.Errorf("plan changed on a refused go: %+v", got)
	}
}

// TestPlansTabConfirmListsOnlySpawnableTargets (FR3.1): a MIXED plan confirms only
// the resolvable targets, names the skipped one, and fires once per spawnable.
func TestPlansTabConfirmListsOnlySpawnableTargets(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Mixed rollout", "the brief")
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "ghost") // not a source of this project
	plans.MarkReady("app", p.ID)

	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.tab = tabPlans
	m.planColIdx = 1
	var roots []string
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		roots = append(roots, root)
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	nm, _ := m.Update(runes("m"))
	m = nm.(Model)
	if m.pendingPlanSpawn == nil {
		t.Fatal("m did not open a confirm for the spawnable half")
	}
	if got := m.pendingPlanSpawn.targets; len(got) != 1 || got[0] != "web" {
		t.Fatalf("confirm targets = %v, want only the spawnable [web]", got)
	}
	if out := m.View(); !strings.Contains(out, "skipping ghost") {
		t.Errorf("the confirm does not name the skipped target:\n%s", out)
	}

	m.binding.confirm = true
	fm, cmd := m.finishPlanSpawn()
	m = fm.(Model)
	if cmd == nil {
		t.Fatal("nil cmd on confirm")
	}
	cmd()
	if len(roots) != 1 || roots[0] != "/r/web" {
		t.Errorf("launched at %v, want exactly one launch at /r/web", roots)
	}
}

// TestSpawnOversizedBriefFoldsToPointer pins FR1.3 end-to-end through the TUI: a
// >16 KB per-source brief produces a launched command UNDER the tmux limit that
// carries the plan's absolute path + the source section instead of the body. This
// is the user's actual failure (a 20 KB plan built a 20 128-byte command line).
func TestSpawnOversizedBriefFoldsToPointer(t *testing.T) {
	t.Setenv(launch.PermissionModeEnv, "auto")
	seedDataHome(t)
	huge := strings.Repeat("normalise, store, embed, hard-filter the catalogue. ", 400) // ~20 KB
	body := "## Goal\nBig one.\n\n## Source briefs\n### web\n" + huge
	p, _ := plans.New("app", "Catalogue side of the matching engine", body)
	plans.AddTarget("app", p.ID, "web")
	plans.MarkReady("app", p.ID)

	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.tab = tabPlans
	m.planColIdx = 1
	var got launch.Intent
	var gotRoot string
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		gotRoot, got = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	nm, _ := m.Update(runes("m"))
	m = nm.(Model)
	if m.pendingPlanSpawn == nil {
		t.Fatal("m did not open the spawn confirm")
	}
	m.binding.confirm = true
	_, cmd := m.finishPlanSpawn()
	if cmd == nil {
		t.Fatal("nil spawn cmd")
	}
	cmd()

	argv := launch.TmuxNewSessionArgs(gotRoot, got)
	if n := launch.TmuxCommandBytes(argv); n > launch.MaxTmuxCommandBytes {
		t.Errorf("launched command is %d bytes - still over tmux's %d-byte limit", n, launch.MaxTmuxCommandBytes)
	}
	if strings.Contains(got.Command, huge) {
		t.Error("the oversized brief was still inlined into the command")
	}
	for _, want := range []string{plans.Path("app", p.ID), "### web", "--correlation " + p.ID} {
		if !strings.Contains(got.Command, want) {
			t.Errorf("folded command missing %q:\n%s", want, got.Command)
		}
	}
}

// TestPlanSlugHintMatchesSessionTransform pins the second half of FR1.7 (REV-005):
// planSlugHint must run the SAME transform the session name does. The old
// `[^a-z0-9]+` regex collapsed an existing dash, so a title's `" - "` became `-`
// in the hint but `---` in the session name and SessionMatchesSlug returned false -
// which is precisely how a plan-spawned work item lost its dot, its attach and its
// peek. Reverting planSlugHint to the old regex must fail this test.
func TestPlanSlugHintMatchesSessionTransform(t *testing.T) {
	for _, title := range []string{
		"Catalogue side - normalise",                                      // the `" - "` collapse
		"Catalogue side of the matching engine - normalise, store, embed", // + the 48-cap
		"Refactor NotificationDeliveryOrchestrationPipelineForRealtimeEvents",
	} {
		hint := planSlugHint(title)
		if want := launch.SlugFromLabel(title); hint != want {
			t.Errorf("planSlugHint(%q) = %q, but the session transform yields %q - the hint cannot attribute to its session", title, hint, want)
		}
		// The property that drift breaks: the session a spawn mints for this title
		// must attribute back to the hint the spawn records.
		session := "gogo-plan-" + launch.SlugFromLabel(title)
		if !launch.SessionMatchesSlug(session, hint) {
			t.Errorf("session %q does not attribute to the recorded SlugHint %q", session, hint)
		}
	}
	// A blank title keeps the plans-tab "plan" fallback.
	if got := planSlugHint("   "); got != "plan" {
		t.Errorf("planSlugHint(blank) = %q, want plan", got)
	}
}

// TestCreateWorkItemFoldsOversizedBrief pins the fold at the `c` create-work-item
// site (REV-005): deleting the FoldToPointer call there must fail this test.
func TestCreateWorkItemFoldsOversizedBrief(t *testing.T) {
	t.Setenv(launch.PermissionModeEnv, "auto")
	seedDataHome(t)
	huge := strings.Repeat("normalise, store, embed, hard-filter the catalogue. ", 400) // ~20 KB
	p, _ := plans.New("app", "Catalogue side of the matching engine", huge)
	plans.AddTarget("app", p.ID, "web")

	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.tab = tabPlans
	detail, _ := plans.Get("app", p.ID)
	m.planDetail = &detail
	m.planSourceIdx = 0
	var got launch.Intent
	var gotRoot string
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		gotRoot, got = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	nm, cmd := m.Update(runes("c"))
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("`c` scheduled no launch")
	}
	cmd()

	if n := launch.TmuxCommandBytes(launch.TmuxNewSessionArgs(gotRoot, got)); n > launch.MaxTmuxCommandBytes {
		t.Errorf("`c` built a %d-byte command line - over tmux's %d-byte limit; the fold is not wired at this site",
			n, launch.MaxTmuxCommandBytes)
	}
	if strings.Contains(got.Command, huge) {
		t.Error("`c` still inlines the whole oversized brief")
	}
	if !strings.Contains(got.Command, plans.Path("app", p.ID)) {
		t.Errorf("`c` folded command does not point at the plan file:\n%s", got.Command)
	}
}

// TestPlanWithClaudeFoldsOversizedGoal pins the fold at the `A` authoring site
// (REV-005): deleting the FoldToPointer call there must fail this test. The goal is
// already the plan file's body, so the fold is a whole-file pointer.
func TestPlanWithClaudeFoldsOversizedGoal(t *testing.T) {
	t.Setenv(launch.PermissionModeEnv, "auto")
	seedDataHome(t)
	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.tab = tabPlans
	huge := strings.Repeat("paste of a multi-KB product spec. ", 700) // ~24 KB
	m.binding = &formBinding{planGoal: huge, planTitle: "Big pasted spec"}
	m.pendingPlanWithClaude = true
	var got launch.Intent
	var gotRoot string
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		gotRoot, got = root, in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}

	nm, cmd := m.finishPlanWithClaude()
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("finishPlanWithClaude scheduled no launch")
	}
	cmd()

	list, _ := plans.List("app")
	if len(list) != 1 {
		t.Fatalf("expected exactly one minted plan, got %d", len(list))
	}
	planPath := plans.Path("app", list[0].ID)

	if n := launch.TmuxCommandBytes(launch.TmuxNewSessionArgs(gotRoot, got)); n > launch.MaxTmuxCommandBytes {
		t.Errorf("`A` built a %d-byte command line - over tmux's %d-byte limit; the fold is not wired at this site",
			n, launch.MaxTmuxCommandBytes)
	}
	if strings.Contains(got.Command, huge) {
		t.Error("`A` still inlines the whole oversized goal")
	}
	if !strings.Contains(got.Command, planPath) {
		t.Errorf("`A` folded command does not point at the plan file:\n%s", got.Command)
	}
	// The skill instruction must survive the fold - only the goal is excised.
	if !strings.Contains(got.Command, "Load and follow the gogo-project-plan skill") {
		t.Error("the fold destroyed the author prompt's skill instruction")
	}
	// The goal is not lost: it is the plan file's body.
	if !strings.Contains(list[0].Description, "paste of a multi-KB product spec.") {
		t.Error("the goal was not preserved in the plan file the pointer points at")
	}
}

// TestSpawnUnderBudgetIsByteForByte is the other half of D1=A's contract: a normal
// brief must launch EXACTLY as it did before the fold existed.
func TestSpawnUnderBudgetIsByteForByte(t *testing.T) {
	t.Setenv(launch.PermissionModeEnv, "auto")
	seedDataHome(t)
	p, _ := plans.New("app", "Token migration", "move the shared token store")
	plans.AddTarget("app", p.ID, "web")
	plans.MarkReady("app", p.ID)

	m := NewWorkspace(&contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasClaude = true
	m.tab = tabPlans
	m.planColIdx = 1
	var got launch.Intent
	m.launcher = func(root string, in launch.Intent) (launch.Result, error) {
		got = in
		return launch.Result{Mode: "tmux", Session: in.Session, Command: in.Command}, nil
	}
	nm, _ := m.Update(runes("m"))
	m = nm.(Model)
	m.binding.confirm = true
	_, cmd := m.finishPlanSpawn()
	cmd()

	want := "/gogo:plan move the shared token store --correlation " + p.ID
	if got.Command != want {
		t.Errorf("under-budget command = %q, want the pre-fold command %q", got.Command, want)
	}
}

// TestStatusSeverityDistinguishesOutcomes pins FR3.2 as a RENDER assertion (a
// model-level status check is not a render check - the 0.16.0 lesson): a cap
// bounce, a launch failure and a success must each render through a DIFFERENT
// voice, and the distinction must survive a colourless terminal (which `go test`
// always is), hence the markers.
func TestStatusSeverityDistinguishesOutcomes(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "busy", Title: "Busy", Source: "web", Root: "/r/web", Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
		{Slug: "next", Title: "Next", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	base := sizedWorkspace(t, repo, proj("app", src("web", "/r/web", 1)))
	base.hasClaude = true
	base.sessions = []string{"gogo-go-busy"} // makes `busy` count against the cap

	// 1. BLOCKED - the cap bounce on the un-started card.
	blocked := base
	blocked.colIdx, blocked.cardIdx[0] = 0, 0
	if f := blocked.focusedCard(); f == nil || f.Slug != "next" {
		t.Fatalf("fixture: focused card = %v, want next", blocked.focusedCard())
	}
	blocked = send(blocked, runes("m"))
	blockedOut := blocked.View()
	if blocked.statusLevel != statusLevelWarn {
		t.Errorf("cap bounce level = %v, want warn", blocked.statusLevel)
	}
	if !strings.Contains(blockedOut, statusWarnMarker) {
		t.Errorf("cap bounce does not render the blocked marker:\n%s", blockedOut)
	}
	if strings.Contains(blockedOut, statusErrMarker) {
		t.Error("cap bounce rendered as a FAILURE")
	}

	// 2. FAILED - a launcher error carrying tmux's own words.
	failed := base
	failed.setStatus(statusLevelErr, "launch failed: tmux new-session failed: exit status 1: command too long")
	failedOut := failed.View()
	if !strings.Contains(failedOut, statusErrMarker) || !strings.Contains(failedOut, "command too long") {
		t.Errorf("failure does not render the error marker + tmux's words:\n%s", failedOut)
	}
	if strings.Contains(failedOut, statusWarnMarker) {
		t.Error("a failure rendered as merely BLOCKED")
	}

	// 3. OK - success keeps the plain dim voice (byte-for-byte with today).
	okm := base
	okm.setStatus(statusLevelOK, "launched /gogo:go next → tmux gogo-go-next")
	okOut := okm.View()
	if strings.Contains(okOut, statusErrMarker) || strings.Contains(okOut, statusWarnMarker) {
		t.Errorf("a success carries a severity marker:\n%s", okOut)
	}
	if !strings.Contains(okOut, "launched /gogo:go next") {
		t.Errorf("the success status is not rendered:\n%s", okOut)
	}

	// The three renderings genuinely differ.
	if blockedOut == failedOut || failedOut == okOut || blockedOut == okOut {
		t.Error("two outcomes render identically - the status line still has one voice")
	}
}

// TestProjectUATRefusalIsBlocked pins REV-006: the project-UAT refusal has TWO arms
// (no members yet / some members unshipped) and they must read the same way. Only
// the first was classified, so the identical refusal rendered amber or dim
// depending on which arm fired.
func TestProjectUATRefusalIsBlocked(t *testing.T) {
	seedDataHome(t)
	// An ACTIVE plan with two members, only one of them shipped -> the second arm.
	p, _ := plans.New("app", "Cross-repo migration", "body")
	plans.AddTarget("app", p.ID, "web")
	plans.AddTarget("app", p.ID, "api")
	plans.AddMember("app", p.ID, plans.Member{Source: "web", SlugHint: "web-item"})
	plans.AddMember("app", p.ID, plans.Member{Source: "api", SlugHint: "api-item"})
	plans.SetStatus("app", p.ID, plans.StatusActive)

	// Correlations is load-bearing: memberFeature matches a member by (Source, plan
	// id). Without it NEITHER member is found, so the refusal fires because both are
	// missing rather than because one is unshipped - a weaker reason than intended.
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "web-item", Source: "web", Root: "/r/web", Class: contract.ClassShipped, Status: "shipped", Correlations: []string{p.ID}},
		{Slug: "api-item", Source: "api", Root: "/r/api", Class: contract.ClassInProgress, Status: "implementing", Correlations: []string{p.ID}},
	}}
	m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web"), src("api", "/r/api")))
	m = tab(m)
	m.planColIdx = 2 // the active column
	m = send(m, runes("m"))

	if !strings.Contains(m.status, "1 of 2 member(s) not shipped") {
		t.Fatalf("status = %q, want exactly the 1-of-2 refusal (a different tally means the fixture's members stopped resolving)", m.status)
	}
	if m.statusLevel != statusLevelWarn {
		t.Errorf("the project-UAT refusal is level %v, want warn - its sibling arm is already blocked", m.statusLevel)
	}
	if out := m.View(); !strings.Contains(out, statusWarnMarker) {
		t.Errorf("the refusal does not render the blocked marker:\n%s", out)
	}

	// The OTHER arm (no members at all) must agree. It MUST be focused explicitly:
	// the shared data home now holds both plans, so trusting planCardIdx[2] = 0 landed
	// back on the 2-member plan above and silently re-tested the same arm (REV-011 -
	// the REV-003 shape recurring). Delete the first plan so the arm is unambiguous.
	if _, err := plans.Delete("app", p.ID); err != nil {
		t.Fatalf("clear the first plan: %v", err)
	}
	q, _ := plans.New("app", "Empty plan", "body")
	plans.SetStatus("app", q.ID, plans.StatusActive)
	m2 := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	m2 = tab(m2)
	m2.planColIdx = 2
	if got := m2.planCols[2]; len(got) != 1 || got[0].ID != q.ID {
		t.Fatalf("fixture: the ACTIVE column holds %d plan(s), want only the 0-member %s", len(got), q.ID)
	}
	m2 = send(m2, runes("m"))
	if !strings.Contains(m2.status, "has no work items yet") {
		t.Fatalf("status = %q, want the NO-MEMBERS arm (the other arm was re-tested)", m2.status)
	}
	if m2.statusLevel != statusLevelWarn {
		t.Errorf("the no-members refusal is level %v, want warn (the two arms must agree)", m2.statusLevel)
	}
}

// TestFinishPlanDoneRefusalIsBlocked guards REV-006's third site: finishPlanDone's
// defensive re-guard (the board can move between opening the confirm and accepting
// it) must classify its refusal like every other refusal. Untested before REV-011.
func TestFinishPlanDoneRefusalIsBlocked(t *testing.T) {
	seedDataHome(t)
	p, _ := plans.New("app", "Cross-repo migration", "body")
	plans.AddTarget("app", p.ID, "web")
	plans.AddMember("app", p.ID, plans.Member{Source: "web", SlugHint: "web-item"})
	plans.SetStatus("app", p.ID, plans.StatusActive)

	// The member IS resolvable (Correlations) but NOT shipped, so the re-guard must
	// refuse for the right reason - not because the member could not be found.
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "web-item", Source: "web", Root: "/r/web", Class: contract.ClassInProgress, Status: "implementing", Correlations: []string{p.ID}},
	}}
	m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web")))
	m.tab = tabPlans
	m.pendingPlanDone = &planDoneEdit{project: "app", id: p.ID, title: p.Title}
	m.binding = &formBinding{confirm: true}

	nm, _ := m.finishPlanDone()
	got := nm.(Model)
	if !strings.Contains(got.status, "not shipped") {
		t.Fatalf("status = %q, want the re-guard refusal", got.status)
	}
	if got.statusLevel != statusLevelWarn {
		t.Errorf("finishPlanDone's refusal is level %v, want warn", got.statusLevel)
	}
	if out := got.View(); !strings.Contains(out, statusWarnMarker) {
		t.Errorf("the re-guard refusal does not render as blocked:\n%s", out)
	}
	// And it refused for real - the plan is untouched.
	if after, _ := plans.Get("app", p.ID); after.Status != plans.StatusActive {
		t.Errorf("the plan was flipped despite the refusal: %+v", after)
	}
}

// TestHeadlessAnalystIsBlocked guards REV-006's second site: the `A` analyst falling
// back to a backgrounded `claude -p` is a DEGRADED outcome (the session the user
// asked to steer is running unattended), so it must not read as a success.
func TestHeadlessAnalystIsBlocked(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	m.tab = tabPlans

	nm, _ := m.Update(planAuthorLaunchedMsg{session: "", logPath: "/tmp/author.log", homeNote: "no source to anchor at"})
	got := nm.(Model)
	for _, want := range []string{"no tmux", "/tmp/author.log", "no source to anchor at"} {
		if !strings.Contains(got.status, want) {
			t.Errorf("headless status %q missing %q", got.status, want)
		}
	}
	if got.statusLevel != statusLevelWarn {
		t.Errorf("the headless-analyst fallback is level %v, want warn - it is a degradation the user should notice", got.statusLevel)
	}
	if out := got.View(); !strings.Contains(out, statusWarnMarker) {
		t.Errorf("the headless fallback does not render as blocked:\n%s", out)
	}
}

// TestAttachingCueStaysOK guards REV-006's fifth site: `attaching <session>` is a
// neutral cue set OUTSIDE the keypress reset, so it must set its level explicitly
// rather than inherit whatever a previous async message left behind.
func TestAttachingCueStaysOK(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasTmux = true
	m.statusLevel = statusLevelErr // a stale level from an earlier async failure

	nm, _ := m.attachSession("gogo-go-x")
	got := nm.(Model)
	if got.statusLevel != statusLevelOK {
		t.Errorf("the attaching cue inherited level %v - it must set its own", got.statusLevel)
	}
	if out := got.View(); strings.Contains(out, statusErrMarker) {
		t.Errorf("the attaching cue rendered as a failure:\n%s", out)
	}
}

// TestPartialKillFailureIsReported pins REV-006's fourth site: a kill that partly
// failed must render as a FAILURE and carry the killer's own words (tmux's, now
// that KillSession returns a TmuxError) instead of a bare count in the dim voice.
func TestPartialKillFailureIsReported(t *testing.T) {
	f := &contract.Feature{Slug: "busy", Source: "web", Root: "/r/web", Class: contract.ClassInProgress, Status: "implementing"}
	m := sizedWorkspace(t, &contract.Repo{Features: []*contract.Feature{f}}, proj("app", src("web", "/r/web")))
	m.hasTmux = true
	m.drill = f
	m.pendingKill = []string{"gogo-go-busy", "gogo-go-busy-2"}
	m.binding = &formBinding{confirm: true}
	m.killer = func(session string) error {
		if session == "gogo-go-busy-2" {
			return errors.New("tmux kill-session failed: exit status 1: can't find session")
		}
		return nil
	}

	nm, _ := m.finishKill()
	got := nm.(Model)
	if got.statusLevel != statusLevelErr {
		t.Errorf("a partial kill failure is level %v, want err", got.statusLevel)
	}
	for _, want := range []string{"killed 1 session", "1 failed", "can't find session"} {
		if !strings.Contains(got.status, want) {
			t.Errorf("status %q missing %q - the real error's words are discarded", got.status, want)
		}
	}
	if out := got.View(); !strings.Contains(out, statusErrMarker) {
		t.Errorf("the partial kill failure does not render as a failure:\n%s", out)
	}

	// A CLEAN kill still reads as the dim success voice (byte-for-byte with today).
	m.killer = func(string) error { return nil }
	cm, _ := m.finishKill()
	clean := cm.(Model)
	if clean.statusLevel != statusLevelOK || clean.status != "killed 2 sessions" {
		t.Errorf("a clean kill = %q at level %v, want the plain dim success line", clean.status, clean.statusLevel)
	}
}

// TestStatusSeverityResetsPerKeypress: a stale severity can never re-colour a later,
// unrelated message (the reason the reset lives at Update's key choke point).
func TestStatusSeverityResetsPerKeypress(t *testing.T) {
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	m.setStatus(statusLevelErr, "something exploded")
	m = send(m, runes("/")) // any unrelated key
	if m.statusLevel != statusLevelOK {
		t.Errorf("statusLevel = %v after an unrelated keypress, want it reset to OK", m.statusLevel)
	}
	if strings.Contains(m.View(), statusErrMarker) {
		t.Error("the filter hint inherited the previous error's red")
	}
}

// TestForceMoveOverridesCap pins FR3.3: `M` on a cap-blocked card reaches the
// confirm (which SAYS what it is overriding), while `m` still bounces.
func TestForceMoveOverridesCap(t *testing.T) {
	repo := &contract.Repo{Features: []*contract.Feature{
		{Slug: "busy", Title: "Busy", Source: "web", Root: "/r/web", Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
		{Slug: "next", Title: "Next", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "plan-accepted"},
	}}
	base := sizedWorkspace(t, repo, proj("app", src("web", "/r/web", 1)))
	base.hasClaude = true
	base.sessions = []string{"gogo-go-busy"}

	// `m` bounces (the cap guard still holds).
	bounced := send(base, runes("m"))
	if bounced.mode == modeForm {
		t.Fatal("m opened a confirm on a cap-blocked card - the guard is gone")
	}
	if !strings.Contains(bounced.status, "cap 1 reached") {
		t.Fatalf("m status = %q, want the cap bounce", bounced.status)
	}

	// `M` reaches the confirm and names the override.
	forced := send(base, runes("M"))
	if forced.mode != modeForm || forced.form == nil {
		t.Fatalf("M did not open the launch confirm (mode=%d)", forced.mode)
	}
	out := forced.View()
	for _, want := range []string{"FORCING past the source cap", "cap 1 reached", "busy", "/gogo:go next"} {
		if !strings.Contains(out, want) {
			t.Errorf("the force confirm does not say %q:\n%s", want, out)
		}
	}

	// `M` on an UNCAPPED card is just `m` - no override wording, still a confirm.
	free := sizedWorkspace(t, repo, proj("app", src("web", "/r/web")))
	free.hasClaude = true
	free = send(free, runes("M"))
	if free.mode != modeForm {
		t.Fatalf("M on an uncapped card did not open the confirm (mode=%d)", free.mode)
	}
	if strings.Contains(free.View(), "FORCING past the source cap") {
		t.Error("M claimed to force past a cap that was never hit")
	}
}

// TestForceMoveClaimsOverrideOnlyWhenItOverrode covers REV-007 AND REV-010 as one
// table: `M` may say "FORCING past the source cap" ONLY on an arm that actually
// consults the cap. attemptAction answers the SELECTION branch (a merged
// /gogo:done) and the plan-acceptance branch (an uncapped /gogo:accept) before any
// capBounce is reached, so both used to get the false claim. A confirm is the
// safety surface for a state-changing launch, so wrong text there is worse than
// cosmetic.
func TestForceMoveClaimsOverrideOnlyWhenItOverrode(t *testing.T) {
	// One cap-1 source with a live build, so every arm below is evaluated against a
	// genuinely exhausted cap - the only variable is which arm answers.
	base := func(t *testing.T) Model {
		repo := &contract.Repo{Features: []*contract.Feature{
			{Slug: "busy", Title: "Busy", Source: "web", Root: "/r/web", Class: contract.ClassInProgress, Phase: "implement", Status: "implementing"},
			{Slug: "next", Title: "Next", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "plan-accepted"},
			{Slug: "pending", Title: "Pending", Source: "web", Root: "/r/web", Class: contract.ClassUnfinished, Status: "awaiting-plan-acceptance"},
			{Slug: "shipme", Title: "Ship me", Source: "web", Root: "/r/web", Class: contract.ClassReadyToShip, Status: "awaiting-uat"},
		}}
		m := sizedWorkspace(t, repo, proj("app", src("web", "/r/web", 1)))
		m.hasClaude = true
		m.sessions = []string{"gogo-go-busy"} // occupies the cap-1 source
		return m
	}
	// focusSlug points the cursor at a slug within the plan column (col 0 holds every
	// unfinished card; ready lives in col 2).
	focusSlug := func(t *testing.T, m Model, col int, slug string) Model {
		t.Helper()
		m.colIdx = col
		for i, f := range m.cols[col] {
			if f.Slug == slug {
				m.cardIdx[col] = i
				return m
			}
		}
		t.Fatalf("fixture: %q not in column %d", slug, col)
		return m
	}

	cases := []struct {
		name         string
		setup        func(*testing.T, Model) Model
		wantCommand  string
		wantOverride bool
	}{
		{
			// The arm that DOES consult the cap - the whole point of `M`.
			name:         "go on a cap-blocked card",
			setup:        func(t *testing.T, m Model) Model { return focusSlug(t, m, 0, "next") },
			wantCommand:  "/gogo:go next",
			wantOverride: true,
		},
		{
			// REV-010: the accept arm returns BEFORE capBounce ("Accept is uncapped").
			name:         "accept on a plan-pending card",
			setup:        func(t *testing.T, m Model) Model { return focusSlug(t, m, 0, "pending") },
			wantCommand:  "/gogo:accept pending",
			wantOverride: false,
		},
		{
			// REV-007: the selection branch answers before any arm is reached. Cursor
			// deliberately left on the cap-blocked card - the reviewer's exact shape.
			name: "ready selection, cursor on a cap-blocked card",
			setup: func(t *testing.T, m Model) Model {
				m = focusSlug(t, m, 0, "next")
				m.selected = map[string]bool{selKey(m, "shipme"): true}
				return m
			},
			wantCommand:  "/gogo:done shipme",
			wantOverride: false,
		},
		{
			// A ready card under the cursor ships; the ready arm never consults the cap.
			name:         "ready card focused, no selection",
			setup:        func(t *testing.T, m Model) Model { return focusSlug(t, m, 2, "shipme") },
			wantCommand:  "/gogo:done shipme",
			wantOverride: false,
		},
	}

	const forcing = "FORCING past the source cap"
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := c.setup(t, base(t))
			m = send(m, runes("M"))
			if m.mode != modeForm {
				t.Fatalf("M did not open a confirm (mode=%d, status=%q)", m.mode, m.status)
			}
			out := m.View()
			if !strings.Contains(out, c.wantCommand) {
				t.Fatalf("confirm does not name %q:\n%s", c.wantCommand, out)
			}
			switch got := strings.Contains(out, forcing); {
			case c.wantOverride && !got:
				t.Errorf("a genuine cap override does not say so:\n%s", out)
			case !c.wantOverride && got:
				t.Errorf("the confirm claims to force past a cap this arm never consulted:\n%s", out)
			}
			// When it DOES claim an override, it must carry the real bounce's detail.
			if c.wantOverride && !strings.Contains(out, "cap 1 reached") {
				t.Errorf("the override note does not name the cap it overrode:\n%s", out)
			}
		})
	}
}

// TestAttachFailureIsReported pins FR1.6: tea.ExecProcess's err used to be
// discarded, so a failed attach reported "detached from X" - indistinguishable
// from a clean detach.
//
// It drives the PRODUCTION decision (tui.attachOutcome), not a copy of it: the
// previous version of this test asserted a test-local re-implementation, so
// deleting the error branch left the suite green (REV-003). Both attach sites now
// route through this one function, so this table guards both.
func TestAttachFailureIsReported(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		want      string
		wantLevel statusLevel
	}{
		{"clean detach", nil, "detached from gogo-go-x", statusLevelOK},
		{"attach failed", errors.New("no server running"), "attach to gogo-go-x failed: no server running", statusLevelErr},
	} {
		got := attachOutcome("gogo-go-x", tc.err)
		if got.status != tc.want {
			t.Errorf("%s: status = %q, want %q", tc.name, got.status, tc.want)
		}
		if got.level != tc.wantLevel {
			t.Errorf("%s: level = %v, want %v", tc.name, got.level, tc.wantLevel)
		}
	}

	// Both production sites must route through it - a site that inlines its own
	// closure would silently escape the guard above. Assert the wiring by driving
	// each site's returned cmd is non-nil and its status is the "attaching" cue.
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	m.hasTmux = true
	nm, cmd := m.attachSession("gogo-go-x")
	if cmd == nil {
		t.Fatal("attachSession returned no cmd")
	}
	if got := nm.(Model); got.status != "attaching gogo-go-x" || got.statusLevel != statusLevelOK {
		t.Errorf("attachSession status = %q level %v, want the dim attaching cue", got.status, got.statusLevel)
	}
	pm := m
	pm.peeking, pm.peekSession = true, "gogo-go-x"
	if _, pcmd := pm.attachFromPeek(); pcmd == nil {
		t.Fatal("attachFromPeek returned no cmd")
	}
}

// TestAttachSitesShareOneOutcome is the structural half of REV-003: neither attach
// site may re-implement the outcome decision inline, or TestAttachFailureIsReported
// stops guarding it. Derived from the source, so a future inline copy fails here.
func TestAttachSitesShareOneOutcome(t *testing.T) {
	for _, f := range []string{"update.go", "peek.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		body := string(src)
		// The literal the discarded-error bug produced. Only attachOutcome may build it.
		n := strings.Count(body, `"detached from "`)
		want := 0
		if f == "update.go" {
			want = 1 // attachOutcome itself
		}
		if n != want {
			t.Errorf("%s builds the attach status %d time(s), want %d - both sites must call attachOutcome so one test guards both", f, n, want)
		}
	}
}

// --- FR2.5: the plans-tab key/help sync guard --------------------------------

// planKeyGlyph maps a raw key name to the glyph the help line documents it with.
var planKeyGlyph = map[string]string{"left": "←", "right": "→", "up": "↑", "down": "↓"}

// planListExempt / planDetailExempt are the keys a help line deliberately does not
// spell out because a documented key already covers them: the vim aliases ride
// along with the arrow glyphs, and the plan detail's q/left/h are aliases of `esc
// back` (q is also the globally-documented quit).
var (
	planListExempt   = map[string]bool{"h": true, "j": true, "k": true, "l": true}
	planDetailExempt = map[string]bool{"q": true, "left": true, "h": true, "j": true, "k": true}
)

// TestPlansTabKeyHelpInSync is the FR2.5 guard the briefing assumed already
// existed (it did not - TestCLICommandEnumerationInSync covers CLI VERBS only,
// and nothing asserted a key binding). It DERIVES the plans-tab keys from the
// `updatePlanList` / `updatePlanDetail` switches in the source and fails if any is
// missing from the corresponding rendered help line - so a new key can never ship
// undocumented.
func TestPlansTabKeyHelpInSync(t *testing.T) {
	seedDataHome(t)
	plans.New("app", "A plan to focus", "body")
	m := sizedWorkspace(t, &contract.Repo{}, proj("app", src("web", "/r/web")))
	m = tab(m) // → plans (the kanban)

	listHelp := lastLine(m.View())
	detail := send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if detail.planDetail == nil {
		t.Fatal("could not open a plan detail to read its help line")
	}
	detailHelp := lastLine(detail.View())

	for _, c := range []struct {
		fn     string
		help   string
		exempt map[string]bool
	}{
		{"updatePlanList", listHelp, planListExempt},
		{"updatePlanDetail", detailHelp, planDetailExempt},
	} {
		keys := switchKeys(t, "plans_tab.go", c.fn)
		if len(keys) < 8 {
			t.Fatalf("parsed only %d keys from %s (%v) - parser drift?", len(keys), c.fn, keys)
		}
		documented := documentedKeys(c.help)
		for _, k := range keys {
			if c.exempt[k] {
				continue
			}
			tok := k
			if g, ok := planKeyGlyph[k]; ok {
				tok = g
			}
			if !documented[tok] {
				t.Errorf("plans-tab key %q (handled by %s) is missing from its help line\n  help: %s\n  documented: %v\n"+
					"  add it to the help line, or (if it is an alias of a documented key) to the exempt set in this test",
					k, c.fn, c.help, sortedKeys(documented))
			}
		}
	}
}

// switchKeys parses fn's `switch msg.String()` block in the package source and
// returns every case's string literal - the keys that function actually handles.
func switchKeys(t *testing.T, file, fn string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var keys []string
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn {
			continue
		}
		ast.Inspect(fd, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			// Only the key switch (`switch msg.String()`), not any nested one.
			call, ok := sw.Tag.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "String" {
				return true
			}
			for _, stmt := range sw.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, e := range cc.List {
					if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						keys = append(keys, strings.Trim(lit.Value, `"`))
					}
				}
			}
			return true
		})
	}
	return keys
}

// documentedKeys reads a help line as a `·`-separated list of `<key> <description>`
// segments and returns the set of documented key tokens - each segment's FIRST
// word, with a glyph compound ("←→") split into its runes. Matching on the segment
// head, not a bare substring, is what makes the guard real: "move" contains a "v",
// so a Contains check would pass with `v` undocumented.
func documentedKeys(help string) map[string]bool {
	out := map[string]bool{}
	for _, seg := range strings.Split(help, "·") {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		head := fields[0]
		if r := []rune(head); len(r) > 1 && !isASCIIWord(head) {
			for _, c := range r {
				out[string(c)] = true
			}
			continue
		}
		out[head] = true
	}
	return out
}

// isASCIIWord reports whether s is all ASCII (so "esc"/"tab" stay whole while a
// glyph compound like "←→" is split into runes).
func isASCIIWord(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// lastLine returns the last non-empty line of a rendered view - where every gogo
// panel puts its help line.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimSpace(lines[i])
		}
	}
	return ""
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
