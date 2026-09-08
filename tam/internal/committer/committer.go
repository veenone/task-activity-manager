// Package committer pushes TAM's journal to Jira: drafts are created and
// rekeyed first, then each edited issue is version-checked, pushed, and
// refreshed, then the board moves in boards.go, then the links. An issue
// whose remote version moved is held back as a conflict carrying base,
// mine, and remote for every pending field; the two resolutions rebase the
// edits or drop them.
package committer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Created pairs a draft's temporary key with the key Jira assigned.
type Created struct {
	TempKey string `json:"tempKey"`
	Key     string `json:"key"`
}

// FieldConflict is one pending field of a held issue: the value when the
// edit was made, the edit, and what Jira holds now.
type FieldConflict struct {
	Field  string `json:"field"`
	Base   string `json:"base"`
	Mine   string `json:"mine"`
	Remote string `json:"remote"`
}

// Conflict is an issue Commit held back. RemoteVersion is the updated stamp
// an override rebases onto.
type Conflict struct {
	Key           string          `json:"key"`
	Summary       string          `json:"summary"`
	RemoteVersion string          `json:"remoteVersion"`
	Fields        []FieldConflict `json:"fields"`
}

// Failure is a push that did not land; its journal rows stay. EntityType
// and RowID name the row it was, because one card can fail a transition and
// drop a rank in the same Commit and a per-failure Undo has to know which
// row to discard; RowID is 0 for a failure that covers every pending row of
// an issue, which is what an edits push is. Retryable is false for the
// failures that will fail identically forever, so "Commit again to retry"
// is only offered where it can help, and Reachable carries the statuses a
// refused transition could have reached instead.
type Failure struct {
	Key        string   `json:"key"`
	EntityType string   `json:"entityType"`
	RowID      int64    `json:"rowId"`
	Error      string   `json:"error"`
	Retryable  bool     `json:"retryable"`
	Reachable  []string `json:"reachable"`
}

// failure is a push failure on no single row: a whole issue's edits, a
// whole pass. Every caller says whether a second Commit could do better,
// because a row nobody can decode will not decode next time either and
// offering "Commit again to retry" for it is a promise the pass cannot
// keep. entityType is empty for a failure that covers a whole pass rather
// than one kind of row.
func failure(key, entityType, message string, retryable bool) Failure {
	return Failure{Key: key, EntityType: entityType, Error: message, Retryable: retryable, Reachable: []string{}}
}

// Linked is a link Commit created.
type Linked struct {
	Key   string `json:"key"`
	ToKey string `json:"toKey"`
	Type  string `json:"type"`
}

// Result is what one Commit did. Remaining counts the journal rows left.
type Result struct {
	Committed []string   `json:"committed"`
	Created   []Created  `json:"created"`
	Linked    []Linked   `json:"linked"`
	Moved     []Moved    `json:"moved"`
	Conflicts []Conflict `json:"conflicts"`
	Failures  []Failure  `json:"failures"`
	Remaining int        `json:"remaining"`
}

// Engine runs commits for one backend and repository pair. order is the
// board order the rank group re-derives its neighbours from; a nil one
// drops the ranks with that reason rather than pushing them against a
// neighbour nobody checked.
type Engine struct {
	b     backend.IssueBackend
	repo  *issuerepo.Repository
	order BoardOrder
}

// New returns an engine over the backend, the store, and the board order.
func New(b backend.IssueBackend, repo *issuerepo.Repository, order BoardOrder) *Engine {
	return &Engine{b: b, repo: repo, order: order}
}

// Commit pushes every pending change of the profile. Only a store failure
// returns an error; per-issue outcomes land in the Result.
func (e *Engine) Commit(ctx context.Context, profileID, projectKey string) (Result, error) {
	res := Result{Committed: []string{}, Created: []Created{}, Linked: []Linked{}, Moved: []Moved{}, Conflicts: []Conflict{}, Failures: []Failure{}}
	all, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return res, err
	}
	byKey := map[string][]journal.PendingChange{}
	var creates, edits []string
	for _, p := range all {
		if p.EntityType == issuerepo.EntityLink || boardRow(p.EntityType) {
			continue
		}
		if _, seen := byKey[p.EntityKey]; !seen {
			if p.EntityType == issuerepo.EntityIssueCreate {
				creates = append(creates, p.EntityKey)
			} else {
				edits = append(edits, p.EntityKey)
			}
		}
		byKey[p.EntityKey] = append(byKey[p.EntityKey], p)
	}
	sort.Strings(creates)
	sort.Strings(edits)
	// journal.List is newest first; apply each issue's rows oldest first.
	for k := range byKey {
		rows := byKey[k]
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	}

	for _, tempKey := range creates {
		e.commitCreate(ctx, profileID, projectKey, tempKey, byKey[tempKey], &res)
	}
	if len(creates) > 0 && len(edits) > 0 {
		// A create rekeys its draft, and Rekey repoints rows that named the
		// temporary key, so the edits pass reads the journal again rather
		// than the snapshot taken before the creates ran.
		byKey, edits = e.regroupEdits(ctx, profileID, byKey, edits, &res)
	}
	for _, key := range edits {
		e.commitEdit(ctx, profileID, key, byKey[key], &res)
	}
	e.commitBoardMoves(ctx, profileID, &res)
	e.commitLinks(ctx, profileID, &res)

	left, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return res, err
	}
	res.Remaining = len(left)
	return res, nil
}

// boardRow is true for the three board move entity types, which the board
// pass owns. Naming them here and in regroupEdits is what keeps them out of
// the edits pass: sorting one into it would have commitEdit send "statusId"
// to Jira as a field, fail on it, and take the issue's genuine edits down
// with it.
func boardRow(entityType string) bool {
	switch entityType {
	case issuerepo.EntityTransition, issuerepo.EntityRank, issuerepo.EntitySprintMove:
		return true
	}
	return false
}

// heldBoardRow says whether any of the rows is a board move that the board
// pass can hold back as a conflict. A rank is not one: it has no before_val
// to compare and is never held.
func heldBoardRow(rows []journal.PendingChange) bool {
	for _, p := range rows {
		if p.EntityType == issuerepo.EntityTransition || p.EntityType == issuerepo.EntitySprintMove {
			return true
		}
	}
	return false
}

// isDraftKey says the key is a local placeholder Commit has not created
// yet, so its rows wait for the next one.
func isDraftKey(key string) bool { return strings.HasPrefix(key, issuerepo.DraftPrefix) }

func (e *Engine) commitCreate(ctx context.Context, profileID, projectKey, tempKey string, rows []journal.PendingChange, res *Result) {
	var createRow journal.PendingChange
	for _, p := range rows {
		if p.EntityType == issuerepo.EntityIssueCreate {
			createRow = p
		}
	}
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(createRow.AfterVal), &d); err != nil {
		// A draft that will not decode will not decode on the next Commit
		// either; the row has to be discarded, not retried.
		res.Failures = append(res.Failures, failure(tempKey, issuerepo.EntityIssueCreate, "the draft could not be decoded: "+err.Error(), false))
		return
	}
	realKey, err := e.b.CreateIssue(ctx, projectKey, d)
	if err != nil {
		res.Failures = append(res.Failures, failure(tempKey, issuerepo.EntityIssueCreate, err.Error(), true))
		return
	}
	if err := e.repo.Rekey(ctx, profileID, tempKey, realKey); err != nil {
		// Jira has the issue even though the local rename failed. Clear the
		// journal under the temp key and audit the creation there so a retry
		// reconciles instead of posting a duplicate; report the real key so
		// the user can find it. The next full sync brings its row in.
		if merr := e.repo.MarkCreatedWithoutRekey(ctx, profileID, tempKey, realKey, rows); merr != nil {
			// Jira holds the issue and the create row is still pending, so a
			// second Commit would post a duplicate rather than recover.
			res.Failures = append(res.Failures, failure(realKey, issuerepo.EntityIssueCreate, fmt.Sprintf("created in Jira as %s but the local row could not be renamed, and the journal could not be cleared: %v", realKey, merr), false))
			return
		}
		// The journal is clear, so there is nothing left for a retry to push:
		// the next sync brings the real row in.
		res.Failures = append(res.Failures, failure(realKey, issuerepo.EntityIssueCreate, fmt.Sprintf("created in Jira as %s but the local row could not be renamed: %v", realKey, err), false))
		return
	}
	// Rekey already moved these rows' audit trail to realKey; follow suit so
	// the commit entries land there too instead of under the old temp key.
	for i := range rows {
		rows[i].EntityKey = realKey
	}
	if err := e.repo.MarkCommitted(ctx, profileID, rows); err != nil {
		// Same duplicate risk as the rekey failure above: Jira has the issue
		// and the create row survived, so this is not for the user to retry.
		res.Failures = append(res.Failures, failure(realKey, issuerepo.EntityIssueCreate, "created in Jira but the journal could not be cleared: "+err.Error(), false))
		return
	}
	e.refresh(ctx, profileID, realKey)
	res.Created = append(res.Created, Created{TempKey: tempKey, Key: realKey})
}

func (e *Engine) commitEdit(ctx context.Context, profileID, key string, rows []journal.PendingChange, res *Result) {
	remote, err := e.b.GetIssue(ctx, key)
	if err != nil {
		res.Failures = append(res.Failures, failure(key, issuerepo.EntityIssue, err.Error(), true))
		return
	}
	// rows is sorted oldest first; comparing against the oldest edit's base is
	// enough because a later edit on the same key can only keep that base or
	// move it forward (EditField reads the row's current `updated` for a
	// fresh edit and journal.Upsert leaves an existing base alone), so it
	// never predates the oldest row's base.
	if remote.Updated != rows[0].BaseVersion {
		res.Conflicts = append(res.Conflicts, e.conflict(ctx, key, remote, rows))
		return
	}
	for _, p := range rows {
		if strings.HasPrefix(p.AfterVal, issuerepo.DraftPrefix) {
			res.Failures = append(res.Failures, failure(key, issuerepo.EntityIssue, fmt.Sprintf("the epic %s has not been created in Jira yet, so this change waits for the next commit", p.AfterVal), true))
			return
		}
	}
	fields := make(map[string]string, len(rows))
	for _, p := range rows {
		fields[p.Field] = p.AfterVal
	}
	if err := e.b.UpdateIssue(ctx, key, fields); err != nil {
		res.Failures = append(res.Failures, failure(key, issuerepo.EntityIssue, err.Error(), true))
		return
	}
	if err := e.repo.MarkCommitted(ctx, profileID, rows); err != nil {
		res.Failures = append(res.Failures, failure(key, issuerepo.EntityIssue, "pushed to Jira but the journal could not be cleared: "+err.Error(), true))
		return
	}
	e.refresh(ctx, profileID, key)
	res.Committed = append(res.Committed, key)
}

// commitLinks pushes every link row, read fresh so a link added from a
// draft carries the key the create pass gave it. A row whose source is
// still a draft (its create failed this pass) is left for next time: it is
// neither pushed nor reported as a failure. Each push is its own journal
// delete, and the source's detail cache is dropped so the panel refetches
// the links Jira now holds.
func (e *Engine) commitLinks(ctx context.Context, profileID string, res *Result) {
	all, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		res.Failures = append(res.Failures, failure("links", issuerepo.EntityLink, "the journal could not be read for links: "+err.Error(), true))
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	for _, p := range all {
		if p.EntityType != issuerepo.EntityLink {
			continue
		}
		if strings.HasPrefix(p.EntityKey, issuerepo.DraftPrefix) {
			continue
		}
		var d backend.LinkDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			// A link row nobody can decode is not going to decode next time.
			res.Failures = append(res.Failures, linkFailure(p, "the link could not be decoded: "+err.Error(), false))
			continue
		}
		if err := e.b.CreateLink(ctx, p.EntityKey, d); err != nil {
			res.Failures = append(res.Failures, linkFailure(p, err.Error(), true))
			continue
		}
		if err := e.repo.MarkCommitted(ctx, profileID, []journal.PendingChange{p}); err != nil {
			res.Failures = append(res.Failures, linkFailure(p, "linked in Jira but the journal could not be cleared: "+err.Error(), true))
			continue
		}
		if err := e.repo.ClearDetail(ctx, profileID, p.EntityKey); err != nil {
			log.Printf("tam: clear detail for %s after a link push: %v", p.EntityKey, err)
		}
		res.Linked = append(res.Linked, Linked{Key: p.EntityKey, ToKey: d.ToKey, Type: d.Type})
	}
}

// linkFailure is one link row that did not land, carrying its row id so the
// dialog can offer to discard exactly that link. Its caller says whether a
// retry can help, for the same reason failure's does.
func linkFailure(p journal.PendingChange, message string, retryable bool) Failure {
	return Failure{Key: p.EntityKey, EntityType: issuerepo.EntityLink, RowID: p.ID, Error: message, Retryable: retryable, Reachable: []string{}}
}

// regroupEdits re-lists the pending changes after the creates pass, in
// case a create's Rekey repointed a pending parentKey edit at the real
// key, and regroups the non-link, non-create rows by key (oldest first per
// key, keys sorted). On a read error it records a Failure and returns
// orig and edits unchanged, so a transient read problem does not drop
// edits already known from the pre-create snapshot.
func (e *Engine) regroupEdits(ctx context.Context, profileID string, orig map[string][]journal.PendingChange, edits []string, res *Result) (map[string][]journal.PendingChange, []string) {
	all, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		res.Failures = append(res.Failures, failure("edits", issuerepo.EntityIssue, "the journal could not be reread after the creates pass: "+err.Error(), true))
		return orig, edits
	}
	byKey := map[string][]journal.PendingChange{}
	var keys []string
	for _, p := range all {
		if p.EntityType == issuerepo.EntityLink || p.EntityType == issuerepo.EntityIssueCreate || boardRow(p.EntityType) {
			continue
		}
		if _, seen := byKey[p.EntityKey]; !seen {
			keys = append(keys, p.EntityKey)
		}
		byKey[p.EntityKey] = append(byKey[p.EntityKey], p)
	}
	sort.Strings(keys)
	for k := range byKey {
		rows := byKey[k]
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	}
	return byKey, keys
}

// conflict builds the three-way view. The remote description is fetched
// only when a description edit is pending.
func (e *Engine) conflict(ctx context.Context, key string, remote backend.Issue, rows []journal.PendingChange) Conflict {
	c := Conflict{Key: key, Summary: remote.Summary, RemoteVersion: remote.Updated, Fields: []FieldConflict{}}
	remoteDesc := ""
	for _, p := range rows {
		if p.Field == "description" {
			if d, err := e.b.GetIssueDetail(ctx, key); err == nil {
				remoteDesc = d.Description
			}
			break
		}
	}
	for _, p := range rows {
		c.Fields = append(c.Fields, FieldConflict{
			Field: p.Field, Base: p.BeforeVal, Mine: p.AfterVal,
			Remote: issuerepo.FieldValue(remote, remoteDesc, p.Field),
		})
	}
	return c
}

// refresh replaces the row from Jira after a push. A failed read is logged,
// not reported: the push succeeded and the next sync refreshes the row.
func (e *Engine) refresh(ctx context.Context, profileID, key string) {
	fresh, err := e.b.GetIssue(ctx, key)
	if err != nil {
		log.Printf("tam: refresh %s after commit: %v", key, err)
		return
	}
	if err := e.repo.ReplaceRow(ctx, profileID, fresh); err != nil {
		log.Printf("tam: replace %s after commit: %v", key, err)
	}
}

// ResolveOverride rebases the held issue's pending changes onto what Jira
// holds now, so the next Commit pushes them over Jira's values. Override
// means the same thing for every row it touches, "take my change anyway",
// but the two kinds of row are held back by different comparisons and so
// have to be rebased differently.
//
// An edit is held back on the issue's updated stamp, which the conflict
// card already carries, so its rebase is local. A board move is held back
// on the row's before_val against the remote status or sprint id, which
// nothing reads a base version for: without rewriting that value the user
// would meet the identical conflict on every Commit from here on. So a key
// with a board row pending costs one read of the issue, and only such a
// key: an override with nothing but edits behind it still works offline.
func (e *Engine) ResolveOverride(ctx context.Context, profileID, key, remoteVersion string) error {
	if err := e.repo.SetBaseVersion(ctx, profileID, key, remoteVersion); err != nil {
		return err
	}
	rows, err := e.repo.PendingForKey(ctx, profileID, key)
	if err != nil {
		return err
	}
	if !heldBoardRow(rows) {
		return nil
	}
	remote, err := e.b.GetIssue(ctx, key)
	if err != nil {
		return err
	}
	return e.repo.RebaseMoves(ctx, profileID, key, remote)
}

// ResolveKeepRemote drops the held issue's edits and takes Jira's row. It
// fetches before discarding anything, so a network failure leaves the local
// edits intact instead of dropping them for a row it never got.
func (e *Engine) ResolveKeepRemote(ctx context.Context, profileID, key string) error {
	fresh, err := e.b.GetIssue(ctx, key)
	if err != nil {
		return err
	}
	if _, err := e.repo.DiscardKey(ctx, profileID, key); err != nil {
		return err
	}
	return e.repo.ReplaceRow(ctx, profileID, fresh)
}
