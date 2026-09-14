package sprintreport_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agile-suite/tam/internal/sprintreport"
)

// TestANegativeSprintIdIsNotARequestForTheDefault. Zero means the board's
// most recent closed sprint, deliberately, because that is the call the view
// makes before its picker has anything in it and Wails decodes a missing
// argument as zero. A negative id is a caller with a real bug, and reporting
// on a different sprint than the one it named, with nothing anywhere saying
// so, is the worst answer available.
func TestANegativeSprintIdIsNotARequestForTheDefault(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	store := newStore(columns(), closedSprint(1))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, -1, false)
	if err != nil {
		t.Fatalf("Build: %v, want the reason in the report rather than an error", err)
	}
	if got.Unavailable != sprintreport.ReasonSprintNotFound {
		t.Errorf("unavailable = %q, want %q: a negative id names no sprint", got.Unavailable, sprintreport.ReasonSprintNotFound)
	}
	if got.Series.SprintID != 0 {
		t.Errorf("the report is of sprint %d, want none: a bad id must not quietly resolve to the newest closed sprint", got.Series.SprintID)
	}
	if history.asked[1] != 0 {
		t.Errorf("sprint 1 was fetched %d time(s), want none", history.asked[1])
	}
}

// TestAnUnavailableReportSendsEmptyListsRatherThanNulls. The view renders
// one state here and should not have to guard three fields three different
// ways to do it: a nil Go slice marshals to null, and the velocity rows, the
// series' days and its truncated list would otherwise disagree about how an
// empty one looks.
func TestAnUnavailableReportSendsEmptyListsRatherThanNulls(t *testing.T) {
	// No columns, so the board cannot say what finished means, which is the
	// reason answered before anything else happens.
	store := newStore(nil, closedSprint(1))

	got, err := service(newHistory(), store).Build(context.Background(), testProfile, testBoard, 1, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Unavailable == "" {
		t.Fatalf("report = %+v, want one carrying a reason", got)
	}
	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal the report the way Wails does: %v", err)
	}
	wire := string(blob)
	for _, want := range []string{`"velocity":[]`, `"days":[]`, `"truncated":[]`} {
		if !strings.Contains(wire, want) {
			t.Errorf("the report on the wire is %s, want %s in it", wire, want)
		}
	}
	if strings.Contains(wire, "null") {
		t.Errorf("the report on the wire is %s, want no nulls in it", wire)
	}
}
