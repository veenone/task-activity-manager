package sprintreport_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/reports"
)

// A stored report is only ever as good as the fetch that made it, and
// reports.AlgoVersion invalidates one only when this app's own
// reconstruction changes. A series built while Jira was handing back cut
// short changelogs is wrong in a way nothing here can detect, so the two
// tests below are the two halves of the only answer to it: a report is
// served from the store by default, and asked for again when the user says
// so.

// storedSprintOne seeds the store with a current-version report for sprint 1
// that says something the fetch fixture does not, so a test can tell which
// of the two a report was served from. The truncation marker is the case
// this matters for: a row carrying one keeps it forever unless something
// builds the series again.
func storedSprintOne(store *fakeStore) {
	store.hold(reports.Series{
		SprintID: 1, SprintName: "Sprint 1", Unit: reports.UnitPoints,
		Committed: 99, Completed: 99, Truncated: []string{"PLAT-100"},
	}, reports.AlgoVersion, "2026-11-01T09:00:00Z")
}

func TestAReportServesTheStoredSeriesWhenItIsNotAskedToRefresh(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	store := newStore(columns(), closedSprint(1))
	storedSprintOne(store)

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Series.Committed != 99 {
		t.Errorf("committed = %v, want 99 from the store: a closed sprint is not refetched for every open of the view", got.Series.Committed)
	}
	if got.BuiltAt != "2026-11-01T09:00:00Z" {
		t.Errorf("builtAt = %q, want the stamp the store kept: a served report says when it was built and not when it was read", got.BuiltAt)
	}
	if history.asked[1] != 0 {
		t.Errorf("sprint 1 was fetched %d time(s), want none", history.asked[1])
	}
}

// TestARefreshedReportIsBuiltAgainAndReplacesWhatWasStored is the path out
// of a stored report that is wrong. Nothing else in TAM can replace a row
// reports.AlgoVersion still considers current, so without this a truncation
// marker from one bad afternoon at Jira's end would outlive every reopening
// of the view.
func TestARefreshedReportIsBuiltAgainAndReplacesWhatWasStored(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	store := newStore(columns(), closedSprint(1))
	storedSprintOne(store)

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1, true)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Series.Committed != 6 {
		t.Errorf("committed = %v, want 6 from the fetch: a refresh does not read the row it is replacing", got.Series.Committed)
	}
	if len(got.Series.Truncated) != 0 {
		t.Errorf("truncated = %v, want nothing: the changelogs came back whole this time", got.Series.Truncated)
	}
	if history.asked[1] == 0 {
		t.Fatal("sprint 1 was never fetched, so nothing was refreshed")
	}
	if !saved(store, 1) {
		t.Fatalf("stored %v, want sprint 1 written again: a refresh nobody kept would have to be asked for every single time", store.saved)
	}

	// And the next ordinary open of the view gets what the refresh left
	// behind rather than the row it replaced.
	next, err := service(newHistory(), store).Build(context.Background(), testProfile, testBoard, 1, false)
	if err != nil {
		t.Fatalf("the report after the refresh: %v", err)
	}
	if next.Series.Committed != 6 {
		t.Errorf("committed = %v after the refresh, want 6: the stale row is gone", next.Series.Committed)
	}
}
