package main

import (
	"errors"

	"agile-suite/pie/internal/query"
)

// RefreshLinks fetches again the issues current's own rows link to, without
// running the query. The rows the query matched stay as they are. With a
// saved query's id the refreshed run replaces its stored one, if that is
// still the run from runAt; 0 refreshes only what the frontend shows.
func (a *App) RefreshLinks(profileID string, queryID int64, runAt string, current query.Result) (query.Result, error) {
	if err := a.requireStore(); err != nil {
		return query.Result{}, err
	}
	client, err := a.client(profileID)
	if err != nil {
		return query.Result{}, err
	}
	names, err := a.queries.Fields(profileID)
	if err != nil {
		return query.Result{}, err
	}
	ids, err := fieldIDs(a.ctx, client, names)
	if err != nil {
		return query.Result{}, err
	}
	own := current
	own.Issues = []query.Issue{}
	for _, is := range current.Issues {
		if is.Matched {
			own.Issues = append(own.Issues, is)
		}
	}
	res := query.Follow(a.progress(), client, own, ids)
	// A refresh that stopped partway would replace a whole run with part of
	// one, so the stored run and the screen keep what they had.
	if res.LinkError != "" {
		return query.Result{}, errors.New("the links could not be refreshed: " + res.LinkError)
	}
	if queryID != 0 {
		if err := a.queries.ReplaceResult(profileID, queryID, runAt, res); err != nil {
			return query.Result{}, err
		}
	}
	return res, nil
}
