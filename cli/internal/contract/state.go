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
	// inComment carries the "inside an unclosed <!--" state ACROSS lines (TEST-001). Without
	// it, a `- **key:** value` line that only exists as EXAMPLE PROSE inside a multi-line
	// legend comment parses as a real field - see commentedOut.
	inComment := false
	for sc.Scan() {
		line := sc.Text()
		// Did this line START inside a comment? Compute before advancing, and advance
		// unconditionally so the flag is correct for the next line.
		commented := inComment
		inComment = advanceComment(line, inComment)
		if commented {
			continue
		}
		m := stateLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		val := stripPlaceholder(stripComment(m[2]))
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

// advanceComment reports whether the parser is inside an unclosed `<!--` AFTER consuming line,
// given whether it was inside one before. It walks the line alternating between "look for the
// closer" and "look for the next opener", so several comments can open and close on one line.
//
// It is the whole of TEST-001's fix. parseStateFile matched `- **key:** value` on ANY line, and
// stripComment only ever removed a comment that opened on the SAME line - so a key line living
// purely as EXAMPLE PROSE inside a multi-line legend block parsed as a real field. The shipped
// template's own optional-correlation legend is exactly that:
//
//	<!-- optional, additive - include ONLY when this work item belongs to ... plans.
//	- **correlation:** [plan-XXXX]   plan id(s) this item belongs to, e.g. [plan-7f3a, plan-9c2e]
//	-->
//
// which parsed into THREE bogus plan ids and painted a `⛓ ×3` chip on any item scaffolded
// straight from the template - the very fixture this release's own Slice-A scenarios prescribe.
// It is the same defect family as stripPlaceholder: the template's legend leaking into card UI.
//
// The decisions, all in the "may only ever make a value MISSING, never WRONG" direction:
//
//   - A line that STARTS inside a comment is skipped whole. So is a line that closes a comment
//     and then carries a real key (`--> - **phase:** plan`) - that shape appears in no template
//     and no real state.md, and skipping it loses a value rather than inventing one.
//   - `<!--` and `-->` on ONE line leave the flag false, so today's single-line behaviour is
//     untouched: the line is still parsed and stripComment still trims the trailing comment
//     from the value, byte-for-byte as before.
//   - A stray `-->` with no opener is ignored (nothing is open, so nothing closes) and its line
//     parses normally. Conversely - and this one has teeth - a `-->` appearing in PROSE inside an
//     open block CLOSES it, exactly as a markdown renderer would, so whatever follows becomes
//     live contract again. That is not a parser bug to work around; it is what the file means.
//     It is why templates/state.template.md now warns against writing comment tokens inside a
//     legend block: the first draft of that very warning contained a literal closer, reopened
//     this defect, and was caught immediately by the test that reads the shipped template.
//   - An UNTERMINATED `<!--` comments out the rest of the file. That is deliberate: it is what
//     every markdown renderer shows the user, so the CLI agrees with the file's appearance
//     rather than second-guessing it. It is not silent either - the keys after it go missing, so
//     the card reads as a malformed/authoring item (no phase, no status) instead of a
//     plausible-looking wrong one, which is the degradation docs/cli-contract.md §2's
//     "parse defensively" mandate already describes. Keys BEFORE the opener are unaffected, so a
//     truncated write loses only what follows it (pinned by test).
func advanceComment(line string, in bool) bool {
	for {
		if in {
			i := strings.Index(line, "-->")
			if i < 0 {
				return true // still open at end of line
			}
			line = line[i+len("-->"):]
			in = false
			continue
		}
		i := strings.Index(line, "<!--")
		if i < 0 {
			return false // nothing open at end of line
		}
		line = line[i+len("<!--"):]
		in = true
	}
}

// stripPlaceholder treats a value that is a bare `<…>` TEMPLATE PLACEHOLDER as absent
// (FR6). templates/state.template.md ships `- **feature:** <one-line title>`,
// `<YYYY-MM-DD>`, `<slug>` and `<git branch | n/a>`; an analyst that scaffolds the file
// and does not finish it leaves those verbatim, and they used to render as REAL data -
// the card title read `<one-line title>`, and a `created:` of `<YYYY-MM-DD>` sorted the
// most-broken card to the TOP of the plan column (sortFeaturesNewestFirst compares raw
// strings and '<' 0x3C > '2' 0x32). Applied to EVERY key after stripComment: one
// defensive-reader rule rather than a per-key list, matching the contract's own
// "parse defensively" mandate (docs/cli-contract.md §2). The rule is the simple one: a
// value that OPENS with `<` and CLOSES with `>` is a placeholder (so the resume line's
// `<phase to re-enter> - <next action>` counts too); a value that merely contains angle
// brackets somewhere inside is untouched. It can only ever make a value MISSING - which
// every reader already handles (the card falls back to the slug) - so the worst case is a
// degradation, never invented data.
func stripPlaceholder(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return ""
	}
	return s
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
