package syncer_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/syncer"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *issuerepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return issuerepo.New(db.DB())
}

// fake is a scripted IssueBackend: fixed pages, an optional page that
// fails, and a record of the since values it was asked for.
type fake struct {
	pages     [][]backend.Issue
	failPage  int // 1-based page index that returns failErr; 0 for none
	failErr   error
	connErr   error
	sinceSeen []string
}

func (f *fake) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "fake"}, f.connErr
}
func (f *fake) IsDemo() bool { return false }
func (f *fake) SearchIssuesPage(_ context.Context, _, _, since string, _ []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	f.sinceSeen = append(f.sinceSeen, since)
	total := 0
	for _, p := range f.pages {
		total += len(p)
	}
	idx := startAt / maxResults
	if f.failPage > 0 && idx+1 == f.failPage {
		return nil, 0, f.failErr
	}
	if idx >= len(f.pages) {
		return []backend.Issue{}, total, nil
	}
	return f.pages[idx], total, nil
}
func (f *fake) GetIssueDetail(context.Context, string) (backend.IssueDetail, error) {
	return backend.IssueDetail{}, errors.New("not used")
}
func (f *fake) IssueTypes(context.Context, string) ([]backend.IssueType, error) { return nil, nil }
func (f *fake) GetIssue(context.Context, string) (backend.Issue, error) {
	return backend.Issue{}, errors.New("not used")
}
func (f *fake) UpdateIssue(context.Context, string, map[string]string) error {
	return errors.New("not used")
}
func (f *fake) CreateIssue(context.Context, string, backend.IssueDraft) (string, error) {
	return "", errors.New("not used")
}
func (f *fake) CreateFields(context.Context, string, string) ([]backend.FieldSpec, error) {
	return nil, errors.New("not used")
}

func issue(key, typ string) backend.Issue {
	return backend.Issue{Key: key, ID: key, Project: "PLAT", Type: typ, Summary: key, Status: "To Do", Rank: key, Updated: "2026-09-01T00:00:00Z"}
}

func fixedClock(ts ...time.Time) func() time.Time {
	i := 0
	return func() time.Time {
		t := ts[i]
		if i < len(ts)-1 {
			i++
		}
		return t
	}
}

func TestSyncPagesEverythingAndRecordsState(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{pages: [][]backend.Issue{
		{issue("PLAT-1", "task"), issue("PLAT-2", "story")},
		{issue("PLAT-3", "bug"), issue("PLAT-4", "")},
	}}
	e := syncer.New(fb, repo)
	e.PageSize = 2
	start := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	e.Now = fixedClock(start, start.Add(3*time.Second))

	var events []syncer.Progress
	sum, err := e.Sync(context.Background(), "p1", "PLAT", "", false, func(p syncer.Progress) { events = append(events, p) })
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if sum.Fetched != 4 || sum.Upserted != 3 || sum.Skipped != 1 || sum.Full || sum.Elapsed != "3s" {
		t.Errorf("summary = %+v", sum)
	}
	n, _ := repo.CountIssues(context.Background(), "p1")
	if n != 3 {
		t.Errorf("cached = %d, want 3 (the untyped issue is skipped)", n)
	}
	st, _ := repo.SyncState(context.Background(), "p1")
	if st.LastSynced != "2026-09-05T10:42:00Z" || st.LastFull != "" || st.LastError != "" {
		t.Errorf("state = %+v", st)
	}
	if len(fb.sinceSeen) == 0 || fb.sinceSeen[0] != "" {
		t.Errorf("first sync must not send a since: %v", fb.sinceSeen)
	}
	last := events[len(events)-1]
	if !last.Done || last.Fetched != 4 || last.Total != 4 {
		t.Errorf("last event = %+v", last)
	}
	if events[0].Stage == "" {
		t.Error("first event should name a stage")
	}
}

func TestIncrementalSendsLastSyncedAndFullClearsAndSendsNothing(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{pages: [][]backend.Issue{{issue("PLAT-1", "task")}}}
	e := syncer.New(fb, repo)
	e.PageSize = 10
	first := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	e.Now = fixedClock(first)
	if _, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil); err != nil {
		t.Fatal(err)
	}
	second := first.Add(time.Hour)
	e.Now = fixedClock(second)
	fb.pages = [][]backend.Issue{{issue("PLAT-2", "task")}}
	if _, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil); err != nil {
		t.Fatal(err)
	}
	if got := fb.sinceSeen[len(fb.sinceSeen)-1]; got != "2026-09-05T10:00:00Z" {
		t.Errorf("incremental since = %q", got)
	}
	n, _ := repo.CountIssues(context.Background(), "p1")
	if n != 2 {
		t.Errorf("after incremental: %d rows, want 2 (PLAT-1 kept)", n)
	}
	e.Now = fixedClock(second.Add(time.Hour))
	sum, err := e.Sync(context.Background(), "p1", "PLAT", "", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := fb.sinceSeen[len(fb.sinceSeen)-1]; got != "" {
		t.Errorf("full sync since = %q, want empty", got)
	}
	if !sum.Full {
		t.Error("summary should say full")
	}
	n, _ = repo.CountIssues(context.Background(), "p1")
	if n != 1 {
		t.Errorf("after full: %d rows, want 1 (PLAT-1 cleared)", n)
	}
	st, _ := repo.SyncState(context.Background(), "p1")
	if st.LastFull != "2026-09-05T12:00:00Z" || st.LastSynced != st.LastFull {
		t.Errorf("state after full = %+v", st)
	}
}

func TestPageFailureKeepsWhatLandedAndDoesNotAdvanceState(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{
		pages:    [][]backend.Issue{{issue("PLAT-1", "task")}, {issue("PLAT-2", "task")}},
		failPage: 2,
		failErr:  errors.New("jira: 502 Bad Gateway"),
	}
	e := syncer.New(fb, repo)
	e.PageSize = 1
	var done bool
	_, err := e.Sync(context.Background(), "p1", "PLAT", "", false, func(p syncer.Progress) { done = done || p.Done })
	var pse *syncer.PartialSyncError
	if !errors.As(err, &pse) || pse.Pages != 1 || !errors.Is(err, fb.failErr) {
		t.Fatalf("err = %v, want a PartialSyncError after 1 page wrapping the cause", err)
	}
	if !done {
		t.Error("a failed sync still sends the terminal progress event")
	}
	n, _ := repo.CountIssues(context.Background(), "p1")
	if n != 1 {
		t.Errorf("rows = %d, want the first page kept", n)
	}
	st, _ := repo.SyncState(context.Background(), "p1")
	if st.LastSynced != "" || st.LastError != "jira: 502 Bad Gateway" {
		t.Errorf("state = %+v: last_synced must stay empty, last_error must be set", st)
	}
}

func TestConnectionFailureStopsBeforeAnyPage(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{pages: [][]backend.Issue{{issue("PLAT-1", "task")}}, connErr: errors.New("401 Unauthorized")}
	e := syncer.New(fb, repo)
	_, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil)
	if err == nil || errors.As(err, new(*syncer.PartialSyncError)) {
		t.Fatalf("err = %v, want a plain error, not a partial sync", err)
	}
	if len(fb.sinceSeen) != 0 {
		t.Error("no page must be requested after a failed connection test")
	}
	st, _ := repo.SyncState(context.Background(), "p1")
	if st.LastError != "401 Unauthorized" {
		t.Errorf("last error = %q", st.LastError)
	}
}

func TestSyncAgainstTheDemoBackend(t *testing.T) {
	repo := newRepo(t)
	e := syncer.New(demobackend.New("DEMO"), repo)
	sum, err := e.Sync(context.Background(), "p1", "DEMO", "", false, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if sum.Fetched != 60 || sum.Upserted != 60 {
		t.Errorf("summary = %+v", sum)
	}
	page, err := repo.ListIssues(context.Background(), "p1", issuerepo.IssueQuery{Types: []string{"epic"}})
	if err != nil || page.Total != 4 {
		t.Errorf("epics after sync = %d, %v", page.Total, err)
	}
}

// cancelOnSearch is a backend whose connection test passes and whose first
// page cancels the caller's context, the way a user pressing Cancel or a
// closing window does mid-sync.
type cancelOnSearch struct {
	cancel context.CancelFunc
}

func (c *cancelOnSearch) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "fake"}, nil
}
func (c *cancelOnSearch) IsDemo() bool { return false }
func (c *cancelOnSearch) SearchIssuesPage(ctx context.Context, _, _, _ string, _ []string, _, _ int) ([]backend.Issue, int, error) {
	c.cancel()
	return nil, 0, ctx.Err()
}
func (c *cancelOnSearch) GetIssueDetail(context.Context, string) (backend.IssueDetail, error) {
	return backend.IssueDetail{}, errors.New("not used")
}
func (c *cancelOnSearch) IssueTypes(context.Context, string) ([]backend.IssueType, error) {
	return nil, nil
}
func (c *cancelOnSearch) GetIssue(context.Context, string) (backend.Issue, error) {
	return backend.Issue{}, errors.New("not used")
}
func (c *cancelOnSearch) UpdateIssue(context.Context, string, map[string]string) error {
	return errors.New("not used")
}
func (c *cancelOnSearch) CreateIssue(context.Context, string, backend.IssueDraft) (string, error) {
	return "", errors.New("not used")
}
func (c *cancelOnSearch) CreateFields(context.Context, string, string) ([]backend.FieldSpec, error) {
	return nil, errors.New("not used")
}

func TestCancelledSyncStillRecordsLastError(t *testing.T) {
	repo := newRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := syncer.New(&cancelOnSearch{cancel: cancel}, repo)

	_, err := e.Sync(ctx, "p1", "PLAT", "", false, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want it to wrap context.Canceled", err)
	}
	st, stErr := repo.SyncState(context.Background(), "p1")
	if stErr != nil {
		t.Fatalf("read state: %v", stErr)
	}
	if st.LastError == "" {
		t.Error("a cancelled sync must still leave last_error for the status bar")
	}
	if st.LastSynced != "" {
		t.Errorf("last_synced = %q, want it left alone", st.LastSynced)
	}
}

func TestFullSyncFailingOnItsFirstPageKeepsThePreviousRows(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	fb := &fake{pages: [][]backend.Issue{{issue("PLAT-1", "task")}}}
	e := syncer.New(fb, repo)
	e.PageSize = 10
	first := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	e.Now = fixedClock(first)
	if _, err := e.Sync(ctx, "p1", "PLAT", "", true, nil); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	before, _ := repo.SyncState(ctx, "p1")
	if before.LastFull == "" || before.IssueCount != 1 {
		t.Fatalf("seed state = %+v", before)
	}

	fb.failPage = 1
	fb.failErr = errors.New("jira: 502 Bad Gateway")
	e.Now = fixedClock(first.Add(time.Hour))
	_, err := e.Sync(ctx, "p1", "PLAT", "", true, nil)
	if err == nil || errors.As(err, new(*syncer.PartialSyncError)) {
		t.Fatalf("err = %v, want a plain failure before any page landed", err)
	}
	after, _ := repo.SyncState(ctx, "p1")
	if after.IssueCount != 1 {
		t.Errorf("rows = %d, want the previous data kept when the full sync never got a page", after.IssueCount)
	}
	if after.LastFull != before.LastFull {
		t.Errorf("last_full = %q, want %q", after.LastFull, before.LastFull)
	}
	if after.LastError != "jira: 502 Bad Gateway" {
		t.Errorf("last error = %q", after.LastError)
	}
}
