package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// journalDraftSprint turns the sprint a draft was created with into a sprint
// move under the key Jira has just handed back. Rekey calls it, in the same
// transaction that repoints the draft's other rows, so a draft's sprint
// travels the way its parent already does.
//
// The sprint is not sent with the create. The Sprint field is missing from
// most Data Center create screens, so a create carrying one is refused on
// exactly the instances TAM is built for, and the create path deliberately
// sends none of the board state for that reason. The Agile move endpoint is
// the one that always works, the board pass of the same Commit already
// pushes sprint moves, and it reads the journal again after the creates, so
// a row written here lands in the same Commit that created the issue.
//
// The before value is empty and not the draft's own sprint: Jira has just
// put the issue in the backlog, and that is what the board pass compares
// the remote against before it pushes. The base version is empty for the
// same reason a board row never checks one: a sprint move is judged against
// the remote sprint, not against an updated stamp.
//
// A draft with no sprint journals nothing at all.
func journalDraftSprint(ctx context.Context, tx *sql.Tx, profileID, key string) error {
	var encoded string
	err := tx.QueryRowContext(ctx,
		`SELECT after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, EntityIssueCreate, key, FieldCreate).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create row of %s: %w", key, err)
	}
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(encoded), &d); err != nil {
		return fmt.Errorf("draft of %s: %w", key, err)
	}
	if strings.TrimSpace(d.SprintID) == "" {
		return nil
	}
	after := MoveValue(d.SprintID, d.SprintName)
	if err := journal.Put(tx, profileID, EntitySprintMove, key, FieldSprintID, "", after, ""); err != nil {
		return err
	}
	if err := applyMoveColumns(ctx, tx, profileID, key, EntitySprintMove, after); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, EntitySprintMove, key, "move", FieldSprintID, "", after,
		"the sprint the draft was created into")
}
