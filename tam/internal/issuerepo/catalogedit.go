package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"agile-suite/tam/internal/backend"
)

// catalogedit.go is the edit path of an issue's components and fix
// versions. The journal holds the ids as a comma list (backend.FormatIDs),
// the row holds them as the JSON array sync writes, and names are only ever
// read through project_component and project_version, so a rename touches
// no issue.

// isCatalogField says the field holds catalog ids.
func isCatalogField(field string) bool {
	return field == "components" || field == "fixVersions"
}

// normalizeCatalogValue rewrites a catalog value in the journal's own form,
// so "10001,10000" and "10001, 10000" are the same edit and a value equal to
// the row's journals nothing. Any other field, or a value that does not
// parse, is returned as given for validateField to judge.
func normalizeCatalogValue(field, value string) string {
	if !isCatalogField(field) {
		return value
	}
	ids, err := backend.ParseIDs(value)
	if err != nil {
		return value
	}
	return backend.FormatIDs(ids)
}

// writeCatalogField stores a catalog value on the row as its JSON array.
func writeCatalogField(ctx context.Context, q execer, profileID, key, field, value string) error {
	ids, err := backend.ParseIDs(value)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE issue SET `+fieldColumns[field]+` = ? WHERE profile_id = ? AND key = ?`, string(encoded), profileID, key)
	return err
}

// checkCatalogIDs refuses an id the edit adds that the profile's catalog
// does not hold, and a fix version that is released or archived. An id the
// issue already had passes whatever its state: a shipped release stays on
// the issues it shipped with, it just cannot be picked again. Ids are
// checked against the catalog rather than trusted from the caller because a
// stale panel can hold one a sync has since removed.
func checkCatalogIDs(ctx context.Context, q execer, profileID, field, current, next string) error {
	if !isCatalogField(field) {
		return nil
	}
	had, err := backend.ParseIDs(current)
	if err != nil {
		return err
	}
	want, err := backend.ParseIDs(next)
	if err != nil {
		return err
	}
	kept := map[int]bool{}
	for _, id := range had {
		kept[id] = true
	}
	for _, id := range want {
		if kept[id] {
			continue
		}
		if err := checkCatalogID(ctx, q, profileID, field, id); err != nil {
			return err
		}
	}
	return nil
}

func checkCatalogID(ctx context.Context, q execer, profileID, field string, id int) error {
	if field == "components" {
		var name string
		err := q.QueryRowContext(ctx, `SELECT name FROM project_component WHERE profile_id = ? AND id = ?`, profileID, id).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("component %d is not in this project; sync and pick again", id)
		}
		return err
	}
	var (
		name               string
		released, archived bool
	)
	err := q.QueryRowContext(ctx, `SELECT name, released, archived FROM project_version WHERE profile_id = ? AND id = ?`, profileID, id).Scan(&name, &released, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("release %d is not in this project; sync and pick again", id)
	}
	if err != nil {
		return err
	}
	if released || archived {
		return fmt.Errorf("release %s is released or archived, so it cannot be added to an issue", name)
	}
	return nil
}
