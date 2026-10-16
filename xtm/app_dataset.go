package main

import (
	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

// GetTestDataset returns a test's dataset: the parameters its ${placeholders}
// refer to, and one row per iteration (#267). The detail panel calls it when
// it opens a test.
//
// Reads are lazy and cache-first, like GetTestSteps, but the cache check is
// deliberately NOT the same. GetTestSteps treats "rows came back" as a hit,
// which works because nearly every test has steps. Nearly every test has no
// dataset, so this stores the absence as a fact and hits on the row existing.
// Checking for a non-empty dataset instead would mean the nineteen tests in
// twenty that are not parameterized each pay for a Jira round trip on every
// single open, to be told again that there is nothing.
//
// forceRefresh is the panel's Refresh: skip the cache and overwrite it.
func (a *App) GetTestDataset(profileID, testKey string, forceRefresh bool) (testrepo.Dataset, error) {
	if err := a.requireStore(); err != nil {
		return testrepo.Dataset{}, err
	}
	// An uncommitted local Test has a placeholder key Jira would reject, so it
	// is answered from the store alone, as its steps are.
	if isLocalTestKey(testKey) {
		ds, _, err := a.repo.GetTestDataset(profileID, testKey, "")
		return ds, err
	}
	if !forceRefresh {
		cached, found, err := a.repo.GetTestDataset(profileID, testKey, "")
		if err != nil {
			return testrepo.Dataset{}, err
		}
		if found {
			return cached, nil
		}
	}

	b, err := a.backendFor(profileID)
	if err != nil {
		return testrepo.Dataset{}, err
	}
	remote, err := b.GetDataset(a.ctx, testKey, "")
	if err != nil {
		return testrepo.Dataset{}, err
	}

	ds := toRepoDataset(remote)
	// Cache on success only. Writing a row after a failed fetch would record
	// "this test has no dataset" on the strength of a server error, and the
	// panel would keep showing nothing until someone pressed Refresh.
	if err := a.repo.SaveTestDataset(profileID, testKey, "", ds); err != nil {
		return testrepo.Dataset{}, err
	}
	return ds, nil
}

// SaveTestDataset replaces a test's dataset locally and queues it for commit
// (#277 W1). The editor calls it; Jira hears nothing until Commit, like every
// other edit in XTM.
//
// A table Xray would refuse is refused here, with the reason, and neither
// cached nor queued. The check lives in the repository so no path can skip it.
func (a *App) SaveTestDataset(profileID, testKey string, ds testrepo.Dataset) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.EditTestDataset(profileID, testKey, ds)
}

// toRepoDataset maps the neutral backend DTO onto the stored shape.
func toRepoDataset(d backend.Dataset) testrepo.Dataset {
	params := make([]testrepo.DatasetParameter, len(d.Parameters))
	for i, p := range d.Parameters {
		params[i] = testrepo.DatasetParameter{
			Name: p.Name, Combinatorial: p.Combinatorial, ID: p.ID,
			Type: p.Type, Options: p.Options, ListScope: p.ListScope, ListID: p.ListID,
		}
	}
	return testrepo.Dataset{Parameters: params, Rows: d.Rows}
}
