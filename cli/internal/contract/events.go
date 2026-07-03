package contract

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// Event is one events.jsonl line (docs/cli-contract.md §5). Parsed leniently:
// unknown fields are ignored (forward-compat) and an unparseable ts keeps the
// event with TSValid=false rather than dropping a real transition.
type Event struct {
	TS       time.Time
	TSRaw    string
	TSValid  bool
	Event    string
	Phase    string
	Status   string
	Round    int
	HasRound bool
	Note     string
	Slug     string
}

// rawEvent is the on-disk JSON shape; Round is a pointer so "absent" is
// distinguishable from 0.
type rawEvent struct {
	TS     string `json:"ts"`
	Event  string `json:"event"`
	Phase  string `json:"phase"`
	Status string `json:"status"`
	Round  *int   `json:"round"`
	Note   string `json:"note"`
	Slug   string `json:"slug"`
}

// ReadEvents parses events.jsonl in file (append) order. It is JSON Lines, not
// a JSON array: each line is decoded independently and a malformed line is
// skipped, never fatal. A missing file yields nil (never an error) — older
// features predate the contract.
func ReadEvents(path string) []Event {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var out []Event
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var re rawEvent
		if json.Unmarshal([]byte(line), &re) != nil {
			continue // skip a malformed line, keep going
		}
		ev := Event{
			TSRaw:  re.TS,
			Event:  re.Event,
			Phase:  re.Phase,
			Status: re.Status,
			Note:   re.Note,
			Slug:   re.Slug,
		}
		if t, perr := time.Parse(time.RFC3339, re.TS); perr == nil {
			ev.TS = t
			ev.TSValid = true
		}
		if re.Round != nil {
			ev.Round = *re.Round
			ev.HasRound = true
		}
		out = append(out, ev)
	}
	return out
}
