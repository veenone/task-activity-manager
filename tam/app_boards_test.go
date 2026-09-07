package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
func (b *simpleBoardBackend) BoardIssueKeys(context.Context, int, string) ([]string, error) {
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
func (b *blockingBoardBackend) BoardIssueKeys(context.Context, int, string) ([]string, error) {
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
