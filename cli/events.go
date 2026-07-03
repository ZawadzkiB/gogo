package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
	"github.com/ZawadzkiB/gogo/cli/internal/textfmt"
)

// cmdEvents prints a feature's events.jsonl as a timeline. A missing stream is
// handled gracefully (older features predate the contract).
func cmdEvents(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "gogo events: missing <slug>")
		return 2
	}
	slug, _ := contract.SlugFromArg(args[0])
	root, ok := findRoot()
	if !ok {
		return 1
	}
	repo, _ := contract.LoadRepo(root)
	f := repo.Feature(slug)
	if f == nil {
		fmt.Fprintf(os.Stderr, "gogo events: no feature %q\n", slug)
		return 1
	}
	evs := contract.ReadEvents(filepath.Join(f.Dir, "events.jsonl"))
	if len(evs) == 0 {
		fmt.Printf("%s — no events recorded yet (events.jsonl is optional; state.md phase is %q)\n", slug, f.Phase)
		return 0
	}
	fmt.Print(textfmt.Timeline(evs))
	return 0
}
