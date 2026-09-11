package sprintreport_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/reports"
)

// The velocity table is the half of this package that can go quietly wrong,
// because some of its rows come from a series reconstructed a moment ago and
// the rest from a series read back out of the store. Each test here pins one
// of the three properties that keeps those two halves the same table.

// asBackend is the sprint rows as internal/reports takes them, so a test can
// ask reports.VelocitySprints what the table should hold and compare that
// against what the service produced.
func asBackend(rows []boardrepo.Sprint) []backend.Sprint {
	out := make([]backend.Sprint, 0, len(rows))
	for _, r := range rows {
		out = append(out, backend.Sprint{
			ID: r.ID, BoardID: r.BoardID, Name: r.Name, State: r.State,
			StartDate: r.StartDate, EndDate: r.EndDate, Goal: r.Goal, CompleteDate: r.CompleteDate,
		})
	}
	return out
}

// TestACachedSprintAndAFreshlyBuiltSprintGiveTheIdenticalRow builds one
// sprint both ways and compares the row: fetched and reconstructed the first
// time, read back out of the store the second. A row assembled in two places
// would drift the first time either place changed, and the drift would show
// up as a velocity table disagreeing with the report printed above it.
func TestACachedSprintAndAFreshlyBuiltSprintGiveTheIdenticalRow(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 4, 2, points(3))
	store := newStore(columns(), closedSprint(1))

	fresh, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1)
	if err != nil {
		t.Fatalf("the first report: %v", err)
	}
	if len(fresh.Velocity) != 1 {
		t.Fatalf("velocity = %+v, want the one closed sprint's row", fresh.Velocity)
	}

	// A backend that refuses everything, so the second report can only have
	// come out of the store the first one filled.
	refusing := newHistory()
	refusing.fail = errors.New("this sprint should have been read from the store")
	cached, err := service(refusing, store).Build(context.Background(), testProfile, testBoard, 1)
	if err != nil {
		t.Fatalf("the second report: %v", err)
	}
	if !reflect.DeepEqual(fresh.Velocity, cached.Velocity) {
		t.Errorf("the stored sprint's row is %+v and the freshly built one is %+v; one sprint has one row however its series arrived",
			cached.Velocity, fresh.Velocity)
	}
}

// TestTheTableHoldsTheSprintsVelocitySprintsPicksInThatOrder pins the second
// property: which sprints are in the table, and in what order, is decided in
// exactly one place. The expectation is not a list written out here but
// whatever reports.VelocitySprints answers, so a change to that rule moves
// both sides of this comparison and a second copy of it in the service moves
// only one.
func TestTheTableHoldsTheSprintsVelocitySprintsPicksInThatOrder(t *testing.T) {
	history := newHistory()
	sprints := []boardrepo.Sprint{}
	for n := 1; n <= 9; n++ {
		sprints = append(sprints, closedSprint(n))
		history.issues[n] = held(n, 2, 1, points(3))
	}
	// A sprint still running has no place in a table of finished work, and
	// its issues are here so that is what excludes it rather than the fetch.
	sprints = append(sprints, liveSprint(10))
	history.issues[10] = held(10, 2, 0, points(3))
	store := newStore(columns(), sprints...)

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 9)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := reports.VelocitySprints(asBackend(sprints))
	if len(got.Velocity) != len(want) {
		t.Fatalf("velocity holds %d row(s), want %d", len(got.Velocity), len(want))
	}
	for i, sprint := range want {
		if got.Velocity[i].SprintID != sprint.ID {
			t.Errorf("row %d is sprint %d, want %d", i, got.Velocity[i].SprintID, sprint.ID)
		}
	}
}

// TestASprintStoredAtAnOlderAlgorithmVersionIsBuiltAgainRatherThanServed
// pins the third property. boardrepo.Report already answers false for such a
// row; what this proves is that the service acts on that answer instead of
// treating the sprint as one it has nothing for and leaving it out of the
// table, which would shorten a board's velocity silently every time the
// reconstruction changed.
func TestASprintStoredAtAnOlderAlgorithmVersionIsBuiltAgainRatherThanServed(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 4, 2, points(3))
	history.issues[2] = held(2, 2, 1, points(3))
	store := newStore(columns(), closedSprint(1), closedSprint(2))
	// What the previous algorithm made of sprint 1, kept under its own
	// version. Twelve is what the current one makes of the same four cards.
	store.hold(reports.Series{SprintID: 1, SprintName: "Sprint 1", Unit: reports.UnitPoints, Committed: 99, Completed: 99},
		reports.AlgoVersion-1, "2026-11-01T09:00:00Z")

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 2)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(got.Velocity) != 2 {
		t.Fatalf("velocity = %+v, want both closed sprints", got.Velocity)
	}
	if got.Velocity[0].SprintID != 1 || got.Velocity[0].Committed != 12 {
		t.Errorf("sprint 1's row = %+v, want it rebuilt at 12 rather than served at 99", got.Velocity[0])
	}
	if history.asked[1] == 0 {
		t.Error("sprint 1 was never fetched; a row an older algorithm wrote has to be built again, not served")
	}
	if !saved(store, 1) {
		t.Errorf("stored %v, want sprint 1 written again so the stale row stops being read", store.saved)
	}
}

// TestVelocityFetchesOnlyTheSprintsTheStoreDoesNotHold is the saving the
// whole arrangement exists for: six sprints of changelog is the most
// expensive thing this app does, and a board whose older sprints were built
// last week should cost one sprint's fetch, not six.
func TestVelocityFetchesOnlyTheSprintsTheStoreDoesNotHold(t *testing.T) {
	history := newHistory()
	sprints := []boardrepo.Sprint{}
	for n := 1; n <= 4; n++ {
		sprints = append(sprints, closedSprint(n))
		history.issues[n] = held(n, 2, 1, points(3))
	}
	store := newStore(columns(), sprints...)
	for n := 1; n <= 3; n++ {
		store.hold(reports.Series{SprintID: n, SprintName: closedSprint(n).Name, Unit: reports.UnitPoints, Committed: 6, Completed: 3},
			reports.AlgoVersion, "2026-11-01T09:00:00Z")
	}

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 4)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(got.Velocity) != 4 {
		t.Fatalf("velocity = %+v, want all four closed sprints", got.Velocity)
	}
	for n := 1; n <= 3; n++ {
		if history.asked[n] != 0 {
			t.Errorf("sprint %d was fetched %d time(s); its series was already stored", n, history.asked[n])
		}
	}
	if history.asked[4] == 0 {
		t.Error("sprint 4 was never fetched, and nothing had been stored for it")
	}
}

// saved reports whether the store was asked to keep that sprint's series
// during the call, in whatever order the report reached it.
func saved(store *fakeStore, sprintID int) bool {
	for _, id := range store.saved {
		if id == sprintID {
			return true
		}
	}
	return false
}
