package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/ZawadzkiB/gogo/cli/internal/contract"
)

// TestNotifyHookSelftest runs hooks/notify.sh's built-in decision-table
// selftest (payload x fixture verdicts, both FR5 parse traps, the authoring
// carve-out, the GOGO_NOTIFY_LEVEL knob, degradation paths). The selftest
// sends nothing; this keeps it green in CI (notify-only-at-user-gates, FR8).
func TestNotifyHookSelftest(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH — selftest not runnable here")
	}
	script := filepath.Join("..", "hooks", "notify.sh")
	out, err := exec.Command(bash, script, "--selftest").CombinedOutput()
	if err != nil {
		t.Fatalf("bash %s --selftest failed: %v\n%s", script, err, out)
	}
	if !strings.Contains(string(out), "passed") {
		t.Fatalf("selftest produced no summary line:\n%s", out)
	}
}

// TestNotifyHookGateStatusesMatchContract is the anti-drift guard
// (notify-only-at-user-gates FR8, the TEST-006 lesson): the gate-status list
// hooks/notify.sh enumerates (GOGO_GATE_STATUSES) must be EXACTLY the statuses
// contract.WaitingForInput() accepts — derived by running every status in the
// state.md enum through the predicate, never hand-copied. The enum itself is
// read from templates/state.template.md's status legend comment (the on-disk
// enumeration a new status lands in), so a contract change surfaces here too.
func TestNotifyHookGateStatusesMatchContract(t *testing.T) {
	// 1. The status enum, from the template's legend comment:
	//    - **status:** X   <!-- awaiting-plan-acceptance | plan-accepted | ... -->
	tmpl := readRepoFile(t, "..", "templates", "state.template.md")
	legendRe := regexp.MustCompile(`\*\*status:\*\*[^<]*<!--([^>]+)-->`)
	m := legendRe.FindStringSubmatch(tmpl)
	if m == nil {
		t.Fatal("templates/state.template.md: no `- **status:** ... <!-- enum -->` legend found — template shape changed, update this parser")
	}
	var enum []string
	for _, tok := range strings.Split(m[1], "|") {
		if s := strings.TrimSpace(tok); s != "" {
			enum = append(enum, s)
		}
	}
	// A guard must never pass vacuously: the enum must be a real list that
	// contains at least the non-gate statuses we know flow unattended.
	if len(enum) < 5 {
		t.Fatalf("status enum parsed from template is implausibly small: %v", enum)
	}

	// 2. The expected gate set: every enum status the contract predicate
	//    accepts. Zero-value PlanUnwritten == plan written, so the
	//    awaiting-plan-acceptance gate counts (the authoring carve-out is
	//    behavioral, covered by the bash selftest's fixtures).
	var want []string
	enumSet := map[string]bool{}
	for _, s := range enum {
		enumSet[s] = true
		f := &contract.Feature{Status: s}
		if f.WaitingForInput() {
			want = append(want, s)
		}
	}
	if len(want) == 0 {
		t.Fatal("no enum status satisfies contract.WaitingForInput() — enum parse or contract broke")
	}

	// 2b. The guard's blind spot (REV-003): running the predicate over the
	//     template's enum only detects drift that also lands in the template.
	//     So source-scan WaitingForInput's body and require every status
	//     literal it names to be in that enum — a new gate status added to the
	//     contract alone now fails here instead of silently shrinking `want`.
	contractSrc := readRepoFile(t, "internal", "contract", "contract.go")
	fnStart := strings.Index(contractSrc, "func (f *Feature) WaitingForInput()")
	if fnStart < 0 {
		t.Fatal("contract.go: WaitingForInput function not found — source scan needs updating")
	}
	fnEnd := strings.Index(contractSrc[fnStart:], "\n}")
	if fnEnd < 0 {
		t.Fatal("contract.go: could not find the end of WaitingForInput")
	}
	body := contractSrc[fnStart : fnStart+fnEnd]
	litRe := regexp.MustCompile(`"([a-z][a-z-]*)"`)
	var bodyLits []string
	for _, m := range litRe.FindAllStringSubmatch(body, -1) {
		bodyLits = append(bodyLits, m[1])
	}
	if len(bodyLits) == 0 {
		t.Fatal("no status literals found inside WaitingForInput — a guard must never pass vacuously")
	}
	for _, l := range bodyLits {
		if !enumSet[l] {
			t.Errorf("contract.WaitingForInput() names %q, which is missing from templates/state.template.md's status enum — the guard cannot see it; add it to the template legend (and to hooks/notify.sh GOGO_GATE_STATUSES if it is a gate)", l)
		}
	}

	// 3. The bash side's enumeration.
	hook := readRepoFile(t, "..", "hooks", "notify.sh")
	gateRe := regexp.MustCompile(`(?m)^GOGO_GATE_STATUSES="([^"]+)"`)
	g := gateRe.FindStringSubmatch(hook)
	if g == nil {
		t.Fatal(`hooks/notify.sh: no GOGO_GATE_STATUSES="..." line found — the guard needs that greppable constant`)
	}
	got := strings.Fields(g[1])

	// 4. Exact set equality, order-independent.
	sortedCopy := func(in []string) []string {
		out := append([]string(nil), in...)
		sort.Strings(out)
		return out
	}
	if fmt.Sprint(sortedCopy(want)) != fmt.Sprint(sortedCopy(got)) {
		t.Fatalf("gate statuses drifted:\n  contract.WaitingForInput() accepts: %v\n  hooks/notify.sh GOGO_GATE_STATUSES:  %v\nchange both or neither (see notify.sh's comment)", want, got)
	}
}
