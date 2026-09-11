package boardrepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/reports"
)

func sampleSeries(sprintID int) reports.Series {
	return reports.Series{
		SprintID:   sprintID,
		SprintName: "Sprint 12",
		Unit:       reports.UnitPoints,
		Committed:  8,
		Completed:  5,
		Days: []reports.Day{
			{Date: "2026-08-18", Scope: 8, Completed: 0, Remaining: 8, Ideal: 8},
			{Date: "2026-09-01", Scope: 8, Completed: 5, Remaining: 3, Ideal: 0},
		},
		Truncated: []string{},
	}
}

func TestSaveReportAndReadItBack(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	series := sampleSeries(12)

	if err := r.SaveReport(ctx, "p1", 1, series); err != nil {
		t.Fatalf("save: %v", err)
	}
	saved, ok, err := r.Report(ctx, "p1", 1, 12)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !ok {
		t.Fatal("report not found, want the one just saved")
	}
	if saved.Unit != reports.UnitPoints {
		t.Errorf("unit = %q, want %q", saved.Unit, reports.UnitPoints)
	}
	if saved.BuiltAt == "" {
		t.Error("builtAt is empty, want a stamp of when this was saved")
	}
	if saved.Series.SprintName != "Sprint 12" || saved.Series.Committed != 8 || saved.Series.Completed != 5 {
		t.Errorf("series = %+v, want the one that was saved", saved.Series)
	}
	if len(saved.Series.Days) != 2 {
		t.Errorf("days = %+v, want both days round-tripped through series_json", saved.Series.Days)
	}
}

func TestReportIsNotFoundWhenNothingIsStored(t *testing.T) {
	r, _ := newRepo(t)
	_, ok, err := r.Report(context.Background(), "p1", 1, 12)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if ok {
		t.Error("ok = true, want false for a sprint nothing has ever built a report for")
	}
}

// TestSaveReportOverwritesTheSameKey covers the ordinary re-save: a live
// sprint's report is rebuilt and stored again under the same primary key
// rather than growing a second row.
func TestSaveReportOverwritesTheSameKey(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	first := sampleSeries(12)
	if err := r.SaveReport(ctx, "p1", 1, first); err != nil {
		t.Fatalf("save first: %v", err)
	}
	second := sampleSeries(12)
	second.Completed = 8
	if err := r.SaveReport(ctx, "p1", 1, second); err != nil {
		t.Fatalf("save second: %v", err)
	}
	saved, ok, err := r.Report(ctx, "p1", 1, 12)
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}
	if saved.Series.Completed != 8 {
		t.Errorf("completed = %v, want the second save's value, not a second row of the first", saved.Series.Completed)
	}
}

// TestReportIsScopedByBoardAsWellAsSprint pins down the whole reason this
// table's key carries board_id: Jira hands one sprint to every board whose
// filter reaches it, and the done rule a report is built from comes from
// its own board's last column, so a report saved for board 1's copy of
// sprint 12 must not answer for board 2's copy of the same sprint.
func TestReportIsScopedByBoardAsWellAsSprint(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	boardOne := sampleSeries(12)
	boardOne.Unit = reports.UnitPoints
	if err := r.SaveReport(ctx, "p1", 1, boardOne); err != nil {
		t.Fatalf("save board 1: %v", err)
	}
	if _, ok, err := r.Report(ctx, "p1", 2, 12); err != nil || ok {
		t.Fatalf("board 2's copy of sprint 12 = ok:%v err:%v, want not found", ok, err)
	}

	boardTwo := sampleSeries(12)
	boardTwo.Unit = reports.UnitCards
	if err := r.SaveReport(ctx, "p1", 2, boardTwo); err != nil {
		t.Fatalf("save board 2: %v", err)
	}
	one, ok, err := r.Report(ctx, "p1", 1, 12)
	if err != nil || !ok {
		t.Fatalf("board 1 after board 2's save: ok:%v err:%v", ok, err)
	}
	if one.Unit != reports.UnitPoints {
		t.Errorf("board 1's unit = %q after board 2's own save, want its own report left alone", one.Unit)
	}
}

// TestReportRebuildsRatherThanServesAnOlderAlgoVersion is the test the brief
// asks for directly: a row written at an older algorithm version is not
// served back as if it were current.
func TestReportRebuildsRatherThanServesAnOlderAlgoVersion(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	if err := r.SaveReport(ctx, "p1", 1, sampleSeries(12)); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Rewrite the row's own version to one below whatever reports.AlgoVersion
	// is today, the way a database saved under an earlier build of TAM would
	// already be.
	if _, err := db.Exec(`UPDATE sprint_report SET algo_version = 0 WHERE profile_id = 'p1' AND board_id = 1 AND sprint_id = 12`); err != nil {
		t.Fatalf("rewind algo_version: %v", err)
	}
	_, ok, err := r.Report(ctx, "p1", 1, 12)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if ok {
		t.Error("ok = true for a row an older algorithm version wrote, want it treated as needing a rebuild")
	}
}

func TestReportIsScopedToTheProfile(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	if err := r.SaveReport(ctx, "p1", 1, sampleSeries(12)); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok, err := r.Report(ctx, "p2", 1, 12); err != nil || ok {
		t.Fatalf("p2's report = ok:%v err:%v, want not found", ok, err)
	}
}

// TestPurgeProfileClearsTheStoredReportsToo is Step 4's test: sprint_report
// is the fifth board table, and a purge that dropped every table but this
// one would leave a stale report behind for the next profile that reuses
// this board and sprint id.
func TestPurgeProfileClearsTheStoredReportsToo(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	if err := r.SaveReport(ctx, "p1", 1, sampleSeries(12)); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := r.PurgeProfile(ctx, "p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, ok, err := r.Report(ctx, "p1", 1, 12); err != nil || ok {
		t.Fatalf("report after purge = ok:%v err:%v, want none left behind", ok, err)
	}
}

// TestRemoveBoardsClearsThatBoardsStoredReports is RemoveBoards's own half
// of step 4: deleting a board should not leave its reports behind for a
// board id Jira later reuses.
func TestRemoveBoardsClearsThatBoardsStoredReports(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	if err := r.SaveReport(ctx, "p1", 1, sampleSeries(12)); err != nil {
		t.Fatalf("save board 1: %v", err)
	}
	if err := r.SaveReport(ctx, "p1", 2, sampleSeries(12)); err != nil {
		t.Fatalf("save board 2: %v", err)
	}
	if err := r.RemoveBoards(ctx, "p1", []int{1}); err != nil {
		t.Fatalf("remove board 1: %v", err)
	}
	if _, ok, err := r.Report(ctx, "p1", 1, 12); err != nil || ok {
		t.Fatalf("removed board's report = ok:%v err:%v, want gone", ok, err)
	}
	if _, ok, err := r.Report(ctx, "p1", 2, 12); err != nil || !ok {
		t.Fatalf("board 2's report = ok:%v err:%v, want it left alone", ok, err)
	}
}
