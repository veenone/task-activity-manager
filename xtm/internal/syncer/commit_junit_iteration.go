package syncer

import (
	"context"
	"encoding/json"
	"strings"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

const importedIterationConflictLabel = "Imported dataset iteration result"

func (e *Engine) commitDeferredJUnitIterations(ctx context.Context, profileID string, rows []testrepo.PendingChange, result *CommitResult) {
	for _, change := range rows {
		execKey, testKey, ok := parseDeferredIterationKey(change)
		if !ok {
			result.Failed = append(result.Failed, FailedCommit{TestKey: change.EntityKey, Error: "malformed deferred dataset iteration key"})
			continue
		}

		var desired testrepo.DeferredIterationResult
		if err := json.Unmarshal([]byte(change.AfterVal), &desired); err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: testKey, Error: "malformed deferred dataset iteration result"})
			continue
		}
		if desired.TestKey != testKey || len(desired.Parameters) == 0 || desired.Result == "" {
			result.Failed = append(result.Failed, FailedCommit{TestKey: testKey, Error: "invalid deferred dataset iteration result"})
			continue
		}

		run, err := e.backend.GetParameterizedRun(ctx, execKey, testKey)
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: testKey, Error: "read imported parameterized run: " + sanitizeError(err.Error())})
			continue
		}
		iterationID, matches := matchingIterationID(run.Iterations, desired.Parameters)
		if run.DatasetSourceKind != desired.DatasetSourceKind || run.DatasetSourceKey != desired.DatasetSourceKey || matches != 1 {
			remote, _ := json.Marshal(run)
			result.Conflicted = append(result.Conflicted, Conflict{
				TestKey: testKey,
				Fields: []ConflictField{{
					PendingID: change.ID, EntityType: change.EntityType, EntityKey: change.EntityKey,
					Field: change.Field, Label: importedIterationConflictLabel,
					Base: change.BeforeVal, Remote: string(remote), Mine: change.AfterVal,
				}},
			})
			continue
		}

		if err := e.backend.SetIterationStatus(ctx, execKey, testKey, iterationID, desired.Result); err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: testKey, Error: "set imported dataset iteration result: " + sanitizeError(err.Error())})
			continue
		}
		if err := e.repo.CommitPendingChanges(profileID, []int64{change.ID}); err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: testKey, Error: "backend accepted the imported dataset result but local cleanup failed: " + err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, testKey)
	}
}

func parseDeferredIterationKey(change testrepo.PendingChange) (string, string, bool) {
	if change.EntityType != "test_run_iteration_import" {
		return "", "", false
	}
	parts := strings.Split(change.EntityKey, ":")
	if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func matchingIterationID(iterations []backend.RunIteration, parameters []backend.IterationParameter) (string, int) {
	id := ""
	matches := 0
	for _, iteration := range iterations {
		if !sameImportedParameters(iteration.Parameters, parameters) {
			continue
		}
		id = iteration.ID
		matches++
	}
	return id, matches
}

func sameImportedParameters(left, right []backend.IterationParameter) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
