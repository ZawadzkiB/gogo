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
		case "correlation":
			// The additive, optional correlation LIST (FR13/L1): plan ids a work item
			// belongs to, e.g. `- **correlation:** [plan-7f3a, plan-9c2e]`. Absent →
			// Correlations stays nil (byte-for-byte parity with a pre-correlation
			// state.md). Many-to-many: a ticket in several plans carries several ids.
			if ids := parseCorrelationList(val); len(ids) > 0 {
				f.Correlations = ids
			}
		default:
			f.Extra[key] = val
		}
	}
	return f
}

// parseCorrelationList parses the correlation value into plan ids, tolerating the
// bracketed list form (`[plan-a, plan-b]`), a bare single id (`plan-a`), extra
// whitespace, and an empty list (`[]` → nil). It never errors — a malformed value
// degrades to whatever tokens it can recover (the reader's defensive style).
func parseCorrelationList(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, tok := range strings.Split(v, ",") {
		if tok = strings.TrimSpace(tok); tok != "" {
			out = append(out, tok)
		}
	}
	return out
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
