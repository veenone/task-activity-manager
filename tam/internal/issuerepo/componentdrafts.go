package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// A component is managed the way a release is (#605): one made in TAM is a
// draft under a negative id, a project_component row with draft = 1 beside a
// component_create journal row, and an edit or delete of a component Jira
// holds is a journal row Commit pushes. Nothing reaches Jira before Commit.

// The journal entity types of a component. Each has one field, so a second
// edit replaces the first and keeps the first one's before value. The key is
// the component id as text.
const (
	// EntityComponentCreate's after_val is the DraftComponent as JSON.
	EntityComponentCreate = "component_create"
	// EntityComponentEdit's after_val is the edited ComponentFields as JSON
	// and its before_val the cached ones.
	EntityComponentEdit = "component_edit"
	// EntityComponentDelete's after_val is a ComponentDelete as JSON.
	EntityComponentDelete = "component_delete"
)

// draftComponentSeq is the profile setting holding the last negative
// component id handed out, so an id is never reused (see draftSprintSeq).
const draftComponentSeq = "draft_component_seq"

// ErrDraftComponentGone is what a write aimed at a draft component answers
// when its component_create row is no longer in the journal.
var ErrDraftComponentGone = errors.New("issuerepo: the draft component is no longer in the journal")

// AssigneeTypes are the default assignee values Jira takes for a component.
var AssigneeTypes = []string{"PROJECT_DEFAULT", "COMPONENT_LEAD", "PROJECT_LEAD", "UNASSIGNED"}

// ComponentFields are the fields a component's create and edit carry. The
// lead is a username with the display name the picker showed beside it.
type ComponentFields struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	LeadName        string `json:"leadName"`
	LeadDisplayName string `json:"leadDisplayName"`
	AssigneeType    string `json:"assigneeType"`
}

// DraftComponent is what a component_create row carries.
type DraftComponent struct {
	Project string `json:"project"`
	ComponentFields
}

// cleanComponentFields trims the text, defaults the assignee type to the
// project's, and refuses a blank name or an assignee type Jira does not take.
func cleanComponentFields(f ComponentFields) (ComponentFields, error) {
	f.Name = strings.TrimSpace(f.Name)
	f.Description = strings.TrimSpace(f.Description)
	f.LeadName = strings.TrimSpace(f.LeadName)
	if f.LeadName == "" {
		f.LeadDisplayName = ""
	}
	if f.AssigneeType == "" {
		f.AssigneeType = AssigneeTypes[0]
	}
	if f.Name == "" {
		return f, errors.New("a component needs a name")
	}
	if !slices.Contains(AssigneeTypes, f.AssigneeType) {
		return f, fmt.Errorf("%q is not a default assignee Jira takes", f.AssigneeType)
	}
	return f, nil
}

// uniqueComponentName refuses a name another component of the project
// already has, ignoring case, which Jira refuses too.
func uniqueComponentName(ctx context.Context, tx *sql.Tx, profileID, project, name string, except int) error {
	var one int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM project_component WHERE profile_id = ? AND project = ? AND id != ? AND name = ? COLLATE NOCASE`,
		profileID, project, except, name).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check the component name: %w", err)
	}
	return fmt.Errorf("a component named %q already exists", name)
}

func componentOf(id int, project string, f ComponentFields, draft bool) backend.Component {
	return backend.Component{ID: id, Project: project, Name: f.Name, Description: f.Description,
		LeadName: f.LeadName, LeadDisplayName: f.LeadDisplayName, AssigneeType: f.AssigneeType, Draft: draft}
}

// CreateDraftComponent journals a new component of the project and writes
// its draft row, and answers with it as the pickers read it.
func (r *Repository) CreateDraftComponent(ctx context.Context, profileID, project string, f ComponentFields) (backend.Component, error) {
	f, err := cleanComponentFields(f)
	if err != nil {
		return backend.Component{}, err
	}
	var made backend.Component
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		if err := uniqueComponentName(ctx, tx, profileID, project, f.Name, 0); err != nil {
			return err
		}
		id, err := nextDraftID(ctx, tx, profileID, "project_component", draftComponentSeq)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(DraftComponent{Project: project, ComponentFields: f})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_component
			(profile_id, project, id, name, description, lead_name, lead_display_name, assignee_type, draft)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			profileID, project, id, f.Name, f.Description, f.LeadName, f.LeadDisplayName, f.AssigneeType); err != nil {
			return fmt.Errorf("insert draft component: %w", err)
		}
		key := strconv.Itoa(id)
		if err := journal.Put(tx, profileID, EntityComponentCreate, key, FieldCreate, "", string(encoded), ""); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, EntityComponentCreate, key, "create", "", "", f.Name, "drafted in "+project); err != nil {
			return err
		}
		made = componentOf(id, project, f, true)
		return nil
	})
	return made, err
}

// cachedComponent reads a component's project and fields from the cache.
func cachedComponent(ctx context.Context, q execer, profileID string, id int) (string, ComponentFields, bool, error) {
	var project string
	var f ComponentFields
	var draft bool
	err := q.QueryRowContext(ctx,
		`SELECT project, name, description, lead_name, lead_display_name, assignee_type, draft FROM project_component WHERE profile_id = ? AND id = ?`,
		profileID, id).Scan(&project, &f.Name, &f.Description, &f.LeadName, &f.LeadDisplayName, &f.AssigneeType, &draft)
	if errors.Is(err, sql.ErrNoRows) {
		return "", f, false, fmt.Errorf("component %d is not in the cache; sync and try again", id)
	}
	if err != nil {
		return "", f, false, fmt.Errorf("read component %d: %w", id, err)
	}
	return project, f, draft, nil
}

// writeComponentFields sets a component's cached fields.
func writeComponentFields(ctx context.Context, tx *sql.Tx, profileID string, id int, f ComponentFields) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE project_component SET name = ?, description = ?, lead_name = ?, lead_display_name = ?, assignee_type = ? WHERE profile_id = ? AND id = ?`,
		f.Name, f.Description, f.LeadName, f.LeadDisplayName, f.AssigneeType, profileID, id); err != nil {
		return fmt.Errorf("write component %d: %w", id, err)
	}
	return nil
}

// sameComponent compares what Jira holds of a component: the display name
// of the lead follows the username, so it is left out, and an empty
// assignee type is the project default it means.
func sameComponent(a, b ComponentFields) bool {
	assignee := func(t string) string {
		if t == "" {
			return AssigneeTypes[0]
		}
		return t
	}
	return a.Name == b.Name && a.Description == b.Description && a.LeadName == b.LeadName && assignee(a.AssigneeType) == assignee(b.AssigneeType)
}

// EditComponent changes a component's fields. A draft's create row is
// rewritten; a component Jira holds gets a component_edit row and its cached
// row changes at once. An edit back to what Jira holds drops the pending
// edit.
func (r *Repository) EditComponent(ctx context.Context, profileID string, id int, f ComponentFields) error {
	f, err := cleanComponentFields(f)
	if err != nil {
		return err
	}
	key := strconv.Itoa(id)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		project, was, _, err := cachedComponent(ctx, tx, profileID, id)
		if err != nil {
			return err
		}
		if err := uniqueComponentName(ctx, tx, profileID, project, f.Name, id); err != nil {
			return err
		}
		if id < 0 {
			return editDraftComponent(ctx, tx, profileID, key, id, project, was, f)
		}
		if err := refuseCatalogQueued(ctx, tx, profileID, key, "component", "edited", componentChangeWord, EntityComponentDelete); err != nil {
			return err
		}
		prev, edited, err := pendingSprintRow(ctx, tx, profileID, EntityComponentEdit, key)
		if err != nil {
			return err
		}
		if edited {
			var orig ComponentFields
			if json.Unmarshal([]byte(prev.BeforeVal), &orig) == nil && sameComponent(orig, f) {
				return discardOne(ctx, tx, profileID, prev)
			}
		} else if sameComponent(was, f) {
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
		if err := journal.Put(tx, profileID, EntityComponentEdit, key, FieldEdit, string(before), string(after), ""); err != nil {
			return err
		}
		if err := writeComponentFields(ctx, tx, profileID, id, f); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityComponentEdit, key, "edit", FieldEdit, was.Name, f.Name, "")
	})
}

func editDraftComponent(ctx context.Context, tx *sql.Tx, profileID, key string, id int, project string, was, f ComponentFields) error {
	if _, ok, err := pendingSprintRow(ctx, tx, profileID, EntityComponentCreate, key); err != nil {
		return err
	} else if !ok {
		return ErrDraftComponentGone
	}
	encoded, err := json.Marshal(DraftComponent{Project: project, ComponentFields: f})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		string(encoded), profileID, EntityComponentCreate, key); err != nil {
		return fmt.Errorf("rewrite draft component %s: %w", key, err)
	}
	if err := writeComponentFields(ctx, tx, profileID, id, f); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, EntityComponentCreate, key, "edit", "", was.Name, f.Name, "")
}

var componentChangeWord = map[string]string{
	EntityComponentDelete: "delete",
	EntityComponentEdit:   "edit",
}

// DiscardDraftComponent is Delete on a draft component: exactly what
// discarding its component_create row from Pending changes does.
func (r *Repository) DiscardDraftComponent(ctx context.Context, profileID string, id int) error {
	key := strconv.Itoa(id)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		p, ok, err := pendingSprintRow(ctx, tx, profileID, EntityComponentCreate, key)
		if err != nil {
			return err
		}
		if !ok {
			return ErrDraftComponentGone
		}
		return discardOne(ctx, tx, profileID, p)
	})
}

// discardDraftComponent is what discarding a component_create row takes
// with it: the draft id leaves every issue and every pending edit naming
// it, and its row goes.
func discardDraftComponent(ctx context.Context, tx *sql.Tx, profileID, key string) error {
	id, err := strconv.Atoi(key)
	if err != nil {
		return fmt.Errorf("draft component id %q: %w", key, err)
	}
	if err := componentKind.rewriteID(ctx, tx, profileID, id, 0); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_component WHERE profile_id = ? AND id = ? AND draft = 1`, profileID, id); err != nil {
		return fmt.Errorf("drop draft component %s: %w", key, err)
	}
	return nil
}

// discardComponentEdit puts the cached component back as the edit found it.
func discardComponentEdit(ctx context.Context, tx *sql.Tx, profileID string, p journal.PendingChange) error {
	var was ComponentFields
	if err := json.Unmarshal([]byte(p.BeforeVal), &was); err != nil {
		return fmt.Errorf("decode component %s before its edit: %w", p.EntityKey, err)
	}
	id, err := strconv.Atoi(p.EntityKey)
	if err != nil {
		return fmt.Errorf("component id %q: %w", p.EntityKey, err)
	}
	return writeComponentFields(ctx, tx, profileID, id, was)
}
