package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
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

// planSections fixes the plans-tab section order (lifecycle order, FR10) and maps
// each section header to its plan status.
var planSections = [3]struct {
	title  string
	status string
}{
	{"ACTIVE", plans.StatusActive},
	{"READY", plans.StatusReady},
	{"DRAFTS", plans.StatusDraft},
}

var planSlugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

// planSlugHint derives the advisory kebab feature slug a spawn pins as the member
// hint (the analyst derives the real slug; the correlation id is the exact link).
func planSlugHint(title string) string {
	s := planSlugUnsafe.ReplaceAllString(strings.ToLower(title), "-")
	if s = strings.Trim(s, "-"); s == "" {
		s = "plan"
	}
	return s
}

// groupedPlans flattens the project's plans into the plans-tab display order
// (ACTIVE, then READY, then DRAFTS — each newest-first, since m.plans is already
// newest-first). `done` plans are terminal and omitted from the three sections.
func (m *Model) groupedPlans() []plans.Plan {
	var out []plans.Plan
	for _, sec := range planSections {
		for _, p := range m.plans {
			if p.Status == sec.status {
				out = append(out, p)
			}
		}
	}
	return out
}

// focusedPlan returns the plan under the list cursor (over groupedPlans), or nil.
func (m *Model) focusedPlan() *plans.Plan {
	g := m.groupedPlans()
	if m.planIdx < 0 || m.planIdx >= len(g) {
		return nil
	}
	return &g[m.planIdx]
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

// planCounts returns the ACTIVE/READY/DRAFT counts for the plans-tab header.
func (m *Model) planCounts() (active, ready, draft int) {
	for _, p := range m.plans {
		switch p.Status {
		case plans.StatusActive:
			active++
		case plans.StatusReady:
			ready++
		case plans.StatusDraft:
			draft++
		}
	}
	return active, ready, draft
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

// updatePlanList handles the plans list keys: ↑↓ nav · enter open · n new · A
// plan-with-claude · r accept (mark-ready + auto-spawn) · x delete.
func (m Model) updatePlanList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	g := m.groupedPlans()
	switch msg.String() {
	case "up", "k":
		m.planIdx = clamp(m.planIdx-1, 0, len(g)-1)
	case "down", "j":
		m.planIdx = clamp(m.planIdx+1, 0, len(g)-1)
	case "enter", "right", "l":
		if p := m.focusedPlan(); p != nil {
			cp := *p
			m.planDetail = &cp
			m.planSourceIdx = 0
			m.status = ""
		}
	case "n":
		if m.project != nil {
			m.startPlanForm()
			return m, m.form.Init()
		}
	case "A":
		return m.planWithClaude()
	case "D":
		return m.planAcceptUAT(m.focusedPlan())
	case "r":
		return m.planReadyAndSpawn()
	case "x":
		if p := m.focusedPlan(); p != nil && m.project != nil {
			id := p.ID
			if _, err := plans.Delete(m.project.Name, id); err != nil {
				m.status = "delete failed: " + err.Error()
			} else {
				m.loadPlans()
				m.planIdx = clamp(m.planIdx, 0, len(m.groupedPlans())-1)
				m.status = "deleted " + id
			}
		}
	}
	return m, nil
}

// updatePlanDetail handles the plan-detail keys: ↑↓ target nav · c create work item
// · + add source · e edit plan · esc/q/← back to the list.
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
	case "D":
		return m.planAcceptUAT(p)
	case "e":
		m.status = "edit the plan by hand at " + plans.Path(m.project.Name, p.ID) + " (or `gogo plan show " + p.ID + "`)"
	}
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
		m.status = "plan " + p.ID + " is already done (project-UAT accepted)"
		return m, nil
	}
	allShipped, unshipped := plans.MembersShippedIn(m.project.Name, *p, m.repo)
	if !allShipped {
		if len(p.Members) == 0 {
			m.status = "refusing — plan " + p.ID + " has no work items yet; spawn + ship members first (c)"
		} else {
			m.status = fmt.Sprintf("refusing — %d of %d member(s) not shipped: %s",
				len(unshipped), len(p.Members), strings.Join(unshipped, ", "))
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
	b := &formBinding{}
	m.binding = b
	m.form = huh.NewForm(huh.NewGroup(
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
// The now-`done` plan drops out of the ACTIVE/READY/DRAFTS sections.
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
		m.status = "no plan " + edit.id + " in " + edit.project
		return m, nil
	}
	if allShipped, unshipped := plans.MembersShippedIn(edit.project, p, m.repo); !allShipped {
		m.status = fmt.Sprintf("refusing — %d member(s) not shipped: %s", len(unshipped), strings.Join(unshipped, ", "))
		return m, nil
	}
	if _, err := plans.MarkDone(edit.project, edit.id); err != nil {
		m.status = "accept failed: " + err.Error()
		return m, nil
	}
	m.loadPlans()
	m.planDetail = nil
	m.planIdx = clamp(m.planIdx, 0, len(m.groupedPlans())-1)
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
		m.status = "no target source selected — add one with +"
		return m, nil
	}
	sourceName := p.Targets[m.planSourceIdx]
	src := m.sourceByName(sourceName)
	if src == nil {
		m.status = "source " + sourceName + " is not in this project"
		return m, nil
	}
	if !m.hasClaude {
		m.status = "claude CLI not on PATH — cannot spawn a work item"
		return m, nil
	}
	body := p.Description
	if strings.TrimSpace(body) == "" {
		body = p.Title
	}
	intent := launch.PlanIntent(p.Title, body, p.ID)
	root := src.Path
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
			// Launch failed → leave the plan UNTOUCHED (no phantom active member).
			return launchDoneMsg{status: "spawn failed: " + err.Error()}
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

// planReadyAndSpawn is the plans-tab `r` ACCEPT step (0.25.0 FR2, D3=a). It overloads
// the old plain draft→ready flip to AUTO-SPAWN a work item into each target the analyst
// chose:
//   - A TARGETLESS plan → today's plain MarkReady (zero launches), byte-for-byte.
//   - A plan with ≥1 target → open a huh confirm listing the UN-spawned targets; on
//     accept, finishPlanSpawn loops the fire-once launcher seam (mirroring
//     planCreateWorkItem) once per un-spawned target with its per-source brief + skip.
//   - Every target already spawned → a no-op status (the plan is already active); a
//     re-`r` never re-launches (idempotent).
//
// Spawning needs claude on PATH; without it the targeted path surfaces a hint (a
// targetless plan still marks ready). c (spawn one focused target) stays the manual
// fallback, unchanged.
func (m Model) planReadyAndSpawn() (tea.Model, tea.Cmd) {
	p := m.focusedPlan()
	if p == nil || m.project == nil {
		return m, nil
	}
	// A targetless (hand-authored / n-drafted) plan spawns nothing — today's MarkReady.
	if len(p.Targets) == 0 {
		if _, err := plans.MarkReady(m.project.Name, p.ID); err != nil {
			m.status = "mark-ready failed: " + err.Error()
		} else {
			m.loadPlans()
			m.status = "marked " + p.ID + " ready"
		}
		return m, nil
	}
	todo := m.unspawnedTargets(*p)
	if len(todo) == 0 {
		m.status = fmt.Sprintf("all %d target(s) already spawned for %s", len(p.Targets), p.ID)
		return m, nil
	}
	if !m.hasClaude {
		m.status = "claude CLI not on PATH — cannot spawn work items (use `gogo plan promote`, or `c` per source)"
		return m, nil
	}
	m.startPlanSpawnForm(p, todo)
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
// *formBinding (TEST-001). Reached only with ≥1 un-spawned target + claude on PATH.
func (m *Model) startPlanSpawnForm(p *plans.Plan, targets []string) {
	m.pendingPlanSpawn = &planSpawnEdit{project: m.project.Name, id: p.ID, title: p.Title, targets: targets}
	b := &formBinding{}
	m.binding = b
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Accept %s and spawn %d work item(s)?", p.ID, len(targets))).
			Description("into: " + strings.Join(targets, ", ") + " — launches /gogo:plan per source, records members, flips the plan active").
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
		m.status = "no plan " + edit.id + " in " + edit.project
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
	for _, target := range edit.targets {
		src := m.sourceByName(target)
		if src == nil {
			continue // source vanished from the project — skip (never a phantom member)
		}
		goal := plans.BriefFor(p, target)
		if strings.TrimSpace(goal) == "" {
			goal = body
		}
		intent := launch.PlanIntent(p.Title, goal, p.ID)
		// Ride the skip flag of the source ALREADY in hand (m.sourceByName scopes to the
		// FOCUSED project), not a first-path-match across EVERY project's sources (REV-001):
		// a repo linked to two projects with opposite PlanAcceptanceSkip must carry the
		// focused project's flag, never whichever identically-pathed source sorts first.
		intent.Command += launch.SkipParams(src.PlanAcceptanceSkip, false)
		spawns = append(spawns, spawn{source: target, root: src.Path, intent: intent})
	}
	if len(spawns) == 0 {
		m.status = "no spawnable targets for " + edit.id
		return m, nil
	}
	launcher := m.launcher
	project := edit.project
	planID := edit.id
	slugHint := planSlugHint(p.Title)

	return m, func() tea.Msg {
		launched, failed := 0, 0
		for _, s := range spawns {
			if _, err := launcher(s.root, s.intent); err != nil {
				failed++
				continue // leave this target un-recorded (no phantom member, REV-005)
			}
			plans.AddMember(project, planID, plans.Member{Source: s.source, SlugHint: slugHint})
			plans.SetStatus(project, planID, plans.StatusActive)
			launched++
		}
		status := fmt.Sprintf("accepted %s — spawned %d work item(s)", planID, launched)
		if failed > 0 {
			status += fmt.Sprintf(" (%d failed)", failed)
		}
		return launchDoneMsg{status: status}
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
			m.status = "add source failed: " + err.Error()
			return m, nil
		}
		m.loadPlans()
		if updated, ok := plans.Get(m.project.Name, p.ID); ok {
			m.planDetail = &updated
		}
		m.status = "added source " + label
		return m, nil
	}
	m.status = "every source is already a target"
	return m, nil
}

// startPlanForm opens the huh new-plan form (FR10 `n`): a title input plus an optional
// DESCRIPTION textarea (0.25.1 — so a quick draft can carry a real brief, not only a bare
// title) under modeForm. It marks pendingPlan (so updateForm routes completion to
// finishPlanForm and a cancel returns to the plans tab) and binds both fields through a
// heap-stable *formBinding (TEST-001).
func (m *Model) startPlanForm() {
	m.pendingPlan = true
	m.binding = &formBinding{}
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("New plan title").
			Description("creates a draft plan in "+m.project.Name+" — target sources + spawn from its detail").
			Value(&m.binding.planTitle),
		huh.NewText().
			Title("Description (optional)").
			Description("the plan's goal / brief — shown in the plan detail; edit later with e").
			Lines(4).
			Value(&m.binding.planDesc),
	))
	m.mode = modeForm
}

// finishPlanForm applies a completed new-plan form: a non-blank title creates a
// draft plan in the focused project (a write to ~/.gogo/ only) carrying the optional
// description, reloads the list, and lands back on the plans tab.
func (m Model) finishPlanForm() (tea.Model, tea.Cmd) {
	title, desc := "", ""
	if m.binding != nil {
		title = strings.TrimSpace(m.binding.planTitle)
		desc = strings.TrimSpace(m.binding.planDesc)
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
		m.status = "no project — cannot create a plan"
		return m, nil
	}
	p, err := plans.New(m.project.Name, title, desc)
	if err != nil {
		m.status = "create failed: " + err.Error()
		return m, nil
	}
	m.loadPlans()
	m.planIdx = 0
	m.status = "created draft " + p.ID
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
		m.status = "no project — cannot author a plan"
		return m, nil
	}
	if !m.hasClaude {
		m.status = "claude CLI not on PATH — cannot start a plan-with-claude session (use `n` for a quick draft)"
		return m, nil
	}
	m.startPlanWithClaudeForm()
	return m, m.form.Init()
}

// startPlanWithClaudeForm opens the huh `A` goal form (0.25.1) under modeForm: a required
// GOAL textarea (what to build/change across the project's sources) plus an optional short
// title (derived from the goal when blank). It marks pendingPlanWithClaude (so updateForm
// routes completion to finishPlanWithClaude and a cancel mints NOTHING) and binds both
// fields through a heap-stable *formBinding (TEST-001).
func (m *Model) startPlanWithClaudeForm() {
	m.pendingPlanWithClaude = true
	m.binding = &formBinding{}
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewText().
			Title("What should gogo plan for this project? (the goal — what to build or change across its sources)").
			Description("the analyst reads the project's sources and writes the plan for this goal").
			Lines(5).
			Value(&m.binding.planGoal),
		huh.NewInput().
			Title("Plan title (optional)").
			Description("defaults to a short title derived from the goal").
			Value(&m.binding.planTitle),
	))
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
	goal, title := "", ""
	if m.binding != nil {
		goal = strings.TrimSpace(m.binding.planGoal)
		title = strings.TrimSpace(m.binding.planTitle)
	}
	m.pendingPlanWithClaude = false
	m.binding = nil
	m.form = nil
	m.mode = modeBoard // renders the active tab (tabPlans)
	if m.project == nil {
		m.status = "no project — cannot author a plan"
		return m, nil
	}
	if goal == "" {
		m.status = "cancelled — no goal given, nothing created"
		return m, nil
	}
	if title == "" {
		title = deriveTitle(goal)
	}
	// The DESCRIPTION is the goal, so the plan is never blank/"Untitled" and BriefFor /
	// the detail view have real content to show.
	p, err := plans.New(m.project.Name, title, goal)
	if err != nil {
		m.status = "create failed: " + err.Error()
		return m, nil
	}
	m.loadPlans()
	m.planIdx = 0

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
	launcher := m.launcher

	return m, func() tea.Msg {
		res, err := launcher(root, intent)
		if err != nil {
			return launchDoneMsg{status: "plan-with-claude failed: " + err.Error()}
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

// viewPlans renders the plans tab (FR10/FR11): the plan detail when one is open, else
// the grouped list. Pure / substring-assertable (no TTY under go test → lipgloss
// emits plain text).
func (m Model) viewPlans() string {
	if m.planDetail != nil {
		return m.viewPlanDetail()
	}
	active, ready, draft := m.planCounts()
	header := colTitleStyle.Render("plans") + "  " +
		dimStyle.Render(fmt.Sprintf("%d active · %d ready · %d drafts", active, ready, draft))
	parts := []string{header, ""}

	idx := 0 // running index into groupedPlans (== the planIdx cursor space)
	for _, sec := range planSections {
		parts = append(parts, colTitleStyle.Render(sec.title))
		any := false
		for _, p := range m.plans {
			if p.Status != sec.status {
				continue
			}
			any = true
			parts = append(parts, m.planCardRow(p, idx == m.planIdx))
			idx++
		}
		if !any {
			parts = append(parts, dimStyle.Render("  (none)"))
		}
		parts = append(parts, "")
	}

	if m.status != "" {
		parts = append(parts, statusStyle(m.status), "")
	}
	help := lipgloss.NewStyle().Faint(true).Render("↑↓ · enter open · n new · A plan-with-claude (goal → attach) · r accept+spawn · D accept UAT · x delete · tab board/config · q quit")
	parts = append(parts, help)
	return strings.Join(parts, "\n")
}

// planCardRow renders one plan in the list. The list cursor `▸ ` (present ONLY on the
// focused row) is the SINGLE focus indicator — the always-on `▸` glyph that used to
// double it (`▸ ▸ …`) is gone. A DRAFT keeps a dashed `◌` marker (FR10); active/ready
// carry none (their section header conveys status). Each card shows the ⛓ plan-XXXX
// chip and, per status, its trailing meta (drafts: `draft · edited <ago>`; ready/
// active: `K of M work items` + the per-source dot strip).
func (m Model) planCardRow(p plans.Plan, focused bool) string {
	glyph := " " // ready/active: no leading glyph (the cursor owns ▸; the section conveys status)
	if p.Status == plans.StatusDraft {
		glyph = "◌"
	}
	title := p.Title
	if title == "" {
		title = "(untitled)"
	}
	cursor := "  "
	if focused {
		cursor = "▸ "
	}
	if focused {
		// The focus fill carries one fg/bg, so render plain (no per-segment tints).
		return changelogFocusStyle.Render(fmt.Sprintf("%s%s %s   ⛓ %s   %s",
			cursor, glyph, title, p.ID, m.planCardMeta(p, true)))
	}
	return fmt.Sprintf("%s%s %s   %s   %s", cursor, glyph, slugStyle.Render(title),
		correlationChipStyle.Render("⛓ "+p.ID), m.planCardMeta(p, false))
}

// planCardMeta is the plan card's trailing metadata (FR10): a DRAFT shows the
// `draft · edited <ago>` nicety; a ready/active plan shows `K of M work items` plus the
// per-source dot strip (colored ● once a source is spawned, dim `·` until then). `plain`
// drops the tints for the focused row's single fg/bg fill.
func (m Model) planCardMeta(p plans.Plan, plain bool) string {
	if p.Status == plans.StatusDraft {
		s := "draft"
		if ago := relAgo(p.Created); ago != "" {
			s += " · edited " + ago
		}
		if plain {
			return s
		}
		return dimStyle.Render(s)
	}
	// A plan whose every member work item is shipped is at the project-UAT gate — flag
	// it `awaiting-project-uat` on the card (distinct from a still-building `active`),
	// so the ACTIVE section makes the ready-to-accept plan visible at a glance (FR3).
	if m.planDerivedStatus(p) == plans.StatusAwaitingProjectUAT {
		label := plans.StatusAwaitingProjectUAT + " · press D"
		if plain {
			return label
		}
		return statusStyle(label)
	}
	created := 0
	for _, t := range p.Targets {
		if m.spawnedFeature(t, p.ID) != nil {
			created++
		}
	}
	count := fmt.Sprintf("%d of %d work items", created, len(p.Targets))
	dots := m.planSourceDots(p, plain)
	if !plain {
		count = dimStyle.Render(count)
	}
	if dots != "" {
		return count + "   " + dots
	}
	return count
}

// planSourceDots renders the plan card's per-source dot strip (FR10/FR11): one dot per
// target source — the source's colored ● once a work item carrying the plan id is
// spawned into it, else a dim `·` ("not created" yet, spelled out in the plan detail).
// Empty when the plan has no targets. `plain` renders untinted for the focused row.
func (m Model) planSourceDots(p plans.Plan, plain bool) string {
	if len(p.Targets) == 0 {
		return ""
	}
	dots := make([]string, len(p.Targets))
	for i, t := range p.Targets {
		switch {
		case m.spawnedFeature(t, p.ID) == nil:
			dots[i] = "·"
			if !plain {
				dots[i] = dimStyle.Render("·")
			}
		case plain:
			dots[i] = "●"
		default:
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

// viewPlanDetail renders a plan's detail (FR11): breadcrumb + ⛓ chip, description,
// and the TARGET SOURCES list — each row a source dot + name + (work-item slug +
// status pill) OR (slug:<hint> + ＋ create work item).
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
	// project-UAT gate — spell out the shipped tally + the `D` accept key (FR3).
	if derived == plans.StatusAwaitingProjectUAT {
		b = append(b,
			statusStyle(fmt.Sprintf("all %d work item(s) shipped — press D to accept the project-UAT (→ done)", len(p.Members))),
			"",
		)
	}
	desc := p.Description
	if strings.TrimSpace(desc) == "" {
		desc = dimStyle.Render("(no description — edit the plan file with e)")
	}
	b = append(b, desc, "", colTitleStyle.Render("TARGET SOURCES"))

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
		b = append(b, "", statusStyle(m.status))
	}
	help := lipgloss.NewStyle().Faint(true).Render("↑↓ · c create item · + add source · D accept project-UAT · e edit plan · esc back")
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
