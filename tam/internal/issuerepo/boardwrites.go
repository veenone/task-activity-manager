package issuerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// The three writes a board drag makes. Each one is a transaction that reads
// the issue's current value and its updated stamp, refuses a key the cache
// does not hold, journals the intent against that stamp, audits it, and
// writes the local column so the view repaints where the card was dropped.
// Nothing here talks to Jira: the journal is what Commit pushes. What a
// journaled value means in columns, and what putting one back may
// overwrite, is movecolumns.go.

// boardRow is what a board write reads before it decides anything: the two
// columns a move can change and the version the intent is journaled
// against.
type boardRow struct {
	status     string
	statusID   string
	sprintID   string
	sprintName string
	updated    string
}

// readBoardRow reads one issue's move-relevant columns, or ErrNotFound.
func readBoardRow(ctx context.Context, q execer, profileID, key string) (boardRow, error) {
	var row boardRow
	err := q.QueryRowContext(ctx,
		`SELECT status, status_id, sprint_id, sprint_name, updated FROM issue WHERE profile_id = ? AND key = ?`,
		profileID, key).Scan(&row.status, &row.statusID, &row.sprintID, &row.sprintName, &row.updated)
	if errors.Is(err, sql.ErrNoRows) {
		return boardRow{}, ErrNotFound
	}
	if err != nil {
		return boardRow{}, fmt.Errorf("read %s: %w", key, err)
	}
	return row, nil
}

// MoveToColumn journals a card dragged across columns and moves it locally.
// statusID is the Jira status the target column collects; the transition
// that reaches it is the backend's to resolve at Commit, since a transition
// id read now would be stale the moment the issue moved.
func (r *Repository) MoveToColumn(ctx context.Context, profileID, key, statusID string) error {
	statusID = strings.TrimSpace(statusID)
	if statusID == "" {
		return errors.New("a column move needs the status the column collects")
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		row, err := readBoardRow(ctx, tx, profileID, key)
		if err != nil {
			return err
		}
		name, err := statusNameFor(ctx, tx, profileID, statusID)
		if err != nil {
			return err
		}
		return recordMove(ctx, tx, profileID, key, EntityTransition, FieldStatusID,
			MoveValue(row.statusID, row.status), MoveValue(statusID, name), row.updated)
	})
}

// MoveToSprint journals a card moved to another sprint and moves it
// locally. An empty sprintID is the backlog, which is a destination and not
// an absence.
//
// sprintName is the destination's name as the caller knows it. The sprint
// list is boardrepo's table, and this package does not read it to name its
// own destination: app.go holds both repositories and asks the one that
// owns the sprints. A caller with no name to give falls back to the name a
// cached issue in that sprint carries.
func (r *Repository) MoveToSprint(ctx context.Context, profileID, key, sprintID, sprintName string) error {
	sprintID = strings.TrimSpace(sprintID)
	sprintName = strings.TrimSpace(sprintName)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		row, err := readBoardRow(ctx, tx, profileID, key)
		if err != nil {
			return err
		}
		name := sprintName
		if name == "" {
			if name, err = sprintNameFor(ctx, tx, profileID, sprintID); err != nil {
				return err
			}
		}
		return recordMove(ctx, tx, profileID, key, EntitySprintMove, FieldSprintID,
			MoveValue(row.sprintID, row.sprintName), MoveValue(sprintID, name), row.updated)
	})
}

// RankIssue journals a card dropped before or after neighbourKey inside its
// cell, on the board the drop was made on. It writes no column, by
// Decision 4: a made-up LexoRank in the cache would be a second source of
// truth the next sync silently overwrites, so the board read is what orders
// a pending rank, by its neighbour.
func (r *Repository) RankIssue(ctx context.Context, profileID, key, neighbourKey string, before bool, boardID int) error {
	neighbourKey = strings.TrimSpace(neighbourKey)
	if neighbourKey == "" {
		return errors.New("a rank needs the card it was dropped against")
	}
	if neighbourKey == key {
		return errors.New("an issue cannot be ranked against itself")
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		row, err := readBoardRow(ctx, tx, profileID, key)
		if err != nil {
			return err
		}
		if _, err := readBoardRow(ctx, tx, profileID, neighbourKey); err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("%s is not in the cache; sync first", neighbourKey)
			}
			return err
		}
		// A rank has no cached value, so there is no before to journal and
		// nothing for its revert to put back.
		return recordMove(ctx, tx, profileID, key, EntityRank, FieldRank,
			"", RankValue(neighbourKey, before, boardID), row.updated)
	})
}

// recordMove is the body every board write shares. current is the value the
// row holds now and after the value it was dropped on; both are packed by
// MoveValue, except a rank's, whose current is empty.
//
// Three answers before anything is written. A drop on the value the journal
// already holds changes nothing. A drop back on the value the row started
// with is an undo: the journal row goes, the columns go back to the
// before_val that row was carrying, and the trail records the undo rather
// than a change to nothing. Anything else is journaled, and a second move
// replaces the first while the first row's before_val survives, or a later
// discard would put the card somewhere it never was.
//
// The undo puts a column back, so it asks first what the Discard path asks:
// the column goes back only while the row still holds the value this app
// wrote there. Both go through holdsMove, so one drag home means one thing
// whichever of them handles it.
//
// Every one of those comparisons is on ids, never on the "id|Name" text: a
// status name that differs between the board configuration and the cached
// issue row would otherwise make a card dragged home look like a move to
// somewhere new, and a sprint renamed between two identical drops would
// rewrite a row that never changed.
func recordMove(ctx context.Context, tx *sql.Tx, profileID, key, entityType, field, current, after, baseVersion string) error {
	if strings.HasPrefix(key, DraftPrefix) {
		return moveDraft(ctx, tx, profileID, key, entityType, field, current, after)
	}
	existingBefore, existingAfter, held, err := readJournaledMove(ctx, tx, profileID, entityType, key, field)
	if err != nil {
		return err
	}
	switch {
	case held && sameMove(entityType, existingAfter, after):
		return nil
	case held && sameMove(entityType, after, existingBefore):
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
			profileID, entityType, key, field); err != nil {
			return fmt.Errorf("undo %s on %s: %w", field, key, err)
		}
		if holdsMove(entityType, current, existingAfter) {
			if err := applyMoveColumns(ctx, tx, profileID, key, entityType, existingBefore); err != nil {
				return err
			}
		}
		return journal.Audit(tx, profileID, entityType, key, "undo", field, existingAfter, existingBefore, "")
	case !held && sameMove(entityType, after, current):
		return nil
	}
	if err := journal.Put(tx, profileID, entityType, key, field, current, after, baseVersion); err != nil {
		return err
	}
	if err := applyMoveColumns(ctx, tx, profileID, key, entityType, after); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, entityType, key, "move", field, current, after, "")
}

// sameMove compares two board values the way every decision here does: on
// ids for a transition and a sprint move, and on the whole text for a rank,
// whose value is a side, a neighbour, and a board with no id half to
// compare.
func sameMove(entityType, a, b string) bool {
	if entityType == EntityRank {
		return a == b
	}
	return MoveID(a) == MoveID(b)
}

// readJournaledMove returns the board row the journal already holds for one
// issue and type, and whether there is one.
func readJournaledMove(ctx context.Context, q execer, profileID, entityType, key, field string) (before, after string, held bool, err error) {
	err = q.QueryRowContext(ctx,
		`SELECT before_val, after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, entityType, key, field).Scan(&before, &after)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("read pending %s of %s: %w", field, key, err)
	}
	return before, after, true, nil
}

// moveDraft moves a draft in place. A TAM-NEW-n card has no Jira state, so
// a drag rewrites the draft's own JSON and the row's columns and journals
// nothing new: the create row is the only journal row a draft has, and
// Commit sends the draft, not the move. A rank has neither a column nor a
// draft field, so it is where a draft's drag stops.
func moveDraft(ctx context.Context, tx *sql.Tx, profileID, key, entityType, field, current, after string) error {
	if entityType == EntityRank || sameMove(entityType, after, current) {
		return nil
	}
	if entityType == EntityTransition {
		// The draft keeps the Draft status its row was created with: it has
		// no Jira status, and what puts it in a column is the status id
		// alone. Writing a real status name onto it would have the Backlog
		// claim a state Jira has never granted.
		if _, err := tx.ExecContext(ctx,
			`UPDATE issue SET status_id = ? WHERE profile_id = ? AND key = ?`,
			MoveID(after), profileID, key); err != nil {
			return fmt.Errorf("move draft %s to status %s: %w", key, MoveID(after), err)
		}
	} else if err := applyMoveColumns(ctx, tx, profileID, key, entityType, after); err != nil {
		return err
	}
	if err := editDraft(ctx, tx, profileID, key, func(d *backend.IssueDraft) {
		if entityType == EntityTransition {
			d.StatusID = MoveID(after)
			return
		}
		d.SprintID = MoveID(after)
		d.SprintName = ""
		if d.SprintID != "" {
			d.SprintName = MoveRawName(after)
		}
	}); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, entityType, key, "move", field, current, after, "")
}

// statusNameFor is the display name the cache knows for a status id: the
// one any issue already carrying that status shows. A board's column
// configuration holds status ids and no names, so this is the only place a
// name for a drop target can come from; an id the cache has never seen
// leaves the name empty and the value reads as the bare id.
func statusNameFor(ctx context.Context, q execer, profileID, statusID string) (string, error) {
	var name string
	err := q.QueryRowContext(ctx,
		`SELECT status FROM issue WHERE profile_id = ? AND status_id = ? AND status <> '' LIMIT 1`,
		profileID, statusID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("name of status %s: %w", statusID, err)
	}
	return name, nil
}

// sprintNameFor is the fallback name of a sprint id: whatever a cached
// issue already in that sprint carries. The sprint list is boardrepo's, and
// a caller that holds it hands the name in rather than having this package
// read another package's table for it.
func sprintNameFor(ctx context.Context, q execer, profileID, sprintID string) (string, error) {
	if sprintID == "" {
		return "", nil
	}
	var name string
	err := q.QueryRowContext(ctx,
		`SELECT sprint_name FROM issue WHERE profile_id = ? AND sprint_id = ? AND sprint_name <> '' LIMIT 1`,
		profileID, sprintID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("name of sprint %s: %w", sprintID, err)
	}
	return name, nil
}

// rekeyRankNeighbours repoints the rank rows that were journaled against a
// draft once that draft has its real key, keeping the side it was dropped
// on and the board it was dropped on. Without it a rank would push
// "TAM-NEW-3" to Jira, which is the bug Phase 2 already fixed once for
// parents.
func rekeyRankNeighbours(ctx context.Context, tx *sql.Tx, profileID, tempKey, realKey string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ?`,
		profileID, EntityRank)
	if err != nil {
		return fmt.Errorf("rank rows of %s: %w", tempKey, err)
	}
	type repoint struct {
		id    int64
		value string
	}
	var todo []repoint
	for rows.Next() {
		var (
			id    int64
			value string
		)
		if err := rows.Scan(&id, &value); err != nil {
			rows.Close()
			return err
		}
		neighbour, before, boardID := ParseRank(value)
		if neighbour == tempKey {
			todo = append(todo, repoint{id: id, value: RankValue(realKey, before, boardID)})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range todo {
		if _, err := tx.ExecContext(ctx,
			`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND id = ?`,
			r.value, profileID, r.id); err != nil {
			return fmt.Errorf("repoint rank %d to %s: %w", r.id, realKey, err)
		}
	}
	return nil
}
