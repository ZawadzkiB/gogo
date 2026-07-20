package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

const projectHelp = `gogo project - manage home-folder projects (~/.gogo/projects/<name>/)

usage:
  gogo project add <name> [--source <repo>]   create an EMPTY project ~/.gogo/projects/<name>/
                                              (config.json + .knowledge/ + .gogo/plans/); --source also
                                              links <repo> as source #1 in one shot
  gogo project add <repo> [--name <name>]     create a project with <repo> as source #1 (the repo must
                                              contain .gogo/; name defaults to the repo basename)
  gogo project list                           print the projects and their sources
  gogo project rm <name>                      remove a project (its home folder only, never a source's .gogo/)

NAME vs PATH: a bare token (no ` + "`/`" + ` or ` + "`\\`" + `, no leading ` + "`~`" + `/` + "`.`" + `, and not an
existing repo dir that contains .gogo/) is read as a project NAME -> an EMPTY project you grow
with ` + "`gogo source add`" + `. A path (a separator, a leading ` + "`~`" + `/` + "`.`" + `, or a dir that already
contains .gogo/) is read as a repo -> source #1. Ambiguous bare tokens bias to NAME.

A PROJECT is a home-folder entity that links many SOURCES (repos with their own
.gogo/). Add more sources with ` + "`gogo source add`" + `. This writes ONLY
~/.gogo/ (the CLI's own data) - never a source's .gogo/ pipeline state.
`

// cmdProject dispatches the `gogo project` subcommands (FR4). It writes ONLY the
// gogo DATA home (~/.gogo/projects/<name>/) - never a source's .gogo/ pipeline
// state (the CLI-reads/skills-write invariant is about a source's .gogo/, and a
// project entity is CLI-owned data, not pipeline state).
func cmdProject(args []string) int {
	if len(args) == 0 {
		fmt.Print(projectHelp)
		return 0
	}
	switch args[0] {
	case "add":
		return projectAdd(args[1:])
	case "list", "ls":
		return projectList()
	case "rm", "remove":
		return projectRemove(args[1:])
	case "-h", "--help", "help":
		fmt.Print(projectHelp)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "gogo project: unknown subcommand %q (add | list | rm)\n", args[0])
		return 2
	}
}

// projectAdd is the DUAL-MODE `gogo project add` (FR1, D=A). Its positional is read
// as either a bare NAME (create an EMPTY project you grow later) or a repo PATH
// (today's project+source-#1 flow, byte-for-byte):
//   - a PATH (a separator, a leading ~/., or a dir that already contains .gogo/) →
//     projectAddRepo (source #1, requires .gogo/).
//   - a bare NAME → projectAddEmpty (~/.gogo/projects/<name>/ + .knowledge/ + plans);
//     an ambiguous bare token biases to NAME ("create empty, add sources later").
//
// The optional --source <repo> links a repo as source #1 in one shot alongside a bare
// name (`gogo project add sanoma --source ~/repos/sanoma-web`); it is not combined with
// a PATH positional (the path is already the source).
func projectAdd(args []string) int {
	pos, name, source, err := parseProjectAddArgs(args)
	if err != nil {
		return 2
	}

	// PATH mode: the positional is a repo path → today's project+source-#1 flow.
	if pos != "" && isPathArg(pos) {
		if source != "" {
			fmt.Fprintln(os.Stderr, "gogo project add: --source is for the bare-NAME form; a repo PATH is already source #1")
			return 2
		}
		return projectAddRepo(pos, name)
	}

	// NAME mode. Resolve --source now (fail fast on a bad repo, before any create).
	srcAbs := ""
	if source != "" {
		var code int
		if srcAbs, code = resolveGogoRepo("gogo project add", source); code != 0 {
			return code
		}
	}
	projName := name // an explicit --name overrides the positional
	if projName == "" {
		projName = pos
	}
	if projName == "" && srcAbs != "" {
		projName = filepath.Base(srcAbs) // `--source <repo>` only → derive the name
	}
	if projName == "" {
		fmt.Fprintln(os.Stderr, "gogo project add: needs a <name> (e.g. `gogo project add sanoma`) or a <repo> path")
		return 2
	}
	return projectAddEmpty(projName, srcAbs)
}

// projectAddRepo is the PATH-mode flow (byte-for-byte today's `gogo project add
// <repo>`): create a home-folder project with <repo> as source #1 (D5) — cleans the
// repo path to absolute, verifies it contains a .gogo/ dir, defaults the project name
// to the repo basename (or --name), the source's concurrentWorkItems to the default
// (1), and mainBranch to the detected git default (else "main"). Re-adding a repo
// already registered under the project is an idempotent no-op; a DIFFERENT set of
// sources on that name directs the user to `gogo source add` rather than clobbering.
// FR2: the project's .knowledge/ + .gogo/plans/ scaffold is ensured too (idempotent).
func projectAddRepo(repo, name string) int {
	abs, code := resolveGogoRepo("gogo project add", repo)
	if code != 0 {
		return code
	}
	// FR22: registering a project also initializes the global cockpit home (writes
	// the ~/.gogo/config.json marker) so the cockpit becomes available even without
	// an explicit `gogo global init`. Best-effort - a marker write failure never
	// blocks the add (Save creates the project dir regardless), and it is idempotent
	// once the home exists.
	projects.EnsureHome()
	if name == "" {
		name = filepath.Base(abs)
	}
	// Auto-assign a default origin color for the project AND its first source
	// (cockpit-colors FR2): a deterministic round-robin over the palette that skips
	// colors already used by existing projects/sources, persisted into config.json.
	all, _ := projects.List()
	taken := projects.TakenColors(all)
	src := projects.Source{
		Path:                abs,
		Name:                filepath.Base(abs),
		MainBranch:          detectMainBranch(abs),
		ConcurrentWorkItems: projects.DefaultConcurrentWorkItems,
		Color:               projects.AssignColor(taken),
	}

	existing, _ := projects.Load(name)
	if len(existing.Sources) > 0 {
		if hasSourcePath(existing.Sources, abs) {
			fmt.Printf("already registered %s in project %q - edit sources with `gogo source`\n", abs, name)
			return 0
		}
		fmt.Fprintf(os.Stderr, "gogo project add: project %q already exists with %d source(s) - add this repo with `gogo source add %s --project %s`\n",
			name, len(existing.Sources), abs, name)
		return 1
	}

	p := projects.Project{Name: name, Color: projects.AssignColor(taken), Sources: []projects.Source{src}}
	if _, err := projects.Add(p); err != nil {
		fmt.Fprintf(os.Stderr, "gogo project add: %v\n", err)
		return 1
	}
	// FR2: scaffold .knowledge/ (seeded template) + .gogo/plans/ — ~/.gogo/ only,
	// idempotent, best-effort (never blocks the add).
	projects.EnsureProjectHome(name)
	fmt.Printf("created project %q → source %s (branch %s, cap %s)\n",
		name, abs, src.MainBranch, capLabel(src.ConcurrentWorkItems))
	return 0
}

// projectAddEmpty is the NAME-mode flow (FR1): create an EMPTY project
// ~/.gogo/projects/<name>/ (config.json with sources: [] + the .knowledge/ seed +
// .gogo/plans/) — no repo, no path required. With srcAbs non-empty (from --source) it
// also links that repo as source #1 in one shot. A name that COLLIDES with an existing
// project preserves it (never clobbers): --source adds the repo like `gogo source add`,
// no --source is a friendly idempotent note. Writes ONLY ~/.gogo/.
func projectAddEmpty(name, srcAbs string) int {
	if !projects.ValidName(name) {
		fmt.Fprintf(os.Stderr, "gogo project add: %q is not a valid project name (a single path component - no `/`, `\\`, `.`, or `..`)\n", name)
		return 2
	}
	// FR22 parity: an empty project add also initializes the cockpit home marker.
	projects.EnsureHome()

	if projects.Exists(name) {
		projects.EnsureProjectHome(name) // idempotent re-scaffold, never clobbers
		if srcAbs != "" {
			src := newColoredSource(name, srcAbs)
			added, err := projects.AddSource(name, src)
			if err != nil {
				fmt.Fprintf(os.Stderr, "gogo project add: %v\n", err)
				return 1
			}
			verb := "added"
			if !added {
				verb = "updated"
			}
			fmt.Printf("%s source %s in project %q (branch %s, cap %s)\n",
				verb, srcAbs, name, src.MainBranch, capLabel(src.ConcurrentWorkItems))
			return 0
		}
		existing, _ := projects.Load(name)
		fmt.Printf("project %q already exists (%d source%s) - add sources with `gogo source add <repo> --project %s`\n",
			name, len(existing.Sources), sourcePlural(len(existing.Sources)), name)
		return 0
	}

	all, _ := projects.List()
	taken := projects.TakenColors(all)
	p := projects.Project{Name: name, Color: projects.AssignColor(taken), Sources: []projects.Source{}}
	if srcAbs != "" {
		p.Sources = []projects.Source{{
			Path:                srcAbs,
			Name:                filepath.Base(srcAbs),
			MainBranch:          detectMainBranch(srcAbs),
			ConcurrentWorkItems: projects.DefaultConcurrentWorkItems,
			Color:               projects.AssignColor(taken),
		}}
	}
	if _, err := projects.Add(p); err != nil {
		fmt.Fprintf(os.Stderr, "gogo project add: %v\n", err)
		return 1
	}
	// FR2: scaffold .knowledge/ (seeded template) + .gogo/plans/ (idempotent, ~/.gogo/ only).
	projects.EnsureProjectHome(name)
	if srcAbs != "" {
		s := p.Sources[0]
		fmt.Printf("created project %q → source %s (branch %s, cap %s)\n",
			name, s.Path, s.MainBranch, capLabel(s.ConcurrentWorkItems))
	} else {
		fmt.Printf("created empty project %q (~/.gogo/projects/%s/) - add sources with `gogo source add <repo> --project %s`\n",
			name, name, name)
	}
	return 0
}

// isPathArg reports whether a `project add` positional is a repo PATH (vs a bare
// project NAME): a path separator, a leading ~ (home-relative) or . (dot-relative),
// or a bare token that resolves (relative to cwd) to a dir already containing .gogo/.
// Everything else is a NAME — the empty-project bias (D=A).
func isPathArg(arg string) bool {
	if arg == "" {
		return false
	}
	if strings.ContainsAny(arg, `/\`) {
		return true // has a path separator
	}
	if strings.HasPrefix(arg, "~") || strings.HasPrefix(arg, ".") {
		return true // ~home-relative or ./.. dot-relative
	}
	// A bare token that IS a real repo dir (has .gogo/) is a path, not a new name.
	if info, err := os.Stat(filepath.Join(arg, ".gogo")); err == nil && info.IsDir() {
		return true
	}
	return false
}

// parseProjectAddArgs pulls the positional plus the optional --name / --source flags
// out of `gogo project add` argv. --name sets the project name (PATH mode) / overrides
// it (NAME mode); --source names a repo to link as source #1. An unknown flag or a
// flag with no value returns an error (a message is printed).
func parseProjectAddArgs(args []string) (pos, name, source string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--name" || a == "--source":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "gogo project add: %s needs a value\n", a)
				return "", "", "", fmt.Errorf("missing value for %s", a)
			}
			if a == "--name" {
				name = args[i+1]
			} else {
				source = args[i+1]
			}
			i++
		case strings.HasPrefix(a, "--name="):
			name = strings.TrimPrefix(a, "--name=")
		case strings.HasPrefix(a, "--source="):
			source = strings.TrimPrefix(a, "--source=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "gogo project add: unknown flag %q\n", a)
			return "", "", "", fmt.Errorf("unknown flag %q", a)
		case pos == "":
			pos = a
		}
	}
	return pos, name, source, nil
}

// newColoredSource builds a Source for repo abs linked into project name, assigning
// the next free palette color (or preserving the source's existing color on a re-add,
// like `gogo source add`). Mirrors sourceAdd's color handling.
func newColoredSource(project, abs string) projects.Source {
	all, _ := projects.List()
	taken := projects.TakenColors(all)
	return projects.Source{
		Path:                abs,
		Name:                filepath.Base(abs),
		MainBranch:          detectMainBranch(abs),
		ConcurrentWorkItems: projects.DefaultConcurrentWorkItems,
		Color:               existingSourceColor(project, abs, projects.AssignColor(taken)),
	}
}

// projectList prints every project + its sources as a stable table.
func projectList() int {
	projs, err := projects.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo project list: %v\n", err)
		return 1
	}
	fmt.Print(FormatProjects(projs))
	return 0
}

// projectRemove removes a project by NAME (its home folder only - never a
// source's .gogo/).
func projectRemove(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "gogo project rm: needs a <name> (see `gogo project list`)")
		return 2
	}
	name := args[0]
	removed, err := projects.Remove(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo project rm: %v\n", err)
		return 1
	}
	if !removed {
		fmt.Fprintf(os.Stderr, "gogo project rm: no project named %q (see `gogo project list`)\n", name)
		return 1
	}
	fmt.Printf("removed project %q (its home folder; no source's .gogo/ was touched)\n", name)
	return 0
}

// capLabel renders a concurrency cap as "∞" for 0 (unlimited) else the number.
func capLabel(n int) string {
	if n <= 0 {
		return "∞"
	}
	return strconv.Itoa(n)
}

// hasSourcePath reports whether sources already carries a source at path.
func hasSourcePath(sources []projects.Source, path string) bool {
	for _, s := range sources {
		if s.Path == path {
			return true
		}
	}
	return false
}

// parseRepoFlag is the generic positional-<repo> + one-value-flag parser.
func parseRepoFlag(cmd, flag string, args []string) (repo, val string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == flag:
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: %s needs a value\n", cmd, flag)
				return "", "", fmt.Errorf("missing value for %s", flag)
			}
			val = args[i+1]
			i++
		case strings.HasPrefix(a, flag+"="):
			val = strings.TrimPrefix(a, flag+"=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "%s: unknown flag %q\n", cmd, a)
			return "", "", fmt.Errorf("unknown flag %q", a)
		case repo == "":
			repo = a
		}
	}
	return repo, val, nil
}

// resolveGogoRepo cleans a repo path to absolute and verifies it contains a
// .gogo/ dir (else it is not a gogo source). Returns the absolute path and a 0
// exit code on success, else "" and a non-zero code (with a printed message).
func resolveGogoRepo(cmd, repo string) (string, int) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: bad path %q: %v\n", cmd, repo, err)
		return "", 1
	}
	abs = filepath.Clean(abs)
	if info, err := os.Stat(filepath.Join(abs, ".gogo")); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "%s: %s has no .gogo/ - not a gogo source (run /gogo:build there first)\n", cmd, abs)
		return "", 1
	}
	return abs, 0
}

// detectMainBranch best-effort resolves a repo's default branch: the
// origin/HEAD symbolic ref, else the current branch, else "main". Never fatal -
// a repo without git (or without a remote) falls straight through to "main".
func detectMainBranch(repo string) string {
	for _, args := range [][]string{
		{"-C", repo, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"},
		{"-C", repo, "branch", "--show-current"},
	} {
		out, err := exec.Command("git", args...).Output()
		if err != nil {
			continue
		}
		if s := strings.TrimPrefix(strings.TrimSpace(string(out)), "origin/"); s != "" {
			return s
		}
	}
	return "main"
}

// FormatProjects renders the projects + their sources as a deterministic
// plain-text table. Exposed so a test can pin it.
func FormatProjects(projs []projects.Project) string {
	var b strings.Builder
	fmt.Fprintf(&b, "gogo projects - %d project(s)  (~/.gogo/projects/)\n\n", len(projs))
	if len(projs) == 0 {
		b.WriteString("(none - create one with `gogo project add <name>` (empty) or `gogo project add <repo>` (with source #1))\n")
		return b.String()
	}
	for _, p := range projs {
		fmt.Fprintf(&b, "%s  (%d source%s)\n", p.Name, len(p.Sources), sourcePlural(len(p.Sources)))
		if p.Description != "" {
			fmt.Fprintf(&b, "  %s\n", p.Description)
		}
		for _, s := range p.Sources {
			branch := s.MainBranch
			if branch == "" {
				branch = "-"
			}
			fmt.Fprintf(&b, "  · %-20s %-8s cap %-3s %s\n", s.Name, branch, capLabel(s.ConcurrentWorkItems), s.Path)
		}
	}
	return b.String()
}

func sourcePlural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
