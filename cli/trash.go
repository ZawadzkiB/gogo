package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/trash"
)

// cmdTrash implements `gogo trash` (FR6): list the .gogo/trash/ entries, or
// `gogo trash restore <entry>` to move one back to .gogo/work/. The board's `x`
// key is the producer; this is the read/restore side.
func cmdTrash(args []string) int {
	root, ok := findRoot()
	if !ok {
		return 1
	}
	if len(args) > 0 && args[0] == "restore" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "gogo trash restore: missing <entry> (see `gogo trash`)")
			return 2
		}
		dest, err := trash.Restore(root, args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "gogo trash restore: %v\n", err)
			return 1
		}
		fmt.Printf("restored → %s\n", dest)
		return 0
	}
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "gogo trash: unknown argument %q (usage: `gogo trash` | `gogo trash restore <entry>`)\n", args[0])
		return 2
	}

	entries, err := trash.List(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogo trash: %v\n", err)
		return 1
	}
	fmt.Print(FormatTrash(entries))
	return 0
}

// FormatTrash renders the trash listing as a deterministic plain-text table.
// Exposed so a test can pin it.
func FormatTrash(entries []trash.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "gogo trash — %d entr%s (.gogo/trash/)\n\n", len(entries), plural(len(entries)))
	if len(entries) == 0 {
		b.WriteString("(empty — deleting a board card with `x` moves its folder here)\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%-20s %-26s %-22s %s\n", "WHEN", "SLUG", "WAS (phase/status)", "ENTRY")
	b.WriteString(strings.Repeat("─", 96) + "\n")
	for _, e := range entries {
		was := e.Phase + "/" + e.Status
		fmt.Fprintf(&b, "%-20s %-26s %-22s %s\n", e.When, e.Slug, was, e.Base)
	}
	b.WriteString("\nrestore:  gogo trash restore <entry>\n")
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
