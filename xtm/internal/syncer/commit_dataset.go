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

		remote, err := e.backend.GetDataset(ctx, c.EntityKey, "")
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{
				TestKey: c.EntityKey,
				Error:   "read the dataset before writing: " + sanitizeError(err.Error()),
			})
			continue
		}
		remoteJSON, err := testrepo.EncodeDataset(fromBackendDataset(remote))
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: c.EntityKey, Error: err.Error()})
			continue
		}
		if remoteJSON != c.BeforeVal {
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

		if err := e.backend.SetDataset(ctx, c.EntityKey, toBackendDataset(mine)); err != nil {
			result.Failed = append(result.Failed, FailedCommit{
				TestKey: c.EntityKey,
				Error:   fmt.Sprintf("write the dataset of %s: %s", c.EntityKey, sanitizeError(err.Error())),
			})
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

// toBackendDataset / fromBackendDataset map between the stored table and the
// neutral one the backend takes. Two near-identical structs are the price of
// testrepo not importing backend.
func toBackendDataset(ds testrepo.Dataset) backend.Dataset {
	params := make([]backend.DatasetParameter, len(ds.Parameters))
	for i, p := range ds.Parameters {
		params[i] = backend.DatasetParameter{Name: p.Name, Combinatorial: p.Combinatorial}
	}
	return backend.Dataset{Parameters: params, Rows: ds.Rows}
}

func fromBackendDataset(d backend.Dataset) testrepo.Dataset {
	params := make([]testrepo.DatasetParameter, len(d.Parameters))
	for i, p := range d.Parameters {
		params[i] = testrepo.DatasetParameter{Name: p.Name, Combinatorial: p.Combinatorial}
	}
	return testrepo.Dataset{Parameters: params, Rows: d.Rows}
}
