// Package tui is the Bubble Tea cockpit: a 4-column kanban board over the
// deterministic contract reader (plan | in progress | ready | changelog),
// drill-in file viewing (glamour / issues table / events timeline / ASCII
// diagrams), native `w` page builds, and column moves that launch Claude via
// the launch package (never mutating pipeline state directly). The model's
// Update/View are pure and unit-tested without a tty.
package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// mode is the WITHIN-tab interaction state. The old modal config/drafts/epics
// screens are gone — those are now the top-level TABS (tabID), and each tab
// composes the same within-tab modes (a drill, an async viewer, a huh form).
type mode int

const (
	modeBoard mode = iota // the tab's normal state (board cards / plans list / config panes)
	modeDrill
	modeViewer
	modeForm
)

// tabID is the top-level cockpit tab (FR8/D6). tab / shift+tab cycle
// board → plans → config; the active tab owns the body below the tab bar, and
// within-tab modes (drill/viewer/form) compose on top of it. Tabs exist ONLY on a
// project board (m.global()); a lone repo shows the single board with no tab bar
// (byte-for-byte fallback, FR7).
type tabID int

const (
	tabBoard tabID = iota
	tabPlans
	tabConfig
)

// tabTitles / tabCount fix the tab bar order left→right.
var tabTitles = [3]string{"board", "plans", "config"}

const tabCount = 3

// formBinding holds the huh field targets behind a pointer so the bindings stay
// valid as the value-type Model is copied between Update calls. Binding huh's
// .Value() directly to a field of the Model (a value receiver copies the struct
// on every Update) would leave the form writing to an orphaned copy, so a
// confirmed launch would read a stale false and silently cancel. TEST-001.
type formBinding struct {
	release  string
	confirm  bool
	selected string // the attach/kill picker's chosen value (session name or a sentinel)
	// Config-tab per-source form fields (FR9): the source's name/path/branch/color/cap
	// as STRINGS so the huh inputs bind heap-stable targets (TEST-001) and the cap is
	// parsed + validated to a non-negative int on completion (never bound as an int the
	// value-type Model would copy out from under the live form).
	srcName   string
	srcPath   string
	srcBranch string
	srcColor  string
	srcCap    string
	// Plans-tab new-plan form field (FR10 `n`): the plan title as a STRING the huh
	// input binds heap-stably (TEST-001).
	planTitle string
}

// sourceEdit marks an in-flight config-tab per-source form (the analog of
// pendingKill/pendingAttach): op is "add" | "edit" | "remove", project is the
// owning project's name the write targets, and origPath is the source's Path
// BEFORE the edit (so a path change on edit is applied against the right entry, and
// remove targets the right key). "" origPath for add.
type sourceEdit struct {
	op       string
	project  string
	origPath string
}

// Picker sentinels — the non-empty values the attach/kill huh.NewSelect writes to
// binding.selected for its non-session options. Plain ASCII, and deliberately NOT
// valid tmux session names or repo paths (a leading space never occurs in
// gogo-<action>-<slug> nor in an absolute path), so they can never collide with a
// real choice. An empty binding.selected means "no picker ran" — the single-session
// Confirm path — so these must stay non-empty (a distinct-from-"" discriminator, no
// extra field needed).
const (
	killAll      = " kill-all"      // kill every pendingKill session
	killCancel   = " kill-cancel"   // cancel the kill picker
	attachCancel = " attach-cancel" // cancel the attach picker
)

// columnOrder / columnTitles fix the 4-column layout left→right.
var (
	columnOrder  = [4]string{contract.ColPlan, contract.ColInProgress, contract.ColReady, contract.ColChangelog}
	columnTitles = [4]string{"plan", "in progress", "ready", "changelog"}
)

// Model is the whole cockpit state.
type Model struct {
	root string // the single repo root; "" in the project (multi-source) board
	repo *contract.Repo

	// Tabbed project board (FR7/FR8, m.root == ""). project is the focused home
	// project whose SOURCES the board aggregates (nil in single-repo mode);
	// allProjects is every home project (the header "M projects" count + the config-
	// tab switcher); sourceColors maps a source label → its card-tag color (hex). The
	// merged repo's features each carry their own Source/Root, so the board tags cards
	// by Feature.Source and the live re-aggregate (reload → LoadProject) stays source-
	// native — no config.Project bridge.
	tab          tabID
	project      *projects.Project
	allProjects  []projects.Project
	sourceColors map[string]string

	// Config tab (FR9): the project-switcher cursor + the per-source cursor + the
	// in-flight per-source edit marker. Reads/writes ONLY ~/.gogo/… via the projects
	// store (never a source's .gogo/).
	projIdx       int
	sourceIdx     int
	pendingSource *sourceEdit

	// Plans tab (FR10/FR11): the focused project's plans (grouped ACTIVE·READY·DRAFTS),
	// the list cursor (planIdx, over the grouped order), the open plan detail (nil =
	// list view), the plan-detail target-source cursor, and the in-flight new-plan form
	// marker. Reads/writes ONLY ~/.gogo/… via the plans store; spawning a work item is
	// a claude -p launch (never a source's .gogo/ write).
	plans         []plans.Plan
	planIdx       int
	planDetail    *plans.Plan
	planSourceIdx int
	pendingPlan   bool

	// sourceChip is the active board source filter (FR7): "" = all sources, else the
	// source label the `p`-cycled chip narrows the board to. Distinct from the free-
	// text filter (m.filter) — both AND together in rebuild.
	sourceChip string

	cols      [4][]*contract.Feature
	colIdx    int
	cardIdx   [4]int
	colOffset [4]int          // per-column scroll offset (first visible card) — TEST-014
	selected  map[string]bool // selected ready-to-ship slugs (space)

	filter    string
	filtering bool
	status    string

	showAllKeys bool // FR-10: ? toggles the full key list under the contextual footer

	width, height int
	mode          mode

	sessions []string

	// drill-in
	drill     *contract.Feature
	artifacts []contract.Artifact
	artIdx    int

	// drill-in CARD detail (Slice B — FR-B1/B2/B4): the card's session rows
	// (registry ⨯ live-tmux cross-check) and a compact recent-events tail,
	// (re)computed by openDrill/loadDrillCard. Description / folder / status are
	// derived from m.drill at render time (no cache — they already live there).
	drillSessions   []sessionRow
	drillEventsTail string

	// viewer
	viewport      viewport.Model
	viewerTitle   string
	viewerReady   bool
	viewerLoading bool              // TEST-003: async render in flight (spinner shown)
	curArtifact   contract.Artifact // the artifact currently open/loading (for width re-render)
	spinner       spinner.Model     // loading spinner while a viewer render runs
	renderCache   map[string]string // rendered content by (kind|path|width) — instant reopen
	dark          bool              // terminal background, detected ONCE before the program starts

	// form
	form            *huh.Form
	pending         launch.Intent
	pendingShip     bool
	pendingDelete   *contract.Feature // FR6: the card a confirmed `x` moves to trash
	pendingKill     []string          // FR-B3: the drill card's live session(s) a confirmed `K` kills
	pendingAttach   []string          // the attach picker's candidate sessions (≥2 live) — FR-2
	pickerFromDrill bool              // the attach picker was opened from the drill (cancel restores modeDrill vs modeBoard)
	binding         *formBinding      // heap-stable targets for the live huh fields

	// peek (FR7): a read-only session-log viewer reusing the async viewer.
	peeking     bool   // the open viewer is a session-log peek (r re-captures)
	peekSlug    string // the card being peeked
	peekSession string // live tmux session name, or "" for a background-log peek
	peekLog     string // background -p log path, or ""

	// capturer snapshots a session's pane for a peek. A seam (defaults to
	// launch.CapturePane) so peek can be driven in tests without real tmux.
	capturer func(session string, lines int) (string, error)

	// launcher spawns a confirmed intent. A seam (defaults to launch.Launch) so
	// the form lifecycle can be driven with a fake in tests — never nil once a
	// Model comes from New.
	launcher func(root string, in launch.Intent) (launch.Result, error)

	// killer kills a live tmux session by exact name (defaults to
	// launch.KillSession) and registry loads a feature's persistent-session
	// registry (defaults to orchestrator.LoadRegistry). Seams (FR-B3/B5) so the
	// drill-in kill wiring + the session-row reader are asserted with fakes, no
	// real tmux/registry file — never nil once a Model comes from New.
	killer   func(session string) error
	registry func(root, slug string) *orchestrator.Registry

	hasTmux, hasClaude, hasGlow bool
	reloadCh                    chan struct{}
	watch                       *watchSet // long-lived fsnotify handle (set by Init)
}

// New loads the single repo at root and builds the SINGLE-REPO board (the graceful
// fallback path, FR7): m.root != "", no home project, no sources. It does NOT
// consult the legacy config registry — a lone repo carries no source-cap and no
// source tags, so its board is byte-for-byte today's single-repo board (no tab bar,
// no chips, no project count; capBounce is inert because m.sources() is empty). It
// does NOT start fsnotify (that happens in Init) so tests can drive Update directly.
func New(root string) Model {
	repo, _ := contract.LoadRepo(root)
	return newFromRepo(repo, root, nil, nil)
}

// NewProjectBoard builds a PROJECT board (the corrected multi-source model, FR7):
// it aggregates the focused project's SOURCES (contract.LoadProject) into one
// source-tagged, tabbed board. allProjects (projects.List) feeds the header
// "M projects" count + the config-tab switcher; the focused project's Sources feed
// the tag colors, the source-cap guard (CapForSource), and the config tab.
func NewProjectBoard(proj projects.Project) Model {
	all, _ := projects.List()
	return newFromRepo(contract.LoadProject(proj), "", &proj, all)
}

// NewWorkspace is the source-native test seam for the tabbed project board: it
// injects an in-memory *contract.Repo (so a test drives Update/View without disk)
// plus the focused project (its sources feed the tags/cap/config-tab). allProjects
// defaults to just that project; a test can widen m.allProjects to exercise the
// header count / switcher. The real entrypoint is NewProjectBoard.
func NewWorkspace(repo *contract.Repo, proj projects.Project) Model {
	return newFromRepo(repo, "", &proj, []projects.Project{proj})
}

// newFromRepo is the shared Model constructor: New (single-repo, root != "",
// project == nil) and NewProjectBoard/NewWorkspace (project board, root == "", a
// non-nil focused project). Keeping one constructor guarantees the two boards are
// byte-for-byte identical except for the project-board-only source state.
func newFromRepo(repo *contract.Repo, root string, project *projects.Project, all []projects.Project) Model {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	sp.Style = lipgloss.NewStyle().Foreground(columnAccent[0])
	m := Model{
		root:        root,
		repo:        repo,
		project:     project,
		allProjects: all,
		selected:    map[string]bool{},
		mode:        modeBoard,
		tab:         tabBoard,
		hasTmux:     launch.HasTmux(),
		hasClaude:   launch.HasClaude(),
		hasGlow:     launch.HasGlow(),
		launcher:    launch.Launch,
		capturer:    launch.CapturePane,
		killer:      launch.KillSession,
		registry:    orchestrator.LoadRegistry,
		reloadCh:    make(chan struct{}, 1),
		viewport:    viewport.New(0, 0),
		spinner:     sp,
		renderCache: map[string]string{},
		// Detect the terminal background ONCE here — before tea.Program grabs the
		// TTY — and pass an explicit glamour style thereafter. The freeze (TEST-003)
		// was glamour's WithAutoStyle re-querying the terminal (termenv OSC-11 +
		// 5s timeout) on EVERY view while Bubble Tea owned stdin; this makes it a
		// single, safe, cached detection.
		dark: lipgloss.HasDarkBackground(),
	}
	if project != nil {
		m.sourceColors = sourceColorMap(project.Sources)
		for i := range all {
			if all[i].Name == project.Name {
				m.projIdx = i // start the config-tab switcher on the focused project
				break
			}
		}
	}
	m.sessions = launch.ListSessions()
	m.loadPlans() // load the project's plans for the plans tab (project board only)
	m.rebuild()
	return m
}

// sources returns the focused project's sources (nil in single-repo mode) — what
// the cap guard (CapForSource) and the config tab read.
func (m *Model) sources() []projects.Source {
	if m.project == nil {
		return nil
	}
	return m.project.Sources
}

// sourceColorMap builds the source-label → tag-color (hex) lookup the project board
// tints cards with, defaulting an unnamed source to its folder base. Shared by the
// constructor and the config-tab refresh so the two never drift.
func sourceColorMap(sources []projects.Source) map[string]string {
	colors := make(map[string]string, len(sources))
	for _, s := range sources {
		name := s.Name
		if name == "" {
			name = filepath.Base(s.Path)
		}
		if s.Color != "" {
			colors[name] = s.Color
		}
	}
	return colors
}

// sourceChips is the ordered set of source-filter chip labels (FR7): "all" first,
// then one per source of the focused project. "" for single-repo (no chips).
func (m *Model) sourceChips() []string {
	if m.project == nil {
		return nil
	}
	out := []string{""} // "" renders as the "all" chip
	for _, s := range m.project.Sources {
		label := s.Name
		if label == "" {
			label = filepath.Base(s.Path)
		}
		out = append(out, label)
	}
	return out
}

// cycleChip advances the active source chip (FR7 `p`): all → source-1 → … → all.
// A no-op on a project with no sources.
func (m *Model) cycleChip(dir int) {
	chips := m.sourceChips()
	if len(chips) <= 1 {
		return
	}
	cur := 0
	for i, c := range chips {
		if c == m.sourceChip {
			cur = i
			break
		}
	}
	m.sourceChip = chips[((cur+dir)%len(chips)+len(chips))%len(chips)]
	m.rebuild()
}

// cycleTab advances the active tab board → plans → config (FR8/D6). Project board
// only; a lone repo has no tabs (guarded by the caller on m.global()).
func (m *Model) cycleTab(dir int) {
	m.tab = tabID(((int(m.tab)+dir)%tabCount + tabCount) % tabCount)
	m.status = ""
}

// focusedProject returns the home project under the config-tab switcher cursor, or
// nil on an empty store / out-of-range index.
func (m *Model) focusedProject() *projects.Project {
	if m.projIdx < 0 || m.projIdx >= len(m.allProjects) {
		return nil
	}
	return &m.allProjects[m.projIdx]
}

// focusedSource returns the source under the config-tab source cursor, or nil.
func (m *Model) focusedSource() *projects.Source {
	srcs := m.sources()
	if m.sourceIdx < 0 || m.sourceIdx >= len(srcs) {
		return nil
	}
	return &srcs[m.sourceIdx]
}

// refreshProject reloads the focused project + the full project list from the store
// (after a config-tab write), re-derives the source colors, re-clamps the cursors,
// and re-aggregates the board so the change shows live. Reads/writes ONLY ~/.gogo/…
func (m *Model) refreshProject() {
	all, _ := projects.List()
	m.allProjects = all
	m.projIdx = clamp(m.projIdx, 0, len(all)-1)
	if p := m.focusedProject(); p != nil {
		m.project = p
	}
	if m.project != nil {
		m.sourceColors = sourceColorMap(m.project.Sources)
		m.sourceIdx = clamp(m.sourceIdx, 0, len(m.project.Sources)-1)
	}
	m.reload()
}

// switchProject points the board at allProjects[idx] (the config-tab `p` switcher),
// re-deriving sources/colors and re-aggregating. Clamps to range; a no-op with no
// projects.
func (m *Model) switchProject(idx int) {
	if len(m.allProjects) == 0 {
		return
	}
	m.projIdx = ((idx % len(m.allProjects)) + len(m.allProjects)) % len(m.allProjects)
	m.project = &m.allProjects[m.projIdx]
	m.sourceColors = sourceColorMap(m.project.Sources)
	m.sourceIdx = clamp(m.sourceIdx, 0, len(m.project.Sources)-1)
	m.sourceChip = ""
	// A project switch invalidates the plans-tab cursor/detail (a different plan set).
	m.planIdx = 0
	m.planDetail = nil
	m.reload()
}

// loadPlans reads the focused project's plans (FR10) for the plans tab and clamps
// the plan cursor. Project board only — a lone repo has no plans tab, so it degrades
// to an empty slice (never a crash). Run at construction and on every reload.
func (m *Model) loadPlans() {
	if m.project == nil {
		m.plans = nil
		return
	}
	m.plans, _ = plans.List(m.project.Name)
	m.planIdx = clamp(m.planIdx, 0, len(m.groupedPlans())-1)
}

// knownCorrelationIDs is the set of plan-correlation ids actually present on the
// board (the union of every loaded feature's Correlations, read straight from
// state.md). The filter treats a `#<id>` token as a correlation filter ONLY when its
// id is in this set; an unknown `#token` degrades to a literal text match
// (byte-for-byte parity — a stray `#` never nukes a board with no correlations, FR14).
func (m *Model) knownCorrelationIDs() map[string]bool {
	ids := map[string]bool{}
	for _, f := range m.repo.Features {
		for _, id := range f.Correlations {
			ids[id] = true
		}
	}
	return ids
}

// global reports whether this is the aggregate multi-project board (no single
// root — each feature carries its own).
func (m *Model) global() bool { return m.root == "" }

// rootFor resolves the repo root a per-feature action must target: the feature's
// OWN root (stamped by LoadRepo) when present, else the board's single root
// (m.root). This makes the aggregate board's actions project-aware (D6=A) while
// keeping single-repo byte-for-byte identical (there f.Root == m.root, so this
// returns the same value the code used before).
func (m *Model) rootFor(f *contract.Feature) string {
	if f != nil && f.Root != "" {
		return f.Root
	}
	return m.root
}

// Init starts the fsnotify watch loop, the reload waiter, and the session
// ticker (TEST-006 — keeps the card session dots fresh between reloads).
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.startWatchCmd(), waitForReload(m.reloadCh), sessionTick())
}

// rebuild partitions the (filtered) features into the four columns and clamps
// focus indices.
func (m *Model) rebuild() {
	known := m.knownCorrelationIDs()
	var cols [4][]*contract.Feature
	for _, f := range m.repo.Features {
		// The `p`-cycled source chip narrows to one source (FR7); it ANDs with the
		// free-text filter. "" (all) never hides anything — single-repo parity.
		if m.sourceChip != "" && f.Source != m.sourceChip {
			continue
		}
		if m.filter != "" && !matchFilter(f, m.filter, m.global(), known) {
			continue
		}
		switch f.Column() {
		case contract.ColPlan:
			cols[0] = append(cols[0], f)
		case contract.ColInProgress:
			cols[1] = append(cols[1], f)
		case contract.ColReady:
			cols[2] = append(cols[2], f)
		case contract.ColChangelog:
			cols[3] = append(cols[3], f)
		}
	}
	m.cols = cols
	for i := range m.cardIdx {
		m.cardIdx[i] = clamp(m.cardIdx[i], 0, len(cols[i])-1)
	}
	m.colIdx = clamp(m.colIdx, 0, 3)
}

// reload re-reads the repo + sessions and rebuilds, preserving filter/focus. In
// the project board it re-runs the multi-source merge (LoadProject) so a change in
// any source is picked up live; in single-repo mode it re-reads the one root exactly
// as before.
func (m *Model) reload() {
	if m.project != nil {
		m.repo = contract.LoadProject(*m.project) // re-aggregate the project's sources
	} else if repo, err := contract.LoadRepo(m.root); err == nil {
		m.repo = repo
	}
	m.sessions = launch.ListSessions()
	m.loadPlans() // re-read the project's plans after the reload
	m.rebuild()
}

// refocus restores the cursor to a slug within the currently focused column
// after a reload (features can be added/removed, shifting indices). If the slug
// still lives in the column the cursor follows it (so the window keeps it
// visible after the reflow); otherwise the index is clamped into range. TEST-014.
func (m *Model) refocus(slug string) {
	col := m.cols[m.colIdx]
	if slug != "" {
		for j, f := range col {
			if f.Slug == slug {
				m.cardIdx[m.colIdx] = j
				return
			}
		}
	}
	m.cardIdx[m.colIdx] = clamp(m.cardIdx[m.colIdx], 0, len(col)-1)
}

func (m *Model) focusedCard() *contract.Feature {
	col := m.cols[m.colIdx]
	if len(col) == 0 {
		return nil
	}
	return col[clamp(m.cardIdx[m.colIdx], 0, len(col)-1)]
}

func (m *Model) selectedSlugs() []string {
	var out []string
	for slug, on := range m.selected {
		if on {
			out = append(out, slug)
		}
	}
	sort.Strings(out)
	return out
}

// matchFilter reports whether feature f matches the board filter q (FR5). The
// `@name` project token is an AGGREGATE-board concept only: it is honored solely
// when global is true. There a leading `@fragment` narrows to features whose
// project label contains it (case-insensitive substring), the remaining non-@
// words keep the slug+title substring match, and when both are present they AND
// together. In single-repo mode (global == false) every feature's Project is ""
// so an `@` token could never match — treating `@` as a token would hide EVERY
// card (REV-002), so the whole query, `@` and all, is instead matched literally
// over slug+title, byte-for-byte as before the token existed (FR7 parity). A bare
// text query (no @) is identical in both modes.
//
// The `#plan-XXXX` CORRELATION token (FR14) is peeled FIRST and applies to BOTH
// boards (a plan's members span sources, and a single-repo board can hold members
// too): it narrows to features whose Correlations (read from state.md) contain that
// id (many-to-many — ANY match). It is only enforced when knownCorrelations has the
// id (a real correlation on the board); an unknown `#token` is left in the query and
// matched literally, so a stray `#` on a board with no correlations degrades to text
// matching and hides nothing (FR14 parity). After the token is removed, the remaining
// query flows through the unchanged single/aggregate logic.
func matchFilter(f *contract.Feature, q string, global bool, knownCorrelations map[string]bool) bool {
	corr, rest := splitCorrelationToken(q, knownCorrelations)
	if corr != "" && !containsFold(f.Correlations, corr) {
		return false
	}
	if !global {
		if rest == "" {
			return true // an epic-only query already filtered above
		}
		return strings.Contains(strings.ToLower(f.Slug+" "+f.Title), strings.ToLower(rest))
	}
	project, text := splitFilter(rest)
	if project != "" && !strings.Contains(strings.ToLower(f.Source), project) {
		return false
	}
	if text != "" && !strings.Contains(strings.ToLower(f.Slug+" "+f.Title), text) {
		return false
	}
	return true
}

// splitCorrelationToken peels a `#plan-XXXX` correlation token from the filter (the
// last one wins, like @project), returning the token's id (lowercased) and the
// REMAINING query with that token removed. A `#`-token is only treated as a
// correlation filter when its id is in knownCorrelations; otherwise it stays in rest
// and is matched literally (the parity fallback so a board with no correlations
// never over-hides).
func splitCorrelationToken(q string, knownCorrelations map[string]bool) (corr, rest string) {
	var keep []string
	for _, tok := range strings.Fields(q) {
		if strings.HasPrefix(tok, "#") {
			if id := strings.ToLower(strings.TrimPrefix(tok, "#")); id != "" && knownCorrelations[id] {
				corr = id
				continue
			}
		}
		keep = append(keep, tok)
	}
	return corr, strings.Join(keep, " ")
}

// containsFold reports whether ss contains want (case-insensitive). Plan ids are
// already [a-z0-9-], so this is effectively an exact compare, but folding keeps a
// user-typed `#PLAN-...` matching regardless of case.
func containsFold(ss []string, want string) bool {
	for _, s := range ss {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// splitFilter parses the board filter into an @project fragment and the leftover
// free text, both lowercased. `@`-prefixed tokens contribute to the project match
// (the last one wins if several are given); everything else joins the text match.
func splitFilter(q string) (project, text string) {
	var textParts []string
	for _, tok := range strings.Fields(q) {
		if strings.HasPrefix(tok, "@") {
			if p := strings.TrimPrefix(tok, "@"); p != "" {
				project = strings.ToLower(p)
			}
			continue
		}
		textParts = append(textParts, tok)
	}
	return project, strings.ToLower(strings.Join(textParts, " "))
}

// badge returns the card's true pipeline STATUS — never a session-liveness word.
// "running" is NOT a status: whether a tmux/claude session is live is a separate
// signal (the green ● name-row dot + the header "● N session" count), decoupled
// here so the pill always reads the real state (a shipped card reads "shipped"
// even while its just-finished gogo-done-<slug> pane lingers; an in-flight card
// reads "review r2", not a "running" that hides its phase). Precedence:
//
//  1. waiting-for-user — a parked decision gate / mid-UAT re-plan (status always
//     wins; a re-plan stays waiting-for-user for the whole stretch, REV-004).
//  2. awaiting-uat — the UAT gate (0.11.0): phase ⑤ left the feature ready but
//     unshipped, pending the user's sign-off (state.md status awaiting-uat).
//  3. awaiting-plan-acceptance — the plan-acceptance gate: surfaced as its own
//     state name so a plan-pending card reads as a gate, not "plan r1" (FR-B2).
//  4. state.md is the current-phase source of truth (it drives the card's
//     column). The latest events.jsonl line only ENRICHES the badge with a
//     round, and only when its phase agrees with state.md's current phase
//     (mapping state.md's fifth phase "knowledge" → events' "report"). When the
//     telemetry lags state.md — a gap docs/cli-contract.md §5 calls normal — the
//     badge is derived from state.md alone (phase + the iterations round), so it
//     never disagrees with its own column. A shipped feature falls through to its
//     "shipped" status here (this is what un-hides it from the old "running").
//
// Older/raw features with no state.md phase fall back to the latest event, then
// to the state.md status, so a bare events-only feature still shows something.
func badge(f *contract.Feature) string {
	if f.WaitingForUser() {
		return "waiting-for-user"
	}
	if f.AwaitingUAT() {
		return "awaiting-uat"
	}
	// The plan-acceptance gate: surface its state name like the other two gates
	// (it had no distinct badge before — FR-B2). Mutually exclusive with the
	// statuses above, so this does not disturb their precedence.
	if f.Status == "awaiting-plan-acceptance" {
		return "awaiting-plan-acceptance"
	}
	phase := f.Phase
	if phase == "" {
		if e := f.LatestEvent; e != nil {
			return phaseRound(e.Phase, e.Round, e.HasRound)
		}
		return f.Status
	}
	// Round: prefer the latest event's round when it agrees with state.md's
	// current phase; otherwise the round recorded in state.md's iterations line.
	if e := f.LatestEvent; e != nil && e.HasRound && contract.EventsPhase(phase) == e.Phase {
		return phaseRound(phase, e.Round, true)
	}
	if r := f.RoundFor(phase); r > 0 {
		return phaseRound(phase, r, true)
	}
	if f.Status != "" && f.Status != phase {
		return f.Status
	}
	return phase
}

// phaseRound renders a phase badge with an optional "rN" round suffix.
func phaseRound(phase string, round int, hasRound bool) string {
	if hasRound {
		return fmt.Sprintf("%s r%d", phase, round)
	}
	return phase
}

// --- redesign: status pills + the live agent chip (cockpit-lean-cards) ---
//
// badge() stays the canonical status producer; pillLabel/pillStyleFor transform
// it into the FR-3 status chip. activeAgent names the live session's agent for the
// FR-6 chip. All pure, all substring-assertable (no TTY under `go test` → lipgloss
// emits plain text).

// activeAgent maps a card's current pipeline phase to the short, lowercase agent
// label the FR-6 live chip shows. state.md's fifth phase is "knowledge" while
// events.jsonl labels it "report" (contract.EventsPhase) — both are the report
// step, so both map to "reporter" (a display label; there is no gogo-reporter
// agent). When f.Phase is empty (a live card whose telemetry momentarily lags) it
// falls back to the status so the chip still names its agent. done/unknown → ""
// (no chip).
func activeAgent(f *contract.Feature) string {
	switch f.Phase {
	case "plan":
		return "analyst"
	case "implement":
		return "developer"
	case "review":
		return "reviewer"
	case "test":
		return "tester"
	case "knowledge", "report":
		return "reporter"
	case "done":
		return ""
	}
	switch f.Status {
	case "implementing":
		return "developer"
	case "reviewing":
		return "reviewer"
	case "testing":
		return "tester"
	}
	return ""
}

// isChangelogCol reports whether board column i is the collapsed changelog list.
func isChangelogCol(i int) bool { return columnOrder[i] == contract.ColChangelog }

// pillLabel is the FR-3 chip text: badge() stays the canonical status producer
// (its tests + the drill/status line depend on it); this transform maps the gate
// states to their answer-first chip wording and passes everything else through.
// A mid-UAT re-plan (waiting-for-user carrying a "UAT round N" open-decision)
// reads "re-planning · UAT N" instead of the generic "decision", so the card says
// what the analyst is doing rather than looking like a stuck decision gate.
func pillLabel(f *contract.Feature) string {
	switch b := badge(f); b {
	case "awaiting-plan-acceptance":
		return waitingMarker + " accept plan"
	case "awaiting-uat":
		return waitingMarker + " awaiting-uat"
	case "waiting-for-user":
		if isUATReplan(f) {
			if n := uatRound(f); n > 0 {
				return fmt.Sprintf("%s re-planning · UAT %d", waitingMarker, n)
			}
			return waitingMarker + " re-planning"
		}
		return waitingMarker + " decision"
	default:
		return b // implement r1 · review r2 · plan-accepted · shipped · phase names
	}
}

// uatRound parses the round N from a mid-UAT re-plan's open-decision, which the
// orchestrator sets to "UAT round N" when it locks the gate (skills/gogo/SKILL.md).
// 0 when absent/unparseable — including a generic decision gate (open-decision
// "D<n>"), which is exactly how isUATReplan tells the two apart.
func uatRound(f *contract.Feature) int {
	od := strings.ToLower(f.OpenDecision)
	i := strings.Index(od, "uat")
	if i < 0 {
		return 0
	}
	// Return the first whole integer that appears after "uat". Scanning one past
	// the end closes a trailing digit run; start<0 keeps the run's first index so a
	// multi-digit round (e.g. "UAT round 10") is read whole.
	rest := od[i+3:]
	start := -1
	for j := 0; j <= len(rest); j++ {
		isDigit := j < len(rest) && rest[j] >= '0' && rest[j] <= '9'
		switch {
		case isDigit && start < 0:
			start = j
		case !isDigit && start >= 0:
			if n, err := strconv.Atoi(rest[start:j]); err == nil {
				return n
			}
			start = -1
		}
	}
	return 0
}

// isUATReplan reports whether a waiting-for-user card is parked in a UAT re-plan
// stretch (analysis → revision → re-acceptance) rather than a generic decision
// fork. The "UAT round N" open-decision is the precise discriminator.
func isUATReplan(f *contract.Feature) bool {
	return f.WaitingForUser() && uatRound(f) > 0
}

// pillStyleFor picks the tinted chip style for a card's status pill, mirroring
// badge()'s own precedence so the color always agrees with pillLabel: red for a
// decision/plan gate (incl. a UAT re-plan), purple for the uat gate, amber for an
// in-flight phase round, dim otherwise. Session liveness is NOT a status, so it
// does not tint the pill — the green ● name-row dot carries that signal instead.
func pillStyleFor(f *contract.Feature) lipgloss.Style {
	switch {
	case f.WaitingForUser():
		return pillRed
	case f.AwaitingUAT():
		return pillPurple
	case f.Status == "awaiting-plan-acceptance":
		return pillRed
	}
	switch f.Phase {
	case "implement", "review", "test":
		return pillAmber
	}
	return pillDim
}

// stripeAccent is the FR-5 left-stripe decision, independent of focus: purple for
// the uat gate, red for any other gate (plan-acceptance / decision), (nil,false)
// for a flowing card. The renderer recolors the heavy-`┃` gateBorder with it.
func stripeAccent(f *contract.Feature) (lipgloss.TerminalColor, bool) {
	switch {
	case f.AwaitingUAT():
		return uatAccent, true
	case f.WaitingForInput():
		return waitAccent, true
	}
	return nil, false
}

// needsYouCount counts the cards parked at a user gate across all four columns —
// the header's "⏸ K need you" data source (the last non-test caller of the removed
// gate enumerator). The left-border stripe (stripeAccent) is now the per-card cue.
func (m Model) needsYouCount() int {
	n := 0
	for i := 0; i < 4; i++ {
		for _, f := range m.cols[i] {
			if f.WaitingForInput() {
				n++
			}
		}
	}
	return n
}

func hasLiveSession(slug string, sessions []string) bool {
	return liveSessionFor(slug, sessions) != ""
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// colTitleStyle is the top/section title style. Per-column card + accent styles
// live in styles.go (precomputed once). View stays substring-assertable because
// go test has no TTY, so lipgloss emits plain text.
var colTitleStyle = lipgloss.NewStyle().Bold(true)
