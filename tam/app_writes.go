package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
)

// acquire marks the profile as running what ("sync", "commit", or "import")
// and refuses while any of them runs. Callers defer release.
// boardOrder is the board-aware half of the store, the one both the commit
// pass and the drop's own check read the columns through.
func (a *App) boardOrder() boardrepo.Order {
	return boardrepo.Order{Boards: a.boards, Issues: a.repo}
}

func (a *App) acquire(profileID, what string) error {
	a.backendMu.Lock()
	defer a.backendMu.Unlock()
	if cur := a.busy[profileID]; cur != "" {
		return fmt.Errorf("a %s is already running for this profile", cur)
	}
	a.busy[profileID] = what
	return nil
}

func (a *App) release(profileID string) {
	a.backendMu.Lock()
	delete(a.busy, profileID)
	a.backendMu.Unlock()
}

// EditIssue journals one field change on an issue or a draft.
func (a *App) EditIssue(profileID, key, field, value string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if strings.TrimSpace(profileID) == "" {
		return errors.New("no profile selected")
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("issue key is empty")
	}
	return a.repo.EditField(a.ctx, profileID, key, field, value)
}

// CreateIssue stores a draft under a temporary key and returns that key.
func (a *App) CreateIssue(profileID string, draft backend.IssueDraft) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	return a.repo.CreateDraft(a.ctx, p.ID, p.ProjectKey, draft)
}

// GetCreateFields asks the backend which required fields the New issue
// form must add for the type.
func (a *App) GetCreateFields(profileID, typeName string) ([]backend.FieldSpec, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	specs, err := b.CreateFields(a.ctx, p.ProjectKey, typeName)
	if err != nil {
		return nil, err
	}
	if specs == nil {
		specs = []backend.FieldSpec{}
	}
	return specs, nil
}

// ListPendingChanges returns the profile's journal, newest first.
func (a *App) ListPendingChanges(profileID string) ([]journal.PendingChange, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	rows, err := a.repo.ListPendingChanges(a.ctx, profileID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []journal.PendingChange{}
	}
	return rows, nil
}

// DiscardPendingChange reverts one journaled change.
func (a *App) DiscardPendingChange(profileID string, id int64) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.DiscardPendingChange(a.ctx, profileID, id)
}

// DiscardAllPendingChanges reverts every journaled change of the profile.
func (a *App) DiscardAllPendingChanges(profileID string) (int, error) {
	if err := a.requireStore(); err != nil {
		return 0, err
	}
	return a.repo.DiscardAllPendingChanges(a.ctx, profileID)
}

// CommitPendingChanges pushes the journal to Jira. It refuses while a sync
// runs for the profile, and a sync refuses while it runs.
func (a *App) CommitPendingChanges(profileID string) (committer.Result, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return committer.Result{}, err
	}
	if err := a.acquire(p.ID, "commit"); err != nil {
		return committer.Result{}, err
	}
	defer a.release(p.ID)
	b, err := a.backendFor(p)
	if err != nil {
		return committer.Result{}, err
	}
	res, err := a.commitEngine(b).Commit(a.ctx, p.ID, p.ProjectKey)
	if err != nil {
		log.Printf("tam: commit %s (%s) failed: %v", p.Name, p.ProjectKey, err)
		return res, err
	}
	log.Printf("tam: committed %s (%s): %d pushed, %d created, %d moved, %d conflicts, %d failures, %d left",
		p.Name, p.ProjectKey, len(res.Committed), len(res.Created), len(res.Moved), len(res.Conflicts), len(res.Failures), res.Remaining)
	return res, nil
}

// commitEngine builds the commit engine over the profile's backend. The
// board order comes from boardrepo paired with the issue cache, which is
// what the rank group re-derives each neighbour from; app.go is the one
// place holding both repositories, so it is where they are joined.
func (a *App) commitEngine(b backend.IssueBackend) *committer.Engine {
	return committer.New(b, a.repo, a.boardOrder())
}

// ResolveConflictOverride rebases a held issue's edits so the next Commit
// pushes them over Jira's values.
func (a *App) ResolveConflictOverride(profileID, key, remoteVersion string) error {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return err
	}
	return a.commitEngine(b).ResolveOverride(a.ctx, p.ID, key, remoteVersion)
}

// ResolveConflictKeepRemote drops a held issue's edits and takes Jira's row.
func (a *App) ResolveConflictKeepRemote(profileID, key string) error {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return err
	}
	return a.commitEngine(b).ResolveKeepRemote(a.ctx, p.ID, key)
}

// ListActivity returns the local audit trail of one issue, newest first.
func (a *App) ListActivity(profileID, key string, limit int) ([]journal.AuditEntry, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	rows, err := a.repo.ListActivity(a.ctx, profileID, key, limit)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []journal.AuditEntry{}
	}
	return rows, nil
}

// GetLinkTypes lists the link types the profile's Jira defines.
func (a *App) GetLinkTypes(profileID string) ([]backend.LinkType, error) {
	_, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	types, err := b.LinkTypes(a.ctx)
	if err != nil {
		return nil, err
	}
	if types == nil {
		types = []backend.LinkType{}
	}
	return types, nil
}

// LookupIssue reads one issue from Jira by key, any project, so the Add
// link form can confirm a target and show its summary.
func (a *App) LookupIssue(profileID, key string) (backend.Issue, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return backend.Issue{}, errors.New("issue key is empty")
	}
	_, b, err := a.backendForProfile(profileID)
	if err != nil {
		return backend.Issue{}, err
	}
	return b.GetIssue(a.ctx, key)
}

// AddLink journals a link from key to the draft's target.
func (a *App) AddLink(profileID, key string, link backend.LinkDraft) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	return a.repo.AddLink(a.ctx, p.ID, strings.TrimSpace(key), link)
}

// GetEpicTree returns the Epics view's data from the cache.
func (a *App) GetEpicTree(profileID string, q issuerepo.TreeQuery) (issuerepo.Tree, error) {
	if err := a.requireStore(); err != nil {
		return issuerepo.Tree{}, err
	}
	return a.repo.EpicTree(a.ctx, profileID, q)
}

// ListEpics returns the cached epics for the Epic picker.
func (a *App) ListEpics(profileID string) ([]backend.Issue, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	epics, err := a.repo.ListEpics(a.ctx, profileID)
	if err != nil {
		return nil, err
	}
	if epics == nil {
		epics = []backend.Issue{}
	}
	return epics, nil
}
