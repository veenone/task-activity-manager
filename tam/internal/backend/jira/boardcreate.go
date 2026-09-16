package jira

import (
	"context"
	"fmt"
	"strings"

	"agile-suite/tam/internal/backend"
)

// The only consumer is a type assertion, so drift would skip a board create
// silently; this fails the build instead.
var _ backend.BoardCreator = (*Backend)(nil)

// SetProjectKey records the profile's own project, the same way
// SetTransitionResolution records its setting: once, when the backend is
// built. CreateBoard is the one write on this backend that needs a project
// of its own rather than taking one as an argument, since a board and the
// filter behind it belong to exactly one project and a profile serves
// exactly one project.
func (b *Backend) SetProjectKey(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.projectKey = strings.TrimSpace(key)
}

func (b *Backend) project() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.projectKey
}

// CreateBoard makes the draft's board in two Jira calls: a filter, shared
// with the project so the board it backs is visible to the whole team, and
// then the board on that filter. When the board create fails, the filter it
// just made would otherwise be a saved filter nobody asked for and nobody
// can trace back to the attempt that made it, so it is deleted before the
// error is returned. Both outcomes go into that error: what failed, and
// that the filter was cleaned up.
//
// A failure of the cleanup itself is worse than the board create failing,
// so it is never swallowed: the returned error still leads with the board
// create's own failure and then names the filter id that was left behind,
// which is the only way anyone could find and remove it later.
func (b *Backend) CreateBoard(ctx context.Context, d backend.BoardDraft) (int, error) {
	projectKey := b.project()
	if projectKey == "" {
		return 0, fmt.Errorf("create board %q: no project is configured for this connection", d.Name)
	}
	filterID, err := b.c.CreateFilter(ctx, d.FilterName, d.JQL, "", projectKey)
	if err != nil {
		return 0, fmt.Errorf("create filter %q for board %q: %w", d.FilterName, d.Name, err)
	}
	boardID, err := b.c.CreateBoard(ctx, d.Name, d.Type, filterID, projectKey)
	if err != nil {
		if delErr := b.c.DeleteFilter(ctx, filterID); delErr != nil {
			return 0, fmt.Errorf(
				"create board %q: %w; cleanup also failed, so filter %s was left behind and needs to be deleted by hand: %v",
				d.Name, err, filterID, delErr,
			)
		}
		return 0, fmt.Errorf("create board %q: %w; filter %s was deleted", d.Name, err, filterID)
	}
	return boardID, nil
}

// AddToBoardBacklog adds keys to boardID's backlog. An empty batch asks
// Jira nothing, the same rule MoveIssuesToSprint follows.
func (b *Backend) AddToBoardBacklog(ctx context.Context, boardID int, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := b.c.AddToBoardBacklog(ctx, boardID, keys); err != nil {
		return fmt.Errorf("add %s to board %d backlog: %w", strings.Join(keys, ", "), boardID, err)
	}
	return nil
}

// The only consumer is a type assertion, so drift here would skip the
// filter check silently; this fails the build instead.
var _ backend.BoardFilterChecker = (*Backend)(nil)

// BoardFilterCheck reads which of keys are on boardID now, the committer's
// courtesy read after an issue_board push. Empty and error answers both
// pass straight through; the committer treats either as "skip the check",
// never as a reason to undo a write that already landed.
func (b *Backend) BoardFilterCheck(ctx context.Context, boardID int, keys []string) ([]string, error) {
	present, err := b.c.BoardFilterCheck(ctx, boardID, keys)
	if err != nil {
		return nil, fmt.Errorf("board %d filter check: %w", boardID, err)
	}
	return present, nil
}
