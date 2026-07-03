package contract

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// stateLine matches the state.md contract grammar (docs/cli-contract.md §2):
//
//   - **<key>:** <value>            <!-- optional trailing HTML comment -->
//
// The leading HTML-comment legend block and any non-bolded line are ignored.
var stateLine = regexp.MustCompile(`^\s*-\s*\*\*([^:*]+):\*\*\s*(.*)$`)

// parseStateFile reads state.md leniently. A missing or unreadable file yields
// an empty Feature (never an error) so absence degrades to "unknown", not a
// crash. Unknown bolded keys are preserved in Extra.
func parseStateFile(path string) *Feature {
	f := &Feature{Extra: map[string]string{}}
	file, err := os.Open(path)
	if err != nil {
		return f
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		m := stateLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		val := stripComment(m[2])
		switch key {
		case "feature":
			f.Title = val
		case "phase":
			f.Phase = firstToken(val)
		case "status":
			f.Status = firstToken(val)
		case "created":
			f.Created = firstToken(val)
		case "completed":
			f.Completed = firstToken(val)
		case "branch":
			f.Branch = val
		case "iterations":
			f.Iterations = val
		case "resume":
			f.Resume = val
		case "open-decision":
			f.OpenDecision = val
		case "stage":
			f.Stage = val
		default:
			f.Extra[key] = val
		}
	}
	return f
}

// stripComment removes a trailing "<!-- ... -->" HTML comment and trims.
func stripComment(s string) string {
	if i := strings.Index(s, "<!--"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// firstToken returns the first whitespace-delimited token (for the enum-like
// keys phase/status/created that must not carry a trailing "(...)" note).
func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}
