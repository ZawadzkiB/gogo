package contract

import (
	"encoding/json"
	"os"
)

// PhaseResult is a phase's per-run result.json (phase-result.schema.json): the
// small record each standalone command writes when it finishes. The CLI
// process-orchestrator (gogo run) reads it to chain phases — status + open_issues
// drive the routing decision (see Route).
type PhaseResult struct {
	Slug         string   `json:"slug"`
	Phase        string   `json:"phase"`  // plan | implement | review | test | report
	Status       string   `json:"status"` // ok | blocked | waiting-for-user
	Round        int      `json:"round"`
	Inputs       []string `json:"inputs"`
	Outputs      []string `json:"outputs"`
	ValidatedIn  bool     `json:"validated_in"`
	ValidatedOut bool     `json:"validated_out"`
	OpenIssues   int      `json:"open_issues"`
	Summary      string   `json:"summary"`
}

// ReadResult parses a phase's result.json. Absent file → (nil, nil): a phase that
// has not written its result yet (or an older run) degrades to "unknown", never a
// crash — the orchestrator then routes on the issues list + state.md alone.
func ReadResult(path string) (*PhaseResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var r PhaseResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
