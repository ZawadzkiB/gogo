package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
)

// slugPattern is the canonical kebab-case feature slug (same shape the typed
// contracts use). It is the write-scope guard: an unvalidated slug flows into
// LockPath / RegistryPath and is filepath.Join'd under `.gogo/resources/`, so a
// slug carrying `..` or `/` could escape that root and breach the hard
// "only ever write under .gogo/" invariant. `gogo plan` intentionally accepts a
// brand-new slug (no feature-existence guard), so it is the live vector (REV-001).
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validSlug(slug string) bool { return slugPattern.MatchString(slug) }

const goHelp = `gogo go — launch or resume a feature's persistent pipeline session

usage:
  gogo go [<slug>] [--attach] [--takeover]

Launches (or --resumes) ONE persistent ` + "`claude -p`" + ` session running the existing
/gogo:go skill for the whole feature — implement warm in-context + review/test as
nested Task subagents + report. The CLI only manages that session's lifecycle: it
guards a one-owner lock, resolves fresh-vs-resume from the session registry,
classifies the child's exit, and reaps at ship. No phase loop, no routing in Go —
the ONE routing rule lives in the skill. Needs the claude CLI on PATH; --attach
needs tmux.

  <slug>       the feature to run (default: the newest plan-accepted / mid-pipeline one)
  --attach     launch an attachable tmux session (interactive claude) so you can
               answer decision/UAT gates live in the warm session (reaped at close)
  --takeover   seize the owner lock from a live session (the prior is reaped)

env:
  GOGO_CLAUDE_PERMISSION_MODE   permission mode for the spawned session (default: auto)

exit: 0 = green (awaiting-uat) / attached · 2 = parked at a gate · 1 = error / refused
`

const planHelp = `gogo plan — launch or resume a feature's persistent planning session

usage:
  gogo plan <slug> [--attach] [--takeover]

Launch-or-resumes ONE persistent ` + "`claude -p`" + ` session running /gogo:plan <slug>
through the same lifecycle machinery as ` + "`gogo go`" + ` (its own tracked session — plan
and go are distinct legs of the same feature's work). Writes an accept-pending plan
and stops for your acceptance. Needs the claude CLI on PATH; --attach needs tmux.
`

const sweepHelp = `gogo sweep — reap orphaned / shipped persistent sessions

usage:
  gogo sweep [--dry-run]

Kills (1) gogo-* tmux sessions whose owning feature is already terminal
(shipped/aborted) — the kill-at-ship backstop — and (2) orphans: a live gogo-*
session with no live, non-terminal owning feature. Attribution is by the exact
gogo-<action>-<slug> convention (never substring). --dry-run lists what it would
kill without touching anything. Also self-heals stale lockfiles.
`

// parseSessionFlags pulls the shared flags (--attach / --takeover / slug) out of an
// argv for gogo go / gogo plan, printing help + signalling exit on -h.
func parseSessionFlags(cmd, help string, args []string) (slug string, attach, takeover, helped bool, code int) {
	for _, a := range args {
		switch {
		case a == "--attach":
			attach = true
		case a == "--takeover":
			takeover = true
		case a == "-h" || a == "--help":
			fmt.Print(help)
			return "", false, false, true, 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "%s: unknown flag %q\n", cmd, a)
			return "", false, false, true, 1
		case slug == "":
			slug = a
		}
	}
	return slug, attach, takeover, false, 0
}

// cmdGo is the `gogo go` entry (FR1/FR3): enforce the SAME acceptance gate /gogo:go
// uses, then launch-or-resume the one persistent session via the lifecycle manager.
func cmdGo(args []string) int {
	slug, attach, takeover, helped, code := parseSessionFlags("gogo go", goHelp, args)
	if helped {
		return code
	}
	if slug != "" && !validSlug(slug) {
		fmt.Fprintf(os.Stderr, "gogo go: invalid slug %q — expected kebab-case [a-z0-9-] (no path separators)\n", slug)
		return 1
	}

	root, ok := findRoot()
	if !ok {
		return 1
	}
	if !launch.HasClaude() {
		fmt.Fprintln(os.Stderr, "gogo go: claude CLI not on PATH — the persistent session runs `claude -p`")
		return 1
	}

	repo, _ := contract.LoadRepo(root)
	if slug == "" {
		f := newestRunnable(repo)
		if f == nil {
			fmt.Fprintln(os.Stderr, "gogo go: no runnable feature — need a plan-accepted or mid-pipeline one (run /gogo:plan and accept a plan first)")
			return 1
		}
		slug = f.Slug
	}
	f := repo.Feature(slug)
	if f == nil {
		fmt.Fprintf(os.Stderr, "gogo go: no feature %q under .gogo/work/\n", slug)
		return 1
	}

	// A terminal feature: opportunistically reap its session (kill-at-ship backstop)
	// and exit friendly, rather than refuse with a bare hint.
	if orchestrator.TerminalStatus(f.Status) {
		sess := &orchestrator.Session{Root: root, Slug: slug, Kind: "go", Out: os.Stdout}
		sess.Reap()
		fmt.Printf("gogo go: %s is %s — nothing to run; reaped any tracked session.\n", slug, f.Status)
		return 0
	}
	if !orchestrator.RunnableStatus(f.Status) {
		fmt.Fprintf(os.Stderr, "gogo go: feature %q is %q — not runnable here. %s\n", slug, f.Status, runnableHint(f.Status))
		return 1
	}

	sess := &orchestrator.Session{
		Root: root, Slug: slug, Kind: "go",
		Out: os.Stdout, Attach: attach, Takeover: takeover,
	}
	fmt.Printf("gogo go %s — launch-or-resume the persistent /gogo:go session (implement in-context + Task review/test + report)\n", slug)
	return runSession(sess, "gogo go")
}

// cmdPlan is the `gogo plan` entry (FR2): launch-or-resume a persistent /gogo:plan
// session for a feature (new or in-planning) through the same lifecycle machinery.
func cmdPlan(args []string) int {
	slug, attach, takeover, helped, code := parseSessionFlags("gogo plan", planHelp, args)
	if helped {
		return code
	}
	if slug == "" {
		fmt.Fprintln(os.Stderr, "gogo plan: needs a <slug> — the feature to plan (e.g. `gogo plan my-feature`)")
		return 1
	}
	if !validSlug(slug) {
		fmt.Fprintf(os.Stderr, "gogo plan: invalid slug %q — expected kebab-case [a-z0-9-] (no path separators)\n", slug)
		return 1
	}

	root, ok := findRoot()
	if !ok {
		return 1
	}
	if !launch.HasClaude() {
		fmt.Fprintln(os.Stderr, "gogo plan: claude CLI not on PATH — the persistent session runs `claude -p`")
		return 1
	}

	repo, _ := contract.LoadRepo(root)
	status := ""
	if f := repo.Feature(slug); f != nil {
		status = f.Status
	}
	if !orchestrator.PlannableStatus(status) {
		fmt.Fprintf(os.Stderr, "gogo plan: feature %q is %q — already shipped; nothing to plan.\n", slug, status)
		return 1
	}

	sess := &orchestrator.Session{
		Root: root, Slug: slug, Kind: "plan",
		Out: os.Stdout, Attach: attach, Takeover: takeover,
	}
	fmt.Printf("gogo plan %s — launch-or-resume the persistent /gogo:plan session\n", slug)
	return runSession(sess, "gogo plan")
}

// runSession drives a lifecycle leg and maps its Outcome to a process exit code
// (FR4): green (awaiting-uat) / attached / terminal → 0 · parked at a gate → 2 ·
// refused / error → 1.
func runSession(sess *orchestrator.Session, cmd string) int {
	out, err := sess.LaunchOrResume()
	if err != nil {
		fmt.Fprintln(os.Stderr, cmd+":", err)
		return 1
	}
	switch out.Result {
	case orchestrator.ResultAwaitingUAT, orchestrator.ResultAttached, orchestrator.ResultTerminal:
		return 0
	case orchestrator.ResultParked, orchestrator.ResultOther:
		return 2
	case orchestrator.ResultRefused:
		return 1
	}
	return 0
}

// cmdSweep is the `gogo sweep` entry (FR9): reap orphaned / terminal-feature
// persistent sessions.
func cmdSweep(args []string) int {
	dryRun := false
	for _, a := range args {
		switch {
		case a == "--dry-run" || a == "-n":
			dryRun = true
		case a == "-h" || a == "--help":
			fmt.Print(sweepHelp)
			return 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "gogo sweep: unknown flag %q\n", a)
			return 1
		default:
			fmt.Fprintf(os.Stderr, "gogo sweep: unexpected argument %q (sweep takes no slug)\n", a)
			return 1
		}
	}

	root, ok := findRoot()
	if !ok {
		return 1
	}
	repo, _ := contract.LoadRepo(root)
	sw := &orchestrator.Sweeper{Root: root, Repo: repo, Out: os.Stdout, DryRun: dryRun}
	killed := sw.Sweep()
	if dryRun && len(killed) > 0 {
		fmt.Printf("(dry-run) %d session(s) would be reaped — re-run without --dry-run to kill.\n", len(killed))
	}
	return 0
}

// cmdRun is the deprecated `gogo run` alias (FR11): it forwards to `gogo go` for
// one version so existing muscle-memory / scripts keep working.
func cmdRun(args []string) int {
	fmt.Fprintln(os.Stderr, "gogo run is deprecated — use `gogo go` (this alias forwards for now and will be removed in a future version).")
	return cmdGo(args)
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
	case "awaiting-plan-acceptance":
		return "accept its plan first (/gogo:accept), then re-run gogo go."
	case "shipped", "done", "aborted":
		return "it's already shipped."
	default:
		return "run /gogo:plan and accept a plan first."
	}
}
