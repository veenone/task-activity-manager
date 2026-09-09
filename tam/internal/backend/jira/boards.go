package jira

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// The only consumer is a type assertion, so drift would skip the sync silently; this fails the build.
var _ backend.BoardBackend = (*Backend)(nil)

// Boards lists the project's boards, scrum and kanban only. Jira also
// serves simple boards and whatever a plugin adds; those have no column
// configuration TAM can lay out, so they are dropped with a log line rather
// than cached as a board that draws nothing.
//
// The error is wrapped with %w, so errors.Is still finds jira.ErrNoAgile and
// the sync can tell "this Jira has no Agile API" from "this call failed",
// while the message names the project it was reading. It never writes.
func (b *Backend) Boards(ctx context.Context, projectKey string) ([]backend.Board, error) {
	raw, err := b.c.Boards(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("project %s boards: %w", projectKey, err)
	}
	out := []backend.Board{}
	for _, rb := range raw {
		switch strings.ToLower(rb.Type) {
		case backend.BoardTypeScrum:
			out = append(out, backend.Board{ID: rb.ID, Name: rb.Name, Type: backend.BoardTypeScrum, ProjectKey: rb.Location.ProjectKey})
		case backend.BoardTypeKanban:
			out = append(out, backend.Board{ID: rb.ID, Name: rb.Name, Type: backend.BoardTypeKanban, ProjectKey: rb.Location.ProjectKey})
		default:
			log.Printf("tam: board %d %q is a %q board, which TAM does not draw; skipping it", rb.ID, rb.Name, rb.Type)
		}
	}
	return out, nil
}

// BoardColumns reads one board's column configuration and flattens each
// column's status objects to their ids. A column with no statuses keeps an
// empty list: that is a real board shape (a Backlog column Jira never
// fills), and the view relies on knowing about it. It never writes.
func (b *Backend) BoardColumns(ctx context.Context, boardID int) ([]backend.BoardColumn, error) {
	cfg, err := b.c.BoardConfiguration(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("board %d configuration: %w", boardID, err)
	}
	out := make([]backend.BoardColumn, 0, len(cfg.ColumnConfig.Columns))
	for _, c := range cfg.ColumnConfig.Columns {
		out = append(out, backend.BoardColumn{Name: c.Name, StatusIDs: c.StatusIDs()})
	}
	return out, nil
}

// BoardSprints lists one board's sprints. A board with none, which is what
// a kanban board is, answers with an empty slice rather than an error.
//
// BoardID is set from the argument, not from the wire: the sprint payload
// carries originBoardId, the board the sprint was created on, which is not
// always the board being read, and a sprint that arrived without it would
// otherwise file itself under board 0. State is lowercased because the
// ordering and the picker's label both work in Jira's own lowercase
// (active, future, closed). It never writes.
func (b *Backend) BoardSprints(ctx context.Context, boardID int) ([]backend.Sprint, error) {
	raw, err := b.c.Sprints(ctx, boardID)
	if errors.Is(err, corejira.ErrNoSprints) {
		return []backend.Sprint{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("board %d sprints: %w", boardID, err)
	}
	out := make([]backend.Sprint, 0, len(raw))
	for _, rs := range raw {
		out = append(out, backend.Sprint{
			ID:        rs.ID,
			BoardID:   boardID,
			Name:      rs.Name,
			State:     strings.ToLower(rs.State),
			StartDate: rs.StartDate,
			EndDate:   rs.EndDate,
		})
	}
	return out, nil
}

// BoardIssueKeys lists the keys the board holds, for one sprint when
// sprintID is set and for the whole board when it is empty, narrowed to
// projectKey. It never writes.
func (b *Backend) BoardIssueKeys(ctx context.Context, boardID int, sprintID, projectKey string) ([]string, error) {
	keys, err := b.c.BoardIssueKeys(ctx, boardID, sprintID, projectKey)
	if err != nil {
		return nil, fmt.Errorf("board %d issues: %w", boardID, err)
	}
	return keys, nil
}

// RankIssue ranks key immediately before or after neighbourKey. It passes
// straight through to the Agile call, which fails on the 207 the endpoint
// answers with when it refused the move, so a rank Jira rejected is never
// reported as landed.
func (b *Backend) RankIssue(ctx context.Context, key, neighbourKey string, before bool) error {
	if err := b.c.RankIssue(ctx, key, neighbourKey, before); err != nil {
		side := "after"
		if before {
			side = "before"
		}
		return fmt.Errorf("rank %s %s %s: %w", key, side, neighbourKey, err)
	}
	return nil
}

// MoveIssuesToSprint moves keys onto sprintID, or onto the backlog when it
// is empty: leaving every sprint is a destination of its own and Jira gives
// it its own endpoint. An empty batch asks Jira nothing.
func (b *Backend) MoveIssuesToSprint(ctx context.Context, sprintID string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if strings.TrimSpace(sprintID) == "" {
		if err := b.c.MoveToBacklog(ctx, keys); err != nil {
			return fmt.Errorf("move %s to the backlog: %w", strings.Join(keys, ", "), err)
		}
		return nil
	}
	if err := b.c.MoveToSprint(ctx, sprintID, keys); err != nil {
		return fmt.Errorf("move %s to sprint %s: %w", strings.Join(keys, ", "), sprintID, err)
	}
	return nil
}

// StartSprint starts sprintID on Jira with the draft's name, goal, and
// dates. It is one of the calls in TAM that reach Jira outside a Commit;
// see core/jira/agile.go's StartSprint doc comment for why.
func (b *Backend) StartSprint(ctx context.Context, sprintID int, s backend.SprintDraft) error {
	if err := b.c.StartSprint(ctx, sprintID, s.Name, s.Goal, s.StartDate, s.EndDate); err != nil {
		return fmt.Errorf("start sprint %d: %w", sprintID, err)
	}
	return nil
}

// CompleteSprint closes sprintID on Jira, sending state closed and nothing
// else. Moving the sprint's unfinished issues out first is the caller's
// job.
func (b *Backend) CompleteSprint(ctx context.Context, sprintID int) error {
	if err := b.c.CompleteSprint(ctx, sprintID); err != nil {
		return fmt.Errorf("complete sprint %d: %w", sprintID, err)
	}
	return nil
}
