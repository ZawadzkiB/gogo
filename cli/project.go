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

const projectHelp = `gogo project — manage home-folder projects (~/.gogo/projects/<name>/)

usage:
  gogo project add <repo> [--name <name>]   create a project (name defaults to the repo basename)
                                            with <repo> as its first source (must contain .gogo/)
  gogo project list                         print the projects and their sources
  gogo project rm <name>                    remove a project (its home folder only, never a source's .gogo/)

A PROJECT is a home-folder entity that links many SOURCES (repos with their own
.gogo/). Add more sources with ` + "`gogo source add`" + `. This writes ONLY
~/.gogo/ (the CLI's own data) — never a source's .gogo/ pipeline state.
`

// cmdProject dispatches the `gogo project` subcommands (FR4). It writes ONLY the
// gogo DATA home (~/.gogo/projects/<name>/) — never a source's .gogo/ pipeline
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

// projectAdd creates a home-folder project with <repo> as source #1 (D5): it
// cleans the repo path to absolute and verifies it contains a .gogo/ dir,
// defaulting the project name to the repo basename (or --name), the source's
// concurrentWorkItems to the default (1), and mainBranch to the detected git
// default (else "main"). Adding a source that is ALREADY registered under the
// project is an idempotent no-op; if a DIFFERENT project already owns the name it
// directs the user to `gogo source add` rather than clobbering.
func projectAdd(args []string) int {
	repo, name, err := parseRepoName("gogo project add", args)
	if err != nil {
		return 2
	}
	if repo == "" {
		fmt.Fprintln(os.Stderr, "gogo project add: needs a <repo> (the repo root that contains .gogo/)")
		return 2
	}
	abs, code := resolveGogoRepo("gogo project add", repo)
	if code != 0 {
		return code
	}
	// FR22: registering a project also initializes the global cockpit home (writes
	// the ~/.gogo/config.json marker) so the cockpit becomes available even without
	// an explicit `gogo global init`. Best-effort — a marker write failure never
	// blocks the add (Save creates the project dir regardless), and it is idempotent
	// once the home exists.
	projects.EnsureHome()
	if name == "" {
		name = filepath.Base(abs)
	}
	// Auto-assign a default origin color for the project AND its first source
	// (cockpit-colors FR2): a deterministic round-robin over the palette that skips
	// colors already used by existing projects/sources, persisted into config.json.
	takenProj, takenSrc := takenColors()
	src := projects.Source{
		Path:                abs,
		Name:                filepath.Base(abs),
		MainBranch:          detectMainBranch(abs),
		ConcurrentWorkItems: projects.DefaultConcurrentWorkItems,
		Color:               projects.AssignColor(takenSrc),
	}

	existing, _ := projects.Load(name)
	if len(existing.Sources) > 0 {
		if hasSourcePath(existing.Sources, abs) {
			fmt.Printf("already registered %s in project %q — edit sources with `gogo source`\n", abs, name)
			return 0
		}
		fmt.Fprintf(os.Stderr, "gogo project add: project %q already exists with %d source(s) — add this repo with `gogo source add %s --project %s`\n",
			name, len(existing.Sources), abs, name)
		return 1
	}

	p := projects.Project{Name: name, Color: projects.AssignColor(takenProj), Sources: []projects.Source{src}}
	if _, err := projects.Add(p); err != nil {
		fmt.Fprintf(os.Stderr, "gogo project add: %v\n", err)
		return 1
	}
	fmt.Printf("created project %q → source %s (branch %s, cap %s)\n",
		name, abs, src.MainBranch, capLabel(src.ConcurrentWorkItems))
	return 0
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

// projectRemove removes a project by NAME (its home folder only — never a
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

// takenColors gathers the non-blank project + source origin colors already in use
// across the whole store — the `taken` sets projects.AssignColor round-robins around so
// a freshly-added project/source fans out to the next free palette swatch
// (cockpit-colors FR2). A missing/empty store yields empty slices (AssignColor then
// picks the first swatch).
func takenColors() (projColors, srcColors []string) {
	all, _ := projects.List()
	for _, p := range all {
		if p.Color != "" {
			projColors = append(projColors, p.Color)
		}
		for _, s := range p.Sources {
			if s.Color != "" {
				srcColors = append(srcColors, s.Color)
			}
		}
	}
	return projColors, srcColors
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

// parseRepoName pulls a positional <repo> and an optional --name/--project flag
// out of argv (shared by project add / source add). flagName selects which flag
// this command accepts ("--name" for project, "--project" for source). An unknown
// flag or a flag with no value returns an error (helped signalled to the caller
// via a non-nil err + a printed message).
func parseRepoName(cmd string, args []string) (repo, name string, err error) {
	return parseRepoFlag(cmd, "--name", args)
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
		fmt.Fprintf(os.Stderr, "%s: %s has no .gogo/ — not a gogo source (run /gogo:build there first)\n", cmd, abs)
		return "", 1
	}
	return abs, 0
}

// detectMainBranch best-effort resolves a repo's default branch: the
// origin/HEAD symbolic ref, else the current branch, else "main". Never fatal —
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
	fmt.Fprintf(&b, "gogo projects — %d project(s)  (~/.gogo/projects/)\n\n", len(projs))
	if len(projs) == 0 {
		b.WriteString("(none — create one with `gogo project add <repo>`)\n")
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
