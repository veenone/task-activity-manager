// Package committer pushes TAM's journal to Jira in phases (phases.go):
// draft sprints, then draft epics, then the other drafts, then sub-tasks,
// each followed by a re-read of the journal so the next phase sees the ids
// the last one rewrote; then each edited issue is version-checked, pushed,
// and refreshed, then the board moves in boards.go, then the links. A row
// naming a placeholder this Commit could not make real is held, not sent.
// An issue whose remote version moved is held back as a conflict carrying
// base, mine, and remote for every pending field; the two resolutions rebase
// the edits or drop them.
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
	// LeftOut names the draft's extra fields the create did not send, for a
	// person: a field nothing could confirm belongs on the issue type's
	// create screen is left out rather than refused by Jira, and this is
	// what keeps that from happening in silence. Empty on almost every
	// create.
	LeftOut []string `json:"leftOut"`
}

// CreatedSprint pairs a draft sprint's negative id with the id Jira gave it.
type CreatedSprint struct {
	DraftID int    `json:"draftId"`
	ID      int    `json:"id"`
	Name    string `json:"name"`
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

// Result is what one Commit did. Held are the rows it did not send because
// something they name was not created; Remaining counts the journal rows
// left, held ones included.
type Result struct {
	Committed      []string        `json:"committed"`
	Created        []Created       `json:"created"`
	CreatedSprints []CreatedSprint `json:"createdSprints"`
	SprintsChanged []string        `json:"sprintsChanged"`
	Linked         []Linked        `json:"linked"`
	Moved          []Moved         `json:"moved"`
	Conflicts      []Conflict      `json:"conflicts"`
	Failures       []Failure       `json:"failures"`
	Held           []Held          `json:"held"`
	Remaining      int             `json:"remaining"`
}

// Engine runs commits for one backend and repository pair. order is the
// board order the rank group re-derives its neighbours from; a nil one
// drops the ranks with that reason rather than pushing them against a
// neighbour nobody checked.
type Engine struct {
	b     backend.IssueBackend
	repo  *issuerepo.Repository
	order BoardOrder
	// Sprints pushes sprint edits, starts, completions and deletes; nil fails
	// each one.
	Sprints SprintWriter
}

// New returns an engine over the backend, the store, and the board order.
func New(b backend.IssueBackend, repo *issuerepo.Repository, order BoardOrder) *Engine {
	return &Engine{b: b, repo: repo, order: order}
}

// Commit pushes every pending change of the profile, phase by phase. Only a
// store failure before the first phase returns an error: once a phase has
// run, Jira may hold what it wrote, and Wails drops the Result of a call
// that also returns an error. So per-row outcomes land in the Result, a
// journal that cannot be re-read between two phases stops the Commit with a
// failure saying so, keeping what the earlier phases did, and a count of the
// rows left that cannot be read keeps the last count that could.
func (e *Engine) Commit(ctx context.Context, profileID, projectKey string) (Result, error) {
	res := Result{
		Committed: []string{}, Created: []Created{}, CreatedSprints: []CreatedSprint{}, SprintsChanged: []string{}, Linked: []Linked{},
		Moved: []Moved{}, Conflicts: []Conflict{}, Failures: []Failure{}, Held: []Held{},
	}
	run := &commitRun{e: e, profileID: profileID, projectKey: projectKey, res: &res, deps: newDependencies()}
	if err := run.reload(ctx); err != nil {
		return res, err
	}
	all := phases()
	for i, p := range all {
		p.run(ctx, run)
		if i == len(all)-1 {
			break
		}
		if err := run.reload(ctx); err != nil {
			res.Failures = append(res.Failures, failure(p.name, "", fmt.Sprintf("the journal could not be reread after the %s phase, so the rest of this Commit did not run: %v", p.name, err), true))
			break
		}
	}
	// reload leaves rows as they were when it fails, which is the count the
	// last successful read gave.
	if err := run.reload(ctx); err != nil {
		log.Printf("tam: count the rows left after a commit: %v", err)
	}
	res.Remaining = len(run.rows)
	return res, nil
}

// boardRow is true for the three board move entity types, which the board
// moves pass (boards.go) owns. pushEdits takes only EntityIssue rows, which
// is what keeps them out of the edits phase: sorting one into it would have
// commitEdit send "statusId" to Jira as a field, fail on it, and take the
// issue's genuine edits down with it.
func boardRow(entityType string) bool {
	switch entityType {
	case issuerepo.EntityTransition, issuerepo.EntityRank, issuerepo.EntitySprintMove:
		return true
	}
	return false
}

// heldBoardRow says whether any of the rows is a board move that the board
// pass can hold back as a conflict. A rank is not one: it has no before_val
// to compare and is never held. Neither is an issue_board add: it has no
// remote scalar to check it against the way a status or a sprint id does,
// so the board pass pushes it straight, the same as a rank.
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

// commitCreate posts one draft and rekeys it. A refused create blocks its
// key, so every draft, edit, move and link naming it is held this Commit.
func (e *Engine) commitCreate(ctx context.Context, r *commitRun, createRow journal.PendingChange, d backend.IssueDraft) {
	tempKey := createRow.EntityKey
	realKey, leftOut, err := e.b.CreateIssue(ctx, r.projectKey, d)
	if err != nil {
		r.res.Failures = append(r.res.Failures, failure(tempKey, issuerepo.EntityIssueCreate, err.Error(), true))
		r.deps.block(tempKey, tempKey, "which Jira refused")
		return
	}
	rows := []journal.PendingChange{createRow}
	if err := e.repo.Rekey(ctx, r.profileID, tempKey, realKey); err != nil {
		// Jira has the issue even though the local rename failed. Clear the
		// journal under the temp key and audit the creation there so a retry
		// reconciles instead of posting a duplicate; report the real key so
		// the user can find it. The next full sync brings its row in.
		r.deps.block(tempKey, tempKey, fmt.Sprintf("which Jira created as %s but TAM could not rename; sync, then set it again", realKey))
		if merr := e.repo.MarkCreatedWithoutRekey(ctx, r.profileID, tempKey, realKey, rows); merr != nil {
			// Jira holds the issue and the create row is still pending, so a
			// second Commit would post a duplicate rather than recover.
			r.res.Failures = append(r.res.Failures, failure(realKey, issuerepo.EntityIssueCreate, fmt.Sprintf("created in Jira as %s but the local row could not be renamed, and the journal could not be cleared: %v", realKey, merr), false))
			return
		}
		r.res.Failures = append(r.res.Failures, failure(realKey, issuerepo.EntityIssueCreate, fmt.Sprintf("created in Jira as %s but the local row could not be renamed: %v", realKey, err), false))
		return
	}
	// Rekey already moved these rows' audit trail to realKey; follow suit so
	// the commit entries land there too instead of under the old temp key.
	rows[0].EntityKey = realKey
	if err := e.repo.MarkCommitted(ctx, r.profileID, rows); err != nil {
		// Same duplicate risk as the rekey failure above: Jira has the issue
		// and the create row survived, so this is not for the user to retry.
		r.res.Failures = append(r.res.Failures, failure(realKey, issuerepo.EntityIssueCreate, "created in Jira but the journal could not be cleared: "+err.Error(), false))
		return
	}
	e.refresh(ctx, r.profileID, realKey)
	r.res.Created = append(r.res.Created, Created{TempKey: tempKey, Key: realKey, LeftOut: leftOut})
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
	fields := make(map[string]string, len(rows))
	for _, p := range rows {
		fields[p.Field] = p.AfterVal
	}
	// pushEdits has already held or failed an edit naming a draft; a
	// reference still naming a placeholder here is a rewriting bug. Only the
	// references are checked, since a summary may read TAM-NEW-12.
	refs := map[string]any{"issue": key}
	if parent, edited := fields["parentKey"]; edited {
		refs["parentKey"] = parent
	}
	if err := assertNoPlaceholders(refs); err != nil {
		res.Failures = append(res.Failures, failure(key, issuerepo.EntityIssue, err.Error(), false))
		return
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

// commitLinks pushes every link row, read fresh so a link added from or to
// a draft carries the key the create pass gave it. A row whose source or
// target is a draft this Commit could not create is held; one whose target
// is a draft nothing in this Commit knows fails, since no retry creates it;
// any other row whose source is still a draft is left for next time,
// neither pushed nor reported. Each push is its own journal delete, and the source's detail
// cache is dropped so the panel refetches the links Jira now holds.
func (e *Engine) commitLinks(ctx context.Context, profileID string, res *Result, deps *dependencies) {
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
		if waits, held := deps.blockedBy(p.EntityKey); held {
			deps.hold(res, p.EntityKey, issuerepo.EntityLink, p.ID, waits)
			continue
		}
		if isDraftKey(p.EntityKey) {
			continue
		}
		var d backend.LinkDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			// A link row nobody can decode is not going to decode next time.
			res.Failures = append(res.Failures, linkFailure(p, "the link could not be decoded: "+err.Error(), false))
			continue
		}
		if waits, held := deps.blockedBy(d.ToKey); held {
			deps.hold(res, p.EntityKey, issuerepo.EntityLink, p.ID, waits)
			continue
		}
		if isDraftKey(d.ToKey) {
			res.Failures = append(res.Failures, linkFailure(p, fmt.Sprintf("its target %s is not a draft this Commit could create; add the link again", d.ToKey), false))
			continue
		}
		if err := assertNoPlaceholders(map[string]any{"from": p.EntityKey, "toKey": d.ToKey}); err != nil {
			res.Failures = append(res.Failures, linkFailure(p, err.Error(), false))
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

// conflict builds the three-way view from the row the version check has
// already read. The description used to cost a second round trip here,
// because backend.Issue did not carry one; it does since the sync started
// caching it, so a description conflict is one Jira call lighter.
func (e *Engine) conflict(_ context.Context, key string, remote backend.Issue, rows []journal.PendingChange) Conflict {
	c := Conflict{Key: key, Summary: remote.Summary, RemoteVersion: remote.Updated, Fields: []FieldConflict{}}
	for _, p := range rows {
		c.Fields = append(c.Fields, FieldConflict{
			Field: p.Field, Base: p.BeforeVal, Mine: p.AfterVal,
			Remote: issuerepo.FieldValue(remote, p.Field),
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
