package boardrepo

import (
	"context"
	"fmt"
)

// SprintChoice is one sprint a view can offer when there is no board on
// screen to scope it: the New issue dialog, the detail panel's sprint
// field, and the importer's Sprint column all pick from these.
//
// BoardName is why this is not a Sprint. A profile syncs several boards and
// each one names its sprints for its own team, so "Sprint 12" on its own is
// a guess in any project with more than one board; the name of the board it
// came from is what makes the choice readable. The board's id is not
// carried, because nothing a choice feeds needs it: a sprint move is pushed
// by sprint id alone.
type SprintChoice struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	BoardName string `json:"boardName"`
	State     string `json:"state"`
}

// openSprintsSQL reads every board's active and future sprints in one pass,
// joined to the board so each row can name where it came from. The state is
// Jira's own lowercase word, kept as it arrived, so the IN and the CASE
// match without folding.
//
// A sprint whose board row is gone is left out by the join rather than
// offered under no name: the sync writes a board and its sprints together,
// so that only happens to a half-removed board, and a choice nobody can
// place is worse than one fewer.
const openSprintsSQL = `
	SELECT sprint.id, sprint.name, board.name, sprint.state
	FROM sprint
	JOIN board ON board.profile_id = sprint.profile_id AND board.id = sprint.board_id
	WHERE sprint.profile_id = ? AND sprint.state IN ('active', 'future')
	ORDER BY CASE sprint.state WHEN 'active' THEN 0 ELSE 1 END, sprint.start_date, sprint.id`

// OpenSprints returns the open sprints of every board the profile has
// synced, active before future and each group by start date.
//
// A profile with no synced boards gets an empty slice and no error. Having
// no boards is not a failure: it is the ordinary state of a profile whose
// Boards view has never been refreshed, and the view says so rather than
// offering an empty select.
func (r *Repository) OpenSprints(ctx context.Context, profileID string) ([]SprintChoice, error) {
	rows, err := r.db.QueryContext(ctx, openSprintsSQL, profileID)
	if err != nil {
		return nil, fmt.Errorf("open sprints: %w", err)
	}
	defer rows.Close()
	out := []SprintChoice{}
	for rows.Next() {
		var s SprintChoice
		if err := rows.Scan(&s.ID, &s.Name, &s.BoardName, &s.State); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
