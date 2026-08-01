package launch

// Session-binding ops (0.32.0): the rename primitive + the session-meta reader
// behind the cockpit's P/K/R keys. These pin the two tmux contracts that caused
// real bugs — the exact `=<name>` SESSION target on `-t` (a prefix target
// provably hit the wrong session) and the bare NAME in the new-name position
// (a `=` would become part of the name) — plus the lenient meta line parse.

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestRenameSessionArgs pins the exact-target contract: `-t` takes the `=<old>`
// SESSION form (prefix/fnmatch fallbacks disabled — FR1.5), while the new name
// is a BARE name, exactly like `new-session -s` (the `=` would join the name;
// verified live on tmux 3.7b).
func TestRenameSessionArgs(t *testing.T) {
	got := RenameSessionArgs("gogo-go-item-a", "gogo-go-item-b")
	want := []string{"rename-session", "-t", "=gogo-go-item-a", "gogo-go-item-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RenameSessionArgs = %v, want %v", got, want)
	}
	if strings.HasPrefix(got[len(got)-1], "=") {
		t.Error("new-name position carries a '=' — it is a NAME, not a target; tmux would name the session '=...'")
	}
}

// TestRenameSessionRefusesEmpty: an empty name is an error, never a tmux call
// (an empty -t would resolve tmux's "most recently used" session — the worst
// possible target for a rename).
func TestRenameSessionRefusesEmpty(t *testing.T) {
	if err := RenameSession("", "gogo-go-x"); err == nil {
		t.Error("RenameSession(\"\", x) succeeded, want an error")
	}
	if err := RenameSession("gogo-go-x", ""); err == nil {
		t.Error("RenameSession(x, \"\") succeeded, want an error")
	}
}

// TestSessionNameForRoundTrips: the exported minting helper produces exactly the
// convention SessionAction parses, so a session renamed by `R` is attributable
// by every reader — the whole point of the rename.
func TestSessionNameForRoundTrips(t *testing.T) {
	cases := []struct {
		action Action
		slug   string
		want   string
	}{
		{ActionGo, "item-b", "gogo-go-item-b"},
		{ActionPlan, "item-b", "gogo-plan-item-b"},
	}
	for _, c := range cases {
		got := SessionNameFor(c.action, c.slug)
		if got != c.want {
			t.Errorf("SessionNameFor(%s, %s) = %q, want %q", c.action, c.slug, got, c.want)
		}
		a, ok := SessionAction(got, c.slug)
		if !ok || a != c.action {
			t.Errorf("SessionAction(%q, %q) = (%v,%v) — the minted name does not round-trip", got, c.slug, a, ok)
		}
	}
}

// TestParseSessionMeta: the pure parser behind ListSessionMeta. Lenient per the
// CLI reader contract — a non-gogo name and a line without all four fields are
// skipped (never a crash); a non-numeric created keeps the row with a zero time
// (the NAME is the binding, the timestamp is cosmetic).
func TestParseSessionMeta(t *testing.T) {
	out := strings.Join([]string{
		"gogo-go-auth|/repos/auth|1722400000|1",
		"gogo-plan-catalogue-side-of-the-matching-engine|/repos/dotai|1722400500|0",
		"other-session|/x|1722400000|0", // not gogo-* → skipped
		"gogo-broken-no-fields",         // garbled → skipped, never a crash
		"gogo-go-weird|/p|notanumber|2", // bad created → kept, zero time, attached
		"",
	}, "\n")
	metas := parseSessionMeta(out)
	if len(metas) != 3 {
		t.Fatalf("parseSessionMeta kept %d rows, want 3: %+v", len(metas), metas)
	}
	first := metas[0]
	if first.Name != "gogo-go-auth" || first.Path != "/repos/auth" || !first.Attached {
		t.Errorf("row 0 = %+v, want gogo-go-auth /repos/auth attached", first)
	}
	if want := time.Unix(1722400000, 0); !first.Created.Equal(want) {
		t.Errorf("row 0 Created = %v, want %v", first.Created, want)
	}
	if metas[1].Attached {
		t.Errorf("row 1 attached = true, want false (field was 0)")
	}
	if !metas[2].Created.IsZero() || !metas[2].Attached {
		t.Errorf("row 2 = %+v, want zero Created (bad epoch) and attached (field 2)", metas[2])
	}
}

// TestParseSessionMetaEmpty: no output → no rows, never a panic.
func TestParseSessionMetaEmpty(t *testing.T) {
	if got := parseSessionMeta(""); len(got) != 0 {
		t.Errorf("parseSessionMeta(\"\") = %+v, want none", got)
	}
}
