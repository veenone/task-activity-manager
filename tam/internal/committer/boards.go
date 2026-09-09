package committer

import (
	"context"
	"errors"
	"fmt"
	"sort"

	corejira "agile-suite/core/jira"
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
//
// Twenty rather than fifty, because the board's bulk move now makes batches
// this large routinely: the endpoint answers a partial refusal with a 207
// that names issues by numeric id, which cannot be mapped back to keys, so
// every card in a batch fails together. Twenty is how much of a planning
// session one refusal can take down, and it is the same width the sprint
// completion pushes at.
const sprintBatch = 20

// BoardOrder is the board's final local order, top to bottom, which the
// rank group re-derives every neighbour from. boardrepo.Order satisfies it
// and app.go injects that; the committer declares the interface rather than
// importing boardrepo, the way boardrepo declares IssueSource rather than
// importing issuerepo.
type BoardOrder interface {
	CellOrder(ctx context.Context, profileID string, boardID int) ([]string, error)
	// ColumnStatuses is every status sharing a board column with this one,
	// that one first. A column collects several statuses and only one is
	// usually reachable from where a card is now, so the push is given the
	// whole set rather than the single id the drop journaled.
	ColumnStatuses(ctx context.Context, profileID, statusID string) ([]string, error)
}

// boardWriter is the part of backend.BoardBackend this pass needs. It asks
// for the two writes and not the four reads, so a test's backend does not
// have to answer for a board configuration to push a rank.
type boardWriter interface {
	RankIssue(ctx context.Context, key, neighbourKey string, before bool) error
	MoveIssuesToSprint(ctx context.Context, sprintID string, keys []string) error
}

// errNoBoardWrites is what a rank or a sprint move reports when the backend
// behind this commit cannot write to boards at all. It says nothing about
// the instance: an instance whose Jira has no Agile API answers the two
// endpoints with a 404, which core/jira turns into jira.ErrNoAgile and
// which arrives here inside the push error. Both are settled facts rather
// than failures of this commit, so neither is worth a retry.
var errNoBoardWrites = errors.New("this connection cannot move cards between sprints or reorder them")

// noBoardWrites stands in for a backend that cannot answer
// backend.BoardBackend, so the rows that need it fail with a reason where
// they are pushed instead of taking every board row down at the door. Both
// shipped backends do answer it; this is the door a future read-only one
// comes through, and a transition still pushes past it.
type noBoardWrites struct{}

func (noBoardWrites) RankIssue(context.Context, string, string, bool) error {
	return errNoBoardWrites
}

func (noBoardWrites) MoveIssuesToSprint(context.Context, string, []string) error {
	return errNoBoardWrites
}

// Moved is one board write this Commit settled: a card transitioned,
// moved to a sprint, or ranked. Target is what was pushed, named rather
// than numbered: a status or a sprint by name, and for a rank the key of
// the card it was placed against, with Side saying which side of that card
// it went. Side is empty for everything but a rank, so a reader that wants
// a sentence joins the two rather than parsing one apart. Satisfied marks
// a row Jira already agreed with, where the move had happened on the web or
// by someone else and the row was dropped rather than pushed again.
type Moved struct {
	Key        string `json:"key"`
	EntityType string `json:"entityType"`
	Target     string `json:"target"`
	Side       string `json:"side"`
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
// then every rank as one group, in ranks.go. A failure is per row and does not stop the
// pass; a row whose key is still a draft waits for the next Commit, the
// rule the link pass already uses.
func (e *Engine) commitBoardMoves(ctx context.Context, profileID string, res *Result) {
	rows, err := e.boardRows(ctx, profileID)
	if err != nil {
		res.Failures = append(res.Failures, failure("board moves", "", "the journal could not be read for board moves: "+err.Error(), true))
		return
	}
	if len(rows) == 0 {
		return
	}
	// A transition goes through Jira's ordinary REST API, so it is pushed
	// whether or not the instance has the Agile one; only the rank and the
	// sprint move need that, and they say so per row rather than failing
	// every board row at the door.
	var w boardWriter = noBoardWrites{}
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
				e.clearSatisfied(ctx, profileID, p, res)
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
					res.Failures = append(res.Failures, boardFailure(p, err, true))
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
// backend resolves the journaled status to a transition at push time.
//
// It is given every status the dropped-on column collects, not only the one
// the drop journaled: a Jira column holds several statuses, and only one of
// them is usually reachable from where the card is now, so pushing the first
// alone refused a move the board was plainly offering. The journaled status
// stays first, so a reachable target is still preferred over its siblings.
// A column whose statuses are all out of reach comes back as
// ErrNoTransition and is reported with the statuses the card can reach.
func (e *Engine) pushTransitions(ctx context.Context, profileID string, plan movePlan, res *Result) {
	for _, key := range plan.keys {
		p, ok := plan.transitions[key]
		if !ok {
			continue
		}
		target := issuerepo.MoveID(p.AfterVal)
		targets, err := e.order.ColumnStatuses(ctx, profileID, target)
		if err != nil || len(targets) == 0 {
			// The board's columns are a local read; failing it should not
			// cost the move, it should only cost the siblings.
			targets = []string{target}
		}
		if err := e.b.Transition(ctx, key, targets); err != nil {
			res.Failures = append(res.Failures, boardFailure(p, namedTarget(err, issuerepo.MoveRawName(p.AfterVal)), true))
			continue
		}
		e.clearMove(ctx, profileID, p, res)
		plan.pushed[key] = true
		res.Moved = append(res.Moved, Moved{Key: key, EntityType: p.EntityType, Target: moveLabel(p.EntityType, p.AfterVal)})
	}
}

// clearMove drops the journal row of a board write that landed, but only
// while its after_val is still the value that was pushed: the move bindings
// take no busy guard, so a card dragged again mid-push updates that row in
// place, and a delete by id would throw away an intent Jira was never told
// about. The commit is audited either way, because the push did happen.
//
// A failure here is a local SQLite write, the most transient thing in the
// pass, and retrying is the fix rather than a false hope: the next Commit
// reads a remote that is already at the target, classifies the row as
// satisfied, and deletes it. So it is retryable.
func (e *Engine) clearMove(ctx context.Context, profileID string, p journal.PendingChange, res *Result) {
	if err := e.repo.MarkMoveCommitted(ctx, profileID, p); err != nil {
		res.Failures = append(res.Failures, boardFailure(p, fmt.Errorf("pushed to Jira but the journal could not be cleared: %w", err), true))
	}
}

// clearSatisfied drops the row of a move Jira had already made. It is the
// same delete with a different word in the trail: nothing was pushed, so
// auditing it as a commit would have the Activity tab claim a push that
// never happened.
func (e *Engine) clearSatisfied(ctx context.Context, profileID string, p journal.PendingChange, res *Result) {
	if err := e.repo.MarkMoveSatisfied(ctx, profileID, p); err != nil {
		res.Failures = append(res.Failures, boardFailure(p, fmt.Errorf("Jira already held this value but the journal could not be cleared: %w", err), true))
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

// boardFailure is one board row that did not land. It carries the row so a
// per-row Undo knows exactly what to discard: one card can fail a
// transition and drop a rank in the same Commit.
//
// Four errors fail identically for as long as they are sent, so none of
// them is retryable however the caller asked for it: a transition with no
// path, one asking for a field TAM cannot fill, a backend that cannot write
// to boards, and an instance whose Jira has no Agile API to write to. The
// first also hands over the statuses the card can actually reach.
var settledFacts = []error{backend.ErrNoTransition, backend.ErrTransitionFields, errNoBoardWrites, corejira.ErrNoAgile}

// namedTarget puts the journaled status name into a refused transition's
// sentence. The backend is given a status id and can only name a status it
// cannot reach by number; the journal row carries "id|Name", and this
// sentence is what the commit banner shows the user word for word. A row
// whose name half is empty, which is a status the cache has never seen,
// leaves the error exactly as it was.
func namedTarget(err error, name string) error {
	var noPath *backend.NoTransition
	if name == "" || !errors.As(err, &noPath) || noPath.TargetStatus != "" {
		return err
	}
	named := *noPath
	named.TargetStatus = name
	return &named
}

func boardFailure(p journal.PendingChange, err error, retryable bool) Failure {
	f := Failure{
		Key: p.EntityKey, EntityType: p.EntityType, RowID: p.ID,
		Error: err.Error(), Retryable: retryable, Reachable: []string{},
	}
	var noPath *backend.NoTransition
	if errors.As(err, &noPath) {
		f.Reachable = backend.NonNil(noPath.Reachable)
	}
	for _, settled := range settledFacts {
		if errors.Is(err, settled) {
			f.Retryable = false
		}
	}
	return f
}
