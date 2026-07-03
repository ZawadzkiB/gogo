package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// classRank fixes the status table's row order (goldenable, independent of
// dates): shipped → ready-to-ship → in-progress → unfinished.
func classRank(class string) int {
	switch class {
	case contract.ClassShipped:
		return 0
	case contract.ClassReadyToShip:
		return 1
	case contract.ClassInProgress:
		return 2
	default:
		return 3
	}
}

// cmdStatus prints the classifier table in a stable column order.
func cmdStatus(_ []string) int {
	root, ok := findRoot()
	if !ok {
		return 1
	}
	repo, _ := contract.LoadRepo(root)
	fmt.Print(FormatStatus(repo))
	return 0
}

// FormatStatus renders the work-index as a deterministic plain-text table.
// Exposed so a golden test can pin it.
func FormatStatus(repo *contract.Repo) string {
	feats := append([]*contract.Feature(nil), repo.Features...)
	sort.SliceStable(feats, func(i, j int) bool {
		if ri, rj := classRank(feats[i].Class), classRank(feats[j].Class); ri != rj {
			return ri < rj
		}
		return feats[i].Slug < feats[j].Slug
	})

	var counts [4]int
	for _, f := range feats {
		counts[classRank(f.Class)]++
	}

	var b strings.Builder
	fmt.Fprintf(&b, "gogo — %d features  (shipped %d · ready %d · in-progress %d · unfinished %d)\n\n",
		len(feats), counts[0], counts[1], counts[2], counts[3])
	fmt.Fprintf(&b, "%-14s %-10s %-18s %-28s %s\n", "CLASS", "PHASE", "STATUS", "ITERATIONS", "SLUG")
	b.WriteString(strings.Repeat("─", 96) + "\n")
	for _, f := range feats {
		fmt.Fprintf(&b, "%-14s %-10s %-18s %-28s %s\n",
			f.Class, dash(f.Phase), dash(f.Status), dash(f.Iterations), f.Slug)
	}
	return b.String()
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
