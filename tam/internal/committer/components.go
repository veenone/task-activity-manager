package committer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Components are pushed like releases (versions.go, #605): creates and
// edits after the release ones in the releases and components phase, so
// the issue edits after it name real ids, and deletes at the end of the
// closing phase, after every issue write.

// componentKey is how a draft component is named to dependencies.
func componentKey(id int) string { return "component:" + strconv.Itoa(id) }

// noComponents is the failure every component change gets from a backend
// that cannot write components.
const noComponents = "this connection cannot manage components"

func (r *commitRun) heldOnDraftComponent(key, entityType string, rowID int64, ids ...int) bool {
	return r.heldOnDraft("component", componentKey, key, entityType, rowID, ids...)
}

// remoteComponents reads the project's components from Jira once per
// Commit, for the conflict checks.
func (r *commitRun) remoteComponents(ctx context.Context) (map[int]backend.Component, error) {
	if r.components != nil {
		return r.components, nil
	}
	cb, ok := r.e.b.(backend.CatalogBackend)
	if !ok {
		return nil, errors.New(noComponents)
	}
	list, err := cb.ProjectComponents(ctx, r.projectKey)
	if err != nil {
		return nil, fmt.Errorf("the project's components could not be read from Jira to check for changes: %w", err)
	}
	r.components = map[int]backend.Component{}
	for _, c := range list {
		r.components[c.ID] = c
	}
	return r.components, nil
}

func componentInput(f issuerepo.ComponentFields) backend.ComponentInput {
	return backend.ComponentInput{Name: f.Name, Description: f.Description, LeadName: f.LeadName, AssigneeType: f.AssigneeType}
}

// pushComponentChanges is the component half of the releases and
// components phase: creates, then edits.
func (r *commitRun) pushComponentChanges(ctx context.Context) {
	cb, canWrite := r.e.b.(backend.ComponentBackend)
	for _, p := range r.versionRows(issuerepo.EntityComponentCreate) {
		r.prog.step(p.EntityKey, 1)
		r.createComponent(ctx, cb, canWrite, p)
	}
	for _, p := range r.versionRows(issuerepo.EntityComponentEdit) {
		r.prog.step(p.EntityKey, 1)
		r.editComponent(ctx, cb, canWrite, p)
	}
}

func (r *commitRun) createComponent(ctx context.Context, cb backend.ComponentBackend, canWrite bool, p journal.PendingChange) {
	draftID, _ := strconv.Atoi(p.EntityKey)
	var d issuerepo.DraftComponent
	if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
		r.fail(p, "draft component "+p.EntityKey, "the draft component could not be decoded: "+err.Error(), false)
		r.deps.block(componentKey(draftID), "draft component "+p.EntityKey, "which could not be read")
		return
	}
	label := "component " + d.Name
	if !canWrite {
		r.fail(p, d.Name, noComponents, false)
		r.deps.block(componentKey(draftID), label, "which this connection cannot create")
		return
	}
	project := d.Project
	if project == "" {
		project = r.projectKey
	}
	made, err := cb.CreateComponent(ctx, project, componentInput(d.ComponentFields))
	if err != nil {
		r.fail(p, d.Name, err.Error(), true)
		r.deps.block(componentKey(draftID), label, "which Jira refused")
		return
	}
	if made.Name == "" {
		made.Name, made.Description, made.AssigneeType = d.Name, d.Description, d.AssigneeType
	}
	if made.LeadName == "" && made.LeadDisplayName == "" {
		made.LeadName, made.LeadDisplayName = d.LeadName, d.LeadDisplayName
	}
	if err := r.e.repo.RekeyComponent(ctx, r.profileID, draftID, made); err != nil {
		msg := fmt.Sprintf("created in Jira as component %d but the local rename failed: %v; sync to see it, then set it on its issues again", made.ID, err)
		if errors.Is(err, issuerepo.ErrDraftComponentGone) {
			msg = fmt.Sprintf("created in Jira as component %d, but the draft was discarded while Commit ran; sync to see it", made.ID)
		}
		r.fail(p, d.Name, msg, false)
		r.deps.block(componentKey(draftID), label, "which Jira created but TAM could not rename")
		return
	}
	r.res.ComponentsChanged = append(r.res.ComponentsChanged, d.Name+" created")
}

// componentDrift lists the fields where Jira's copy differs from what the
// edit started from, as versionDrift does for a release.
func componentDrift(was, mine issuerepo.ComponentFields, remote backend.Component) []string {
	var out []string
	for _, f := range []struct{ name, base, mine, remote string }{
		{"name", was.Name, mine.Name, remote.Name},
		{"description", was.Description, mine.Description, remote.Description},
		{"lead", was.LeadName, mine.LeadName, remote.LeadName},
		{"default assignee", was.AssigneeType, mine.AssigneeType, remote.AssigneeType},
	} {
		if f.base != f.remote && !(f.name == "default assignee" && f.base == "") {
			out = append(out, fmt.Sprintf("%s: %q when you edited it, %q yours, %q in Jira", f.name, f.base, f.mine, f.remote))
		}
	}
	return out
}

func (r *commitRun) editComponent(ctx context.Context, cb backend.ComponentBackend, canWrite bool, p journal.PendingChange) {
	var was, mine issuerepo.ComponentFields
	if err := json.Unmarshal([]byte(p.BeforeVal), &was); err != nil {
		r.fail(p, "component "+p.EntityKey, "the edit could not be decoded: "+err.Error(), false)
		return
	}
	if err := json.Unmarshal([]byte(p.AfterVal), &mine); err != nil {
		r.fail(p, was.Name, "the edit could not be decoded: "+err.Error(), false)
		return
	}
	id, remote, ok := r.remoteComponent(ctx, p, was.Name, canWrite)
	if !ok {
		return
	}
	if drift := componentDrift(was, mine, remote); len(drift) > 0 {
		r.fail(p, was.Name, "changed in Jira since TAM read it ("+strings.Join(drift, "; ")+"). Discard this edit and sync to take Jira's, then edit it again if you still want to", false)
		return
	}
	if _, err := cb.UpdateComponent(ctx, id, componentInput(mine)); err != nil {
		r.fail(p, was.Name, err.Error(), true)
		return
	}
	r.landComponent(ctx, p, was.Name, mine.Name+" edited")
}

// remoteComponent finds the row's component in Jira. One Jira no longer
// has fails the row.
func (r *commitRun) remoteComponent(ctx context.Context, p journal.PendingChange, name string, canWrite bool) (int, backend.Component, bool) {
	if !canWrite {
		r.fail(p, name, noComponents, false)
		return 0, backend.Component{}, false
	}
	id, err := strconv.Atoi(p.EntityKey)
	if err != nil || id <= 0 {
		r.fail(p, name, "the component id "+p.EntityKey+" is not one Jira holds", false)
		return 0, backend.Component{}, false
	}
	all, err := r.remoteComponents(ctx)
	if err != nil {
		r.fail(p, name, err.Error(), true)
		return 0, backend.Component{}, false
	}
	remote, found := all[id]
	if !found {
		r.fail(p, name, "Jira no longer has this component; discard the change and sync", false)
		return 0, backend.Component{}, false
	}
	return id, remote, true
}

// landComponent clears a pushed row and applies its change to the cache.
func (r *commitRun) landComponent(ctx context.Context, p journal.PendingChange, name, did string) {
	if err := r.e.repo.ApplyComponentCommitted(ctx, r.profileID, p); err != nil {
		r.fail(p, name, "pushed to Jira but the journal could not be cleared: "+err.Error(), true)
		return
	}
	r.res.ComponentsChanged = append(r.res.ComponentsChanged, did)
}

// deleteComponents is the component half of the closing phase.
func (r *commitRun) deleteComponents(ctx context.Context) {
	cb, canWrite := r.e.b.(backend.ComponentBackend)
	for _, p := range r.versionRows(issuerepo.EntityComponentDelete) {
		r.prog.step(p.EntityKey, 1)
		r.deleteComponent(ctx, cb, canWrite, p)
	}
}

func (r *commitRun) deleteComponent(ctx context.Context, cb backend.ComponentBackend, canWrite bool, p journal.PendingChange) {
	var d issuerepo.ComponentDelete
	if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
		r.fail(p, "component "+p.EntityKey, "the delete could not be decoded: "+err.Error(), false)
		return
	}
	if r.heldOnDraftComponent(d.Name, p.EntityType, p.ID, d.MoveTo) {
		return
	}
	if !canWrite {
		r.fail(p, d.Name, noComponents, false)
		return
	}
	id, _ := strconv.Atoi(p.EntityKey)
	all, err := r.remoteComponents(ctx)
	if err != nil {
		r.fail(p, d.Name, err.Error(), true)
		return
	}
	if _, found := all[id]; !found {
		r.landComponent(ctx, p, d.Name, d.Name+" was already gone from Jira")
		return
	}
	if err := cb.DeleteComponent(ctx, id, d.MoveTo); err != nil {
		r.fail(p, d.Name, err.Error(), true)
		return
	}
	r.landComponent(ctx, p, d.Name, d.Name+" deleted")
}
