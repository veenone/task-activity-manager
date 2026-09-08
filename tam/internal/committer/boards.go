package committer

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// The board pass runs between the edits and the links. It is not an edits
// pass with different fields: a board row is checked against the issue's
// remote status or sprint rather than its updated stamp, it is pushed
// through the transition and agile endpoints rather than through the field
// update, and its ranks go last as one group because a rank is the one
// write that only means something relative to another card.

// sprintBatch is how many issues one sprint move carries. Jira's endpoint
// takes fifty, and a standup's worth of planning would otherwise be fifty
// round trips.
const sprintBatch = 50

// backlogLabel is what an empty sprint value reads as. Leaving every sprint
// is a destination, not an absence, and a conflict card that showed an
// empty cell for it would be unreadable.
const backlogLabel = "Backlog"

// BoardOrder is the board's final local order, top to bottom, which the
// rank group re-derives every neighbour from. boardrepo.Order satisfies it
// and app.go injects that; the committer declares the interface rather than
// importing boardrepo, the way boardrepo declares IssueSource rather than
// importing issuerepo.
type BoardOrder interface {
	CellOrder(ctx context.Context, profileID string, boardID int) ([]string, error)
}

// boardWriter is the part of backend.BoardBackend this pass needs. It asks
// for the two writes and not the four reads, so a test's backend does not
// have to answer for a board configuration to push a rank.
type boardWriter interface {
	RankIssue(ctx context.Context, key, neighbourKey string, before bool) error
	MoveIssuesToSprint(ctx context.Context, sprintID string, keys []string) error
}

// errNoAgile is what a rank or a sprint move reports on an instance whose
// Jira has no Agile API. It is a fact about the instance, not a failure of
// this commit, so retrying will not change it.
var errNoAgile = errors.New("this connection cannot move cards between sprints or reorder them: its Jira has no agile api")

// noAgileWrites stands in for a backend that cannot answer
// backend.BoardBackend, so the rows that need it fail with a reason where
// they are pushed instead of taking every board row down at the door.
type noAgileWrites struct{}

func (noAgileWrites) RankIssue(context.Context, string, string, bool) error { return errNoAgile }

func (noAgileWrites) MoveIssuesToSprint(context.Context, string, []string) error { return errNoAgile }

// Moved is one board write this Commit settled: a card transitioned,
// moved to a sprint, or ranked. Target is what was pushed, named rather
// than numbered. Satisfied marks a row Jira already agreed with, where the
// move had happened on the web or by someone else and the row was dropped
// rather than pushed again.
type Moved struct {
	Key        string `json:"key"`
	EntityType string `json:"entityType"`
	Target     string `json:"target"`
	Satisfied  bool   `json:"satisfied"`
}

// movePlan is what one remote read per issue decided: which rows are still
// worth pushing, and which keys need a refresh once they have been.
type movePlan struct {
	keys        []string
	sprints     map[string]journal.PendingChange
	transitions map[string]journal.PendingChange
	ranks       []journal.PendingChange
	pushed      map[string]bool
}

// commitBoardMoves pushes the journal's board rows: per issue the sprint
// move and then the transition, since a sprint move changes which columns
// apply and a transition changes the status a rank is measured against, and
// then every rank as one group. A failure is per row and does not stop the
// pass; a row whose key is still a draft waits for the next Commit, the
// rule the link pass already uses.
func (e *Engine) commitBoardMoves(ctx context.Context, profileID string, res *Result) {
	rows, err := e.boardRows(ctx, profileID)
	if err != nil {
		res.Failures = append(res.Failures, Failure{Key: "board moves", Error: "the journal could not be read for board moves: " + err.Error(), Retryable: true, Reachable: []string{}})
		return
	}
	if len(rows) == 0 {
		return
	}
	// A transition goes through Jira's ordinary REST API, so it is pushed
	// whether or not the instance has the Agile one; only the rank and the
	// sprint move need that, and they say so per row rather than failing
	// every board row at the door.
	var w boardWriter = noAgileWrites{}
	if bw, ok := e.b.(boardWriter); ok {
		w = bw
	}
	plan := e.planBoardMoves(ctx, profileID, rows, res)
	e.pushSprints(ctx, profileID, w, plan, res)
	e.pushTransitions(ctx, profileID, plan, res)
	for _, key := range plan.keys {
		if plan.pushed[key] {
			e.refresh(ctx, profileID, key)
		}
	}
	e.pushRanks(ctx, profileID, w, plan.ranks, res)
}

// boardRows reads the journal again, after the creates and the edits have
// run, and returns the board rows oldest first. Rereading is what lets a
// row that was journaled against a draft be pushed under the key the create
// pass gave it; a key that is still a draft is left out entirely.
func (e *Engine) boardRows(ctx context.Context, profileID string) ([]journal.PendingChange, error) {
	all, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	rows := make([]journal.PendingChange, 0, len(all))
	for _, p := range all {
		if !boardRow(p.EntityType) || isDraftKey(p.EntityKey) {
			continue
		}
		rows = append(rows, p)
	}
	return rows, nil
}

// planBoardMoves reads each issue's remote state once and decides what to
// do with its sprint and transition rows. Three answers, against the remote
// status or sprint and never against the updated stamp: a comment bumps
// updated without moving the card.
//
//   - Remote already at the journaled target: the move happened, on the web
//     or by someone else. The row is dropped as satisfied and counted, not
//     raised as a conflict over an outcome the user asked for.
//   - Remote at the journaled before value: nothing moved under us, push it.
//   - Anything else: the issue is held back as a conflict carrying before,
//     target, and remote.
//
// Ranks are collected untouched: a rank has nothing to compare and nothing
// to rebase, so it is never held back.
func (e *Engine) planBoardMoves(ctx context.Context, profileID string, rows []journal.PendingChange, res *Result) movePlan {
	plan := movePlan{sprints: map[string]journal.PendingChange{}, transitions: map[string]journal.PendingChange{}, pushed: map[string]bool{}}
	byKey := map[string][]journal.PendingChange{}
	var keys []string
	for _, p := range rows {
		if p.EntityType == issuerepo.EntityRank {
			plan.ranks = append(plan.ranks, p)
			continue
		}
		if _, seen := byKey[p.EntityKey]; !seen {
			keys = append(keys, p.EntityKey)
		}
		byKey[p.EntityKey] = append(byKey[p.EntityKey], p)
	}
	sort.Strings(keys)

	for _, key := range keys {
		remote, err := e.b.GetIssue(ctx, key)
		if err != nil {
			for _, p := range byKey[key] {
				res.Failures = append(res.Failures, boardFailure(p, err, true))
			}
			continue
		}
		var held []FieldConflict
		ready := map[string]journal.PendingChange{}
		for _, p := range byKey[key] {
			switch classifyMove(p, remoteValue(p.EntityType, remote)) {
			case moveSatisfied:
				e.clearMove(ctx, profileID, p, res)
				res.Moved = append(res.Moved, Moved{Key: key, EntityType: p.EntityType, Target: moveLabel(p.EntityType, p.AfterVal), Satisfied: true})
			case movePush:
				ready[p.EntityType] = p
			default:
				held = append(held, FieldConflict{
					Field:  p.Field,
					Base:   moveLabel(p.EntityType, p.BeforeVal),
					Mine:   moveLabel(p.EntityType, p.AfterVal),
					Remote: remoteLabel(p.EntityType, remote),
				})
			}
		}
		if len(held) > 0 {
			// The whole issue is held: pushing its other board row while one
			// of them disagrees with Jira would commit half an intent
			// against a card that is not where the user left it.
			e.holdBoard(res, key, remote, held)
			continue
		}
		if p, ok := ready[issuerepo.EntitySprintMove]; ok {
			plan.sprints[key] = p
		}
		if p, ok := ready[issuerepo.EntityTransition]; ok {
			plan.transitions[key] = p
		}
		if len(ready) > 0 {
			plan.keys = append(plan.keys, key)
		}
	}
	return plan
}

// pushSprints moves the cards, batched by target sprint, in batches of
// fifty. Every row of a failed batch keeps its journal row and is reported;
// the rest of the pass carries on.
func (e *Engine) pushSprints(ctx context.Context, profileID string, w boardWriter, plan movePlan, res *Result) {
	byTarget := map[string][]journal.PendingChange{}
	var targets []string
	for _, key := range plan.keys {
		p, ok := plan.sprints[key]
		if !ok {
			continue
		}
		target := issuerepo.MoveID(p.AfterVal)
		if _, seen := byTarget[target]; !seen {
			targets = append(targets, target)
		}
		byTarget[target] = append(byTarget[target], p)
	}
	sort.Strings(targets)
	for _, target := range targets {
		rows := byTarget[target]
		for start := 0; start < len(rows); start += sprintBatch {
			end := start + sprintBatch
			if end > len(rows) {
				end = len(rows)
			}
			batch := rows[start:end]
			keys := make([]string, 0, len(batch))
			for _, p := range batch {
				keys = append(keys, p.EntityKey)
			}
			if err := w.MoveIssuesToSprint(ctx, target, keys); err != nil {
				for _, p := range batch {
					res.Failures = append(res.Failures, boardFailure(p, err, !errors.Is(err, errNoAgile)))
				}
				continue
			}
			for _, p := range batch {
				e.clearMove(ctx, profileID, p, res)
				plan.pushed[p.EntityKey] = true
				res.Moved = append(res.Moved, Moved{Key: p.EntityKey, EntityType: p.EntityType, Target: moveLabel(p.EntityType, p.AfterVal)})
			}
		}
	}
}

// pushTransitions fires one transition per issue, in key order. The
// backend resolves the journaled status id to a transition at push time; a
// target with no path back comes back as ErrNoTransition and is reported
// with the statuses the card can actually reach.
func (e *Engine) pushTransitions(ctx context.Context, profileID string, plan movePlan, res *Result) {
	for _, key := range plan.keys {
		p, ok := plan.transitions[key]
		if !ok {
			continue
		}
		if err := e.b.Transition(ctx, key, issuerepo.MoveID(p.AfterVal)); err != nil {
			res.Failures = append(res.Failures, boardFailure(p, err, true))
			continue
		}
		e.clearMove(ctx, profileID, p, res)
		plan.pushed[key] = true
		res.Moved = append(res.Moved, Moved{Key: key, EntityType: p.EntityType, Target: moveLabel(p.EntityType, p.AfterVal)})
	}
}

// pushRanks is the last group, after every transition and sprint move has
// landed, because both change where a card sits. Each board's ranks push in
// that board's final local order, top to bottom, every card anchored with
// rankAfterIssue against the card already anchored above it: mixing before
// and after between two cards that were ranked against each other is how an
// order becomes a cycle.
//
// A rank needs no issue refresh afterwards, which halves the calls in the
// group that has the most rows. A rank whose board is gone from the store,
// whose card has left that board, or that has nothing above it to anchor
// against is dropped with its reason rather than pushed against a card that
// is not there; its journal row stays, so the user can undo the move or
// sync the boards and commit again.
func (e *Engine) pushRanks(ctx context.Context, profileID string, w boardWriter, ranks []journal.PendingChange, res *Result) {
	if len(ranks) == 0 {
		return
	}
	byBoard := map[int][]journal.PendingChange{}
	var boards []int
	for _, p := range ranks {
		_, _, boardID := issuerepo.ParseRank(p.AfterVal)
		if _, seen := byBoard[boardID]; !seen {
			boards = append(boards, boardID)
		}
		byBoard[boardID] = append(byBoard[boardID], p)
	}
	sort.Ints(boards)
	for _, boardID := range boards {
		rows := byBoard[boardID]
		if e.order == nil {
			e.dropRanks(rows, res, errors.New("this commit was built with no board order to rank against"))
			continue
		}
		order, err := e.order.CellOrder(ctx, profileID, boardID)
		if err != nil {
			e.dropRanks(rows, res, fmt.Errorf("board %d's order could not be read: %w", boardID, err))
			continue
		}
		want := make(map[string]journal.PendingChange, len(rows))
		for _, p := range rows {
			want[p.EntityKey] = p
		}
		prev := ""
		for _, key := range order {
			p, ok := want[key]
			if !ok {
				prev = key
				continue
			}
			delete(want, key)
			if prev == "" {
				res.Failures = append(res.Failures, boardFailure(p, fmt.Errorf("%s is at the top of board %d, so there is no card above it to rank it against", key, boardID), false))
			} else if err := w.RankIssue(ctx, key, prev, false); err != nil {
				res.Failures = append(res.Failures, boardFailure(p, err, !errors.Is(err, errNoAgile)))
			} else {
				e.clearMove(ctx, profileID, p, res)
				res.Moved = append(res.Moved, Moved{Key: key, EntityType: p.EntityType, Target: "after " + prev})
			}
			prev = key
		}
		// Whatever is left never appeared in the order: the card has gone
		// from the board, so the cell the rank was measured in no longer
		// holds it.
		leftover := make([]journal.PendingChange, 0, len(want))
		for _, p := range rows {
			if _, still := want[p.EntityKey]; still {
				leftover = append(leftover, p)
			}
		}
		for _, p := range leftover {
			res.Failures = append(res.Failures, boardFailure(p, fmt.Errorf("%s is no longer on board %d, so there is nothing to rank it against", p.EntityKey, boardID), false))
		}
	}
}

// dropRanks reports every rank of one board with the same reason.
func (e *Engine) dropRanks(rows []journal.PendingChange, res *Result, err error) {
	for _, p := range rows {
		res.Failures = append(res.Failures, boardFailure(p, err, false))
	}
}

// clearMove drops the journal row of a board write that landed, but only
// while its after_val is still the value that was pushed: the move bindings
// take no busy guard, so a card dragged again mid-push updates that row in
// place, and a delete by id would throw away an intent Jira was never told
// about. The commit is audited either way, because the push did happen.
func (e *Engine) clearMove(ctx context.Context, profileID string, p journal.PendingChange, res *Result) {
	if _, err := e.repo.MarkMoveCommitted(ctx, profileID, p, p.AfterVal); err != nil {
		res.Failures = append(res.Failures, boardFailure(p, fmt.Errorf("pushed to Jira but the journal could not be cleared: %w", err), false))
	}
}

// holdBoard adds the held fields to the issue's conflict card, joining the
// one the edits pass may already have raised for the same key rather than
// reporting the issue twice.
func (e *Engine) holdBoard(res *Result, key string, remote backend.Issue, fields []FieldConflict) {
	for i := range res.Conflicts {
		if res.Conflicts[i].Key == key {
			res.Conflicts[i].Fields = append(res.Conflicts[i].Fields, fields...)
			return
		}
	}
	res.Conflicts = append(res.Conflicts, Conflict{
		Key: key, Summary: remote.Summary, RemoteVersion: remote.Updated, Fields: fields,
	})
}

// The three answers a fresh remote read gives one journaled board move.
const (
	moveSatisfied = iota
	movePush
	moveConflict
)

// classifyMove compares the journal's ids with the remote's, never the
// "id|Name" text: a status name that differs between the board
// configuration and the cached row would make a card that never moved look
// like a conflict.
func classifyMove(p journal.PendingChange, remoteID string) int {
	switch {
	case issuerepo.MoveID(p.AfterVal) == remoteID:
		return moveSatisfied
	case issuerepo.MoveID(p.BeforeVal) == remoteID:
		return movePush
	default:
		return moveConflict
	}
}

// remoteValue is the id a board row is checked against.
func remoteValue(entityType string, remote backend.Issue) string {
	if entityType == issuerepo.EntitySprintMove {
		return remote.SprintID
	}
	return remote.StatusID
}

// moveLabel is what a journaled board value reads as. issuerepo.FieldValue
// is not used anywhere in this file: it knows the six editable fields and
// would render an empty string for a status or a sprint.
func moveLabel(entityType, value string) string {
	if entityType == issuerepo.EntitySprintMove && issuerepo.MoveID(value) == "" {
		return backlogLabel
	}
	return issuerepo.MoveName(value)
}

// remoteLabel is what Jira holds now, for the conflict card.
func remoteLabel(entityType string, remote backend.Issue) string {
	if entityType == issuerepo.EntitySprintMove {
		switch {
		case remote.SprintID == "":
			return backlogLabel
		case remote.SprintName != "":
			return remote.SprintName
		}
		return remote.SprintID
	}
	if remote.Status != "" {
		return remote.Status
	}
	return remote.StatusID
}

// boardFailure is one board row that did not land. It carries the row so a
// per-row Undo knows exactly what to discard: one card can fail a
// transition and drop a rank in the same Commit. A transition with no path
// and one asking for a field TAM cannot fill will fail identically forever,
// so neither is retryable however the caller asked for it, and the first
// hands over the statuses the card can actually reach.
func boardFailure(p journal.PendingChange, err error, retryable bool) Failure {
	f := Failure{
		Key: p.EntityKey, EntityType: p.EntityType, RowID: p.ID,
		Error: err.Error(), Retryable: retryable, Reachable: []string{},
	}
	var noPath *backend.NoTransition
	if errors.As(err, &noPath) {
		f.Reachable = backend.NonNil(noPath.Reachable)
	}
	if errors.Is(err, backend.ErrNoTransition) || errors.Is(err, backend.ErrTransitionFields) {
		f.Retryable = false
	}
	return f
}
