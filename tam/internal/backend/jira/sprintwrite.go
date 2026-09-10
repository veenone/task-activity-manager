package jira

import (
	"context"
	"fmt"
	"strings"

	"agile-suite/tam/internal/backend"
)

// CreateSprint creates a sprint on boardID with the draft's name, goal, and
// dates. BoardID on the returned Sprint is set from the argument, not from
// the wire, the same reason BoardSprints gives: core/jira.CreateSprint's own
// answer does not always carry it, and a sprint that arrived without one
// would otherwise file itself under board 0.
//
// A zero id in the answer, which core/jira.CreateSprint returns rather than
// treating as an error, is passed straight through: the create reached Jira,
// and the caller is expected to refresh its sprint list rather than trust
// this call's own echo of what it just made.
func (b *Backend) CreateSprint(ctx context.Context, boardID int, d backend.SprintDraft) (backend.Sprint, error) {
	raw, err := b.c.CreateSprint(ctx, boardID, d.Name, d.Goal, d.StartDate, d.EndDate)
	if err != nil {
		return backend.Sprint{}, fmt.Errorf("create sprint on board %d: %w", boardID, err)
	}
	return backend.Sprint{
		ID:        raw.ID,
		BoardID:   boardID,
		Name:      raw.Name,
		State:     strings.ToLower(raw.State),
		StartDate: raw.StartDate,
		EndDate:   raw.EndDate,
		Goal:      raw.Goal,
	}, nil
}

// EditSprint edits sprintID with the draft's name, dates, and goal,
// touching only the fields the draft carries; clearGoal sends an empty goal
// on purpose. It reaches Jira immediately, the same as StartSprint.
//
// The call below is named UpdateSprint, not EditSprint: core/jira names its
// client methods for the wire operation, a partial update, while the layers
// above it are named for the user's action, so the chain reads Service.Edit
// to BoardBackend.EditSprint to Client.UpdateSprint. The wrapped error says
// "edit" for the same reason, so it matches the method the caller invoked
// rather than the one this file calls into.
func (b *Backend) EditSprint(ctx context.Context, sprintID int, d backend.SprintDraft, clearGoal bool) error {
	if err := b.c.UpdateSprint(ctx, sprintID, d.Name, d.Goal, d.StartDate, d.EndDate, clearGoal); err != nil {
		return fmt.Errorf("edit sprint %d: %w", sprintID, err)
	}
	return nil
}

// DeleteSprint deletes sprintID. Jira returns the sprint's issues to the
// backlog; removing them from TAM's own cache is the caller's job.
func (b *Backend) DeleteSprint(ctx context.Context, sprintID int) error {
	if err := b.c.DeleteSprint(ctx, sprintID); err != nil {
		return fmt.Errorf("delete sprint %d: %w", sprintID, err)
	}
	return nil
}
