package boardrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

// DeleteSprintEverywhere removes one sprint's row and its membership from
// every board of the profile that holds a copy, in one transaction.
//
// Neither delete carries a board id, and that is the reason this exists
// beside ReplaceSprints rather than reusing it. The sprint table is keyed
// (profile_id, board_id, id) because Jira Data Center hands the same
// sprint to every board whose filter reaches it, and OpenSprints joins
// across every board of the profile and folds by sprint id. A delete
// scoped to one board, the way ReplaceSprints is, would clear that
// board's row and membership and leave a second board's copy standing,
// and OpenSprints would keep offering a sprint Jira has already destroyed
// to the New issue dialog, the detail panel, and the importer's Sprint
// column, which is the entire outcome this method exists to prevent.
// Deleting by (profile_id, id) and (profile_id, sprint_id) is what reaches
// every board's copy in one call instead of one per board.
func (r *Repository) DeleteSprintEverywhere(ctx context.Context, profileID string, sprintID int) error {
	scope := strconv.Itoa(sprintID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM board_issue WHERE profile_id = ? AND sprint_id = ?`,
			profileID, scope); err != nil {
			return fmt.Errorf("clear membership of sprint %d on every board: %w", sprintID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM sprint WHERE profile_id = ? AND id = ?`,
			profileID, sprintID); err != nil {
			return fmt.Errorf("delete sprint %d from every board: %w", sprintID, err)
		}
		return nil
	})
}
