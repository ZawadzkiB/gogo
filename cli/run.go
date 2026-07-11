package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
)

const runHelp = `gogo run — the CLI process-orchestrator

usage:
  gogo run [<slug>] [--attach]

Drives a feature's ②→③→④(→⑤) loop by spawning each phase as its own ` + "`claude -p`" + ` session:
the DEVELOPER session is kept warm across fix rounds via --resume (never re-reads the
codebase), REVIEW and TEST are spawned FRESH (fresh eyes). It coexists with the in-chat
/gogo:go orchestrator over the same phase skills + typed contracts. Stops at awaiting-uat
(run /gogo:done to ship). Needs the claude CLI on PATH; --attach needs tmux.

  <slug>      the feature to run (default: the newest plan-accepted / mid-pipeline one)
  --attach    on a decision gate, launch an interactive /gogo:resume session to answer live

env:
  GOGO_RUN_MAX_ROUNDS     TOTAL fix-round budget per feature (default 3; hitting it gates)
  GOGO_RUN_COST_CEILING   per-feature USD ceiling (default 10.00; 0 disables; gates)
  GOGO_CLAUDE_PERMISSION_MODE   permission mode for spawned sessions (default: auto)

exit: 0 = green (awaiting-uat) · 2 = paused at a decision gate · 1 = error
`

// cmdRun is the `gogo run` entry (FR1): enforce the SAME acceptance gate /gogo:go
// uses, then hand off to the orchestrator loop.
func cmdRun(args []string) int {
	attach := false
	slug := ""
	for _, a := range args {
		switch {
		case a == "--attach":
			attach = true
		case a == "-h" || a == "--help":
			fmt.Print(runHelp)
			return 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "gogo run: unknown flag %q\n", a)
			return 1
		case slug == "":
			slug = a
		}
	}

	root, ok := findRoot()
	if !ok {
		return 1
	}
	if !launch.HasClaude() {
		fmt.Fprintln(os.Stderr, "gogo run: claude CLI not on PATH — the orchestrator spawns `claude -p` sessions")
		return 1
	}

	repo, _ := contract.LoadRepo(root)
	if slug == "" {
		f := newestRunnable(repo)
		if f == nil {
			fmt.Fprintln(os.Stderr, "gogo run: no runnable feature — need a plan-accepted or mid-pipeline one (run /gogo:plan and accept a plan first)")
			return 1
		}
		slug = f.Slug
	}
	f := repo.Feature(slug)
	if f == nil {
		fmt.Fprintf(os.Stderr, "gogo run: no feature %q under .gogo/work/\n", slug)
		return 1
	}
	if !orchestrator.RunnableStatus(f.Status) {
		fmt.Fprintf(os.Stderr, "gogo run: feature %q is %q — not runnable here. %s\n", slug, f.Status, runnableHint(f.Status))
		return 1
	}

	cfg := orchestrator.ConfigFromEnv(os.Stdout, attach)
	o := orchestrator.New(root, slug, cfg)
	fmt.Printf("gogo run %s — ②→③→④(→⑤) over claude -p (dev warm via --resume, review/test fresh)\n", slug)
	outcome, err := o.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gogo run:", err)
		return 1
	}
	if outcome.Result == orchestrator.ResultGated {
		return 2 // paused for a human decision
	}
	return 0
}

// newestRunnable returns the newest-first feature whose status permits a run, or nil.
func newestRunnable(repo *contract.Repo) *contract.Feature {
	if repo == nil {
		return nil
	}
	for _, f := range repo.Features { // LoadRepo sorts newest-first
		if orchestrator.RunnableStatus(f.Status) {
			return f
		}
	}
	return nil
}

// runnableHint mirrors /gogo:go's guidance for a non-runnable status.
func runnableHint(status string) string {
	switch status {
	case "awaiting-uat":
		return "it's at the UAT gate — run /gogo:done to ship, or give feedback to loop it back."
	case "waiting-for-user":
		return "it's paused on a decision — resolve it and re-accept (→ plan-accepted) first."
	case "shipped", "done":
		return "it's already shipped."
	default:
		return "run /gogo:plan and accept a plan first."
	}
}
