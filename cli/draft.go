package main

import (
	"fmt"

	"github.com/ZawadzkiB/gogo/cli/internal/plans"
)

const draftHelp = `gogo draft — a thin ALIAS into ` + "`gogo plan`" + ` (a draft is a plan in status draft, D9)

usage:
  gogo draft new "<title>" [--project <p>] [--desc <text>]   create a draft plan (== gogo plan new)
  gogo draft list [--project <p>]                            list the DRAFTS (== gogo plan list --status draft)
  gogo draft show <id> [--project <p>]                       show a plan
  gogo draft ready <id> [--project <p>]                      mark a draft ready to spawn
  gogo draft rm <id> [--project <p>]                         delete a draft (a memberless plan)
  gogo draft delete <id> [--project <p>]                     delete a draft (alias of ` + "`draft rm`" + `)

A DRAFT is just a plan in the draft status (D8). Most subcommands forward to
` + "`gogo plan`" + `; ` + "`draft list`" + ` narrows to status draft, and ` + "`draft rm <id>`" + ` (a single id,
no ` + "`<source>`" + `) DELETES the memberless draft. See ` + "`gogo plan --help`" + `.
`

// cmdDraft is the `gogo draft` alias (D9): a draft is a plan in status draft, so this
// forwards to the project-scoped plan store, with `draft list` narrowed to drafts.
func cmdDraft(args []string) int {
	if len(args) == 0 {
		fmt.Print(draftHelp)
		return 0
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Print(draftHelp)
		return 0
	case "list", "ls":
		return planList(append([]string{"--status", plans.StatusDraft}, args[1:]...))
	case "rm", "remove":
		return draftRemove(args[1:])
	default:
		// new / show / ready / add / promote / delete → the canonical plan store.
		return cmdPlanStore(args)
	}
}

// draftRemove implements `gogo draft rm` (TEST-001). A draft is a MEMBERLESS plan, so
// the intuitive `gogo draft rm <id>` (a single id, no <source>) DELETES the draft —
// mapping to the plan-delete path — instead of forwarding to plan-rm-target, which
// demands a second <source> arg a draft never has and made the documented command a
// dead end. `gogo draft rm <id> <source>[:<slug>]` (an explicit target given) still
// unlinks via the canonical plan store, and `gogo plan rm <id> <source>` is unchanged.
func draftRemove(args []string) int {
	pa, ok, code := parsePlanArgs("gogo draft rm", args)
	if !ok {
		return code
	}
	if len(pa.pos) >= 2 {
		return planRemove(args) // <id> <source>[:<slug>] → the canonical target/member unlink
	}
	return planDelete(args) // <id> (or bare) → delete the memberless draft
}
