package sprintreport_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/reports"
	"agile-suite/tam/internal/sprintreport"
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

// rowFor is one sprint's row out of a table, so a test can name the sprint
// it means rather than count positions in a slice.
func rowFor(t *testing.T, rows []reports.VelocityRow, sprintID int) reports.VelocityRow {
	t.Helper()
	for _, row := range rows {
		if row.SprintID == sprintID {
			return row
		}
	}
	t.Fatalf("no row for sprint %d in %+v", sprintID, rows)
	return reports.VelocityRow{}
}

// TestACachedSprintAndAFreshlyBuiltSprintGiveTheIdenticalRow builds one
// sprint both ways and compares its row: reconstructed from a fetch the
// first time, read back out of the store the second. A row assembled in two
// places would drift the first time either place changed, and the drift
// would show up as a velocity table disagreeing with the report printed
// above it.
//
// The sprint compared is deliberately not the sprint both reports are
// about. The table has two branches, one for the series the report already
// has in hand and one for every other row, and a test comparing the
// report's own sprint against itself runs the first branch twice and proves
// nothing: a row built differently inside that branch moves both sides of
// the comparison together. Sprint 1 is the report's own sprint in the first
// call and an ordinary older row in the second, so each branch builds it
// once.
func TestACachedSprintAndAFreshlyBuiltSprintGiveTheIdenticalRow(t *testing.T) {
	history := newHistory()
	// Unestimated, so the row carries a unit reason, and one issue whose
	// changelog came back cut short, so it carries a truncation marker too.
	// A row is more than its two numbers, and a branch that dropped the
	// rest of it would still pass a comparison of the numbers alone.
	history.issues[1] = held(1, 4, 2, nil)
	history.issues[1][0].Truncated = true
	history.issues[2] = held(2, 2, 1, points(3))
	store := newStore(columns(), closedSprint(1), closedSprint(2))

	fresh, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1, false)
	if err != nil {
		t.Fatalf("the first report: %v", err)
	}
	built := rowFor(t, fresh.Velocity, 1)
	if built.UnitReason == "" || !built.Truncated {
		t.Fatalf("sprint 1's row = %+v, want a unit reason and a truncation marker on it; without both this test compares two numbers", built)
	}

	// A backend that refuses everything, so the second report's rows can
	// only have come out of the store the first one filled.
	refusing := newHistory()
	refusing.fail = errors.New("this sprint should have been read from the store")
	cached, err := service(refusing, store).Build(context.Background(), testProfile, testBoard, 2, false)
	if err != nil {
		t.Fatalf("the second report: %v", err)
	}
	if served := rowFor(t, cached.Velocity, 1); !reflect.DeepEqual(built, served) {
		t.Errorf("sprint 1 read back out of the store is %+v and freshly built it is %+v; one sprint has one row however its series arrived",
			served, built)
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

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 9, false)
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

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 2, false)
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

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 4, false)
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

// TestTheSprintTheReportIsAboutIsNotReadTwice pins the short circuit that
// hands the velocity table the series the report already has. Without it the
// sprint on screen is read a second time for its own row in the table below,
// which on a refresh is a second fetch of the most expensive read in the
// app. A refresh is what makes that visible: without one the second read
// comes back out of the store and costs only a query.
func TestTheSprintTheReportIsAboutIsNotReadTwice(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	history.issues[2] = held(2, 2, 1, points(3))
	store := newStore(columns(), closedSprint(1), closedSprint(2))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 2, true)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(got.Velocity) != 2 {
		t.Fatalf("velocity = %+v, want both closed sprints", got.Velocity)
	}
	if history.asked[2] != 1 {
		t.Errorf("sprint 2 was fetched %d time(s), want 1: the report's own sprint is reconstructed once and its row reuses that series", history.asked[2])
	}
}

// TestASprintWhoseDatesCannotBeReadIsNeverFetchedForTheTable is the waste
// the selection rule exists to stop. A sprint TAM cannot reconstruct is
// dropped before anything is asked for, not after several pages of changelog
// have come back for it.
func TestASprintWhoseDatesCannotBeReadIsNeverFetchedForTheTable(t *testing.T) {
	unreadable := closedSprint(1)
	// A readable end and an unreadable start, which is the pair a check on
	// the end date alone lets through.
	unreadable.StartDate = "the fifth of January"
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	history.issues[2] = held(2, 2, 1, points(3))
	store := newStore(columns(), unreadable, closedSprint(2))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 2, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(got.Velocity) != 1 || got.Velocity[0].SprintID != 2 {
		t.Fatalf("velocity = %+v, want only sprint 2: a sprint with no readable start cannot be reconstructed", got.Velocity)
	}
	if history.asked[1] != 0 {
		t.Errorf("sprint 1 was fetched %d time(s), want none: that is pages of changelog for a series Build would refuse", history.asked[1])
	}
}

// TestASprintThatEndsBeforeItStartsIsDroppedRatherThanFailingTheTable
// covers the one unreportable sprint the selection cannot see. Both its
// dates parse, so it is picked, fetched and handed to reports.Build, which
// refuses it with ErrNoDates. Dropping that row is the whole of the answer:
// failing the call would take five good rows away to report one bad one.
func TestASprintThatEndsBeforeItStartsIsDroppedRatherThanFailingTheTable(t *testing.T) {
	backwards := closedSprint(1)
	backwards.StartDate, backwards.EndDate = backwards.EndDate, backwards.StartDate
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	history.issues[2] = held(2, 2, 1, points(3))
	store := newStore(columns(), backwards, closedSprint(2))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 2, false)
	if err != nil {
		t.Fatalf("Build: %v, want the one bad sprint dropped and the rest of the table kept", err)
	}
	if len(got.Velocity) != 1 || got.Velocity[0].SprintID != 2 {
		t.Fatalf("velocity = %+v, want only sprint 2", got.Velocity)
	}
	if history.asked[1] == 0 {
		t.Error("sprint 1 was never fetched, so this test is not exercising the drop it was written for")
	}
}

// TestTheOlderSprintsOfTheTableReportTheirOwnProgress pins the velocity
// phase. A report covering six sprints spends most of its minutes on the
// five the user did not ask for, and a frame that named only the sprint on
// screen would leave the view claiming one sprint's fetch for the whole
// wait.
func TestTheOlderSprintsOfTheTableReportTheirOwnProgress(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	history.issues[2] = held(2, 2, 1, points(3))
	store := newStore(columns(), closedSprint(1), closedSprint(2))
	svc := service(history, store)
	var frames []sprintreport.Progress
	svc.Progress = func(p sprintreport.Progress) { frames = append(frames, p) }

	if _, err := svc.Build(context.Background(), testProfile, testBoard, 2, false); err != nil {
		t.Fatalf("Build: %v", err)
	}
	var older, asked bool
	for _, f := range frames {
		switch f.Phase {
		case sprintreport.PhaseVelocity:
			if f.SprintID == 1 && f.SprintName == "Sprint 1" {
				older = true
			}
		case sprintreport.PhaseSprint:
			if f.SprintID == 2 {
				asked = true
			}
		}
	}
	if !asked {
		t.Errorf("frames = %+v, want the sprint that was asked for reported on the sprint phase", frames)
	}
	if !older {
		t.Errorf("frames = %+v, want sprint 1 named on the velocity phase: the table's own fetches are most of the wait", frames)
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
