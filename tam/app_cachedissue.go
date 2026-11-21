package main

import (
	"errors"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// GetCachedIssue is one cached row by key, for the detail panel opening a
// linked or parent issue in place. A key the sync never stored answers nil
// rather than an error, so the panel can say so and offer Jira instead.
func (a *App) GetCachedIssue(profileID, key string) (*backend.Issue, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	iss, err := a.repo.GetIssue(a.ctx, profileID, key)
	if errors.Is(err, issuerepo.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &iss, nil
}
