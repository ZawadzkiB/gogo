package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCLICommandEnumerationInSync is the enumeration-sync guard for Slice A
// (cockpit-cards-and-cli-awareness, FR-A5). The gogo CLI command surface is
// defined in cli/main.go (the runtime truth) and MUST stay mirrored in every
// human/agent-facing enumeration of it — the FOUR FR-A5 sync sources:
//   - cli/main.go printHelp            (the runtime help text)
//   - README.md "## The gogo CLI"      (the human install + feature narrative)
//   - docs/cli-contract.md             (the consumer file-surface contract)
//   - skills/gogo-cli/SKILL.md         (the on-demand CLI companion reference)
//
// It derives the canonical command verbs from main.go's `switch args[0]`
// dispatch (so a new/renamed/removed command is picked up automatically, not
// from hand-maintained prose) and fails if any verb is missing from any of the
// four sources — so a missing or renamed command can't drift silently. Checking
// printHelp too (REV-002) closes the loop: the help text can't drift from the
// dispatch either (the dispatch uses "go"-style case literals, printHelp uses
// "gogo go"-style usage lines, so this genuinely checks the help block).
func TestCLICommandEnumerationInSync(t *testing.T) {
	verbs := canonicalCommandVerbs(t)
	if len(verbs) < 5 {
		t.Fatalf("parsed only %d command verbs from main.go (%v) — parser drift?", len(verbs), verbs)
	}

	docs := map[string]string{
		"cli/main.go printHelp":    readRepoFile(t, "main.go"),
		"README.md":                readRepoFile(t, "..", "README.md"),
		"docs/cli-contract.md":     readRepoFile(t, "..", "docs", "cli-contract.md"),
		"skills/gogo-cli/SKILL.md": readRepoFile(t, "..", "skills", "gogo-cli", "SKILL.md"),
	}

	for _, v := range verbs {
		// A subcommand is enumerated as "gogo <verb>"; the version flag as "--version".
		needle := "gogo " + v
		if strings.HasPrefix(v, "-") {
			needle = v
		}
		for name, body := range docs {
			if !strings.Contains(body, needle) {
				t.Errorf("command %q missing from %s (expected substring %q)\n"+
					"  the CLI surface drifted — update EVERY enumeration: cli/main.go help / README ## The gogo CLI / docs/cli-contract.md / skills/gogo-cli/SKILL.md",
					v, name, needle)
			}
		}
	}
}

// canonicalCommandVerbs parses cli/main.go's `switch args[0]` dispatch block and
// returns the real command verbs (the version flag kept, the help/version-alias
// tokens dropped) in the order they appear.
func canonicalCommandVerbs(t *testing.T) []string {
	t.Helper()
	src := readRepoFile(t, "main.go")

	start := strings.Index(src, "switch args[0] {")
	if start < 0 {
		t.Fatal("could not find `switch args[0] {` in main.go — dispatch parser needs updating")
	}
	// The switch body ends at the first single-tab-indented closing brace.
	rest := src[start:]
	end := strings.Index(rest, "\n\t}")
	if end < 0 {
		t.Fatal("could not find the end of the dispatch switch in main.go")
	}
	block := rest[:end]

	// Aliases that are not distinct commands in the user-facing enumeration.
	alias := map[string]bool{"-v": true, "version": true, "-h": true, "--help": true, "help": true}
	litRe := regexp.MustCompile(`"([^"]+)"`)

	seen := map[string]bool{}
	var verbs []string
	for _, line := range strings.Split(block, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "case ") {
			continue
		}
		for _, m := range litRe.FindAllStringSubmatch(line, -1) {
			tok := m[1]
			if alias[tok] || seen[tok] {
				continue
			}
			seen[tok] = true
			verbs = append(verbs, tok)
		}
	}
	return verbs
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(data)
}
