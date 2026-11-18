package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// EntityAttachment is the journal entity type of a file to upload to an
// issue. The row's field is the file's path and its after_val the
// AttachmentDraft as JSON. The field is the path because the journal is
// unique on (type, key, field): an issue takes as many files as somebody
// picks, and picking the same file twice should leave one row, not two
// uploads. Nothing local changes when one is journalled: Jira owns the
// attachment, and TAM holds where the file is until Commit sends it.
const EntityAttachment = "attachment"

// AttachFile journals a file against an issue the cache holds, a draft
// included: nothing reaches Jira until Commit uploads it.
func (r *Repository) AttachFile(ctx context.Context, profileID, key string, d backend.AttachmentDraft) error {
	if d.Path == "" || d.Name == "" {
		return errors.New("the attachment names no file")
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode attachment: %w", err)
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM issue WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("check %s exists: %w", key, err)
		}
		if err := journal.Put(tx, profileID, EntityAttachment, key, d.Path, "", string(encoded), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityAttachment, key, "attachment", d.Path, "", d.Name, "")
	})
}

// PendingAttachments returns the files the journal holds for key, oldest
// first, as Attachment rows flagged pending with their journal ids.
func (r *Repository) PendingAttachments(ctx context.Context, profileID, key string) ([]backend.Attachment, error) {
	rows, err := journal.ListForKey(r.db, profileID, key)
	if err != nil {
		return nil, err
	}
	out := []backend.Attachment{}
	for _, p := range rows {
		if p.EntityType != EntityAttachment {
			continue
		}
		var d backend.AttachmentDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			return nil, fmt.Errorf("decode pending attachment %d: %w", p.ID, err)
		}
		out = append(out, backend.Attachment{Name: d.Name, Size: d.Size, Path: d.Path, Pending: true, PendingID: p.ID})
	}
	return out, nil
}
