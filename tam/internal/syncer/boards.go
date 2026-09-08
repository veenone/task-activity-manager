package syncer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/errtext"
)

// settingBoardsUnavailable is the profile setting the pass writes whenever
// the boards call answers at all: "true" when the instance answered
// ErrNoAgile, cleared the moment boards come back, so a profile repointed
// at a Jira Software instance stops claiming there are none.
const settingBoardsUnavailable = "boards_unavailable"

// slowBoard is how long one board's read has to take before it is worth a
// log line naming it.
const slowBoard = 5 * time.Second

// BoardSummary is what one boards pass reports.
type BoardSummary struct {
	Boards  int `json:"boards"`
	Columns int `json:"columns"`
	Sprints int `json:"sprints"`
	// Cards is how many distinct issue keys the boards that landed hold: a
	// key that is both on a board's own list and in one of its sprints is
	// one card, counted once. A key on two boards is two cards, because it
	// is drawn on both.
	Cards       int      `json:"cards"`
	Dropped     []string `json:"dropped"`
	// Foreign is how many boards were left alone because they belong to
	// another project. It is not a failure, so it is counted rather than
	// listed in Dropped.
	Foreign     int  `json:"foreign"`
	Unavailable bool `json:"unavailable"`
	Elapsed     string   `json:"elapsed"`
}

// EnsureDropped makes Dropped a non-nil slice, which is what the frontend
// wants: a nil slice marshals to null and every reader would need its own
// guard. The bound methods call it on the way out, so the engine itself can
// keep the idiomatic zero value.
func (s *BoardSummary) EnsureDropped() {
	if s.Dropped == nil {
		s.Dropped = []string{}
	}
}

// boardParts is what one board's read step gathers before anything is
// written for it: its columns, its sprints, and the issue keys of every
// scope, keyed by sprint id with "" for the board's own list.
type boardParts struct {
	columns []backend.BoardColumn
	sprints []backend.Sprint
	keys    map[string][]string
}

// SyncBoards pulls a project's boards, their columns, their sprints, and
// the issue keys each holds, then removes any board Jira no longer
// returns. It is reached through backend.BoardBackend, a capability off
// the engine's own backend rather than a widening of IssueBackend, so a
// backend that cannot speak Jira's Agile API is simply skipped.
//
// Every board is read in full before anything is written for it: its
// columns, its sprints, its own issue list, and its active and future
// sprints' issue keys. A board whose read or write fails at any point is
// recorded in Dropped with its name and one readable line of the reason,
// the pass carries on with the next board, and whatever that board held
// before this run is left exactly as it was. Once every read for a board
// has succeeded, ReplaceBoard writes all of it in one transaction, so a
// reader never catches a board with some of its parts replaced and the
// rest still old.
//
// An instance with no Agile API at all answers Boards with ErrNoAgile,
// which is not a failure: the summary comes back with Unavailable true and
// no error, the boards already cached are left alone, and the
// boards_unavailable profile setting records the fact for the view. That
// setting is written whenever the boards call answers, either way: "true"
// on ErrNoAgile, cleared as soon as boards are found again, so a profile
// repointed at a Jira Software instance stops claiming there are none. A
// pass that never ran, and a Boards call that failed for some other
// reason, leave the setting alone: a 500 says nothing about whether the
// instance has an Agile API.
func (e *Engine) SyncBoards(ctx context.Context, profileID, projectKey string, onProgress func(Progress)) (BoardSummary, error) {
	emit := func(p Progress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	var sum BoardSummary
	if e.Boards == nil {
		return sum, nil
	}
	bb, ok := e.b.(backend.BoardBackend)
	if !ok {
		log.Printf("tam: backend has no board capability, skipping the boards sync for %s", profileID)
		return sum, nil
	}

	start := e.Now().UTC()
	// elapsed goes on every return, not just the happy one: the frontend
	// shows the summary beside the error.
	elapsed := func() string { return e.Now().Sub(start).Round(time.Millisecond).String() }

	boards, err := bb.Boards(ctx, projectKey)
	if err != nil {
		sum.Elapsed = elapsed()
		if errors.Is(err, corejira.ErrNoAgile) {
			if serr := e.repo.SetProfileSetting(ctx, profileID, settingBoardsUnavailable, "true"); serr != nil {
				return sum, serr
			}
			sum.Unavailable = true
			return sum, nil
		}
		return sum, err
	}
	if err := e.repo.SetProfileSetting(ctx, profileID, settingBoardsUnavailable, ""); err != nil {
		sum.Elapsed = elapsed()
		return sum, err
	}

	boards, foreign := ownBoards(boards, projectKey, e.AllProjectBoards)
	for _, name := range foreign {
		log.Printf("tam: board %q belongs to another project; not syncing it for %s", name, projectKey)
	}
	sum.Foreign = len(foreign)

	existing, err := e.Boards.ListBoards(ctx, profileID)
	if err != nil {
		sum.Elapsed = elapsed()
		return sum, err
	}
	jiraIDs := make(map[int]bool, len(boards))
	for _, b := range boards {
		jiraIDs[b.ID] = true
	}

	for i, b := range boards {
		emit(Progress{Phase: "boards", Fetched: i, Total: len(boards), Stage: b.Name})
		boardStart := e.Now()
		parts, err := e.readBoard(ctx, bb, b, projectKey)
		// Per board, so a slow pass can be attributed to a board rather than
		// guessed at: this walk is one request per column set, per sprint
		// page, and per page of every scope's issue keys.
		if d := e.Now().Sub(boardStart); d > slowBoard {
			log.Printf("tam: board %q took %s to read (%d sprints, %d cards)",
				b.Name, d.Round(time.Millisecond), len(parts.sprints), distinctKeys(parts.keys))
		}
		if err != nil {
			sum.Dropped = append(sum.Dropped, fmt.Sprintf("%s: %s", b.Name, errtext.Line(err)))
			continue
		}
		if err := e.Boards.ReplaceBoard(ctx, profileID, b, parts.columns, parts.sprints, parts.keys); err != nil {
			// A write that fails is the same kind of trouble as a read that
			// fails: this board is dropped with its reason and the pass
			// carries on, so one bad board never costs every board after
			// it its sync, and RemoveBoards below still runs.
			sum.Dropped = append(sum.Dropped, fmt.Sprintf("%s: %s", b.Name, errtext.Line(err)))
			continue
		}
		sum.Boards++
		sum.Columns += len(parts.columns)
		sum.Sprints += len(parts.sprints)
		sum.Cards += distinctKeys(parts.keys)
	}

	var toRemove []int
	for _, b := range existing {
		if !jiraIDs[b.ID] {
			toRemove = append(toRemove, b.ID)
		}
	}
	if err := e.Boards.RemoveBoards(ctx, profileID, toRemove); err != nil {
		sum.Elapsed = elapsed()
		return sum, err
	}

	sum.Elapsed = elapsed()
	return sum, nil
}

// readBoard gathers everything one board needs before any of it is
// written: its columns, its sprints, its own issue list, and the issue
// keys of its active and future sprints, every scope narrowed to the
// project being synced. Only that project's issues are ever in the cache,
// so a key outside it could not be drawn anyway, and reading the board
// entire cost a minute on a board whose filter spans far more than the
// project (8,485 cards against the project's 38). Closed sprints are kept in the
// sprint list but their keys are never fetched: the picker only offers
// what a sync actually pulled membership for. Any failure here means
// nothing is written for this board and its previous copy, if it has one,
// stays exactly as it was.
func (e *Engine) readBoard(ctx context.Context, bb backend.BoardBackend, b backend.Board, projectKey string) (boardParts, error) {
	cols, err := bb.BoardColumns(ctx, b.ID)
	if err != nil {
		return boardParts{}, err
	}
	sprints, err := bb.BoardSprints(ctx, b.ID)
	if err != nil {
		return boardParts{}, err
	}
	ownKeys, err := bb.BoardIssueKeys(ctx, b.ID, "", projectKey)
	if err != nil {
		return boardParts{}, err
	}
	keys := map[string][]string{"": ownKeys}
	for _, s := range sprints {
		if s.State != "active" && s.State != "future" {
			continue
		}
		sid := strconv.Itoa(s.ID)
		sprintKeys, err := bb.BoardIssueKeys(ctx, b.ID, sid, projectKey)
		if err != nil {
			return boardParts{}, err
		}
		keys[sid] = sprintKeys
	}
	return boardParts{columns: cols, sprints: sprints, keys: keys}, nil
}

// ownBoards splits the boards Jira answered with into this project's own and
// the rest. Jira's board list answers with every board whose *filter*
// mentions the project, so a board another team owns comes back too: one
// seen in the field held 8,485 cards while the project being synced had 38,
// and reading it cost a minute of every sync for cards that were not the
// project's to draw. A board whose home the instance did not report is kept:
// an instance that sends no location must not lose every board.
//
// all keeps them anyway, for the profile that genuinely works across a
// programme board.
func ownBoards(boards []backend.Board, projectKey string, all bool) (own []backend.Board, foreign []string) {
	if all {
		return boards, nil
	}
	own = make([]backend.Board, 0, len(boards))
	for _, b := range boards {
		if b.ProjectKey == "" || strings.EqualFold(b.ProjectKey, projectKey) {
			own = append(own, b)
			continue
		}
		foreign = append(foreign, b.Name)
	}
	return own, foreign
}

// distinctKeys counts the issue keys one board holds across all its scopes,
// counting a card that is both on the board's own list and in one of its
// sprints once.
func distinctKeys(keys map[string][]string) int {
	seen := make(map[string]bool)
	for _, scope := range keys {
		for _, key := range scope {
			seen[key] = true
		}
	}
	return len(seen)
}
