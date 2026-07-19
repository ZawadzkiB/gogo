package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZawadzkiB/gogo/cli/internal/projects"
)

const sourceHelp = `gogo source - manage a project's sources (repos with their own .gogo/)

usage:
  gogo source add <repo> [--project <name>]      link a repo as a source of a project
  gogo source rm <repo|name> [--project <name>]  unlink a source (by path or name)

A SOURCE is a repo (or a monorepo service, pointed at its service dir) that carries
its own .gogo/. --project selects the target project; it defaults to the sole
project and is REQUIRED when more than one project exists. This writes ONLY the
project entity under ~/.gogo/ - never a source's .gogo/ pipeline state.
`

// cmdSource dispatches the `gogo source` subcommands (FR5). Like `gogo project` it
// writes ONLY the project entity under ~/.gogo/ - never a source's .gogo/.
func cmdSource(args []string) int {
	if len(args) == 0 {
		fmt.Print(sourceHelp)
		return 0
	}
	switch args[0] {
	case "add":
		return sourceAdd(args[1:])
	case "rm", "remove":
		return sourceRemove(args[1:])
	case "-h", "--help", "help":
		fmt.Print(sourceHelp)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "gogo source: unknown subcommand %q (add | rm)\n", args[0])
		return 2
	}
}

// sourceAdd appends a source (a repo with .gogo/) to a project (D5): the target
// project defaults to the sole project and errors ambiguously when several exist
// and no --project is given. A re-add of the same path updates it in place.
func sourceAdd(args []string) int {
	repo, projName, err := parseRepoFlag("gogo source add", "--project", args)
	if err != nil {
		return 2
	}
	if repo == "" {
		fmt.Fprintln(os.Stderr, "gogo source add: needs a <repo> (a repo root that contains .gogo/)")
		return 2
	}
	abs, code := resolveGogoRepo("gogo source add", repo)
	if code != 0 {
		return code
	}
	name, code := resolveProjectName("gogo source add", projName)
	if code != 0 {
		return code
	}
	// Auto-assign a default origin color (cockpit-colors FR2) - the next free palette
	// swatch, skipping colors already taken. A re-add of a source that already carries a
	// color keeps it (never churns a user's customized color).
	all, _ := projects.List()
	taken := projects.TakenColors(all)
	src := projects.Source{
		Path:                abs,
		Name:                filepath.Base(abs),
		MainBranch:          detectMainBranch(abs),
		ConcurrentWorkItems: projects.DefaultConcurrentWorkItems,
		Color:               existingSourceColor(name, abs, projects.AssignColor(taken)),
	}
	added, err := projects.AddSource(name, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo source add: %v\n", err)
		return 1
	}
	verb := "added"
	if !added {
		verb = "updated"
	}
	fmt.Printf("%s source %s in project %q (branch %s, cap %s)\n",
		verb, abs, name, src.MainBranch, capLabel(src.ConcurrentWorkItems))
	return 0
}

// sourceRemove unlinks a source (by path or name) from a project.
func sourceRemove(args []string) int {
	key, projName, err := parseRepoFlag("gogo source rm", "--project", args)
	if err != nil {
		return 2
	}
	if key == "" {
		fmt.Fprintln(os.Stderr, "gogo source rm: needs a <repo|name> (see `gogo project list`)")
		return 2
	}
	name, code := resolveProjectName("gogo source rm", projName)
	if code != 0 {
		return code
	}
	// A path given relatively is retried against its cleaned absolute form so
	// `source rm .` matches a stored absolute path.
	removed, err := projects.RemoveSource(name, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo source rm: %v\n", err)
		return 1
	}
	if !removed {
		if abs, aerr := filepath.Abs(key); aerr == nil {
			if abs = filepath.Clean(abs); abs != key {
				removed, err = projects.RemoveSource(name, abs)
				if err != nil {
					fmt.Fprintf(os.Stderr, "gogo source rm: %v\n", err)
					return 1
				}
			}
		}
	}
	if !removed {
		fmt.Fprintf(os.Stderr, "gogo source rm: no source matching %q in project %q\n", key, name)
		return 1
	}
	fmt.Printf("removed source %s from project %q\n", key, name)
	return 0
}

// existingSourceColor returns the color a source at path already carries in the named
// project (so a re-add preserves a customized color), else the supplied fallback (a
// freshly-assigned palette swatch). Keeps `gogo source add` idempotent on color.
func existingSourceColor(project, path, fallback string) string {
	p, _ := projects.Load(project)
	for _, s := range p.Sources {
		if s.Path == path && s.Color != "" {
			return s.Color
		}
	}
	return fallback
}

// resolveProjectName resolves the target project for a source op: the given
// --project when non-empty; else the sole project when exactly one exists; else an
// error (none registered, or ambiguous → need --project). Returns the name and a 0
// exit code on success.
func resolveProjectName(cmd, projName string) (string, int) {
	if projName != "" {
		return projName, 0
	}
	projs, _ := projects.List()
	switch len(projs) {
	case 0:
		fmt.Fprintf(os.Stderr, "%s: no projects yet - create one with `gogo project add <repo>`\n", cmd)
		return "", 1
	case 1:
		return projs[0].Name, 0
	default:
		names := make([]string, 0, len(projs))
		for _, p := range projs {
			names = append(names, p.Name)
		}
		fmt.Fprintf(os.Stderr, "%s: several projects exist (%s) - pass --project <name>\n", cmd, joinNames(names))
		return "", 1
	}
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
