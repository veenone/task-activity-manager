package main

import (
	"context"
	"errors"
	"path/filepath"
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
// previous good copy before a test swaps in a slower backend.
type simpleBoardBackend struct {
	stubIssueBackend
	boards  []backend.Board
	columns []backend.BoardColumn
}

func (b *simpleBoardBackend) Boards(context.Context, string) ([]backend.Board, error) {
	return b.boards, nil
}
func (b *simpleBoardBackend) BoardColumns(context.Context, int) ([]backend.BoardColumn, error) {
	return b.columns, nil
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
// BoardColumns until the test closes proceed, so a test can read the store
// while a sync is caught mid-board and prove nothing half-written is
// visible yet.
type blockingBoardBackend struct {
	stubIssueBackend
	boards  []backend.Board
	columns []backend.BoardColumn
	started chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (b *blockingBoardBackend) Boards(context.Context, string) ([]backend.Board, error) {
	return b.boards, nil
}
func (b *blockingBoardBackend) BoardColumns(context.Context, int) ([]backend.BoardColumn, error) {
	b.once.Do(func() { close(b.started) })
	<-b.proceed
	return b.columns, nil
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

// TestSyncBoardsRefusesWhileASyncHoldsTheBusyGuard verifies SyncBoards is
// refused, not queued, while a.busy already names another operation for
// the same profile, the same rule SyncIssues and CommitPendingChanges obey.
func TestSyncBoardsRefusesWhileASyncHoldsTheBusyGuard(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	if err := a.acquire(p.ID, "sync"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer a.release(p.ID)

	if _, err := a.SyncBoards(p.ID); err == nil {
		t.Fatal("SyncBoards while a sync is running = nil error, want a refusal")
	}
}

// TestGetBoardDuringSyncReturnsThePreviousBoardNotAHalfWrittenOne verifies
// a board read that lands while SyncBoards is still fetching from Jira for
// a board sees that board's previous good copy, not a name or column list
// updated ahead of the rest of it.
func TestGetBoardDuringSyncReturnsThePreviousBoardNotAHalfWrittenOne(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	seed := &simpleBoardBackend{
		boards:  []backend.Board{{ID: 1, Name: "Old Name", Type: backend.BoardTypeScrum}},
		columns: []backend.BoardColumn{{Name: "To Do", StatusIDs: []string{"1"}}},
	}
	a.backends[p.ID] = seed
	if _, err := a.SyncBoards(p.ID); err != nil {
		t.Fatalf("seed sync: %v", err)
	}

	blocking := &blockingBoardBackend{
		boards:  []backend.Board{{ID: 1, Name: "New Name", Type: backend.BoardTypeScrum}},
		columns: []backend.BoardColumn{{Name: "Done", StatusIDs: []string{"5"}}},
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
	if len(boards) != 1 || boards[0].Name != "Old Name" {
		t.Fatalf("boards mid-sync = %+v, want the previous good copy", boards)
	}
	view, err := a.GetBoard(p.ID, 1, "", "")
	if err != nil {
		t.Fatalf("get board mid-sync: %v", err)
	}
	if len(view.Columns) != 1 || view.Columns[0].Name != "To Do" {
		t.Fatalf("columns mid-sync = %+v, want the previous good copy", view.Columns)
	}

	close(blocking.proceed)
	if err := <-done; err != nil {
		t.Fatalf("sync: %v", err)
	}

	boards, err = a.ListBoards(p.ID)
	if err != nil {
		t.Fatalf("list boards after sync: %v", err)
	}
	if len(boards) != 1 || boards[0].Name != "New Name" {
		t.Fatalf("boards after sync = %+v, want the new copy", boards)
	}
}
