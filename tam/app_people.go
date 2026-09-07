package main

import (
	"log"
	"strings"

	"agile-suite/tam/internal/backend"
)

// app_people.go answers the two lookups the create and edit forms need from
// the instance rather than from the user's memory: who can be assigned an
// issue, and what the priorities are called.

// SearchUsers lists the people assignable on this profile whose username or
// display name matches query. Jira is asked first and its answer is cached;
// when Jira cannot be reached the cache answers on its own, so the picker
// keeps working offline and on a demo profile. A blank query is the picker's
// opening list.
func (a *App) SearchUsers(profileID, query string) ([]backend.User, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	users, err := b.SearchUsers(a.ctx, p.ProjectKey, strings.TrimSpace(query))
	if err != nil {
		cached, cacheErr := a.repo.SearchCachedUsers(a.ctx, profileID, query)
		if cacheErr != nil {
			return nil, err // the live failure is the one worth reporting
		}
		log.Printf("tam: user search for %s fell back to the cache: %v", p.Name, err)
		return cached, nil
	}
	if err := a.repo.CacheUsers(a.ctx, profileID, users); err != nil {
		// A cache write failing only costs the next offline lookup.
		log.Printf("tam: cache users for %s: %v", p.Name, err)
	}
	return users, nil
}

// ListPriorities is the instance's priority names, highest first. The forms
// fall back to a free-text field when this fails, so the error is theirs to
// render rather than something to swallow here.
func (a *App) ListPriorities(profileID string) ([]string, error) {
	_, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	return b.Priorities(a.ctx)
}

// GetSubtaskTypeName is what this profile's project calls its sub-task level,
// "" when it has none. The forms use it to name what they are creating,
// because the instance chooses the word: "Sub-task" by default, "Technical
// task" on some.
func (a *App) GetSubtaskTypeName(profileID string) (string, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return "", err
	}
	return b.SubtaskTypeName(a.ctx, p.ProjectKey)
}
