package boardrepo_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

// staticIssues is the issue cache the view reads through: a fixed set of
// rows, answering in the caller's order and skipping what it does not hold,
// exactly as issuerepo.IssuesByKeys does, plus the profile's drafts, which
// issuerepo answers from the cache and not from any key list. asked records
// every key the board looked up, so a test can prove where a card came
// from.
type staticIssues struct {
	byKey  map[string]backend.Issue
	drafts []backend.Issue
	moves  []backend.PendingMove
	asked  *[]string
}

func newIssues(issues ...backend.Issue) staticIssues {
	s := staticIssues{byKey: map[string]backend.Issue{}, asked: &[]string{}}
	for _, iss := range issues {
		s.byKey[iss.Key] = iss
	}
	return s
}

// withDrafts hands the source the profile's local drafts.
func (s staticIssues) withDrafts(drafts ...backend.Issue) staticIssues {
	s.drafts = drafts
	return s
}

func (s staticIssues) IssuesByKeys(_ context.Context, _ string, keys []string) ([]backend.Issue, error) {
	*s.asked = append(*s.asked, keys...)
	out := []backend.Issue{}
	for _, k := range keys {
		if iss, ok := s.byKey[k]; ok {
			out = append(out, iss)
		}
	}
	return out, nil
}

func (s staticIssues) DraftIssues(_ context.Context, _ string) ([]backend.Issue, error) {
	return s.drafts, nil
}

// withMoves hands the source the board intents the journal holds.
func (s staticIssues) withMoves(moves ...backend.PendingMove) staticIssues {
	s.moves = moves
	return s
}

func (s staticIssues) PendingMoves(_ context.Context, _ string) ([]backend.PendingMove, error) {
	return s.moves, nil
}

func pts(v float64) *float64 { return &v }

// card is one board card: key, status id, and whatever else a test needs.
func card(key, statusName, statusID string) backend.Issue {
	return backend.Issue{Key: key, Summary: key, Status: statusName, StatusID: statusID, Labels: []string{}}
}

// draftCard is one local draft: the status a draft row shows, no status id,
// and the flag scanIssue sets from the key prefix.
func draftCard(key string) backend.Issue {
	c := card(key, "Draft", "")
	c.Draft = true
	return c
}

// seedScopes writes board 1 with the columns and the membership of each
// scope given, the way one boards pass writes a board.
func seedScopes(t *testing.T, r *boardrepo.Repository, cols []backend.BoardColumn, keys map[string][]string) {
	t.Helper()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	if err := r.ReplaceBoard(context.Background(), "p1", board, cols, nil, keys); err != nil {
		t.Fatalf("seed board: %v", err)
	}
}

// seedBoard writes the sample columns for board 1 and the keys in the order
// given, and returns a source holding the cards.
func seedBoard(t *testing.T, r *boardrepo.Repository, cols []backend.BoardColumn, cards []backend.Issue) staticIssues {
	t.Helper()
	keys := make([]string, 0, len(cards))
	for _, c := range cards {
		keys = append(keys, c.Key)
	}
	seedScopes(t, r, cols, map[string][]string{"": keys})
	return newIssues(cards...)
}

// laneByLabel finds one lane of a view.
func laneByLabel(t *testing.T, view boardrepo.BoardView, label string) boardrepo.LaneView {
	t.Helper()
	for _, l := range view.Lanes {
		if l.Label == label {
			return l
		}
	}
	t.Fatalf("no lane %q in %+v", label, view.Lanes)
	return boardrepo.LaneView{}
}

// cellKeys is the keys one cell renders.
func cellKeys(cell []backend.Issue) []string {
	out := make([]string, 0, len(cell))
	for _, c := range cell {
		out = append(out, c.Key)
	}
	return out
}

func TestBoardJoinsCardsToColumnsByStatusID(t *testing.T) {
	r, _ := newRepo(t)
	cards := []backend.Issue{
		card("PLAT-409", "To Do", "1"),
		card("PLAT-412", "In Progress", "3"),
		card("PLAT-401", "In Review", "4"), // the second id of the same column
		card("PLAT-331", "Done", "5"),
	}
	src := seedBoard(t, r, sampleColumns(), cards)
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if len(view.Columns) != 5 || view.Columns[0].Name != "Backlog" {
		t.Fatalf("columns = %+v", view.Columns)
	}
	if len(view.Lanes) != 1 {
		t.Fatalf("lanes = %+v, want one for the none swimlane", view.Lanes)
	}
	lane := view.Lanes[0]
	if got := cellKeys(lane.Cells[1]); len(got) != 1 || got[0] != "PLAT-409" {
		t.Errorf("To Do = %v", got)
	}
	if got := cellKeys(lane.Cells[2]); len(got) != 2 || got[0] != "PLAT-412" || got[1] != "PLAT-401" {
		t.Errorf("In Progress = %v, want both status ids of the column in board order", got)
	}
	if got := cellKeys(lane.Cells[4]); len(got) != 1 || got[0] != "PLAT-331" {
		t.Errorf("Done = %v", got)
	}
	if len(lane.Cells[0]) != 0 {
		t.Errorf("Backlog collects no status and must stay empty: %v", cellKeys(lane.Cells[0]))
	}
	if lane.Count != 4 {
		t.Errorf("lane count = %d, want 4", lane.Count)
	}
	if view.Unmapped != 0 || view.NotSynced != 0 || view.Capped || view.NeedsStatusSync {
		t.Errorf("view counters = %+v", view)
	}
}

func TestBoardCountsAStatusNoColumnCollects(t *testing.T) {
	r, _ := newRepo(t)
	cards := []backend.Issue{
		card("PLAT-409", "To Do", "1"),
		card("PLAT-388", "Approved", "10099"),
		card("PLAT-395", "Approved", "10099"),
		card("PLAT-390", "Blocked", "10098"),
	}
	src := seedBoard(t, r, sampleColumns(), cards)
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if view.Unmapped != 3 {
		t.Errorf("unmapped = %d, want 3", view.Unmapped)
	}
	if len(view.UnmappedStatuses) != 2 || view.UnmappedStatuses[0] != "Approved" || view.UnmappedStatuses[1] != "Blocked" {
		t.Errorf("unmapped statuses = %v, want them deduplicated and sorted", view.UnmappedStatuses)
	}
	// An unmapped card is drawn nowhere and counted in no column.
	drawn := 0
	total := 0
	for _, l := range view.Lanes {
		for _, cell := range l.Cells {
			drawn += len(cell)
		}
	}
	for _, c := range view.Columns {
		total += c.Total
	}
	if drawn != 1 || total != 1 {
		t.Errorf("drawn = %d, column totals = %d; want only the mapped card", drawn, total)
	}
}

func TestADraftLandsInTheFirstColumnThatHasStatusIDs(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	// The scrum board's sprint and the kanban board's whole-board list are
	// two different key lists, and neither of them names the draft: the
	// draft is project-level and must appear on both.
	seedScopes(t, r, sampleColumns(), map[string][]string{"12": {"PLAT-409"}, "": {"PLAT-409"}})
	src := newIssues(card("PLAT-409", "To Do", "1")).withDrafts(draftCard("TAM-NEW-1"))

	for _, tc := range []struct {
		name     string
		sprintID string
	}{
		{"scrum sprint", "12"},
		{"kanban board", ""},
	} {
		view, err := r.Board(ctx, src, "p1", 1, tc.sprintID, boardrepo.SwimlaneNone)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		lane := view.Lanes[0]
		if len(lane.Cells[0]) != 0 {
			t.Errorf("%s: the leading Backlog column has no statuses and must not take the draft: %v", tc.name, cellKeys(lane.Cells[0]))
		}
		if got := cellKeys(lane.Cells[1]); len(got) != 2 || got[1] != "TAM-NEW-1" {
			t.Errorf("%s: To Do = %v, want the draft in the first column that has status ids", tc.name, got)
		}
		if view.Unmapped != 0 {
			t.Errorf("%s: unmapped = %d, want the draft drawn rather than hidden", tc.name, view.Unmapped)
		}
		if view.Columns[1].Total != 2 {
			t.Errorf("%s: To Do total = %d, want the draft counted with the card", tc.name, view.Columns[1].Total)
		}
		if lane.Count != 2 {
			t.Errorf("%s: lane count = %d, want the draft in the lane too", tc.name, lane.Count)
		}
	}
}

func TestADraftDoesNotArriveThroughIssuesByKeys(t *testing.T) {
	r, _ := newRepo(t)
	src := seedBoard(t, r, sampleColumns(), []backend.Issue{card("PLAT-409", "To Do", "1")}).
		withDrafts(draftCard("TAM-NEW-1"))
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if got := cellKeys(view.Lanes[0].Cells[1]); len(got) != 2 {
		t.Fatalf("To Do = %v, want the card and the draft", got)
	}
	// board_issue is filled from Jira's board issue list, so a draft key can
	// never be in it. The draft reached the board anyway, and not by being
	// looked up as one of the board's keys.
	for _, k := range *src.asked {
		if strings.HasPrefix(k, "TAM-NEW-") {
			t.Errorf("the board asked IssuesByKeys for %s; drafts come from DraftIssues", k)
		}
	}
	if view.NotSynced != 0 {
		t.Errorf("not synced = %d, want the draft not to count as a key with no row", view.NotSynced)
	}
}

func TestADraftIsUnmappedWhenNoColumnHasAStatus(t *testing.T) {
	r, _ := newRepo(t)
	cols := []backend.BoardColumn{{Name: "Backlog", StatusIDs: []string{}}, {Name: "Later", StatusIDs: []string{}}}
	src := seedBoard(t, r, cols, nil).withDrafts(draftCard("TAM-NEW-1"))
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if view.Unmapped != 1 {
		t.Errorf("unmapped = %d, want the draft counted: the board has nowhere to put it", view.Unmapped)
	}
	if len(view.UnmappedStatuses) != 1 || view.UnmappedStatuses[0] != "Draft" {
		t.Errorf("unmapped statuses = %v", view.UnmappedStatuses)
	}
}

func TestACardWithNoStatusIDIsUnmappedUnlessItIsADraft(t *testing.T) {
	r, _ := newRepo(t)
	// Right after the version 4 migration every cached row carries an empty
	// status id. None of them is a draft, and reading them as drafts would
	// pile the whole backlog into the first column.
	cards := []backend.Issue{card("PLAT-409", "", ""), card("PLAT-412", "To Do", "1")}
	src := seedBoard(t, r, sampleColumns(), cards)
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if view.Unmapped != 1 {
		t.Errorf("unmapped = %d, want the card with no status id counted", view.Unmapped)
	}
	if len(view.UnmappedStatuses) != 0 {
		t.Errorf("unmapped statuses = %v, want an empty status to name nothing", view.UnmappedStatuses)
	}
	lane := view.Lanes[0]
	if got := cellKeys(lane.Cells[1]); len(got) != 1 || got[0] != "PLAT-412" {
		t.Errorf("To Do = %v, want only the card a column collects", got)
	}
	if view.Columns[0].Total != 0 || view.Columns[1].Total != 1 {
		t.Errorf("column totals = %d and %d, want the unplaced card in neither", view.Columns[0].Total, view.Columns[1].Total)
	}
	if lane.Count != 1 {
		t.Errorf("lane count = %d, want only the placed card", lane.Count)
	}
}

func TestNeedsStatusSyncOnlyWhenNoCardHasAStatusID(t *testing.T) {
	r, _ := newRepo(t)
	cards := []backend.Issue{card("PLAT-409", "To Do", ""), card("PLAT-412", "In Progress", "")}
	src := seedBoard(t, r, sampleColumns(), cards)
	ctx := context.Background()
	view, err := r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if !view.NeedsStatusSync {
		t.Error("every cached card has an empty status id, so the view must ask for a sync")
	}
	cards[1].StatusID = "3"
	src = seedBoard(t, r, sampleColumns(), cards)
	view, err = r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if view.NeedsStatusSync {
		t.Error("one card carries a status id, so the sync has run")
	}
	// An empty board is not a board waiting for a sync.
	empty := seedBoard(t, r, sampleColumns(), nil)
	view, err = r.Board(ctx, empty, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if view.NeedsStatusSync {
		t.Error("a board with no cards must not ask for a sync")
	}
}

func TestBoardKeysWithNoCachedIssueCountAsNotSynced(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	// The board's filter reaches into another project; those keys were
	// never synced into this profile's cache.
	seedScopes(t, r, sampleColumns(), map[string][]string{"": {"PLAT-409", "OPS-7", "OPS-9"}})
	src := newIssues(card("PLAT-409", "To Do", "1"))
	view, err := r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if view.NotSynced != 2 {
		t.Errorf("not synced = %d, want the two keys the cache does not hold", view.NotSynced)
	}
	if view.Unmapped != 0 {
		t.Errorf("unmapped = %d: a key with no row is not an unmapped status", view.Unmapped)
	}
	if view.Columns[1].Total != 1 {
		t.Errorf("To Do total = %d, want the one card that is cached", view.Columns[1].Total)
	}
}

func TestACellRendersTwoHundredCardsAndReportsTheRest(t *testing.T) {
	r, _ := newRepo(t)
	cards := make([]backend.Issue, 0, 250)
	for i := 0; i < 250; i++ {
		c := card(fmt.Sprintf("PLAT-%d", 1000+i), "To Do", "1")
		c.StoryPoints = pts(2)
		cards = append(cards, c)
	}
	src := seedBoard(t, r, sampleColumns(), cards)
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	lane := view.Lanes[0]
	if len(lane.Cells[1]) != boardrepo.MaxCardsPerCell {
		t.Errorf("rendered %d cards, want %d", len(lane.Cells[1]), boardrepo.MaxCardsPerCell)
	}
	if lane.Overflow[1] != 50 {
		t.Errorf("overflow = %d, want 50", lane.Overflow[1])
	}
	if lane.Count != 250 {
		t.Errorf("lane count = %d, want every card", lane.Count)
	}
	if view.Columns[1].Total != 250 {
		t.Errorf("column total = %d, want every card whether drawn or not", view.Columns[1].Total)
	}
	if view.Columns[1].Points != 500 {
		t.Errorf("column points = %v, want 500", view.Columns[1].Points)
	}
	if view.Capped {
		t.Error("a full cell is not the view cap")
	}
}

func TestTheViewCapBoundsTheWholePayload(t *testing.T) {
	r, _ := newRepo(t)
	// Fifteen assignee lanes with a full cell each: the per-cell cap alone
	// would render three thousand cards.
	cards := []backend.Issue{}
	for lane := 0; lane < 15; lane++ {
		for i := 0; i < boardrepo.MaxCardsPerCell; i++ {
			c := card(fmt.Sprintf("PLAT-%d-%d", lane, i), "To Do", "1")
			c.Assignee = fmt.Sprintf("Dev %02d", lane)
			cards = append(cards, c)
		}
	}
	src := seedBoard(t, r, sampleColumns(), cards)
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneAssignee)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if !view.Capped {
		t.Error("the view cap was reached and must be reported")
	}
	drawn, counted, overflow := 0, 0, 0
	for _, l := range view.Lanes {
		counted += l.Count
		for i, cell := range l.Cells {
			drawn += len(cell)
			overflow += l.Overflow[i]
		}
	}
	if drawn != boardrepo.MaxCardsPerView {
		t.Errorf("drew %d cards, want the view cap of %d", drawn, boardrepo.MaxCardsPerView)
	}
	if counted != len(cards) || view.Columns[1].Total != len(cards) {
		t.Errorf("counted %d and the column says %d, want every one of the %d cards", counted, view.Columns[1].Total, len(cards))
	}
	if drawn+overflow != len(cards) {
		t.Errorf("drawn %d plus overflow %d, want %d", drawn, overflow, len(cards))
	}
}

func TestSwimlanesGroupTheCardsAndPutTheEmptyValueLast(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	mk := func(key, assignee, epic string) backend.Issue {
		c := card(key, "To Do", "1")
		c.Assignee, c.ParentKey = assignee, epic
		return c
	}
	cards := []backend.Issue{
		mk("PLAT-1", "S. Kim", "PLAT-320"),
		mk("PLAT-2", "", ""),
		mk("PLAT-3", "M. Ortiz", "PLAT-310"),
		mk("PLAT-4", "S. Kim", "PLAT-310"),
	}
	src := seedBoard(t, r, sampleColumns(), cards)

	none, err := r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("none: %v", err)
	}
	if len(none.Lanes) != 1 || none.Lanes[0].Count != 4 {
		t.Errorf("none lanes = %+v, want one lane of four", none.Lanes)
	}

	byAssignee, err := r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneAssignee)
	if err != nil {
		t.Fatalf("assignee: %v", err)
	}
	labels := []string{}
	for _, l := range byAssignee.Lanes {
		labels = append(labels, l.Label)
	}
	want := []string{"M. Ortiz", "S. Kim", "Unassigned"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Errorf("assignee lanes = %v, want %v with the empty value last", labels, want)
	}
	if laneByLabel(t, byAssignee, "S. Kim").Count != 2 {
		t.Errorf("S. Kim lane = %+v", laneByLabel(t, byAssignee, "S. Kim"))
	}
	if got := cellKeys(laneByLabel(t, byAssignee, "Unassigned").Cells[1]); len(got) != 1 || got[0] != "PLAT-2" {
		t.Errorf("Unassigned cell = %v", got)
	}

	byEpic, err := r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneEpic)
	if err != nil {
		t.Fatalf("epic: %v", err)
	}
	labels = labels[:0]
	for _, l := range byEpic.Lanes {
		labels = append(labels, l.Label)
	}
	want = []string{"PLAT-310", "PLAT-320", "No epic"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Errorf("epic lanes = %v, want %v", labels, want)
	}
	if laneByLabel(t, byEpic, "PLAT-310").ID != "PLAT-310" {
		t.Error("a lane carries its raw grouping value as well as its label")
	}
}

func TestColumnTotalsAndPointSums(t *testing.T) {
	r, _ := newRepo(t)
	mk := func(key, statusID string, points *float64) backend.Issue {
		c := card(key, "status "+statusID, statusID)
		c.StoryPoints = points
		return c
	}
	cards := []backend.Issue{
		mk("PLAT-1", "1", pts(5)),
		mk("PLAT-2", "1", pts(3)),
		mk("PLAT-3", "1", nil), // no estimate: it counts, it adds nothing
		mk("PLAT-4", "3", pts(8)),
	}
	src := seedBoard(t, r, sampleColumns(), cards)
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneAssignee)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if view.Columns[1].Total != 3 || view.Columns[1].Points != 8 {
		t.Errorf("To Do = %d cards, %v points; want 3 and 8", view.Columns[1].Total, view.Columns[1].Points)
	}
	if view.Columns[2].Total != 1 || view.Columns[2].Points != 8 {
		t.Errorf("In Progress = %d cards, %v points", view.Columns[2].Total, view.Columns[2].Points)
	}
	if view.Columns[4].Total != 0 || view.Columns[4].Points != 0 {
		t.Errorf("Done = %+v, want an empty column", view.Columns[4])
	}
}

func TestAnUnknownSwimlaneIsAnError(t *testing.T) {
	r, _ := newRepo(t)
	src := seedBoard(t, r, sampleColumns(), nil)
	_, err := r.Board(context.Background(), src, "p1", 1, "", "priority")
	if err == nil {
		t.Fatal("an unknown swimlane must fail")
	}
	for _, want := range []string{"none", "assignee", "epic"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

func TestABoardWithNoColumnsIsAnEmptyViewNotAnError(t *testing.T) {
	r, _ := newRepo(t)
	src := newIssues(card("PLAT-409", "To Do", "1"))
	view, err := r.Board(context.Background(), src, "p1", 7, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("a board with nothing cached must not fail: %v", err)
	}
	if len(view.Columns) != 0 || len(view.Lanes) != 0 {
		t.Errorf("view = %+v, want it empty", view)
	}
	if view.UnmappedStatuses == nil {
		t.Error("the empty view still carries an empty status list, not nil")
	}
	if view.BoardID != 7 || view.Swimlane != boardrepo.SwimlaneNone {
		t.Errorf("view = %+v, want the request echoed back", view)
	}
}

func TestTheSprintScopeIsItsOwnMembership(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	seedScopes(t, r, sampleColumns(), map[string][]string{
		"":   {"PLAT-1", "PLAT-2", "PLAT-3"},
		"12": {"PLAT-3", "PLAT-1"},
	})
	src := newIssues(card("PLAT-1", "To Do", "1"), card("PLAT-2", "To Do", "1"), card("PLAT-3", "To Do", "1"))

	whole, err := r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("whole board: %v", err)
	}
	if whole.Columns[1].Total != 3 {
		t.Errorf("whole board = %d cards, want 3", whole.Columns[1].Total)
	}
	sprint, err := r.Board(ctx, src, "p1", 1, "12", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("sprint: %v", err)
	}
	if got := cellKeys(sprint.Lanes[0].Cells[1]); len(got) != 2 || got[0] != "PLAT-3" || got[1] != "PLAT-1" {
		t.Errorf("sprint 12 = %v, want the sprint's own cards in the board's order", got)
	}
	if sprint.SprintID != "12" {
		t.Errorf("view sprint = %q", sprint.SprintID)
	}
}

func TestDonePointsCountEveryCardInTheColumnNotJustTheDrawnOnes(t *testing.T) {
	r, _ := newRepo(t)
	cards := make([]backend.Issue, 0, 250)
	for i := 0; i < 250; i++ {
		c := card(fmt.Sprintf("PLAT-%d", 1000+i), "Done", "5")
		c.StoryPoints = pts(2)
		cards = append(cards, c)
	}
	src := seedBoard(t, r, sampleColumns(), cards)
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if len(view.Lanes[0].Cells[4]) != boardrepo.MaxCardsPerCell {
		t.Fatalf("rendered %d cards, want the cell capped so the count is worth making", len(view.Lanes[0].Cells[4]))
	}
	// The column head reads 500 points, and the done half has to agree with
	// it: a walk over the drawn cards would say 400.
	if view.Columns[4].Points != 500 {
		t.Errorf("Done column points = %v, want 500", view.Columns[4].Points)
	}
	if view.DonePoints != 500 {
		t.Errorf("done points = %v, want 500: the cap decides what is drawn, not what is counted", view.DonePoints)
	}
}

func TestDonePointsFollowTheStatusRatherThanTheLastColumn(t *testing.T) {
	r, _ := newRepo(t)
	cols := []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "In Progress", StatusIDs: []string{"3"}},
		{Name: "Blocked", StatusIDs: []string{"9"}},
	}
	todo := card("PLAT-1", "To Do", "1")
	todo.StoryPoints = pts(3)
	// A board's rightmost column is as often Blocked as it is Done, and a
	// done card can sit anywhere: this one is still in the middle column.
	done := card("PLAT-2", "Done", "3")
	done.StoryPoints = pts(5)
	blocked := card("PLAT-3", "Blocked", "9")
	blocked.StoryPoints = pts(3)

	src := seedBoard(t, r, cols, []backend.Issue{todo, done, blocked})
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if view.DonePoints != 5 {
		t.Errorf("done points = %v, want the 5 of the one card whose status is done", view.DonePoints)
	}
	total := 0.0
	for _, c := range view.Columns {
		total += c.Points
	}
	if total != 11 {
		t.Errorf("column points total = %v, want 11", total)
	}
}

func TestAPendingTransitionDrawsTheCardInItsTargetColumn(t *testing.T) {
	r, _ := newRepo(t)
	cards := []backend.Issue{card("PLAT-409", "To Do", "1"), card("PLAT-412", "To Do", "1")}
	src := seedBoard(t, r, sampleColumns(), cards).
		withMoves(backend.PendingMove{Key: "PLAT-412", StatusID: "3", HasTransition: true})
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	lane := view.Lanes[0]
	if got := cellKeys(lane.Cells[1]); len(got) != 1 || got[0] != "PLAT-409" {
		t.Errorf("To Do = %v, want only the card that did not move", got)
	}
	if got := cellKeys(lane.Cells[2]); len(got) != 1 || got[0] != "PLAT-412" {
		t.Errorf("In Progress = %v, want the moved card drawn where it was dropped", got)
	}
	if view.Columns[1].Total != 1 || view.Columns[2].Total != 1 {
		t.Errorf("column totals = %d and %d, want the move counted in the target", view.Columns[1].Total, view.Columns[2].Total)
	}
}

func TestACardMovedIntoTheSprintIsDrawnThoughTheMembershipPredatesIt(t *testing.T) {
	r, _ := newRepo(t)
	// The sprint's membership is Jira's own from the last sync, so the card
	// that just moved in is not in it. Unioning the pending move's key into
	// the list before the cards are read is the only place this can be
	// rescued.
	seedScopes(t, r, sampleColumns(), map[string][]string{
		"":   {"PLAT-1", "PLAT-2"},
		"12": {"PLAT-1"},
	})
	src := newIssues(card("PLAT-1", "To Do", "1"), card("PLAT-2", "To Do", "1")).
		withMoves(backend.PendingMove{Key: "PLAT-2", SprintID: "12", HasSprint: true})
	view, err := r.Board(context.Background(), src, "p1", 1, "12", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	got := cellKeys(view.Lanes[0].Cells[1])
	if len(got) != 2 || got[0] != "PLAT-1" || got[1] != "PLAT-2" {
		t.Errorf("sprint 12 = %v, want the card that was moved into it as well", got)
	}
	if view.NotSynced != 0 {
		t.Errorf("not synced = %d: a key added by a pending move is not a board key with no row", view.NotSynced)
	}
	if view.Columns[1].Total != 2 {
		t.Errorf("To Do total = %d, want both cards", view.Columns[1].Total)
	}
}

func TestACardMovedOutOfTheSprintLeavesIt(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	seedScopes(t, r, sampleColumns(), map[string][]string{
		"":   {"PLAT-1", "PLAT-2"},
		"12": {"PLAT-1", "PLAT-2"},
	})
	src := newIssues(card("PLAT-1", "To Do", "1"), card("PLAT-2", "To Do", "1")).
		withMoves(backend.PendingMove{Key: "PLAT-1", SprintID: "13", HasSprint: true})

	sprint, err := r.Board(ctx, src, "p1", 1, "12", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("sprint: %v", err)
	}
	if got := cellKeys(sprint.Lanes[0].Cells[1]); len(got) != 1 || got[0] != "PLAT-2" {
		t.Errorf("sprint 12 = %v, want the card that was moved to sprint 13 gone", got)
	}
	if sprint.Columns[1].Total != 1 {
		t.Errorf("To Do total = %d, want the card that left uncounted", sprint.Columns[1].Total)
	}

	// The whole board has no sprint to leave, so nothing is dropped from it.
	whole, err := r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("whole board: %v", err)
	}
	if got := cellKeys(whole.Lanes[0].Cells[1]); len(got) != 2 {
		t.Errorf("whole board = %v, want both cards", got)
	}
}

func TestAPendingRankPlacesTheCardBesideItsNeighbour(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	cards := []backend.Issue{
		card("PLAT-1", "To Do", "1"),
		card("PLAT-2", "To Do", "1"),
		card("PLAT-3", "To Do", "1"),
	}
	for _, tc := range []struct {
		name   string
		before bool
		want   []string
	}{
		{"before", true, []string{"PLAT-3", "PLAT-1", "PLAT-2"}},
		{"after", false, []string{"PLAT-1", "PLAT-3", "PLAT-2"}},
	} {
		src := seedBoard(t, r, sampleColumns(), cards).withMoves(backend.PendingMove{
			Key: "PLAT-3", RankNeighbour: "PLAT-1", RankBefore: tc.before, HasRank: true,
		})
		view, err := r.Board(ctx, src, "p1", 1, "", boardrepo.SwimlaneNone)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := cellKeys(view.Lanes[0].Cells[1])
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s: To Do = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestARankAgainstANeighbourInAnotherCellLeavesTheOrderAlone(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	mk := func(key, statusID, assignee string) backend.Issue {
		c := card(key, "status "+statusID, statusID)
		c.Assignee = assignee
		return c
	}
	cards := []backend.Issue{
		mk("PLAT-1", "1", "S. Kim"),
		mk("PLAT-2", "1", "S. Kim"),
		mk("PLAT-9", "3", "S. Kim"), // another column
		mk("PLAT-7", "1", "M. Ortiz"),
	}
	for _, tc := range []struct {
		name      string
		neighbour string
		swimlane  string
	}{
		{"another column", "PLAT-9", boardrepo.SwimlaneNone},
		{"another lane", "PLAT-7", boardrepo.SwimlaneAssignee},
		{"a card the board does not hold", "PLAT-404", boardrepo.SwimlaneNone},
	} {
		src := seedBoard(t, r, sampleColumns(), cards).withMoves(backend.PendingMove{
			Key: "PLAT-2", RankNeighbour: tc.neighbour, RankBefore: true, HasRank: true,
		})
		view, err := r.Board(ctx, src, "p1", 1, "", tc.swimlane)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		lane := view.Lanes[0]
		if tc.swimlane == boardrepo.SwimlaneAssignee {
			lane = laneByLabel(t, view, "M. Ortiz")
			if got := cellKeys(lane.Cells[1]); len(got) != 1 || got[0] != "PLAT-7" {
				t.Errorf("%s: the neighbour's lane = %v, want it untouched", tc.name, got)
			}
			lane = laneByLabel(t, view, "S. Kim")
		}
		got := cellKeys(lane.Cells[1])
		if len(got) < 2 || got[0] != "PLAT-1" || got[1] != "PLAT-2" {
			t.Errorf("%s: To Do = %v, want the board's own order kept", tc.name, got)
		}
	}
}

func TestAPendingMoveDoesNotDisturbACardWithoutOne(t *testing.T) {
	r, _ := newRepo(t)
	cards := []backend.Issue{card("PLAT-1", "To Do", "1"), card("PLAT-2", "Done", "5")}
	src := seedBoard(t, r, sampleColumns(), cards).withMoves(
		backend.PendingMove{Key: "PLAT-404", StatusID: "3", HasTransition: true})
	view, err := r.Board(context.Background(), src, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	lane := view.Lanes[0]
	if got := cellKeys(lane.Cells[1]); len(got) != 1 || got[0] != "PLAT-1" {
		t.Errorf("To Do = %v", got)
	}
	if got := cellKeys(lane.Cells[4]); len(got) != 1 || got[0] != "PLAT-2" {
		t.Errorf("Done = %v", got)
	}
}
