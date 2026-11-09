package main

import (
	"errors"
	"strings"

	"agile-suite/xtm/internal/backend"
)

// ContainerTestsMatching asks the backend which of a container's member tests
// also match jql, so the member table can keep only those, the way Jira's own
// Test Execution page filters its tests (#446). It needs a Jira-side backend
// and a connection; Kiwi has no JQL and the demo profile has no Jira.
func (a *App) ContainerTestsMatching(profileID, containerKey, jql string) ([]string, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(jql) == "" {
		return nil, errors.New("enter a JQL query to filter by")
	}
	board, err := a.repo.GetContainerBoard(profileID, containerKey)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(board.Rows))
	for _, r := range board.Rows {
		keys = append(keys, r.TestKey)
	}
	if len(keys) == 0 {
		return []string{}, nil
	}
	b, err := a.backendFor(profileID)
	if err != nil {
		return nil, err
	}
	m, ok := b.(backend.KeyMatcher)
	if !ok {
		return nil, errors.New("this profile's backend has no JQL, so its tests cannot be filtered by it")
	}
	return m.KeysMatching(a.ctx, keys, jql)
}
