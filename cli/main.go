// Command gogo is the deterministic cockpit for a project's gogo pipeline: a
// Bubble Tea kanban board (run with no args) plus scriptable subcommands
// (status, view, events). It reads the frozen file contract in docs/
// cli-contract.md with no LLM in the read path, and delegates every
// state-changing action to Claude via attachable tmux sessions.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	"github.com/ZawadzkiB/gogo/cli/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// Version mirrors the plugin version (.claude-plugin/plugin.json). A breaking
// change to the CLI contract bumps both together.
const Version = "0.23.0"

func main() {
	// One-shot, best-effort, non-destructive migration of the legacy flat registry
	// into home-folder projects (FR6, D4). Guarded to run at most once and never
	// blocks startup - a no-op on every machine that already uses ~/.gogo/projects/.
	projects.Migrate()
	// Phase-C extension of the migration (FR6): fold the legacy global drafts + epics
	// stores into project plans (runs after projects exist; guarded + non-destructive).
	plans.MigrateLegacy()

	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runBoard())
	}
	switch args[0] {
	case "--version", "-v", "version":
		fmt.Printf("gogo %s\n", Version)
	case "go":
		os.Exit(cmdGo(args[1:]))
	case "plan":
		os.Exit(cmdPlan(args[1:]))
	case "sweep":
		os.Exit(cmdSweep(args[1:]))
	case "run":
		os.Exit(cmdRun(args[1:])) // deprecated alias → gogo go (FR11)
	case "status":
		os.Exit(cmdStatus(args[1:]))
	case "view":
		os.Exit(cmdView(args[1:]))
	case "events":
		os.Exit(cmdEvents(args[1:]))
	case "trash":
		os.Exit(cmdTrash(args[1:]))
	case "project":
		os.Exit(cmdProject(args[1:]))
	case "global":
		os.Exit(cmdGlobal(args[1:]))
	case "source":
		os.Exit(cmdSource(args[1:]))
	case "draft":
		os.Exit(cmdDraft(args[1:]))
	case "epic":
		os.Exit(cmdEpic(args[1:]))
	case "-h", "--help", "help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "gogo: unknown command %q\n\n", args[0])
		printHelp()
		os.Exit(2)
	}
}

// runBoard opens the interactive TUI. Returns a process exit code. It resolves
// which board to open through the two-mode chooseBoard (UAT round 1): inside a repo
// → THAT repo's single board (always); outside any repo → the global cockpit when
// the home is initialized (else a hint to `gogo global init`). `gogo global` forces
// the cockpit regardless of cwd.
func runBoard() int {
	root, err := contract.FindRoot(".")
	choice := chooseBoard(root, err == nil, projects.Home(), projects.List, projects.Initialized)
	if choice.model == nil {
		fmt.Fprintln(os.Stderr, choice.err)
		return 1
	}
	return runProgram(choice.model)
}

// boardChoice is the resolved runBoard decision (REV-003): the board Model to run,
// or - when model is nil - the stderr line to print before exiting 1. Extracting
// it makes the branch unit-testable without opening a TTY.
type boardChoice struct {
	model tea.Model
	kind  string // "project" | "single" | "none" - the branch taken (test seam)
	err   string // stderr line, set only when model == nil
}

// chooseBoard decides which board a bare `gogo` opens - the two-mode model (UAT
// round 1). Pure/no-TTY testable: the project store is injected via listProjects and
// the global-home "initialized" check via initialized, so every branch is driven
// with fakes in tests.
//
//  1. Inside a repo (rootFound) → THAT repo's single board, ALWAYS - even when the
//     repo is a registered project's source. Per-repo stays simple (no auto-route to
//     the project cockpit); the graceful single-repo board is byte-for-byte unchanged.
//  2. Outside any repo + the global cockpit initialized + ≥1 project → the UNIFIED
//     cockpit board across EVERY project (NewCockpit), `p` cycles the project filter.
//  3. Outside + initialized + 0 projects → a "add a project" hint (no crash).
//  4. Outside + NOT initialized → a "run gogo global init" hint (no crash).
//
// `gogo global` (cmdGlobal) forces the cockpit regardless of cwd; this function is
// only the bare-`gogo` resolver.
//
// FR1 (cockpit-colors): dataHome is the global data home (projects.Home() == ~/.gogo).
// contract.FindRoot walks UP looking for any .gogo/, so from ~ (or any child of ~
// without its own .gogo/) it resolves the DATA home itself and would open an empty
// single-repo board. A root whose .gogo IS the data home is therefore NOT a repo - we
// fall through to the global-cockpit path. (A real gogo repo living AT the data home is
// pathological: the data home wins → global cockpit; don't register $HOME as a source.)
func chooseBoard(root string, rootFound bool, dataHome string, listProjects func() ([]projects.Project, error), initialized func() bool) boardChoice {
	if rootFound && !sameDir(filepath.Join(root, ".gogo"), dataHome) {
		return boardChoice{model: tui.New(root), kind: "single"}
	}
	if !initialized() {
		return boardChoice{kind: "none", err: "gogo: no repo here and no global cockpit yet - run `gogo global init`, or cd into a gogo repo"}
	}
	projs, _ := listProjects() // missing / malformed store → empty, never a crash
	if len(projs) == 0 {
		return boardChoice{kind: "none", err: "gogo: global cockpit is empty - add a project with `gogo project add <repo>`"}
	}
	return boardChoice{model: tui.NewCockpit(projs), kind: "project"}
}

// sameDir reports whether two paths point at the same directory (cleaned compare) -
// the FR1 data-home guard.
func sameDir(a, b string) bool { return filepath.Clean(a) == filepath.Clean(b) }

// runProgram runs a built board Model to completion in the alt screen.
func runProgram(m tea.Model) int {
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "gogo:", err)
		return 1
	}
	return 0
}

func printHelp() {
	fmt.Print(`gogo - the deterministic cockpit for your gogo pipeline

usage:
  gogo                 open the kanban board (plan | in progress | ready | changelog) - in a repo → THAT repo; outside a repo → the global cockpit
  gogo go [<slug>]     launch-or-resume the feature's persistent /gogo:go session (implement + review/test + report)
  gogo plan <cmd>      manage project plans (new | list | show | add | rm | ready | promote | delete) - a plan targets sources & spawns work items
  gogo plan <slug>     (bare slug) launch-or-resume the feature's persistent /gogo:plan session
  gogo sweep           reap orphaned / shipped persistent sessions (kill-at-ship backstop)
  gogo status          print the work-index classifier table
  gogo view <target>   view a plan/report - glamour in the terminal, or --web for the browser
  gogo events <slug>   print a feature's events.jsonl timeline
  gogo trash           list .gogo/trash/ entries (deleted work, recoverable)
  gogo trash restore <entry>   move a trashed entry back to .gogo/work/
  gogo project <cmd>   manage home-folder projects (add <repo> [--name <n>] | list | rm <name>) - a project links many sources
  gogo global <cmd>    the global cockpit across projects (init | board) - gogo global opens it from anywhere; gogo in a repo shows THAT repo
  gogo source <cmd>    manage a project's sources - repos with .gogo/ (add <repo> [--project <name>] | rm <repo|name> [--project <name>])
  gogo draft <cmd>     alias into ` + "`gogo plan`" + ` - a draft is a plan in status draft (new | list | show | ready | rm)
  gogo epic <cmd>      alias into ` + "`gogo plan`" + ` - an epic is a plan with members (new | list | show | add <id> <source>:<slug> | rm | delete)
  gogo run [<slug>]    DEPRECATED alias for "gogo go" (forwards; will be removed)
  gogo --version       print the version (mirrors the plugin)

gogo go / gogo plan flags:
  --attach             launch an attachable tmux session (interactive claude) to answer gates live
  --takeover           seize the owner lock from a live session (the prior is reaped)
  --force              (gogo go) override the project's concurrency cap (maxConcurrent)
  (env: GOGO_CLAUDE_PERMISSION_MODE - permission mode for the spawned session; see "gogo go --help")

gogo sweep flags:
  --dry-run            list what would be reaped without killing anything

view targets:
  <slug>               the feature's report if it exists, else its plan
  <slug>:plan          the feature's plan bundle
  <slug>:report        the feature's report bundle
  <date>-<name>        a changelog entry

view flags:
  --web                build the self-contained interactive HTML page (no LLM)
  --open               with --web, open the page in a browser

board keys:
  ←→/h columns · ↑↓/jk cards · space select (ready) · enter drill-in · v quick-view
  w web page · m move/launch (accepts a plan-pending card) · d ship · a attach session
  l peek log · x delete→trash · p project chip (unified board) · tab board/plans/config · @name / #plan-<id> filter · / filter · G glow · q quit
  ⏸ marks a card waiting on you (plan-acceptance / decision / UAT gate)

plans tab keys:
  ↑↓ plans · enter open · n new · A plan-with-claude · r mark ready · x delete
  in a plan: ↑↓ target sources · c create work item (spawn /gogo:plan --correlation) · + add source · e edit · esc back

drill-in keys (enter on a card - shows description / folder / status / sessions / events):
  ↑↓/jk files · enter open file · a attach session (picker if ≥2) · K kill session (confirm; one/all picker if ≥2)
  G glow · w web page · esc/q back

launch permission mode (FR8): board-launched claude sessions run in auto
(classifier) permission mode; set GOGO_CLAUDE_PERMISSION_MODE to override (any
claude --permission-mode value; empty string omits the flag → claude prompts).
`)
}

// findRoot resolves the project root or exits with a helpful message.
func findRoot() (string, bool) {
	root, err := contract.FindRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gogo: no .gogo/ found from here up - run inside a gogo project")
		return "", false
	}
	return root, true
}
