package boardrepo_test

import (
	"context"
	"fmt"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

// sprintDetailByName finds one node of a BoardSprintDetails answer by its
// Name, the same way laneByLabel finds a lane.
func sprintDetailByName(t *testing.T, details []boardrepo.SprintDetail, name string) boardrepo.SprintDetail {
	t.Helper()
	for _, d := range details {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("no node %q in %+v", name, details)
	return boardrepo.SprintDetail{}
}

func TestSprintDetailsOrderActiveThenFutureThenClosedDescendingWithUnassignedLast(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprints := []backend.Sprint{
		{ID: 21, BoardID: 1, Name: "Sprint 21", State: "closed", StartDate: "2026-07-01T09:00:00Z", EndDate: "2026-07-15T09:00:00Z"},
		{ID: 22, BoardID: 1, Name: "Sprint 22", State: "closed", StartDate: "2026-08-01T09:00:00Z", EndDate: "2026-08-15T09:00:00Z"},
		{ID: 23, BoardID: 1, Name: "Sprint 23", State: "future", StartDate: "2026-09-15T09:00:00Z", EndDate: "2026-09-29T09:00:00Z"},
		{ID: 24, BoardID: 1, Name: "Sprint 24", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
		{ID: 25, BoardID: 1, Name: "Sprint 25", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), sprints, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	details, err := r.BoardSprintDetails(ctx, newIssues(), "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	// Active first, then the future ones ascending by start date same as
	// the picker, then closed descending by start date, which
	// listSprintsSQL does not do, and last the unassigned node.
	want := []string{"Sprint 25", "Sprint 24", "Sprint 23", "Sprint 22", "Sprint 21", boardrepo.UnassignedSprintName}
	if len(details) != len(want) {
		t.Fatalf("details = %d nodes, want %d", len(details), len(want))
	}
	for i, name := range want {
		if details[i].Name != name {
			t.Errorf("node %d = %q, want %q", i, details[i].Name, name)
		}
	}
	last := details[len(details)-1]
	if last.State != boardrepo.UnassignedSprintState {
		t.Errorf("unassigned node state = %q, want %q", last.State, boardrepo.UnassignedSprintState)
	}
}

func TestSprintDetailsCountTotalsDoneAndPoints(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprint := backend.Sprint{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"}

	todo := card("PLAT-1", "To Do", "1")
	todo.StoryPoints = pts(3)
	done := card("PLAT-2", "Done", "5")
	done.StoryPoints = pts(5)
	doneNoPoints := card("PLAT-3", "Done", "5")

	keys := map[string][]string{"12": {"PLAT-1", "PLAT-2", "PLAT-3"}}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), []backend.Sprint{sprint}, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := newIssues(todo, done, doneNoPoints)
	details, err := r.BoardSprintDetails(ctx, src, "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	sp := sprintDetailByName(t, details, "Sprint 12")
	if sp.Total != 3 {
		t.Errorf("total = %d, want 3", sp.Total)
	}
	if sp.Done != 2 {
		t.Errorf("done = %d, want 2", sp.Done)
	}
	if sp.Points != 8 {
		t.Errorf("points = %v, want 8: the card with no estimate counts toward Total, not toward Points", sp.Points)
	}
	if sp.DonePoints != 5 {
		t.Errorf("done points = %v, want 5: only the done card that also carries an estimate", sp.DonePoints)
	}
}

func TestAClosedSprintReportsMembershipUncachedButActiveAndFutureDo(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprints := []backend.Sprint{
		{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z", EndDate: "2026-08-18T09:00:00Z"},
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), sprints, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	details, err := r.BoardSprintDetails(ctx, newIssues(), "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	// board_issue is empty for every sprint here, since none of them was
	// seeded any membership. A closed sprint's emptiness means the sync
	// never asked; an active or future sprint's would as easily mean a
	// failed sync as a genuinely empty one, which is exactly why
	// MembershipCached is read from state and not from board_issue.
	if sprintDetailByName(t, details, "Sprint 11").MembershipCached {
		t.Error("a closed sprint's membership is never fetched and must read uncached")
	}
	if !sprintDetailByName(t, details, "Sprint 12").MembershipCached {
		t.Error("an active sprint's membership is fetched and must read cached")
	}
	if !sprintDetailByName(t, details, "Sprint 13").MembershipCached {
		t.Error("a future sprint's membership is fetched and must read cached")
	}
	if !sprintDetailByName(t, details, boardrepo.UnassignedSprintName).MembershipCached {
		t.Error("the unassigned node's cards come from the board's own list, which is always cached")
	}
}

func TestACardJournaledIntoASprintAppearsThereAndNotInItsSource(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprints := []backend.Sprint{
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
	}
	// PLAT-1 is on sprint 12's own membership, the way the last boards sync
	// left it; the journal has since moved it to sprint 13, and board_issue
	// never learns about that until the next boards sync.
	keys := map[string][]string{
		"":   {"PLAT-1", "PLAT-2"},
		"12": {"PLAT-1"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), sprints, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := newIssues(card("PLAT-1", "To Do", "1"), card("PLAT-2", "To Do", "1")).
		withMoves(backend.PendingMove{Key: "PLAT-1", SprintID: "13", SprintName: "Sprint 13", HasSprint: true})
	details, err := r.BoardSprintDetails(ctx, src, "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	if got := cellKeys(sprintDetailByName(t, details, "Sprint 13").Issues); len(got) != 1 || got[0] != "PLAT-1" {
		t.Errorf("sprint 13 = %v, want the card journaled into it", got)
	}
	if got := cellKeys(sprintDetailByName(t, details, "Sprint 12").Issues); len(got) != 0 {
		t.Errorf("sprint 12 = %v, want the moved card gone from its source", got)
	}
	if got := cellKeys(sprintDetailByName(t, details, boardrepo.UnassignedSprintName).Issues); len(got) != 1 || got[0] != "PLAT-2" {
		t.Errorf("unassigned = %v, want only the card nothing claimed", got)
	}
}

func TestTheUnassignedNodeIsTheBoardsOwnListMinusEverySprintsScope(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprint := backend.Sprint{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"}
	// This is how readBoard actually seeds a board: the board's own list is
	// Jira's whole issue list, sprint issues included, so PLAT-1 sits in
	// both "" and "12". A fixture that only ever put a key in one scope or
	// the other would pass even if the subtraction were skipped entirely.
	keys := map[string][]string{
		"":   {"PLAT-1", "PLAT-2"},
		"12": {"PLAT-1"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), []backend.Sprint{sprint}, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := newIssues(card("PLAT-1", "To Do", "1"), card("PLAT-2", "To Do", "1"))
	details, err := r.BoardSprintDetails(ctx, src, "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	unassigned := sprintDetailByName(t, details, boardrepo.UnassignedSprintName)
	if got := cellKeys(unassigned.Issues); len(got) != 1 || got[0] != "PLAT-2" {
		t.Errorf("unassigned = %v, want only PLAT-2: PLAT-1 is on the board's own list and in sprint 12's scope and must not be drawn twice", got)
	}
	if unassigned.Total != 1 {
		t.Errorf("unassigned total = %d, want 1", unassigned.Total)
	}
	sp := sprintDetailByName(t, details, "Sprint 12")
	if got := cellKeys(sp.Issues); len(got) != 1 || got[0] != "PLAT-1" {
		t.Errorf("sprint 12 = %v, want PLAT-1", got)
	}
}

func TestTheCapIsSharedAcrossEveryNodeAndReportsTruncation(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprint := backend.Sprint{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"}

	const sprintCards = boardrepo.MaxCardsPerView - 10
	const unassignedCards = 50

	all := make([]backend.Issue, 0, sprintCards+unassignedCards)
	sprintKeys := make([]string, 0, sprintCards)
	for i := 0; i < sprintCards; i++ {
		c := card(fmt.Sprintf("PLAT-S-%d", i), "To Do", "1")
		all = append(all, c)
		sprintKeys = append(sprintKeys, c.Key)
	}
	boardKeys := append([]string(nil), sprintKeys...)
	for i := 0; i < unassignedCards; i++ {
		c := card(fmt.Sprintf("PLAT-U-%d", i), "To Do", "1")
		all = append(all, c)
		boardKeys = append(boardKeys, c.Key)
	}

	keys := map[string][]string{"": boardKeys, "12": sprintKeys}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), []backend.Sprint{sprint}, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := newIssues(all...)
	details, err := r.BoardSprintDetails(ctx, src, "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}

	sp := sprintDetailByName(t, details, "Sprint 12")
	if sp.Total != sprintCards || len(sp.Issues) != sprintCards {
		t.Errorf("sprint total = %d, rendered = %d, want both %d: the sprint alone does not reach the cap", sp.Total, len(sp.Issues), sprintCards)
	}
	if sp.Truncated {
		t.Error("the sprint alone did not reach the cap and must not report truncation")
	}

	unassigned := sprintDetailByName(t, details, boardrepo.UnassignedSprintName)
	if unassigned.Total != unassignedCards {
		t.Errorf("unassigned total = %d, want every unclaimed card counted: %d", unassigned.Total, unassignedCards)
	}
	if len(unassigned.Issues) != 10 {
		t.Errorf("unassigned rendered = %d, want the 10 cards left in the shared budget of %d", len(unassigned.Issues), boardrepo.MaxCardsPerView)
	}
	if !unassigned.Truncated {
		t.Error("the unassigned node inherited an already-spent cap from the sprint ahead of it and must report truncation")
	}
}

func TestACardJournaledOutOfASprintToTheBacklogLandsInUnassignedAndNotItsOldSprint(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprint := backend.Sprint{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"}
	// PLAT-1 is on sprint 12's own membership, the way the last boards sync
	// left it; the journal has since moved it out to the backlog (HasSprint
	// true, SprintID empty), and board_issue never learns about that until
	// the next boards sync. This is the case that tells apart building
	// claimed from the cards applyMoves has already replayed from building
	// it from the cards IssuesByKeys handed back: both orderings claim a
	// key moved between two sprints, but only the shipped order lets a key
	// moved out to the backlog land anywhere.
	keys := map[string][]string{
		"":   {"PLAT-1"},
		"12": {"PLAT-1"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), []backend.Sprint{sprint}, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := newIssues(card("PLAT-1", "To Do", "1")).
		withMoves(backend.PendingMove{Key: "PLAT-1", HasSprint: true, SprintID: "", SprintName: ""})
	details, err := r.BoardSprintDetails(ctx, src, "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	if got := cellKeys(sprintDetailByName(t, details, "Sprint 12").Issues); len(got) != 0 {
		t.Errorf("sprint 12 = %v, want the card gone: the journal moved it out to the backlog", got)
	}
	if got := cellKeys(sprintDetailByName(t, details, boardrepo.UnassignedSprintName).Issues); len(got) != 1 || got[0] != "PLAT-1" {
		t.Errorf("unassigned = %v, want the card the journal moved out of its sprint, not lost entirely", got)
	}
}

func TestTheUnassignedNodesPendingTransitionCountsTowardDoneAndPoints(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	// No sprint claims PLAT-9, so it can only ever be drawn by the
	// unassigned node's own applyMoves pass, not by a sprint's.
	c := card("PLAT-9", "To Do", "1")
	c.StoryPoints = pts(3)
	keys := map[string][]string{"": {"PLAT-9"}}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), nil, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := newIssues(c).
		withMoves(backend.PendingMove{Key: "PLAT-9", HasTransition: true, StatusID: "5", StatusName: "Done"})
	details, err := r.BoardSprintDetails(ctx, src, "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	unassigned := sprintDetailByName(t, details, boardrepo.UnassignedSprintName)
	if unassigned.Done != 1 {
		t.Errorf("done = %d, want 1: the pending transition to Done must be replayed onto the unassigned node too", unassigned.Done)
	}
	if unassigned.DonePoints != 3 {
		t.Errorf("done points = %v, want 3", unassigned.DonePoints)
	}
	if got := cellKeys(unassigned.Issues); len(got) != 1 || got[0] != "PLAT-9" {
		t.Errorf("unassigned issues = %v, want PLAT-9", got)
	}
	if unassigned.Issues[0].Status != "Done" {
		t.Errorf("rendered status = %q, want the journaled name Done", unassigned.Issues[0].Status)
	}
}

func TestNotSyncedCountsAScopesKeysTheIssueCacheIsMissingRatherThanDroppingThemSilently(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprint := backend.Sprint{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"}
	// Sprint 12's own membership and the board's own list are kept disjoint
	// on purpose, so each node's NotSynced can be checked against a key
	// that scope alone holds, the way a board synced before its issues
	// finished syncing looks: PLAT-2 and PLAT-6 are both names Jira handed
	// back that the issue cache has no row for yet.
	keys := map[string][]string{
		"":   {"PLAT-5", "PLAT-6"},
		"12": {"PLAT-1", "PLAT-2"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), []backend.Sprint{sprint}, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := newIssues(card("PLAT-1", "To Do", "1"), card("PLAT-5", "To Do", "1"))
	details, err := r.BoardSprintDetails(ctx, src, "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	sp := sprintDetailByName(t, details, "Sprint 12")
	if sp.NotSynced != 1 {
		t.Errorf("sprint 12 not synced = %d, want 1: PLAT-2 is in its scope but the cache has no row for it", sp.NotSynced)
	}
	if sp.Total != 1 {
		t.Errorf("sprint 12 total = %d, want 1: PLAT-2 must not be counted as present", sp.Total)
	}
	unassigned := sprintDetailByName(t, details, boardrepo.UnassignedSprintName)
	if unassigned.NotSynced != 1 {
		t.Errorf("unassigned not synced = %d, want 1: PLAT-6 is on the board's own list but the cache has no row for it", unassigned.NotSynced)
	}
	if unassigned.Total != 1 {
		t.Errorf("unassigned total = %d, want 1: PLAT-6 must not be counted as present", unassigned.Total)
	}
}
