package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// A release (a Jira fix version) is managed the way a sprint is: a release
// made in TAM is a draft under a negative id, a project_version row with
// draft = 1 beside a version_create journal row, and an edit, release,
// archive or delete of a release Jira holds is a journal row Commit pushes
// (#605). Nothing reaches Jira before Commit.

// The journal entity types of a release. Each has one field, so a second
// edit replaces the first and keeps the first one's before value. The key
// is the release id as text.
const (
	// EntityVersionCreate's after_val is the DraftVersion as JSON.
	EntityVersionCreate = "version_create"
	// EntityVersionEdit's after_val is the edited VersionFields as JSON and
	// its before_val the cached ones.
	EntityVersionEdit = "version_edit"
	// EntityVersionRelease's after_val is a VersionRelease as JSON.
	EntityVersionRelease = "version_release"
	// EntityVersionArchive's after_val is a VersionArchive as JSON.
	EntityVersionArchive = "version_archive"
	// EntityVersionDelete's after_val is a VersionDelete as JSON.
	EntityVersionDelete = "version_delete"
)

// draftVersionSeq is the profile setting holding the last negative release
// id handed out, so an id is never reused (see draftSprintSeq).
const draftVersionSeq = "draft_version_seq"

// ErrDraftVersionGone is what a write aimed at a draft release answers when
// its version_create row is no longer in the journal.
var ErrDraftVersionGone = errors.New("issuerepo: the draft release is no longer in the journal")

// VersionFields are the fields a release's create and edit carry. Dates are
// Jira's own YYYY-MM-DD, or empty for none.
type VersionFields struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	StartDate   string `json:"startDate"`
	ReleaseDate string `json:"releaseDate"`
}

// DraftVersion is what a version_create row carries.
type DraftVersion struct {
	Project string `json:"project"`
	VersionFields
}

// cleanVersionFields trims the name and checks the dates: each empty or a
// day, and a start no later than the release.
func cleanVersionFields(f VersionFields) (VersionFields, error) {
	f.Name = strings.TrimSpace(f.Name)
	f.Description = strings.TrimSpace(f.Description)
	if f.Name == "" {
		return f, errors.New("a release needs a name")
	}
	for _, d := range []string{f.StartDate, f.ReleaseDate} {
		if d == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return f, fmt.Errorf("%q is not a date in the form 2026-12-31", d)
		}
	}
	if f.StartDate != "" && f.ReleaseDate != "" && f.ReleaseDate < f.StartDate {
		return f, errors.New("a release cannot end before it starts")
	}
	return f, nil
}

// uniqueVersionName refuses a name another release of the project already
// has, ignoring case, which Jira refuses too.
func uniqueVersionName(ctx context.Context, tx *sql.Tx, profileID, project, name string, except int) error {
	var one int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM project_version WHERE profile_id = ? AND project = ? AND id != ? AND name = ? COLLATE NOCASE`,
		profileID, project, except, name).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check the release name: %w", err)
	}
	return fmt.Errorf("this project already has a release called %s", name)
}

// CreateDraftVersion journals a new release of the project and writes its
// draft row, and answers with it as the pickers read it.
func (r *Repository) CreateDraftVersion(ctx context.Context, profileID, project string, f VersionFields) (backend.Version, error) {
	f, err := cleanVersionFields(f)
	if err != nil {
		return backend.Version{}, err
	}
	var made backend.Version
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		if err := uniqueVersionName(ctx, tx, profileID, project, f.Name, 0); err != nil {
			return err
		}
		id, err := nextDraftID(ctx, tx, profileID, "project_version", draftVersionSeq)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(DraftVersion{Project: project, VersionFields: f})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_version
			(profile_id, project, id, name, description, start_date, release_date, released, archived, draft)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, 1)`,
			profileID, project, id, f.Name, f.Description, f.StartDate, f.ReleaseDate); err != nil {
			return fmt.Errorf("insert draft release: %w", err)
		}
		key := strconv.Itoa(id)
		if err := journal.Put(tx, profileID, EntityVersionCreate, key, FieldCreate, "", string(encoded), ""); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, EntityVersionCreate, key, "create", "", "", f.Name, "drafted in "+project); err != nil {
			return err
		}
		made = backend.Version{ID: id, Project: project, Name: f.Name, Description: f.Description, StartDate: f.StartDate, ReleaseDate: f.ReleaseDate, Draft: true}
		return nil
	})
	return made, err
}

// cachedVersion reads a release's project and fields from the cache.
func cachedVersion(ctx context.Context, q execer, profileID string, id int) (string, VersionFields, error) {
	var project string
	var f VersionFields
	err := q.QueryRowContext(ctx,
		`SELECT project, name, description, start_date, release_date FROM project_version WHERE profile_id = ? AND id = ?`,
		profileID, id).Scan(&project, &f.Name, &f.Description, &f.StartDate, &f.ReleaseDate)
	if errors.Is(err, sql.ErrNoRows) {
		return "", f, fmt.Errorf("release %d is not in the cache; sync and try again", id)
	}
	if err != nil {
		return "", f, fmt.Errorf("read release %d: %w", id, err)
	}
	return project, f, nil
}

// writeVersionFields sets a release's cached fields.
func writeVersionFields(ctx context.Context, tx *sql.Tx, profileID string, id int, f VersionFields) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE project_version SET name = ?, description = ?, start_date = ?, release_date = ? WHERE profile_id = ? AND id = ?`,
		f.Name, f.Description, f.StartDate, f.ReleaseDate, profileID, id); err != nil {
		return fmt.Errorf("write release %d: %w", id, err)
	}
	return nil
}

// EditVersion changes a release's name, description and dates. A draft's
// create row is rewritten; a release Jira holds gets a version_edit row and
// its cached row changes at once. An edit back to what Jira holds drops the
// pending edit.
func (r *Repository) EditVersion(ctx context.Context, profileID string, id int, f VersionFields) error {
	f, err := cleanVersionFields(f)
	if err != nil {
		return err
	}
	key := strconv.Itoa(id)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		project, was, err := cachedVersion(ctx, tx, profileID, id)
		if err != nil {
			return err
		}
		if err := uniqueVersionName(ctx, tx, profileID, project, f.Name, id); err != nil {
			return err
		}
		if id < 0 {
			return editDraftVersion(ctx, tx, profileID, key, id, project, was, f)
		}
		if err := refuseVersionQueued(ctx, tx, profileID, key, "edited", EntityVersionDelete); err != nil {
			return err
		}
		prev, edited, err := pendingSprintRow(ctx, tx, profileID, EntityVersionEdit, key)
		if err != nil {
			return err
		}
		if edited {
			var orig VersionFields
			if json.Unmarshal([]byte(prev.BeforeVal), &orig) == nil && orig == f {
				return discardOne(ctx, tx, profileID, prev)
			}
		} else if was == f {
			return nil
		}
		before, err := json.Marshal(was)
		if err != nil {
			return err
		}
		after, err := json.Marshal(f)
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntityVersionEdit, key, FieldEdit, string(before), string(after), ""); err != nil {
			return err
		}
		if err := writeVersionFields(ctx, tx, profileID, id, f); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityVersionEdit, key, "edit", FieldEdit, was.Name, f.Name, "")
	})
}

func editDraftVersion(ctx context.Context, tx *sql.Tx, profileID, key string, id int, project string, was, f VersionFields) error {
	if _, ok, err := pendingSprintRow(ctx, tx, profileID, EntityVersionCreate, key); err != nil {
		return err
	} else if !ok {
		return ErrDraftVersionGone
	}
	encoded, err := json.Marshal(DraftVersion{Project: project, VersionFields: f})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		string(encoded), profileID, EntityVersionCreate, key); err != nil {
		return fmt.Errorf("rewrite draft release %s: %w", key, err)
	}
	if err := writeVersionFields(ctx, tx, profileID, id, f); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, EntityVersionCreate, key, "edit", "", was.Name, f.Name, "")
}

// refuseVersionQueued refuses a write while one of the given changes of the
// release waits for Commit.
func refuseVersionQueued(ctx context.Context, tx *sql.Tx, profileID, key, verb string, types ...string) error {
	for _, t := range types {
		if _, ok, err := pendingSprintRow(ctx, tx, profileID, t, key); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("this release has a %s waiting for Commit, so it cannot be %s; discard that change first", versionChangeWord[t], verb)
		}
	}
	return nil
}

var versionChangeWord = map[string]string{
	EntityVersionRelease: "release",
	EntityVersionArchive: "change to its archived state",
	EntityVersionDelete:  "delete",
	EntityVersionEdit:    "edit",
}

// DiscardDraftVersion is Delete on a draft release: exactly what discarding
// its version_create row from Pending changes does.
func (r *Repository) DiscardDraftVersion(ctx context.Context, profileID string, id int) error {
	key := strconv.Itoa(id)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		p, ok, err := pendingSprintRow(ctx, tx, profileID, EntityVersionCreate, key)
		if err != nil {
			return err
		}
		if !ok {
			return ErrDraftVersionGone
		}
		return discardOne(ctx, tx, profileID, p)
	})
}

// discardDraftVersion is what discarding a version_create row takes with
// it: the draft id leaves every issue and every pending edit naming it, and
// its row goes.
func discardDraftVersion(ctx context.Context, tx *sql.Tx, profileID, key string) error {
	id, err := strconv.Atoi(key)
	if err != nil {
		return fmt.Errorf("draft release id %q: %w", key, err)
	}
	if err := versionKind.rewriteID(ctx, tx, profileID, id, 0); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_version WHERE profile_id = ? AND id = ? AND draft = 1`, profileID, id); err != nil {
		return fmt.Errorf("drop draft release %s: %w", key, err)
	}
	return nil
}

// discardVersionEdit puts the cached release back as the edit found it.
func discardVersionEdit(ctx context.Context, tx *sql.Tx, profileID string, p journal.PendingChange) error {
	var was VersionFields
	if err := json.Unmarshal([]byte(p.BeforeVal), &was); err != nil {
		return fmt.Errorf("decode release %s before its edit: %w", p.EntityKey, err)
	}
	id, err := strconv.Atoi(p.EntityKey)
	if err != nil {
		return fmt.Errorf("release id %q: %w", p.EntityKey, err)
	}
	return writeVersionFields(ctx, tx, profileID, id, was)
}
