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
// own names. Jira is asked and its answer cached against the issue's project
// and type, the two things it decides a screen by, so every other issue of
// that type answers from disk and the app keeps editing with no network.
//
// An empty list means nothing is known, not that nothing may be edited: a
// profile that has never reached Jira has no screen to draw from, and the
// panel falls back to its own fixed list rather than refusing every field.
// A draft has no issue in Jira to ask about at all, so it is answered the
// same way without a request.
func (a *App) GetEditableFields(profileID, key string) ([]string, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	iss, err := a.repo.GetIssue(a.ctx, profileID, key)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(key, issuerepo.DraftPrefix) {
		return []string{}, nil
	}
	fields, err := b.EditableFields(a.ctx, key)
	if err != nil {
		cached, ok, cacheErr := a.repo.EditScreen(a.ctx, profileID, iss.Project, iss.Type)
		if cacheErr != nil || !ok {
			log.Printf("tam: the edit screen of %s on %s could not be read and none is cached: %v", key, p.Name, err)
			return []string{}, nil
		}
		log.Printf("tam: the edit screen of %s on %s fell back to the cache: %v", key, p.Name, err)
		return cached, nil
	}
	if err := a.repo.PutEditScreen(a.ctx, profileID, iss.Project, iss.Type, fields); err != nil {
		// A cache write failing only costs the next offline read.
		log.Printf("tam: cache the edit screen of %s %s: %v", iss.Project, iss.Type, err)
	}
	return fields, nil
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
