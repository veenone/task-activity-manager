// Dataset commits, the write half of #277 W1.
//
// A separate file because commit.go is already far past the 400-line limit C2
// sets, and because this bucket behaves unlike the others: the whole table is
// sent on every write, so the conflict check has to be its own rather than the
// per-field scan in conflict.go.

package syncer

import (
	"context"
	"encoding/json"
	"fmt"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

// datasetConflictLabel names the dataset in the conflict dialog, which lists
// one line per conflicting field and has only this to go on.
const datasetConflictLabel = "Dataset"

// datasetAPIAbsent replaces the raw 404 an instance below Xray 5.1 answers
// with. The dataset endpoints arrived in 5.1, and no REST call reports the
// plugin version, so the write is how XTM finds out. Reported as a failure
// rather than a skip: the capability is on for this backend, and it is this
// instance that cannot take the write. Either way the row stays pending.
const datasetAPIAbsent = "this Jira has no Xray dataset API, which Xray 5.1 added: the dataset was not written and is still pending"

// commitDatasets pushes queued dataset edits (#277 W1).
//
// Each write replaces the test's whole table, so a remote change that the
// local edit never saw would be destroyed by committing over it. The table
// Jira holds now is read first and compared with the one the edit started
// from; anything but agreement holds the change back as a conflict for the
// user to resolve, with the three tables to choose between.
func (e *Engine) commitDatasets(ctx context.Context, profileID string, rows []testrepo.PendingChange, result *CommitResult) {
	for _, c := range rows {
		var mine testrepo.Dataset
		if err := json.Unmarshal([]byte(c.AfterVal), &mine); err != nil {
			result.Failed = append(result.Failed, FailedCommit{
				TestKey: c.EntityKey,
				Error:   fmt.Sprintf("malformed dataset payload: %s", err),
			})
			continue
		}

		remote, err := e.backend.GetDataset(ctx, c.EntityKey)
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{
				TestKey: c.EntityKey,
				Error:   "read the dataset before writing: " + sanitizeError(err.Error()),
			})
			continue
		}
		current := fromBackendDataset(remote)
		remoteJSON, err := testrepo.EncodeDataset(current)
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: c.EntityKey, Error: err.Error()})
			continue
		}
		changed, err := datasetContentDiffers(current, c.BeforeVal)
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: c.EntityKey, Error: err.Error()})
			continue
		}
		if changed {
			result.Conflicted = append(result.Conflicted, Conflict{
				TestKey: c.EntityKey,
				Fields: []ConflictField{{
					PendingID:  c.ID,
					EntityType: c.EntityType,
					EntityKey:  c.EntityKey,
					Field:      c.Field,
					Label:      datasetConflictLabel,
					Base:       c.BeforeVal,
					Remote:     remoteJSON,
					Mine:       c.AfterVal,
				}},
			})
			continue
		}

		if err := e.backend.SetDataset(ctx, c.EntityKey, toBackendDataset(adoptServerIDs(mine, current))); err != nil {
			reason := fmt.Sprintf("write the dataset of %s: %s", c.EntityKey, sanitizeError(err.Error()))
			if isNotFoundErr(err) {
				reason = datasetAPIAbsent
			}
			result.Failed = append(result.Failed, FailedCommit{TestKey: c.EntityKey, Error: reason})
			continue
		}
		if err := e.repo.CommitPendingChanges(profileID, []int64{c.ID}); err != nil {
			result.Failed = append(result.Failed, FailedCommit{
				TestKey: c.EntityKey,
				Error:   "Jira saved the dataset but local cleanup failed: " + err.Error(),
			})
			continue
		}
		result.Succeeded = append(result.Succeeded, c.EntityKey)
	}
}

// datasetContentDiffers reports whether what Jira holds has moved away from
// the table the local edit started from, comparing what a user can see rather
// than the ids underneath it.
func datasetContentDiffers(current testrepo.Dataset, before string) (bool, error) {
	var base testrepo.Dataset
	if err := json.Unmarshal([]byte(before), &base); err != nil {
		return false, fmt.Errorf("malformed dataset base: %w", err)
	}
	currentContent, err := testrepo.EncodeDatasetContent(current)
	if err != nil {
		return false, err
	}
	baseContent, err := testrepo.EncodeDatasetContent(base)
	if err != nil {
		return false, err
	}
	return currentContent != baseContent, nil
}

// adoptServerIDs puts the ids and list bindings Jira holds now onto the table
// about to be written, matching a parameter by its id where that id is still
// there and by its name otherwise.
//
// A write sends the whole table, so a stale id would be refused outright
// ("Parameters (N) not associated with the dataset") and a stale list binding
// would be written back over a list that has moved on. Matching by id first
// is what lets a rename stay a rename: the name has changed, the id has not.
// A parameter that matches neither is new, and is written as one.
func adoptServerIDs(mine, current testrepo.Dataset) testrepo.Dataset {
	byID := make(map[int]testrepo.DatasetParameter, len(current.Parameters))
	byName := make(map[string]testrepo.DatasetParameter, len(current.Parameters))
	for _, p := range current.Parameters {
		if p.ID > 0 {
			byID[p.ID] = p
		}
		byName[p.Name] = p
	}

	out := testrepo.Dataset{
		Parameters: make([]testrepo.DatasetParameter, len(mine.Parameters)),
		Rows:       mine.Rows,
	}
	for i, p := range mine.Parameters {
		server, found := byID[p.ID]
		if !found {
			server, found = byName[p.Name]
		}
		if !found {
			// New here: it keeps its name and nothing else, and the client
			// writes it as a text parameter.
			out.Parameters[i] = testrepo.DatasetParameter{
				Name: p.Name, Combinatorial: p.Combinatorial,
			}
			continue
		}
		out.Parameters[i] = testrepo.DatasetParameter{
			Name:          p.Name,
			Combinatorial: p.Combinatorial,
			ID:            server.ID,
			Type:          server.Type,
			Options:       server.Options,
			ListScope:     server.ListScope,
			ListID:        server.ListID,
		}
	}
	return out
}

// toBackendDataset / fromBackendDataset map between the stored table and the
// neutral one the backend takes. Two near-identical structs are the price of
// testrepo not importing backend.
func toBackendDataset(ds testrepo.Dataset) backend.Dataset {
	params := make([]backend.DatasetParameter, len(ds.Parameters))
	for i, p := range ds.Parameters {
		params[i] = backend.DatasetParameter{
			Name: p.Name, Combinatorial: p.Combinatorial, ID: p.ID,
			Type: p.Type, Options: p.Options, ListScope: p.ListScope, ListID: p.ListID,
		}
	}
	return backend.Dataset{Parameters: params, Rows: ds.Rows}
}

func fromBackendDataset(d backend.Dataset) testrepo.Dataset {
	params := make([]testrepo.DatasetParameter, len(d.Parameters))
	for i, p := range d.Parameters {
		params[i] = testrepo.DatasetParameter{
			Name: p.Name, Combinatorial: p.Combinatorial, ID: p.ID,
			Type: p.Type, Options: p.Options, ListScope: p.ListScope, ListID: p.ListID,
		}
	}
	return testrepo.Dataset{Parameters: params, Rows: d.Rows}
}
