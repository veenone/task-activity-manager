package main

import (
	"context"
	"time"

	"agile-suite/pie/internal/issuegraph"
	"agile-suite/pie/internal/query"
)

// ExpandIssue returns an issue and the issues it links to, for the Link map's
// issue graph (#464). Jira answers when it can, and what it sends is kept;
// when Jira cannot be reached the answer comes from what was kept, marked
// Offline.
func (a *App) ExpandIssue(profileID, key string) (issuegraph.Neighbourhood, error) {
	if err := a.requireStore(); err != nil {
		return issuegraph.Neighbourhood{}, err
	}
	client, err := a.client(profileID)
	if err != nil {
		return issuegraph.Neighbourhood{}, err
	}
	names, err := a.queries.Fields(profileID)
	if err != nil {
		return issuegraph.Neighbourhood{}, err
	}
	// Reading the custom field ids asks Jira too. Offline, the graph does
	// without them: it draws no product base or client field.
	ids, err := fieldIDs(a.ctx, client, names)
	if err != nil && !issuegraph.IsOffline(err) {
		return issuegraph.Neighbourhood{}, err
	}
	fetch := func(ctx context.Context, keys []string) ([]query.Issue, error) {
		return query.Issues(ctx, client, keys, ids)
	}
	return issuegraph.Expand(a.ctx, fetch, issuegraph.New(a.db.DB()), profileID, key, time.Now())
}
