package sprints_test

import (
	"context"
	"errors"

	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprints"
)

// auditCall is one row the write path left in the local audit trail.
type auditCall struct {
	sprintID int
	action   string
	before   string
	after    string
}

// fakeIssues is the issue cache seam: the cached sprint columns a delete
// blanks, and the audit rows all three management writes leave behind. Its
// calls land in the store's own ordered log, because the thing worth
// asserting is the order the two repositories are touched in, which neither
// of them can enforce from inside its own transaction.
type fakeIssues struct {
	log *[]string
	// carrying is each cached issue's sprint id, by key.
	carrying map[string]string
	clearErr error

	audits   []auditCall
	auditErr error
}

func newIssues(store *fakeStore) *fakeIssues {
	return &fakeIssues{log: &store.ops, carrying: map[string]string{}}
}

func (i *fakeIssues) ClearSprint(_ context.Context, _, sprintID string) error {
	*i.log = append(*i.log, "issue columns")
	if i.clearErr != nil {
		return i.clearErr
	}
	for key, held := range i.carrying {
		if held == sprintID {
			i.carrying[key] = ""
		}
	}
	return nil
}

func (i *fakeIssues) AuditSprint(_ context.Context, _ string, sprintID int, action, before, after string) error {
	i.audits = append(i.audits, auditCall{sprintID: sprintID, action: action, before: before, after: after})
	return i.auditErr
}

// manageService is the service the three management writes are exercised
// through: the same one the ceremonies use, with the issue cache wired the
// way app.go wires it.
func manageService(b *fakeBackend, store *fakeStore, issues *fakeIssues) *sprints.Service {
	s := newService(b, store)
	s.Issues = issues
	return s
}

// draft is what the dialog collects: bare dates, since that is what a date
// input produces.
func draft(name, goal string) backend.SprintDraft {
	return backend.SprintDraft{Name: name, Goal: goal, StartDate: "2026-09-09", EndDate: "2026-09-23"}
}

// TestCreateSendsTheDraftToJiraAndCachesTheSprintWithItsGoal is the whole of
// a create: the two bare dates reach Jira in the Agile API's own datetime
// format, the sprint Jira made comes back, and the board's re-read list
// lands in the cache with the goal on it, which is the field the row and the
// edit dialog are both drawn from.
func TestCreateSendsTheDraftToJiraAndCachesTheSprintWithItsGoal(t *testing.T) {
	made := backend.Sprint{ID: 14, BoardID: 1, Name: "Sprint 14", State: "future", Goal: "Ship the grid"}
	b := &fakeBackend{made: made, sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "active"}, made}}
	store := newStore()
	issues := newIssues(store)

	got, note, err := manageService(b, store, issues).Create(context.Background(), "p1", 1, draft("Sprint 14", "Ship the grid"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if note != "" {
		t.Errorf("note = %q, want none: the list was re-read", note)
	}
	if got.ID != 14 {
		t.Errorf("create = %+v, want the sprint Jira made, id and all", got)
	}
	if len(b.created) != 1 || b.created[0].boardID != 1 {
		t.Fatalf("creates = %+v, want one on board 1", b.created)
	}
	sent := b.created[0].draft
	if sent.Goal != "Ship the grid" {
		t.Errorf("goal sent = %q, want the one the dialog collected", sent.Goal)
	}
	if !strings.HasPrefix(sent.StartDate, "2026-09-09T09:00:00.000") || !strings.HasPrefix(sent.EndDate, "2026-09-23T09:00:00.000") {
		t.Errorf("dates sent = %q..%q, want Jira's own datetime format", sent.StartDate, sent.EndDate)
	}
	if len(store.sprints) != 2 || store.sprints[1].Goal != "Ship the grid" {
		t.Errorf("cached sprints = %+v, want the re-read list carrying the goal", store.sprints)
	}
	want := auditCall{sprintID: 14, action: "create", after: "Sprint 14"}
	if len(issues.audits) != 1 || issues.audits[0] != want {
		t.Errorf("audit rows = %+v, want one %+v", issues.audits, want)
	}
}

// TestACreateWhoseRefreshComesBackEmptyStillReportsTheSprintAndWhatToDo is
// the case the refusal in refreshSprints produces on this path. The sprint
// exists in Jira, so the create is a success and the sprint has to travel
// back with the note; Wails would drop it if the note were an error. The
// note has to say what to do, because until a Refresh lands the picker shows
// a board without the sprint that was just made on it.
func TestACreateWhoseRefreshComesBackEmptyStillReportsTheSprintAndWhatToDo(t *testing.T) {
	b := &fakeBackend{made: backend.Sprint{ID: 14, BoardID: 1, Name: "Sprint 14", State: "future"}}
	store := newStore()
	issues := newIssues(store)

	got, note, err := manageService(b, store, issues).Create(context.Background(), "p1", 1, draft("Sprint 14", ""))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.ID != 14 {
		t.Errorf("create = %+v, want the new sprint back beside the note", got)
	}
	if !strings.Contains(note, "no sprints at all") || !strings.Contains(note, "Refresh") {
		t.Errorf("note = %q, want it to say the list was left alone and to press Refresh", note)
	}
	if store.written != 0 {
		t.Error("the empty answer was written to the cache, which would erase the board's whole sprint history")
	}
}

// TestCreateRefusesAnEndBeforeItsStartBeforeJiraHearsAboutIt keeps the date
// pair honest on the path the dialog is not the only way into.
func TestCreateRefusesAnEndBeforeItsStartBeforeJiraHearsAboutIt(t *testing.T) {
	b := &fakeBackend{}
	store := newStore()
	d := backend.SprintDraft{Name: "Sprint 14", StartDate: "2026-09-23", EndDate: "2026-09-09"}

	if _, _, err := manageService(b, store, newIssues(store)).Create(context.Background(), "p1", 1, d); err == nil {
		t.Fatal("create = nil error, want an end before a start refused")
	}
	if len(b.created) != 0 {
		t.Errorf("creates = %+v, want none: the refusal happens before Jira is called", b.created)
	}
}

// TestEditSendsTheDraftAndTheClearGoalFlagAndRecordsTheRename covers the one
// argument that cannot be inferred from the draft: an empty goal box means
// "leave it alone" on its own and "remove the goal" with the flag, and the
// two have to reach the backend as different requests.
func TestEditSendsTheDraftAndTheClearGoalFlagAndRecordsTheRename(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "active"}}}
	store := newStore()
	issues := newIssues(store)

	note, err := manageService(b, store, issues).Edit(context.Background(), "p1", 1, 13, draft("Sprint 13 renamed", ""), true)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if note != "" {
		t.Errorf("note = %q, want none", note)
	}
	if len(b.edited) != 1 {
		t.Fatalf("edits = %+v, want one", b.edited)
	}
	got := b.edited[0]
	if got.sprintID != 13 || !got.clearGoal {
		t.Errorf("edit = %+v, want sprint 13 with the goal cleared", got)
	}
	if !strings.HasPrefix(got.draft.StartDate, "2026-09-09T09:00:00.000") {
		t.Errorf("start sent = %q, want Jira's own datetime format", got.draft.StartDate)
	}
	want := auditCall{sprintID: 13, action: "edit", before: "Sprint 13", after: "Sprint 13 renamed"}
	if len(issues.audits) != 1 || issues.audits[0] != want {
		t.Errorf("audit rows = %+v, want one %+v", issues.audits, want)
	}
}

// TestEditRefusesAClosedSprintJiraReportsWhileTheCacheStillCallsItFuture is
// why the state is re-read over the wire. The cache here says future, which
// is what a cache that has not been refreshed since the sprint closed says,
// and a closed sprint's dates are what velocity and burndown are drawn from.
func TestEditRefusesAClosedSprintJiraReportsWhileTheCacheStillCallsItFuture(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "closed"}}}
	store := newStore()
	if state := store.onBoard["1/13"]; state != "future" {
		t.Fatalf("the fixture's cached state is %q, and this test needs the stale future it was written for", state)
	}

	_, err := manageService(b, store, newIssues(store)).Edit(context.Background(), "p1", 1, 13, draft("Sprint 13", ""), false)
	if err == nil {
		t.Fatal("edit = nil error, want a closed sprint refused")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("edit error = %q, want it to name the state that refused it", err)
	}
	if len(b.edited) != 0 {
		t.Errorf("edits = %+v, want none: nothing may reach Jira after that refusal", b.edited)
	}
}

// TestNeitherEditNorDeleteTrustsASprintListWithNothingInIt is the guard the
// whole delete path turns on. core/jira maps any 400 on the first page of
// the sprint endpoint to ErrNoSprints, for the kanban board that really has
// none, and the backend turns that into an empty slice and no error. So one
// flaky 400 at 2am looks exactly like "this sprint is not there", and
// reading it as an absence would tell the user their sprint was already
// deleted and then remove TAM's copy of a sprint still running in Jira.
func TestNeitherEditNorDeleteTrustsASprintListWithNothingInIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*sprints.Service) error
	}{
		{"an edit", func(s *sprints.Service) error {
			_, err := s.Edit(context.Background(), "p1", 1, 13, draft("Sprint 13", ""), false)
			return err
		}},
		{"a delete", func(s *sprints.Service) error {
			_, err := s.Delete(context.Background(), "p1", 1, 13)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &fakeBackend{sprints: []backend.Sprint{}}
			store := newStore()
			issues := newIssues(store)

			err := tc.call(manageService(b, store, issues))
			if err == nil {
				t.Fatal("an empty sprint list was taken as an answer")
			}
			if len(b.edited) != 0 || len(b.deleted) != 0 {
				t.Errorf("edits = %+v and deletes = %v, want nothing to have reached Jira", b.edited, b.deleted)
			}
			if len(store.ops) != 0 {
				t.Errorf("the cache was touched (%v), and it must be left exactly as it was", store.ops)
			}
			if len(issues.audits) != 0 {
				t.Errorf("audit rows = %+v, want none: nothing happened to record", issues.audits)
			}
		})
	}
}

// TestDeleteRefusesASprintJiraSaysIsActiveWhileTheCacheStillCallsItFuture is
// the same staleness from the other side. Every other guard in this package
// refuses too much when the cache is stale, which costs a Refresh; this one
// would permit too much, and it costs a sprint a team is working in.
func TestDeleteRefusesASprintJiraSaysIsActiveWhileTheCacheStillCallsItFuture(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "active"}}}
	store := newStore()
	issues := newIssues(store)

	_, err := manageService(b, store, issues).Delete(context.Background(), "p1", 1, 13)
	if err == nil {
		t.Fatal("delete = nil error, want a running sprint refused")
	}
	if !strings.Contains(err.Error(), "active") {
		t.Errorf("delete error = %q, want it to name the state Jira reported", err)
	}
	if len(b.deleted) != 0 || len(store.ops) != 0 {
		t.Errorf("deletes = %v and cache work = %v, want neither", b.deleted, store.ops)
	}
}

// TestDeleteRefusesWhenTheSprintListCouldNotBeReadAtAll keeps a read failure
// out of the "already gone" branch: not knowing is not the same as knowing
// it is not there.
func TestDeleteRefusesWhenTheSprintListCouldNotBeReadAtAll(t *testing.T) {
	b := &fakeBackend{sprintErr: errors.New("dial tcp: connection refused")}
	store := newStore()
	issues := newIssues(store)

	_, err := manageService(b, store, issues).Delete(context.Background(), "p1", 1, 13)
	if err == nil {
		t.Fatal("delete = nil error, want an unreadable sprint list refused")
	}
	if len(b.deleted) != 0 || len(store.ops) != 0 {
		t.Errorf("deletes = %v and cache work = %v, want the cache left exactly as it was", b.deleted, store.ops)
	}
}

// TestDeleteOfASprintJiraNoLongerHasIsAlreadyGoneAndStillCleansTheCache is
// the third answer, and the reason it is a success: somebody deleted the
// sprint on the web, the list that says so is a real list with real sprints
// in it, and all that is left to do is remove TAM's own copy.
func TestDeleteOfASprintJiraNoLongerHasIsAlreadyGoneAndStillCleansTheCache(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"}}}
	store := newStore()
	store.rows = map[int][]int{1: {12, 13}}
	issues := newIssues(store)
	issues.carrying = map[string]string{"PLAT-1": "13"}

	line, err := manageService(b, store, issues).Delete(context.Background(), "p1", 1, 13)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !strings.Contains(line, "already gone") {
		t.Errorf("delete said %q, want it to say the sprint was gone before TAM asked", line)
	}
	if len(b.deleted) != 0 {
		t.Errorf("deletes = %v, want none: there was nothing left in Jira to delete", b.deleted)
	}
	if len(store.rows[1]) != 1 || store.rows[1][0] != 12 {
		t.Errorf("board 1 holds %v, want only the sprint that still exists", store.rows[1])
	}
	if issues.carrying["PLAT-1"] != "" {
		t.Errorf("PLAT-1 still carries sprint %q, want its sprint columns blanked", issues.carrying["PLAT-1"])
	}
}

// TestDeleteRefusesWhilePendingChangesTargetTheSprintAndNamesCommit stops
// the queue being aimed at a sprint that is about to stop existing. Commit
// is what resolves those rows either way, so the sentence has to name it.
func TestDeleteRefusesWhilePendingChangesTargetTheSprintAndNamesCommit(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"}}}
	store := newStore()
	issues := newIssues(store)
	s := manageService(b, store, issues)
	s.Pending = func(context.Context, string, int) (int, error) { return 2, nil }

	_, err := s.Delete(context.Background(), "p1", 1, 13)
	if err == nil {
		t.Fatal("delete = nil error, want it refused while changes are queued against the sprint")
	}
	if !strings.Contains(err.Error(), "commit") {
		t.Errorf("delete error = %q, want it to name Commit as the thing to do first", err)
	}
	if len(b.deleted) != 0 || len(store.ops) != 0 {
		t.Errorf("deletes = %v and cache work = %v, want neither", b.deleted, store.ops)
	}
}

// TestACleanDeleteReachesEveryBoardsCopyAndEveryIssueCarryingIt is the whole
// of a delete. The sprint is seeded on two boards, because Jira hands one
// sprint to every board whose filter reaches it and a fixture with one board
// would pass while a board-scoped delete shipped. The order the cache is
// touched in is asserted too: board rows first, then the issue columns, then
// the board's re-read list, which is what decides what a crash between two
// transactions in two repositories leaves behind.
func TestACleanDeleteReachesEveryBoardsCopyAndEveryIssueCarryingIt(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"},
	}}
	store := newStore()
	store.rows = map[int][]int{1: {12, 13}, 2: {13}}
	issues := newIssues(store)
	issues.carrying = map[string]string{"PLAT-1": "13", "PLAT-2": "13", "PLAT-3": "12"}

	line, err := manageService(b, store, issues).Delete(context.Background(), "p1", 1, 13)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if line != "" {
		t.Errorf("delete said %q, want nothing beside the success", line)
	}
	if len(b.deleted) != 1 || b.deleted[0] != 13 {
		t.Fatalf("deletes = %v, want sprint 13 deleted in Jira once", b.deleted)
	}
	for boardID, want := range map[int][]int{1: {12}, 2: {}} {
		if len(store.rows[boardID]) != len(want) {
			t.Errorf("board %d holds %v, want %v: the delete carries no board id for exactly this reason", boardID, store.rows[boardID], want)
		}
	}
	for key, held := range issues.carrying {
		if held == "13" {
			t.Errorf("%s still carries the deleted sprint, so it would keep offering it in three views", key)
		}
	}
	if issues.carrying["PLAT-3"] != "12" {
		t.Errorf("PLAT-3 = %q, want the sprint it is really in left alone", issues.carrying["PLAT-3"])
	}
	if got := strings.Join(store.ops, ", "); got != "board rows, issue columns, board list" {
		t.Errorf("cache work happened in the order %q, want board rows, issue columns, board list", got)
	}
	for _, cached := range store.sprints {
		if cached.ID == 13 {
			t.Error("the re-read list still holds the deleted sprint")
		}
	}
	want := auditCall{sprintID: 13, action: "delete", before: "Sprint 13"}
	if len(issues.audits) != 1 || issues.audits[0] != want {
		t.Errorf("audit rows = %+v, want one %+v, the only trace of the sprint left on the machine", issues.audits, want)
	}
}

// TestADeleteWhoseCacheWorkFailsIsStillADeleteWithANote holds the line every
// piece of bookkeeping in this package holds: Jira has destroyed the sprint,
// and reporting that as a failure would tell the user to try again on
// something that already happened.
func TestADeleteWhoseCacheWorkFailsIsStillADeleteWithANote(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"}}}
	store := newStore()
	store.deleteErr = errors.New("database is locked")
	issues := newIssues(store)

	line, err := manageService(b, store, issues).Delete(context.Background(), "p1", 1, 13)
	if err != nil {
		t.Fatalf("delete = %v, want the delete reported as the success it was", err)
	}
	if !strings.Contains(line, "Refresh") {
		t.Errorf("note = %q, want the one thing the user can do about a stale cache", line)
	}
	if len(b.deleted) != 1 {
		t.Errorf("deletes = %v, want the sprint deleted in Jira all the same", b.deleted)
	}
	if len(issues.audits) != 1 || issues.audits[0].action != "delete" {
		t.Errorf("audit rows = %+v, want the delete recorded before the cache work was tried", issues.audits)
	}
}

// TestAManagementWriteWithNoIssueCacheWiredStillLandsInJira is the shape of
// the seam: the audit row and the cached sprint columns are bookkeeping
// after the fact, so a service without them writes to Jira and logs what it
// could not record, rather than refusing the write.
func TestAManagementWriteWithNoIssueCacheWiredStillLandsInJira(t *testing.T) {
	made := backend.Sprint{ID: 14, BoardID: 1, Name: "Sprint 14", State: "future"}
	b := &fakeBackend{made: made, sprints: []backend.Sprint{made}}
	store := newStore()

	got, _, err := newService(b, store).Create(context.Background(), "p1", 1, draft("Sprint 14", ""))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.ID != 14 || len(b.created) != 1 {
		t.Errorf("create = %+v after %d calls, want the sprint made in Jira anyway", got, len(b.created))
	}
}
