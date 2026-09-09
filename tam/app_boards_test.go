package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agile-suite/core/profile"
	"agile-suite/core/shareddb"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/suiteprofiles"
	"agile-suite/tam/internal/tamstore"
)

// memCredentialStore is an in-memory stand-in for profile.CredentialStore so
// these tests never touch the real OS credential manager.
type memCredentialStore struct {
	mu   sync.Mutex
	data map[string]string
}

func newMemCredentialStore() *memCredentialStore {
	return &memCredentialStore{data: map[string]string{}}
}

func (m *memCredentialStore) Save(id, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[id] = secret
	return nil
}

func (m *memCredentialStore) Load(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[id], nil
}

func (m *memCredentialStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id)
	return nil
}

var _ profile.CredentialStore = (*memCredentialStore)(nil)

// newTestApp builds a fully-wired App against temp-dir SQLite stores, with
// an in-memory credential store, for exercising App methods directly
// without Wails or startup. Mirrors initStore's wiring.
func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	local, err := tamstore.Open(filepath.Join(dir, "tam.db"))
	if err != nil {
		t.Fatalf("open local store: %v", err)
	}
	t.Cleanup(func() { _ = local.Close() })
	shared, err := shareddb.Open(filepath.Join(dir, "profiles.db"))
	if err != nil {
		t.Fatalf("open shared store: %v", err)
	}
	t.Cleanup(func() { _ = shared.Close() })

	a := &App{}
	a.ctx = context.Background()
	a.local = local
	a.shared = shared
	a.repo = issuerepo.New(local.DB())
	a.boards = boardrepo.New(local.DB())
	a.backends = map[string]backend.IssueBackend{}
	a.busy = map[string]string{}
	a.profiles = profile.NewManager(shared.DB())
	a.creds = newMemCredentialStore()
	return a
}

// newTestProfile creates a plain Jira profile, non-demo, so a test that
// wants to exercise SyncBoards without a real Jira instance has to inject
// its own backend into a.backends first.
func newTestProfile(t *testing.T, a *App) profile.Profile {
	t.Helper()
	p, err := a.profiles.Create("Test", "https://jira.example.com", "PLAT", "", "", "", "", "", false, suiteprofiles.Backend)
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return p
}

// stubIssueBackend implements backend.IssueBackend with "not used" stubs
// for every method the boards pass never calls, so a board-only fake does
// not have to spell out eleven methods it will never exercise.
type stubIssueBackend struct{}

func (stubIssueBackend) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "fake"}, nil
}
func (stubIssueBackend) IsDemo() bool { return false }
func (stubIssueBackend) SearchIssuesPage(context.Context, string, string, string, []string, int, int) ([]backend.Issue, int, error) {
	return nil, 0, errors.New("not used")
}
func (stubIssueBackend) GetIssueDetail(context.Context, string) (backend.IssueDetail, error) {
	return backend.IssueDetail{}, errors.New("not used")
}
func (stubIssueBackend) IssueTypes(context.Context, string) ([]backend.IssueType, error) {
	return nil, nil
}
func (stubIssueBackend) GetIssue(context.Context, string) (backend.Issue, error) {
	return backend.Issue{}, errors.New("not used")
}
func (stubIssueBackend) UpdateIssue(context.Context, string, map[string]string) error {
	return errors.New("not used")
}
func (stubIssueBackend) CreateIssue(context.Context, string, backend.IssueDraft) (string, error) {
	return "", errors.New("not used")
}
func (stubIssueBackend) CreateFields(context.Context, string, string) ([]backend.FieldSpec, error) {
	return nil, errors.New("not used")
}
func (stubIssueBackend) LinkTypes(context.Context) ([]backend.LinkType, error) {
	return nil, errors.New("not used")
}
func (stubIssueBackend) CreateLink(context.Context, string, backend.LinkDraft) error {
	return errors.New("not used")
}
func (stubIssueBackend) Transition(context.Context, string, []string) error {
	return errors.New("not used")
}

// The two board writes live on the issue stub rather than on each board
// fake, so all three of them inherit a refusal: a boards sync never writes,
// and a test that made one would rather see this than a silent success.
func (stubIssueBackend) RankIssue(context.Context, string, string, bool) error {
	return errors.New("not used")
}
func (stubIssueBackend) MoveIssuesToSprint(context.Context, string, []string) error {
	return errors.New("not used")
}
func (stubIssueBackend) StartSprint(context.Context, int, backend.SprintDraft) error {
	return errors.New("not used")
}
func (stubIssueBackend) CompleteSprint(context.Context, int) error {
	return errors.New("not used")
}
func (stubIssueBackend) CanTransition(context.Context, string, []string) (backend.TransitionCheck, error) {
	return backend.TransitionCheck{}, errors.New("not used")
}

// The three lookups the forms use. A board test never reaches them, but
// IssueBackend carries them, so the stub has to answer.
func (stubIssueBackend) SubtaskTypeName(context.Context, string) (string, error) {
	return "", errors.New("not used")
}
func (stubIssueBackend) SearchUsers(context.Context, string, string) ([]backend.User, error) {
	return nil, errors.New("not used")
}
func (stubIssueBackend) Priorities(context.Context) ([]string, error) {
	return nil, errors.New("not used")
}

// simpleBoardBackend answers the board calls straight away, for seeding a
// previous good copy before a test swaps in a slower backend. Its columns
// are per board id, since the boards it answers with are too.
type simpleBoardBackend struct {
	stubIssueBackend
	boards  []backend.Board
	columns map[int][]backend.BoardColumn
}

func (b *simpleBoardBackend) Boards(context.Context, string) ([]backend.Board, error) {
	return b.boards, nil
}
func (b *simpleBoardBackend) BoardColumns(_ context.Context, boardID int) ([]backend.BoardColumn, error) {
	return b.columns[boardID], nil
}
func (b *simpleBoardBackend) BoardSprints(context.Context, int) ([]backend.Sprint, error) {
	return []backend.Sprint{}, nil
}
func (b *simpleBoardBackend) BoardIssueKeys(context.Context, int, string, string) ([]string, error) {
	return []string{}, nil
}

var (
	_ backend.IssueBackend = (*simpleBoardBackend)(nil)
	_ backend.BoardBackend = (*simpleBoardBackend)(nil)
)

// blockingBoardBackend answers Boards straight away but blocks inside
// BoardColumns for the board named by blockOn until the test closes
// proceed. With blockOn set to the second board, the sync is caught
// between two boards' writes: one has landed, the other has not, and a
// test can prove a reader sees each of them whole.
type blockingBoardBackend struct {
	stubIssueBackend
	boards  []backend.Board
	columns map[int][]backend.BoardColumn
	blockOn int
	started chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (b *blockingBoardBackend) Boards(context.Context, string) ([]backend.Board, error) {
	return b.boards, nil
}
func (b *blockingBoardBackend) BoardColumns(_ context.Context, boardID int) ([]backend.BoardColumn, error) {
	if boardID == b.blockOn {
		b.once.Do(func() { close(b.started) })
		<-b.proceed
	}
	return b.columns[boardID], nil
}
func (b *blockingBoardBackend) BoardSprints(context.Context, int) ([]backend.Sprint, error) {
	return []backend.Sprint{}, nil
}
func (b *blockingBoardBackend) BoardIssueKeys(context.Context, int, string, string) ([]string, error) {
	return []string{}, nil
}

var (
	_ backend.IssueBackend = (*blockingBoardBackend)(nil)
	_ backend.BoardBackend = (*blockingBoardBackend)(nil)
)

// issueAndBoardBackend answers one page of issues as well as the board
// calls, so a test can run the whole SyncIssues path: the issue pass, then
// the boards pass the engine only runs when app_issues.go hands it the
// board repository.
type issueAndBoardBackend struct {
	simpleBoardBackend
	issues []backend.Issue
}

func (b *issueAndBoardBackend) SearchIssuesPage(_ context.Context, _, _, _ string, _ []string, startAt, _ int) ([]backend.Issue, int, error) {
	if startAt > 0 {
		return []backend.Issue{}, len(b.issues), nil
	}
	return b.issues, len(b.issues), nil
}

var (
	_ backend.IssueBackend = (*issueAndBoardBackend)(nil)
	_ backend.BoardBackend = (*issueAndBoardBackend)(nil)
)

// twoBoards is the pair every test here syncs: a scrum board and a kanban
// board, each with one column.
func twoBoards(scrumName, kanbanName, column string) *simpleBoardBackend {
	return &simpleBoardBackend{
		boards: []backend.Board{
			{ID: 1, Name: scrumName, Type: backend.BoardTypeScrum},
			{ID: 2, Name: kanbanName, Type: backend.BoardTypeKanban},
		},
		columns: map[int][]backend.BoardColumn{
			1: {{Name: column, StatusIDs: []string{"1"}}},
			2: {{Name: column, StatusIDs: []string{"1"}}},
		},
	}
}

// TestSyncBoardsRefusesWhileASyncHoldsTheBusyGuard verifies SyncBoards is
// refused, not queued, while a.busy already names another operation for
// the same profile, the same rule SyncIssues and CommitPendingChanges obey.
// The backend is injected first and the free path run for real, so the
// refusal is the guard talking and not a call the profile's URL could
// never have completed.
func TestSyncBoardsRefusesWhileASyncHoldsTheBusyGuard(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")

	if _, err := a.SyncBoards(p.ID); err != nil {
		t.Fatalf("sync boards with the guard free: %v, want it to succeed", err)
	}

	if err := a.acquire(p.ID, "sync"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer a.release(p.ID)

	_, err := a.SyncBoards(p.ID)
	if err == nil {
		t.Fatal("SyncBoards while a sync is running = nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("err = %v, want the busy guard's own refusal", err)
	}
}

// TestBoardReadsDuringASyncSeeEachBoardWhole catches the sync between two
// boards: the first has been written, the second has not. A reader must
// see the first entirely new, its name and its columns together, and the
// second entirely as the previous run left it. A board whose name had
// landed ahead of its columns, which the four separate transactions
// allowed, would show up here.
func TestBoardReadsDuringASyncSeeEachBoardWhole(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	a.backends[p.ID] = twoBoards("Old Scrum", "Old Kanban", "To Do")
	if _, err := a.SyncBoards(p.ID); err != nil {
		t.Fatalf("seed sync: %v", err)
	}

	seed := twoBoards("New Scrum", "New Kanban", "Done")
	blocking := &blockingBoardBackend{
		boards:  seed.boards,
		columns: seed.columns,
		blockOn: 2,
		started: make(chan struct{}),
		proceed: make(chan struct{}),
	}
	a.backends[p.ID] = blocking

	done := make(chan error, 1)
	go func() {
		_, err := a.SyncBoards(p.ID)
		done <- err
	}()
	<-blocking.started

	boards, err := a.ListBoards(p.ID)
	if err != nil {
		t.Fatalf("list boards mid-sync: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("boards mid-sync = %+v, want both", boards)
	}
	byID := map[int]boardrepo.Board{}
	for _, b := range boards {
		byID[b.ID] = b
	}
	if byID[1].Name != "New Scrum" {
		t.Errorf("board 1 mid-sync = %q, want the copy this run already wrote", byID[1].Name)
	}
	if byID[2].Name != "Old Kanban" {
		t.Errorf("board 2 mid-sync = %q, want the previous copy: its write has not run", byID[2].Name)
	}
	for _, want := range []struct {
		id     int
		column string
	}{{1, "Done"}, {2, "To Do"}} {
		view, err := a.GetBoard(p.ID, want.id, "", "")
		if err != nil {
			t.Fatalf("get board %d mid-sync: %v", want.id, err)
		}
		if len(view.Columns) != 1 || view.Columns[0].Name != want.column {
			t.Errorf("board %d columns mid-sync = %+v, want %q, matching the name the same read saw", want.id, view.Columns, want.column)
		}
	}

	close(blocking.proceed)
	if err := <-done; err != nil {
		t.Fatalf("sync: %v", err)
	}

	boards, err = a.ListBoards(p.ID)
	if err != nil {
		t.Fatalf("list boards after sync: %v", err)
	}
	if len(boards) != 2 || boards[0].Name != "New Kanban" || boards[1].Name != "New Scrum" {
		t.Fatalf("boards after sync = %+v, want both new copies", boards)
	}
	view, err := a.GetBoard(p.ID, 2, "", "")
	if err != nil {
		t.Fatalf("get board 2 after sync: %v", err)
	}
	if len(view.Columns) != 1 || view.Columns[0].Name != "Done" {
		t.Errorf("board 2 columns after sync = %+v, want the new copy", view.Columns)
	}
}

// TestSyncIssuesRunsTheBoardsPass is the test behind one line: app_issues.go
// hands the engine the board repository before it calls Sync. Without it
// the boards pass compiles, its own tests pass, and no real sync ever runs
// it, so the summary this asserts on is the only place the wiring shows.
func TestSyncIssuesRunsTheBoardsPass(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	b := twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")
	a.backends[p.ID] = &issueAndBoardBackend{
		simpleBoardBackend: *b,
		issues: []backend.Issue{{
			Key: "PLAT-1", ID: "PLAT-1", Project: "PLAT", Type: "Task",
			Summary: "A task", Status: "To Do", Rank: "a", Updated: "2026-09-01T00:00:00Z",
		}},
	}

	sum, err := a.SyncIssues(p.ID, true)
	if err != nil {
		t.Fatalf("sync issues: %v", err)
	}
	if sum.Upserted != 1 {
		t.Errorf("issues upserted = %d, want the one page", sum.Upserted)
	}
	if sum.Boards == nil {
		t.Fatal("summary carries no boards summary: SyncIssues built an engine with no board repository")
	}
	if sum.Boards.Boards != 2 {
		t.Errorf("boards summary = %+v, want both boards landed", sum.Boards)
	}
	if sum.Boards.Dropped == nil {
		t.Error("dropped is nil, want the empty slice the frontend expects")
	}
	cached, err := a.ListBoards(p.ID)
	if err != nil || len(cached) != 2 {
		t.Fatalf("boards after a sync = %+v, %v, want both cached", cached, err)
	}
	if cached[0].Name != "PLAT Kanban" || cached[1].Name != "PLAT Scrum" {
		t.Errorf("boards after a sync = %+v, want the pair the backend answered with", cached)
	}
}

// checkingBackend answers CanTransition with a fixed check, or with an
// error when refuse is set, so a test can prove which of the two the
// binding passes on.
type checkingBackend struct {
	stubIssueBackend
	check  backend.TransitionCheck
	refuse error
}

func (b *checkingBackend) CanTransition(context.Context, string, []string) (backend.TransitionCheck, error) {
	if b.refuse != nil {
		return backend.TransitionCheck{}, b.refuse
	}
	return b.check, nil
}

// seedCard puts one card in the cache so the board writes have a row to
// read, journal against, and move.
func seedCard(t *testing.T, a *App, profileID, key, statusID string) {
	t.Helper()
	rows := []backend.Issue{{
		Key: key, ID: key, Project: "PLAT", Type: backend.TypeStory, Summary: key,
		Status: "To Do", StatusID: statusID, SprintID: "12", SprintName: "Sprint 12",
		Rank: "0|" + key, Updated: "2026-09-01T00:00:00Z",
	}}
	if err := a.repo.UpsertPage(context.Background(), profileID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

// TestABoardWriteJournalsWhileACommitRuns is the busy guard's absence,
// asserted rather than assumed. The board writes are local journal writes,
// like EditIssue and CreateIssue, so a drag during a commit is allowed;
// giving them acquire would refuse a drag no other write refuses.
func TestABoardWriteJournalsWhileACommitRuns(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedCard(t, a, p.ID, "PLAT-1", "1")

	if err := a.acquire(p.ID, "commit"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer a.release(p.ID)

	if err := a.MoveIssueToColumn(p.ID, "PLAT-1", "3"); err != nil {
		t.Fatalf("move a card while a commit runs: %v, want the write to go through", err)
	}
	rows, err := a.repo.PendingForKey(a.ctx, p.ID, "PLAT-1")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(rows) != 1 || rows[0].EntityType != issuerepo.EntityTransition {
		t.Errorf("pending rows = %+v, want the one transition row", rows)
	}
}

// TestMoveIssueToColumnJournalsTheStatusAsIDAndName pins the value the
// binding writes through. Commit pushes the id half and the Pending
// changes dialog and the Activity tab read the name half, so a row
// carrying one without the other is either a push with nothing to push or
// a dialog printing "3".
func TestMoveIssueToColumnJournalsTheStatusAsIDAndName(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	// Two cards, so the target column's status is named in the cache the
	// way it is in the app: by an issue already sitting in it.
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "PLAT-1", Project: "PLAT", Type: backend.TypeStory, Summary: "one",
			Status: "To Do", StatusID: "1", Rank: "0|a", Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-2", ID: "PLAT-2", Project: "PLAT", Type: backend.TypeStory, Summary: "two",
			Status: "In Progress", StatusID: "3", Rank: "0|b", Updated: "2026-09-01T00:00:00Z"},
	}
	if err := a.repo.UpsertPage(context.Background(), p.ID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := a.MoveIssueToColumn(p.ID, "PLAT-1", "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	pending, err := a.repo.PendingForKey(a.ctx, p.ID, "PLAT-1")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending rows = %+v, want exactly one", pending)
	}
	if row := pending[0]; row.BeforeVal != "1|To Do" || row.AfterVal != "3|In Progress" {
		t.Errorf("journal row = %+v, want id|Name on both sides of the move", row)
	}
}

// TestMoveIssueToSprintRefusesASprintIdThatIsNotANumber keeps a value that
// would end up in a URL path from reaching one. The backlog, which is the
// empty id, is a destination and stays allowed.
func TestMoveIssueToSprintRefusesASprintIdThatIsNotANumber(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedCard(t, a, p.ID, "PLAT-1", "1")

	err := a.MoveIssueToSprint(p.ID, "PLAT-1", "fourteen")
	if err == nil {
		t.Fatal("a sprint id of \"fourteen\" was accepted, want a refusal")
	}
	if !strings.Contains(err.Error(), "not a number") {
		t.Errorf("err = %v, want it to say the id is not a number", err)
	}
	if err := a.MoveIssueToSprint(p.ID, "PLAT-1", ""); err != nil {
		t.Errorf("move to the backlog: %v, want the empty id to be a destination", err)
	}
}

// TestRankIssueRefusesANeighbourTheCacheDoesNotHold keeps a rank from being
// journaled against an issue nobody has seen, which Commit would then push
// against a key Jira may not have.
func TestRankIssueRefusesANeighbourTheCacheDoesNotHold(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedCard(t, a, p.ID, "PLAT-1", "1")

	err := a.RankIssue(p.ID, "PLAT-1", "PLAT-404", true, 1)
	if err == nil {
		t.Fatal("a rank against an uncached neighbour was accepted, want a refusal")
	}
	if !strings.Contains(err.Error(), "sync first") {
		t.Errorf("err = %v, want the cache's own refusal", err)
	}
	rows, err := a.repo.PendingForKey(a.ctx, p.ID, "PLAT-1")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("pending rows = %+v, want nothing journaled", rows)
	}
}

// TestCanTransitionPassesOnBothAnswers proves the check is best effort at
// the boundary: an answer arrives with its reachable statuses filled in,
// and a backend that could not answer arrives as an error rather than as a
// check that says the move is illegal.
func TestCanTransitionPassesOnBothAnswers(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	b := &checkingBackend{check: backend.TransitionCheck{Allowed: false}}
	a.backends[p.ID] = b

	check, err := a.CanTransition(p.ID, "PLAT-1", "3")
	if err != nil {
		t.Fatalf("can transition: %v", err)
	}
	if check.Allowed {
		t.Error("check says the move is allowed, want the backend's own answer")
	}
	if check.Reachable == nil {
		t.Error("reachable is nil, want the empty slice the frontend expects")
	}

	b.refuse = errors.New("503 Service Unavailable")
	if _, err := a.CanTransition(p.ID, "PLAT-1", "3"); err == nil {
		t.Fatal("a backend that could not answer returned no error, want the caller to hear it could not check")
	}
}

// lifecycleBackend blocks inside StartSprint until the test lets it go, so a
// second ceremony can be attempted while the first is genuinely in flight.
// Everything else it answers comes from the simple board backend.
type lifecycleBackend struct {
	simpleBoardBackend
	started chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (b *lifecycleBackend) StartSprint(context.Context, int, backend.SprintDraft) error {
	b.once.Do(func() { close(b.started) })
	<-b.proceed
	return nil
}

var _ backend.BoardBackend = (*lifecycleBackend)(nil)

// seedSprintCard puts one card in the cache in a named sprint, so the bulk
// move has rows to read, journal against and move.
func seedSprintCard(t *testing.T, a *App, profileID, key, sprintID, sprintName string) {
	t.Helper()
	rows := []backend.Issue{{
		Key: key, ID: key, Project: "PLAT", Type: backend.TypeStory, Summary: key,
		Status: "To Do", StatusID: "1", SprintID: sprintID, SprintName: sprintName,
		Rank: "0|" + key, Updated: "2026-09-01T00:00:00Z",
	}}
	if err := a.repo.UpsertPage(context.Background(), profileID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

// seedScrumBoard caches one board with the two sprints the moves aim at, so
// the destination has a name to be journaled under.
func seedScrumBoard(t *testing.T, a *App, profileID string) {
	t.Helper()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	cols := []backend.BoardColumn{{Name: "To Do", StatusIDs: []string{"1"}}, {Name: "Done", StatusIDs: []string{"5"}}}
	sprints := []backend.Sprint{
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
	}
	if err := a.boards.ReplaceBoard(a.ctx, profileID, board, cols, sprints, nil); err != nil {
		t.Fatalf("seed board: %v", err)
	}
}

// TestALifecycleCallIsRefusedWhileABoardsRefreshHoldsTheLock is the guard
// these two take and the journal writes deliberately do not: they push to
// Jira, so they queue behind a sync, a commit, an import or a boards
// refresh, and the refusal has to name which of them is actually running.
func TestALifecycleCallIsRefusedWhileABoardsRefreshHoldsTheLock(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")

	if err := a.acquire(p.ID, "boards refresh"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer a.release(p.ID)

	err := a.StartSprint(p.ID, 1, 13, "Sprint 13", "", "2026-09-09", "2026-09-23")
	if err == nil {
		t.Fatal("StartSprint during a boards refresh = nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), "boards refresh") {
		t.Errorf("err = %v, want it to name the operation that is running", err)
	}
	if _, err = a.CompleteSprint(p.ID, 1, 12, ""); err == nil || !strings.Contains(err.Error(), "boards refresh") {
		t.Errorf("CompleteSprint err = %v, want the same refusal", err)
	}
}

// TestASecondCeremonyIsRefusedWhileTheFirstRuns catches the guard doing its
// real job: a start that is in flight, not one the test set a flag for. Two
// sprints being started at once is exactly what the lock exists to stop.
func TestASecondCeremonyIsRefusedWhileTheFirstRuns(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seed := twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")
	blocking := &lifecycleBackend{
		simpleBoardBackend: *seed,
		started:            make(chan struct{}),
		proceed:            make(chan struct{}),
	}
	a.backends[p.ID] = blocking

	done := make(chan error, 1)
	go func() { done <- a.StartSprint(p.ID, 1, 13, "Sprint 13", "", "2026-09-09", "2026-09-23") }()
	<-blocking.started

	_, err := a.CompleteSprint(p.ID, 1, 12, "")
	if err == nil {
		t.Fatal("a second ceremony while one runs = nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), "sprint") || !strings.Contains(err.Error(), "already running") {
		t.Errorf("err = %v, want the guard's own refusal naming the sprint action", err)
	}

	close(blocking.proceed)
	if err := <-done; err != nil {
		t.Fatalf("the first start: %v, want it to finish", err)
	}
	// The lock is released, so the next ceremony is attempted rather than
	// refused: it fails on the board's own state, which is the backend
	// talking and not the guard.
	if _, err := a.CompleteSprint(p.ID, 1, 12, ""); err == nil || strings.Contains(err.Error(), "already running") {
		t.Errorf("err = %v, want the guard free once the first call returned", err)
	}
}

// TestJournalSprintMovesWritesOneRowPerMovedCard is the bulk move: one
// journal row per card, named by the destination the sprint list knows,
// through the same entity type and the same discard path a single move
// already has.
func TestJournalSprintMovesWritesOneRowPerMovedCard(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedScrumBoard(t, a, p.ID)
	keys := []string{"PLAT-1", "PLAT-2", "PLAT-3"}
	for _, key := range keys {
		seedSprintCard(t, a, p.ID, key, "12", "Sprint 12")
	}

	if err := a.JournalSprintMoves(p.ID, keys, "13"); err != nil {
		t.Fatalf("bulk move: %v", err)
	}
	for _, key := range keys {
		rows, err := a.repo.PendingForKey(a.ctx, p.ID, key)
		if err != nil {
			t.Fatalf("pending for %s: %v", key, err)
		}
		if len(rows) != 1 || rows[0].EntityType != issuerepo.EntitySprintMove {
			t.Fatalf("%s pending = %+v, want the one sprint move", key, rows)
		}
		if rows[0].BeforeVal != "12|Sprint 12" || rows[0].AfterVal != "13|Sprint 13" {
			t.Errorf("%s journal row = %+v, want id|Name on both sides of the move", key, rows[0])
		}
	}
}

// TestABulkMoveSkipsAnUncachedKeyAndACardAlreadyThere is the selection as it
// really arrives. One card is not in the cache, which must cost that card
// its move and nothing else, and one is already in the target sprint, which
// journals nothing at all: "one row per key" is wrong for any realistic
// selection, and a stale card must not take the rest of the batch down.
func TestABulkMoveSkipsAnUncachedKeyAndACardAlreadyThere(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedScrumBoard(t, a, p.ID)
	seedSprintCard(t, a, p.ID, "PLAT-1", "12", "Sprint 12")
	seedSprintCard(t, a, p.ID, "PLAT-2", "12", "Sprint 12")
	seedSprintCard(t, a, p.ID, "PLAT-3", "13", "Sprint 13")

	keys := []string{"PLAT-1", "PLAT-404", "PLAT-2", "PLAT-3"}
	if err := a.JournalSprintMoves(p.ID, keys, "13"); err != nil {
		t.Fatalf("bulk move: %v, want the uncached key to cost only itself", err)
	}
	for _, want := range []struct {
		key  string
		rows int
	}{{"PLAT-1", 1}, {"PLAT-2", 1}, {"PLAT-3", 0}, {"PLAT-404", 0}} {
		rows, err := a.repo.PendingForKey(a.ctx, p.ID, want.key)
		if err != nil {
			t.Fatalf("pending for %s: %v", want.key, err)
		}
		if len(rows) != want.rows {
			t.Errorf("%s pending = %+v, want %d row(s)", want.key, rows, want.rows)
		}
	}
	all, err := a.repo.ListPendingChanges(a.ctx, p.ID)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("journal = %+v, want the two cards that actually moved", all)
	}
}

// TestABulkMoveRefusesASprintIdThatIsNotANumber keeps the single move's own
// guard on the bulk path: the id ends up in a URL path at Commit.
func TestABulkMoveRefusesASprintIdThatIsNotANumber(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedSprintCard(t, a, p.ID, "PLAT-1", "12", "Sprint 12")

	if err := a.JournalSprintMoves(p.ID, []string{"PLAT-1"}, "fourteen"); err == nil {
		t.Fatal("a sprint id that is not a number was accepted, want a refusal")
	}
	if err := a.JournalSprintMoves(p.ID, nil, "13"); err == nil {
		t.Error("an empty selection was accepted, want a refusal")
	}
	if err := a.JournalSprintMoves(p.ID, []string{"PLAT-404"}, "13"); err == nil {
		t.Error("a selection of nothing but uncached keys was accepted, want it to say so")
	}
}

// TestPendingInSprintCountsOnlyTheCardsStayingInIt is what the Complete
// button asks before it opens its dialog. A card journaled out of the sprint
// is not a reason to stop, and it needs no case of its own: the move wrote
// the destination onto the cached row as it was made.
func TestPendingInSprintCountsOnlyTheCardsStayingInIt(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedScrumBoard(t, a, p.ID)
	seedSprintCard(t, a, p.ID, "PLAT-1", "12", "Sprint 12")
	seedSprintCard(t, a, p.ID, "PLAT-2", "12", "Sprint 12")

	if n, err := a.PendingInSprint(p.ID, 12); err != nil || n != 0 {
		t.Fatalf("pending in sprint 12 = %d, %v, want none before anything is journaled", n, err)
	}
	if err := a.EditIssue(p.ID, "PLAT-2", "summary", "Renamed"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if err := a.MoveIssueToSprint(p.ID, "PLAT-1", "13"); err != nil {
		t.Fatalf("move: %v", err)
	}

	n, err := a.PendingInSprint(p.ID, 12)
	if err != nil {
		t.Fatalf("pending in sprint 12: %v", err)
	}
	if n != 1 {
		t.Errorf("pending in sprint 12 = %d, want only the edit on the card that is staying", n)
	}
	if n, err = a.PendingInSprint(p.ID, 13); err != nil || n != 1 {
		t.Errorf("pending in sprint 13 = %d, %v, want the card journaled into it", n, err)
	}
}

// TestSuggestSprintDatesReadsTheBoardsOwnHistory is the start dialog's
// defaults, end to end: the board's closed sprints decide the length and its
// last sprint decides the name, both from the cache, so the dialog opens
// whether or not Jira can be reached.
func TestSuggestSprintDatesReadsTheBoardsOwnHistory(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprints := []backend.Sprint{
		{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z", EndDate: "2026-08-11T09:00:00Z"},
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-08-25T09:00:00Z"},
	}
	cols := []backend.BoardColumn{{Name: "To Do", StatusIDs: []string{"1"}}}
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, board, cols, sprints, nil); err != nil {
		t.Fatalf("seed board: %v", err)
	}

	got, err := a.SuggestSprintDates(p.ID, 1)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if got.Length != 7 || !got.FromHistory {
		t.Errorf("suggestion = %+v, want the board's own week", got)
	}
	if got.Name != "Sprint 13" {
		t.Errorf("name = %q, want the number after the board's last sprint", got.Name)
	}
	if got.Start != time.Now().Format("2006-01-02") {
		t.Errorf("start = %q, want today", got.Start)
	}
	empty, err := a.SuggestSprintDates(p.ID, 99)
	if err != nil {
		t.Fatalf("suggest for a board with no history: %v", err)
	}
	if empty.Length != 14 || empty.FromHistory {
		t.Errorf("suggestion = %+v, want a fortnight marked as invented", empty)
	}
}
