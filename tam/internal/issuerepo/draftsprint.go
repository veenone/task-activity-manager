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
//
// A draft can never already hold its own issue_sprint row when this runs,
// and that matters: journal.Put upserts on (profile, entity_type,
// entity_key, field), and Rekey repoints every pending row from the temp key
// to the real one before calling here, so a row under the real key at this
// point would be overwritten with the create JSON's original sprint, undoing
// whatever the user chose afterwards, and the board pass would then see
// before equal to after and different from remote and raise a bogus
// conflict on an issue created seconds ago. It cannot happen only because a
// drag or a menu move on a draft goes through moveDraft (boardwrites.go)
// instead: that rewrites the draft's own JSON in place and journals nothing,
// so the create row this function reads always carries the draft's current
// sprint, whichever choice was made last.
//
// The applyMoveColumns call below writes the two columns CreateDrafts has
// already written from the same draft. Neither write is dead: the create
// writes what the Backlog and the panel read before Commit, and this one is
// what every other sprint move does to the cached row, so the path stays the
// same shape as the one a drag takes. They agree because they read the one
// draft.
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
