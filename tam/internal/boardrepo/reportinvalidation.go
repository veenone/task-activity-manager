package boardrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"agile-suite/tam/internal/backend"
)

type sprintReportInput struct {
	state     string
	startDate string
	endDate   string
}

// invalidateChangedSprintReports removes only the stored series whose sprint
// inputs are about to change. Reports reconstruct from the start through the
// actual close of a closed sprint when Jira supplies one, falling back to its
// planned end otherwise; a removed sprint has no valid report left to serve.
// The caller runs this inside the same transaction as the sprint replacement.
func invalidateChangedSprintReports(ctx context.Context, tx *sql.Tx, profileID string, boardID int, next []backend.Sprint) error {
	before, err := storedSprintReportInputs(ctx, tx, profileID, boardID)
	if err != nil {
		return err
	}
	after := make(map[int]sprintReportInput, len(next))
	for _, sprint := range next {
		after[sprint.ID] = reportInput(sprint.State, sprint.StartDate, sprint.EndDate, sprint.CompleteDate)
	}
	for sprintID, old := range before {
		current, kept := after[sprintID]
		if kept && old == current {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM sprint_report WHERE profile_id = ? AND board_id = ? AND sprint_id = ?`,
			profileID, boardID, sprintID); err != nil {
			return fmt.Errorf("clear report for board %d sprint %d after its dates or state changed: %w", boardID, sprintID, err)
		}
	}
	return nil
}

func storedSprintReportInputs(ctx context.Context, tx *sql.Tx, profileID string, boardID int) (map[int]sprintReportInput, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, state, start_date, end_date, complete_date FROM sprint WHERE profile_id = ? AND board_id = ?`,
		profileID, boardID)
	if err != nil {
		return nil, fmt.Errorf("read report inputs of board %d's sprints: %w", boardID, err)
	}
	out := map[int]sprintReportInput{}
	for rows.Next() {
		var id int
		var state, startDate, endDate, completeDate string
		if err := rows.Scan(&id, &state, &startDate, &endDate, &completeDate); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("read report inputs of board %d's sprint: %w", boardID, err)
		}
		out[id] = reportInput(state, startDate, endDate, completeDate)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("read report inputs of board %d's sprints: %w", boardID, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close report inputs of board %d's sprints: %w", boardID, err)
	}
	return out, nil
}

func reportInput(state, startDate, endDate, completeDate string) sprintReportInput {
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "closed" && strings.TrimSpace(completeDate) != "" {
		endDate = completeDate
	}
	return sprintReportInput{state: state, startDate: startDate, endDate: endDate}
}
