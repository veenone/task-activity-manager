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

// CatalogSetResult is what a bulk set did: the keys it journaled an edit
// for, and the keys it skipped because they belong to another project.
type CatalogSetResult struct {
	Changed []string `json:"changed"`
	Skipped []string `json:"skipped"`
}

// SetCatalog sets one catalog field, components or fixVersions, on many
// issues in one transaction: the Backlog's Set release and Set components.
// add unions ids with what each issue already has, keeping the issue's own
// order first; otherwise ids replace them. An issue outside project is
// skipped, because the catalog is the profile's project's and its ids mean
// nothing on another project's issue. An issue that already holds the
// result journals nothing. Any refused id fails the whole batch, so a
// selection is never left half set.
func (r *Repository) SetCatalog(ctx context.Context, profileID, project string, keys []string, field string, ids []int, add bool) (CatalogSetResult, error) {
	res := CatalogSetResult{Changed: []string{}, Skipped: []string{}}
	if !isCatalogField(field) {
		return res, fmt.Errorf("field %q is not components or fixVersions", field)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	for _, key := range keys {
		var own string
		err := tx.QueryRowContext(ctx, `SELECT project FROM issue WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&own)
		if errors.Is(err, sql.ErrNoRows) {
			return res, fmt.Errorf("%s: %w", key, ErrNotFound)
		}
		if err != nil {
			return res, fmt.Errorf("read %s: %w", key, err)
		}
		if own != project {
			res.Skipped = append(res.Skipped, key)
			continue
		}
		current, updated, _, err := readField(ctx, tx, profileID, key, field)
		if err != nil {
			return res, fmt.Errorf("%s: %w", key, err)
		}
		next := ids
		if add {
			have, err := backend.ParseIDs(current)
			if err != nil {
				return res, fmt.Errorf("%s: %w", key, err)
			}
			next = append(have, ids...)
		}
		value := normalizeCatalogValue(field, backend.FormatIDs(next))
		if value == current {
			continue
		}
		if err := checkCatalogIDs(ctx, tx, profileID, field, current, value); err != nil {
			return res, fmt.Errorf("%s: %w", key, err)
		}
		if err := journalFieldEdit(ctx, tx, profileID, key, field, current, value, updated, ""); err != nil {
			return res, err
		}
		res.Changed = append(res.Changed, key)
	}
	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

// journalFieldEdit writes a checked value on the row and journals it: a
// draft's create JSON is rewritten, any other issue gets an edit row. note
// is what the audit records.
func journalFieldEdit(ctx context.Context, tx *sql.Tx, profileID, key, field, current, value, updated, note string) error {
	if err := writeField(ctx, tx, profileID, key, field, value); err != nil {
		return err
	}
	var err error
	if strings.HasPrefix(key, DraftPrefix) {
		err = updateDraftJSON(ctx, tx, profileID, key, field, value)
	} else {
		err = journal.Upsert(tx, profileID, EntityIssue, key, field, current, value, updated)
	}
	if err != nil {
		return err
	}
	return journal.Audit(tx, profileID, EntityIssue, key, "edit", field, current, value, note)
}
