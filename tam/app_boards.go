package main

import (
	"log"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/syncer"
)

// ListBoards returns the profile's cached boards, by name.
func (a *App) ListBoards(profileID string) ([]boardrepo.Board, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	boards, err := a.boards.ListBoards(a.ctx, profileID)
	if err != nil {
		return nil, err
	}
	if boards == nil {
		boards = []boardrepo.Board{}
	}
	return boards, nil
}

// ListBoardSprints returns one board's cached sprints, active first, then
// future, then closed.
func (a *App) ListBoardSprints(profileID string, boardID int) ([]boardrepo.Sprint, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	sprints, err := a.boards.ListSprints(a.ctx, profileID, boardID)
	if err != nil {
		return nil, err
	}
	if sprints == nil {
		sprints = []boardrepo.Sprint{}
	}
	return sprints, nil
}

// GetBoard composes the Boards view's data from the cache: the board's
// columns in order, the cached cards bucketed into them, and the lanes the
// chosen swimlane asks for. a.repo is the IssueSource: the cards themselves
// live in the issue cache, not in boardrepo's own tables.
func (a *App) GetBoard(profileID string, boardID int, sprintID, swimlane string) (boardrepo.BoardView, error) {
	if err := a.requireStore(); err != nil {
		return boardrepo.BoardView{}, err
	}
	return a.boards.Board(a.ctx, a.repo, profileID, boardID, sprintID, swimlane)
}

// SyncBoards pulls the profile's boards, sprints, and issue keys. It runs
// under the same busy guard as SyncIssues, so a sync, a commit, and a
// boards sync never overlap for one profile.
//
// Unlike SyncIssues, it emits no progress frames. The frontend's sync
// reducer only understands a run that its own SYNC_START opened; a frame
// arriving with no such run behind it would be a second, undeclared way
// into shared state. The Refresh button that calls this owns its own
// pending label instead.
func (a *App) SyncBoards(profileID string) (syncer.BoardSummary, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return syncer.BoardSummary{}, err
	}
	if err := a.acquire(p.ID, "sync"); err != nil {
		return syncer.BoardSummary{}, err
	}
	defer a.release(p.ID)

	b, err := a.backendFor(p)
	if err != nil {
		return syncer.BoardSummary{}, err
	}
	eng := syncer.New(b, a.repo)
	eng.Boards = a.boards
	sum, err := eng.SyncBoards(a.ctx, p.ID, p.ProjectKey, nil)
	sum.EnsureDropped()
	if err != nil {
		log.Printf("tam: sync boards %s (%s) failed: %v", p.Name, p.ProjectKey, err)
		return sum, err
	}
	log.Printf("tam: synced boards %s (%s): %d boards, %d columns, %d sprints, %d distinct cards, %d dropped in %s",
		p.Name, p.ProjectKey, sum.Boards, sum.Columns, sum.Sprints, sum.Cards, len(sum.Dropped), sum.Elapsed)
	return sum, nil
}
