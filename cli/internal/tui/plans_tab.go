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
// plan-with-claude · r mark-ready · x delete.
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
		if p := m.focusedPlan(); p != nil && m.project != nil {
			if _, err := plans.MarkReady(m.project.Name, p.ID); err != nil {
				m.status = "mark-ready failed: " + err.Error()
			} else {
				m.loadPlans()
				m.status = "marked " + p.ID + " ready"
			}
		}
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

// startPlanForm opens the huh new-plan form (FR10 `n`): a single title input under
// modeForm. It marks pendingPlan (so updateForm routes completion to finishPlanForm
// and a cancel returns to the plans tab) and binds the title through a heap-stable
// *formBinding (TEST-001).
func (m *Model) startPlanForm() {
	m.pendingPlan = true
	m.binding = &formBinding{}
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("New plan title").
			Description("creates a draft plan in " + m.project.Name + " — target sources + spawn from its detail").
			Value(&m.binding.planTitle),
	))
	m.mode = modeForm
}

// finishPlanForm applies a completed new-plan form: a non-blank title creates a
// draft plan in the focused project (a write to ~/.gogo/ only), reloads the list,
// and lands back on the plans tab.
func (m Model) finishPlanForm() (tea.Model, tea.Cmd) {
	title := ""
	if m.binding != nil {
		title = strings.TrimSpace(m.binding.planTitle)
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
	p, err := plans.New(m.project.Name, title, "")
	if err != nil {
		m.status = "create failed: " + err.Error()
		return m, nil
	}
	m.loadPlans()
	m.planIdx = 0
	m.status = "created draft " + p.ID
	return m, nil
}

// planWithClaude is the plans-tab `A` plan-with-claude authoring trigger (FR-D — the
// user's "start a claude session and prepare a plan" ask). It authors the PROJECT
// PLAN, not a source work item: it MINTS a fresh draft plan up front (so its
// plan-<hash> correlation id exists in ~/.gogo/projects/<name>/.gogo/plans/<id>.md
// before anything spawns), then fires — through the launcher seam, EXACTLY ONCE — a
// PLAIN interactive `claude` session (NOT a slash command) seeded by
// launch.AuthorPlanIntent to READ + EDIT that plan file IN PLACE. It deliberately does
// NOT launch /gogo:plan: that skill's Step 1 unconditionally scaffolds a source
// `.gogo/work/feature-<slug>/`, the wrong thing for a project-plan file (and the
// D3-rejected "prose is ignorable" failure). The session is ANCHORED at the focused
// project's FIRST SOURCE root — a real repo the user already trusts in Claude — NOT the
// untrusted `~/.gogo/projects/<name>/` home (first-run trust prompts would park the
// session there, TEST-013); the plan file is edited by its absolute ~/.gogo/ path, so
// anchoring at a source is safe and never touches that source's `.gogo/work/`. With no
// sources yet (rare), it falls back to the project home with a note. `n` stays the
// quick inline draft.
func (m Model) planWithClaude() (tea.Model, tea.Cmd) {
	if m.project == nil {
		m.status = "no project — cannot author a plan"
		return m, nil
	}
	if !m.hasClaude {
		m.status = "claude CLI not on PATH — cannot start a plan-with-claude session (use `n` for a quick draft)"
		return m, nil
	}
	p, err := plans.New(m.project.Name, "Untitled plan", "")
	if err != nil {
		m.status = "create failed: " + err.Error()
		return m, nil
	}
	m.loadPlans()
	m.planIdx = 0

	// Seed a plain authoring session to flesh out the brief IN the plan's own file. The
	// whole prompt reaches claude as ONE trailing argv element (AuthorPlanIntent —
	// injection-safe); the correlation id rides in the prose (already in front-matter),
	// NOT as a --correlation flag (that is a /gogo:plan spawn param, not a plain session).
	planPath := plans.Path(m.project.Name, p.ID)
	// FR2: seed the author to READ the project's cross-repo .knowledge/ first, so the
	// whole-domain context flows into the brief (and each spawned work item's goal).
	intent := launch.AuthorPlanIntent(p.Title, planPath, p.ID, projects.KnowledgeDir(m.project.Name), m.sourceNames())

	// Anchor at a real source root (trusted repo) so the session doesn't park on a
	// first-run trust prompt for the ~/.gogo/ home. No source yet → fall back to the
	// project home (with a note); the plan file is edited by its absolute path regardless.
	root, atSource := m.firstSourcePath()
	if !atSource {
		root = projects.Dir(m.project.Name)
		m.status = "authoring " + p.ID + " (no source to anchor at — session runs in the project home; approve it if Claude prompts)"
	}
	launcher := m.launcher

	return m, func() tea.Msg {
		res, err := launcher(root, intent)
		if err != nil {
			return launchDoneMsg{status: "plan-with-claude failed: " + err.Error()}
		}
		where := res.Session
		if where == "" {
			where = res.LogPath
		}
		return launchDoneMsg{status: "authoring " + p.ID + " with claude → " + res.Command + " (" + where + ")"}
	}
}

// sourceNames returns the focused project's source labels (Name, else the path
// basename) in order — the choice of targets the plan-with-claude author is offered in
// the seeded prompt. Nil in single-repo mode (no project).
func (m *Model) sourceNames() []string {
	if m.project == nil {
		return nil
	}
	out := make([]string, 0, len(m.project.Sources))
	for _, s := range m.project.Sources {
		label := s.Name
		if label == "" {
			label = filepath.Base(s.Path)
		}
		out = append(out, label)
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
	help := lipgloss.NewStyle().Faint(true).Render("↑↓ · enter open · n new · A plan-with-claude · r ready · D accept UAT · x delete · tab board/config · q quit")
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
