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

// rewriteVersionID repoints every place a release id is carried from one id
// to another: the cached issues' fix versions, both halves of every pending
// fixVersions edit, and every draft issue's JSON. A to of 0 takes the id
// away, which is how a discarded draft release leaves the issues it was on;
// an edit that then changes nothing is dropped. It is also how a created
// release's draft id becomes its real one.
func rewriteVersionID(ctx context.Context, tx *sql.Tx, profileID string, from, to int) error {
	if err := rewriteIssueVersions(ctx, tx, profileID, from, to); err != nil {
		return err
	}
	if err := rewriteVersionEdits(ctx, tx, profileID, from, to); err != nil {
		return err
	}
	drafts, err := draftsWhere(ctx, tx, profileID, func(d backend.IssueDraft) bool {
		_, has := swapID(d.FixVersions, from, to)
		return has
	})
	if err != nil {
		return err
	}
	for _, key := range drafts {
		if err := editDraft(ctx, tx, profileID, key, func(d *backend.IssueDraft) {
			d.FixVersions, _ = swapID(d.FixVersions, from, to)
		}); err != nil {
			return err
		}
	}
	return rewriteVersionTargets(ctx, tx, profileID, from, to)
}

// rewriteIssueVersions rewrites the cached rows' fix_versions column.
func rewriteIssueVersions(ctx context.Context, tx *sql.Tx, profileID string, from, to int) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT key, fix_versions FROM issue WHERE profile_id = ? AND fix_versions LIKE ?`,
		profileID, "%"+strconv.Itoa(from)+"%")
	if err != nil {
		return fmt.Errorf("issues in release %d: %w", from, err)
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
		if _, err := tx.ExecContext(ctx, `UPDATE issue SET fix_versions = ? WHERE profile_id = ? AND key = ?`, value, profileID, key); err != nil {
			return fmt.Errorf("repoint %s from release %d: %w", key, from, err)
		}
	}
	return nil
}

// rewriteVersionEdits rewrites the pending fixVersions edits naming from.
// An edit left with the same value on both sides changes nothing any more
// and is dropped, audited as a discard.
func rewriteVersionEdits(ctx context.Context, tx *sql.Tx, profileID string, from, to int) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, entity_key, before_val, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND field = 'fixVersions'`,
		profileID, EntityIssue)
	if err != nil {
		return fmt.Errorf("fix version edits: %w", err)
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
			if err := journal.Audit(tx, profileID, EntityIssue, e.key, "discard", "fixVersions", e.after, e.before, "the draft release it named was discarded"); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE pending_change SET before_val = ?, after_val = ? WHERE profile_id = ? AND id = ?`,
			e.before, e.after, profileID, e.id); err != nil {
			return fmt.Errorf("repoint fix version edit %d: %w", e.id, err)
		}
	}
	return nil
}

// rewriteVersionTargets repoints the release changes that name another
// release as where issues go: a release's carry-over and a delete's move
// target. A to of 0 clears the target and its name.
func rewriteVersionTargets(ctx context.Context, tx *sql.Tx, profileID string, from, to int) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, after_val FROM pending_change WHERE profile_id = ? AND entity_type IN (?, ?)`,
		profileID, EntityVersionRelease, EntityVersionDelete)
	if err != nil {
		return fmt.Errorf("release changes naming %d: %w", from, err)
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
			return fmt.Errorf("repoint release change %d: %w", id, err)
		}
	}
	return nil
}
