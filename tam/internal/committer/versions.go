package committer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Releases (Jira fix versions) are pushed in two phases (#605). The
// releases phase runs after the sprint changes: drafts are created and
// rekeyed, so the edits after it name real ids, then edits of releases Jira
// holds go. The closing phase runs last: releases, archives, then deletes.
// A release goes after the issue edits so the carry-overs and descopes it
// queued have landed; while any of them is still in the journal the release
// is held.

// versionKey is how a draft release is named to dependencies, which also
// holds draft sprints under their bare negative ids.
func versionKey(id int) string { return "version:" + strconv.Itoa(id) }

// noVersions is the failure every release change gets from a backend that
// cannot write versions.
const noVersions = "this connection cannot manage releases"

// remoteVersions reads the project's versions from Jira once per Commit,
// for the conflict checks.
func (r *commitRun) remoteVersions(ctx context.Context) (map[int]backend.Version, error) {
	if r.versions != nil {
		return r.versions, nil
	}
	cb, ok := r.e.b.(backend.CatalogBackend)
	if !ok {
		return nil, errors.New(noVersions)
	}
	list, err := cb.ProjectVersions(ctx, r.projectKey)
	if err != nil {
		return nil, fmt.Errorf("the project's releases could not be read from Jira to check for changes: %w", err)
	}
	r.versions = map[int]backend.Version{}
	for _, v := range list {
		r.versions[v.ID] = v
	}
	return r.versions, nil
}

// versionRows are the rows of one release entity type, oldest first.
func (r *commitRun) versionRows(entityType string) []journal.PendingChange {
	var out []journal.PendingChange
	for _, p := range r.rows {
		if p.EntityType == entityType {
			out = append(out, p)
		}
	}
	return out
}

// pushVersionChanges is the releases phase: creates, then edits.
func (r *commitRun) pushVersionChanges(ctx context.Context) {
	vb, canWrite := r.e.b.(backend.VersionBackend)
	for _, p := range r.versionRows(issuerepo.EntityVersionCreate) {
		r.prog.step(p.EntityKey, 1)
		r.createVersion(ctx, vb, canWrite, p)
	}
	for _, p := range r.versionRows(issuerepo.EntityVersionEdit) {
		r.prog.step(p.EntityKey, 1)
		r.editVersion(ctx, vb, canWrite, p)
	}
}

func (r *commitRun) createVersion(ctx context.Context, vb backend.VersionBackend, canWrite bool, p journal.PendingChange) {
	draftID, _ := strconv.Atoi(p.EntityKey)
	var d issuerepo.DraftVersion
	if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
		r.fail(p, "draft release "+p.EntityKey, "the draft release could not be decoded: "+err.Error(), false)
		r.deps.block(versionKey(draftID), "draft release "+p.EntityKey, "which could not be read")
		return
	}
	label := "release " + d.Name
	if !canWrite {
		r.fail(p, d.Name, noVersions, false)
		r.deps.block(versionKey(draftID), label, "which this connection cannot create")
		return
	}
	project := d.Project
	if project == "" {
		project = r.projectKey
	}
	made, err := vb.CreateVersion(ctx, project, backend.VersionInput{Name: d.Name, Description: d.Description, StartDate: d.StartDate, ReleaseDate: d.ReleaseDate})
	if err != nil {
		r.fail(p, d.Name, err.Error(), true)
		r.deps.block(versionKey(draftID), label, "which Jira refused")
		return
	}
	if made.Name == "" {
		made.Name, made.Description, made.StartDate, made.ReleaseDate = d.Name, d.Description, d.StartDate, d.ReleaseDate
	}
	if err := r.e.repo.RekeyVersion(ctx, r.profileID, draftID, made); err != nil {
		msg := fmt.Sprintf("created in Jira as release %d but the local rename failed: %v; sync to see it, then set it on its issues again", made.ID, err)
		if errors.Is(err, issuerepo.ErrDraftVersionGone) {
			msg = fmt.Sprintf("created in Jira as release %d, but the draft was discarded while Commit ran; sync to see it", made.ID)
		}
		r.fail(p, d.Name, msg, false)
		r.deps.block(versionKey(draftID), label, "which Jira created but TAM could not rename")
		return
	}
	r.res.VersionsChanged = append(r.res.VersionsChanged, d.Name+" created")
}

// versionDrift lists the fields where Jira's copy differs from what the
// edit started from, as "name: X when edited, Y yours, Z in Jira".
func versionDrift(was, mine issuerepo.VersionFields, remote backend.Version) []string {
	var out []string
	for _, f := range []struct{ name, base, mine, remote string }{
		{"name", was.Name, mine.Name, remote.Name},
		{"description", was.Description, mine.Description, remote.Description},
		{"start date", was.StartDate, mine.StartDate, remote.StartDate},
		{"release date", was.ReleaseDate, mine.ReleaseDate, remote.ReleaseDate},
	} {
		if f.base != f.remote {
			out = append(out, fmt.Sprintf("%s: %q when you edited it, %q yours, %q in Jira", f.name, f.base, f.mine, f.remote))
		}
	}
	return out
}

func (r *commitRun) editVersion(ctx context.Context, vb backend.VersionBackend, canWrite bool, p journal.PendingChange) {
	var was, mine issuerepo.VersionFields
	if err := json.Unmarshal([]byte(p.BeforeVal), &was); err != nil {
		r.fail(p, "release "+p.EntityKey, "the edit could not be decoded: "+err.Error(), false)
		return
	}
	if err := json.Unmarshal([]byte(p.AfterVal), &mine); err != nil {
		r.fail(p, was.Name, "the edit could not be decoded: "+err.Error(), false)
		return
	}
	id, remote, ok := r.remoteVersion(ctx, p, was.Name, canWrite)
	if !ok {
		return
	}
	if drift := versionDrift(was, mine, remote); len(drift) > 0 {
		r.fail(p, was.Name, "changed in Jira since TAM read it ("+strings.Join(drift, "; ")+"). Discard this edit and sync to take Jira's, then edit it again if you still want to", false)
		return
	}
	if _, err := vb.UpdateVersion(ctx, id, backend.VersionInput{Name: mine.Name, Description: mine.Description, StartDate: mine.StartDate, ReleaseDate: mine.ReleaseDate}); err != nil {
		r.fail(p, was.Name, err.Error(), true)
		return
	}
	r.landVersion(ctx, p, was.Name, mine.Name+" edited")
}

// remoteVersion finds the row's release in Jira. A release Jira no longer
// has fails the row: nothing a retry does brings it back.
func (r *commitRun) remoteVersion(ctx context.Context, p journal.PendingChange, name string, canWrite bool) (int, backend.Version, bool) {
	if !canWrite {
		r.fail(p, name, noVersions, false)
		return 0, backend.Version{}, false
	}
	id, err := strconv.Atoi(p.EntityKey)
	if err != nil || id <= 0 {
		r.fail(p, name, "the release id "+p.EntityKey+" is not one Jira holds", false)
		return 0, backend.Version{}, false
	}
	all, err := r.remoteVersions(ctx)
	if err != nil {
		r.fail(p, name, err.Error(), true)
		return 0, backend.Version{}, false
	}
	remote, found := all[id]
	if !found {
		r.fail(p, name, "Jira no longer has this release; discard the change and sync", false)
		return 0, backend.Version{}, false
	}
	return id, remote, true
}

// landVersion clears a pushed row and applies its change to the cache.
func (r *commitRun) landVersion(ctx context.Context, p journal.PendingChange, name, did string) {
	if err := r.e.repo.ApplyVersionCommitted(ctx, r.profileID, p); err != nil {
		r.fail(p, name, "pushed to Jira but the journal could not be cleared: "+err.Error(), true)
		return
	}
	r.res.VersionsChanged = append(r.res.VersionsChanged, did)
}

// heldOnDraftVersion holds, or fails, a row naming a release by an id that
// is still a draft's: held when that draft's create failed this Commit,
// failed when nothing in the journal will create it.
func (r *commitRun) heldOnDraftVersion(key, entityType string, rowID int64, ids ...int) bool {
	return r.heldOnDraft("release", versionKey, key, entityType, rowID, ids...)
}

// heldOnDraft is heldOnDraftVersion for either catalog: noun names the
// kind in the failure, depKey is how its drafts are named to dependencies.
func (r *commitRun) heldOnDraft(noun string, depKey func(int) string, key, entityType string, rowID int64, ids ...int) bool {
	for _, id := range ids {
		if id >= 0 {
			continue
		}
		if waits, held := r.deps.blockedBy(depKey(id)); held {
			r.deps.hold(r.res, key, entityType, rowID, waits)
			return true
		}
		r.res.Failures = append(r.res.Failures, Failure{Key: key, EntityType: entityType, RowID: rowID, Error: fmt.Sprintf("it names %s %d, which is not a draft this Commit could create; set the %s again", noun, id, noun), Reachable: []string{}})
		return true
	}
	return false
}

// sortedKeys is a set's keys in order, for a sentence.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
