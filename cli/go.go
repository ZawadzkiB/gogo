package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/orchestrator"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

// sessionLister lists live gogo-* tmux sessions for the concurrency-cap guard. A
// package seam (defaults to launch.ListSessions) so the over-cap refusal can be
// driven with a fake session set in tests - no real tmux (FR8). Only the go-cap
// guard reads it; every other launch path is unchanged.
var sessionLister = launch.ListSessions

// slugPattern is the canonical kebab-case feature slug (same shape the typed
// contracts use). It is the write-scope guard: an unvalidated slug flows into
// LockPath / RegistryPath and is filepath.Join'd under `.gogo/resources/`, so a
// slug carrying `..` or `/` could escape that root and breach the hard
// "only ever write under .gogo/" invariant. `gogo plan` intentionally accepts a
// brand-new slug (no feature-existence guard), so it is the live vector (REV-001).
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validSlug(slug string) bool { return slugPattern.MatchString(slug) }

const goHelp = `gogo go - launch or resume a feature's persistent pipeline session

usage:
  gogo go [<slug>] [--attach] [--takeover] [--force]

Launches (or --resumes) ONE persistent ` + "`claude -p`" + ` session running the existing
/gogo:go skill for the whole feature - implement warm in-context + review/test as
nested Task subagents + report. The CLI only manages that session's lifecycle: it
guards a one-owner lock, resolves fresh-vs-resume from the session registry,
classifies the child's exit, and reaps at ship. No phase loop, no routing in Go -
the ONE routing rule lives in the skill. Needs the claude CLI on PATH; --attach
needs tmux.

  <slug>       the feature to run (default: the newest plan-accepted / mid-pipeline one)
  --attach     launch an attachable tmux session (interactive claude) so you can
               answer decision/UAT gates live in the warm session (reaped at close)
  --takeover   seize the owner lock from a live session (the prior is reaped)
  --force      override the project's concurrency cap (start work even when the
               repo is already at its maxConcurrent live in-progress features)

env:
  GOGO_CLAUDE_PERMISSION_MODE   permission mode for the spawned session (default: auto)

exit: 0 = green (awaiting-uat) / attached · 2 = parked at a gate · 1 = error / refused
`

const planHelp = `gogo plan - launch or resume a feature's persistent planning session

usage:
  gogo plan <slug> [--attach] [--takeover]

Launch-or-resumes ONE persistent ` + "`claude -p`" + ` session running /gogo:plan <slug>
through the same lifecycle machinery as ` + "`gogo go`" + ` (its own tracked session - plan
and go are distinct legs of the same feature's work). Writes an accept-pending plan
and stops for your acceptance. Needs the claude CLI on PATH; --attach needs tmux.
`

const sweepHelp = `gogo sweep - reap orphaned / shipped persistent sessions

usage:
  gogo sweep [--dry-run] [<slug>...]

With no slug (whole-board), kills (1) gogo-* tmux sessions whose owning feature is
already terminal (shipped/aborted) - the kill-at-ship backstop - and (2) orphans:
a live gogo-* session with no live, non-terminal owning feature. Attribution is by
the exact gogo-<action>-<slug> convention (never substring).

With one or more <slug> args (TARGETED), restricts the reap to just those slugs'
sessions (and their lock/registry cleanup) - this is what /gogo:done runs at ship
so it reaps only the shipped card's own sessions and never a different feature's
concurrent ship. The session hosting this sweep is always spared (no self-kill).

--dry-run lists what it would kill without touching anything. Also self-heals stale
lockfiles (scoped to the named slugs in targeted mode).
`

// parseSessionFlags pulls the shared flags (--attach / --takeover / --force / slug)
// out of an argv for gogo go / gogo plan, printing help + signalling exit on -h.
// --force is the concurrency-cap escape hatch (D3): honored by `gogo go` (it
// overrides the cap), parsed-but-ignored by `gogo plan` (planning is uncapped).
// It is DISTINCT from --takeover (which seizes the per-feature owner lock).
func parseSessionFlags(cmd, help string, args []string) (slug string, attach, takeover, force, helped bool, code int) {
	for _, a := range args {
		switch {
		case a == "--attach":
			attach = true
		case a == "--takeover":
			takeover = true
		case a == "--force":
			force = true
		case a == "-h" || a == "--help":
			fmt.Print(help)
			return "", false, false, false, true, 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "%s: unknown flag %q\n", cmd, a)
			return "", false, false, false, true, 1
		case slug == "":
			slug = a
		}
	}
	return slug, attach, takeover, force, false, 0
}

// cmdGo is the `gogo go` entry (FR1/FR3): enforce the SAME acceptance gate /gogo:go
// uses, then launch-or-resume the one persistent session via the lifecycle manager.
func cmdGo(args []string) int {
	slug, attach, takeover, force, helped, code := parseSessionFlags("gogo go", goHelp, args)
	if helped {
		return code
	}
	if slug != "" && !validSlug(slug) {
		fmt.Fprintf(os.Stderr, "gogo go: invalid slug %q - expected kebab-case [a-z0-9-] (no path separators)\n", slug)
		return 1
	}

	root, ok := findRoot()
	if !ok {
		return 1
	}
	if !launch.HasClaude() {
		fmt.Fprintln(os.Stderr, "gogo go: claude CLI not on PATH - the persistent session runs `claude -p`")
		return 1
	}

	repo, _ := contract.LoadRepo(root)
	if slug == "" {
		f := newestRunnable(repo)
		if f == nil {
			fmt.Fprintln(os.Stderr, "gogo go: no runnable feature - need a plan-accepted or mid-pipeline one (run /gogo:plan and accept a plan first)")
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
		fmt.Printf("gogo go: %s is %s - nothing to run; reaped any tracked session.\n", slug, f.Status)
		return 0
	}
	if !orchestrator.RunnableStatus(f.Status) {
		fmt.Fprintf(os.Stderr, "gogo go: feature %q is %q - not runnable here. %s\n", slug, f.Status, runnableHint(f.Status))
		return 1
	}

	// Concurrency-cap guard (FR4/FR5, D3): refuse a go that would start work on an
	// (N+1)th live in-progress feature in this repo - two live build sessions
	// clobber the shared working tree. A resume of THIS feature is never blocked
	// (slug excluded from its own count); cap 0 = unlimited (fallback); --force
	// overrides. Read-side only - it composes with the one-owner lock.
	if msg := capBlock(root, repo, slug, force); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		return 1
	}

	sess := &orchestrator.Session{
		Root: root, Slug: slug, Kind: "go",
		Out: os.Stdout, Attach: attach, Takeover: takeover,
	}
	// FR4: resolve this SOURCE's gate-skip flags and thread them into the session so
	// the launched /gogo:go carries --skip-acceptance / --skip-uat (the skills honor
	// them). Explicit + visible: every fire prints a line so an auto-skip is never
	// silent. Default false / unregistered source → today's gated command byte-for-byte.
	planSkip, uatSkip, label := resolveSourceSkip(root)
	sess.SkipAcceptance, sess.SkipUAT = planSkip, uatSkip
	if planSkip {
		// Announce the source's opt-in, conditionally: a `gogo go` is only runnable at
		// plan-accepted or later (the plan gate is already behind on a resume/mid-pipeline
		// leg), so state that /gogo:go auto-accepts IF it reaches the plan gate — never the
		// over-claimed "auto-skipped" on a run that skips nothing this leg (REV-005).
		fmt.Printf("gogo go: source %s has planAcceptanceSkip — /gogo:go auto-accepts the plan if it hits the plan gate this run\n", label)
	}
	if uatSkip {
		fmt.Printf("gogo go: UAT auto-skipped for source %s (uatAcceptanceSkip)\n", label)
	}
	fmt.Printf("gogo go %s - launch-or-resume the persistent /gogo:go session (implement in-context + Task review/test + report)\n", slug)
	return runSession(sess, "gogo go")
}

// resolveSourceSkip resolves the per-source gate-skip flags (FR4) for the repo at
// root plus its display label (for the printed note), reading the projects store
// through the same flattened source set the cap guard uses (projects.AllSources /
// SkipForSource). An unregistered root → (false, false, "") — no skip, no note.
func resolveSourceSkip(root string) (planSkip, uatSkip bool, label string) {
	projs, _ := projects.List()
	sources := projects.AllSources(projs)
	planSkip, uatSkip = projects.SkipForSource(sources, root)
	for _, s := range sources {
		if s.Path == root {
			label = s.Name
			if label == "" {
				label = filepath.Base(s.Path)
			}
			break
		}
	}
	return planSkip, uatSkip, label
}

// capBlock returns a refusal message when a go-launch for slug in root would
// exceed the SOURCE's concurrency cap (and --force was not given), else "". Pure
// over its inputs (the projects store + the live session set are read through
// seams: projects.List and sessionLister) so the over-cap decision is
// unit-testable. It names the cap and the live feature(s) already building, plus
// the --force hint. The cap is now per-source (corrected model): root is a source
// repo, so its ConcurrentWorkItems is resolved from the flattened source set.
func capBlock(root string, repo *contract.Repo, slug string, force bool) string {
	if force {
		return ""
	}
	projs, _ := projects.List()
	cap := orchestrator.CapForSource(projects.AllSources(projs), root)
	if cap <= 0 {
		return "" // unlimited / unregistered → never blocks (byte-for-byte fallback)
	}
	active := orchestrator.ActiveWorkSlugs(repo, root, sessionLister(), slug)
	if !orchestrator.CapExceeded(cap, len(active)) {
		return ""
	}
	return fmt.Sprintf("gogo go: %s is capped at %d concurrent feature(s) - already building: %s.\n"+
		"  ship/finish one first, or re-run `gogo go %s --force` to override.",
		root, cap, strings.Join(active, ", "), slug)
}

// cmdPlan is the `gogo plan` entry. It serves TWO surfaces off the same verb (the
// corrected project→plans model layered over the persistent-session lifecycle):
//   - `gogo plan <store-verb> …` (new/list/show/add/rm/ready/promote/delete) →
//     the PROJECT-scoped plan store (cmdPlanStore, FR17);
//   - `gogo plan <slug>` (a bare feature slug) → launch-or-resume the feature's
//     persistent /gogo:plan session (the lifecycle command, unchanged).
//
// The store verbs are a small RESERVED set that SHADOW a bare slug (REV-004): a
// single-token slug that IS a store verb (e.g. a feature literally named `ready`,
// `promote`, or `show`) resolves to the store, NOT a session - such a feature must be
// launched another way (`gogo go`, or the board). Multi-word slugs never collide.
func cmdPlan(args []string) int {
	// `gogo plan -h`/`--help`/`help` shows BOTH surfaces (store verbs + the bare-slug
	// session launch, REV-007). Intercept it here before the store-verb dispatch, which
	// would otherwise print the store help alone.
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		printPlanHelp()
		return 0
	}
	if len(args) > 0 && isPlanStoreVerb(args[0]) {
		return cmdPlanStore(args)
	}
	// --force is parsed for flag-shape parity but ignored here (planning is uncapped, D6).
	slug, attach, takeover, _, helped, code := parseSessionFlags("gogo plan", planHelp, args)
	if helped {
		return code
	}
	if slug == "" {
		fmt.Fprintln(os.Stderr, "gogo plan: needs a <slug> - the feature to plan (e.g. `gogo plan my-feature`)")
		return 1
	}
	if !validSlug(slug) {
		fmt.Fprintf(os.Stderr, "gogo plan: invalid slug %q - expected kebab-case [a-z0-9-] (no path separators)\n", slug)
		return 1
	}

	root, ok := findRoot()
	if !ok {
		return 1
	}
	if !launch.HasClaude() {
		fmt.Fprintln(os.Stderr, "gogo plan: claude CLI not on PATH - the persistent session runs `claude -p`")
		return 1
	}

	repo, _ := contract.LoadRepo(root)
	status := ""
	if f := repo.Feature(slug); f != nil {
		status = f.Status
	}
	if !orchestrator.PlannableStatus(status) {
		fmt.Fprintf(os.Stderr, "gogo plan: feature %q is %q - already shipped; nothing to plan.\n", slug, status)
		return 1
	}

	sess := &orchestrator.Session{
		Root: root, Slug: slug, Kind: "plan",
		Out: os.Stdout, Attach: attach, Takeover: takeover,
	}
	fmt.Printf("gogo plan %s - launch-or-resume the persistent /gogo:plan session\n", slug)
	return runSession(sess, "gogo plan")
}

// printPlanHelp prints the COMBINED `gogo plan -h` help (REV-007): the project-scoped
// store verbs AND the bare-slug persistent-session launch usage, so neither surface is
// hidden behind the other. It also states the reserved-word caveat (REV-004).
func printPlanHelp() {
	fmt.Print(planStoreHelp)
	fmt.Print(`
gogo plan <slug> - (a bare feature SLUG, not a store subcommand) launch-or-resume the
feature's persistent /gogo:plan session:

  gogo plan <slug> [--attach] [--takeover]

Reserved words: the store verbs (new, list, show, add, rm, ready, promote, delete)
shadow a bare slug, so a feature literally named e.g. ` + "`ready`" + ` or ` + "`promote`" + ` resolves to
the store subcommand - launch such a feature another way (` + "`gogo go`" + `, or the board).
`)
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
	var only []string // targeted mode (D4=B): reap only these slugs' sessions
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
			if !validSlug(a) {
				fmt.Fprintf(os.Stderr, "gogo sweep: invalid slug %q\n", a)
				return 1
			}
			only = append(only, a)
		}
	}

	root, ok := findRoot()
	if !ok {
		return 1
	}
	repo, _ := contract.LoadRepo(root)
	// Self-guard (FR3): tell the sweeper which session it is itself running in so
	// it never reaps its own host - makes `gogo sweep` safe to invoke from any
	// context, including the /gogo:done ship-reap inside a gogo-done-<slug> session.
	// Only (D4=B): with slug args, a TARGETED sweep that touches only those slugs'
	// sessions (the ship-reap) - never another feature's concurrent ship (REV-002);
	// with no slug, the whole-board manual cleanup.
	sw := &orchestrator.Sweeper{Root: root, Repo: repo, Out: os.Stdout, DryRun: dryRun, Self: launch.CurrentSession(), Only: only}
	killed := sw.Sweep()
	if dryRun && len(killed) > 0 {
		fmt.Printf("(dry-run) %d session(s) would be reaped - re-run without --dry-run to kill.\n", len(killed))
	}
	return 0
}

// cmdRun is the deprecated `gogo run` alias (FR11): it forwards to `gogo go` for
// one version so existing muscle-memory / scripts keep working.
func cmdRun(args []string) int {
	fmt.Fprintln(os.Stderr, "gogo run is deprecated - use `gogo go` (this alias forwards for now and will be removed in a future version).")
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
		return "it's at the UAT gate - run /gogo:done to ship, or give feedback to loop it back."
	case "waiting-for-user":
		return "it's paused on a decision - resolve it and re-accept (→ plan-accepted) first."
	case "awaiting-plan-acceptance":
		return "accept its plan first (/gogo:accept), then re-run gogo go."
	case "shipped", "done", "aborted":
		return "it's already shipped."
	default:
		return "run /gogo:plan and accept a plan first."
	}
}
