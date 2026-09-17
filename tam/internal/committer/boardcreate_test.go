package committer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// CreateBoard, AddToBoardBacklog and BoardFilterCheck are the boards phase's
// three Jira-facing calls, alongside CreateSprint (phases_test.go) and the
// board-move writes (boards_test.go).

func (f *fake) CreateBoard(_ context.Context, d backend.BoardDraft) (int, error) {
	if f.boardCreateErr != nil {
		return 0, f.boardCreateErr
	}
	id := 900 + f.nextBoard
	f.nextBoard++
	f.boardsMade = append(f.boardsMade, fmt.Sprintf("%s %s %s %s", d.Name, d.Type, d.FilterName, d.JQL))
	return id, nil
}

func (f *fake) AddToBoardBacklog(_ context.Context, boardID int, keys []string) error {
	if f.backlogErr != nil {
		return f.backlogErr
	}
	f.backlogAdds = append(f.backlogAdds, fmt.Sprintf("%d %s", boardID, strings.Join(keys, ",")))
	return nil
}

func (f *fake) BoardFilterCheck(_ context.Context, boardID int, keys []string) ([]string, error) {
	f.filterChecks = append(f.filterChecks, fmt.Sprintf("%d %s", boardID, strings.Join(keys, ",")))
	if err := f.filterCheckErr[boardID]; err != nil {
		return nil, err
	}
	missing := map[string]bool{}
	for _, k := range f.filterMissing[boardID] {
		missing[k] = true
	}
	present := make([]string, 0, len(keys))
	for _, k := range keys {
		if !missing[k] {
			present = append(present, k)
		}
	}
	return present, nil
}

// draftBoard drafts a board and returns its negative id.
func draftBoard(t *testing.T, h harness, name string) int {
	t.Helper()
	made, err := h.repo.CreateDraftBoard(context.Background(), "p1", issuerepo.DraftBoard{
		Name: name, Type: "scrum", FilterName: name + " filter", JQL: "project = PLAT",
	})
	if err != nil {
		t.Fatal(err)
	}
	return made.ID
}

// draftSprintOnDraftBoard journals a sprint_create row naming a draft
// board's own negative id as its originBoardId, sprint draft id -1.
//
// issuerepo.CreateDraftSprint refuses a BoardID <= 0 (sprintdrafts.go:77),
// which blocks drafting a sprint directly onto a draft board through the
// public entry point; that guard predates draft boards and is unchanged by
// this bundle (issuerepo is out of this task's files). This writes the
// journal and draft-sprint row by hand, the same way
// TestABoardRowUnderADraftKeyWaitsForTheNextCommit (boards_test.go) already
// writes a pending_change row by hand for a shape the public API cannot
// produce, to exercise the boards phase's own resolution of it in isolation.
func draftSprintOnDraftBoard(t *testing.T, h harness, boardDraftID int, boardName, sprintName string) {
	t.Helper()
	d := issuerepo.DraftSprint{BoardID: boardDraftID, BoardName: boardName, Name: sprintName}
	encoded, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Exec(
		`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date, goal, draft) VALUES (?, -1, ?, ?, 'future', '', '', '', 1)`,
		"p1", boardDraftID, sprintName); err != nil {
		t.Fatal(err)
	}
	if err := journal.Put(h.db, "p1", issuerepo.EntitySprintCreate, "-1", issuerepo.FieldCreate, "", string(encoded), ""); err != nil {
		t.Fatal(err)
	}
}

// Case 1: board create succeeds, then the sprint is pushed with the real
// board id -- the case the boards-before-sprints ordering exists for.
func TestADraftBoardWithADraftSprintOnItCommitTogetherWithTheRealBoardID(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	boardID := draftBoard(t, h, "PLAT Checkout Board")
	draftSprintOnDraftBoard(t, h, boardID, "PLAT Checkout Board", "Sprint 1")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 {
		t.Fatalf("result: %+v", res)
	}
	if len(h.jira.boardsMade) != 1 || h.jira.boardsMade[0] != "PLAT Checkout Board scrum PLAT Checkout Board filter project = PLAT" {
		t.Errorf("board created: %v", h.jira.boardsMade)
	}
	if len(h.jira.sprintsMade) != 1 || h.jira.sprintsMade[0] != "900 Sprint 1" {
		t.Fatalf("the sprint is pushed with the real board id, not the placeholder: %v", h.jira.sprintsMade)
	}
}

// Case 2: board create fails, so the sprint is held with a stated reason and
// no placeholder reaches Jira; the held row survives for the next Commit.
func TestADraftBoardCreateFailureHoldsItsDraftSprintWithAReason(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	boardID := draftBoard(t, h, "PLAT Checkout Board")
	draftSprintOnDraftBoard(t, h, boardID, "PLAT Checkout Board", "Sprint 1")
	h.jira.boardCreateErr = errors.New("POST failed: 403 you cannot manage boards")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.sprintsMade) != 0 {
		t.Fatalf("no placeholder board id reached Jira: %v", h.jira.sprintsMade)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != "PLAT Checkout Board" || res.Failures[0].EntityType != issuerepo.EntityBoardCreate || !res.Failures[0].Retryable {
		t.Fatalf("the board create failed and is worth retrying: %+v", res.Failures)
	}
	if len(res.Held) != 1 || res.Held[0].Key != "-1" || res.Held[0].EntityType != issuerepo.EntitySprintCreate {
		t.Fatalf("the sprint is held: %+v", res.Held)
	}
	want := `waits for board "PLAT Checkout Board", which Jira refused`
	if res.Held[0].Reason != want {
		t.Errorf("reason = %q, want %q", res.Held[0].Reason, want)
	}
	if res.Remaining != 2 {
		t.Errorf("remaining = %d, want the board and the sprint both kept", res.Remaining)
	}
	pend, err := h.repo.ListPendingChanges(ctx, "p1")
	if err != nil || len(pend) != 2 {
		t.Fatalf("both rows survive for the next Commit: %+v %v", pend, err)
	}
}

// Case 3: neither of the above leaves the journal in a state where a second
// Commit double-creates the board.
func TestASecondCommitAfterAFailedBoardCreateDoesNotCreateTheBoardTwice(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	boardID := draftBoard(t, h, "PLAT Checkout Board")
	draftSprintOnDraftBoard(t, h, boardID, "PLAT Checkout Board", "Sprint 1")
	h.jira.boardCreateErr = errors.New("POST failed: 403 you cannot manage boards")

	if _, err := h.eng.Commit(ctx, "p1", "PLAT"); err != nil {
		t.Fatal(err)
	}
	if len(h.jira.boardsMade) != 0 {
		t.Fatalf("nothing was created on the failed attempt: %v", h.jira.boardsMade)
	}

	h.jira.boardCreateErr = nil
	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.boardsMade) != 1 {
		t.Errorf("the retry creates the board exactly once: %v", h.jira.boardsMade)
	}
	if len(h.jira.sprintsMade) != 1 || h.jira.sprintsMade[0] != "900 Sprint 1" {
		t.Errorf("the held sprint is finally pushed, with the real id: %v", h.jira.sprintsMade)
	}
	if len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
}

// Test 4: a backlog add batches and a 207 fails that batch without failing
// the whole Commit's other work. The batching and the 207 handling
// themselves are core/jira's (bulkWrite, tested there); here only the
// committer's own reaction to a failed push is under test.
func TestABacklogAddFailureLeavesItsRowAndTheRestOfTheCommitLands(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.AddToBoard(ctx, "p1", []string{"PLAT-1"}, demoBoard, issuerepo.ScopeBacklog); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-2", "summary", "dos"); err != nil {
		t.Fatal(err)
	}
	h.jira.backlogErr = errors.New("POST failed: 207 multi-status")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != "PLAT-1" || res.Failures[0].EntityType != issuerepo.EntityIssueBoard || !res.Failures[0].Retryable {
		t.Fatalf("the backlog push failed and is worth retrying: %+v", res.Failures)
	}
	if strings.Join(res.Committed, ",") != "PLAT-2" {
		t.Errorf("the edit still landed: %+v", res)
	}
	if res.Remaining != 1 {
		t.Errorf("remaining = %d, want the board add kept for a retry", res.Remaining)
	}
}

// Test 4b: the bundle's headline flow -- draft a board, queue an add onto
// it while it is still a draft, then Commit. The board is created and
// rekeyed before the queued add is pushed, so the add must go out against
// the real board id, not the placeholder the row was journaled under.
func TestADraftBoardWithAQueuedAddCommitsTheAddAgainstTheRealBoardID(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	boardID := draftBoard(t, h, "PLAT Checkout Board")
	if err := h.repo.AddToBoard(ctx, "p1", []string{"PLAT-1"}, boardID, issuerepo.ScopeBacklog); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 {
		t.Fatalf("result: %+v", res)
	}
	if len(h.jira.backlogAdds) != 1 || h.jira.backlogAdds[0] != "900 PLAT-1" {
		t.Fatalf("the add is pushed with the real board id, not the placeholder: %v", h.jira.backlogAdds)
	}
}

// Test 4c: the board create lands but the queued add fails on the same
// Commit (a retryable Jira error), so the row survives for a retry. The
// board's create row is gone by then -- r.boardRealID, which only lives for
// one Commit, cannot resolve it on the retry -- so the row's own board id
// must already be the real one, which only RekeyBoard can have made true.
func TestARetriedBoardAddAfterTheBoardWasAlreadyCreatedPushesWithTheRealBoardID(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	boardID := draftBoard(t, h, "PLAT Checkout Board")
	if err := h.repo.AddToBoard(ctx, "p1", []string{"PLAT-1"}, boardID, issuerepo.ScopeBacklog); err != nil {
		t.Fatal(err)
	}
	h.jira.backlogErr = errors.New("POST failed: 503 service unavailable")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.boardsMade) != 1 {
		t.Fatalf("the board was created on the first Commit: %v", h.jira.boardsMade)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != "PLAT-1" || !res.Failures[0].Retryable {
		t.Fatalf("the add failed and is worth retrying: %+v", res.Failures)
	}

	h.jira.backlogErr = nil
	res, err = h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.boardsMade) != 1 {
		t.Fatalf("the board is not created a second time: %v", h.jira.boardsMade)
	}
	if len(res.Failures) != 0 {
		t.Fatalf("the retried add is pushed with the real board id, not refused as a placeholder: %+v", res.Failures)
	}
	if len(h.jira.backlogAdds) != 1 || h.jira.backlogAdds[0] != "900 PLAT-1" {
		t.Fatalf("the retry pushes with the real board id: %v", h.jira.backlogAdds)
	}
}

// Test 5: a sprint-scope add goes through MoveIssuesToSprint and makes no
// AddToBoardBacklog call. Which call was made is the behaviour under test
// here, per the brief.
func TestASprintScopeBoardAddGoesThroughMoveIssuesToSprintNotAddToBoardBacklog(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.AddToBoard(ctx, "p1", []string{"PLAT-1"}, demoBoard, "13"); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.backlogAdds) != 0 {
		t.Errorf("no AddToBoardBacklog call for a sprint-scope add: %v", h.jira.backlogAdds)
	}
	if len(h.jira.pushed) != 1 || h.jira.pushed[0] != "sprint 13 PLAT-1" {
		t.Fatalf("the add goes through the same MoveIssuesToSprint the board moves pass uses: %v", h.jira.pushed)
	}
	if len(res.Moved) != 1 || res.Moved[0].Key != "PLAT-1" || res.Moved[0].EntityType != issuerepo.EntityIssueBoard || res.Moved[0].Target != "board 1, sprint 13" {
		t.Errorf("moved: %+v", res.Moved)
	}
	if res.Remaining != 0 {
		t.Errorf("remaining: %d", res.Remaining)
	}
}

// Test 6: a draft issue is held until the create phase gives it a key, then
// pushed. The boards phase runs before the create phases, so an add under a
// still-draft key is left alone for this Commit and pushed on the next.
//
// AddToBoard cannot set this scenario up: a draft has nowhere local to carry
// a pending board membership, so moveDraft (issuerepo/boardwrites.go)
// no-ops an issue_board add on a draft key and journals nothing. The row
// this guards against is written by hand, the same way
// TestABoardRowUnderADraftKeyWaitsForTheNextCommit (boards_test.go) already
// does for the other three board-row types.
func TestABoardAddOnADraftIssueWaitsForItsCreateThenPushesOnTheNextCommit(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "New task"})
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Put(h.db, "p1", issuerepo.EntityIssueBoard, temp, issuerepo.BoardField(demoBoard),
		"", issuerepo.MoveValue(strconv.Itoa(demoBoard), issuerepo.ScopeBacklog), ""); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 || res.Created[0].TempKey != temp || res.Created[0].Key != "PLAT-501" {
		t.Fatalf("the draft was created: %+v", res)
	}
	if len(h.jira.backlogAdds) != 0 {
		t.Fatalf("not pushed yet, the boards phase ran before the create: %v", h.jira.backlogAdds)
	}
	if res.Remaining != 1 {
		t.Fatalf("the board add is left pending, now under the real key: %d", res.Remaining)
	}
	pend, err := h.repo.PendingForKey(ctx, "p1", "PLAT-501")
	if err != nil || len(pend) != 1 || pend[0].EntityType != issuerepo.EntityIssueBoard {
		t.Fatalf("rekeyed under the real key: %+v %v", pend, err)
	}

	res, err = h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.backlogAdds) != 1 || h.jira.backlogAdds[0] != "1 PLAT-501" {
		t.Fatalf("the second Commit pushes it: %v", h.jira.backlogAdds)
	}
	if res.Remaining != 0 {
		t.Errorf("remaining: %d", res.Remaining)
	}
}
