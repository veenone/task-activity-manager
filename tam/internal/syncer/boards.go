package syncer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// settingBoardsUnavailable is the profile setting the pass writes on every
// run: "true" when the instance answered ErrNoAgile, cleared the moment
// boards come back, so a profile repointed at a Jira Software instance
// stops claiming there are none.
const settingBoardsUnavailable = "boards_unavailable"

// BoardSummary is what one boards pass reports.
type BoardSummary struct {
	Boards      int      `json:"boards"`
	Columns     int      `json:"columns"`
	Sprints     int      `json:"sprints"`
	Cards       int      `json:"cards"`
	Dropped     []string `json:"dropped"`
	Unavailable bool     `json:"unavailable"`
	Elapsed     string   `json:"elapsed"`
}

// boardParts is what one board's read step gathers before anything is
// written for it: its columns, its sprints, its own issue list, and the
// issue keys of its active and future sprints, keyed by sprint id.
type boardParts struct {
	columns    []backend.BoardColumn
	sprints    []backend.Sprint
	ownKeys    []string
	sprintKeys map[string][]string
}

// SyncBoards pulls a project's boards, their columns, their sprints, and
// the issue keys each holds, then removes any board Jira no longer
// returns. It is reached through backend.BoardBackend, a capability off
// the engine's own backend rather than a widening of IssueBackend, so a
// backend that cannot speak Jira's Agile API is simply skipped.
//
// Every board is read in full before anything is written for it: its
// columns, its sprints, its own issue list, and its active and future
// sprints' issue keys. A board whose read fails at any point is recorded
// in Dropped with its name and the trimmed reason, and whatever that board
// held before this run is left exactly as it was. Only once every read for
// a board has succeeded does the pass write it, so a reader never catches
// a board with some of its parts replaced and the rest still old.
//
// An instance with no Agile API at all answers Boards with ErrNoAgile,
// which is not a failure: the summary comes back with Unavailable true and
// no error, the boards already cached are left alone, and the
// boards_unavailable profile setting records the fact for the view. That
// setting is written on every run, cleared as soon as boards are found
// again, so a profile repointed at a Jira Software instance stops claiming
// there are none.
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

	boards, err := bb.Boards(ctx, projectKey)
	if err != nil {
		if errors.Is(err, corejira.ErrNoAgile) {
			if serr := e.repo.SetProfileSetting(ctx, profileID, settingBoardsUnavailable, "true"); serr != nil {
				return sum, serr
			}
			sum.Unavailable = true
			sum.Elapsed = e.Now().Sub(start).Round(time.Millisecond).String()
			return sum, nil
		}
		return sum, err
	}
	if err := e.repo.SetProfileSetting(ctx, profileID, settingBoardsUnavailable, ""); err != nil {
		return sum, err
	}

	existing, err := e.Boards.ListBoards(ctx, profileID)
	if err != nil {
		return sum, err
	}
	jiraIDs := make(map[int]bool, len(boards))
	for _, b := range boards {
		jiraIDs[b.ID] = true
	}

	for _, b := range boards {
		emit(Progress{Phase: "boards", Stage: b.Name})
		parts, err := e.readBoard(ctx, bb, b)
		if err != nil {
			sum.Dropped = append(sum.Dropped, fmt.Sprintf("%s: %s", b.Name, err.Error()))
			continue
		}
		if err := e.writeBoard(ctx, profileID, b, start, parts); err != nil {
			return sum, err
		}
		sum.Boards++
		sum.Columns += len(parts.columns)
		sum.Sprints += len(parts.sprints)
		sum.Cards += len(parts.ownKeys)
		for _, keys := range parts.sprintKeys {
			sum.Cards += len(keys)
		}
	}

	var toRemove []int
	for _, b := range existing {
		if !jiraIDs[b.ID] {
			toRemove = append(toRemove, b.ID)
		}
	}
	if err := e.Boards.RemoveBoards(ctx, profileID, toRemove); err != nil {
		return sum, err
	}

	sum.Elapsed = e.Now().Sub(start).Round(time.Millisecond).String()
	return sum, nil
}

// readBoard gathers everything one board needs before any of it is
// written: its columns, its sprints, its own issue list, and the issue
// keys of its active and future sprints. Closed sprints are kept in the
// sprint list but their keys are never fetched: the picker only offers
// what a sync actually pulled membership for. Any failure here means
// nothing is written for this board and its previous copy, if it has one,
// stays exactly as it was.
func (e *Engine) readBoard(ctx context.Context, bb backend.BoardBackend, b backend.Board) (boardParts, error) {
	cols, err := bb.BoardColumns(ctx, b.ID)
	if err != nil {
		return boardParts{}, err
	}
	sprints, err := bb.BoardSprints(ctx, b.ID)
	if err != nil {
		return boardParts{}, err
	}
	ownKeys, err := bb.BoardIssueKeys(ctx, b.ID, "")
	if err != nil {
		return boardParts{}, err
	}
	sprintKeys := map[string][]string{}
	for _, s := range sprints {
		if s.State != "active" && s.State != "future" {
			continue
		}
		sid := strconv.Itoa(s.ID)
		keys, err := bb.BoardIssueKeys(ctx, b.ID, sid)
		if err != nil {
			return boardParts{}, err
		}
		sprintKeys[sid] = keys
	}
	return boardParts{columns: cols, sprints: sprints, ownKeys: ownKeys, sprintKeys: sprintKeys}, nil
}

// writeBoard replaces one board's row, columns, sprints, and issue keys.
// It is only ever called once readBoard has succeeded in full, so a write
// failure here is a real store error, not a board Jira could not answer
// for; the caller aborts the pass rather than guess at a partial state.
func (e *Engine) writeBoard(ctx context.Context, profileID string, b backend.Board, syncedAt time.Time, parts boardParts) error {
	if err := e.Boards.UpsertBoards(ctx, profileID, []backend.Board{b}, syncedAt); err != nil {
		return err
	}
	if err := e.Boards.UpsertColumns(ctx, profileID, b.ID, parts.columns); err != nil {
		return err
	}
	if err := e.Boards.UpsertSprints(ctx, profileID, b.ID, parts.sprints); err != nil {
		return err
	}
	if err := e.Boards.UpsertIssueKeys(ctx, profileID, b.ID, "", parts.ownKeys); err != nil {
		return err
	}
	for sprintID, keys := range parts.sprintKeys {
		if err := e.Boards.UpsertIssueKeys(ctx, profileID, b.ID, sprintID, keys); err != nil {
			return err
		}
	}
	return nil
}
