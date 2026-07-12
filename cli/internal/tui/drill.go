package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/diagram"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	"github.com/ZawadzkiB/gogo/cli/internal/pages"
	"github.com/ZawadzkiB/gogo/cli/internal/textfmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// openDrill enters the file list for a feature (only files that exist) and
// assembles the CARD detail panel (Slice B) that sits above it.
func (m *Model) openDrill(f *contract.Feature) {
	m.drill = f
	m.artifacts = contract.Artifacts(f)
	m.artIdx = 0
	m.loadDrillCard(f)
	m.mode = modeDrill
	m.status = ""
}

// loadDrillCard (Slice B) assembles the card detail panel's deterministic data:
// the feature's persistent-session rows (registry ⨯ live-tmux cross-check —
// FR-B2/B5) and a compact recent-events tail (FR-B4). These are pure reads (no
// launch, no registry write), so opening — or refreshing — a drill never mutates
// state (FR-B5). Also called after a kill to refresh the panel in place.
func (m *Model) loadDrillCard(f *contract.Feature) {
	reg := m.registry(m.root, f.Slug)
	m.drillSessions = sessionRows(reg, m.sessions, f.Slug)
	m.drillEventsTail = eventsTail(contract.ReadEvents(filepath.Join(f.Dir, "events.jsonl")), 5)
}

// sessionRow is one line of the drill card's session panel (FR-B5): a tracked
// registry leg or an untracked-but-live tmux session, carrying the live/stale +
// tracked/untracked cross-check (D4) and per-leg cost/turns.
type sessionRow struct {
	Kind     string  // "go" | "plan" for a tracked leg; "" for an untracked live session
	Status   string  // registry lifecycle status (running|parked|awaiting-uat|shipped|reaped); "" when untracked
	Session  string  // tmux session name (the kill/attach target); "" for a headless tracked leg
	Live     bool    // a live gogo-* tmux session backs this row right now
	Tracked  bool    // present in the registry (vs an untracked board-launched racer)
	CostUSD  float64 // summed cost for the leg (0 when untracked)
	NumTurns int     // summed turns for the leg (0 when untracked)
}

// sessionRows maps (registry, live tmux sessions, slug) → display rows,
// deterministically and LLM-free (FR-B5). Tracked legs (registry, kinds go then
// plan) come first, each cross-checked against the slug's live sessions by EXACT
// SessionMatchesSlug (never substring — oauth ≠ auth, waiting-card ≠ awaiting-card;
// TEST-005) and flagged live/stale; any remaining live session for the slug with
// no tracked leg is shown as an untracked-live row (a board-launched racer). A
// missing/garbled registry (empty Persistent) with no live sessions yields no
// rows — the caller renders "no tracked sessions" (never a crash).
func sessionRows(reg *orchestrator.Registry, live []string, slug string) []sessionRow {
	if reg == nil {
		reg = &orchestrator.Registry{}
	}
	// The slug's own live sessions (exact convention parse) — a working set we
	// consume as tracked legs claim their session.
	mine := map[string]bool{}
	var mineOrder []string
	for _, s := range live {
		if launch.SessionMatchesSlug(s, slug) && !mine[s] {
			mine[s] = true
			mineOrder = append(mineOrder, s)
		}
	}

	var rows []sessionRow
	for _, kind := range []string{"go", "plan"} {
		ps := reg.Get(kind)
		if ps == nil {
			continue
		}
		row := sessionRow{
			Kind:     kind,
			Status:   ps.Status,
			Session:  ps.Tmux,
			Tracked:  true,
			CostUSD:  ps.CostUSD,
			NumTurns: ps.NumTurns,
		}
		// A tracked leg is live iff its recorded tmux session is live right now.
		if ps.Tmux != "" && mine[ps.Tmux] {
			row.Live = true
			delete(mine, ps.Tmux) // claimed — don't also list it as untracked
		}
		rows = append(rows, row)
	}
	// Remaining live sessions for the slug had no tracked leg → untracked live.
	for _, s := range mineOrder {
		if mine[s] {
			rows = append(rows, sessionRow{Session: s, Live: true})
		}
	}
	return rows
}

// eventsTail renders the last n events as a compact inline tail for the drill
// card (FR-B4 / D3): "hh:mm:ss event phase[ rN][ — note]". The FULL
// textfmt.Timeline (what `gogo events` renders) stays reachable via the existing
// events row — no duplicate renderer, just an at-a-glance tail. No events → "".
func eventsTail(evs []contract.Event, n int) string {
	if len(evs) == 0 {
		return ""
	}
	if n > 0 && len(evs) > n {
		evs = evs[len(evs)-n:]
	}
	var b strings.Builder
	for i, e := range evs {
		if i > 0 {
			b.WriteString("\n")
		}
		ts := e.TSRaw
		if e.TSValid {
			ts = e.TS.Format("15:04:05")
		}
		line := ts + "  " + e.Event
		if e.Phase != "" {
			line += " " + e.Phase
		}
		if e.HasRound {
			line += fmt.Sprintf(" r%d", e.Round)
		}
		if e.Note != "" {
			line += " — " + e.Note
		}
		b.WriteString(line)
	}
	return b.String()
}

// quickView (v on the board) opens the DEFAULT file for the focused card's
// column (TEST-004): ready → report/report.md (else legacy root report.md);
// plan → plan.md; changelog → the entry's report.md; in progress → the file
// list (no file). A missing default also falls back to the file list. `enter`
// (openDrill) is unchanged — it always shows the file list.
func (m *Model) quickView(f *contract.Feature) tea.Cmd {
	m.openDrill(f) // sets up the file list + modeDrill as the fallback
	a, ok := m.defaultArtifact(f)
	if !ok {
		return nil // stay on the file list
	}
	// Highlight the matching row in the file list, when it is one of the
	// feature's own files (a changelog report lives outside f.Dir).
	for i, art := range m.artifacts {
		if art.Path == a.Path {
			m.artIdx = i
			break
		}
	}
	return m.openArtifact(a)
}

// defaultArtifact resolves the column's default view file for f, or ok=false to
// keep the file list (in-progress, or a missing default).
func (m *Model) defaultArtifact(f *contract.Feature) (contract.Artifact, bool) {
	switch f.Class {
	case contract.ClassInProgress:
		return contract.Artifact{}, false
	case contract.ClassReadyToShip:
		if f.ReportPath != "" {
			return contract.Artifact{Label: markdownLabel(f, f.ReportPath), Path: f.ReportPath, Kind: contract.KindMarkdown}, true
		}
	case contract.ClassShipped:
		if f.ChangelogPath != "" {
			rep := filepath.Join(f.ChangelogPath, "report.md")
			if fileExists(rep) {
				return contract.Artifact{Label: "report.md", Path: rep, Kind: contract.KindMarkdown}, true
			}
		}
		// Shipped by status line only (no changelog entry): fall back to the
		// feature's own report bundle, else the file list.
		if f.ReportPath != "" {
			return contract.Artifact{Label: markdownLabel(f, f.ReportPath), Path: f.ReportPath, Kind: contract.KindMarkdown}, true
		}
	default: // unfinished / plan
		plan := filepath.Join(f.Dir, "plan.md")
		if fileExists(plan) {
			return contract.Artifact{Label: "plan.md", Path: plan, Kind: contract.KindMarkdown}, true
		}
	}
	return contract.Artifact{}, false
}

// markdownLabel names a report path as the drill-in list does (report/report.md
// for the bundle, report.md for the legacy root).
func markdownLabel(f *contract.Feature, path string) string {
	if filepath.Dir(path) == filepath.Join(f.Dir, "report") {
		return "report/report.md"
	}
	return filepath.Base(path)
}

// openArtifact switches to the viewer and starts an ASYNC render (TEST-003): the
// heavy work (glamour / ASCII diagram / large read) runs in a tea.Cmd so the UI
// never blocks, with a spinner "rendering…" state until content arrives. A cache
// keyed by (kind|path|width) makes reopening — and returning at the same width —
// instant.
func (m *Model) openArtifact(a contract.Artifact) tea.Cmd {
	m.curArtifact = a
	m.viewerTitle = a.Label
	m.mode = modeViewer
	m.viewport.GotoTop()
	key := cacheKey(a, m.width)
	if cached, ok := m.renderCache[key]; ok {
		m.viewport.SetContent(cached)
		m.viewerLoading = false
		return nil
	}
	m.viewport.SetContent("")
	m.viewerLoading = true
	return tea.Batch(m.renderArtifactCmd(a, m.width), m.spinner.Tick)
}

// cacheKey identifies a rendered artifact at a width.
func cacheKey(a contract.Artifact, width int) string {
	return string(a.Kind) + "|" + a.Path + "|" + strconv.Itoa(width)
}

// renderArtifactCmd renders an artifact off the UI goroutine and reports the
// content back as a viewerContentMsg. The captured values (path, width, dark)
// make the closure pure — safe to run concurrently with Update.
func (m Model) renderArtifactCmd(a contract.Artifact, width int) tea.Cmd {
	dark := m.dark
	key := cacheKey(a, width)
	return func() tea.Msg {
		return viewerContentMsg{key: key, content: renderArtifactContent(a, width, dark)}
	}
}

// renderArtifactContent is the pure render for any artifact kind. NEVER touches
// the terminal (no WithAutoStyle) — that was the TEST-003 freeze.
func renderArtifactContent(a contract.Artifact, width int, dark bool) string {
	switch a.Kind {
	case contract.KindMarkdown:
		return renderMarkdownContent(a.Path, width, dark)
	case contract.KindIssues:
		list, err := contract.ReadIssues(a.Path)
		if err != nil {
			return "failed to read issues: " + err.Error()
		}
		return renderIssues(list)
	case contract.KindEvents:
		return renderTimeline(contract.ReadEvents(a.Path))
	case contract.KindMermaid:
		return renderDiagramFile(a.Path, width)
	default:
		raw, _ := os.ReadFile(a.Path)
		return string(raw)
	}
}

// renderMarkdownContent reads, pre-processes mermaid fences (TEST-005), and
// renders markdown with an EXPLICIT dark/light style — never WithAutoStyle,
// which queries the terminal and froze the UI (TEST-003).
func renderMarkdownContent(path string, width int, dark bool) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "failed to read " + path + ": " + err.Error()
	}
	if width <= 0 {
		width = 80
	}
	src := preprocessMermaid(string(raw), width)
	// TEST-009: the shared custom "article" style (mdstyle.go) — airier spacing,
	// a clear heading hierarchy, stronger emphasis — instead of the stock
	// dark/light StyleConfig. Still zero terminal queries (no WithAutoStyle).
	r, err := glamour.NewTermRenderer(glamour.WithStyles(markdownStyle(dark)), glamour.WithWordWrap(width))
	if err != nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return out
}

// renderIssues / renderTimeline delegate to the shared textfmt package.
func renderIssues(list *contract.IssuesList) string { return textfmt.Issues(list) }
func renderTimeline(evs []contract.Event) string    { return textfmt.Timeline(evs) }

// renderDiagramFile renders a .mmd via the mermaid-ascii engine: Unicode
// box-drawing for flowchart/graph, a real render for sequenceDiagram; kinds
// without a terminal renderer (class/state/…) show the source + the "press w"
// hint. Never crashes — a render error also falls back to the source.
func renderDiagramFile(path string, width int) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "failed to read " + path
	}
	if rendered, err := diagram.Render(string(raw), width); err == nil {
		return rendered
	}
	return "(this diagram kind renders in the browser — press w for the interactive view)\n\n" + string(raw)
}

// buildPageCmd builds the `w` page for the focused/drilled feature and opens
// the browser (best-effort).
func (m Model) buildPageCmd() tea.Cmd {
	f := m.drill
	if f == nil {
		f = m.focusedCard()
	}
	if f == nil {
		return func() tea.Msg { return launchDoneMsg{status: "no feature to view"} }
	}
	root := m.root
	feat := f
	return func() tea.Msg {
		bundle, err := bundleFor(root, feat)
		if err != nil {
			return launchDoneMsg{status: "cannot build page: " + err.Error()}
		}
		page, err := pages.WritePage(root, bundle)
		if err != nil {
			return launchDoneMsg{status: "page build failed: " + err.Error()}
		}
		openBrowser(page)
		return launchDoneMsg{status: "page: " + page}
	}
}

// bundleFor picks the report bundle when a report exists, else the plan
// bundle, and wires up its diagram + before/ dirs and manifest.
func bundleFor(root string, f *contract.Feature) (pages.Bundle, error) {
	if f.ReportPath != "" {
		dir := filepath.Dir(f.ReportPath)
		before := filepath.Join(dir, "before")
		if !dirExists(before) {
			before = ""
		}
		// legacy root report → diagrams live under charts/
		diagDir := dir
		if filepath.Base(dir) != "report" {
			diagDir = filepath.Join(f.Dir, "charts")
		}
		return pages.Bundle{
			Name:         f.Slug,
			Title:        "gogo — " + f.Slug,
			MarkdownPath: f.ReportPath,
			DiagramDir:   diagDir,
			BeforeDir:    before,
			ManifestPath: filepath.Join(dir, "manifest.json"),
		}, nil
	}
	plan := filepath.Join(f.Dir, "plan.md")
	if !fileExists(plan) {
		return pages.Bundle{}, fmt.Errorf("no report or plan for %s", f.Slug)
	}
	charts := filepath.Join(f.Dir, "charts")
	before := filepath.Join(charts, "before")
	if !dirExists(before) {
		before = ""
	}
	return pages.Bundle{
		Name:         f.Slug + "-plan",
		Title:        "gogo — " + f.Slug + " (plan)",
		MarkdownPath: plan,
		DiagramDir:   charts,
		BeforeDir:    before,
		ManifestPath: filepath.Join(charts, "manifest.json"),
	}, nil
}

// openInGlow suspends the TUI and opens the current artifact in the glow binary
// (soft dep). No glow → a status hint.
func (m Model) openInGlow() (tea.Model, tea.Cmd) {
	if !m.hasGlow {
		m.status = "glow not installed — showing the built-in glamour view"
		return m, nil
	}
	if m.artIdx >= len(m.artifacts) {
		return m, nil
	}
	path := m.artifacts[m.artIdx].Path
	c := exec.Command("glow", "-p", path)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return launchDoneMsg{status: "closed glow"}
	})
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// openBrowser is best-effort: open (macOS) → xdg-open (Linux) → print path.
func openBrowser(path string) {
	if _, err := exec.LookPath("open"); err == nil {
		_ = exec.Command("open", path).Start()
		return
	}
	if _, err := exec.LookPath("xdg-open"); err == nil {
		_ = exec.Command("xdg-open", path).Start()
	}
}
