// Package tui is the Bubble Tea cockpit: a 4-column kanban board over the
// deterministic contract reader (plan | in progress | ready | changelog),
// drill-in file viewing (glamour / issues table / events timeline / ASCII
// diagrams), native `w` page builds, and column moves that launch Claude via
// the launch package (never mutating pipeline state directly). The model's
// Update/View are pure and unit-tested without a tty.
package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	"github.com/charmbracelet/bubbles/key"
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
	// modeSessions is the S sessions panel (0.33.0): a full-screen list of EVERY
	// live gogo-* session (modelled on modeDrill — it stays open, unlike a picker),
	// from which R re-assigns the focused session onto a work item and K closes it.
	modeSessions
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

// statusLevel is the severity of a status-line message (FR3.2). Today every
// outcome - a cap bounce, a dangling plan target, a tmux failure and a plain
// success - rendered through the same faint grey, so the user could not tell
// "blocked" from "failed". The zero value is OK, which keeps every unclassified
// call site byte-for-byte on the old dim voice.
type statusLevel int

const (
	statusLevelOK   statusLevel = iota // dim - it worked (the existing voice)
	statusLevelWarn                    // amber - blocked / a gate: name the unblock
	statusLevelErr                     // red - it failed: carry the real error's words
)

// setStatus records a message AND its severity. The three shorthands below are
// what the launch sites use, so classifying an outcome is a one-word change.
func (m *Model) setStatus(level statusLevel, s string) {
	m.status, m.statusLevel = s, level
}

// statusFailed marks a failure (red): a launch that errored, a page build that
// blew up - always carrying the underlying error's own words.
func (m *Model) statusFailed(s string) { m.setStatus(statusLevelErr, s) }

// statusBlocked marks a refusal/gate (amber): a cap bounce, a dangling plan
// target, a missing claude - always carrying how to unblock it.
func (m *Model) statusBlocked(s string) { m.setStatus(statusLevelWarn, s) }

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
	// Per-source gate-skip toggles (FR4): opt this source out of the plan-acceptance /
	// UAT gate. Bools bound heap-stably to two huh Confirm fields (TEST-001).
	srcPlanSkip bool
	srcUatSkip  bool
	// Plans-tab new-plan form field (FR10 `n`): the plan title as a STRING the huh
	// input binds heap-stably (TEST-001).
	planTitle string
	// Plans-tab description/goal Text-field values, bound heap-stably (TEST-001): planDesc
	// is the `n` quick-draft's optional description; planGoal is the `A` plan-with-claude
	// goal (what to build/change across the project's sources) captured before minting.
	planDesc string
	planGoal string
	// Project-first minting (project-first-plan-authoring FR1): planProject is the
	// destination project the mint-form Select writes (pre-seeded to the focused
	// project); planAttach is the attachments Text value — one path/URL per line
	// (FR4). Both heap-stable like every other field (TEST-001).
	planProject string
	planAttach  string
	// Config-tab project label-color form field (cockpit-colors FR4): the project's
	// origin color (hex or a swatch name) as a STRING bound heap-stably (TEST-001).
	projColor string
}

// gogoKeyMap is huh's default keymap with ONLY the Text group rebound
// (project-first-plan-authoring FR3): `enter` INSERTS a newline instead of
// advancing (measured against the vendored huh v1.0.0, whose Text.Next is
// {"tab","enter"} — typing a newline was impossible; paste already worked),
// `tab` advances, `shift+tab` goes back, `ctrl+d` submits. `shift+enter` is
// included KNOWINGLY INERT (FR3.3): no bubbletea v1.3.10 key renders as
// "shift+enter" on common terminals — it is future-proofing, nothing more.
// Every other field group is untouched, so Input/Select/Confirm keep `enter`
// exactly as today.
func gogoKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Text.NewLine = key.NewBinding(key.WithKeys("enter", "shift+enter", "alt+enter", "ctrl+j"), key.WithHelp("enter", "new line"))
	km.Text.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next"))
	km.Text.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back"))
	km.Text.Submit = key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "submit"))
	return km
}

// newForm is the ONE construction site for every gogo huh form — huh.NewForm plus
// gogoKeyMap. All 12 form sites use it (FR3.2): ten contain no Text field, so
// their behaviour is provably unchanged — the blanket swap is what stops the next
// Text field somebody adds from silently regressing to "enter submits"
// (enumeration-drift, this repo's recorded top trap).
func newForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithKeyMap(gogoKeyMap())
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

// projectEdit marks an in-flight config-tab project label-color form (cockpit-colors
// FR4): name is the project the color write targets. Analogous to sourceEdit but for the
// project-level Color field.
type projectEdit struct {
	name string
}

// planDoneEdit marks an in-flight plans-tab project-UAT accept confirm (FR3, `D`):
// project + id name the CLI-owned plan the MarkDone accept targets; title is shown in
// the confirm. Analogous to sourceEdit/projectEdit but for the project-UAT gate.
type planDoneEdit struct {
	project string
	id      string
	title   string
}

// planSpawnEdit marks an in-flight plans-tab accept+spawn confirm (0.25.0 FR2, `r`):
// project + id name the CLI-owned plan the auto-spawn fans out from; targets is the set
// of UN-spawned target sources the confirm will launch a `/gogo:plan` into. Analogous to
// planDoneEdit but for the accept-and-spawn gate.
type planSpawnEdit struct {
	project string
	id      string
	title   string
	targets []string
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
	adoptCancel  = " adopt-cancel"  // cancel the R re-assign (adopt) picker
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
	tab           tabID
	project       *projects.Project
	allProjects   []projects.Project
	sourceColors  map[string]string // source label → resolved never-blank color hex
	projectColors map[string]string // project name → resolved never-blank color hex (D5 combo)

	// Config tab (FR9): the project-switcher cursor + the per-source cursor + the
	// in-flight per-source edit marker. Reads/writes ONLY ~/.gogo/… via the projects
	// store (never a source's .gogo/).
	projIdx        int
	sourceIdx      int
	pendingSource  *sourceEdit
	pendingProject *projectEdit // in-flight project label-color form (cockpit-colors FR4)

	// Plans tab (plans-board FR1): the focused project's plans rendered as a 4-column
	// KANBAN (drafts·ready·active·done) mirroring the work board — planCols partitions
	// m.plans by lifecycle status, planColIdx/planCardIdx are the board-style column+card
	// cursors, planColOffset the per-column scroll window. planDetail (nil = the kanban)
	// drills into a plan; planSourceIdx is its target-source cursor. Reads/writes ONLY
	// ~/.gogo/… via the plans store; spawning a work item is a claude -p launch (never a
	// source's .gogo/ write).
	plans         []plans.Plan
	planCols      [4][]plans.Plan
	planColIdx    int
	planCardIdx   [4]int
	planColOffset [4]int
	planDetail    *plans.Plan
	planSourceIdx int
	// planViewing marks the open viewer as a PLAN view (FR2.2), mirroring the
	// existing `peeking` flag. It is load-bearing: updateViewer's esc otherwise sets
	// mode = modeDrill, and a plan `v` has no drilled card - the return path must land
	// back on the plans tab instead.
	planViewing           bool
	pendingPlan           bool
	pendingPlanWithClaude bool           // in-flight `A` goal form (0.25.1) — mint+launch+attach on submit
	pendingPlanDone       *planDoneEdit  // in-flight project-UAT accept confirm (plans-board `m` on active)
	pendingPlanSpawn      *planSpawnEdit // in-flight go/spawn confirm (plans-board `m` on ready)

	// autoPickedUp is the fire-once set for the reload-driven auto-pickup (plans-board
	// FR6, D4=B): the composite featureKey of every member work item this cockpit has
	// already auto-launched a `/gogo:go` for, so a later reload never relaunches it.
	// Cap-skipped (repo-busy) members are NOT recorded — a freed slot auto-fires on the
	// next reload. Nil until the first pickup (never keyed on a single-repo board).
	autoPickedUp map[string]bool

	// unified marks the multi-project cockpit board (0.23.0): the board aggregates
	// EVERY registered project (LoadWorkspace) rather than one project's sources
	// (LoadProject). It gates the project-chip filter row + the `●project ●source`
	// two-dot origin (every feature carries a Project only in this mode) and points
	// reload()/capBounce/watchDirs at the whole workspace. false = the single-repo
	// board (m.root != "") or the legacy single-project test seam.
	unified bool

	// projectChip is the active board PROJECT filter (FR3, D3=A): "" = all projects,
	// else the project name the `p`-cycled chip narrows the board to. Unified board
	// only (source narrowing survives via the per-card source dot + the free-text
	// `@name` token). Distinct from the free-text filter (m.filter) — both AND in
	// rebuild. The interactive source-chip row it supersedes is retired.
	projectChip string

	cols      [4][]*contract.Feature
	colIdx    int
	cardIdx   [4]int
	colOffset [4]int          // per-column scroll offset (first visible card) — TEST-014
	selected  map[string]bool // selected ready-to-ship cards, keyed by featureKey (Root\x00Slug — workspace-unique, REV-001)

	filter    string
	filtering bool
	status    string
	// statusLevel is the SEVERITY of the current status line (FR3.2). The zero value
	// is statusLevelOK, so any site that just assigns m.status keeps today's dim
	// voice; Update resets it on every keypress, so a stale severity can never
	// re-colour an unrelated message.
	statusLevel statusLevel

	showAllKeys bool // FR-10: ? toggles the full key list under the contextual footer

	width, height int
	mode          mode

	sessions []string
	// sessionMeta is the live gogo-* sessions WITH their tmux facts (anchor path /
	// created / attached), refreshed alongside m.sessions (session tick + reload) —
	// what the `R` adopt picker rows, the sessions panel, and the unbound-session
	// count read (FR3/FR4).
	sessionMeta []launch.SessionMeta
	// Sessions panel state (0.33.0): sessIdx is the panel's cursor into
	// m.sessionMeta (clamped on every tick — the list is live); sessionsOrigin is
	// the mode `S` was pressed in (board or drill), so esc/q return exactly there.
	sessIdx        int
	sessionsOrigin mode

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
	form          *huh.Form
	pending       launch.Intent
	pendingShip   bool
	pendingDelete *contract.Feature // FR6: the card a confirmed `x` moves to trash
	pendingKill   []string          // FR-B3: the focused/drilled card's live session(s) a confirmed `K` kills
	pendingAttach []string          // the attach picker's candidate sessions (≥2 live) — FR-2
	// pendingPlanSession marks an in-flight `P` plan-session confirm: the card whose
	// /gogo:plan session a completed confirm launches THEN attaches (session-binding
	// ops FR1). pendingAdopt marks an in-flight `R` adopt picker: the card the chosen
	// live session is renamed onto (FR3). Both nil when no such form is open.
	pendingPlanSession *contract.Feature
	pendingAdopt       *contract.Feature
	// pendingReassign marks the sessions panel's in-flight `R` target picker
	// (0.33.0 FR4): the live session name the chosen work item is renamed FOR.
	// pendingKillSession is the panel's `K` confirm target. "" when no such form
	// is open. Both route back to the panel via pickerOrigin.
	pendingReassign    string
	pendingKillSession string
	// pickerOrigin is the mode a kill/attach/adopt picker (or a P confirm) was opened
	// FROM — set where each picker starts, so cancel/finish restore exactly that mode.
	// It replaces the pickerFromDrill bool, which inferred the origin from
	// `m.drill != nil` — stale for a board-originated picker after an earlier drill
	// visit, landing the user in a drill they never opened (FR2's in-passing fix).
	pickerOrigin mode
	binding      *formBinding // heap-stable targets for the live huh fields

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

	// renamer re-binds a live tmux session onto a work item by renaming it to the
	// conventional base name (collision-suffixed), returning the final name. A seam
	// (defaults to launch.RenameSessionUnique) so the `R` adopt wiring is asserted
	// with a fake — never nil once a Model comes from New.
	renamer func(old, base string) (string, error)

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

// NewCockpit is the real entrypoint for `gogo global` (0.23.0): it builds the UNIFIED
// board across EVERY registered project (contract.LoadWorkspace) — each feature tagged
// with BOTH its project + source. projs[0] is the DEFAULT focus for the plans/config
// tabs (the project-chip / config switcher share this m.project, D4). It replaces
// NewProjectBoard(projs[0]) at the two call sites (main.chooseBoard, global.globalBoard).
// A single registered project degrades cleanly (the project chip row collapses to
// `all` + one). Callers guard against an empty project set (a friendly hint), but this
// stays crash-safe with none.
func NewCockpit(projs []projects.Project) Model {
	var focus *projects.Project
	var repo *contract.Repo
	if len(projs) > 0 {
		focus = &projs[0]
		repo = contract.LoadWorkspace(projs)
	} else {
		repo = &contract.Repo{}
	}
	m := newFromRepo(repo, "", focus, projs)
	m.unified = true
	return m
}

// NewWorkspaceAll is the unified-board test seam: it injects a MERGED in-memory
// *contract.Repo (features already carrying Project + Source, as LoadWorkspace stamps
// them) plus the full project set, so a test drives the aggregate board — project chip
// filter, two-dot origin, cross-project cap/watch — without disk. projs[0] is the
// default focus. The real entrypoint is NewCockpit.
func NewWorkspaceAll(repo *contract.Repo, projs []projects.Project) Model {
	var focus *projects.Project
	if len(projs) > 0 {
		focus = &projs[0]
	}
	m := newFromRepo(repo, "", focus, projs)
	m.unified = true
	return m
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
		renamer:     launch.RenameSessionUnique,
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
		m.projectColors = projectColorMap(all)
		for i := range all {
			if all[i].Name == project.Name {
				m.projIdx = i // start the config-tab switcher on the focused project
				break
			}
		}
	}
	m.refreshSessions()
	m.loadPlans() // load the project's plans for the plans tab (project board only)
	m.rebuild()
	return m
}

// refreshSessions re-reads the live tmux session names AND their meta (anchor
// path / created / attached) together, so every reader of one sees the other
// move in the same tick (session-binding ops FR3/FR4).
func (m *Model) refreshSessions() {
	m.sessions = launch.ListSessions()
	m.sessionMeta = launch.ListSessionMeta()
}

// sources returns the focused project's sources (nil in single-repo mode) — what the
// config tab reads.
func (m *Model) sources() []projects.Source {
	if m.project == nil {
		return nil
	}
	return m.project.Sources
}

// capWatchSources is the source set the concurrency-cap guard (capBounce) + the
// fsnotify watch (watchDirs) resolve against (FR5). On the UNIFIED board a card's
// source can live in a NON-focused project, so both must span EVERY project's sources
// (projects.AllSources) — resolving from only the focused project (m.sources()) left
// such a card uncapped + unwatched. Off the unified board it stays the focused
// project's sources (nil in single-repo mode), so those paths are byte-for-byte. One
// helper so the cap guard and the watch never drift.
func (m *Model) capWatchSources() []projects.Source {
	if m.unified {
		return projects.AllSources(m.allProjects)
	}
	return m.sources()
}

// projectChips is the ordered set of PROJECT-filter chip labels (FR3, D3=A): "all"
// first, then one per registered project. nil off the unified board (no chips), so the
// single-repo + legacy single-project seams stay byte-for-byte (source-narrowing there
// never had a project row).
func (m *Model) projectChips() []string {
	if !m.unified {
		return nil
	}
	out := []string{""} // "" renders as the "all" chip
	for _, p := range m.allProjects {
		out = append(out, p.Name)
	}
	return out
}

// cycleProjectChip advances the board PROJECT filter (FR3 `p`): all → proj-1 → … → all,
// narrowing the board to that project's features. Per D4 it ALSO moves the shared focus
// (m.project) to the chip's project — or to allProjects[0] when returning to "all" — so
// the plans/config tabs act on the chip's project. A no-op off the unified board / with
// no projects.
func (m *Model) cycleProjectChip(dir int) {
	chips := m.projectChips()
	if len(chips) <= 1 {
		return
	}
	cur := 0
	for i, c := range chips {
		if c == m.projectChip {
			cur = i
			break
		}
	}
	m.projectChip = chips[((cur+dir)%len(chips)+len(chips))%len(chips)]
	m.focusProject(m.projectChip) // D4: the board chip + config switcher share m.project
	m.rebuild()
}

// focusProject points the shared focus (m.project / projIdx) at the named project — or
// allProjects[0] when name is "" (the board's "all" chip) — re-deriving the source
// colors, clamping the source cursor, and resetting the plans-tab cursor/detail (a
// different plan set). It does NOT re-read the repo (the unified board already holds
// every project's features; the projectChip filters them) — the caller rebuilds. This
// is the single focus mover the board project chip and the config switcher share (D4).
func (m *Model) focusProject(name string) {
	if len(m.allProjects) == 0 {
		return
	}
	idx := 0
	if name != "" {
		for i := range m.allProjects {
			if m.allProjects[i].Name == name {
				idx = i
				break
			}
		}
	}
	m.projIdx = idx
	m.project = &m.allProjects[idx]
	m.sourceColors = sourceColorMap(m.project.Sources)
	m.sourceIdx = clamp(m.sourceIdx, 0, len(m.project.Sources)-1)
	// A different project = a different plan set: reset the kanban cursors + close any
	// open detail so the plans tab starts fresh on the new project.
	m.planColIdx = 0
	m.planCardIdx = [4]int{}
	m.planColOffset = [4]int{}
	m.planDetail = nil
	m.loadPlans()
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
	m.projectColors = projectColorMap(all)
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

// switchProject points the shared focus at allProjects[idx] (the config-tab `p`
// switcher, D4), re-deriving sources/colors and reloading. Clamps to range; a no-op
// with no projects. On the unified board it ALSO narrows the board project chip to the
// focused project so the board + config never disagree (they share m.project); on the
// legacy single-project seam reload() re-aggregates to the newly focused project
// (LoadProject), the pre-0.23 behaviour.
func (m *Model) switchProject(idx int) {
	if len(m.allProjects) == 0 {
		return
	}
	idx = ((idx % len(m.allProjects)) + len(m.allProjects)) % len(m.allProjects)
	m.projectColors = projectColorMap(m.allProjects)
	m.focusProject(m.allProjects[idx].Name)
	if m.unified {
		m.projectChip = m.project.Name // board chip follows the config switcher (D4)
	}
	m.reload()
}

// loadPlans reads the focused project's plans (plans-board FR1) for the plans tab and
// re-partitions them into the kanban columns (rebuildPlans clamps the cursors). Project
// board only — a lone repo has no plans tab, so it degrades to an empty slice (never a
// crash). Run at construction and on every reload.
func (m *Model) loadPlans() {
	if m.project == nil {
		m.plans = nil
		m.rebuildPlans()
		return
	}
	m.plans, _ = plans.List(m.project.Name)
	m.rebuildPlans()
}

// rebuildPlans partitions the focused project's plans into the 4 kanban columns
// (drafts·ready·active·done, planColumnStatus order) and clamps the column/card cursors
// into range — the plans-tab analog of rebuild() over the work board. m.plans is already
// newest-first, so each column preserves that order.
func (m *Model) rebuildPlans() {
	var cols [4][]plans.Plan
	for _, p := range m.plans {
		for i, st := range planColumnStatus {
			if p.Status == st {
				cols[i] = append(cols[i], p)
				break
			}
		}
	}
	m.planCols = cols
	for i := range m.planCardIdx {
		m.planCardIdx[i] = clamp(m.planCardIdx[i], 0, len(cols[i])-1)
	}
	m.planColIdx = clamp(m.planColIdx, 0, 3)
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
	m.pruneSelection()
	known := m.knownCorrelationIDs()
	var cols [4][]*contract.Feature
	for _, f := range m.repo.Features {
		// The `p`-cycled PROJECT chip narrows to one project (FR3, D3=A); it ANDs with
		// the free-text filter. "" (all) never hides anything. Only ever set on the
		// unified board, so the single-repo + single-project seams are unaffected.
		if m.projectChip != "" && f.Project != m.projectChip {
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

// reload re-reads the repo + sessions and rebuilds, preserving filter/focus. On the
// UNIFIED board it re-runs the multi-project merge (LoadWorkspace) so a change in ANY
// project's source is picked up live; on the legacy single-project seam it re-runs the
// single-project source merge (LoadProject); in single-repo mode it re-reads the one
// root exactly as before.
func (m *Model) reload() {
	switch {
	case m.unified:
		m.repo = contract.LoadWorkspace(m.allProjects) // re-aggregate every project
	case m.project != nil:
		m.repo = contract.LoadProject(*m.project) // re-aggregate the one project's sources
	default:
		if repo, err := contract.LoadRepo(m.root); err == nil {
			m.repo = repo
		}
	}
	m.refreshSessions()
	m.loadPlans() // re-read the focused project's plans after the reload
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

// featureKey is a feature's WORKSPACE-UNIQUE identity: a slug alone is unique only
// per-source, so on the unified board two projects that share a slug (e.g. both have
// `feature-cli`) would collide (REV-001). Root is unique per source, so `Root\x00Slug`
// disambiguates them everywhere a per-feature lookup must be workspace-unique — the
// selection set, above all. The NUL separator can never appear in a path or a slug, so
// the composite is unambiguous. nil → "" (never keys a real selection).
func featureKey(f *contract.Feature) string {
	if f == nil {
		return ""
	}
	return f.Root + "\x00" + f.Slug
}

// pruneSelection drops any selection entry whose card is no longer selectable, and it
// runs on every rebuild (so on every reload). REV-027 filtered the stale entry at the
// READ, which stopped it shipping - but the entry SURVIVED, so REV-033: a card that went
// ready -> waiting-for-user (✓ gone, correctly) -> ready again came back SELECTED, with
// its ✓ restored and `m` returning /gogo:done, without the user ever pressing space. A
// selection the user did not make and cannot remember making is exactly the invisible-arm
// failure REV-027 was about; filtering hid it, pruning removes it.
func (m *Model) pruneSelection() {
	if len(m.selected) == 0 {
		return
	}
	live := map[string]bool{}
	for _, f := range m.repo.Features {
		if selectableForShip(f) {
			live[featureKey(f)] = true
		}
	}
	for k := range m.selected {
		if !live[k] {
			delete(m.selected, k)
		}
	}
}

// selectableForShip is the ONE predicate deciding whether a card may be part of a
// ship selection. Every surface that touches the selection quotes it - the `space`
// toggle, the card's ✓ marker, and the action path below - because they used to
// disagree (REV-027): the toggle and the renderer both filtered on
// ClassReadyToShip while selectedFeatures filtered on nothing. A card selected
// while ready and then RECLASSIFIED (a UAT round locks it to waiting-for-user, a
// rerun moves it back to in-progress) stayed in m.selected, vanished from the
// display because the renderer filtered it, and still shipped because the action
// path did not - `m` returned /gogo:done for a card the user could not see was
// selected, straight past the decision-gate guard that runs later in the function.
// Selection is not cleared on reload by design (it survives a refresh), so the
// filter has to live at the read, not at the write.
func selectableForShip(f *contract.Feature) bool {
	return f != nil && f.Class == contract.ClassReadyToShip
}

// selectedFeatures returns the loaded features currently selected for ship, resolved by
// their composite featureKey (so two same-slug cards from different projects never
// collapse into one - REV-001), sorted by slug for a deterministic merged-release name.
// Filtered by selectableForShip at the READ (REV-027).
func (m *Model) selectedFeatures() []*contract.Feature {
	var out []*contract.Feature
	for _, f := range m.repo.Features {
		if m.selected[featureKey(f)] && selectableForShip(f) {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// selectedSlugs returns the slugs of the selected features (composite-keyed), sorted.
// A merged ship is guaranteed same-root by attemptAction's selectionSpansProjects guard,
// so the slugs are unambiguous within that one repo.
func (m *Model) selectedSlugs() []string {
	feats := m.selectedFeatures()
	out := make([]string, 0, len(feats))
	for _, f := range feats {
		out = append(out, f.Slug)
	}
	return out
}

// matchFilter reports whether feature f matches the board filter q (FR5). The
// `@name` origin token is an AGGREGATE-board concept only: it is honored solely
// when global is true. There a leading `@fragment` narrows to features whose
// PROJECT or SOURCE label contains it (case-insensitive substring — D3=A extends it
// past the old Source-only match), the remaining non-@ words keep the slug+title
// substring match, and when both are present they AND together. In single-repo mode
// (global == false) every feature's Project + Source is ""
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
	token, text := splitFilter(rest)
	// The `@name` token narrows to a feature's PROJECT or SOURCE (D3=A fixes the
	// long-standing drift where the "project token" only ever matched Source): on the
	// unified board source labels collide across projects, so the token must reach both
	// — an @fragment hits when it is a case-insensitive substring of EITHER.
	if token != "" &&
		!strings.Contains(strings.ToLower(f.Source), token) &&
		!strings.Contains(strings.ToLower(f.Project), token) {
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
	// Still being AUTHORED (0.29.0): the status on disk reads the plan-acceptance gate
	// (or nothing at all) while plan.md is absent or a stub, so this card must NOT read
	// like a plan waiting for the user. Checked BEFORE the gate arm it shadows - and only
	// there, so the three real gates above keep their precedence untouched.
	if f.Authoring() {
		return "authoring"
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

// sessionAgent is the FR14 agent chip: activeAgent (the phase-derived label) corrected by
// what the live session is actually DOING, whenever the two disagree.
//
// The bug it fixes: state.md is written at a phase's exit, so for the whole of a build the
// file still reads `phase: plan` - and activeAgent maps that to "analyst". A card being
// BUILT therefore displayed `● analyst`, wrong in its column, its status pill AND its
// agent chip, all from the same stale phase line. A live `gogo-go-<slug>` session is
// direct evidence of the developer working, so it wins over a phase line that has not
// caught up; a live `gogo-plan-<slug>` session is the same evidence for the analyst.
// Every other action (done / accept / author / resume) is not a pipeline phase, so it
// leaves the phase-derived answer alone.
func sessionAgent(f *contract.Feature, sessions []string) string {
	if f == nil {
		return ""
	}
	switch {
	case launch.HasSessionAction(f.Slug, sessions, launch.ActionGo):
		// Only OVERRIDE when the file disagrees. In implement/review/test the phase line
		// is the more precise answer (the same warm `go` session drives all three), so a
		// reviewing card keeps `● reviewer`.
		if a := activeAgent(f); a != "" && isPipelinePhase(f.Phase) {
			return a
		}
		return "developer"
	case launch.HasSessionAction(f.Slug, sessions, launch.ActionPlan):
		return "analyst"
	}
	return activeAgent(f)
}

// isPipelinePhase reports whether f.Phase names a phase a warm `go` session genuinely
// runs, i.e. whether the phase line is more precise than "there is a build session".
func isPipelinePhase(phase string) bool {
	switch phase {
	case "implement", "review", "test", "knowledge", "report":
		return true
	}
	return false
}

// buildingDisagreement reports the FR14 live-vs-file disagreement: a live BUILD session
// says the feature is being built while its file-derived status still reads a pre-build
// state (`plan-accepted`, or the plan-acceptance gate). It covers the
// launch-to-first-write window that moving the phases' state write to phase ENTRY
// shrinks but cannot close.
//
// D6=A: the card keeps its FILE-derived column and gains a cue. Overriding the column
// from the session signal would make the TUI structurally disagree with `gogo status` and
// with every headless reader (which have no tmux), which is the split this plan rejected
// for the classifier too - one source of truth for placement, a cue for the disagreement.
func buildingDisagreement(f *contract.Feature, sessions []string) bool {
	if f == nil {
		return false
	}
	switch f.Status {
	case "plan-accepted", "awaiting-plan-acceptance":
	default:
		return false
	}
	return launch.HasSessionAction(f.Slug, sessions, launch.ActionGo)
}

// stalledPhase reports the FR15 mirror case: the feature's status says a phase is
// WORKING (implementing / reviewing / testing) but no session is live for it, so the
// session died mid-phase. With the phases writing their status at ENTRY this becomes the
// normal shape of a killed build, and it must read as stalled rather than running -
// otherwise the honest writer would trade one silent lie for another. It is resumable
// (RunnableStatus is true for all three), which is why the cue is quiet and the recovery
// is named in the footer/status line rather than shouted on the card.
func stalledPhase(f *contract.Feature, sessions []string) bool {
	if f == nil {
		return false
	}
	switch f.Status {
	case "implementing", "reviewing", "testing":
		return !hasLiveSession(f.Slug, sessions)
	}
	return false
}

// phaseLineLags reports the THIRD disagreement shape, and the one that catches a phase skill
// forgetting its entry write (REV-006). It is deterministic and file-derived: the telemetry
// contradicts the phase line while a build session is still live, so work has moved on while
// `state.md` still describes an earlier phase.
//
// Why this is worth its own cue: FR11 moves the occupancy write to phase ENTRY, but that is
// LLM prose, and it was skipped on ALL THREE live runs of this very feature - ③ kept
// `implement` / `implementing` through a whole review round three times, including twice after
// the instruction was moved into the numbered step list. The other two cues cannot see it -
// `● building` and the cap both key on a live `gogo-go` session, which is present for the WHOLE
// warm run through ②③④⑤, so a phase line lagging by one phase looks identical to a healthy one.
// `events.jsonl` is the missing evidence, and it is already loaded (Feature.LatestEvent).
//
// TWO arms, because a skipped occupancy write leaves two different traces (REV-009):
//
//	A. the newest event is a `phase-done` for the phase `state.md` names - the named phase is,
//	   per its own telemetry, finished, and nothing has claimed the next one. This catches a
//	   FORWARD hand-off (②→③, ③→④, ④→⑤) where the next phase emitted no entry event either.
//	   ARM A IS TRANSIENT-BY-DESIGN SINCE THE EXIT WRITE CAME BACK. Each phase now writes
//	   phase/status at its END as well as its start (belt and braces, because the entry write is
//	   prose that gets skipped), so every HEALTHY hand-off passes through arm A's exact shape for
//	   the gap between one phase's exit write and the next phase's entry write - a gap that
//	   includes the next phase's validate-in and, for ③/④, spawning a subagent, so seconds to
//	   about a minute. That is not a false positive in the strict sense (the phase the line names
//	   HAS ended, and nothing has claimed the next one yet), and it self-clears the moment the
//	   next phase writes - exactly how `● building` covers the launch-to-first-write window. But
//	   it does mean arm A alone is weaker evidence than arm B: it says "consistent but nothing
//	   has moved on yet", where arm B says "these two files actively disagree". When the entry
//	   write really is skipped, arm A stays lit for the whole phase instead of blinking, which is
//	   the difference the user actually sees. Both shapes are pinned in TestPhaseLineLagsCue so
//	   the trade-off is deliberate rather than inherited.
//
//	   DO NOT "FIX" THE BLINK BY DELETING ARM A, and note the sharper reason: before the exit
//	   write was restored, arm A was SILENT EXACTLY WHEN THE FILE WAS WORST. With the entry write
//	   skipped, `state.md` kept naming an earlier phase, so `e.Phase == line` failed and nothing
//	   fired at all. §④'s write is what makes arm A's precondition hold in the first place - so
//	   restoring it made this arm MORE useful, not less, and deleting it now would surrender both
//	   the blink and the detection.
//
//	   The one feasible narrowing was considered and REJECTED: gating on
//	   `time.Since(e.TS) > grace` to let the hand-off blink expire. It puts a wall clock into a
//	   pure file-derived predicate (against the read-path determinism NFR - the same reasoning
//	   that kept `classify()` host-independent), no grace constant can be chosen safely across
//	   unknown hosts and subagent spawn times, and the board re-renders on **fsnotify, not a
//	   timer**, so a grace would only move WHEN the cue appears unless a ticker were added to
//	   drive a purely cosmetic cue.
//	B. the newest event is an ENTRY event (`phase-started` / `fix-round`) for a DIFFERENT phase
//	   than `state.md` names - the telemetry asserts work began on X while the phase line still
//	   says Y. This catches the pipeline's most common re-entry, the loop BACK to implement
//	   (`fix-round`/implement while the line still reads review/reviewing), and the partial
//	   step 1 where the event landed and the `state.md` write did not.
//
// Arm B is deliberately restricted to ENTRY events rather than "any event whose phase
// disagrees". An entry event is the only kind that asserts a phase has BEGUN. The shape that
// makes the narrowing right is `implement`/`implementing` with a newest `issues-found`/review:
// there, `state.md` is CORRECT - it names the phase actually running - and only the
// best-effort telemetry is behind, so there is no lag to report. (Not, as an earlier version
// of this comment claimed, because "the writer did everything right": implement's own
// `fix-round` would be the newest event if it had. Something did fail there - the event
// append - but it is not the half this cue is about.)
//
// WHAT THE CUE CAN AND CANNOT PROVE. Arm B's shape is AMBIGUOUS by construction:
// `implement`/`implementing` + newest `phase-started`/review is byte-identical on disk whether
// (i) review started and skipped its state write - the cue is right - or (ii) implement
// re-entered, wrote its state, and its best-effort event append failed - `state.md` is correct
// and the cue blames the right file for the wrong reason. Nothing on disk separates them, so
// the honest reading of `· state lags` is narrower than "the phase line is stale": it is
// **`state.md` and `events.jsonl` disagree about the current phase - one half of step 1 did
// not land**, and the user should check which. That is still worth surfacing, because either
// half failing is a defect.
//
// And SILENCE IS NOT PROOF OF HEALTH: arm A can be masked. A phase that skips step 1 entirely
// but later appends a mid-phase event (`issues-found`/`round-opened`) overwrites the
// `phase-done` that arm A keys on, and the cue goes quiet while the phase line is still stale.
// The cue is a detector, not a guarantee - which is exactly why the writer-side rule stays in
// the skills rather than being replaced by it.
//
// The remaining conditions make false positives impossible in the healthy case:
//   - a live BUILD session - work is genuinely continuing, so some phase should have claimed
//     it (an authoring or done session is not a build and never implies a phase moved on);
//   - a WORKING status (implementing / reviewing / testing) - the statuses the entry write is
//     supposed to have set. This mirrors stalledPhase's whitelist and is what keeps a terminal
//     item with a lingering session from reading "lagging" when nothing is (REV-010): an
//     `aborted` feature whose phase is still `implement` classifies ClassInProgress and so
//     renders as a real card. A user gate is excluded by the same whitelist - `awaiting-uat`
//     legitimately sits after a completed phase with its `state.md` correct.
//
// state.md's fifth phase is `knowledge` while events call it `report`, hence EventsPhase on
// every comparison. A feature with no events.jsonl (best-effort telemetry, absent on older
// features) has a nil LatestEvent and never trips this - absence degrades to today's output.
func phaseLineLags(f *contract.Feature, sessions []string) bool {
	if f == nil || f.LatestEvent == nil || f.Phase == "" {
		return false
	}
	// The working-status whitelist also subsumes the user gates and every terminal status,
	// so it is the single status rule rather than two overlapping ones.
	switch f.Status {
	case "implementing", "reviewing", "testing":
	default:
		return false
	}
	e, line := f.LatestEvent, contract.EventsPhase(f.Phase)
	lagging := false
	switch {
	case e.Event == "phase-done" && e.Phase == line:
		lagging = true // arm A: the named phase ended, nothing claimed the next
	case isEntryEvent(e.Event) && e.Phase != line:
		lagging = true // arm B: an entry event names a phase the line does not
	}
	if !lagging {
		return false
	}
	return launch.HasSessionAction(f.Slug, sessions, launch.ActionGo)
}

// isEntryEvent reports whether an events.jsonl event name is one a phase appends as it
// BEGINS - the only kind whose phase, when it disagrees with the phase line, is evidence that
// work moved on rather than ordinary telemetry lag. `phase-started` opens a phase;
// `fix-round` marks implement re-entering to fix an issues list (its entry event in
// `--issues` mode). Every other event in the vocabulary (`phase-done`, `round-opened`,
// `issues-found`, the gate/uat/ship events) is mid- or post-phase.
func isEntryEvent(name string) bool {
	return name == "phase-started" || name == "fix-round"
}

// cardStateCue is the one producer of a card's live-vs-file cue text (glyph + word, so it
// survives a colourless terminal and is assertable in View()): `● building` for the FR14
// disagreement, `· state lags` when the telemetry contradicts the phase line while work
// continues (REV-006/REV-009), `· stalled` for the FR15 mirror, and "" for a card whose file,
// telemetry and sessions all agree. Pure, so the renderer stays free of the decision.
//
// The three arms are MUTUALLY EXCLUSIVE, so the switch order is not load-bearing - and that
// is a property worth stating precisely, because an earlier version of this comment claimed
// "order matters" and a mutation proved the claim unpinned (REV-013). What actually holds:
//
//   - buildingDisagreement needs a PRE-BUILD status (plan-accepted / awaiting-plan-acceptance)
//     plus a live build session;
//   - phaseLineLags needs a WORKING status (implementing / reviewing / testing) plus a live
//     build session - a disjoint status set, since REV-010 added that whitelist;
//   - stalledPhase needs a WORKING status and NO live session.
//
// So no two can be true at once. TestCueArmsAreMutuallyExclusive pins it over a cross-product,
// which is what turns "the order happens not to matter today" into a guard: widening any arm's
// status set into another's fails that test and forces the precedence to be decided on purpose
// rather than inherited from the switch.
func cardStateCue(f *contract.Feature, sessions []string) string {
	switch {
	case buildingDisagreement(f, sessions):
		return buildingMarker + " building"
	case phaseLineLags(f, sessions):
		return stalledMarker + " state lags"
	case stalledPhase(f, sessions):
		return stalledMarker + " stalled"
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
	case "authoring":
		// Glyph + word (FR14b): a dim `✎ authoring` where an acceptable plan shows the
		// red `⏸ accept plan`, so "the analyst is still writing this" and "your turn"
		// can never look the same.
		return authoringMarker + " authoring"
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
	case f.Authoring():
		return pillDim // not a gate: nothing to act on yet (mirrors badge()'s precedence)
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
