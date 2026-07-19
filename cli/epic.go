package main

import (
	"fmt"
	"os"

	"github.com/ZawadzkiB/gogo/cli/internal/plans"
)

const epicHelp = `gogo epic — a thin ALIAS into ` + "`gogo plan`" + ` (an epic is a plan with members, D9)

usage:
  gogo epic new "<title>" [--project <p>] [--desc <text>]   create a plan (== gogo plan new)
  gogo epic list [--project <p>]                            list the EPICS — plans that own members/targets (any status)
  gogo epic show <id> [--project <p>]                       show a plan + its work items
  gogo epic add <id> <source>:<slug> [--project <p>]        link an existing work item (many-to-many, FR16)
  gogo epic rm  <id> <source>:<slug> [--project <p>]        unlink a work item
  gogo epic delete <id> [--project <p>]                     delete a plan

An EPIC is just a plan that owns members across sources (D8). Every subcommand
forwards to ` + "`gogo plan`" + `; ` + "`epic list`" + ` narrows to plans that carry ≥1 member (or
≥1 target) — REGARDLESS of status, since ` + "`epic add`" + ` links a member without flipping
status (so a just-linked epic still appears). See ` + "`gogo plan --help`" + `.
`

// cmdEpic is the `gogo epic` alias (D9): an epic is a plan with members, so this
// forwards to the project-scoped plan store, with `epic list` narrowed to the
// member-bearing (or target-bearing) plans — independent of status (REV-003).
func cmdEpic(args []string) int {
	if len(args) == 0 {
		fmt.Print(epicHelp)
		return 0
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Print(epicHelp)
		return 0
	case "list", "ls":
		return epicList(args[1:])
	default:
		// new / show / add / rm / delete → the canonical plan store.
		return cmdPlanStore(args)
	}
}

// epicList prints the project's EPICS — plans that own ≥1 member (or ≥1 target),
// REGARDLESS of lifecycle status (D9: an epic is a plan with members). `epic add` /
// AddMember links a member without flipping status, so filtering on `status==active`
// (the old behaviour) hid a just-linked draft/ready epic from its own list (REV-003).
func epicList(args []string) int {
	pa, ok, code := parsePlanArgs("gogo epic list", args)
	if !ok {
		return code
	}
	project, code := resolveProjectName("gogo epic list", pa.project)
	if code != 0 {
		return code
	}
	list, err := plans.List(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo epic list: %v\n", err)
		return 1
	}
	epics := list[:0]
	for _, p := range list {
		if p.HasMembers() || len(p.Targets) > 0 {
			epics = append(epics, p)
		}
	}
	fmt.Print(FormatPlans(project, epics))
	return 0
}
