package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/launch"
	"github.com/ZawadzkiB/gogo/cli/internal/plans"
	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

const planStoreHelp = `gogo plan - manage project-scoped plans (~/.gogo/projects/<project>/.gogo/plans/)

usage:
  gogo plan new "<title>" [--project <p>] [--desc <text>]   create a draft plan (prints its plan-<hash> id)
  gogo plan list [--project <p>] [--status <s>]             print the plans (newest-first)
  gogo plan show <id> [--project <p>]                       print a plan + its targets/members
  gogo plan add <id> <source>[:<slug>] [--project <p>]      add a target source (or link an existing work item)
  gogo plan rm  <id> <source>[:<slug>] [--project <p>]      remove a target (or unlink a work item)
  gogo plan ready <id> [--project <p>]                      ACCEPT: auto-spawn a work item into each target (targetless: just mark ready)
  gogo plan promote <id> <source> [--project <p>]           SPAWN one work item: launch /gogo:plan --correlation plan-<hash> in the source
  gogo plan done <id> [--project <p>]                       accept the project-UAT (refuses unless every member work item is shipped)
  gogo plan delete <id> [--project <p>]                     delete a plan

A PLAN is one lifecycle entity (draft → ready → active → done) owned by a home
project; it targets the project's sources and spawns a work item per source, each
stamped with the plan's correlation id in its state.md. --project defaults to the
sole project and is REQUIRED when several exist. This writes ONLY ~/.gogo/ - a spawn
LAUNCHES /gogo:plan in the source (the skill writes its .gogo/work/), never the CLI.

Note: "gogo plan <slug>" (a bare feature slug, not a subcommand) still launches the
feature's persistent /gogo:plan session - the lifecycle command, unchanged.
`

// planLauncher is the injectable spawn seam (mirroring draftLauncher): tests swap it
// for a fake to assert a promote fires the launch once with the right intent, no real
// tmux/claude.
var planLauncher func(root string, in launch.Intent) (launch.Result, error) = launch.Launch

// planStoreVerbs is the set of `gogo plan` subcommands that address the PROJECT-scoped
// plan store (vs a bare `gogo plan <slug>` which launches a persistent session).
func isPlanStoreVerb(v string) bool {
	switch v {
	case "new", "list", "ls", "show", "add", "rm", "remove", "delete", "del", "ready", "promote", "done":
		return true
	case "-h", "--help", "help":
		return true
	}
	return false
}

// cmdPlanStore dispatches the project-scoped `gogo plan` subcommands (FR17). It writes
// ONLY the plans store under ~/.gogo/ - never a source's .gogo/ (a promote LAUNCHES
// the skill in the source, which writes the work item + stamps the correlation).
func cmdPlanStore(args []string) int {
	if len(args) == 0 {
		fmt.Print(planStoreHelp)
		return 0
	}
	switch args[0] {
	case "new":
		return planNew(args[1:])
	case "list", "ls":
		return planList(args[1:])
	case "show":
		return planShow(args[1:])
	case "add":
		return planAdd(args[1:])
	case "rm", "remove":
		return planRemove(args[1:])
	case "delete", "del":
		return planDelete(args[1:])
	case "ready":
		return planReady(args[1:])
	case "promote":
		return planPromote(args[1:])
	case "done":
		return planDone(args[1:])
	case "-h", "--help", "help":
		fmt.Print(planStoreHelp)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "gogo plan: unknown subcommand %q (new | list | show | add | rm | ready | promote | done | delete)\n", args[0])
		return 2
	}
}

// planArgs holds the parsed positionals + flags for the plan subcommands.
type planArgs struct {
	pos     []string
	project string
	desc    string
	status  string
}

// parsePlanArgs pulls --project/--desc/--status flags (both --flag value and
// --flag=value shapes) and the leftover positionals out of an argv.
func parsePlanArgs(cmd string, args []string) (planArgs, bool, int) {
	var p planArgs
	for i := 0; i < len(args); i++ {
		a := args[i]
		takeVal := func(flag string) (string, bool) {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: %s needs a value\n", cmd, flag)
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case a == "--project":
			v, ok := takeVal("--project")
			if !ok {
				return p, false, 2
			}
			p.project = v
		case strings.HasPrefix(a, "--project="):
			p.project = strings.TrimPrefix(a, "--project=")
		case a == "--desc":
			v, ok := takeVal("--desc")
			if !ok {
				return p, false, 2
			}
			p.desc = v
		case strings.HasPrefix(a, "--desc="):
			p.desc = strings.TrimPrefix(a, "--desc=")
		case a == "--status":
			v, ok := takeVal("--status")
			if !ok {
				return p, false, 2
			}
			p.status = v
		case strings.HasPrefix(a, "--status="):
			p.status = strings.TrimPrefix(a, "--status=")
		case a == "-h" || a == "--help":
			fmt.Print(planStoreHelp)
			return p, false, 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "%s: unknown flag %q\n", cmd, a)
			return p, false, 2
		default:
			p.pos = append(p.pos, a)
		}
	}
	return p, true, 0
}

// planNew creates a draft plan in the resolved project.
func planNew(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan new", args)
	if !ok {
		return code
	}
	title := strings.Join(pa.pos, " ")
	if strings.TrimSpace(title) == "" {
		fmt.Fprintln(os.Stderr, `gogo plan new: needs a "<title>" (e.g. gogo plan new "cross-repo token migration")`)
		return 2
	}
	project, code := resolveProjectName("gogo plan new", pa.project)
	if code != 0 {
		return code
	}
	p, err := plans.New(project, title, pa.desc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo plan new: %v\n", err)
		return 1
	}
	fmt.Printf("created plan %s (%s) in %q - %s\n", p.ID, p.Status, project, p.Title)
	return 0
}

// planList prints the plans of the resolved project (optionally filtered by status).
func planList(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan list", args)
	if !ok {
		return code
	}
	project, code := resolveProjectName("gogo plan list", pa.project)
	if code != 0 {
		return code
	}
	list, err := plans.List(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo plan list: %v\n", err)
		return 1
	}
	if pa.status != "" {
		filtered := list[:0]
		for _, p := range list {
			if p.Status == pa.status {
				filtered = append(filtered, p)
			}
		}
		list = filtered
	}
	fmt.Print(FormatPlans(project, list))
	return 0
}

// planShow prints one plan + its targets/members.
func planShow(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan show", args)
	if !ok {
		return code
	}
	if len(pa.pos) == 0 {
		fmt.Fprintln(os.Stderr, "gogo plan show: needs an <id> (see `gogo plan list`)")
		return 2
	}
	project, code := resolveProjectName("gogo plan show", pa.project)
	if code != 0 {
		return code
	}
	p, found := plans.Get(project, pa.pos[0])
	if !found {
		fmt.Fprintf(os.Stderr, "gogo plan show: no plan %q in %q (see `gogo plan list`)\n", pa.pos[0], project)
		return 1
	}
	fmt.Printf("%s  %s  [%s]\n", p.ID, p.Title, p.Status)
	if p.Description != "" {
		fmt.Printf("  %s\n", p.Description)
	}
	if len(p.Targets) > 0 {
		fmt.Printf("  targets: %s\n", strings.Join(p.Targets, ", "))
	}
	if len(p.Members) == 0 {
		fmt.Println("  (no work items - spawn one with `gogo plan promote " + p.ID + " <source>`)")
		return 0
	}
	fmt.Printf("  %d work item%s:\n", len(p.Members), planPlural(len(p.Members)))
	for _, m := range p.Members {
		fmt.Printf("    %s : %s\n", m.Source, m.SlugHint)
	}
	return 0
}

// planAdd adds a target source (or, with a `:slug`, links an existing work item -
// the retroactive many-to-many connect, FR16).
func planAdd(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan add", args)
	if !ok {
		return code
	}
	if len(pa.pos) < 2 {
		fmt.Fprintln(os.Stderr, "gogo plan add: needs <id> <source>[:<slug>]")
		return 2
	}
	project, code := resolveProjectName("gogo plan add", pa.project)
	if code != 0 {
		return code
	}
	id := pa.pos[0]
	source, slug := splitSourceSlug(pa.pos[1])
	if slug == "" {
		if _, err := plans.AddTarget(project, id, source); err != nil {
			fmt.Fprintf(os.Stderr, "gogo plan add: no plan %q in %q (see `gogo plan list`)\n", id, project)
			return 1
		}
		fmt.Printf("added target %s to plan %s\n", source, id)
		return 0
	}
	if _, err := plans.AddMember(project, id, plans.Member{Source: source, SlugHint: slug}); err != nil {
		fmt.Fprintf(os.Stderr, "gogo plan add: no plan %q in %q (see `gogo plan list`)\n", id, project)
		return 1
	}
	fmt.Printf("linked %s:%s to plan %s (re-stamp its state.md correlation via `gogo plan promote` or the skill)\n", source, slug, id)
	return 0
}

// planRemove removes a target (or unlinks a work item).
func planRemove(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan rm", args)
	if !ok {
		return code
	}
	if len(pa.pos) < 2 {
		fmt.Fprintln(os.Stderr, "gogo plan rm: needs <id> <source>[:<slug>]")
		return 2
	}
	project, code := resolveProjectName("gogo plan rm", pa.project)
	if code != 0 {
		return code
	}
	id := pa.pos[0]
	source, slug := splitSourceSlug(pa.pos[1])
	var removed bool
	var err error
	if slug == "" {
		removed, err = plans.RemoveTarget(project, id, source)
	} else {
		removed, err = plans.RemoveMember(project, id, plans.Member{Source: source, SlugHint: slug})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo plan rm: %v\n", err)
		return 1
	}
	if !removed {
		fmt.Fprintf(os.Stderr, "gogo plan rm: %q is not on plan %s (see `gogo plan show %s`)\n", pa.pos[1], id, id)
		return 1
	}
	fmt.Printf("removed %s from plan %s\n", pa.pos[1], id)
	return 0
}

// planDelete deletes a plan by id.
func planDelete(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan delete", args)
	if !ok {
		return code
	}
	if len(pa.pos) == 0 {
		fmt.Fprintln(os.Stderr, "gogo plan delete: needs an <id> (see `gogo plan list`)")
		return 2
	}
	project, code := resolveProjectName("gogo plan delete", pa.project)
	if code != 0 {
		return code
	}
	removed, err := plans.Delete(project, pa.pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo plan delete: %v\n", err)
		return 1
	}
	if !removed {
		fmt.Fprintf(os.Stderr, "gogo plan delete: no plan %q in %q (see `gogo plan list`)\n", pa.pos[0], project)
		return 1
	}
	fmt.Printf("deleted plan %s\n", pa.pos[0])
	return 0
}

// planReady is the headless ACCEPT step (0.25.0 FR2, the CLI mirror of the plans-tab
// `r`). A TARGETLESS plan just advances draft → ready (today's behaviour, byte-for-byte).
// A plan WITH targets fans out: it AUTO-SPAWNS a work item into each UN-spawned target
// source — one `/gogo:plan <brief> --correlation plan-XXXX` per target (its per-source
// brief as the goal, its `--skip-acceptance` when the source opted out), records a member
// + flips the plan active on each SUCCESSFUL launch, and is idempotent (a target already
// carrying a member is skipped, so a re-run never re-launches). The CLI writes nothing
// under a source's .gogo/ — each launched skill does. `gogo plan promote` (single source)
// stays the manual fallback.
func planReady(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan ready", args)
	if !ok {
		return code
	}
	if len(pa.pos) == 0 {
		fmt.Fprintln(os.Stderr, "gogo plan ready: needs an <id> (see `gogo plan list`)")
		return 2
	}
	project, code := resolveProjectName("gogo plan ready", pa.project)
	if code != 0 {
		return code
	}
	id := pa.pos[0]
	p, found := plans.Get(project, id)
	if !found {
		fmt.Fprintf(os.Stderr, "gogo plan ready: no plan %q in %q (see `gogo plan list`)\n", id, project)
		return 1
	}
	// Targetless plan → today's plain draft → ready (no spawn).
	if len(p.Targets) == 0 {
		updated, err := plans.MarkReady(project, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gogo plan ready: %v\n", err)
			return 1
		}
		fmt.Printf("marked plan %s ready\n", updated.ID)
		return 0
	}
	// Fan out to the un-spawned targets (mirrors the plans-tab `r` auto-spawn). Load the
	// project ONCE, surfacing the error (REV-003) instead of swallowing it; a project with
	// no sources cannot resolve any target, so say so up front rather than N stderr lines.
	proj, err := projects.Load(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo plan ready: cannot load project %q: %v\n", project, err)
		return 1
	}
	if len(proj.Sources) == 0 {
		fmt.Fprintf(os.Stderr, "gogo plan ready: project %q has no sources - add one (`gogo source add`) before accepting a targeted plan\n", project)
		return 1
	}
	body := p.Description
	if strings.TrimSpace(body) == "" {
		body = p.Title
	}
	spawned, alreadySpawned := 0, 0
	var invalid, failed []string
	for _, target := range p.Targets {
		if planHasMember(p, target) {
			alreadySpawned++
			continue // already spawned (recorded member) — idempotent
		}
		src, sname, ok := sourceInProject(project, target)
		if !ok {
			invalid = append(invalid, target)
			fmt.Fprintf(os.Stderr, "gogo plan ready: target %q is not a source of %q — skipping\n", target, project)
			continue
		}
		// REV-002: also skip a target spawned OUT OF BAND — a work item already in the
		// source carrying this plan's correlation id but never recorded as a member (the
		// same member-OR-feature guard the plans-tab `r` applies, so the two mirrors agree).
		// A pure READ of the source's contract, never a source write.
		if planFeatureSpawned(src.Path, p.ID) {
			alreadySpawned++
			continue
		}
		goal := plans.BriefFor(p, target)
		if strings.TrimSpace(goal) == "" {
			goal = body
		}
		intent := launch.PlanIntent(p.Title, goal, p.ID)
		intent.Command += launch.SkipParams(src.PlanAcceptanceSkip, false)
		res, err := planLauncher(src.Path, intent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gogo plan ready: spawn into %s failed: %v\n", sname, err)
			failed = append(failed, sname)
			continue // leave the target un-recorded (no phantom member)
		}
		// Record the spawn (advisory member + ready→active) — store writes ~/.gogo/ only.
		plans.AddMember(project, id, plans.Member{Source: sname, SlugHint: planKebab(p.Title)})
		plans.SetStatus(project, id, plans.StatusActive)
		where := res.Session
		if where == "" {
			where = res.LogPath
		}
		fmt.Printf("spawned work item for plan %s in %s - launched %s (%s)\n", id, sname, intent.Command, where)
		spawned++
	}
	// Summarize anything that did NOT spawn cleanly — a targets: entry that is not a source
	// (REV-003) and/or a launch attempt that FAILED (TEST-001). Both are real problems that
	// must never masquerade as the clean idempotent "all already spawned" success.
	var problems []string
	if len(invalid) > 0 {
		problems = append(problems, fmt.Sprintf("%d unresolved target(s): %s", len(invalid), strings.Join(invalid, ", ")))
	}
	if len(failed) > 0 {
		problems = append(problems, fmt.Sprintf("%d failed to launch: %s", len(failed), strings.Join(failed, ", ")))
	}
	if spawned == 0 {
		// Only a pure idempotent no-op (everything already spawned, nothing failed or
		// unresolved) is a success. A launch failure or unresolved target here means ZERO
		// work items exist — report it on stderr with a non-zero exit so a scripted/CI
		// caller checking $? is never told "nothing to do" (REV-003 + TEST-001).
		if len(problems) > 0 {
			fmt.Fprintf(os.Stderr, "gogo plan ready: plan %s: nothing to spawn - %d already spawned, %s\n",
				id, alreadySpawned, strings.Join(problems, "; "))
			return 1
		}
		fmt.Printf("plan %s: all %d target(s) already spawned - nothing to do\n", id, len(p.Targets))
		return 0
	}
	if len(problems) > 0 {
		// Some spawned, but a target did not resolve or a launch failed — signal the partial
		// failure (non-zero) and name the offending targets so the problem is visible.
		fmt.Fprintf(os.Stderr, "gogo plan ready: accepted plan %s - spawned %d work item(s), but %s\n",
			id, spawned, strings.Join(problems, "; "))
		return 1
	}
	fmt.Printf("accepted plan %s - spawned %d work item(s)\n", id, spawned)
	return 0
}

// planFeatureSpawned reports whether the source at root already carries a work item
// stamped with the plan's correlation id — the out-of-band / retroactive-link signal
// the plans-tab `r` guards on beyond a recorded member (REV-002). A pure READ of the
// source's .gogo/ contract; a missing/unreadable source degrades to false (never a
// crash, never a source write).
func planFeatureSpawned(root, planID string) bool {
	repo, err := contract.LoadRepo(root)
	if err != nil || repo == nil {
		return false
	}
	for _, f := range repo.Features {
		for _, id := range f.Correlations {
			if id == planID {
				return true
			}
		}
	}
	return false
}

// planHasMember reports whether the plan already records a member for source — the
// store-side idempotency signal the headless fan-out (planReady) skips on.
func planHasMember(p plans.Plan, source string) bool {
	for _, m := range p.Members {
		if m.Source == source {
			return true
		}
	}
	return false
}

// planPromote SPAWNS a work item for a plan into a source (FR11/FR15/D3): it launches
// /gogo:plan <body> --correlation plan-<hash> in the source root (the skill writes the
// work item + stamps the correlation), records an advisory member, and advances the
// plan to active. The CLI never writes the source's .gogo/. Fires the launch seam once.
func planPromote(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan promote", args)
	if !ok {
		return code
	}
	if len(pa.pos) < 2 {
		fmt.Fprintln(os.Stderr, "gogo plan promote: needs <id> <source> (the source to spawn a work item into)")
		return 2
	}
	project, code := resolveProjectName("gogo plan promote", pa.project)
	if code != 0 {
		return code
	}
	id := pa.pos[0]
	p, found := plans.Get(project, id)
	if !found {
		fmt.Fprintf(os.Stderr, "gogo plan promote: no plan %q in %q (see `gogo plan list`)\n", id, project)
		return 1
	}
	src, sname, found := sourceInProject(project, pa.pos[1])
	if !found {
		fmt.Fprintf(os.Stderr, "gogo plan promote: %q is not a source of %q (see `gogo project list`)\n", pa.pos[1], project)
		return 1
	}
	body := p.Description
	if strings.TrimSpace(body) == "" {
		body = p.Title
	}
	intent := launch.PlanIntent(p.Title, body, p.ID)
	res, err := planLauncher(src.Path, intent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo plan promote: %v\n", err)
		return 1
	}
	// Record the spawn (advisory member + ready→active) - store writes to ~/.gogo/ only.
	plans.AddMember(project, id, plans.Member{Source: sname, SlugHint: planKebab(p.Title)})
	plans.SetStatus(project, id, plans.StatusActive)
	where := res.Session
	if where == "" {
		where = res.LogPath
	}
	fmt.Printf("spawned work item for plan %s in %s - launched %s (%s)\n", id, sname, intent.Command, where)
	return 0
}

// planDone is the project-UAT acceptance (FR3, accept-only v1): it REFUSES (non-zero,
// naming the unshipped members) unless EVERY member work item of the plan is shipped,
// then records the accept via plans.MarkDone (appends a `## Project UAT` round to the
// plan body + flips the plan to `done`). The member-shipped check READS each member's
// source state.md (never writes a source's .gogo/). This is an ADDITIONAL gate on top
// of each member's own /gogo:done UAT, not a replacement (FR3×FR4 orthogonality).
func planDone(args []string) int {
	pa, ok, code := parsePlanArgs("gogo plan done", args)
	if !ok {
		return code
	}
	if len(pa.pos) == 0 {
		fmt.Fprintln(os.Stderr, "gogo plan done: needs an <id> (see `gogo plan list`)")
		return 2
	}
	project, code := resolveProjectName("gogo plan done", pa.project)
	if code != 0 {
		return code
	}
	id := pa.pos[0]
	p, found := plans.Get(project, id)
	if !found {
		fmt.Fprintf(os.Stderr, "gogo plan done: no plan %q in %q (see `gogo plan list`)\n", id, project)
		return 1
	}
	if p.Status == plans.StatusDone {
		fmt.Printf("plan %s is already done (project-UAT accepted)\n", id)
		return 0
	}
	if len(p.Members) == 0 {
		fmt.Fprintf(os.Stderr, "gogo plan done: refusing - plan %s has no work items yet; spawn + ship members first (`gogo plan promote %s <source>`)\n", id, id)
		return 1
	}
	allShipped, unshipped := plans.MembersShipped(project, p)
	if !allShipped {
		fmt.Fprintf(os.Stderr, "gogo plan done: refusing - %d of %d member(s) not shipped yet: %s\n",
			len(unshipped), len(p.Members), strings.Join(unshipped, ", "))
		fmt.Fprintf(os.Stderr, "  ship each member (its own /gogo:done UAT), then re-run `gogo plan done %s`.\n", id)
		return 1
	}
	updated, err := plans.MarkDone(project, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo plan done: %v\n", err)
		return 1
	}
	fmt.Printf("accepted project-UAT for plan %s - all %d member(s) shipped; plan is now %s\n",
		id, len(updated.Members), updated.Status)
	return 0
}

// sourceInProject resolves a source key (name or path) within a project, returning
// the source, its display label, and whether it matched.
func sourceInProject(project, key string) (projects.Source, string, bool) {
	p, _ := projects.Load(project)
	abs := key
	if a, err := filepath.Abs(key); err == nil {
		abs = filepath.Clean(a)
	}
	for _, s := range p.Sources {
		label := s.Name
		if label == "" {
			label = filepath.Base(s.Path)
		}
		if s.Name == key || s.Path == key || s.Path == abs || label == key {
			return s, label, true
		}
	}
	return projects.Source{}, "", false
}

// splitSourceSlug splits a "<source>:<slug>" spec (slugs are [a-z0-9-] and never
// carry a colon, so the tail after the LAST ':' is the slug). No ':' → the whole
// token is the source (an add/rm of a target, not a work-item link).
func splitSourceSlug(spec string) (source, slug string) {
	i := strings.LastIndex(spec, ":")
	if i < 0 {
		return strings.TrimSpace(spec), ""
	}
	return strings.TrimSpace(spec[:i]), strings.TrimSpace(spec[i+1:])
}

var planKebabUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

// planKebab derives the advisory kebab feature slug a spawn pins as the member hint.
func planKebab(title string) string {
	s := planKebabUnsafe.ReplaceAllString(strings.ToLower(title), "-")
	if s = strings.Trim(s, "-"); s == "" {
		s = "plan"
	}
	return s
}

// FormatPlans renders a project's plans as a deterministic plain-text table
// (id · status · items · title). Exposed so a test can pin it.
func FormatPlans(project string, list []plans.Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "gogo plans - %d in %q  (~/.gogo/projects/%s/.gogo/plans/)\n\n", len(list), project, project)
	if len(list) == 0 {
		b.WriteString("(none - create one with `gogo plan new \"<title>\"`)\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%-18s %-8s %-7s %s\n", "ID", "STATUS", "ITEMS", "TITLE")
	b.WriteString(strings.Repeat("─", 72) + "\n")
	for _, p := range list {
		title := p.Title
		if title == "" {
			title = "(untitled)"
		}
		fmt.Fprintf(&b, "%-18s %-8s %-7d %s\n", p.ID, p.Status, len(p.Members), title)
	}
	return b.String()
}

// planPlural is the "s" suffix for a count.
func planPlural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
