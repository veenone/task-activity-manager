package boardrepo

import (
	"context"
	"fmt"
	"strconv"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/dbtx"
)

// detailSprintsSQL orders one board's sprints the way the Sprints view wants
// them, which is not listSprintsSQL's order: active first, then future by
// start date ascending same as the picker, but closed by start date
// descending, so the sprint that finished most recently reads first instead
// of last. The second and third ORDER BY expressions are each NULL for the
// state the other column belongs to, so within the closed group only the
// third column carries a value and decides the order, and within the active
// and future groups only the second does; one query sorts both directions at
// once by giving each direction its own column instead of trying to flip the
// sign of a text column.
const detailSprintsSQL = `
	SELECT id, board_id, name, state, start_date, end_date, goal FROM sprint
	WHERE profile_id = ? AND board_id = ?
	ORDER BY CASE state WHEN 'active' THEN 0 WHEN 'future' THEN 1 WHEN 'closed' THEN 2 ELSE 3 END,
	         CASE WHEN state = 'closed' THEN NULL ELSE start_date END ASC,
	         CASE WHEN state = 'closed' THEN start_date ELSE NULL END DESC,
	         id`

// UnassignedSprintState is the state SprintDetail's unassigned node carries
// on its embedded Sprint. It is deliberately not one of Jira's three states:
// the node is not a sprint, and leaving State empty or borrowing "future"
// would read as if it were.
const UnassignedSprintState = "unassigned"

// UnassignedSprintName is the unassigned node's display name. It cannot
// reuse laneUnassigned: that word already means "no assignee" on the
// Boards view's assignee swimlane, and a view that can show both an
// assignee swimlane and this node side by side needs the two to read as
// different things, so this one names the board's own list rather than a
// missing person.
const UnassignedSprintName = "Board backlog"

// SprintDetail is one sprint the Sprints view draws, or the board's own
// unassigned work under the same shape: the row, its issues after
// withMovedIn and applyMoves have replayed the journal onto them in the
// same order composeBoard uses them in, and the four numbers a fill bar
// reads from. It does not replay pending rank the way composeBoard does, so
// cards come back in board_issue.position order with any pending reorder
// ignored, and it draws no drafts: composeBoard draws every draft on every
// board and every sprint regardless of the draft's own sprint id, since a
// draft is project level there and DraftIssues answers unfiltered by scope.
// Copying that same everywhere-placement into a per-sprint read would be no
// better an answer, since it is not sprint-true either, so this read leaves
// the exclusion explicit instead. A draft created into a sprint therefore
// shows in neither its sprint nor the unassigned node, and this view's
// counts can disagree with the Boards view's for the same sprint.
type SprintDetail struct {
	Sprint
	// Issues is this scope's cards, capped by MaxCardsPerView, the same
	// running budget the whole call spends across every sprint and the
	// unassigned node together: this payload crosses Wails too, and it is
	// invalidated on several mutations, so it gets the board's own budget
	// rather than the epic tree's larger one.
	Issues []backend.Issue `json:"issues"`

	// Total, Done, Points and DonePoints are counted over every card the
	// journal-replayed scope holds, whether or not the shared cap let it
	// into Issues, the same way BoardView.DonePoints is summed over every
	// mapped card rather than only the ones a capped cell drew.
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	Points     float64 `json:"points"`
	DonePoints float64 `json:"donePoints"`

	// MembershipCached is derived from the sprint's own state, never from
	// whether board_issue happens to hold a row for it: board_issue is
	// equally empty for a closed sprint, whose membership the sync
	// deliberately never fetches, and for a future sprint whose last sync
	// attempt failed partway through. This field can only say whether the
	// sync tries to fetch this scope's membership at all; it cannot say
	// whether the last attempt actually landed. It is always true on the
	// unassigned node, whose cards come from the board's own list rather
	// than from a sprint's membership.
	MembershipCached bool `json:"membershipCached"`

	// NotSynced counts this node's own scope keys the issue cache does not
	// hold: IssuesByKeys silently drops a key it cannot find, so a sprint
	// of thirty issues on a board synced before its issues were would
	// otherwise report a Total of ten with nothing saying so. It differs
	// from MembershipCached, which only says whether the sync tries to
	// fetch this scope's membership at all; NotSynced says how many of the
	// keys that scope actually named went missing from the count, the same
	// way BoardView.NotSynced does for a board's own list.
	NotSynced int `json:"notSynced"`

	// Truncated is set when the shared MaxCardsPerView budget stopped this
	// node's Issues short. The four numbers above are unaffected, since
	// they are counted before the cap is applied.
	Truncated bool `json:"truncated"`
}

// BoardSprintDetails composes the Sprints view's data: one board's sprints,
// active first, then future, then closed most-recently-finished first, and
// last the board's own unassigned work, each carrying its issues and its
// four progress numbers.
//
// It runs on one deferred read transaction, the same one Board uses, so the
// sprint rows, the membership tables, and the issue cache all come from one
// snapshot: a reader spread across the handle could otherwise land between
// two statements of a boards sync and count a sprint against membership that
// sync had already replaced.
func (r *Repository) BoardSprintDetails(ctx context.Context, issues IssueSource, profileID string, boardID int) ([]SprintDetail, error) {
	var out []SprintDetail
	err := r.inReadTx(ctx, func(q dbtx.Querier) error {
		v, err := sprintDetails(ctx, q, issues, profileID, boardID)
		out = v
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// sprintDetails is the read itself, every statement on the querier it was
// given.
func sprintDetails(ctx context.Context, q dbtx.Querier, issues IssueSource, profileID string, boardID int) ([]SprintDetail, error) {
	sprints, err := sprintsForDetail(ctx, q, profileID, boardID)
	if err != nil {
		return nil, err
	}
	moves, err := issues.PendingMoves(ctx, q, profileID)
	if err != nil {
		return nil, err
	}
	boardKeys, err := issueKeys(ctx, q, profileID, boardID, "")
	if err != nil {
		return nil, err
	}

	rendered := 0
	// claimed is every key a sprint's journal-replayed scope holds, across
	// every sprint. The unassigned scope is the board's own list minus this
	// set: board_issue's own list already carries every sprint's issues
	// too, so without the subtraction each one would be drawn twice, once
	// under its sprint and once again as unassigned.
	claimed := map[string]bool{}
	out := make([]SprintDetail, 0, len(sprints)+1)
	for _, s := range sprints {
		sprintID := strconv.Itoa(s.ID)
		scopeKeys, err := issueKeys(ctx, q, profileID, boardID, sprintID)
		if err != nil {
			return nil, err
		}
		// Same helpers, same order composeBoard uses them in: withMovedIn
		// pulls in a card the journal has just moved into this sprint,
		// which board_issue cannot yet know about, before the cards are
		// read; applyMoves then drops whatever the journal has moved back
		// out. Skipping either step is what leaves a bulk move looking
		// like it did nothing until the next boards sync.
		movedKeys := withMovedIn(scopeKeys, moves, sprintID)
		cards, err := issues.IssuesByKeys(ctx, q, profileID, movedKeys)
		if err != nil {
			return nil, err
		}
		// Counted against scopeKeys, the sprint's own membership, and not
		// movedKeys: a key withMovedIn added came from the journal, not
		// from a sync that might have missed it, so it has nothing to do
		// with what this scope failed to sync.
		notSynced := countNotSynced(scopeKeys, cards)
		cards = applyMoves(cards, moves, sprintID)
		// claimed is built from cards after applyMoves has run, so a card
		// the journal has just moved out of this sprint is not claimed by
		// it: applyMoves already dropped it here, and the unassigned scope
		// below is where it belongs instead. Building claimed from the
		// pre-replay cards would claim it anyway and it would vanish from
		// both.
		for _, c := range cards {
			claimed[c.Key] = true
		}
		detail := newSprintDetail(s)
		detail.NotSynced = notSynced
		fillDetail(&detail, cards, &rendered)
		out = append(out, detail)
	}

	unassignedKeys := make([]string, 0, len(boardKeys))
	for _, k := range boardKeys {
		if !claimed[k] {
			unassignedKeys = append(unassignedKeys, k)
		}
	}
	unassignedCards, err := issues.IssuesByKeys(ctx, q, profileID, unassignedKeys)
	if err != nil {
		return nil, err
	}
	unassignedNotSynced := countNotSynced(unassignedKeys, unassignedCards)
	// applyMoves is still worth a pass here: a card can carry a pending
	// transition without a pending sprint move, and this scope should draw
	// it in its journaled status the same as every sprint's does. Passing
	// "" as the scope is what a whole-board view passes too, and it drops
	// nothing here for the same reason it drops nothing there: applyMoves'
	// own drop branch only fires when sprintID != "", so an empty scope has
	// no sprint to leave.
	unassignedCards = applyMoves(unassignedCards, moves, "")
	// Built through the same constructor as every sprint node, rather than
	// a separate literal, so Issues starts as an empty slice on every path
	// and fillDetail's nil guard has only one meaning to carry.
	unassigned := newSprintDetail(Sprint{BoardID: boardID, Name: UnassignedSprintName, State: UnassignedSprintState})
	unassigned.MembershipCached = true
	unassigned.NotSynced = unassignedNotSynced
	fillDetail(&unassigned, unassignedCards, &rendered)
	out = append(out, unassigned)

	return out, nil
}

// newSprintDetail seeds one sprint's node with MembershipCached derived from
// its state: the sync only ever asks Jira for an active or future sprint's
// membership (internal/syncer/boards.go), so a closed sprint's board_issue
// rows are absent because nobody asked, not because the sprint is empty.
func newSprintDetail(s Sprint) SprintDetail {
	return SprintDetail{
		Sprint:           s,
		Issues:           []backend.Issue{},
		MembershipCached: s.State == "active" || s.State == "future",
	}
}

// fillDetail counts every card into the four numbers, then appends it to
// Issues until the shared MaxCardsPerView budget runs out, past which the
// card is still counted but not rendered, and Truncated says so. It assumes
// d.Issues already starts as an empty slice, the way newSprintDetail leaves
// it on every caller.
func fillDetail(d *SprintDetail, cards []backend.Issue, rendered *int) {
	for _, c := range cards {
		d.Total++
		done := backend.IsDone(c.Status)
		if done {
			d.Done++
		}
		if c.StoryPoints != nil {
			d.Points += *c.StoryPoints
			if done {
				d.DonePoints += *c.StoryPoints
			}
		}
		if *rendered >= MaxCardsPerView {
			d.Truncated = true
			continue
		}
		d.Issues = append(d.Issues, c)
		(*rendered)++
	}
}

// sprintsForDetail reads one board's sprints in detailSprintsSQL's order, on
// whichever querier the caller hands it, so the sprint rows and the cards
// they are filled with come from one snapshot.
func sprintsForDetail(ctx context.Context, q dbtx.Querier, profileID string, boardID int) ([]Sprint, error) {
	rows, err := q.QueryContext(ctx, detailSprintsSQL, profileID, boardID)
	if err != nil {
		return nil, fmt.Errorf("board %d sprint details: %w", boardID, err)
	}
	defer rows.Close()
	out := []Sprint{}
	for rows.Next() {
		var s Sprint
		if err := rows.Scan(&s.ID, &s.BoardID, &s.Name, &s.State, &s.StartDate, &s.EndDate, &s.Goal); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
