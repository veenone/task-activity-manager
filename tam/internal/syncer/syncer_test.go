package syncer_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	corejira "agile-suite/core/jira"
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
//
// The board fields (below sinceSeen) let the same fake stand in for
// backend.BoardBackend too, scripted per board id so the composed-sync
// tests in this file and the boards pass tests in boards_test.go can share
// one backend rather than keeping two.
type fake struct {
	pages     [][]backend.Issue
	failPage  int // 1-based page index that returns failErr; 0 for none
	failErr   error
	connErr   error
	sinceSeen []string

	boards       []backend.Board
	boardsErr    error
	columns      map[int][]backend.BoardColumn
	columnsErr   map[int]error
	sprints      map[int][]backend.Sprint
	sprintsErr   map[int]error
	issueKeys    map[int]map[string][]string
	issueKeysErr map[int]map[string]error
	// keysRequested records every (boardID, sprintID) pair BoardIssueKeys
	// was asked for, so a test can assert only active and future sprints
	// had their keys fetched.
	keysRequested []boardKeyRequest
}

// boardKeyRequest is one call the boards pass made to BoardIssueKeys.
type boardKeyRequest struct {
	BoardID  int
	SprintID string
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
func (f *fake) LinkTypes(context.Context) ([]backend.LinkType, error) {
	return nil, errors.New("not used")
}
func (f *fake) CreateLink(context.Context, string, backend.LinkDraft) error {
	return errors.New("not used")
}

// Boards, BoardColumns, BoardSprints, and BoardIssueKeys make *fake satisfy
// backend.BoardBackend too, each scripted by board id so the same fake
// drives both an issues pass and a boards pass in one test.
func (f *fake) Boards(context.Context, string) ([]backend.Board, error) {
	return f.boards, f.boardsErr
}

func (f *fake) BoardColumns(_ context.Context, boardID int) ([]backend.BoardColumn, error) {
	if err := f.columnsErr[boardID]; err != nil {
		return nil, err
	}
	return f.columns[boardID], nil
}

func (f *fake) BoardSprints(_ context.Context, boardID int) ([]backend.Sprint, error) {
	if err := f.sprintsErr[boardID]; err != nil {
		return nil, err
	}
	return f.sprints[boardID], nil
}

func (f *fake) BoardIssueKeys(_ context.Context, boardID int, sprintID, _ string) ([]string, error) {
	f.keysRequested = append(f.keysRequested, boardKeyRequest{BoardID: boardID, SprintID: sprintID})
	if errs, ok := f.issueKeysErr[boardID]; ok {
		if err := errs[sprintID]; err != nil {
			return nil, err
		}
	}
	return f.issueKeys[boardID][sprintID], nil
}

// The two board writes and the two transition calls: a sync never makes
// one, but the seams carry them, so the fake has to answer.
func (f *fake) RankIssue(context.Context, string, string, bool) error {
	return errors.New("not used")
}

func (f *fake) MoveIssuesToSprint(context.Context, string, []string) error {
	return errors.New("not used")
}

func (f *fake) StartSprint(context.Context, int, backend.SprintDraft) error {
	return errors.New("not used")
}

func (f *fake) CompleteSprint(context.Context, int) error {
	return errors.New("not used")
}

func (f *fake) Transition(context.Context, string, []string) error {
	return errors.New("not used")
}

func (f *fake) CanTransition(context.Context, string, []string) (backend.TransitionCheck, error) {
	return backend.TransitionCheck{}, errors.New("not used")
}

var _ backend.BoardBackend = (*fake)(nil)

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
func (c *cancelOnSearch) LinkTypes(context.Context) ([]backend.LinkType, error) {
	return nil, errors.New("not used")
}
func (c *cancelOnSearch) CreateLink(context.Context, string, backend.LinkDraft) error {
	return errors.New("not used")
}
func (c *cancelOnSearch) Transition(context.Context, string, []string) error {
	return errors.New("not used")
}
func (c *cancelOnSearch) CanTransition(context.Context, string, []string) (backend.TransitionCheck, error) {
	return backend.TransitionCheck{}, errors.New("not used")
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

// The two lookups the forms use; neither sync nor commit calls them.
func (f *fake) SearchUsers(context.Context, string, string) ([]backend.User, error) {
	return nil, nil
}
func (f *fake) Priorities(context.Context) ([]string, error) { return nil, nil }

func (c *cancelOnSearch) SearchUsers(context.Context, string, string) ([]backend.User, error) {
	return nil, nil
}
func (c *cancelOnSearch) Priorities(context.Context) ([]string, error) { return nil, nil }

func (f *fake) SubtaskTypeName(context.Context, string) (string, error) { return "Technical task", nil }

func (c *cancelOnSearch) SubtaskTypeName(context.Context, string) (string, error) { return "", nil }

func TestSyncRunsIssuesThenBoards(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := &fake{
		pages:   [][]backend.Issue{{issue("PLAT-1", "task")}},
		boards:  []backend.Board{{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}},
		columns: map[int][]backend.BoardColumn{1: {{Name: "To Do", StatusIDs: []string{"1"}}}},
		sprints: map[int][]backend.Sprint{1: {}},
		issueKeys: map[int]map[string][]string{
			1: {"": {"PLAT-1"}},
		},
	}
	e := syncer.New(fb, repo)
	e.Boards = boards

	var frames []syncer.Progress
	sum, err := e.Sync(context.Background(), "p1", "PLAT", "", false, func(p syncer.Progress) {
		frames = append(frames, p)
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if sum.Upserted != 1 {
		t.Errorf("issues upserted = %d, want 1", sum.Upserted)
	}
	if sum.Boards == nil {
		t.Fatal("summary.Boards is nil, want the boards pass to have run")
	}
	// The frames say the order: every issue frame the pass emits lands
	// before the first board frame, and the terminal frame closes the run
	// after both passes.
	firstBoard := -1
	for i, f := range frames {
		if f.Phase == "boards" {
			firstBoard = i
			break
		}
	}
	if firstBoard < 1 {
		t.Fatalf("frames = %+v, want issue frames before the first board frame", frames)
	}
	for _, f := range frames[:firstBoard] {
		if f.Phase != "issues" || f.Done {
			t.Errorf("frame before the boards phase = %+v, want an unfinished issues frame", f)
		}
	}
	last := frames[len(frames)-1]
	if last.Phase != "issues" || !last.Done {
		t.Errorf("last frame = %+v, want the terminal issues frame", last)
	}
	if sum.Boards.Boards != 1 || sum.Boards.Unavailable {
		t.Errorf("boards summary = %+v", sum.Boards)
	}
	got, err := boards.ListBoards(context.Background(), "p1")
	if err != nil || len(got) != 1 {
		t.Fatalf("boards after a composed sync = %+v, %v", got, err)
	}
}

func TestBoardsFailureDoesNotFailTheIssueSyncAndKeepsLastSynced(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := &fake{
		pages:     [][]backend.Issue{{issue("PLAT-1", "task")}},
		boardsErr: errors.New("jira: 500 Internal Server Error"),
	}
	e := syncer.New(fb, repo)
	e.Boards = boards
	start := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	e.Now = fixedClock(start)

	sum, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil)
	if err != nil {
		t.Fatalf("sync: %v, want the boards failure not to fail the issue sync", err)
	}
	if sum.Upserted != 1 {
		t.Errorf("issues upserted = %d, want 1", sum.Upserted)
	}
	if sum.Boards == nil {
		t.Fatal("summary.Boards is nil, want the failed pass carried in the summary")
	}
	st, err := repo.SyncState(context.Background(), "p1")
	if err != nil {
		t.Fatalf("sync state: %v", err)
	}
	// The watermark is this run's own start: the issues landed, so the
	// next sync must not refetch the world because the boards failed.
	if st.LastSynced != start.Format(time.RFC3339) {
		t.Errorf("last_synced = %q, want this run's start %q", st.LastSynced, start.Format(time.RFC3339))
	}
	if st.LastError != "" {
		t.Errorf("last error = %q, want the issue sync recorded as successful", st.LastError)
	}
}

func TestErrNoAgileMarksUnavailableAndRemovesNoBoards(t *testing.T) {
	repo, boards := newBoardRepos(t)
	// Seed a board from an earlier run when the instance still had an
	// Agile API, so the test can prove ErrNoAgile leaves it alone.
	if err := boards.ReplaceBoard(context.Background(), "p1",
		backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}, nil, nil, nil); err != nil {
		t.Fatalf("seed board: %v", err)
	}

	fb := &fake{
		pages:     [][]backend.Issue{{issue("PLAT-1", "task")}},
		boardsErr: fmt.Errorf("project PLAT boards: %w", corejira.ErrNoAgile),
	}
	e := syncer.New(fb, repo)
	e.Boards = boards

	sum, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if sum.Boards == nil || !sum.Boards.Unavailable {
		t.Fatalf("boards summary = %+v, want Unavailable", sum.Boards)
	}
	got, err := boards.ListBoards(context.Background(), "p1")
	if err != nil || len(got) != 1 {
		t.Fatalf("boards after ErrNoAgile = %+v, %v, want the seeded board left alone", got, err)
	}
	v, err := repo.ProfileSetting(context.Background(), "p1", "boards_unavailable")
	if err != nil || v != "true" {
		t.Errorf("boards_unavailable = %q, %v, want it recorded", v, err)
	}
}

func TestNilBoardsFieldRunsNoBoardsPass(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{pages: [][]backend.Issue{{issue("PLAT-1", "task")}}}
	e := syncer.New(fb, repo)

	sum, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if sum.Boards != nil {
		t.Errorf("summary.Boards = %+v, want nil when the engine has no Boards repository", sum.Boards)
	}
}
