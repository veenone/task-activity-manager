package issuerepo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// rewriteParentKey repoints every other draft whose JSON names from as its
// parent. Rekey already repoints the cached rows and every pending parentKey
// edit; a draft's own parent lives in its create JSON, which no UPDATE over
// a column can reach, and the next phase of the same Commit posts that JSON.
func rewriteParentKey(ctx context.Context, tx *sql.Tx, profileID, from, to string) error {
	drafts, err := draftsWhere(ctx, tx, profileID, func(d backend.IssueDraft) bool { return d.ParentKey == from })
	if err != nil {
		return err
	}
	for _, key := range drafts {
		if err := editDraft(ctx, tx, profileID, key, func(d *backend.IssueDraft) { d.ParentKey = to }); err != nil {
			return err
		}
	}
	return nil
}

// RekeySprint turns a draft sprint into the sprint Jira just created, in one
// transaction: the draft row takes Jira's id and values and stops being a
// draft, every card, journaled move and draft issue naming the negative id
// names the real one, and the sprint_create row goes. The board pass of the
// same Commit then pushes the moves into it.
//
// A row Jira's id already has on the draft's own board, which a boards
// refresh between the create and this call would leave, is replaced rather
// than collided with: the primary key is (profile, board, id). Another
// board's copy of the same sprint is that board's cache, boardrepo's, and
// stays.
func (r *Repository) RekeySprint(ctx context.Context, profileID string, draftID int, made backend.Sprint) error {
	from, to := strconv.Itoa(draftID), strconv.Itoa(made.ID)
	state := made.State
	if state == "" {
		state = "future"
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		_, raw, err := readDraftSprint(ctx, tx, profileID, from)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM sprint WHERE profile_id = ? AND id = ? AND draft = 0
			 AND board_id = (SELECT board_id FROM sprint WHERE profile_id = ? AND id = ? AND draft = 1)`,
			profileID, made.ID, profileID, draftID); err != nil {
			return fmt.Errorf("clear sprint %s before the rekey: %w", to, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE sprint SET id = ?, draft = 0, name = ?, state = ?, start_date = ?, end_date = ?, goal = ?
			 WHERE profile_id = ? AND id = ? AND draft = 1`,
			made.ID, made.Name, state, made.StartDate, made.EndDate, made.Goal, profileID, draftID); err != nil {
			return fmt.Errorf("rekey sprint %s to %s: %w", from, to, err)
		}
		if err := rewriteSprintID(ctx, tx, profileID, from, to, made.Name); err != nil {
			return err
		}
		if err := deleteDraftSprintRow(ctx, tx, profileID, from); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, EntitySprintCreate, from, "commit", FieldCreate, "", raw, "created in Jira as sprint "+to); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntitySprint, to, "create", "", "", made.Name, "created in Jira from draft sprint "+from)
	})
}

// MarkSprintCreatedWithoutRekey clears a draft sprint Jira accepted when the
// local rename failed: the create row and the draft row go, so a retry does
// not create a second sprint, and the trail says which sprint Jira made. The
// cards moved into the draft keep naming its negative id; the Commit that
// hit this reports them held, and the user moves them again after a boards
// refresh brings the real sprint in.
func (r *Repository) MarkSprintCreatedWithoutRekey(ctx context.Context, profileID string, draftID, realID int) error {
	from := strconv.Itoa(draftID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if err := deleteDraftSprintRow(ctx, tx, profileID, from); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM sprint WHERE profile_id = ? AND id = ? AND draft = 1`, profileID, draftID); err != nil {
			return fmt.Errorf("drop draft sprint %s: %w", from, err)
		}
		note := fmt.Sprintf("created in Jira as sprint %d but the local rename failed; refresh the board to see it, and move its cards again", realID)
		return journal.Audit(tx, profileID, EntitySprintCreate, from, "created", "", from, strconv.Itoa(realID), note)
	})
}

// deleteDraftSprintRow drops a draft sprint's sprint_create row, the one
// statement both ways out of a successful Jira create share.
func deleteDraftSprintRow(ctx context.Context, tx *sql.Tx, profileID, key string) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		profileID, EntitySprintCreate, key); err != nil {
		return fmt.Errorf("clear draft sprint %s: %w", key, err)
	}
	return nil
}
