package launch

import "testing"

func hasPair(argv []string, flag, val string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == val {
			return true
		}
	}
	return false
}

func hasFlag(argv []string, flag string) bool {
	for _, a := range argv {
		if a == flag {
			return true
		}
	}
	return false
}

func TestPhaseArgsFreshSession(t *testing.T) {
	cmd := "/gogo:review my-slug"
	argv := PhaseArgs(cmd, PhaseOpts{SessionID: "uuid-1", JSON: true})

	if !hasPair(argv, "--session-id", "uuid-1") {
		t.Errorf("expected --session-id uuid-1 as separate elements: %v", argv)
	}
	if hasFlag(argv, "--resume") {
		t.Errorf("fresh session must not carry --resume: %v", argv)
	}
	if !hasPair(argv, "--output-format", "json") {
		t.Errorf("expected --output-format json: %v", argv)
	}
	// Injection safety: the command is the single final element, never split.
	if n := len(argv); argv[n-2] != "-p" || argv[n-1] != cmd {
		t.Errorf("command must be the single element after -p: %v", argv)
	}
}

func TestPhaseArgsResume(t *testing.T) {
	argv := PhaseArgs("/gogo:implement s --issues p --in-session", PhaseOpts{Resume: "dev-uuid"})
	if !hasPair(argv, "--resume", "dev-uuid") {
		t.Errorf("expected --resume dev-uuid: %v", argv)
	}
	if hasFlag(argv, "--session-id") {
		t.Errorf("a resume must not also pre-assign --session-id: %v", argv)
	}
}

func TestPhaseArgsResumeWinsOverSessionID(t *testing.T) {
	// You cannot pre-assign an id to an existing session; resume must win.
	argv := PhaseArgs("/gogo:test s", PhaseOpts{SessionID: "fresh", Resume: "warm"})
	if !hasPair(argv, "--resume", "warm") || hasFlag(argv, "--session-id") {
		t.Errorf("resume must win over session-id: %v", argv)
	}
}

func TestPhaseArgsOneShot(t *testing.T) {
	// A plain one-shot (⑤ report): neither --session-id nor --resume.
	argv := PhaseArgs("/gogo:report s", PhaseOpts{})
	if hasFlag(argv, "--session-id") || hasFlag(argv, "--resume") {
		t.Errorf("one-shot must carry no session flags: %v", argv)
	}
	n := len(argv)
	if argv[n-2] != "-p" || argv[n-1] != "/gogo:report s" {
		t.Errorf("command must be the single final element: %v", argv)
	}
}
