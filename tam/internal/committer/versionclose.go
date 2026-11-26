package committer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// closeVersions is the closing phase: releases, then archives, then
// deletes, oldest first within a kind.
func (r *commitRun) closeVersions(ctx context.Context) {
	vb, canWrite := r.e.b.(backend.VersionBackend)
	for _, p := range r.versionRows(issuerepo.EntityVersionRelease) {
		r.prog.step(p.EntityKey, 1)
		r.releaseVersion(ctx, vb, canWrite, p)
	}
	for _, p := range r.versionRows(issuerepo.EntityVersionArchive) {
		r.prog.step(p.EntityKey, 1)
		r.archiveVersion(ctx, vb, canWrite, p)
	}
	for _, p := range r.versionRows(issuerepo.EntityVersionDelete) {
		r.prog.step(p.EntityKey, 1)
		r.deleteVersion(ctx, vb, canWrite, p)
	}
}

// leavingVersion lists the issues whose fixVersions edit taking them out of
// release id is still in the journal: it failed, conflicted or was held
// this Commit.
func (r *commitRun) leavingVersion(id int) []string {
	keys := map[string]bool{}
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityIssue || p.Field != "fixVersions" {
			continue
		}
		before, err1 := backend.ParseIDs(p.BeforeVal)
		after, err2 := backend.ParseIDs(p.AfterVal)
		if err1 == nil && err2 == nil && hasID(before, id) && !hasID(after, id) {
			keys[p.EntityKey] = true
		}
	}
	return sortedKeys(keys)
}

func hasID(ids []int, id int) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func (r *commitRun) holdVersion(p journal.PendingChange, name, waits, reason string) {
	r.res.Held = append(r.res.Held, Held{Key: name, EntityType: p.EntityType, RowID: p.ID, WaitsFor: waits, Reason: reason})
}

func (r *commitRun) releaseVersion(ctx context.Context, vb backend.VersionBackend, canWrite bool, p journal.PendingChange) {
	var v issuerepo.VersionRelease
	if err := json.Unmarshal([]byte(p.AfterVal), &v); err != nil {
		r.fail(p, "release "+p.EntityKey, "the release could not be decoded: "+err.Error(), false)
		return
	}
	id, _ := strconv.Atoi(p.EntityKey)
	if keys := r.leavingVersion(id); len(keys) > 0 {
		r.holdVersion(p, v.Name, strings.Join(keys, ", "),
			fmt.Sprintf("waits for the fix version changes on %s, which have not reached Jira; releasing first would leave them unfinished in a released version", strings.Join(keys, ", ")))
		return
	}
	if r.heldOnDraftVersion(v.Name, p.EntityType, p.ID, v.MoveTo) {
		return
	}
	id, remote, ok := r.remoteVersion(ctx, p, v.Name, canWrite)
	if !ok {
		return
	}
	if remote.Released {
		r.landVersion(ctx, p, v.Name, v.Name+" was already released in Jira")
		return
	}
	open, err := vb.UnresolvedInVersion(ctx, id)
	if err != nil {
		r.fail(p, v.Name, "the release's unfinished issues could not be read from Jira: "+err.Error(), true)
		return
	}
	if len(open) > 0 && v.MoveTo == 0 {
		r.holdVersion(p, v.Name, strings.Join(open, ", "),
			fmt.Sprintf("Jira still has unfinished issues in %s that the Release dialog did not cover (%s), and no release was picked to carry them over to; discard this release and release it again with a carry-over release, or finish those issues", v.Name, strings.Join(open, ", ")))
		return
	}
	if err := vb.ReleaseVersion(ctx, id, v.ReleaseDate, v.MoveTo); err != nil {
		r.fail(p, v.Name, err.Error(), true)
		return
	}
	did := v.Name + " released"
	if len(open) > 0 {
		did += fmt.Sprintf(", %d unfinished moved to %s", len(open), v.MoveToName)
	}
	r.landVersion(ctx, p, v.Name, did)
}

func (r *commitRun) archiveVersion(ctx context.Context, vb backend.VersionBackend, canWrite bool, p journal.PendingChange) {
	var v issuerepo.VersionArchive
	if err := json.Unmarshal([]byte(p.AfterVal), &v); err != nil {
		r.fail(p, "release "+p.EntityKey, "the change could not be decoded: "+err.Error(), false)
		return
	}
	id, _, ok := r.remoteVersion(ctx, p, v.Name, canWrite)
	if !ok {
		return
	}
	if err := vb.ArchiveVersion(ctx, id, v.Archived); err != nil {
		r.fail(p, v.Name, err.Error(), true)
		return
	}
	did := v.Name + " archived"
	if !v.Archived {
		did = v.Name + " unarchived"
	}
	r.landVersion(ctx, p, v.Name, did)
}

func (r *commitRun) deleteVersion(ctx context.Context, vb backend.VersionBackend, canWrite bool, p journal.PendingChange) {
	var v issuerepo.VersionDelete
	if err := json.Unmarshal([]byte(p.AfterVal), &v); err != nil {
		r.fail(p, "release "+p.EntityKey, "the delete could not be decoded: "+err.Error(), false)
		return
	}
	if r.heldOnDraftVersion(v.Name, p.EntityType, p.ID, v.MoveTo) {
		return
	}
	if !canWrite {
		r.fail(p, v.Name, noVersions, false)
		return
	}
	id, _ := strconv.Atoi(p.EntityKey)
	all, err := r.remoteVersions(ctx)
	if err != nil {
		r.fail(p, v.Name, err.Error(), true)
		return
	}
	if _, found := all[id]; !found {
		r.landVersion(ctx, p, v.Name, v.Name+" was already gone from Jira")
		return
	}
	if err := vb.DeleteVersion(ctx, id, v.MoveTo); err != nil {
		r.fail(p, v.Name, err.Error(), true)
		return
	}
	r.landVersion(ctx, p, v.Name, v.Name+" deleted")
}
