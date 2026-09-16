package committer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// The boards phase (phases.go) runs before every other phase: a draft
// sprint's originBoardId and a card queued onto a board before Commit both
// name the board by id, and neither can be pushed while that id is still a
// negative placeholder. This file creates the drafted boards first, then
// pushes the queued adds, then makes one courtesy read per board to see
// which of the keys it just pushed actually match the board's filter.

// boardCreateFailure names a draft board's failure by its name, the way
// sprintFailure names a draft sprint's: the row's own key is a negative
// number nobody reading the banner could act on.
func boardCreateFailure(p journal.PendingChange, name, message string, retryable bool) Failure {
	return Failure{Key: name, EntityType: issuerepo.EntityBoardCreate, RowID: p.ID, Error: message, Retryable: retryable, Reachable: []string{}}
}

// createBoards creates every drafted board, oldest first, and records the
// draft id -> real id it was given in r.boardRealID so every later phase in
// this same Commit can resolve a reference to it. A board Jira refused, or
// one this connection cannot create, is blocked (r.deps.block) under its own
// negative id, so a draft sprint or a card naming it is held rather than
// sent with a placeholder; the row itself is left for the next Commit to
// retry, the same shape createSprints already uses for a refused sprint.
func (r *commitRun) createBoards(ctx context.Context) {
	bc, canCreate := r.e.b.(backend.BoardCreator)
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityBoardCreate {
			continue
		}
		var d issuerepo.DraftBoard
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			r.res.Failures = append(r.res.Failures, boardCreateFailure(p, "draft board "+p.EntityKey, "the draft board could not be decoded: "+err.Error(), false))
			r.deps.block(p.EntityKey, "draft board "+p.EntityKey, "which could not be read")
			continue
		}
		label := fmt.Sprintf("board %q", d.Name)
		draftID, err := strconv.Atoi(p.EntityKey)
		if err != nil || draftID >= 0 {
			r.res.Failures = append(r.res.Failures, boardCreateFailure(p, d.Name, fmt.Sprintf("the draft board's id %q is not a draft id", p.EntityKey), false))
			r.deps.block(p.EntityKey, label, "which could not be sent")
			continue
		}
		if !canCreate {
			r.res.Failures = append(r.res.Failures, boardCreateFailure(p, d.Name, "this connection cannot create boards", false))
			r.deps.block(p.EntityKey, label, "which this connection cannot create")
			continue
		}
		realID, err := bc.CreateBoard(ctx, backend.BoardDraft{Name: d.Name, Type: d.Type, FilterName: d.FilterName, JQL: d.JQL})
		if err != nil {
			r.res.Failures = append(r.res.Failures, boardCreateFailure(p, d.Name, err.Error(), true))
			r.deps.block(p.EntityKey, label, "which Jira refused")
			continue
		}
		if realID <= 0 {
			r.res.Failures = append(r.res.Failures, boardCreateFailure(p, d.Name, "Jira created the board but answered with no id; refresh the boards to see it, then add its issues again", false))
			r.deps.block(p.EntityKey, label, "which Jira created but answered with no id")
			continue
		}
		if err := r.e.repo.RekeyBoard(ctx, r.profileID, draftID, realID); err != nil {
			r.res.Failures = append(r.res.Failures, boardCreateFailure(p, d.Name, fmt.Sprintf("created in Jira as board %d but the local rename failed: %v", realID, err), false))
			r.deps.block(p.EntityKey, label, fmt.Sprintf("which Jira created as board %d but TAM could not rename; refresh the boards, then try again", realID))
			continue
		}
		r.boardRealID[draftID] = realID
		r.boardName[realID] = d.Name
	}
}

// boardAdd is one issue_board row, grouped by the board and scope it names.
type boardAdd struct {
	row   journal.PendingChange
	scope string
}

// pushBoardAdds pushes every issue_board row: a backlog scope batches
// through BoardCreator.AddToBoardBacklog, a sprint scope goes through the
// same boardWriter.MoveIssuesToSprint the board moves pass already uses to
// push a sprint move, since moving an issue onto a sprint already puts it on
// that sprint's board -- there is no second call to make for that half.
//
// A row naming a board this Commit could not create waits on it, through the
// same r.deps.blockedBy the board moves pass already holds board rows on. A
// row under a draft issue key is left alone, silently: the create phases run
// after this one and Rekey repoints its entity_key to the real one they
// give it, so the next Commit finds it under a real key and pushes it, the
// same rule boardRows (boards.go) already follows for a board-move row under
// a draft key.
func (r *commitRun) pushBoardAdds(ctx context.Context) {
	bc, canAddBacklog := r.e.b.(backend.BoardCreator)
	var w boardWriter = noBoardWrites{}
	if bw, ok := r.e.b.(boardWriter); ok {
		w = bw
	}

	byBoard := map[int][]boardAdd{}
	var boards []int
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityIssueBoard {
			continue
		}
		if isDraftKey(p.EntityKey) {
			continue
		}
		boardID, err := strconv.Atoi(issuerepo.MoveID(p.AfterVal))
		if err != nil {
			r.res.Failures = append(r.res.Failures, boardFailure(p, errors.New("its board could not be read"), false))
			continue
		}
		if real, ok := r.boardRealID[boardID]; ok {
			// issuerepo.RekeyBoard already rewrote this row's field and
			// value in the store the moment its board went real; r.rows is
			// the snapshot from before that ran, so p is corrected to match
			// what is on disk now. Left stale, clearMove's after_val match
			// (boards.go) finds nothing to delete and a pushed add reports
			// Moved while its row lives on for the next Commit to push
			// again.
			scope := issuerepo.MoveRawName(p.AfterVal)
			p.Field = issuerepo.BoardField(real)
			p.AfterVal = issuerepo.MoveValue(strconv.Itoa(real), scope)
			boardID = real
		} else if waits, held := r.deps.blockedBy(strconv.Itoa(boardID)); held {
			r.deps.hold(r.res, p.EntityKey, issuerepo.EntityIssueBoard, p.ID, waits)
			continue
		}
		if _, seen := byBoard[boardID]; !seen {
			boards = append(boards, boardID)
		}
		byBoard[boardID] = append(byBoard[boardID], boardAdd{row: p, scope: issuerepo.MoveRawName(p.AfterVal)})
	}
	sort.Ints(boards)

	pushed := map[int][]string{}
	for _, boardID := range boards {
		var backlog []journal.PendingChange
		bySprint := map[string][]journal.PendingChange{}
		var sprints []string
		for _, a := range byBoard[boardID] {
			if a.scope == issuerepo.ScopeBacklog {
				backlog = append(backlog, a.row)
				continue
			}
			if _, seen := bySprint[a.scope]; !seen {
				sprints = append(sprints, a.scope)
			}
			bySprint[a.scope] = append(bySprint[a.scope], a.row)
		}
		if len(backlog) > 0 {
			r.pushBacklogAdd(ctx, bc, canAddBacklog, boardID, backlog, pushed)
		}
		sort.Strings(sprints)
		for _, sprintID := range sprints {
			r.pushSprintAdd(ctx, w, boardID, sprintID, bySprint[sprintID], pushed)
		}
	}
	r.checkBoardFilters(ctx, pushed)
}

// pushBacklogAdd pushes one board's backlog-scope adds in a single call:
// AddToBoardBacklog already batches at 20 and fails a batch whole on a 207
// (core/jira's bulkWrite), so there is nothing left for this pass to batch.
func (r *commitRun) pushBacklogAdd(ctx context.Context, bc backend.BoardCreator, can bool, boardID int, rows []journal.PendingChange, pushed map[int][]string) {
	keys := rowKeys(rows)
	if !can {
		for _, p := range rows {
			r.res.Failures = append(r.res.Failures, boardFailure(p, errors.New("this connection cannot add issues to a board's backlog"), false))
		}
		return
	}
	if err := assertNoPlaceholders(map[string]any{"boardId": boardID, "issues": keys}); err != nil {
		for _, p := range rows {
			r.res.Failures = append(r.res.Failures, boardFailure(p, err, false))
		}
		return
	}
	if err := bc.AddToBoardBacklog(ctx, boardID, keys); err != nil {
		for _, p := range rows {
			r.res.Failures = append(r.res.Failures, boardFailure(p, err, true))
		}
		return
	}
	r.settleBoardAdds(ctx, rows, pushed, boardID)
}

// pushSprintAdd pushes one board's adds onto one sprint. A sprint id that is
// still a draft board's or a draft sprint's own placeholder trips
// assertNoPlaceholders here rather than reaching Jira: the boards phase
// cannot itself resolve a draft sprint (the sprints phase, which creates
// one, runs after this one), so a card queued onto a sprint that is still a
// draft when Commit runs is refused with a clear reason instead of being
// sent.
func (r *commitRun) pushSprintAdd(ctx context.Context, w boardWriter, boardID int, sprintID string, rows []journal.PendingChange, pushed map[int][]string) {
	keys := rowKeys(rows)
	if err := assertNoPlaceholders(map[string]any{"boardId": boardID, "sprintId": sprintID, "issues": keys}); err != nil {
		for _, p := range rows {
			r.res.Failures = append(r.res.Failures, boardFailure(p, err, false))
		}
		return
	}
	if err := w.MoveIssuesToSprint(ctx, sprintID, keys); err != nil {
		for _, p := range rows {
			r.res.Failures = append(r.res.Failures, boardFailure(p, err, true))
		}
		return
	}
	r.settleBoardAdds(ctx, rows, pushed, boardID)
}

// settleBoardAdds clears every row of a push that landed, reports it as a
// Moved the way every other board write is, and records the keys against
// their board for the filter check that follows.
func (r *commitRun) settleBoardAdds(ctx context.Context, rows []journal.PendingChange, pushed map[int][]string, boardID int) {
	for _, p := range rows {
		r.e.clearMove(ctx, r.profileID, p, r.res)
		r.res.Moved = append(r.res.Moved, Moved{Key: p.EntityKey, EntityType: issuerepo.EntityIssueBoard, Target: moveLabel(issuerepo.EntityIssueBoard, p.AfterVal)})
		pushed[boardID] = append(pushed[boardID], p.EntityKey)
	}
}

// boardLabel names a board for a result message: its drafted name when this
// Commit is the one that created it, else "board <id>", since the committer
// owns no general board-name lookup.
func (r *commitRun) boardLabel(boardID int) string {
	if name, ok := r.boardName[boardID]; ok {
		return name
	}
	return fmt.Sprintf("board %d", boardID)
}

func rowKeys(rows []journal.PendingChange) []string {
	keys := make([]string, 0, len(rows))
	for _, p := range rows {
		keys = append(keys, p.EntityKey)
	}
	return keys
}

// checkBoardFilters is the courtesy read after pushBoardAdds lands: for each
// board that received at least one add this Commit, which of those keys
// Jira's board filter actually kept. A key it dropped already had its
// journal row removed above -- Jira accepted the write, the issue simply
// does not match the filter, and leaving the row would retry it forever --
// so this only adds a result line, once per board rather than once per
// issue. A backend that cannot answer the check, or a read that fails,
// changes nothing about the Commit: it is a courtesy read after a write that
// already landed, logged and otherwise ignored.
func (r *commitRun) checkBoardFilters(ctx context.Context, pushed map[int][]string) {
	checker, ok := r.e.b.(backend.BoardFilterChecker)
	if !ok {
		return
	}
	var boards []int
	for boardID := range pushed {
		boards = append(boards, boardID)
	}
	sort.Ints(boards)
	for _, boardID := range boards {
		keys := pushed[boardID]
		present, err := checker.BoardFilterCheck(ctx, boardID, keys)
		if err != nil {
			log.Printf("tam: board %d filter check: %v", boardID, err)
			continue
		}
		inFilter := make(map[string]bool, len(present))
		for _, k := range present {
			inFilter[k] = true
		}
		for _, k := range keys {
			if inFilter[k] {
				continue
			}
			r.res.Failures = append(r.res.Failures, Failure{
				Key:        k,
				EntityType: issuerepo.EntityIssueBoard,
				Error:      fmt.Sprintf("%s is outside %s's filter, so it will not show on that board.", k, r.boardLabel(boardID)),
				Retryable:  false,
				Reachable:  []string{},
			})
		}
	}
}
