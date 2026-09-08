package committer_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

// demoBoard is the board every rank here is dropped on.
const demoBoard = 1

// statusNames is what the fake calls the three status ids the tests move
// between, so a transition writes a name onto the row the way Jira does.
var statusNames = map[string]string{"1": "To Do", "3": "In Progress", "5": "Done"}

// boardOrder is the board's final local order as a test dictates it, the
// one thing boardrepo answers for in the app. A board with no order
// scripted answers the way a board missing from the store does.
type boardOrder struct {
	order map[int][]string
	err   map[int]error
}

func newBoardOrder() *boardOrder {
	return &boardOrder{order: map[int][]string{}, err: map[int]error{}}
}

func (b *boardOrder) CellOrder(_ context.Context, _ string, boardID int) ([]string, error) {
	if err := b.err[boardID]; err != nil {
		return nil, err
	}
	order, ok := b.order[boardID]
	if !ok {
		return nil, fmt.Errorf("board %d is not in the store; sync the boards first", boardID)
	}
	return order, nil
}

// harness is the engine, the store, the fake Jira, the board order, and the
// handle underneath them. Every committer test builds one; the board tests
// are the ones that need all five.
type harness struct {
	eng   *committer.Engine
	repo  *issuerepo.Repository
	jira  *fake
	order *boardOrder
	db    *sql.DB
}

// newHarness seeds three cached issues: two in To Do with no sprint, and
// one Done in Sprint 12, which is what lets a move name its destination
// (the cache is where a status id's name comes from).
func newHarness(t *testing.T) harness {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := issuerepo.New(db.DB())
	f := newFake()
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", StatusID: "1", Priority: "Medium", Labels: []string{"a"}, StoryPoints: pts(3), Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeStory, Summary: "two", Status: "To Do", StatusID: "1", Labels: []string{}, Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-3", ID: "3", Project: "PLAT", Type: backend.TypeStory, Summary: "three", Status: "Done", StatusID: "5", Labels: []string{}, SprintID: "12", SprintName: "Sprint 12", Updated: "2026-09-01T00:00:00Z"},
	}
	for _, r := range rows {
		f.rows[r.Key] = r
	}
	f.desc["PLAT-1"] = "remote text"
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	order := newBoardOrder()
	return harness{eng: committer.New(f, repo, order), repo: repo, jira: f, order: order, db: db.DB()}
}

// The board writes the fake records. Transition and MoveIssuesToSprint
// rewrite the rows they touch, so a later remote read sees what landed.

func (f *fake) Transition(_ context.Context, key, targetStatusID string) error {
	if f.onTransition != nil {
		f.onTransition()
	}
	if err := f.transitionErr[key]; err != nil {
		return err
	}
	iss, ok := f.rows[key]
	if !ok {
		return fmt.Errorf("no issue %s", key)
	}
	iss.StatusID, iss.Status = targetStatusID, statusNames[targetStatusID]
	iss.Updated = "2026-09-07T00:00:00Z"
	f.rows[key] = iss
	f.pushed = append(f.pushed, "transition "+key+" "+targetStatusID)
	return nil
}

func (f *fake) CanTransition(_ context.Context, key, targetStatusID string) (backend.TransitionCheck, error) {
	return backend.TransitionCheck{Reachable: []string{statusNames[targetStatusID]}, Allowed: f.transitionErr[key] == nil}, nil
}

func (f *fake) RankIssue(_ context.Context, key, neighbourKey string, before bool) error {
	if err := f.rankErr[key]; err != nil {
		return err
	}
	side := "after"
	if before {
		side = "before"
	}
	f.pushed = append(f.pushed, "rank "+key+" "+side+" "+neighbourKey)
	return nil
}

func (f *fake) MoveIssuesToSprint(_ context.Context, sprintID string, keys []string) error {
	if f.sprintErr != nil {
		return f.sprintErr
	}
	for _, key := range keys {
		iss, ok := f.rows[key]
		if !ok {
			return fmt.Errorf("no issue %s", key)
		}
		iss.SprintID, iss.SprintName = sprintID, "Sprint "+sprintID
		if sprintID == "" {
			iss.SprintName = ""
		}
		iss.Updated = "2026-09-07T00:00:00Z"
		f.rows[key] = iss
	}
	f.pushed = append(f.pushed, "sprint "+sprintID+" "+strings.Join(keys, ","))
	return nil
}

func TestBoardMovesPushTheSprintThenTheTransitionThenTheRanks(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-1", "13", "Sprint 13"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-2", "PLAT-1", "PLAT-3"}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sprint 13 PLAT-1", "transition PLAT-1 5", "rank PLAT-1 after PLAT-2"}
	if strings.Join(h.jira.pushed, " | ") != strings.Join(want, " | ") {
		t.Errorf("push order: %v", h.jira.pushed)
	}
	if len(res.Moved) != 3 || len(res.Failures) != 0 || len(res.Conflicts) != 0 || res.Remaining != 0 {
		t.Fatalf("result: %+v", res)
	}
	byType := map[string]committer.Moved{}
	for _, m := range res.Moved {
		byType[m.EntityType] = m
	}
	if m := byType[issuerepo.EntitySprintMove]; m.Key != "PLAT-1" || m.Target != "Sprint 13" || m.Satisfied {
		t.Errorf("sprint move: %+v", m)
	}
	if m := byType[issuerepo.EntityTransition]; m.Target != "Done" {
		t.Errorf("transition: %+v", m)
	}
	if m := byType[issuerepo.EntityRank]; m.Target != "PLAT-2" || m.Side != issuerepo.RankSideAfter {
		t.Errorf("rank: %+v", m)
	}
	iss, _ := h.repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Pending || iss.StatusID != "5" || iss.SprintID != "13" {
		t.Errorf("the row was refreshed from Jira after the writes: %+v", iss)
	}
	act, _ := h.repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	commits := 0
	for _, a := range act {
		if a.Action == "commit" {
			commits++
		}
	}
	if commits != 3 {
		t.Errorf("one commit entry per board write: %+v", act)
	}
}

func TestABoardRowNeverReachesTheEditsPass(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.updates) != 1 || h.jira.updates[0] != "PLAT-1 summary=uno" {
		t.Errorf("the field update carries the edit and nothing else: %v", h.jira.updates)
	}
	for _, f := range res.Failures {
		if strings.Contains(f.Error, issuerepo.FieldStatusID) {
			t.Errorf("a board row reached commitEdit: %+v", res.Failures)
		}
	}
	if strings.Join(res.Committed, ",") != "PLAT-1" || len(res.Failures) != 0 || len(res.Moved) != 1 || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
}

// TestABoardRowSurvivesTheRegroupAfterACreate is the other half of that
// guard, and the half nothing exercised: regroupEdits re-lists the journal,
// but only when a commit carries both creates and edits, so a board row it
// failed to filter out would be swept into commitEdit, sent to Jira as a
// field, and deleted along with the edits it rode in with. This is the one
// commit shape that reaches that code.
func TestABoardRowSurvivesTheRegroupAfterACreate(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "New task"}); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 1 || h.jira.pushed[0] != "transition PLAT-1 5" {
		t.Fatalf("the board pass pushed the move, not the edits pass: %v", h.jira.pushed)
	}
	if len(h.jira.updates) != 1 || h.jira.updates[0] != "PLAT-1 summary=uno" {
		t.Errorf("the field update carries the edit and nothing else: %v", h.jira.updates)
	}
	if len(res.Created) != 1 || strings.Join(res.Committed, ",") != "PLAT-1" {
		t.Errorf("the create and the edit both landed: %+v", res)
	}
	if len(res.Moved) != 1 || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
}

func TestATransitionWithNoPathFailsNamesTheReachableStatusesAndLeavesTheRankAlone(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-2", "PLAT-1"}
	h.jira.transitionErr["PLAT-1"] = &backend.NoTransition{Key: "PLAT-1", TargetStatusID: "5", Reachable: []string{"In Progress"}}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("one failure, the transition's: %+v", res.Failures)
	}
	f := res.Failures[0]
	if f.Key != "PLAT-1" || f.EntityType != issuerepo.EntityTransition || f.RowID == 0 || f.Retryable {
		t.Errorf("failure names the row and is not retryable: %+v", f)
	}
	if strings.Join(f.Reachable, ",") != "In Progress" || !strings.Contains(f.Error, "In Progress") {
		t.Errorf("failure carries where the card can go: %+v", f)
	}
	// The banner shows this sentence word for word, so the target reads as
	// the status it is and not as the id the backend was handed.
	if !strings.Contains(f.Error, "cannot move to Done") || strings.Contains(f.Error, "status 5") {
		t.Errorf("failure names the target status: %q", f.Error)
	}
	if len(h.jira.pushed) != 1 || h.jira.pushed[0] != "rank PLAT-1 after PLAT-2" {
		t.Errorf("the rank still pushed: %v", h.jira.pushed)
	}
	pend, _ := h.repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(pend) != 1 || pend[0].EntityType != issuerepo.EntityTransition || res.Remaining != 1 {
		t.Errorf("only the transition row stayed: %+v (remaining %d)", pend, res.Remaining)
	}
}

func TestARemoteStatusAlreadyAtTheTargetDropsTheRowAsSatisfied(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	// Somebody moved the card on the web while the laptop was offline.
	remote := h.jira.rows["PLAT-1"]
	remote.StatusID, remote.Status, remote.Updated = "5", "Done", "2026-09-06T00:00:00Z"
	h.jira.rows["PLAT-1"] = remote

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 0 {
		t.Errorf("an outcome Jira already holds is not pushed again: %v", h.jira.pushed)
	}
	if len(res.Conflicts) != 0 || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Fatalf("satisfaction is not a conflict: %+v", res)
	}
	if len(res.Moved) != 1 || !res.Moved[0].Satisfied || res.Moved[0].Target != "Done" {
		t.Errorf("counted as satisfied: %+v", res.Moved)
	}
	// The trail has to say what happened. "commit" here would read as
	// "pushed the move to Done" for a push this Commit never made.
	entries, err := h.repo.ListActivity(ctx, "p1", "PLAT-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAction(entries, "satisfied") || hasAction(entries, "commit") {
		t.Errorf("audit of a satisfied move: %+v", entries)
	}
}

// hasAction says whether any audit entry carries that action.
func hasAction(entries []journal.AuditEntry, action string) bool {
	for _, a := range entries {
		if a.Action == action {
			return true
		}
	}
	return false
}

func TestARemoteStatusSomewhereElseHoldsTheIssueAsAConflict(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	remote := h.jira.rows["PLAT-1"]
	remote.StatusID, remote.Status = "3", "In Progress"
	h.jira.rows["PLAT-1"] = remote

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 0 {
		t.Errorf("a held card is not pushed: %v", h.jira.pushed)
	}
	if len(res.Conflicts) != 1 || len(res.Failures) != 0 || res.Remaining != 1 {
		t.Fatalf("result: %+v", res)
	}
	c := res.Conflicts[0]
	if c.Key != "PLAT-1" || c.Summary != "one" || len(c.Fields) != 1 {
		t.Fatalf("conflict: %+v", c)
	}
	if fc := c.Fields[0]; fc.Field != issuerepo.FieldStatusID || fc.Base != "To Do" || fc.Mine != "Done" || fc.Remote != "In Progress" {
		t.Errorf("the card carries before, target, and remote in words: %+v", fc)
	}
}

func TestASprintMoveIsCheckedAndBatchedByItsTarget(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for _, key := range []string{"PLAT-1", "PLAT-2"} {
		if err := h.repo.MoveToSprint(ctx, "p1", key, "13", "Sprint 13"); err != nil {
			t.Fatal(err)
		}
	}
	// PLAT-3 leaves its sprint for the backlog, which is a destination of
	// its own and so a batch of its own.
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-3", "", ""); err != nil {
		t.Fatal(err)
	}
	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sprint  PLAT-3", "sprint 13 PLAT-1,PLAT-2"}
	if strings.Join(h.jira.pushed, " | ") != strings.Join(want, " | ") {
		t.Errorf("one call per target: %v", h.jira.pushed)
	}
	if len(res.Moved) != 3 || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
	for _, m := range res.Moved {
		if m.Key == "PLAT-3" && m.Target != "Backlog" {
			t.Errorf("the backlog is named, not left blank: %+v", m)
		}
	}
}

func TestABoardRowUnderADraftKeyWaitsForTheNextCommit(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "New task"})
	if err != nil {
		t.Fatal(err)
	}
	// A drag on a draft moves it in place and journals no board row, so the
	// row this guards against has to be written by hand. It is the same
	// rule the link pass uses, and the cost of getting it wrong is a
	// TAM-NEW key sent to Jira as an issue key.
	if err := journal.Put(h.db, "p1", issuerepo.EntityTransition, temp, issuerepo.FieldStatusID,
		issuerepo.MoveValue("1", "To Do"), issuerepo.MoveValue("5", "Done"), ""); err != nil {
		t.Fatal(err)
	}
	h.jira.createErr = errors.New("POST failed: 400 Severity is required")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 0 {
		t.Errorf("a draft's board row must not be pushed: %v", h.jira.pushed)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != temp || !strings.Contains(res.Failures[0].Error, "Severity") {
		t.Errorf("only the create failed: %+v", res.Failures)
	}
	if res.Remaining != 2 {
		t.Errorf("the create row and the board row both stayed: %d", res.Remaining)
	}
}

func TestACardDraggedAgainMidPushKeepsTheNewerIntent(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	// The board takes no busy guard, so the card can be dragged again while
	// the pass is mid-push. That drag rewrites the same row.
	h.jira.onTransition = func() {
		if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
			t.Errorf("second drag: %v", err)
		}
	}
	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 1 || h.jira.pushed[0] != "transition PLAT-1 5" {
		t.Fatalf("the first intent was pushed: %v", h.jira.pushed)
	}
	pend, _ := h.repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(pend) != 1 || issuerepo.MoveID(pend[0].AfterVal) != "3" {
		t.Fatalf("the intent nobody sent is still journaled: %+v", pend)
	}
	if res.Remaining != 1 {
		t.Errorf("remaining: %d", res.Remaining)
	}
}

func TestRemainingCountsTheBoardRowsThatStayed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-1", "13", "Sprint 13"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-2", "5"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-3", "PLAT-2", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-2", "PLAT-3"}
	h.jira.transitionErr["PLAT-2"] = errors.New("POST failed: 503 service unavailable")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Moved) != 2 || len(res.Failures) != 1 {
		t.Fatalf("the sprint move and the rank landed, the transition did not: %+v", res)
	}
	if !res.Failures[0].Retryable {
		t.Errorf("a transport failure is worth retrying: %+v", res.Failures[0])
	}
	if res.Remaining != 1 {
		t.Errorf("one row stayed: %d", res.Remaining)
	}
}
