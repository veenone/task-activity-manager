package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// Deleting a component, and the cache work after Commit pushed a component
// change (#605). A delete changes nothing in the cache until Commit pushes
// it, as a release's delete does.

// ComponentDelete is what a component_delete row carries. MoveTo is where
// Jira moves the component's issues, 0 to leave them without it.
type ComponentDelete struct {
	Name       string `json:"name"`
	MoveTo     int    `json:"moveTo"`
	MoveToName string `json:"moveToName"`
}

// targetComponent checks a component named as where issues go: one of the
// same project and not the component itself. It answers with its name.
func targetComponent(ctx context.Context, q execer, profileID, project string, from, to int) (string, error) {
	if to == 0 {
		return "", nil
	}
	if to == from {
		return "", errors.New("issues cannot be moved to the component they are leaving")
	}
	toProject, f, _, err := cachedComponent(ctx, q, profileID, to)
	if err != nil {
		return "", err
	}
	if toProject != project {
		return "", fmt.Errorf("%s belongs to another project", f.Name)
	}
	return f.Name, nil
}

// QueueComponentDelete deletes a component. A draft is discarded at once,
// which takes it off every issue; a component Jira holds is queued for
// Commit, which moves its issues to moveTo when that is set. A pending edit
// is reverted and dropped, since the delete makes it moot.
func (r *Repository) QueueComponentDelete(ctx context.Context, profileID string, id, moveTo int) error {
	if id < 0 {
		return r.DiscardDraftComponent(ctx, profileID, id)
	}
	key := strconv.Itoa(id)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if err := refuseCatalogQueued(ctx, tx, profileID, key, "component", "deleted", componentChangeWord, EntityComponentDelete); err != nil {
			return err
		}
		project, f, draft, err := cachedComponent(ctx, tx, profileID, id)
		if err != nil {
			return err
		}
		if draft {
			return fmt.Errorf("%s is a draft; discard it from Pending changes", f.Name)
		}
		moveToName, err := targetComponent(ctx, tx, profileID, project, id, moveTo)
		if err != nil {
			return err
		}
		if edit, edited, err := pendingSprintRow(ctx, tx, profileID, EntityComponentEdit, key); err != nil {
			return err
		} else if edited {
			if err := discardOne(ctx, tx, profileID, edit); err != nil {
				return err
			}
			if _, f, _, err = cachedComponent(ctx, tx, profileID, id); err != nil {
				return err
			}
		}
		after, err := json.Marshal(ComponentDelete{Name: f.Name, MoveTo: moveTo, MoveToName: moveToName})
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntityComponentDelete, key, FieldDelete, f.Name, string(after), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityComponentDelete, key, "delete", FieldDelete, f.Name, "", "queued for Commit")
	})
}

// RekeyComponent turns a draft component Jira has just created into the
// real one: its negative id becomes made.ID on every issue, pending edit,
// draft issue and delete that named it, its row takes Jira's fields and
// stops being a draft, and its component_create row is cleared. A draft
// whose create row is gone answers ErrDraftComponentGone and changes
// nothing.
func (r *Repository) RekeyComponent(ctx context.Context, profileID string, draftID int, made backend.Component) error {
	key := strconv.Itoa(draftID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		p, ok, err := pendingSprintRow(ctx, tx, profileID, EntityComponentCreate, key)
		if err != nil {
			return err
		}
		if !ok {
			return ErrDraftComponentGone
		}
		if err := componentKind.rewriteID(ctx, tx, profileID, draftID, made.ID); err != nil {
			return err
		}
		// A sync that ran while Commit did may have cached Jira's copy
		// already; the draft row carries the same component and wins.
		if _, err := tx.ExecContext(ctx, `DELETE FROM project_component WHERE profile_id = ? AND id = ?`, profileID, made.ID); err != nil {
			return fmt.Errorf("clear component %d before the rekey: %w", made.ID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE project_component SET id = ?, name = ?, description = ?, lead_name = ?, lead_display_name = ?, assignee_type = ?, draft = 0
			 WHERE profile_id = ? AND id = ? AND draft = 1`,
			made.ID, made.Name, made.Description, made.LeadName, made.LeadDisplayName, made.AssigneeType, profileID, draftID); err != nil {
			return fmt.Errorf("rekey component %s to %d: %w", key, made.ID, err)
		}
		if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityComponentCreate, key, "commit", "", "", strconv.Itoa(made.ID), "created in Jira as component "+strconv.Itoa(made.ID))
	})
}

// ApplyComponentCommitted is the cache work after Commit pushed one
// component change, done in the transaction that clears its row: a delete
// moves the component's issues to its target, as Jira just did, and drops
// the row. An edit's change is already in the cache.
func (r *Repository) ApplyComponentCommitted(ctx context.Context, profileID string, p journal.PendingChange) error {
	id, err := strconv.Atoi(p.EntityKey)
	if err != nil {
		return fmt.Errorf("component id %q: %w", p.EntityKey, err)
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if p.EntityType == EntityComponentDelete {
			var d ComponentDelete
			if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
				return fmt.Errorf("decode the delete of %s: %w", p.EntityKey, err)
			}
			if err := componentKind.rewriteIssues(ctx, tx, profileID, id, d.MoveTo); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM project_component WHERE profile_id = ? AND id = ?`, profileID, id); err != nil {
				return fmt.Errorf("drop component %d: %w", id, err)
			}
		}
		if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "commit", p.Field, p.BeforeVal, p.AfterVal, "")
	})
}

// ComponentIssues lists the cached issues filed under the component, in
// rank order. Never nil.
func (r *Repository) ComponentIssues(ctx context.Context, profileID string, id int) ([]backend.Issue, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+issueColumns+` FROM issue
		WHERE profile_id = ? AND EXISTS (SELECT 1 FROM json_each(issue.components) WHERE json_each.value = ?)
		ORDER BY rank, key`, profileID, id)
	if err != nil {
		return nil, fmt.Errorf("issues under component %d: %w", id, err)
	}
	defer rows.Close()
	out := []backend.Issue{}
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, iss)
	}
	return out, rows.Err()
}

// ComponentCounts counts every component's cached issues, by component id.
// Like VersionProgress it reads the cache only, so the profile's scope JQL
// can leave issues out; the Components view says so.
func (r *Repository) ComponentCounts(ctx context.Context, profileID string) (map[int]int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT CAST(json_each.value AS INTEGER), COUNT(*) FROM issue, json_each(issue.components)
		 WHERE issue.profile_id = ? GROUP BY json_each.value`, profileID)
	if err != nil {
		return nil, fmt.Errorf("count component issues: %w", err)
	}
	defer rows.Close()
	out := map[int]int{}
	for rows.Next() {
		var id, n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
