package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// --- config tab (FR9) — source-native, writes ONLY ~/.gogo/ via the projects store -

// updateConfig drives the config tab: `p` cycles the project switcher, ↑↓/jk move
// the per-source cursor, and a/e/x open the add/edit/remove huh form (under
// modeForm). The persistent keys (q quit · tab cycle · ?) are handled one level up
// in updateActive. It never writes — every mutation is deferred to finishSourceForm
// after the form is confirmed.
func (m Model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "p":
		// Cycle the focused home project (FR9 switcher). Re-aggregates the board so the
		// board tab reflects the switch too.
		m.switchProject(m.projIdx + 1)
		return m, nil
	case "c":
		// Edit the focused PROJECT's label color (cockpit-colors FR4). Source colors are
		// edited via the per-source e form's Color field.
		if m.project != nil {
			m.startProjectColorForm()
			return m, m.form.Init()
		}
	case "up", "k":
		m.sourceIdx = clamp(m.sourceIdx-1, 0, len(m.sources())-1)
	case "down", "j":
		m.sourceIdx = clamp(m.sourceIdx+1, 0, len(m.sources())-1)
	case "a":
		if m.project != nil {
			m.startSourceForm("add", nil)
			return m, m.form.Init()
		}
	case "e":
		if s := m.focusedSource(); s != nil {
			m.startSourceForm("edit", s)
			return m, m.form.Init()
		}
	case "x":
		if s := m.focusedSource(); s != nil {
			m.startSourceForm("remove", s)
			return m, m.form.Init()
		}
	}
	return m, nil
}

// startSourceForm opens the huh add/edit/remove per-source form under modeForm
// (FR9). It marks pendingSource (so updateForm routes completion to finishSourceForm
// and a cancel returns to the config tab) and binds every field through the heap-
// stable *formBinding string targets (TEST-001). Add starts blank with the cap +
// branch defaulted; edit seeds the fields from the focused source; remove is a single
// Confirm (defaulting to Cancel).
func (m *Model) startSourceForm(op string, s *projects.Source) {
	edit := &sourceEdit{op: op}
	if m.project != nil {
		edit.project = m.project.Name
	}
	b := &formBinding{}
	if s != nil {
		edit.origPath = s.Path
		b.srcName = s.Name
		b.srcPath = s.Path
		b.srcBranch = s.MainBranch
		b.srcColor = s.Color
		b.srcCap = strconv.Itoa(s.ConcurrentWorkItems)
	}
	m.pendingSource = edit
	m.binding = b

	if op == "remove" {
		label := b.srcName
		if label == "" {
			label = b.srcPath
		}
		m.form = huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title("Remove source " + label + "?").
				Description("removes it from " + edit.project + "'s config.json (~/.gogo/) — the repo's .gogo/ is untouched").
				Affirmative("Remove").
				Negative("Cancel").
				Value(&b.confirm),
		))
		m.mode = modeForm
		return
	}

	if op == "add" {
		b.srcCap = strconv.Itoa(projects.DefaultConcurrentWorkItems) // a new source defaults to a cap of 1
		if b.srcBranch == "" {
			b.srcBranch = "main"
		}
	}
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Path").Description("the repo/service root that contains .gogo/").Value(&b.srcPath),
		huh.NewInput().Title("Name").Description("display name (defaults to the folder name)").Value(&b.srcName),
		huh.NewInput().Title("Main branch").Description("the source's default branch").Value(&b.srcBranch),
		huh.NewInput().Title("Label color").Description("origin-dot hex or a swatch name (e.g. teal, #58a6ff) — blank auto-assigns").Value(&b.srcColor),
		huh.NewInput().Title("Concurrent work items").Description("0 = unlimited; N caps live in-progress features").Value(&b.srcCap),
	))
	m.mode = modeForm
}

// startProjectColorForm opens the huh project label-color form under modeForm
// (cockpit-colors FR4, `c`): a single input seeded from the focused project's Color,
// bound through the heap-stable *formBinding.projColor (TEST-001). Completion routes to
// finishProjectColorForm; a cancel returns to the config tab.
func (m *Model) startProjectColorForm() {
	p := m.project
	m.pendingProject = &projectEdit{name: p.Name}
	b := &formBinding{projColor: p.Color}
	m.binding = b
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Project label color — " + p.Name).
			Description("origin-dot hex or a swatch name (e.g. teal, #58a6ff) — the project's color in the switcher").
			Value(&b.projColor),
	))
	m.mode = modeForm
}

// finishProjectColorForm applies a completed project label-color form: it resolves a
// swatch NAME to its hex (or keeps a raw hex), persists Project.Color to config.json (a
// ~/.gogo/ write only), and re-tints the board live via refreshProject. Lands back on
// the config tab.
func (m Model) finishProjectColorForm() (tea.Model, tea.Cmd) {
	edit := m.pendingProject
	b := m.binding
	m.pendingProject = nil
	m.binding = nil
	m.form = nil
	m.mode = modeBoard // renders the active tab (tabConfig)
	if edit == nil || b == nil {
		return m, nil
	}
	color := strings.TrimSpace(b.projColor)
	if hex, ok := projects.SwatchByName(color); ok {
		color = hex
	}
	p, _ := projects.Load(edit.name)
	if p.Name == "" {
		m.status = "no such project " + edit.name
		return m, nil
	}
	p.Color = color
	if err := projects.Save(p); err != nil {
		m.status = "recolor failed: " + err.Error()
		return m, nil
	}
	m.refreshProject()
	m.status = "recolored project " + edit.name
	return m, nil
}

// finishSourceForm applies a completed add/edit/remove per-source form (updateForm
// routes a completed form here whenever pendingSource is set). It validates the
// inputs (the path must resolve to a dir that contains .gogo/, like `gogo source add`;
// the cap must parse to a non-negative int), then mutates the project through
// projects.AddSource / RemoveSource — the sanctioned CLI write to ~/.gogo/, NEVER a
// source's .gogo/. On success it reloads the project + re-aggregates the board. It
// always lands back on the config tab.
func (m Model) finishSourceForm() (tea.Model, tea.Cmd) {
	edit := m.pendingSource
	b := m.binding
	m.pendingSource = nil
	m.binding = nil
	m.form = nil
	m.mode = modeBoard // renders the active tab (tabConfig)
	if edit == nil || b == nil {
		return m, nil
	}

	if edit.op == "remove" {
		if !b.confirm {
			m.status = "cancelled"
			return m, nil
		}
		if _, err := projects.RemoveSource(edit.project, edit.origPath); err != nil {
			m.status = "remove failed: " + err.Error()
			return m, nil
		}
		m.refreshProject()
		m.status = "removed " + edit.origPath
		return m, nil
	}

	// add / edit — validate the path (must contain .gogo/) and the cap.
	abs, err := filepath.Abs(strings.TrimSpace(b.srcPath))
	if err != nil {
		m.status = "invalid path: " + err.Error()
		return m, nil
	}
	abs = filepath.Clean(abs)
	if info, err := os.Stat(filepath.Join(abs, ".gogo")); err != nil || !info.IsDir() {
		m.status = abs + " has no .gogo/ — not a gogo source"
		return m, nil
	}
	cap, err := strconv.Atoi(strings.TrimSpace(nonEmpty(b.srcCap, "0")))
	if err != nil || cap < 0 {
		m.status = "concurrent work items must be a non-negative integer (0 = unlimited)"
		return m, nil
	}
	name := strings.TrimSpace(b.srcName)
	if name == "" {
		name = filepath.Base(abs)
	}
	// Label color (cockpit-colors FR4): accept a hex OR a swatch name; a blank color on
	// ADD auto-assigns the next free palette swatch, so a new source is never colorless.
	color := strings.TrimSpace(b.srcColor)
	if hex, ok := projects.SwatchByName(color); ok {
		color = hex
	}
	if color == "" && edit.op == "add" {
		color = projects.AssignColor(projects.TakenColors(m.allProjects))
	}
	// An edit that moves the path removes the old entry so it is not orphaned
	// (AddSource dedupes by path, so a same-path edit updates in place).
	if edit.op == "edit" && edit.origPath != "" && edit.origPath != abs {
		projects.RemoveSource(edit.project, edit.origPath)
	}
	src := projects.Source{
		Name:                name,
		Path:                abs,
		MainBranch:          strings.TrimSpace(b.srcBranch),
		Color:               color,
		ConcurrentWorkItems: cap,
	}
	if _, err := projects.AddSource(edit.project, src); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}
	m.refreshProject()
	m.status = "saved " + name
	return m, nil
}

// nonEmpty returns s if it is non-blank, else the fallback — a tiny guard so a blank
// cap input is read as "0" (unlimited) rather than a parse error.
func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// viewConfig renders the config tab (FR9): a LEFT column with the project switcher
// (all home projects, ▸ on the focused one) + the focused project's sources (▸ on the
// cursor, each `● name  path  branch  cap N/∞`), and a RIGHT column with the focused
// source's detail + a knowledge explorer (its .gogo/knowledge/ files + sizes, and the
// project's .knowledge/). Reads/writes ONLY ~/.gogo/. Pure / substring-assertable (no
// TTY under go test → lipgloss emits plain text).
func (m Model) viewConfig() string {
	left := m.viewConfigLeft()
	right := m.viewConfigRight()
	half := m.configPaneWidth()
	leftCol := lipgloss.NewStyle().Width(half).Render(left)
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", right)

	parts := []string{colTitleStyle.Render("config — sources & knowledge"), "", body}
	if m.status != "" {
		parts = append(parts, "", statusStyle(m.status))
	}
	help := lipgloss.NewStyle().Faint(true).Render("↑↓/jk source · p switch project · c project color · a add · e edit · x remove · tab board/plans · q quit")
	parts = append(parts, "", help)
	return strings.Join(parts, "\n")
}

// configPaneWidth is the width of each config-tab column — half the terminal with a
// 30-col floor. Shared by viewConfig (which sizes the two columns) and viewConfigLeft
// (which truncates the switcher rows to it, REV-003) so the two never drift.
func (m Model) configPaneWidth() int {
	half := m.width / 2
	if half < 30 {
		half = 30
	}
	return half
}

// viewConfigLeft is the config tab's left column: the project switcher + the focused
// project's sources.
func (m Model) viewConfigLeft() string {
	pane := m.configPaneWidth()
	var b []string
	b = append(b, dimStyle.Render("project  ")+dimStyle.Render("(p to switch)"))
	if len(m.allProjects) == 0 {
		b = append(b, dimStyle.Render("  (no home project) — add one with `gogo project add <repo>`"))
	}
	for i, p := range m.allProjects {
		cursor := "  "
		focused := i == m.projIdx
		// Origin dots (D5): a project dot + its first source's dot (`●P ●S`) — the
		// multi-project combo that reads "project P, source S" at a glance. A focused row
		// renders the dots plain (the focus fill owns fg/bg). A live-session ● trails the
		// focused project's row when it has running sessions (they aggregate to it).
		dots := m.projectOriginDots(p, focused)
		meta := dimStyle.Render(fmt.Sprintf("  %d %s", len(p.Sources), plural(len(p.Sources), "source")))
		live := ""
		if focused && len(m.sessions) > 0 {
			live = " " + sessionStyle.Render("●")
		}
		if focused {
			cursor = "▸ "
		}
		// Truncate the project NAME so the composed `▸ ●P ●S name  N sources ●` row fits
		// the switcher pane instead of wrapping for a long name (REV-003). Widths are
		// measured ANSI-aware so the styled dots/meta/live don't inflate the budget.
		nameBudget := pane - lipgloss.Width(cursor+dots+" ") - lipgloss.Width(meta) - lipgloss.Width(live)
		name := truncate(p.Name, nameBudget)
		if focused {
			b = append(b, changelogFocusStyle.Render(cursor+dots+" "+name)+meta+live)
			continue
		}
		b = append(b, cursor+dots+" "+name+meta+live)
	}

	b = append(b, "", colTitleStyle.Render("sources"))
	srcs := m.sources()
	if len(srcs) == 0 {
		b = append(b, dimStyle.Render("  (none) — press a to add a source"))
	}
	for i, s := range srcs {
		cursor := "  "
		focused := i == m.sourceIdx
		if focused {
			cursor = "▸ "
		}
		name := s.Name
		if name == "" {
			name = filepath.Base(s.Path)
		}
		branch := s.MainBranch
		if branch == "" {
			branch = "main"
		}
		// The source's colored origin dot (cockpit-colors FR4); a focused row renders it
		// plain so the single focus fg/bg fill has no per-segment hole.
		dot := "●"
		if !focused {
			dot = m.sourceDot(name)
		}
		row := fmt.Sprintf("%s%s %-16s %-28s %-8s %s", cursor, dot, truncate(name, 16), truncate(s.Path, 28), branch, capText(s.ConcurrentWorkItems))
		if focused {
			b = append(b, changelogFocusStyle.Render(row))
		} else {
			b = append(b, row)
		}
	}
	return strings.Join(b, "\n")
}

// projectOriginDots renders a config-switcher project row's origin dots (D5): a project
// dot + the project's FIRST source's dot (`●P ●S`), or a lone project dot when the
// project has no sources yet. `plain` drops the tint for the focused (fill-owned) row.
func (m Model) projectOriginDots(p projects.Project, plain bool) string {
	pc := m.projectColor(p.Name)
	if len(p.Sources) == 0 {
		return originDots(nil, pc, plain) // a single project dot
	}
	first := p.Sources[0]
	label := first.Name
	if label == "" {
		label = filepath.Base(first.Path)
	}
	sc := colorFor(first.Color, label)
	return originDots(pc, sc, plain)
}

// viewConfigRight is the config tab's right column: the focused source's detail + the
// knowledge explorer (the source's .gogo/knowledge/ and the project's .knowledge/).
func (m Model) viewConfigRight() string {
	var b []string
	s := m.focusedSource()
	if s == nil {
		b = append(b, dimStyle.Render("(no source selected)"))
		return strings.Join(b, "\n")
	}
	name := s.Name
	if name == "" {
		name = filepath.Base(s.Path)
	}
	branch := s.MainBranch
	if branch == "" {
		branch = "main"
	}
	// Label color (design 3b): the resolved never-blank color named by its swatch
	// (`teal`) or shown as a raw hex; a blank stored color is flagged `(default)` so the
	// user knows it is the auto fallback, editable via the source `e` form's Label color.
	colorLabel := swatchName(m.sourceColors[name])
	if s.Color == "" {
		colorLabel += " (default)"
	}
	b = append(b,
		colTitleStyle.Render("source — "+name),
		dimStyle.Render("path         ")+s.Path,
		dimStyle.Render("branch       ")+branch,
		dimStyle.Render("label color  ")+m.sourceDot(name)+" "+colorLabel,
		dimStyle.Render("cap          ")+capText(s.ConcurrentWorkItems),
	)

	b = append(b, "", colTitleStyle.Render("knowledge"))
	files := knowledgeFiles(filepath.Join(s.Path, ".gogo", "knowledge"))
	if m.project != nil {
		files = append(files, knowledgeFiles(filepath.Join(projects.Dir(m.project.Name), ".knowledge"))...)
	}
	if len(files) == 0 {
		b = append(b, dimStyle.Render("  (no knowledge files)"))
	}
	for _, f := range files {
		b = append(b, fmt.Sprintf("  %-30s %s", truncate(f.name, 30), dimStyle.Render(humanSize(f.size))))
	}
	return strings.Join(b, "\n")
}

// capText renders a per-source concurrency cap: `cap ∞` for 0 (unlimited), else
// `cap N`.
func capText(n int) string {
	if n > 0 {
		return fmt.Sprintf("cap %d", n)
	}
	return "cap ∞"
}

// knEntry is one knowledge file (name + byte size) in the config-tab explorer.
type knEntry struct {
	name string
	size int64
}

// knowledgeFiles lists the regular files directly under dir (name-sorted, sizes),
// degrading to nothing on a missing / unreadable dir — never a crash (the same
// defensive style the contract reader uses). Non-recursive: it lists the knowledge
// dir's own files, not nested trees.
func knowledgeFiles(dir string) []knEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []knEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, knEntry{name: e.Name(), size: info.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// humanSize renders a byte count compactly (B / KB / MB) for the knowledge explorer.
func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
