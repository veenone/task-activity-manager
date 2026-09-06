// Package committer pushes TAM's journal to Jira: drafts are created and
// rekeyed first, then each edited issue is version-checked, pushed, and
// refreshed. An issue whose remote version moved is held back as a conflict
// carrying base, mine, and remote for every pending field; the two
// resolutions rebase the edits or drop them.
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

// Failure is an issue whose push or create failed; its rows stay.
type Failure struct {
	Key   string `json:"key"`
	Error string `json:"error"`
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
	Conflicts []Conflict `json:"conflicts"`
	Failures  []Failure  `json:"failures"`
	Remaining int        `json:"remaining"`
}

// Engine runs commits for one backend and repository pair.
type Engine struct {
	b    backend.IssueBackend
	repo *issuerepo.Repository
}

// New returns an engine over the backend and the store.
func New(b backend.IssueBackend, repo *issuerepo.Repository) *Engine {
	return &Engine{b: b, repo: repo}
}

// Commit pushes every pending change of the profile. Only a store failure
// returns an error; per-issue outcomes land in the Result.
func (e *Engine) Commit(ctx context.Context, profileID, projectKey string) (Result, error) {
	res := Result{Committed: []string{}, Created: []Created{}, Linked: []Linked{}, Conflicts: []Conflict{}, Failures: []Failure{}}
	all, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return res, err
	}
	byKey := map[string][]journal.PendingChange{}
	var creates, edits []string
	for _, p := range all {
		if p.EntityType == issuerepo.EntityLink {
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
	for _, key := range edits {
		e.commitEdit(ctx, profileID, key, byKey[key], &res)
	}
	e.commitLinks(ctx, profileID, &res)

	left, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return res, err
	}
	res.Remaining = len(left)
	return res, nil
}

func (e *Engine) commitCreate(ctx context.Context, profileID, projectKey, tempKey string, rows []journal.PendingChange, res *Result) {
	var createRow journal.PendingChange
	for _, p := range rows {
		if p.EntityType == issuerepo.EntityIssueCreate {
			createRow = p
		}
	}
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(createRow.AfterVal), &d); err != nil {
		res.Failures = append(res.Failures, Failure{Key: tempKey, Error: "the draft could not be decoded: " + err.Error()})
		return
	}
	realKey, err := e.b.CreateIssue(ctx, projectKey, d)
	if err != nil {
		res.Failures = append(res.Failures, Failure{Key: tempKey, Error: err.Error()})
		return
	}
	if err := e.repo.Rekey(ctx, profileID, tempKey, realKey); err != nil {
		// Jira has the issue even though the local rename failed. Clear the
		// journal under the temp key and audit the creation there so a retry
		// reconciles instead of posting a duplicate; report the real key so
		// the user can find it. The next full sync brings its row in.
		if merr := e.repo.MarkCreatedWithoutRekey(ctx, profileID, tempKey, realKey, rows); merr != nil {
			res.Failures = append(res.Failures, Failure{Key: realKey, Error: fmt.Sprintf("created in Jira as %s but the local row could not be renamed, and the journal could not be cleared: %v", realKey, merr)})
			return
		}
		res.Failures = append(res.Failures, Failure{Key: realKey, Error: fmt.Sprintf("created in Jira as %s but the local row could not be renamed: %v", realKey, err)})
		return
	}
	// Rekey already moved these rows' audit trail to realKey; follow suit so
	// the commit entries land there too instead of under the old temp key.
	for i := range rows {
		rows[i].EntityKey = realKey
	}
	if err := e.repo.MarkCommitted(ctx, profileID, rows); err != nil {
		res.Failures = append(res.Failures, Failure{Key: realKey, Error: "created in Jira but the journal could not be cleared: " + err.Error()})
		return
	}
	e.refresh(ctx, profileID, realKey)
	res.Created = append(res.Created, Created{TempKey: tempKey, Key: realKey})
}

func (e *Engine) commitEdit(ctx context.Context, profileID, key string, rows []journal.PendingChange, res *Result) {
	remote, err := e.b.GetIssue(ctx, key)
	if err != nil {
		res.Failures = append(res.Failures, Failure{Key: key, Error: err.Error()})
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
	if err := e.b.UpdateIssue(ctx, key, fields); err != nil {
		res.Failures = append(res.Failures, Failure{Key: key, Error: err.Error()})
		return
	}
	if err := e.repo.MarkCommitted(ctx, profileID, rows); err != nil {
		res.Failures = append(res.Failures, Failure{Key: key, Error: "pushed to Jira but the journal could not be cleared: " + err.Error()})
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
		res.Failures = append(res.Failures, Failure{Key: "links", Error: "the journal could not be read for links: " + err.Error()})
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
			res.Failures = append(res.Failures, Failure{Key: p.EntityKey, Error: "the link could not be decoded: " + err.Error()})
			continue
		}
		if err := e.b.CreateLink(ctx, p.EntityKey, d); err != nil {
			res.Failures = append(res.Failures, Failure{Key: p.EntityKey, Error: err.Error()})
			continue
		}
		if err := e.repo.MarkCommitted(ctx, profileID, []journal.PendingChange{p}); err != nil {
			res.Failures = append(res.Failures, Failure{Key: p.EntityKey, Error: "linked in Jira but the journal could not be cleared: " + err.Error()})
			continue
		}
		if err := e.repo.ClearDetail(ctx, profileID, p.EntityKey); err != nil {
			log.Printf("tam: clear detail for %s after a link push: %v", p.EntityKey, err)
		}
		res.Linked = append(res.Linked, Linked{Key: p.EntityKey, ToKey: d.ToKey, Type: d.Type})
	}
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

// ResolveOverride rebases the held issue's edits onto the remote version so
// the next Commit pushes them over Jira's values.
func (e *Engine) ResolveOverride(ctx context.Context, profileID, key, remoteVersion string) error {
	return e.repo.SetBaseVersion(ctx, profileID, key, remoteVersion)
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
