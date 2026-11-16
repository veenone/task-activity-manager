package testrepo

import "fmt"

// The precondition sync writers. Each one leaves alone what the pending
// journal still owns: a deleted precondition stays gone, an edited field keeps
// its edit, and a Test whose association set was changed keeps that set.

// UpsertPreconditions inserts or updates a batch of Preconditions.
func (r *Repository) UpsertPreconditions(profileID string, preconditions []Precondition) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	deleted, err := pendingKeys(tx, profileID, entityPreconditionDelete)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT INTO precondition (profile_id, jira_key, summary, type, description, condition, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, jira_key) DO UPDATE SET
		   summary     = excluded.summary,
		   type        = excluded.type,
		   description = excluded.description,
		   condition   = excluded.condition,
		   status      = excluded.status`)
	if err != nil {
		return fmt.Errorf("prepare upsert precondition: %w", err)
	}
	defer stmt.Close()

	for _, p := range preconditions {
		if deleted[p.Key] {
			continue
		}
		if _, err := stmt.Exec(profileID, p.Key, p.Summary, p.Type, p.Description, p.Condition, p.Status); err != nil {
			return fmt.Errorf("upsert precondition %s: %w", p.Key, err)
		}
	}
	if err := reapplyFieldEdits(tx, profileID, "precondition", entityPreconditionEdit, preconditionFields); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkTestPreconditions upserts a batch of Test-to-Precondition links, stamping
// each with the current sync generation. Safe to call repeatedly during one
// pass: rows already present at this generation are left alone, and rows
// carried over from an earlier generation are re-stamped rather than
// duplicated. Nothing is deleted here — see SweepTestPreconditions.
//
// A Test with a pending association edit is skipped whole, and so is any link
// to a Precondition with a pending delete.
func (r *Repository) MarkTestPreconditions(profileID string, gen int64, links map[string][]string) error {
	if len(links) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	edited, err := pendingKeys(tx, profileID, entityPreconditionSet)
	if err != nil {
		return err
	}
	deleted, err := pendingKeys(tx, profileID, entityPreconditionDelete)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT INTO test_precondition (profile_id, test_key, precondition_key, sync_gen)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id, test_key, precondition_key) DO UPDATE SET
		   sync_gen = excluded.sync_gen`)
	if err != nil {
		return fmt.Errorf("prepare mark link: %w", err)
	}
	defer stmt.Close()

	for testKey, preKeys := range links {
		if edited[testKey] {
			continue
		}
		for _, pk := range preKeys {
			if deleted[pk] {
				continue
			}
			if _, err := stmt.Exec(profileID, testKey, pk, gen); err != nil {
				return fmt.Errorf("mark link %s -> %s: %w", testKey, pk, err)
			}
		}
	}
	return tx.Commit()
}

// SweepTestPreconditions removes a profile's Test-to-Precondition links left
// over from an earlier sync generation, and returns how many it deleted. This
// is what makes removed links actually disappear on resync.
//
// Call it ONLY after a precondition pass completed cleanly. Sweeping after a
// partial pass would delete links the pass simply never reached, which is the
// defect this whole mechanism exists to prevent (RND_P_4TFINT_05-336).
//
// Links of a Test with a pending association edit are kept: the mark step
// skipped that Test, so none of its links carry this generation.
func (r *Repository) SweepTestPreconditions(profileID string, gen int64) (int64, error) {
	res, err := r.db.Exec(
		`DELETE FROM test_precondition WHERE profile_id = ? AND sync_gen < ?
		   AND test_key NOT IN (
		     SELECT entity_key FROM pending_change WHERE profile_id = ? AND entity_type = ?)`,
		profileID, gen, profileID, entityPreconditionSet,
	)
	if err != nil {
		return 0, fmt.Errorf("sweep precondition links: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sweep precondition links rows: %w", err)
	}
	return n, nil
}
