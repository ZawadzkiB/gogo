package main

import (
	"fmt"
	"os"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
	"github.com/ZawadzkiB/gogo/cli/internal/tui"
)

const globalHelp = `gogo global - the global cockpit (a UNIFIED board · plans · config across all your projects)

usage:
  gogo global init     initialize the global cockpit home (~/.gogo/) - where your projects live
  gogo global          open the unified tabbed cockpit from anywhere (even inside a repo)
  gogo global board    alias for ` + "`gogo global`" + ` (open the cockpit)

The two modes (UAT round 1):
  - gogo INSIDE a repo (a dir with .gogo/) → THAT repo's own board - always, even
    when the repo is a registered project's source. Per-repo stays simple.
  - gogo global → the UNIFIED cockpit across EVERY project at once (each card tagged
    ` + "`●project ●source`" + `; ` + "`p`" + ` cycles the project filter), from anywhere.
  - gogo OUTSIDE any repo → the global cockpit.

Set it up once: ` + "`gogo global init`" + ` then ` + "`gogo project add <repo>`" + `. Writes ONLY ~/.gogo/.
`

// cmdGlobal dispatches the `gogo global` verb (FR19/FR20). `init` sets up the global
// cockpit home (~/.gogo/); a bare `global` (or `global board`) opens the tabbed
// cockpit from anywhere - the explicit "global mode" that complements the repo-local
// `gogo` (which always shows the repo's own board).
func cmdGlobal(args []string) int {
	if len(args) == 0 {
		return globalBoard()
	}
	switch args[0] {
	case "init":
		return globalInit()
	case "board":
		return globalBoard()
	case "-h", "--help", "help":
		fmt.Print(globalHelp)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "gogo global: unknown subcommand %q (init | board)\n", args[0])
		return 2
	}
}

// globalInit initializes the global cockpit home ~/.gogo/ (FR19): it creates the
// dir + ~/.gogo/projects/ and writes the ~/.gogo/config.json marker via
// projects.EnsureHome. Idempotent - a re-run reports "already initialized" and still
// exits 0. Prints the home path + a per-run status + the next step. It writes ONLY
// under ~/.gogo/.
func globalInit() int {
	created, err := projects.EnsureHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo global init: %v\n", err)
		return 1
	}
	if created {
		fmt.Printf("initialized the global cockpit home at %s\n", projects.Home())
	} else {
		fmt.Printf("global cockpit already initialized at %s\n", projects.Home())
	}
	projs, _ := projects.List()
	fmt.Printf("  %d project(s) registered\n", len(projs))
	fmt.Println("next: gogo project add <repo>")
	return 0
}

// globalBoard opens the global tabbed cockpit from anywhere (FR20), forcing the
// cockpit regardless of cwd. It hints instead of crashing when there is nothing to
// show: an uninitialized home → run `gogo global init`; an initialized home with 0
// projects → add a project. With ≥1 project it opens the UNIFIED board across EVERY
// project (NewCockpit) — `p` cycles the project filter; a single project degrades cleanly.
func globalBoard() int {
	if !projects.Initialized() {
		fmt.Fprintln(os.Stderr, "gogo global: no global cockpit yet - run `gogo global init` to set it up")
		return 1
	}
	projs, _ := projects.List()
	if len(projs) == 0 {
		fmt.Fprintln(os.Stderr, "gogo global: no projects yet - add one with `gogo project add <repo>`")
		return 1
	}
	return runProgram(tui.NewCockpit(projs))
}
