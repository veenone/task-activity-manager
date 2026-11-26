package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// Releasing, archiving and deleting a release Jira holds. Each is a journal
// row that changes nothing in the cache until Commit pushes it, the way a
// sprint's start and completion work, so the Releases view marks the row
// "Releasing on Commit" and the like until then.

// FieldRelease and FieldArchive are the one field of those two rows.
const (
	FieldRelease = "release"
	FieldArchive = "archive"
)

// ReleaseMove is the Release dialog's choice for one unfinished issue: carry
// it over to release To, or descope it when To is 0. Either way the closing
// release leaves the issue's fix versions and nothing else changes.
type ReleaseMove struct {
	Key string `json:"key"`
	To  int    `json:"to"`
}

// VersionRelease is what a version_release row carries. MoveTo is the
// set-all carry-over release, 0 for none: at Commit, unfinished issues the
// dialog never saw (outside the profile's scope) go there.
type VersionRelease struct {
	Name        string `json:"name"`
	ReleaseDate string `json:"releaseDate"`
	MoveTo      int    `json:"moveTo"`
	MoveToName  string `json:"moveToName"`
}

// VersionArchive is what a version_archive row carries: archive, or
// unarchive when Archived is false.
type VersionArchive struct {
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

// VersionDelete is what a version_delete row carries. MoveTo is where Jira
// moves the release's issues, 0 to leave them without it.
type VersionDelete struct {
	Name       string `json:"name"`
	MoveTo     int    `json:"moveTo"`
	MoveToName string `json:"moveToName"`
}

// versionRow is a release's cached state.
type versionRow struct {
	project, name             string
	released, archived, draft bool
}

func readVersionRow(ctx context.Context, q execer, profileID string, id int) (versionRow, error) {
	var v versionRow
	err := q.QueryRowContext(ctx,
		`SELECT project, name, released, archived, draft FROM project_version WHERE profile_id = ? AND id = ?`,
		profileID, id).Scan(&v.project, &v.name, &v.released, &v.archived, &v.draft)
	if errors.Is(err, sql.ErrNoRows) {
		return v, fmt.Errorf("release %d is not in the cache; sync and try again", id)
	}
	if err != nil {
		return v, fmt.Errorf("read release %d: %w", id, err)
	}
	return v, nil
}

// realVersion reads a release Jira holds, refusing a draft with the reason.
func realVersion(ctx context.Context, q execer, profileID string, id int, verb string) (versionRow, error) {
	v, err := readVersionRow(ctx, q, profileID, id)
	if err != nil {
		return v, err
	}
	if v.draft || id < 0 {
		return v, fmt.Errorf("%s is a draft; commit it first, then it can be %s", v.name, verb)
	}
	return v, nil
}

// targetVersion checks a release named as where issues go: one of the same
// project, not the release itself, and for a carry-over neither released
// nor archived. It answers with the target's name.
func targetVersion(ctx context.Context, q execer, profileID, project string, from, to int, open bool) (string, error) {
	if to == 0 {
		return "", nil
	}
	if to == from {
		return "", errors.New("issues cannot be moved to the release they are leaving")
	}
	v, err := readVersionRow(ctx, q, profileID, to)
	if err != nil {
		return "", err
	}
	if v.project != project {
		return "", fmt.Errorf("%s belongs to another project", v.name)
	}
	if v.archived || (open && v.released) {
		return "", fmt.Errorf("%s is released or archived, so issues cannot be moved to it", v.name)
	}
	return v.name, nil
}

// QueueVersionRelease queues the release of a release Jira holds, with the
// Release dialog's choice for each unfinished issue journaled as an ordinary
// fixVersions edit in the same transaction. Commit sends the edits before
// the release, and holds the release while any of them has not landed.
func (r *Repository) QueueVersionRelease(ctx context.Context, profileID string, id int, date string, moveTo int, moves []ReleaseMove) error {
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return fmt.Errorf("%q is not a date in the form 2026-12-31", date)
		}
	}
	key := strconv.Itoa(id)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		v, err := realVersion(ctx, tx, profileID, id, "released")
		if err != nil {
			return err
		}
		if v.released || v.archived {
			return fmt.Errorf("%s is already released or archived", v.name)
		}
		if err := refuseVersionQueued(ctx, tx, profileID, key, "released", EntityVersionRelease, EntityVersionDelete); err != nil {
			return err
		}
		moveToName, err := targetVersion(ctx, tx, profileID, v.project, id, moveTo, true)
		if err != nil {
			return err
		}
		for _, m := range moves {
			if err := moveOutOfVersion(ctx, tx, profileID, id, v.name, m); err != nil {
				return err
			}
		}
		after, err := json.Marshal(VersionRelease{Name: v.name, ReleaseDate: date, MoveTo: moveTo, MoveToName: moveToName})
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntityVersionRelease, key, FieldRelease, "", string(after), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityVersionRelease, key, "release", FieldRelease, "", date, fmt.Sprintf("queued for Commit with %d issue choices", len(moves)))
	})
}

// moveOutOfVersion journals one issue leaving the release being closed:
// swapped for the carry-over release, or dropped when descoped.
func moveOutOfVersion(ctx context.Context, tx *sql.Tx, profileID string, id int, name string, m ReleaseMove) error {
	current, updated, _, err := readField(ctx, tx, profileID, m.Key, "fixVersions")
	if err != nil {
		return fmt.Errorf("%s: %w", m.Key, err)
	}
	ids, err := backend.ParseIDs(current)
	if err != nil {
		return fmt.Errorf("%s: %w", m.Key, err)
	}
	next, had := swapID(ids, id, m.To)
	if !had {
		return fmt.Errorf("%s is not in %s; refresh the Releases view", m.Key, name)
	}
	value := backend.FormatIDs(next)
	if err := checkCatalogIDs(ctx, tx, profileID, "fixVersions", current, value); err != nil {
		return fmt.Errorf("%s: %w", m.Key, err)
	}
	return journalFieldEdit(ctx, tx, profileID, m.Key, "fixVersions", current, value, updated, "releasing "+name)
}

// QueueVersionArchive queues archiving a release Jira holds, or unarchiving
// it when archived is false.
func (r *Repository) QueueVersionArchive(ctx context.Context, profileID string, id int, archived bool) error {
	key := strconv.Itoa(id)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		verb := "archived"
		if !archived {
			verb = "unarchived"
		}
		v, err := realVersion(ctx, tx, profileID, id, verb)
		if err != nil {
			return err
		}
		if v.archived == archived {
			return fmt.Errorf("%s is already %s", v.name, verb)
		}
		if err := refuseVersionQueued(ctx, tx, profileID, key, verb, EntityVersionArchive, EntityVersionDelete); err != nil {
			return err
		}
		after, err := json.Marshal(VersionArchive{Name: v.name, Archived: archived})
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntityVersionArchive, key, FieldArchive, "", string(after), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityVersionArchive, key, FieldArchive, FieldArchive, "", verb, "queued for Commit")
	})
}

// QueueVersionDelete deletes a release. A draft is discarded at once, which
// takes it off every issue; a release Jira holds is queued for Commit, which
// moves its issues to moveTo when that is set. A pending edit is reverted
// and dropped, since the delete makes it moot.
func (r *Repository) QueueVersionDelete(ctx context.Context, profileID string, id, moveTo int) error {
	if id < 0 {
		return r.DiscardDraftVersion(ctx, profileID, id)
	}
	key := strconv.Itoa(id)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if edit, edited, err := pendingSprintRow(ctx, tx, profileID, EntityVersionEdit, key); err != nil {
			return err
		} else if edited {
			if err := discardOne(ctx, tx, profileID, edit); err != nil {
				return err
			}
		}
		if err := refuseVersionQueued(ctx, tx, profileID, key, "deleted", EntityVersionRelease, EntityVersionArchive, EntityVersionDelete); err != nil {
			return err
		}
		v, err := realVersion(ctx, tx, profileID, id, "deleted")
		if err != nil {
			return err
		}
		moveToName, err := targetVersion(ctx, tx, profileID, v.project, id, moveTo, false)
		if err != nil {
			return err
		}
		after, err := json.Marshal(VersionDelete{Name: v.name, MoveTo: moveTo, MoveToName: moveToName})
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntityVersionDelete, key, FieldDelete, v.name, string(after), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityVersionDelete, key, "delete", FieldDelete, v.name, "", "queued for Commit")
	})
}
