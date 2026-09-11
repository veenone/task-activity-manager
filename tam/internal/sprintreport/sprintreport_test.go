package sprintreport_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/reports"
	"agile-suite/tam/internal/sprintdate"
	"agile-suite/tam/internal/sprintreport"
)

// The board every test here reports on, and the clock it reports at. The
// sprint dates are written the way Jira writes one, with an offset carrying
// no colon, for the reason internal/reports' own fixtures give: time.RFC3339
// rejects that shape, so a fixture written with a Z would pass against a
// parser no real instance could feed.
const (
	testBoard   = 7
	testProfile = "profile-1"
)

var afterTheSprints = time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)

// firstMonday is where the run of fortnightly sprints below starts, so each
// one opens on a Monday and closes on the second Friday.
var firstMonday = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// closedSprint is sprint n of that run, as the board cache holds it.
func closedSprint(n int) boardrepo.Sprint {
	start := firstMonday.AddDate(0, 0, 14*(n-1))
	return boardrepo.Sprint{
		ID:        n,
		BoardID:   testBoard,
		Name:      fmt.Sprintf("Sprint %d", n),
		State:     "closed",
		StartDate: sprintdate.Format(start),
		EndDate:   sprintdate.Format(start.AddDate(0, 0, 11)),
	}
}

// liveSprint is the same sprint still running.
func liveSprint(n int) boardrepo.Sprint {
	sp := closedSprint(n)
	sp.State = "active"
	return sp
}

// columns is a board whose last column collects one status, which is what
// donerule builds a rule from. A test that wants the rule to refuse passes
// none of these.
func columns() []backend.BoardColumn {
	return []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "Done", StatusIDs: []string{"10001"}},
	}
}

func points(v float64) *float64 { return &v }

// card is one issue as the cache holds it today, with no changelog: enough
// for a sprint's totals, which is what these tests read. The reconstruction
// itself is internal/reports' to test and is tested there.
func card(key, status, statusID string, sprintID int, estimate *float64) backend.IssueHistory {
	return backend.IssueHistory{
		Issue: backend.Issue{
			Key: key, Status: status, StatusID: statusID,
			SprintID: fmt.Sprint(sprintID), StoryPoints: estimate,
		},
	}
}

// held is one sprint's issues, all of them in it from the start, the first
// finished and the rest still open.
func held(sprintID, count, finished int, estimate *float64) []backend.IssueHistory {
	out := make([]backend.IssueHistory, 0, count)
	for i := 0; i < count; i++ {
		status, statusID := "To Do", "1"
		if i < finished {
			status, statusID = "Done", "10001"
		}
		out = append(out, card(fmt.Sprintf("PLAT-%d%02d", sprintID, i), status, statusID, sprintID, estimate))
	}
	return out
}

// storedRow is a stored report with the algorithm version that wrote it,
// which is what boardrepo.Report compares before serving one.
type storedRow struct {
	report  boardrepo.SavedReport
	version int
}

// fakeStore is the board cache. It keys its rows by sprint alone rather
// than by board and sprint the way boardrepo does, because every test here
// reports on one board; a test that needed two would need the real key.
type fakeStore struct {
	columns []backend.BoardColumn
	sprints []boardrepo.Sprint
	rows    map[int]storedRow
	saved   []int
	fail    error
}

func newStore(cols []backend.BoardColumn, sprints ...boardrepo.Sprint) *fakeStore {
	return &fakeStore{columns: cols, sprints: sprints, rows: map[int]storedRow{}}
}

// hold puts a report in the store as a given algorithm version wrote it, so
// a test can seed both a current row and a stale one.
//
// The series goes through JSON on the way in and out, which is what the
// real store does with it, so a test comparing a stored report against a
// freshly built one is comparing what a user would actually be served and
// not a Go value that never left the process.
func (f *fakeStore) hold(series reports.Series, version int, builtAt string) {
	blob, err := json.Marshal(series)
	if err != nil {
		panic(err)
	}
	var kept reports.Series
	if err := json.Unmarshal(blob, &kept); err != nil {
		panic(err)
	}
	f.rows[series.SprintID] = storedRow{
		report:  boardrepo.SavedReport{Series: kept, Unit: kept.Unit, BuiltAt: builtAt},
		version: version,
	}
}

func (f *fakeStore) Columns(context.Context, string, int) ([]backend.BoardColumn, error) {
	return f.columns, f.fail
}

func (f *fakeStore) ListSprints(context.Context, string, int) ([]boardrepo.Sprint, error) {
	return f.sprints, f.fail
}

func (f *fakeStore) SaveReport(_ context.Context, _ string, _ int, series reports.Series) error {
	if f.fail != nil {
		return f.fail
	}
	f.saved = append(f.saved, series.SprintID)
	f.hold(series, reports.AlgoVersion, "2026-12-01T12:00:00Z")
	return nil
}

func (f *fakeStore) Report(_ context.Context, _ string, _, sprintID int) (boardrepo.SavedReport, bool, error) {
	if f.fail != nil {
		return boardrepo.SavedReport{}, false, f.fail
	}
	row, ok := f.rows[sprintID]
	// The real store answers false for a row an older reports.AlgoVersion
	// wrote, without saying which of the two cases it was, and the service
	// is meant to act on that the same way it acts on a missing row.
	if !ok || row.version != reports.AlgoVersion {
		return boardrepo.SavedReport{}, false, nil
	}
	return row.report, true, nil
}

var _ sprintreport.Store = (*fakeStore)(nil)

// fakeHistory answers the changelog search out of a fixture, one page at a
// time, and records what it was asked for.
type fakeHistory struct {
	issues map[int][]backend.IssueHistory
	asked  map[int]int
	pages  int
	seen   int
	fail   error
}

func newHistory() *fakeHistory {
	return &fakeHistory{issues: map[int][]backend.IssueHistory{}, asked: map[int]int{}}
}

func (f *fakeHistory) SearchIssuesWithHistory(_ context.Context, jql string, startAt, maxResults int) ([]backend.IssueHistory, int, error) {
	if f.fail != nil {
		return nil, 0, f.fail
	}
	var id int
	if _, err := fmt.Sscanf(jql, "sprint = %d", &id); err != nil {
		return nil, 0, fmt.Errorf("the search asked %q, which is not the sprint query a report makes: %w", jql, err)
	}
	f.asked[id]++
	f.pages++
	f.seen = maxResults
	all := f.issues[id]
	if startAt >= len(all) {
		return []backend.IssueHistory{}, len(all), nil
	}
	end := startAt + maxResults
	if end > len(all) {
		end = len(all)
	}
	return all[startAt:end], len(all), nil
}

var _ backend.HistoryBackend = (*fakeHistory)(nil)

// service is the service under test with its clock and its zone pinned, so
// nothing here depends on the day it is run or the machine it is run on.
func service(b backend.HistoryBackend, store sprintreport.Store) *sprintreport.Service {
	s := sprintreport.New(b, store)
	s.Now = func() time.Time { return afterTheSprints }
	s.Loc = time.UTC
	return s
}

func TestABoardWhoseColumnsCannotSayWhatIsFinishedIsARefusalAndNotAnError(t *testing.T) {
	history := newHistory()
	store := newStore(nil, closedSprint(1))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1)
	if err != nil {
		t.Fatalf("Build: %v, want the reason in the report rather than an error", err)
	}
	if got.Unavailable != sprintreport.ReasonBoardNotSynced {
		t.Errorf("unavailable = %q, want %q", got.Unavailable, sprintreport.ReasonBoardNotSynced)
	}
	// Before the network, not after it: a changelog TAM cannot classify is
	// minutes of fetching spent on an answer it could not use.
	if history.pages != 0 {
		t.Errorf("the search ran %d time(s); a board with no rule for finished is refused before anything is fetched", history.pages)
	}
}

func TestASprintTheCacheDoesNotHoldIsARefusalAndNotAnError(t *testing.T) {
	store := newStore(columns(), closedSprint(1))

	got, err := service(newHistory(), store).Build(context.Background(), testProfile, testBoard, 99)
	if err != nil {
		t.Fatalf("Build: %v, want the reason in the report rather than an error", err)
	}
	if got.Unavailable != sprintreport.ReasonSprintNotFound {
		t.Errorf("unavailable = %q, want %q", got.Unavailable, sprintreport.ReasonSprintNotFound)
	}
}

func TestASprintWithNoReadableDatesIsARefusalAndNotAnError(t *testing.T) {
	undated := closedSprint(1)
	undated.StartDate = ""
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	store := newStore(columns(), undated)

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1)
	if err != nil {
		t.Fatalf("Build: %v, want the reason in the report rather than an error", err)
	}
	if got.Unavailable != sprintreport.ReasonNoDates {
		t.Errorf("unavailable = %q, want %q", got.Unavailable, sprintreport.ReasonNoDates)
	}
}

func TestABoardThatHasNeverClosedASprintHasNoReportToOpenOn(t *testing.T) {
	store := newStore(columns(), liveSprint(1))

	got, err := service(newHistory(), store).Build(context.Background(), testProfile, testBoard, 0)
	if err != nil {
		t.Fatalf("Build: %v, want the reason in the report rather than an error", err)
	}
	if got.Unavailable != sprintreport.ReasonNoClosedSprint {
		t.Errorf("unavailable = %q, want %q", got.Unavailable, sprintreport.ReasonNoClosedSprint)
	}
	if len(got.Velocity) != 0 {
		t.Errorf("velocity = %+v, want no rows: the board has closed nothing", got.Velocity)
	}
}

func TestNamingNoSprintOpensOnTheBoardsMostRecentClosedSprint(t *testing.T) {
	history := newHistory()
	for n := 1; n <= 3; n++ {
		history.issues[n] = held(n, 2, 1, points(3))
	}
	store := newStore(columns(), closedSprint(1), closedSprint(2), closedSprint(3), liveSprint(4))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Series.SprintID != 3 {
		t.Errorf("report is of sprint %d, want 3: the newest closed sprint, not the live one and not the oldest", got.Series.SprintID)
	}
}

func TestASprintOfMoreThanOnePageComesBackWhole(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 5, 2, points(3))
	store := newStore(columns(), closedSprint(1))
	svc := service(history, store)
	svc.PageSize = 2

	got, err := svc.Build(context.Background(), testProfile, testBoard, 1)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Five three-point cards, which only add up if every page was read: the
	// first page alone would commit six.
	if got.Series.Committed != 15 {
		t.Errorf("committed = %v, want 15: five three-point cards over three pages", got.Series.Committed)
	}
	if history.asked[1] < 3 {
		t.Errorf("the search ran %d time(s) for a five-issue sprint at two per page, want at least 3", history.asked[1])
	}
}

func TestAClosedSprintsSeriesIsStored(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 3, 1, points(2))
	store := newStore(columns(), closedSprint(1))

	if _, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(store.saved) != 1 || store.saved[0] != 1 {
		t.Fatalf("stored %v, want sprint 1's series kept: a closed sprint's report is what makes the view readable offline", store.saved)
	}
}

func TestALiveSprintsSeriesIsNeitherStoredNorServedFromTheStore(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 3, 1, points(2))
	store := newStore(columns(), liveSprint(1))
	// A stored row from when the sprint held twice the work. Serving it
	// would answer this morning's question with last week's numbers, and it
	// would look exactly like a correct answer.
	store.hold(reports.Series{SprintID: 1, SprintName: "Sprint 1", Unit: reports.UnitPoints, Committed: 99},
		reports.AlgoVersion, "2026-11-01T09:00:00Z")

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Series.Committed != 6 {
		t.Errorf("committed = %v, want 6 from the fetch: a live sprint is never served from the store", got.Series.Committed)
	}
	if len(store.saved) != 0 {
		t.Errorf("stored %v, want nothing: a live sprint's series is out of date the moment a card moves", store.saved)
	}
}

func TestAFailedFetchIsAnErrorAndNotAReason(t *testing.T) {
	history := newHistory()
	history.fail = errors.New("the connection was reset")
	store := newStore(columns(), closedSprint(1))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1)
	if err == nil {
		t.Fatalf("Build = %+v, want an error: a transport failure is a failure of the call and not a fact about the sprint", got)
	}
	if got.Unavailable != "" {
		t.Errorf("unavailable = %q, want empty: the view names a failed read itself and keeps its last report", got.Unavailable)
	}
}

func TestEachPageOfASprintsFetchReportsItsProgress(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 4, 1, points(2))
	store := newStore(columns(), closedSprint(1))
	svc := service(history, store)
	svc.PageSize = 2
	var frames []sprintreport.Progress
	svc.Progress = func(p sprintreport.Progress) { frames = append(frames, p) }

	if _, err := svc.Build(context.Background(), testProfile, testBoard, 1); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(frames) < 2 {
		t.Fatalf("frames = %+v, want one per page and a last one: a read this long cannot be silent", frames)
	}
	first := frames[0]
	if first.Phase != sprintreport.PhaseSprint || first.SprintID != 1 || first.SprintName != "Sprint 1" {
		t.Errorf("first frame = %+v, want the sprint being read named on the sprint phase", first)
	}
	if first.Total != 4 || first.Fetched != 2 {
		t.Errorf("first frame = %+v, want 2 of 4 after the first page of two", first)
	}
	if last := frames[len(frames)-1]; !last.Done || last.Fetched != 4 {
		t.Errorf("last frame = %+v, want the fetch marked finished with everything counted", last)
	}
}

// TestAReportAsksForSmallerPagesThanTheSyncDoes pins the page size a report
// defaults to. Twenty five is not a measurement, and the reason it is
// smaller than the sync's fifty is in the constant's own comment; what this
// test protects is that a report does not quietly inherit the sync's page
// size for a request several times heavier per issue.
func TestAReportAsksForSmallerPagesThanTheSyncDoes(t *testing.T) {
	history := newHistory()
	history.issues[1] = held(1, 2, 1, points(3))
	store := newStore(columns(), closedSprint(1))

	// New's own page size, not one this test set.
	if _, err := sprintreport.New(history, store).Build(context.Background(), testProfile, testBoard, 1); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if history.seen != 25 {
		t.Errorf("the search asked for %d issues a page, want 25", history.seen)
	}
}
