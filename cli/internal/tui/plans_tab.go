package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/pages"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// --- plans tab (FR10/FR11) --------------------------------------------------------
//
// The plans tab lists the focused project's plans grouped by lifecycle status
// (ACTIVE · READY · DRAFTS — D8) and drills into a plan's detail where the user
// targets sources and SPAWNS a work item per source. A spawn is a `claude -p`
// `gogo:plan --correlation plan-XXXX` launch through the SAME launcher seam the board
// uses (fire-exactly-once): the CLI writes nothing under a source's .gogo/work/; the
// skill writes the work item + stamps the correlation, which the board reads back as
// a ⛓ chip. m.plans is loaded on construction/reload (loadPlans); m.planDetail nil =
// the list, non-nil = the detail.

// planColumnTitles / planColumnStatus fix the plans KANBAN's 4-column layout left→right
// (plans-board FR1): drafts · ready · active · done, mirroring the work board's columns.
// planColumnStatus[i] is the lifecycle status that partitions into column i (rebuildPlans).
var (
	planColumnTitles = [4]string{"drafts", "ready", "active", "done"}
	planColumnStatus = [4]string{plans.StatusDraft, plans.StatusReady, plans.StatusActive, plans.StatusDone}
)

// planSlugHint derives the advisory kebab feature slug a spawn pins as the member
// hint (the analyst derives the real slug; the correlation id is the exact link).
//
// It runs the SAME transform the launch package uses for a session name
// (launch.SlugFromLabel == sanitizeLabel), because the two used to disagree and
// that is how a plan-spawned work item lost its attribution (FR1.7): the old
// `[^a-z0-9]+` regex collapsed an existing `-`, so a title's `" - "` became `-`
// here but `---` in the session name, and SessionMatchesSlug then returned false.
// One transform, no drift.
func planSlugHint(title string) string {
	if strings.TrimSpace(title) == "" {
		return "plan"
	}
	return launch.SlugFromLabel(title)
}

// focusedPlan returns the plan under the kanban cursor (planColIdx, planCardIdx), or nil
// when the focused column is empty.
func (m *Model) focusedPlan() *plans.Plan {
	col := m.planCols[m.planColIdx]
	if len(col) == 0 {
		return nil
	}
	idx := clamp(m.planCardIdx[m.planColIdx], 0, len(col)-1)
	return &col[idx]
}

// sourceByName returns the focused project's source with that label (default
// basename), or nil.
func (m *Model) sourceByName(name string) *projects.Source {
	if m.project == nil {
		return nil
	}
	for i := range m.project.Sources {
		s := &m.project.Sources[i]
		label := s.Name
		if label == "" {
			label = filepath.Base(s.Path)
		}
		if label == name {
			return s
		}
	}
	return nil
}

// resolveTargets partitions a plan's targets into those that resolve to a SOURCE
// of the focused project (spawnable) and those that do not (unknown) - FR3.1.
// A `targets:` entry naming a source of a DIFFERENT project (the user's live
// `dotai/plan-1948afcd`, which targets `gogo`) used to sail past every guard, open
// a confirm promising a spawn, and then be silently `continue`d over inside
// finishPlanSpawn - zero launches, plan untouched, and a status that never said
// why. Partitioning UP FRONT is what lets the confirm promise only what it can
// honour and lets an unknown target be named.
func (m Model) resolveTargets(p plans.Plan) (spawnable, unknown []string) {
	for _, t := range p.Targets {
		if m.sourceByName(t) != nil {
			spawnable = append(spawnable, t)
			continue
		}
		unknown = append(unknown, t)
	}
	return spawnable, unknown
}

// unknownTargetHint is the FR3.1 refusal: it NAMES the unresolvable target(s) and
// the project they are missing from, plus the two ways out. A status the user can
// act on, instead of "no spawnable targets for plan-XXXX".
func (m Model) unknownTargetHint(unknown []string) string {
	project := "this project"
	if m.project != nil {
		project = m.project.Name
	}
	noun := "which is not a source of"
	if len(unknown) > 1 {
		noun = "which are not sources of"
	}
	return "plan targets " + strings.Join(unknown, ", ") + ", " + noun + " project " + project +
		" - add it in the config tab, or retarget the plan"
}

// spawnedFeature returns the work item spawned for (source, plan) — a feature tagged
// with that source whose state.md correlation list contains the plan id — or nil
// when the source has not been spawned into yet (the ＋ create state).
func (m *Model) spawnedFeature(sourceName, planID string) *contract.Feature {
	if m.repo == nil {
		return nil
	}
	for _, f := range m.repo.Features {
		if f.Source != sourceName {
			continue
		}
		// On the unified board a source NAME can collide across projects (m.repo spans
		// every project), so scope the member lookup to the FOCUSED project — a same-named
		// source in another project must not match (REV-002). A feature with no Project
		// (the single-project seam, where m.repo is already one project's) is inert here.
		if f.Project != "" && m.project != nil && f.Project != m.project.Name {
			continue
		}
		for _, id := range f.Correlations {
			if id == planID {
				return f
			}
		}
	}
	return nil
}

// updatePlans drives the plans tab (FR10/FR11). It dispatches to the plan-detail
// handler when a detail is open, else the list handler. The persistent keys (q / tab
// / ?) are handled one level up in updateActive.
func (m Model) updatePlans(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.planDetail != nil {
		return m.updatePlanDetail(msg)
	}
	return m.updatePlanList(msg)
}

// updatePlanList handles the plans KANBAN keys (plans-board FR1/FR2): ←→/h l move
// columns · ↑↓/j k move cards · enter open detail · v terminal view · w web page ·
// n new · A plan-with-claude · m move (draft→ready→go→done) · p switch project ·
// x delete. The persistent keys (q / tab / ?) are handled one level up in
// updateActive.
//
// TestPlansTabKeyHelpInSync derives this switch's keys and fails if any is missing
// from the help line below - so a new key can never ship undocumented.
func (m Model) updatePlanList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		m.planColIdx = clamp(m.planColIdx-1, 0, 3)
	case "right", "l":
		m.planColIdx = clamp(m.planColIdx+1, 0, 3)
	case "p":
		// FR2.1: switch the plans tab's project IN PLACE — the exact call the config
		// tab's `p` already makes (one shared mover, zero new state). Kanban only:
		// a plan detail belongs to one plan of one project (FR2.3).
		m.switchProject(m.projIdx + 1)
	case "up", "k":
		m.planCardIdx[m.planColIdx] = clamp(m.planCardIdx[m.planColIdx]-1, 0, len(m.planCols[m.planColIdx])-1)
	case "down", "j":
		m.planCardIdx[m.planColIdx] = clamp(m.planCardIdx[m.planColIdx]+1, 0, len(m.planCols[m.planColIdx])-1)
	case "enter":
		if p := m.focusedPlan(); p != nil {
			cp := *p
			m.planDetail = &cp
			m.planSourceIdx = 0
			m.status = ""
		}
	case "v":
		return m.planView()
	case "w":
		return m, m.planPageCmd()
	case "n":
		if m.project != nil {
			m.startPlanForm()
			return m, m.form.Init()
		}
	case "A":
		return m.planWithClaude()
	case "m":
		return m.planMove(m.focusedPlan())
	case "x":
		if p := m.focusedPlan(); p != nil && m.project != nil {
			id := p.ID
			if _, err := plans.Delete(m.project.Name, id); err != nil {
				m.statusFailed("delete failed: " + err.Error())
			} else {
				m.loadPlans()
				m.status = "deleted " + id
			}
		}
	}
	m.reflowPlanColumns()
	return m, nil
}

// updatePlanDetail handles the plan-detail keys: ↑↓ work-item nav · c create work item
// · + add source · v terminal view · w web page · m move (advance the plan's lifecycle)
// · e edit plan · esc/q/← back. TestPlansTabKeyHelpInSync guards these against the
// detail's help line.
func (m Model) updatePlanDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.planDetail
	switch msg.String() {
	case "esc", "q", "left", "h":
		m.planDetail = nil
		m.status = ""
	case "up", "k":
		m.planSourceIdx = clamp(m.planSourceIdx-1, 0, len(p.Targets)-1)
	case "down", "j":
		m.planSourceIdx = clamp(m.planSourceIdx+1, 0, len(p.Targets)-1)
	case "c":
		return m.planCreateWorkItem()
	case "+":
		return m.planAddTarget()
	case "v":
		return m.planView()
	case "w":
		return m, m.planPageCmd()
	case "m":
		return m.planMove(p)
	case "e":
		m.status = "edit the plan by hand at " + plans.Path(m.project.Name, p.ID) + " (or `gogo plan show " + p.ID + "`)"
	}
	return m, nil
}

// --- plan viewers: v (terminal) / w (web) - FR2 -------------------------------
//
// The work board's cards have had both for releases; plan cards had neither, so a
// plan could only be read in the cramped inline detail pane. Both reuse the seams
// that already exist - openArtifact (the same glamour article renderer, width-keyed
// cache, spinner and paging keys) and pages.Bundle (which degrades cleanly with no
// diagram set, verified against a real plan) - so neither needs a new renderer.

// planPath is the focused project's on-disk path for a plan id
// (~/.gogo/projects/<name>/.gogo/plans/<id>.md) - the file `v` renders, `w` builds
// from, and an over-budget launch folds to a pointer at. "" with no project.
func (m Model) planPath(id string) string {
	if m.project == nil {
		return ""
	}
	return plans.Path(m.project.Name, id)
}

// currentPlan is the plan the plans tab is acting on: the open detail when there is
// one, else the plan under the kanban cursor. nil when the focused column is empty.
func (m Model) currentPlan() *plans.Plan {
	if m.planDetail != nil {
		return m.planDetail
	}
	return m.focusedPlan()
}

// planView (v) opens the focused plan's markdown in the TERMINAL viewer (FR2.1) -
// the same article renderer the board's `v` uses, over the one markdown file a plan
// is. It sets planViewing FIRST: that flag is what makes the viewer's esc return
// here instead of to modeDrill, which would nil-deref m.drill (FR2.2/D3).
func (m Model) planView() (tea.Model, tea.Cmd) {
	p := m.currentPlan()
	if p == nil || m.project == nil {
		m.statusBlocked("no plan focused - nothing to view")
		return m, nil
	}
	path := plans.Path(m.project.Name, p.ID)
	if !fileExists(path) {
		m.statusBlocked("no plan file at " + path)
		return m, nil
	}
	m.planViewing = true
	return m, m.openArtifact(contract.Artifact{Label: p.ID + ".md", Path: path, Kind: contract.KindMarkdown})
}

// closePlanView returns from a plan view to the plans tab (FR2.2). It mirrors
// closePeek exactly - the established precedent for "this viewer was not opened
// from a drill". It must NEVER leave mode == modeDrill: a plan has no drilled
// card, and viewDrill dereferences m.drill.
func (m Model) closePlanView() Model {
	m.planViewing = false
	m.mode = modeBoard // renders the active tab
	m.tab = tabPlans
	return m
}

// planPageCmd (w) builds the focused plan's self-contained interactive page and
// opens it (FR2.3). The page is written under the PROJECT HOME
// (~/.gogo/projects/<name>/.gogo/resources/view/<plan-id>.html - D2=A): a plan has
// no repo of its own, and the CLI never writes a source repo's .gogo/ (FR2.4).
func (m Model) planPageCmd() tea.Cmd {
	p := m.currentPlan()
	if p == nil || m.project == nil {
		return func() tea.Msg {
			return launchDoneMsg{status: "no plan focused - nothing to build", level: statusLevelWarn}
		}
	}
	project := m.project.Name
	root := projects.Dir(project)
	bundle := planBundleFor(project, *p)
	if !fileExists(bundle.MarkdownPath) {
		return func() tea.Msg {
			return launchDoneMsg{status: "no plan file at " + bundle.MarkdownPath, level: statusLevelWarn}
		}
	}
	return func() tea.Msg {
		page, err := pages.WritePage(root, bundle)
		if err != nil {
			return launchDoneMsg{status: "page build failed: " + err.Error(), level: statusLevelErr}
		}
		openBrowser(page)
		return launchDoneMsg{status: "page: " + page}
	}
}

// planBundleFor is the plan's page bundle (FR2.3): the plan markdown and NOTHING
// else. A project plan is one file with no charts/ - and pages.BuildHTML degrades
// cleanly on empty DiagramDir/BeforeDir/ManifestPath (contract.ReadManifest("")
// returns (nil, nil) and Manifest.TitleFor is nil-safe), so the page renders the
// summary with an empty diagram slot rather than erroring.
func planBundleFor(project string, p plans.Plan) pages.Bundle {
	title := p.Title
	if strings.TrimSpace(title) == "" {
		title = p.ID
	}
	return pages.Bundle{
		Name:         p.ID,
		Title:        "gogo - " + title,
		MarkdownPath: plans.Path(project, p.ID),
	}
}

// planMove is the plans-tab `m` move (plans-board FR2): it advances the given plan one
// column right, resolving the phase action by its CURRENT (persisted) status — draft→ready
// (mark ready, no spawn), ready→active (go: fan-out spawn), active→done (project-UAT
// accept). A done plan is terminal (a status bounce). Shared by the kanban (focused plan)
// and the plan detail, so both surfaces move a plan the same way.
func (m Model) planMove(p *plans.Plan) (tea.Model, tea.Cmd) {
	if p == nil || m.project == nil {
		return m, nil
	}
	switch p.Status {
	case plans.StatusDraft:
		return m.planMarkReady(p)
	case plans.StatusReady:
		return m.planGo(p)
	case plans.StatusActive:
		return m.planAcceptUAT(p)
	case plans.StatusDone:
		m.statusBlocked("plan " + p.ID + " is already done")
		return m, nil
	}
	return m, nil
}

// planMarkReady is the draft→ready move (plans-board FR3): it marks the plan ready WITHOUT
// spawning anything — the plan waits for implementation. This RE-SEQUENCES the 0.25.0
// behaviour (which fired the auto-spawn on the old `r`); the fan-out now lives on planGo
// (ready→active).
func (m Model) planMarkReady(p *plans.Plan) (tea.Model, tea.Cmd) {
	if p == nil || m.project == nil {
		return m, nil
	}
	if _, err := plans.MarkReady(m.project.Name, p.ID); err != nil {
		m.statusFailed("mark-ready failed: " + err.Error())
		return m, nil
	}
	m.loadPlans()
	m.status = "marked " + p.ID + " ready - press m again to go (spawn its work items)"
	return m, nil
}

// planAcceptUAT is the plans-tab project-UAT accept trigger (FR3, `D`) — the TUI
// mirror of `gogo plan done`. It applies the SAME guard: it REFUSES (a status message
// naming any unshipped members, no state change) unless EVERY member work item of the
// plan is shipped, reading each member's source state.md through the already-loaded
// board repo (plans.MembersShippedIn — never a source .gogo/ write). Only when all are
// shipped does it open a huh confirm; the confirm's completion (finishPlanDone) records
// the accept via plans.MarkDone (appends a `## Project UAT` round + flips the plan to
// the persisted `done`).
func (m Model) planAcceptUAT(p *plans.Plan) (tea.Model, tea.Cmd) {
	if p == nil || m.project == nil {
		return m, nil
	}
	if p.Status == plans.StatusDone {
		m.statusBlocked("plan " + p.ID + " is already done (project-UAT accepted)")
		return m, nil
	}
	allShipped, unshipped := plans.MembersShippedIn(m.project.Name, *p, m.repo)
	if !allShipped {
		if len(p.Members) == 0 {
			m.statusBlocked("refusing — plan " + p.ID + " has no work items yet; spawn + ship members first (c)")
		} else {
			// FR3.2: the SAME project-UAT refusal as the arm above, so it must read the
			// same way - it rendered dim while its sibling rendered amber (REV-006).
			m.statusBlocked(fmt.Sprintf("refusing — %d of %d member(s) not shipped: %s",
				len(unshipped), len(p.Members), strings.Join(unshipped, ", ")))
		}
		return m, nil
	}
	m.startPlanDoneForm(p)
	return m, m.form.Init()
}

// startPlanDoneForm opens the huh project-UAT accept confirm under modeForm (FR3, `D`).
// It marks pendingPlanDone (so updateForm routes completion to finishPlanDone and a
// cancel returns to the plans tab) and binds the confirm through a heap-stable
// *formBinding (TEST-001). Reached only after planAcceptUAT's members-shipped guard
// passed, so accepting flips a genuinely-ready plan.
func (m *Model) startPlanDoneForm(p *plans.Plan) {
	m.pendingPlanDone = &planDoneEdit{project: m.project.Name, id: p.ID, title: p.Title}
	// CONFIRM-DEFAULT CONVENTION (TEST-001) - see startFormOverriding in move.go for the
	// canonical statement. A FORWARD pipeline move (`m`: launch / spawn / accept) seeds
	// `confirm: true`, so the affirmative starts highlighted and a bare Enter submits it -
	// the board's `m` has always behaved that way, and an unseeded binding here made the
	// SAME keystroke silently cancel on the plans tab. A DESTRUCTIVE action (delete, kill)
	// seeds `confirm: false` on purpose so Enter is safe and the user must arrow over.
	// Do not "align" the two: the asymmetry IS the safety rule.
	b := &formBinding{confirm: true}
	m.binding = b
	m.form = newForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Accept project-UAT for " + p.ID + "?").
			Description("all members shipped — flips this plan to done + records a project-UAT round (~/.gogo/ only)").
			Affirmative("Accept").
			Negative("Cancel").
			Value(&b.confirm),
	))
	m.mode = modeForm
}

// finishPlanDone applies a completed project-UAT accept confirm (FR3). On Accept it
// re-guards the members-shipped invariant (defensive — the board may have moved since
// the confirm opened), records the accept via plans.MarkDone (a ~/.gogo/ write only,
// never a source's .gogo/), reloads the plans list, and lands back on the plans tab.
// The now-`done` plan moves into the kanban's 4th `done` column (it stays in the store).
func (m Model) finishPlanDone() (tea.Model, tea.Cmd) {
	edit := m.pendingPlanDone
	b := m.binding
	m.pendingPlanDone = nil
	m.binding = nil
	m.form = nil
	m.mode = modeBoard // renders the active tab (tabPlans)
	if edit == nil || b == nil {
		return m, nil
	}
	if !b.confirm {
		m.status = "cancelled"
		return m, nil
	}
	p, ok := plans.Get(edit.project, edit.id)
	if !ok {
		m.statusFailed("no plan " + edit.id + " in " + edit.project)
		return m, nil
	}
	if allShipped, unshipped := plans.MembersShippedIn(edit.project, p, m.repo); !allShipped {
		m.statusBlocked(fmt.Sprintf("refusing — %d member(s) not shipped: %s", len(unshipped), strings.Join(unshipped, ", ")))
		return m, nil
	}
	if _, err := plans.MarkDone(edit.project, edit.id); err != nil {
		m.statusFailed("accept failed: " + err.Error())
		return m, nil
	}
	m.loadPlans()
	m.planDetail = nil
	// The now-`done` plan stays in the store (rebuildPlans clamps the cursors) and shows
	// in the kanban's 4th `done` column — no cursor reset needed.
	m.status = "accepted project-UAT for " + edit.id + " — plan is now done"
	return m, nil
}

// planDerivedStatus computes p's DISPLAY status (FR3, derive-at-read): an `active`
// plan whose every member work item is shipped reads `awaiting-project-uat`, else the
// persisted status. The members-shipped decision reads the already-loaded board repo
// (never a source .gogo/ write).
func (m Model) planDerivedStatus(p plans.Plan) string {
	project := ""
	if m.project != nil {
		project = m.project.Name
	}
	allShipped, _ := plans.MembersShippedIn(project, p, m.repo)
	return plans.DerivedStatus(p, allShipped)
}

// planCreateWorkItem SPAWNS a work item for the focused target source (FR11): it
// builds launch.PlanIntent(plan.Title, body, plan.ID) — carrying the correlation id
// as an explicit --correlation param — and fires it through the launcher seam
// EXACTLY ONCE, anchored at the source root. The CLI writes NOTHING under the
// source's .gogo/work/: the launched `gogo:plan` skill writes the work item + stamps
// the correlation. The advisory member + ready→active flip are recorded ONLY AFTER a
// SUCCESSFUL launch (REV-005) — a failed spawn leaves the plan untouched (no phantom
// active member), so the store never over-reports a work item that was never created.
func (m Model) planCreateWorkItem() (tea.Model, tea.Cmd) {
	p := m.planDetail
	if p == nil {
		return m, nil
	}
	if m.planSourceIdx < 0 || m.planSourceIdx >= len(p.Targets) {
		m.statusBlocked("no target source selected — add one with +")
		return m, nil
	}
	sourceName := p.Targets[m.planSourceIdx]
	src := m.sourceByName(sourceName)
	if src == nil {
		// FR3.1: name the source AND the project it is missing from, plus the way out.
		m.statusBlocked(m.unknownTargetHint([]string{sourceName}))
		return m, nil
	}
	if !m.hasClaude {
		m.statusBlocked("claude CLI not on PATH — cannot spawn a work item")
		return m, nil
	}
	body := p.Description
	if strings.TrimSpace(body) == "" {
		body = p.Title
	}
	root := src.Path
	intent := launch.PlanIntent(p.Title, body, p.ID)
	intent.Root = root
	// FR5.2: name the plan's attachments to the launched session (bounded decorator,
	// before the fold so the budget check measures the real command).
	intent = launch.WithAttachments(intent, p.Attachments)
	// FR1.3 (D1=A): a multi-KB brief inlined into the tmux command line is what tmux
	// refuses with `command too long`. Over budget, swap the inlined body for a
	// pointer at the file that already holds it. A no-op under budget.
	intent = launch.FoldToPointer(intent, m.planPath(p.ID), sourceName)
	launcher := m.launcher
	planID := p.ID
	member := plans.Member{Source: sourceName, SlugHint: planSlugHint(p.Title)}
	project := ""
	if m.project != nil {
		project = m.project.Name
	}

	return m, func() tea.Msg {
		res, err := launcher(root, intent)
		if err != nil {
			// Launch failed → leave the plan UNTOUCHED (no phantom active member). The
			// error now carries tmux's own words (FR1.1) and renders red (FR3.2).
			return launchDoneMsg{status: "spawn failed: " + err.Error(), level: statusLevelErr}
		}
		// Record the spawn ONLY on success (advisory member + ready→active) — store
		// writes to ~/.gogo/ only, never the source's .gogo/. The launchDoneMsg handler
		// reloads m.plans so the Model catches up to this store write.
		if project != "" {
			plans.AddMember(project, planID, member)
			plans.SetStatus(project, planID, plans.StatusActive)
		}
		where := res.Session
		if where == "" {
			where = res.LogPath
		}
		return launchDoneMsg{status: "spawning work item in " + sourceName + " → " + res.Command + " (" + where + ")"}
	}
}

// planGo is the ready→active move (plans-board FR3): it fans out a work item into each
// UN-spawned target the analyst chose — a huh confirm listing the targets, then
// finishPlanSpawn loops the fire-once launcher seam (mirroring planCreateWorkItem) once
// per un-spawned target with its per-source brief + skip, recording a member + flipping
// the plan active ONLY on a successful launch. A targetless plan has nothing to spawn (a
// hint pointing at + to add a target); every target already spawned is a no-op (idempotent
// — a re-move never re-launches). Spawning needs claude on PATH; c (spawn one focused
// target) stays the manual fallback, unchanged.
func (m Model) planGo(p *plans.Plan) (tea.Model, tea.Cmd) {
	if p == nil || m.project == nil {
		return m, nil
	}
	if len(p.Targets) == 0 {
		m.statusBlocked("plan " + p.ID + " has no targets - open it (enter) and press + to add one before go")
		return m, nil
	}
	todo := m.unspawnedTargets(*p)
	if len(todo) == 0 {
		m.status = fmt.Sprintf("all %d target(s) already spawned for %s", len(p.Targets), p.ID)
		return m, nil
	}
	// FR3.1: partition BEFORE the confirm. A target that resolves to no source of this
	// project cannot be spawned into, so the confirm must never promise it - and a plan
	// with NO resolvable target must not open a confirm it cannot honour at all.
	spawnable, unknown := m.resolveTargets(plans.Plan{Targets: todo})
	if len(spawnable) == 0 {
		m.statusBlocked(m.unknownTargetHint(unknown))
		return m, nil
	}
	if !m.hasClaude {
		m.statusBlocked("claude CLI not on PATH — cannot spawn work items (use `gogo plan promote`, or `c` per source)")
		return m, nil
	}
	m.startPlanSpawnForm(p, spawnable, unknown)
	return m, m.form.Init()
}

// unspawnedTargets returns the plan's targets that have NOT been spawned into yet — the
// fan-out set the `r` accept confirms. A target counts as spawned when the plan already
// records a member for it OR a board feature carries the plan id in that source (the same
// signal the plan card's dot strip uses), so a re-`r` (or a target spawned earlier via
// `c`) is skipped (idempotent).
func (m Model) unspawnedTargets(p plans.Plan) []string {
	var out []string
	for _, t := range p.Targets {
		if m.targetSpawned(p, t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// targetSpawned reports whether the plan already has a work item for source t — a
// recorded member (the store-side, launcher-driven idempotency signal) or a board
// feature carrying the plan id (the out-of-band `c` / retroactive-link signal).
func (m Model) targetSpawned(p plans.Plan, t string) bool {
	for _, mem := range p.Members {
		if mem.Source == t {
			return true
		}
	}
	return m.spawnedFeature(t, p.ID) != nil
}

// startPlanSpawnForm opens the huh accept+spawn confirm under modeForm (0.25.0 FR2, `r`).
// It marks pendingPlanSpawn (so updateForm routes completion to finishPlanSpawn and a
// cancel returns to the plans tab) and binds the confirm through a heap-stable
// *formBinding (TEST-001). Reached only with ≥1 SPAWNABLE un-spawned target + claude
// on PATH: targets carries only what can actually be launched, and unknown (FR3.1)
// carries the unresolvable names so the confirm SAYS what it is skipping instead of
// dropping them silently after the user says yes.
func (m *Model) startPlanSpawnForm(p *plans.Plan, targets, unknown []string) {
	m.pendingPlanSpawn = &planSpawnEdit{project: m.project.Name, id: p.ID, title: p.Title, targets: targets}
	// CONFIRM-DEFAULT CONVENTION (TEST-001) - see startFormOverriding in move.go for the
	// canonical statement. This is a FORWARD pipeline move (`m` on a ready plan), so it
	// seeds `confirm: true` and a bare Enter spawns - matching the board's `m`, whose
	// muscle memory a user brings to this tab. Destructive confirms (delete, kill) seed
	// `confirm: false` on purpose; do not align the two.
	b := &formBinding{confirm: true}
	m.binding = b
	desc := "into: " + strings.Join(targets, ", ") + " — launches /gogo:plan per source, records members, flips the plan active"
	if len(unknown) > 0 {
		desc += "\nskipping " + strings.Join(unknown, ", ") + " - not a source of project " + m.project.Name
	}
	m.form = newForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Accept %s and spawn %d work item(s)?", p.ID, len(targets))).
			Description(desc).
			Affirmative("Spawn").
			Negative("Cancel").
			Value(&b.confirm),
	))
	m.mode = modeForm
}

// finishPlanSpawn applies a completed accept+spawn confirm (0.25.0 FR2, D3=a). On Spawn
// it LOOPS the fire-once launcher seam once per un-spawned target — building
// PlanIntent(title, BriefFor(target) or body, planID) + the target source's per-source
// `--skip-acceptance` — and records a member + flips the plan `active` ONLY on a
// SUCCESSFUL launch (REV-005: a failed launch leaves no phantom member). The CLI writes
// NOTHING under a source's .gogo/: each launched `gogo:plan` skill writes the work item +
// stamps the correlation. The launchDoneMsg handler reloads m.plans so the Model catches
// up to the store writes.
func (m Model) finishPlanSpawn() (tea.Model, tea.Cmd) {
	edit := m.pendingPlanSpawn
	b := m.binding
	m.pendingPlanSpawn = nil
	m.binding = nil
	m.form = nil
	m.mode = modeBoard // renders the active tab (tabPlans)
	if edit == nil || b == nil {
		return m, nil
	}
	if !b.confirm {
		m.status = "cancelled"
		return m, nil
	}
	p, ok := plans.Get(edit.project, edit.id)
	if !ok {
		m.statusFailed("no plan " + edit.id + " in " + edit.project)
		return m, nil
	}
	body := p.Description
	if strings.TrimSpace(body) == "" {
		body = p.Title
	}
	// Resolve each target's root + intent NOW (the Model still carries project/sources);
	// the fired cmd only touches the launcher + the ~/.gogo/ store.
	type spawn struct {
		source string
		root   string
		intent launch.Intent
	}
	var spawns []spawn
	var unresolved []string
	for _, target := range edit.targets {
		src := m.sourceByName(target)
		if src == nil {
			// The confirm only ever lists spawnable targets (FR3.1), so reaching here
			// means the source vanished between the confirm and the submit. Never a
			// phantom member - but no longer a SILENT drop either: name it below.
			unresolved = append(unresolved, target)
			continue
		}
		goal := plans.BriefFor(p, target)
		if strings.TrimSpace(goal) == "" {
			goal = body
		}
		intent := launch.PlanIntent(p.Title, goal, p.ID)
		intent.Root = src.Path
		// Ride the skip flag of the source ALREADY in hand (m.sourceByName scopes to the
		// FOCUSED project), not a first-path-match across EVERY project's sources (REV-001):
		// a repo linked to two projects with opposite PlanAcceptanceSkip must carry the
		// focused project's flag, never whichever identically-pathed source sorts first.
		intent.Command += launch.SkipParams(src.PlanAcceptanceSkip, false)
		// FR5.2: name the plan's attachments to each spawned session (bounded decorator,
		// before the fold so the budget check measures the real command).
		intent = launch.WithAttachments(intent, p.Attachments)
		// FR1.3: fold an over-budget brief to a pointer at the plan file, naming this
		// source's `### <name>` subsection. The skip params were appended first and are
		// preserved by the fold. A no-op under budget.
		intent = launch.FoldToPointer(intent, plans.Path(edit.project, edit.id), target)
		spawns = append(spawns, spawn{source: target, root: src.Path, intent: intent})
	}
	if len(spawns) == 0 {
		m.statusBlocked(m.unknownTargetHint(unresolved))
		return m, nil
	}
	launcher := m.launcher
	project := edit.project
	planID := edit.id
	slugHint := planSlugHint(p.Title)

	return m, func() tea.Msg {
		launched, failed := 0, 0
		firstErr := ""
		for _, s := range spawns {
			if _, err := launcher(s.root, s.intent); err != nil {
				failed++
				if firstErr == "" {
					firstErr = err.Error()
				}
				continue // leave this target un-recorded (no phantom member, REV-005)
			}
			plans.AddMember(project, planID, plans.Member{Source: s.source, SlugHint: slugHint})
			plans.SetStatus(project, planID, plans.StatusActive)
			launched++
		}
		status := fmt.Sprintf("accepted %s — spawned %d work item(s)", planID, launched)
		level := statusLevelOK
		if failed > 0 {
			// FR1.1/FR3.2: carry the first real error's words (tmux's own, now that
			// stderr is captured) rather than an opaque count, and render it as a failure.
			status += fmt.Sprintf(" (%d failed: %s)", failed, firstErr)
			level = statusLevelErr
		}
		if len(unresolved) > 0 {
			status += " · skipped " + strings.Join(unresolved, ", ") + " (no such source)"
			if level == statusLevelOK {
				level = statusLevelWarn
			}
		}
		return launchDoneMsg{status: status, level: level}
	}
}

// planAddTarget adds the next project source not yet targeted to the plan's targets
// (FR11 `+ add source`), persists it, and refreshes the open detail. A no-op with a
// status when every source is already a target.
func (m Model) planAddTarget() (tea.Model, tea.Cmd) {
	p := m.planDetail
	if p == nil || m.project == nil {
		return m, nil
	}
	for _, s := range m.project.Sources {
		label := s.Name
		if label == "" {
			label = filepath.Base(s.Path)
		}
		if containsString(p.Targets, label) {
			continue
		}
		if _, err := plans.AddTarget(m.project.Name, p.ID, label); err != nil {
			m.statusFailed("add source failed: " + err.Error())
			return m, nil
		}
		m.loadPlans()
		if updated, ok := plans.Get(m.project.Name, p.ID); ok {
			m.planDetail = &updated
		}
		m.status = "added source " + label
		return m, nil
	}
	m.statusBlocked("every source is already a target")
	return m, nil
}

// projectSelectField is the FR1.1 destination-project Select — the FIRST field of
// both mint forms whenever more than one project is registered (FR1.2 mirrors
// resolveProjectName's established rule: one project = no ambiguity = no prompt).
// The caller pre-seeds binding.planProject to the focused project BEFORE this
// builds the field, so huh's selectValue puts the cursor on it and the common case
// stays one keystroke (FR1.3).
func (m *Model) projectSelectField(b *formBinding) huh.Field {
	opts := make([]huh.Option[string], 0, len(m.allProjects))
	for _, p := range m.allProjects {
		opts = append(opts, huh.NewOption(p.Name, p.Name))
	}
	return huh.NewSelect[string]().
		Title("Project").
		Description("the project this plan is created in").
		Options(opts...).
		Value(&b.planProject)
}

// attachmentsField is the optional FR4 attachments Text — ONE local file path or
// http(s) URL PER LINE, refused at submit with a named error when a line is
// neither (validateAttachments). A Text, not an Input, on purpose: FR3 just made
// multi-line entry work, paths contain spaces, and huh.NewInput silently flattens
// a pasted newline (measured).
func attachmentsField(b *formBinding) huh.Field {
	return huh.NewText().
		Title("Attachments (optional)").
		Description("one local file path or http(s):// URL per line — recorded on the plan + named to the launched session").
		Lines(3).
		Validate(validateAttachments).
		Value(&b.planAttach)
}

// attachmentLines splits the attachments Text value into trimmed, non-empty lines.
func attachmentLines(raw string) []string {
	var out []string
	for _, ln := range strings.Split(raw, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// normalizeAttachment validates ONE attachment line and returns its stored form:
// an http(s) URL verbatim (SHAPE-checked only, never fetched — the core loop's
// no-external-deps bar), or a `~`-expanded, ABSOLUTE local path that must exist
// (so the record is cwd-independent). A comma is refused outright — the store's
// list format splits on `,` (D4), so it would be silently corrupted, never stored.
func normalizeAttachment(line string) (string, error) {
	if strings.Contains(line, ",") {
		return "", fmt.Errorf("%q: a comma cannot be stored (the attachments: list splits on it)", line)
	}
	if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
		if strings.TrimPrefix(strings.TrimPrefix(line, "https://"), "http://") == "" {
			return "", fmt.Errorf("%q: not a usable URL", line)
		}
		return line, nil
	}
	p := line
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("%q: %v", line, err)
	}
	// REV-004: the STORED value is the normalized one — a comma re-entering via cwd
	// or $HOME would be silently split into two bogus entries by parseList, so the
	// D4 refusal must also run on what is actually stored.
	if strings.Contains(abs, ",") {
		return "", fmt.Errorf("the resolved path %q contains a comma, which the attachments: list cannot store - move the file or pass a comma-free path", abs)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("%q: not an existing local file (nor an http(s):// URL)", line)
	}
	// REV-007: the launched session is told "a local path is a file" — keep the
	// promise true and refuse a directory here rather than in the session.
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("%q: is a directory (or not a regular file) - attach a file", line)
	}
	return abs, nil
}

// validateAttachments is the huh Validate hook for the attachments field (FR4.3):
// it refuses to advance, naming the offending line, when any line is neither an
// existing local file nor an http(s) URL (or carries a comma — FR4.4).
func validateAttachments(raw string) error {
	for _, ln := range attachmentLines(raw) {
		if _, err := normalizeAttachment(ln); err != nil {
			return err
		}
	}
	return nil
}

// parseAttachments returns the normalized attachment set for a validated
// attachments Text value (absolute local paths, verbatim URLs). Lines that fail
// to normalize are skipped — validateAttachments already refused them at submit,
// so this is belt-and-braces, never a second error path.
func parseAttachments(raw string) []string {
	var out []string
	for _, ln := range attachmentLines(raw) {
		if a, err := normalizeAttachment(ln); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// focusChosenProject moves the shared focus to the mint form's chosen project
// (FR1.5) BEFORE anything mints or launches, so plans.New, projects.KnowledgeDir,
// sourceRefs and firstSourcePath all resolve against the SAME project — without
// this the plan file would land in project B while the analyst session anchored
// at project A's first source. A blank choice, an unknown name, or the
// already-focused project is a no-op, so the single-project form (no Select)
// stays byte-for-byte today's path.
func (m *Model) focusChosenProject(chosen string) {
	if chosen == "" || m.project == nil || chosen == m.project.Name {
		return
	}
	for i := range m.allProjects {
		if m.allProjects[i].Name == chosen {
			m.switchProject(i)
			return
		}
	}
}

// startPlanForm opens the huh new-plan form (FR10 `n`): a destination-project
// Select FIRST when several projects exist (FR1.1, pre-seeded to the focused one),
// then the title input, the optional DESCRIPTION textarea (0.25.1) and the optional
// attachments Text (FR4), under modeForm. It marks pendingPlan (so updateForm
// routes completion to finishPlanForm and a cancel returns to the plans tab) and
// binds every field through a heap-stable *formBinding (TEST-001).
func (m *Model) startPlanForm() {
	m.pendingPlan = true
	b := &formBinding{}
	m.binding = b
	var fields []huh.Field
	titleDesc := "creates a draft plan in " + m.project.Name + " — target sources + spawn from its detail"
	if len(m.allProjects) > 1 {
		b.planProject = m.project.Name // pre-seed BEFORE the field is built (FR1.3)
		fields = append(fields, m.projectSelectField(b))
		titleDesc = "creates a draft plan in the project selected above — target sources + spawn from its detail"
	}
	fields = append(fields,
		huh.NewInput().
			Title("New plan title").
			Description(titleDesc).
			Value(&b.planTitle),
		huh.NewText().
			Title("Description (optional)").
			Description("the plan's goal / brief — enter = new line · tab = next · ctrl+e = $EDITOR").
			Lines(4).
			Value(&b.planDesc),
		attachmentsField(b),
	)
	m.form = newForm(huh.NewGroup(fields...))
	m.mode = modeForm
}

// finishPlanForm applies a completed new-plan form: a non-blank title creates a
// draft plan in the CHOSEN project (FR1.5 — the shared focus switches there first;
// a write to ~/.gogo/ only) carrying the optional description + attachments,
// reloads the list, and lands back on the plans tab with a status NAMING the
// destination project (FR1.4).
func (m Model) finishPlanForm() (tea.Model, tea.Cmd) {
	title, desc, chosen, attach := "", "", "", ""
	if m.binding != nil {
		title = strings.TrimSpace(m.binding.planTitle)
		desc = strings.TrimSpace(m.binding.planDesc)
		chosen = strings.TrimSpace(m.binding.planProject)
		attach = m.binding.planAttach
	}
	m.pendingPlan = false
	m.binding = nil
	m.form = nil
	m.mode = modeBoard // renders the active tab (tabPlans)
	if title == "" {
		m.status = "cancelled"
		return m, nil
	}
	if m.project == nil {
		m.statusBlocked("no project — cannot create a plan")
		return m, nil
	}
	m.focusChosenProject(chosen) // FR1.5: switch BEFORE minting
	p, err := plans.New(m.project.Name, title, desc)
	if err != nil {
		m.statusFailed("create failed: " + err.Error())
		return m, nil
	}
	if atts := parseAttachments(attach); len(atts) > 0 {
		if _, err := plans.SetAttachments(m.project.Name, p.ID, atts); err != nil {
			m.statusFailed("attachments failed: " + err.Error())
			return m, nil
		}
	}
	m.loadPlans()
	m.planColIdx = 0 // a new plan is a draft — focus the drafts column
	m.planCardIdx[0] = 0
	m.status = "created draft " + p.ID + " in " + m.project.Name
	return m, nil
}

// planAuthorLaunchedMsg carries the outcome of the plans-tab `A` analyst launch (0.25.1)
// back to Update so it can ATTACH the user into the live session. session is the created
// tmux session name (attachable) or "" when the launcher fell back to a backgrounded
// `claude -p` (no tmux → nothing to attach). logPath is that backgrounded run's log
// (res.LogPath) — the one diagnostic pointer surfaced on the headless path so a stalled/
// failed run is inspectable (REV-006). homeNote carries the no-source anchor heads-up
// ("runs in the project home; approve it if Claude prompts", REV-008) when the session
// fell back to the untrusted project home. Distinct from launchDoneMsg because the attach
// must happen on the UI goroutine (tea.ExecProcess), after the launcher fired.
type planAuthorLaunchedMsg struct {
	session  string
	logPath  string
	homeNote string
}

// planWithClaude is the plans-tab `A` plan-with-claude authoring trigger (FR-D — the
// user's "start a claude session and prepare a plan" ask). It authors the PROJECT PLAN,
// not a source work item. 0.25.1 fixes the two UAT-critical bugs: it FIRST opens a goal
// form (no more blank "Untitled plan" mint) and, on submit, launches AND ATTACHES the
// session (no more detached, unseen run). This handler only guards + opens the form; the
// mint/launch/attach happens in finishPlanWithClaude once the goal is captured. `n` stays
// the quick inline draft.
func (m Model) planWithClaude() (tea.Model, tea.Cmd) {
	if m.project == nil {
		m.statusBlocked("no project — cannot author a plan")
		return m, nil
	}
	if !m.hasClaude {
		m.statusBlocked("claude CLI not on PATH — cannot start a plan-with-claude session (use `n` for a quick draft)")
		return m, nil
	}
	m.startPlanWithClaudeForm()
	return m, m.form.Init()
}

// startPlanWithClaudeForm opens the huh `A` goal form (0.25.1) under modeForm: a
// destination-project Select FIRST when several projects exist (FR1.1, pre-seeded
// to the focused one — this form used to be the silent wrong-project mint), then
// the required GOAL textarea (multi-line for real since FR3), the optional short
// title (derived from the goal when blank) and the optional attachments Text
// (FR4). It marks pendingPlanWithClaude (so updateForm routes completion to
// finishPlanWithClaude and a cancel mints NOTHING) and binds every field through a
// heap-stable *formBinding (TEST-001).
func (m *Model) startPlanWithClaudeForm() {
	m.pendingPlanWithClaude = true
	b := &formBinding{}
	m.binding = b
	var fields []huh.Field
	goalDesc := "the analyst reads " + m.project.Name + "'s sources and writes the plan for this goal"
	if len(m.allProjects) > 1 {
		b.planProject = m.project.Name // pre-seed BEFORE the field is built (FR1.3)
		fields = append(fields, m.projectSelectField(b))
		goalDesc = "the analyst reads the selected project's sources and writes the plan for this goal"
	}
	fields = append(fields,
		huh.NewText().
			Title("What should gogo plan for this project? (the goal — what to build or change across its sources)").
			Description(goalDesc+" — enter = new line · tab = next · ctrl+e = $EDITOR").
			Lines(5).
			Value(&b.planGoal),
		huh.NewInput().
			Title("Plan title (optional)").
			Description("defaults to a short title derived from the goal").
			Value(&b.planTitle),
		attachmentsField(b),
	)
	m.form = newForm(huh.NewGroup(fields...))
	m.mode = modeForm
}

// finishPlanWithClaude applies a completed `A` goal form (0.25.1). On CANCEL / empty goal
// it mints NOTHING (no blank draft, no launch). On submit it mints a draft plan whose
// DESCRIPTION IS THE GOAL (never blank/"Untitled") with the typed-or-derived title, then
// fires — through the launcher seam, EXACTLY ONCE — a PLAIN interactive `claude` session
// (NOT a slash command) seeded by launch.AuthorPlanIntent NAMING the goal, to READ + EDIT
// the plan file IN PLACE. It deliberately does NOT launch /gogo:plan (that skill scaffolds
// a source `.gogo/work/`, the wrong thing for a project-plan file). The session is ANCHORED
// at the project's FIRST SOURCE root — a real repo the user already trusts in Claude — NOT
// the untrusted `~/.gogo/projects/<name>/` home (first-run trust prompts would park the
// session there, TEST-013); the plan file is edited by its absolute ~/.gogo/ path, so
// anchoring at a source is safe. With no sources (rare) it falls back to the project home.
// The launch returns a planAuthorLaunchedMsg so Update can ATTACH the user in.
func (m Model) finishPlanWithClaude() (tea.Model, tea.Cmd) {
	goal, title, chosen, attach := "", "", "", ""
	if m.binding != nil {
		goal = strings.TrimSpace(m.binding.planGoal)
		title = strings.TrimSpace(m.binding.planTitle)
		chosen = strings.TrimSpace(m.binding.planProject)
		attach = m.binding.planAttach
	}
	m.pendingPlanWithClaude = false
	m.binding = nil
	m.form = nil
	m.mode = modeBoard // renders the active tab (tabPlans)
	if m.project == nil {
		m.statusBlocked("no project — cannot author a plan")
		return m, nil
	}
	if goal == "" {
		m.status = "cancelled — no goal given, nothing created"
		return m, nil
	}
	if title == "" {
		title = deriveTitle(goal)
	}
	// FR1.5: switch the shared focus to the CHOSEN project BEFORE minting, so the plan
	// file, the knowledge dir, the source refs and the session anchor below all resolve
	// against the SAME project.
	m.focusChosenProject(chosen)
	// The DESCRIPTION is the goal, so the plan is never blank/"Untitled" and BriefFor /
	// the detail view have real content to show.
	p, err := plans.New(m.project.Name, title, goal)
	if err != nil {
		m.statusFailed("create failed: " + err.Error())
		return m, nil
	}
	atts := parseAttachments(attach)
	if len(atts) > 0 {
		if _, err := plans.SetAttachments(m.project.Name, p.ID, atts); err != nil {
			m.statusFailed("attachments failed: " + err.Error())
			return m, nil
		}
	}
	m.loadPlans()
	m.planColIdx = 0 // a fresh authored plan is a draft — focus the drafts column
	m.planCardIdx[0] = 0
	m.status = "created draft " + p.ID + " in " + m.project.Name + " — launching the analyst" // FR1.4

	// Seed a plain authoring session to flesh out the brief IN the plan's own file, NAMING
	// the goal so the analyst plans FOR IT. The whole prompt reaches claude as ONE trailing
	// argv element (AuthorPlanIntent — injection-safe); the correlation id rides in the prose
	// (already in front-matter), NOT as a --correlation flag (that is a /gogo:plan spawn param).
	planPath := plans.Path(m.project.Name, p.ID)
	// FR2: seed the author to READ the project's cross-repo .knowledge/ first, so the
	// whole-domain context flows into the brief (and each spawned work item's goal).
	intent := launch.AuthorPlanIntent(p.Title, goal, planPath, p.ID, projects.KnowledgeDir(m.project.Name), m.sourceRefs())

	// Anchor at a real source root (trusted repo) so the session doesn't park on a
	// first-run trust prompt for the ~/.gogo/ home. No source yet → fall back to the
	// project home; the plan file is edited by its absolute path regardless. Carry the
	// anchor heads-up (REV-008) so the headless path can still warn about the trust prompt.
	root, atSource := m.firstSourcePath()
	homeNote := ""
	if !atSource {
		root = projects.Dir(m.project.Name)
		homeNote = "no source to anchor at — the session runs in the project home; approve it if Claude prompts"
	}
	intent.Root = root
	// FR5.2: name the plan's attachments to the launched session — a bounded
	// decorator, applied BEFORE the fold so the budget check measures the real
	// command. An empty set returns the intent unchanged, byte-for-byte.
	intent = launch.WithAttachments(intent, atts)
	// FR1.3: a pasted multi-KB goal blows tmux's command budget exactly like a plan
	// body does. It is ALREADY the plan file's body (plans.New wrote it above), so
	// fold to a whole-file pointer - no section, the goal IS the body. No-op under
	// budget, so a normal goal launches byte-for-byte as before.
	intent = launch.FoldToPointer(intent, planPath, "")
	launcher := m.launcher

	return m, func() tea.Msg {
		res, err := launcher(root, intent)
		if err != nil {
			return launchDoneMsg{status: "plan-with-claude failed: " + err.Error(), level: statusLevelErr}
		}
		// Hand the created session name to Update so it can ATTACH the user in (tmux) or
		// surface the headless status (no tmux → res.Session == "") — naming the log path
		// (REV-006) + the no-source anchor note (REV-008) so a stalled headless run is
		// diagnosable.
		return planAuthorLaunchedMsg{session: res.Session, logPath: res.LogPath, homeNote: homeNote}
	}
}

// deriveTitle makes a short plan title from the goal's first non-blank line, trimmed to
// ~50 chars (word-safe when possible) — the default when the `A` form's title is left
// blank. Never empty (a goal that is all blank lines yields "Untitled plan", but the caller
// only reaches here with a non-empty goal).
func deriveTitle(goal string) string {
	first := "Untitled plan"
	for _, ln := range strings.Split(goal, "\n") {
		if s := strings.TrimSpace(ln); s != "" {
			first = s
			break
		}
	}
	// Cut on RUNES, not bytes (REV-005): a >50-byte multibyte first line with no late
	// ASCII space (e.g. Japanese/Polish) byte-sliced at maxLen split a rune and shipped
	// an INVALID-UTF-8 title. []rune(first)[:maxRunes] never splits a rune. The word-safe
	// LastIndex runs over an already rune-safe slice and space is single-byte, so cut[:i]
	// stays a valid rune boundary.
	const maxRunes = 50
	r := []rune(first)
	if len(r) <= maxRunes {
		return first
	}
	cut := strings.TrimRight(string(r[:maxRunes]), " ")
	if i := strings.LastIndex(cut, " "); i > 20 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ") + "…"
}

// sourceRefs returns the focused project's sources as label+absolute-path pairs in
// order — what the analyst-grade plan-with-claude session (0.25.0 FR1) needs to READ +
// ANALYZE each source repo (by path) and key its per-source brief (by label). Nil in
// single-repo mode (no project).
func (m *Model) sourceRefs() []launch.SourceRef {
	if m.project == nil {
		return nil
	}
	out := make([]launch.SourceRef, 0, len(m.project.Sources))
	for _, s := range m.project.Sources {
		label := s.Name
		if label == "" {
			label = filepath.Base(s.Path)
		}
		out = append(out, launch.SourceRef{Label: label, Path: s.Path})
	}
	return out
}

// firstSourcePath returns the focused project's first source path — a trusted repo
// root to anchor an author session at — and true, or ("", false) when the project has
// no sources yet.
func (m *Model) firstSourcePath() (string, bool) {
	if m.project == nil || len(m.project.Sources) == 0 {
		return "", false
	}
	return m.project.Sources[0].Path, true
}

// viewPlans renders the plans tab (plans-board FR1): the plan detail when one is open,
// else the 4-column KANBAN. Pure / substring-assertable (no TTY under go test → lipgloss
// emits plain text).
func (m Model) viewPlans() string {
	if m.planDetail != nil {
		return m.viewPlanDetail()
	}
	return m.viewPlansBoard()
}

// viewPlansBoard renders the plans KANBAN (plans-board FR1): four columns
// (drafts·ready·active·done) reusing the work board's column width + vertical separators
// + card box styles, with a contextual help line below. Each column windows its plan
// cards (reflowPlanColumns) with the focused card highlighted.
func (m Model) viewPlansBoard() string {
	colWidth := m.boardColWidth()
	rendered := make([]string, 4)
	for i := 0; i < 4; i++ {
		rendered[i] = m.renderPlanColumn(i, colWidth)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, interleaveSeparators(rendered)...)
	var parts []string
	// FR2.2: the tab can never again be silent about which project it shows — a
	// project header row (color dot + name + the switch key), mirroring
	// viewConfigLeft's `project  (p to switch)` line.
	if m.project != nil {
		parts = append(parts, m.projectDot(m.project.Name)+" "+colTitleStyle.Render(m.project.Name)+dimStyle.Render("  (p to switch)"), "")
	}
	parts = append(parts, body)
	if m.status != "" {
		parts = append(parts, m.renderStatus(m.status))
	}
	// FR2.5: every key updatePlanList handles appears here - TestPlansTabKeyHelpInSync
	// derives the switch's cases and fails if one is missing.
	help := lipgloss.NewStyle().Faint(true).Render("←→ cols · ↑↓ cards · enter open · v view · w web · n new · A plan-with-claude · m move (ready→go→done) · p switch project · x delete · tab board/config · q quit")
	parts = append(parts, help)
	return strings.Join(parts, "\n")
}

// renderPlanColumn renders one kanban column (plans-board FR1): its header + windowed
// plan cards, mirroring the work board's renderColumn (same width, windowing, and
// per-column card box styles). An empty column shows `(none)`.
func (m Model) renderPlanColumn(i, colWidth int) string {
	col := m.planCols[i]
	if len(col) == 0 {
		parts := []string{m.planColumnHeader(i, ""), "", dimStyle.Render("(none)")}
		return lipgloss.NewStyle().Width(colWidth).Render(strings.Join(parts, "\n"))
	}
	cardW := colWidth - 4
	if cardW < 14 {
		cardW = 14
	}
	cards := make([]string, len(col))
	heights := make([]int, len(col))
	for j := range col {
		focused := i == m.planColIdx && j == m.planCardIdx[i]
		cards[j] = m.renderPlanCard(i, col[j], focused, cardW)
		heights[j] = lipgloss.Height(cards[j])
	}
	start, end := 0, len(col)
	if m.height > 0 {
		avail := m.planColAvail() // the kanban's own budget — the header row is chrome (REV-005)
		if avail < 1 {
			avail = 1
		}
		start = clamp(m.planColOffset[i], 0, len(col)-1)
		end = fitEnd(heights, start, avail)
	}
	hint := ""
	if start > 0 || end < len(col) {
		hint = fmt.Sprintf("%d–%d", start+1, end)
	}
	parts := []string{m.planColumnHeader(i, hint), ""}
	if start > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
	}
	parts = append(parts, cards[start:end]...)
	if below := len(col) - end; below > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
	}
	return lipgloss.NewStyle().Width(colWidth).Render(strings.Join(parts, "\n"))
}

// planColumnHeader renders a kanban column's title + count + a ▸ focus marker + an
// optional overflow hint — the plans-tab analog of columnHeader (the same per-column
// accent header style, so the two boards read alike).
func (m Model) planColumnHeader(i int, hint string) string {
	st := columnStyles[i]
	marker := "  "
	if i == m.planColIdx {
		marker = "▸ "
	}
	head := marker + st.header.Underline(true).Render(planColumnTitles[i]) + dimStyle.Render(fmt.Sprintf(" %d", len(m.planCols[i])))
	if hint != "" {
		head += dimStyle.Render(" · " + hint)
	}
	return head
}

// renderPlanCard draws one plan as a bordered card (plans-board FR1) reusing the work
// board's per-column card box styles: the plan title + its ⛓ plan-XXXX chip on the name
// row, and the status-specific meta (draft → `draft · edited <ago>`; ready/active/done →
// `K of M work items` + the per-source dot strip, or the `awaiting-project-uat` cue) on
// the second row. ONE styled body for every state (FR5, mirroring the work board): the
// focused card changes only its FRAME (the double-line focus border). A member work item
// blocked by its source's cap surfaces the "needs manual trigger" cue (plans-board FR6).
func (m Model) renderPlanCard(colIdx int, p plans.Plan, focused bool, width int) string {
	title := p.Title
	if title == "" {
		title = "(untitled)"
	}
	textW := width - 2
	chipPlain := "⛓ " + p.ID
	// Name row: title left, ⛓ chip right-aligned; the chip drops when the card is too
	// narrow to keep the title readable.
	var head string
	titleBudget := textW - lipgloss.Width(chipPlain) - 1
	if titleBudget >= 8 {
		head = placeApart(slugStyle.Render(truncate(title, titleBudget)), correlationChipStyle.Render(chipPlain), textW)
	} else {
		head = slugStyle.Render(truncate(title, textW))
	}
	meta := m.planCardMeta(p)
	if cue := m.planPickupCue(p); cue != "" {
		meta += "  " + cue
	}
	body := strings.Join([]string{head, meta}, "\n")
	style := columnStyles[colIdx].card
	if focused {
		style = columnStyles[colIdx].cardFocused
	}
	return style.Width(width).Render(body)
}

// planCardMeta is the plan card's trailing metadata (FR10): a DRAFT shows the
// `draft · edited <ago>` nicety; a ready/active plan shows `K of M work items` plus the
// per-source dot strip (colored ● once a source is spawned, dim `·` until then).
// Always styled — the focused card changes only its frame now (FR5).
func (m Model) planCardMeta(p plans.Plan) string {
	if p.Status == plans.StatusDraft {
		s := "draft"
		if ago := relAgo(p.Created); ago != "" {
			s += " · edited " + ago
		}
		return dimStyle.Render(s)
	}
	// A plan whose every member work item is shipped is at the project-UAT gate — flag
	// it `awaiting-project-uat` on the card (distinct from a still-building `active`),
	// so the ACTIVE section makes the ready-to-accept plan visible at a glance (FR3).
	if m.planDerivedStatus(p) == plans.StatusAwaitingProjectUAT {
		return statusStyle(plans.StatusAwaitingProjectUAT + " · press m")
	}
	created := 0
	for _, t := range p.Targets {
		if m.spawnedFeature(t, p.ID) != nil {
			created++
		}
	}
	count := dimStyle.Render(fmt.Sprintf("%d of %d work items", created, len(p.Targets)))
	if dots := m.planSourceDots(p); dots != "" {
		return count + "   " + dots
	}
	return count
}

// planSourceDots renders the plan card's per-source dot strip (FR10/FR11): one dot per
// target source — the source's colored ● once a work item carrying the plan id is
// spawned into it, else a dim `·` ("not created" yet, spelled out in the plan detail).
// Empty when the plan has no targets. Always tinted — the focused card keeps its
// colours now (FR5).
func (m Model) planSourceDots(p plans.Plan) string {
	if len(p.Targets) == 0 {
		return ""
	}
	dots := make([]string, len(p.Targets))
	for i, t := range p.Targets {
		if m.spawnedFeature(t, p.ID) == nil {
			dots[i] = dimStyle.Render("·")
		} else {
			dots[i] = m.sourceDot(t) // source-colored ●
		}
	}
	return strings.Join(dots, " ")
}

// relAgo renders an RFC3339 timestamp as a compact relative age ("3d", "5h", "just
// now"), or "" when empty/unparseable — the drafts-section `edited <ago>` nicety (FR10).
// Best-effort; never a hard dependency on a specific clock value.
func relAgo(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	switch d := time.Since(t); {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// viewPlanDetail renders a plan's detail (plans-board FR4): breadcrumb + ⛓ chip,
// description, and the WORK ITEMS list — each row a source dot + name + (work-item slug +
// its live status pill) OR (slug:<hint> + ＋ create work item).
func (m Model) viewPlanDetail() string {
	p := m.planDetail
	title := p.Title
	if title == "" {
		title = "(untitled)"
	}
	// Derive the DISPLAY status (FR3): an active plan with every member shipped reads
	// `awaiting-project-uat` (distinct from `active`); a done plan reads `done`.
	derived := m.planDerivedStatus(*p)
	var b []string
	b = append(b,
		colTitleStyle.Render("plans / "+title)+"   "+correlationChipStyle.Render("⛓ "+p.ID),
		dimStyle.Render("status  ")+derived,
		"",
	)
	// Project-UAT affordance: when every member is shipped, the plan is at the
	// project-UAT gate — spell out the shipped tally + the `m` accept move (plans-board FR2).
	if derived == plans.StatusAwaitingProjectUAT {
		b = append(b,
			statusStyle(fmt.Sprintf("all %d work item(s) shipped - press m to accept the project-UAT (→ done)", len(p.Members))),
			"",
		)
	}
	desc := p.Description
	if strings.TrimSpace(desc) == "" {
		desc = dimStyle.Render("(no description — edit the plan file with e)")
	}
	b = append(b, desc, "")
	// FR4.5: the plan's attachments, one row per entry — a local path that no longer
	// exists is marked `· missing` (attachments are referenced, never copied — D3).
	if len(p.Attachments) > 0 {
		b = append(b, colTitleStyle.Render("ATTACHMENTS"))
		for _, a := range p.Attachments {
			row := "  " + a
			if !strings.HasPrefix(a, "http://") && !strings.HasPrefix(a, "https://") && !fileExists(a) {
				row += dimStyle.Render(" · missing")
			}
			b = append(b, row)
		}
		b = append(b, "")
	}
	b = append(b, colTitleStyle.Render("WORK ITEMS"))

	if len(p.Targets) == 0 {
		b = append(b, dimStyle.Render("  (no target sources — press + to add one)"))
	}
	hint := planSlugHint(p.Title)
	for i, sourceName := range p.Targets {
		cursor := "  "
		if i == m.planSourceIdx {
			cursor = "▸ "
		}
		if f := m.spawnedFeature(sourceName, p.ID); f != nil {
			// Spawned: solid source-colored ● + the work item's slug + its status pill.
			row := fmt.Sprintf("%s%s %-14s %s  %s", cursor, m.sourceDot(sourceName), sourceName, slugStyle.Render(f.Slug), pillStyleFor(f).Render(pillLabel(f)))
			b = append(b, row)
		} else {
			// Not spawned yet: a greyed `·` dot + a `· not created` note + the ＋ affordance.
			row := fmt.Sprintf("%s%s %-14s %s  %s", cursor, dimStyle.Render("·"), sourceName,
				dimStyle.Render("· not created · slug:"+hint), keyChipStyle.Render("＋ create work item"))
			b = append(b, row)
		}
	}

	if m.status != "" {
		b = append(b, "", m.renderStatus(m.status))
	}
	// FR2.5: guarded against updatePlanDetail's switch by TestPlansTabKeyHelpInSync.
	help := lipgloss.NewStyle().Faint(true).Render("↑↓ · c create item · + add source · v view · w web · m move (ready→go→done) · e edit plan · esc back")
	b = append(b, "", help)
	return strings.Join(b, "\n")
}

// containsString reports whether ss contains want (exact).
func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// sourceDot is the small colored origin dot the plans tab / changelog / filter chips
// prefix a source with — always the source's never-blank palette color (cockpit-colors
// FR2), dropping the old grey "no color" fallback.
func (m Model) sourceDot(sourceName string) string {
	return lipgloss.NewStyle().Foreground(m.sourceColor(sourceName)).Render("●")
}

// projectDot is the small colored origin dot the board project-filter chips prefix a
// project with — always the project's never-blank palette color (FR3), mirroring
// sourceDot over the project palette.
func (m Model) projectDot(projectName string) string {
	return lipgloss.NewStyle().Foreground(m.projectColor(projectName)).Render("●")
}
