package issuerepo

import (
	"context"
	"strconv"

	"agile-suite/core/journal"
)

// EntitySprint is the audit trail's entity type for a sprint TAM created,
// edited or deleted in Jira. Unlike every other entity type in this package
// it is not a journal entity type: nothing about a sprint is ever queued in
// pending_change, because those three writes reach Jira the moment they are
// made.
const EntitySprint = "sprint"

// AuditSprint records one of those three writes. The entity key is the
// sprint's numeric id, which no issue key can collide with since every one
// of those carries its project's prefix; before and after are the sprint's
// name as it was and as it is, empty before a create and empty after a
// delete; and field names which of an edit's fields actually moved, empty
// for a create or a delete, where there is only ever the one name to report.
//
// It lives here rather than in boardrepo, which owns the sprint rows,
// because the audit_log table is this repository's: every other row in it is
// written from this package, and one writer is what keeps the actor stamp
// and the column list in one place.
//
// Nothing on screen reads a row of this shape. ListActivity takes an issue
// key and the Activity tab passes the selected issue's, so a sprint's rows
// are reachable only by reading the database with another tool. They are
// written because once Jira no longer has the sprint this is the only trace
// of it left on the machine, and nothing in the app tells a user to go and
// read them.
//
// The context is taken for the shape every other call here has and is not
// used, the same way ListPendingChanges takes one journal.List does not
// need.
func (r *Repository) AuditSprint(_ context.Context, profileID string, sprintID int, action, field, before, after string) error {
	return journal.Audit(r.db, profileID, EntitySprint, strconv.Itoa(sprintID), action, field, before, after, "")
}
