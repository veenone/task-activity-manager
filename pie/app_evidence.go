package main

import "agile-suite/pie/internal/evidence"

// The bindings for a case's evidence. Each returns the case's evidence as it
// stands after the change, so the page redraws from what was stored. None of
// them touches a verdict.

// Evidence returns a case's evidence.
func (a *App) Evidence(profileID string, caseID int64) ([]evidence.Item, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.evidence.List(profileID, caseID)
}

// FindEvidence asks Jira for suggestions about the case's issue: its links,
// projects sharing a component or fix version name, and the analyst's phrases.
func (a *App) FindEvidence(profileID string, caseID int64, phrases []string) ([]evidence.Item, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	c, err := a.cases.Get(profileID, caseID)
	if err != nil {
		return nil, err
	}
	client, err := a.client(profileID)
	if err != nil {
		return nil, err
	}
	if _, err := evidence.Find(a.ctx, client, a.evidence, profileID, caseID, c.Key, phrases); err != nil {
		return nil, err
	}
	return a.evidence.List(profileID, caseID)
}

// SetEvidenceState accepts or dismisses a suggestion.
func (a *App) SetEvidenceState(profileID string, caseID, id int64, state string) ([]evidence.Item, error) {
	return a.afterEvidence(profileID, caseID, func() error { return a.evidence.SetState(profileID, id, state) })
}

// AddEvidenceNote records a note for one project on a case.
func (a *App) AddEvidenceNote(profileID string, caseID int64, project, text string) ([]evidence.Item, error) {
	return a.afterEvidence(profileID, caseID, func() error { return a.evidence.AddNote(profileID, caseID, project, text) })
}

// AddEvidenceIssue records an issue the analyst found for one project.
func (a *App) AddEvidenceIssue(profileID string, caseID int64, project, key string) ([]evidence.Item, error) {
	return a.afterEvidence(profileID, caseID, func() error { return a.evidence.AddIssue(profileID, caseID, project, key) })
}

// RemoveEvidence deletes evidence added by hand.
func (a *App) RemoveEvidence(profileID string, caseID, id int64) ([]evidence.Item, error) {
	return a.afterEvidence(profileID, caseID, func() error { return a.evidence.Remove(profileID, id) })
}

func (a *App) afterEvidence(profileID string, caseID int64, change func() error) ([]evidence.Item, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	if err := change(); err != nil {
		return nil, err
	}
	return a.evidence.List(profileID, caseID)
}
