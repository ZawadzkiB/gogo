// Command gogo is the deterministic cockpit for a project's gogo pipeline: a
// Bubble Tea kanban board (run with no args) plus scriptable subcommands
// (status, view, events). It reads the frozen file contract in docs/
// cli-contract.md with no LLM in the read path, and delegates every
// state-changing action to Claude via attachable tmux sessions.
package main

import (
	"fmt"
	"os"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// Version mirrors the plugin version (.claude-plugin/plugin.json). A breaking
// change to the CLI contract bumps both together.
const Version = "0.14.0"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runBoard())
	}
	switch args[0] {
	case "--version", "-v", "version":
		fmt.Printf("gogo %s\n", Version)
	case "run":
		os.Exit(cmdRun(args[1:]))
	case "status":
		os.Exit(cmdStatus(args[1:]))
	case "view":
		os.Exit(cmdView(args[1:]))
	case "events":
		os.Exit(cmdEvents(args[1:]))
	case "trash":
		os.Exit(cmdTrash(args[1:]))
	case "-h", "--help", "help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "gogo: unknown command %q\n\n", args[0])
		printHelp()
		os.Exit(2)
	}
}

// runBoard opens the interactive TUI. Returns a process exit code.
func runBoard() int {
	root, err := contract.FindRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gogo: no .gogo/ found from here up — run inside a gogo project (or use /gogo:build)")
		return 1
	}
	p := tea.NewProgram(tui.New(root), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "gogo:", err)
		return 1
	}
	return 0
}

func printHelp() {
	fmt.Print(`gogo — the deterministic cockpit for your gogo pipeline

usage:
  gogo                 open the kanban board (plan | in progress | ready | changelog)
  gogo run [<slug>]    orchestrate ②→③→④(→⑤) over claude -p — dev warm via --resume, review/test fresh
  gogo status          print the work-index classifier table
  gogo view <target>   view a plan/report — glamour in the terminal, or --web for the browser
  gogo events <slug>   print a feature's events.jsonl timeline
  gogo trash           list .gogo/trash/ entries (deleted work, recoverable)
  gogo trash restore <entry>   move a trashed entry back to .gogo/work/
  gogo --version       print the version (mirrors the plugin)

gogo run flags:
  --attach             on a decision gate, launch an interactive /gogo:resume session
  (env: GOGO_RUN_MAX_ROUNDS, GOGO_RUN_COST_CEILING — bounds that gate; see "gogo run --help")

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
  l peek log · x delete→trash · / filter · G glow · q quit
  ⏸ marks a card waiting on you (plan-acceptance / decision / UAT gate)

launch permission mode (FR8): board-launched claude sessions run in auto
(classifier) permission mode; set GOGO_CLAUDE_PERMISSION_MODE to override (any
claude --permission-mode value; empty string omits the flag → claude prompts).
`)
}

// findRoot resolves the project root or exits with a helpful message.
func findRoot() (string, bool) {
	root, err := contract.FindRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gogo: no .gogo/ found from here up — run inside a gogo project")
		return "", false
	}
	return root, true
}
