package main

import (
	"log"
	"strings"

	"agile-suite/tam/internal/issuerepo"
)

// app_editscreen.go answers which fields an issue may be edited with. Jira
// decides that per project and issue type through the edit screen they
// carry, and TAM has never asked: it offered a fixed list of seven and let
// Commit find out. See issue #52.

// GetEditableFields is the fields the issue's edit screen carries, by TAM's
// own names. Jira is asked on every call and its answer written to the store
// against the issue's project and type, the two things Jira decides a screen
// by. The store is read when Jira cannot be, which is what keeps the panel
// editing with no network; it is not a read-through cache, and the thing
// that stops one call per issue is the caller's own staleness window
// (queries/pending.ts keys this by issue type).
//
// An empty list means nothing is known, not that nothing may be edited. A
// profile that has never reached Jira has no screen to draw from, and the
// panel falls back to its own fixed list rather than refusing every field.
// An empty answer is never written to the store either: cached, it would
// make ListUnpushableEdits read every pending edit on that project and type
// as doomed.
//
// A draft has no issue in Jira to ask about, so it is answered as unknown
// without a request, and without reading a row for a key Jira has never
// seen.
func (a *App) GetEditableFields(profileID, key string) ([]string, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(key, issuerepo.DraftPrefix) {
		return []string{}, nil
	}
	iss, err := a.repo.GetIssue(a.ctx, profileID, key)
	if err != nil {
		return nil, err
	}
	fields, err := b.EditableFields(a.ctx, key)
	switch {
	case err == nil && len(fields) > 0:
		if err := a.repo.PutEditScreen(a.ctx, profileID, iss.Project, iss.Type, fields); err != nil {
			// A store write failing only costs the next offline read.
			log.Printf("tam: store the edit screen of %s %s: %v", iss.Project, iss.Type, err)
		}
		return fields, nil
	case err == nil:
		log.Printf("tam: the edit screen of %s on %s carries none of the fields TAM edits, which is read as nothing known", key, p.Name)
	default:
		log.Printf("tam: the edit screen of %s on %s could not be read: %v", key, p.Name, err)
	}
	if cached, ok, cacheErr := a.repo.EditScreen(a.ctx, profileID, iss.Project, iss.Type); cacheErr == nil && ok {
		return cached, nil
	}
	return []string{}, nil
}

// ListUnpushableEdits names the journalled edits whose field is not on the
// edit screen Jira last reported for their issue. Commit would be refused
// for each of them, so Pending changes says so before the user presses it.
// Nothing here discards a row: the value is what the user typed, and
// throwing it away stays their decision.
func (a *App) ListUnpushableEdits(profileID string) ([]issuerepo.UnpushableEdit, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.UnpushableEdits(a.ctx, profileID)
}
