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

// LinkField is the journal field of a link row: type, direction, and
// target, so the journal's uniqueness refuses the same link twice.
func LinkField(d backend.LinkDraft) string {
	return d.Type + "|" + d.Direction + "|" + d.ToKey
}

// AddLink journals a link from key to the draft's target. The source must
// be a cached issue (a draft counts), the target another key, and the same
// link must not exist yet, cached or pending.
func (r *Repository) AddLink(ctx context.Context, profileID, key string, d backend.LinkDraft) error {
	d.Type = strings.TrimSpace(d.Type)
	d.ToKey = strings.TrimSpace(d.ToKey)
	switch {
	case d.Type == "":
		return errors.New("link type is empty")
	case d.Direction != "outward" && d.Direction != "inward":
		return fmt.Errorf("link direction %q is neither outward nor inward", d.Direction)
	case d.ToKey == "":
		return errors.New("target issue key is empty")
	case strings.EqualFold(d.ToKey, key):
		return errors.New("an issue cannot link to itself")
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode link: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM issue WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("check %s exists: %w", key, err)
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM issue_link WHERE profile_id = ? AND from_key = ? AND link_type = ? AND direction = ? AND to_key = ?`,
		profileID, key, d.Type, d.Direction, d.ToKey).Scan(&exists); err == nil {
		return fmt.Errorf("%s is already linked to %s that way", key, d.ToKey)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check link exists: %w", err)
	}
	field := LinkField(d)
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, EntityLink, key, field).Scan(&exists); err == nil {
		return fmt.Errorf("a link from %s to %s is already pending", key, d.ToKey)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check pending link: %w", err)
	}
	if err := journal.Put(tx, profileID, EntityLink, key, field, "", string(encoded), ""); err != nil {
		return err
	}
	if err := journal.Audit(tx, profileID, EntityLink, key, "link", field, "", d.ToKey, ""); err != nil {
		return err
	}
	return tx.Commit()
}

// PendingLinks returns the links the journal holds for key, as Link rows
// flagged pending with their journal ids.
func (r *Repository) PendingLinks(ctx context.Context, profileID, key string) ([]backend.Link, error) {
	rows, err := journal.ListForKey(r.db, profileID, key)
	if err != nil {
		return nil, err
	}
	links := []backend.Link{}
	for _, p := range rows {
		if p.EntityType != EntityLink {
			continue
		}
		var d backend.LinkDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			return nil, fmt.Errorf("decode pending link %d: %w", p.ID, err)
		}
		links = append(links, backend.Link{
			Direction: d.Direction, Type: d.Type, Key: d.ToKey, Summary: d.ToSummary, IssueType: d.ToType,
			Pending: true, PendingID: p.ID,
		})
	}
	return links, nil
}
