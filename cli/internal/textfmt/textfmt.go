// Package textfmt renders contract types as plain, terminal-friendly text —
// shared by the TUI drill-in viewers and the non-interactive subcommands so
// there is one formatting implementation.
package textfmt

import (
	"fmt"
	"strings"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// Issues formats a review/test issues.json as a readable table + detail list.
func Issues(list *contract.IssuesList) string {
	if list == nil {
		return "no issues"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s · track %s · round %d", list.Slug, list.Track, list.Round)
	if list.Updated != "" {
		fmt.Fprintf(&b, " · updated %s", list.Updated)
	}
	b.WriteString("\n\n")
	if len(list.Issues) == 0 {
		b.WriteString("(no findings)\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%-9s %-9s %-4s %-10s %s\n", "ID", "SEVERITY", "PRIO", "STATUS", "TITLE")
	b.WriteString(strings.Repeat("─", 60) + "\n")
	for _, is := range list.Issues {
		fmt.Fprintf(&b, "%-9s %-9s %-4s %-10s %s\n", is.ID, is.Severity, is.Priority, is.Status, is.Title)
	}
	b.WriteString("\n")
	for _, is := range list.Issues {
		fmt.Fprintf(&b, "▸ %s — %s\n", is.ID, is.Title)
		if is.Description != "" {
			fmt.Fprintf(&b, "  %s\n", is.Description)
		}
		if is.ProposedSolution != "" {
			fmt.Fprintf(&b, "  fix: %s\n", is.ProposedSolution)
		}
		if is.FixSummary != "" {
			fmt.Fprintf(&b, "  fixed (r%d): %s\n", is.FixedInRound, is.FixSummary)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Timeline formats events.jsonl as a chronological "ts · event · phase · round
// · note" list. An invalid ts is shown raw (the line was still kept).
func Timeline(evs []contract.Event) string {
	if len(evs) == 0 {
		return "no events recorded"
	}
	var b strings.Builder
	b.WriteString("events timeline (ts · event · phase · round · note)\n\n")
	for _, e := range evs {
		ts := e.TSRaw
		if e.TSValid {
			ts = e.TS.Format("2006-01-02 15:04:05")
		}
		round := ""
		if e.HasRound {
			round = fmt.Sprintf(" r%d", e.Round)
		}
		line := fmt.Sprintf("%s  %-14s %-9s%s", ts, e.Event, e.Phase, round)
		if e.Note != "" {
			line += "  — " + e.Note
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
