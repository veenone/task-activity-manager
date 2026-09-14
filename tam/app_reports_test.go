package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
	"agile-suite/tam/internal/sprintreport"
)

// reportBoard is the board these tests report on, seeded straight into the
// board cache rather than through a sync: a report reads its columns and its
// sprints from there, and what puts them there is not what is under test.
const reportBoard = 1

// seedReportBoard puts one board with a Done column and one closed sprint in
// the cache, which is the least a report needs before it fetches anything.
func seedReportBoard(t *testing.T, a *App, profileID string) {
	t.Helper()
	board := backend.Board{ID: reportBoard, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	cols := []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "Done", StatusIDs: []string{"10001"}},
	}
	sprints := []backend.Sprint{{
		ID: 11, BoardID: reportBoard, Name: "Sprint 11", State: "closed",
		StartDate: "2026-08-03T09:00:00.000+0000",
		EndDate:   "2026-08-14T09:00:00.000+0000",
	}}
	if err := a.boards.ReplaceBoard(context.Background(), profileID, board, cols, sprints, nil); err != nil {
		t.Fatalf("seed the board: %v", err)
	}
}

// blockingHistoryBackend answers the changelog search by blocking until the
// report's own context is cancelled, which is what a sprint of two hundred
// issues looks like to a user who has already left the view.
type blockingHistoryBackend struct {
	simpleBoardBackend
	started chan struct{}
	once    sync.Once
}

func (b *blockingHistoryBackend) SearchIssuesWithHistory(ctx context.Context, _ string, _, _ int) ([]backend.IssueHistory, int, error) {
	b.once.Do(func() { close(b.started) })
	<-ctx.Done()
	return nil, 0, ctx.Err()
}

var (
	_ backend.IssueBackend   = (*blockingHistoryBackend)(nil)
	_ backend.HistoryBackend = (*blockingHistoryBackend)(nil)
)

// blockingHistory builds one with its started channel ready, so a test that
// never reaches the search is not a test that panics closing a nil channel.
func blockingHistory() *blockingHistoryBackend {
	return &blockingHistoryBackend{
		simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do"),
		started:            make(chan struct{}),
	}
}

func TestGetSprintReportRequiresAProfile(t *testing.T) {
	a := newTestApp(t)

	if _, err := a.GetSprintReport("", reportBoard, 11, false); err == nil {
		t.Error("GetSprintReport with no profile = nil error, want a refusal")
	}
}

// TestGetSprintReportIsRefusedWhileASyncHoldsTheLock is the guard past
// requireProfile, which a valid profile never reaches on its own. A report
// is a long read on the same connection a sync uses, so it queues behind
// nothing: it is refused, and the refusal names the sync that is running.
func TestGetSprintReportIsRefusedWhileASyncHoldsTheLock(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = blockingHistory()
	seedReportBoard(t, a, p.ID)

	if err := a.acquire(p.ID, "sync"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer a.release(p.ID)

	_, err := a.GetSprintReport(p.ID, reportBoard, 11, false)
	if err == nil || !strings.Contains(err.Error(), "sync") {
		t.Errorf("GetSprintReport err = %v, want it to name the operation that is running", err)
	}
}

// TestAConnectionThatCannotReadAChangelogIsRefused covers the type
// assertion: the changelog is a capability off the backend rather than a
// method on IssueBackend, so a backend without it has to be refused here
// with a sentence instead of answering an empty report.
func TestAConnectionThatCannotReadAChangelogIsRefused(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")
	seedReportBoard(t, a, p.ID)

	_, err := a.GetSprintReport(p.ID, reportBoard, 11, false)
	if err == nil || !strings.Contains(err.Error(), "history") {
		t.Errorf("GetSprintReport err = %v, want it to say the connection cannot read an issue's history", err)
	}
}

// TestABoardThatWasNeverSyncedNamesItsReasonRatherThanFailing is the other
// half of that distinction, end to end through the real board cache: a board
// with no columns cannot say what finished means, and that is a fact about
// the data the view renders beside the board's name, not a failed call.
// Wails fills in either the value or the error and never both, so sending it
// as an error would throw the report away with it.
func TestABoardThatWasNeverSyncedNamesItsReasonRatherThanFailing(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = blockingHistory()

	got, err := a.GetSprintReport(p.ID, reportBoard, 11, false)
	if err != nil {
		t.Fatalf("GetSprintReport: %v, want the reason in the report", err)
	}
	if got.Unavailable != sprintreport.ReasonBoardNotSynced {
		t.Errorf("unavailable = %q, want %q", got.Unavailable, sprintreport.ReasonBoardNotSynced)
	}
}

// TestACancelledReportReleasesTheProfileLock is the point of the
// cancellation, and asserting the error alone would not prove it: a report
// that came back with "context canceled" while still holding the lock would
// pass that test and block the user's next sync for as long as the fetch
// would have taken.
func TestACancelledReportReleasesTheProfileLock(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	blocking := blockingHistory()
	a.backends[p.ID] = blocking
	seedReportBoard(t, a, p.ID)

	done := make(chan error, 1)
	go func() {
		_, err := a.GetSprintReport(p.ID, reportBoard, 11, false)
		done <- err
	}()

	select {
	case <-blocking.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the report never reached the changelog search")
	}

	a.CancelSprintReport(p.ID)

	select {
	case err := <-done:
		if err == nil {
			t.Error("a cancelled report = nil error, want the read to report that it stopped")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the report did not return after it was cancelled")
	}

	if err := a.acquire(p.ID, "sync"); err != nil {
		t.Fatalf("acquire after a cancelled report: %v, want the profile free again", err)
	}
	a.release(p.ID)
}

// TestCancellingWithNoReportRunningDoesNothing is the call the view makes
// every time it unmounts, including the times when the report it opened with
// has already finished.
func TestCancellingWithNoReportRunningDoesNothing(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	a.CancelSprintReport(p.ID)

	if err := a.acquire(p.ID, "sync"); err != nil {
		t.Fatalf("acquire: %v, want the profile untouched", err)
	}
	a.release(p.ID)
}

// quietHistoryBackend answers the changelog search with nothing at all,
// which is what a sprint whose issues were every one of them deleted looks
// like, and is enough to take a report all the way through a build without
// a fixture.
type quietHistoryBackend struct {
	simpleBoardBackend
}

func (b *quietHistoryBackend) SearchIssuesWithHistory(context.Context, string, int, int) ([]backend.IssueHistory, int, error) {
	return nil, 0, nil
}

var (
	_ backend.IssueBackend   = (*quietHistoryBackend)(nil)
	_ backend.HistoryBackend = (*quietHistoryBackend)(nil)
)

func quietHistory() *quietHistoryBackend {
	return &quietHistoryBackend{simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")}
}

// TestAReportRunsWithNoWailsRuntimeBehindIt pins the two halves of this
// file agreeing about a nil a.ctx. It is nil until Wails calls startup, and
// it is nil in a test; emitReportProgress has always dropped its frame for
// that, and context.WithCancel panics on a nil parent rather than returning
// an error, so the report has to be given a parent of its own.
func TestAReportRunsWithNoWailsRuntimeBehindIt(t *testing.T) {
	a := newTestApp(t)
	a.ctx = nil
	p := newTestProfile(t, a)
	a.backends[p.ID] = quietHistory()
	seedReportBoard(t, a, p.ID)

	got, err := a.GetSprintReport(p.ID, reportBoard, 11, false)
	if err != nil {
		t.Fatalf("GetSprintReport: %v", err)
	}
	if got.Unavailable != "" {
		t.Errorf("unavailable = %q, want a report: the sprint is closed, dated and in the cache", got.Unavailable)
	}
	if got.Series.SprintID != 11 {
		t.Errorf("the report is of sprint %d, want 11", got.Series.SprintID)
	}
}

// TestRefreshReachesTheServiceFromTheBinding. The two directions are the
// service's own to test and are tested there; what this pins is that the
// argument travels, because a binding that dropped it would leave the
// control Task 5 builds doing nothing at all and every test below it would
// still pass.
func TestRefreshReachesTheServiceFromTheBinding(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = quietHistory()
	seedReportBoard(t, a, p.ID)
	// A stored report saying something the backend above never would.
	stored := reports.Series{SprintID: 11, SprintName: "Sprint 11", Unit: reports.UnitPoints, Committed: 99}
	if err := a.boards.SaveReport(context.Background(), p.ID, reportBoard, stored); err != nil {
		t.Fatalf("seed the stored report: %v", err)
	}

	served, err := a.GetSprintReport(p.ID, reportBoard, 11, false)
	if err != nil {
		t.Fatalf("GetSprintReport: %v", err)
	}
	if served.Series.Committed != 99 {
		t.Errorf("committed = %v without a refresh, want 99 from the store", served.Series.Committed)
	}

	rebuilt, err := a.GetSprintReport(p.ID, reportBoard, 11, true)
	if err != nil {
		t.Fatalf("GetSprintReport with a refresh: %v", err)
	}
	if rebuilt.Series.Committed != 0 {
		t.Errorf("committed = %v with a refresh, want 0 from the fetch: the stored row is the thing being replaced", rebuilt.Series.Committed)
	}
}
