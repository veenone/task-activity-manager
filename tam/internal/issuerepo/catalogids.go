package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// catalogKind is what differs between the two catalogs an issue names by
// id, releases (fix versions) and components (#605): the issue column and
// journal field that carry the ids, the draft issue's list, and the changes
// whose payload names another entry as where issues go.
type catalogKind struct {
	noun   string
	column string
	field  string
	// drafted is the draft issue's list of this kind's ids.
	drafted func(d *backend.IssueDraft) *[]int
	// targets are the entity types whose after_val carries moveTo and
	// moveToName naming an entry of this kind.
	targets []string
}

var versionKind = catalogKind{
	noun:    "release",
	column:  "fix_versions",
	field:   "fixVersions",
	drafted: func(d *backend.IssueDraft) *[]int { return &d.FixVersions },
	targets: []string{EntityVersionRelease, EntityVersionDelete},
}

// swapID replaces from with to in ids, keeping the order and dropping
// repeats. A to of 0 removes from instead. changed says whether from was
// there at all.
func swapID(ids []int, from, to int) ([]int, bool) {
	out := make([]int, 0, len(ids))
	seen := map[int]bool{}
	changed := false
	for _, id := range ids {
		if id == from {
			changed = true
			if to == 0 {
				continue
			}
			id = to
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, changed
}

// rewriteID repoints every place an id of the kind is carried from one id
// to another: the cached issues' column, both halves of every pending edit
// of the field, every draft issue's JSON, and every change naming it as a
// move target. A to of 0 takes the id away, which is how a discarded draft
// leaves the issues it was on; an edit that then changes nothing is
// dropped. It is also how a created draft's id becomes its real one.
func (k catalogKind) rewriteID(ctx context.Context, tx *sql.Tx, profileID string, from, to int) error {
	if err := k.rewriteIssues(ctx, tx, profileID, from, to); err != nil {
		return err
	}
	if err := k.rewriteEdits(ctx, tx, profileID, from, to); err != nil {
		return err
	}
	drafts, err := draftsWhere(ctx, tx, profileID, func(d backend.IssueDraft) bool {
		_, has := swapID(*k.drafted(&d), from, to)
		return has
	})
	if err != nil {
		return err
	}
	for _, key := range drafts {
		if err := editDraft(ctx, tx, profileID, key, func(d *backend.IssueDraft) {
			ids := k.drafted(d)
			*ids, _ = swapID(*ids, from, to)
		}); err != nil {
			return err
		}
	}
	return k.rewriteTargets(ctx, tx, profileID, from, to)
}

// rewriteIssues rewrites the cached rows' column.
func (k catalogKind) rewriteIssues(ctx context.Context, tx *sql.Tx, profileID string, from, to int) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT key, `+k.column+` FROM issue WHERE profile_id = ? AND `+k.column+` LIKE ?`,
		profileID, "%"+strconv.Itoa(from)+"%")
	if err != nil {
		return fmt.Errorf("issues in %s %d: %w", k.noun, from, err)
	}
	updates := map[string]string{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			rows.Close()
			return err
		}
		var ids []int
		if json.Unmarshal([]byte(raw), &ids) != nil {
			continue
		}
		next, changed := swapID(ids, from, to)
		if !changed {
			continue
		}
		encoded, err := json.Marshal(next)
		if err != nil {
			rows.Close()
			return err
		}
		updates[key] = string(encoded)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for key, value := range updates {
		if _, err := tx.ExecContext(ctx, `UPDATE issue SET `+k.column+` = ? WHERE profile_id = ? AND key = ?`, value, profileID, key); err != nil {
			return fmt.Errorf("repoint %s from %s %d: %w", key, k.noun, from, err)
		}
	}
	return nil
}

// rewriteEdits rewrites the pending edits of the field naming from. An edit
// left with the same value on both sides changes nothing any more and is
// dropped, audited as a discard.
func (k catalogKind) rewriteEdits(ctx context.Context, tx *sql.Tx, profileID string, from, to int) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, entity_key, before_val, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND field = ?`,
		profileID, EntityIssue, k.field)
	if err != nil {
		return fmt.Errorf("%s edits: %w", k.noun, err)
	}
	type edit struct {
		id                 int64
		key, before, after string
		changed            bool
	}
	var todo []edit
	for rows.Next() {
		var e edit
		if err := rows.Scan(&e.id, &e.key, &e.before, &e.after); err != nil {
			rows.Close()
			return err
		}
		for _, v := range []*string{&e.before, &e.after} {
			ids, perr := backend.ParseIDs(*v)
			if perr != nil {
				continue
			}
			if next, changed := swapID(ids, from, to); changed {
				*v, e.changed = backend.FormatIDs(next), true
			}
		}
		if e.changed {
			todo = append(todo, e)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, e := range todo {
		if e.before == e.after {
			if err := journal.Delete(tx, profileID, []int64{e.id}); err != nil {
				return err
			}
			if err := journal.Audit(tx, profileID, EntityIssue, e.key, "discard", k.field, e.after, e.before, "the draft "+k.noun+" it named was discarded"); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE pending_change SET before_val = ?, after_val = ? WHERE profile_id = ? AND id = ?`,
			e.before, e.after, profileID, e.id); err != nil {
			return fmt.Errorf("repoint %s edit %d: %w", k.noun, e.id, err)
		}
	}
	return nil
}

// rewriteTargets repoints the changes that name another entry of the kind
// as where issues go: a release's carry-over and a delete's move target. A
// to of 0 clears the target and its name.
func (k catalogKind) rewriteTargets(ctx context.Context, tx *sql.Tx, profileID string, from, to int) error {
	args := []any{profileID}
	marks := ""
	for i, t := range k.targets {
		if i > 0 {
			marks += ", "
		}
		marks += "?"
		args = append(args, t)
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, after_val FROM pending_change WHERE profile_id = ? AND entity_type IN (`+marks+`)`, args...)
	if err != nil {
		return fmt.Errorf("%s changes naming %d: %w", k.noun, from, err)
	}
	updates := map[int64]string{}
	for rows.Next() {
		var id int64
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		var payload map[string]any
		if json.Unmarshal([]byte(raw), &payload) != nil {
			continue
		}
		if n, ok := payload["moveTo"].(float64); !ok || int(n) != from {
			continue
		}
		payload["moveTo"] = to
		if to == 0 {
			payload["moveToName"] = ""
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			rows.Close()
			return err
		}
		updates[id] = string(encoded)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for id, value := range updates {
		if _, err := tx.ExecContext(ctx, `UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND id = ?`, value, profileID, id); err != nil {
			return fmt.Errorf("repoint %s change %d: %w", k.noun, id, err)
		}
	}
	return nil
}
