package committer

import (
	"context"
	"encoding/json"
	"os"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Attached is a file Commit uploaded.
type Attached struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// pushAttachments uploads every journalled file, one row at a time with its
// own journal delete, the way pushWorklogs sends entries: a refusal on one
// file takes that row alone and the rest still go.
//
// A file under a key this Commit could not create is held, and one still
// under a draft key is left for next time. A draft that was created needs
// nothing rewritten here: the rekey renames every journal row of the old
// key, so by this phase the row names the key Jira gave.
//
// The journal holds where the file is, not the file. One that is gone by
// now is reported as that and not as retryable: sending it again will find
// the same empty path, and the row is there to be discarded.
func (r *commitRun) pushAttachments(ctx context.Context) {
	a, canAttach := r.e.b.(backend.AttachmentBackend)
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityAttachment {
			continue
		}
		if waits, held := r.deps.blockedBy(p.EntityKey); held {
			r.deps.hold(r.res, p.EntityKey, issuerepo.EntityAttachment, p.ID, waits)
			continue
		}
		if isDraftKey(p.EntityKey) {
			continue
		}
		var d backend.AttachmentDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			r.res.Failures = append(r.res.Failures, attachmentFailure(p, "the attachment could not be decoded: "+err.Error(), false))
			continue
		}
		if !canAttach {
			r.res.Failures = append(r.res.Failures, attachmentFailure(p, "this connection cannot add attachments", false))
			continue
		}
		if info, err := os.Stat(d.Path); err != nil || !info.Mode().IsRegular() {
			r.res.Failures = append(r.res.Failures, attachmentFailure(p, "the file is no longer at "+d.Path+". Discard this change, or put the file back and commit again", false))
			continue
		}
		if err := a.AddAttachment(ctx, p.EntityKey, d); err != nil {
			r.res.Failures = append(r.res.Failures, attachmentFailure(p, err.Error(), true))
			continue
		}
		if err := r.e.repo.MarkCommitted(ctx, r.profileID, []journal.PendingChange{p}); err != nil {
			r.res.Failures = append(r.res.Failures, attachmentFailure(p, "uploaded to Jira but the journal could not be cleared: "+err.Error(), true))
			continue
		}
		r.res.Attached = append(r.res.Attached, Attached{Key: p.EntityKey, Name: d.Name})
	}
}

// attachmentFailure is one file that did not land, carrying its row id so
// the dialog can offer to discard exactly that file and no other.
func attachmentFailure(p journal.PendingChange, message string, retryable bool) Failure {
	return Failure{Key: p.EntityKey, EntityType: issuerepo.EntityAttachment, RowID: p.ID, Error: message, Retryable: retryable, Reachable: []string{}}
}
