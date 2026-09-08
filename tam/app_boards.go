package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"agile-suite/tam/internal/backend"
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

// The three board writes are shaped like EditIssue in app_writes.go: check
// the store, check what the caller sent, call the repository, return. None
// of them takes the busy guard. acquire covers the five long operations
// that talk to Jira, and a local journal write is not one of them: giving
// these one would refuse a drag while a commit runs, which no other write
// does. The race that leaves is the commit pass's to settle, in its
// conditional delete, rather than the board's to prevent by locking the
// user out of it.

// requireMove is the check every board write starts with: a profile to
// write against and an issue to write about.
func requireMove(profileID, key string) error {
	if strings.TrimSpace(profileID) == "" {
		return errors.New("no profile selected")
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("issue key is empty")
	}
	return nil
}

// MoveIssueToColumn journals a card dragged into another column. statusID
// is the status that column collects; which transition reaches it is the
// backend's to resolve at Commit, from what Jira offers for that issue at
// that moment.
func (a *App) MoveIssueToColumn(profileID, key, statusID string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if err := requireMove(profileID, key); err != nil {
		return err
	}
	return a.repo.MoveToColumn(a.ctx, profileID, key, statusID)
}

// RankIssue journals a card dropped before or after neighbourKey on the
// board the drop was made on. The repository refuses a neighbour the cache
// does not hold, so a rank can never be pushed against an issue nobody has
// seen.
func (a *App) RankIssue(profileID, key, neighbourKey string, before bool, boardID int) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if err := requireMove(profileID, key); err != nil {
		return err
	}
	return a.repo.RankIssue(a.ctx, profileID, key, strings.TrimSpace(neighbourKey), before, boardID)
}

// MoveIssueToSprint journals a card dropped on another sprint, or on the
// backlog, and moves it in the local cache. The destination's name comes
// from boardrepo, which owns the sprint list; the issue repository writes
// it into the row and the journal but does not read another package's
// tables to learn it. This is the one method that holds both repositories,
// which is why the lookup is here.
//
// A sprint id that is not a number is refused here rather than at Commit:
// it ends up in a URL path, and the only honest answer to "sprint fourteen"
// is that it is not a sprint id at all.
func (a *App) MoveIssueToSprint(profileID, key, sprintID string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if err := requireMove(profileID, key); err != nil {
		return err
	}
	sprintID = strings.TrimSpace(sprintID)
	if sprintID != "" {
		if _, err := strconv.Atoi(sprintID); err != nil {
			return fmt.Errorf("sprint id %q is not a number", sprintID)
		}
	}
	name, err := a.boards.SprintName(a.ctx, profileID, sprintID)
	if err != nil {
		return err
	}
	return a.repo.MoveToSprint(a.ctx, profileID, key, sprintID, name)
}

// CanTransition asks Jira whether the card can reach statusID from where it
// sits right now, and what it can reach instead. It is the one board
// binding that touches the network, and it is best effort: an error means
// the check could not be made, never that the move is illegal, so the
// caller shows nothing rather than a warning it cannot stand behind.
func (a *App) CanTransition(profileID, key, statusID string) (backend.TransitionCheck, error) {
	if err := requireMove(profileID, key); err != nil {
		return backend.TransitionCheck{}, err
	}
	_, b, err := a.backendForProfile(profileID)
	if err != nil {
		return backend.TransitionCheck{}, err
	}
	// The same set the commit will try: the whole column, the dropped-on
	// status first, so the optimistic check and the push agree about what
	// the drop means. A local read, so a failure only costs the siblings.
	targets, terr := a.boardOrder().ColumnStatuses(a.ctx, profileID, statusID)
	if terr != nil || len(targets) == 0 {
		targets = []string{statusID}
	}
	check, err := b.CanTransition(a.ctx, key, targets)
	if err != nil {
		return backend.TransitionCheck{}, err
	}
	check.Reachable = backend.NonNil(check.Reachable)
	return check, nil
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
	// Acquired under its own name, not "sync": this holds the same per-profile
	// lock a sync does, so whichever runs second is refused, and the refusal
	// has to say which one is actually running.
	if err := a.acquire(p.ID, "boards refresh"); err != nil {
		return syncer.BoardSummary{}, err
	}
	defer a.release(p.ID)

	log.Printf("tam: boards refresh started for %s (%s)", p.Name, p.ProjectKey)
	b, err := a.backendFor(p)
	if err != nil {
		return syncer.BoardSummary{}, err
	}
	eng := syncer.New(b, a.repo)
	eng.Boards = a.boards
	eng.AllProjectBoards = a.allProjectBoards(p.ID)
	// The pass reports per board. It used to be given no progress sink at
	// all, so a refresh that walks every board's sprints, which takes
	// minutes on a real project, showed nothing anywhere and read as a
	// frozen app.
	sum, err := eng.SyncBoards(a.ctx, p.ID, p.ProjectKey, a.emitProgress)
	sum.EnsureDropped()
	if err != nil {
		log.Printf("tam: sync boards %s (%s) failed: %v", p.Name, p.ProjectKey, err)
		return sum, err
	}
	log.Printf("tam: synced boards %s (%s): %d boards, %d columns, %d sprints, %d distinct cards, %d dropped in %s",
		p.Name, p.ProjectKey, sum.Boards, sum.Columns, sum.Sprints, sum.Cards, len(sum.Dropped), sum.Elapsed)
	return sum, nil
}
