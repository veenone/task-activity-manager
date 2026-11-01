// Committing a step result on a plain execution (#371 Phase 6).
//
// It is commit_iteration.go's shape one level flatter: read the remote tree,
// compare the targeted stable id against before_val, and treat a changed
// remote as a conflict, an already-applied value as satisfied, and a backend
// refusal as this row's failure alone.
//
// The methods are on backend.RunStepResultManager rather than on Backend,
// because only a backend whose step results carry their own ids can answer
// them at all. The capability is checked first and the assertion is the
// backstop, which is how every optional capability in this package is reached.

package syncer

import (
	"context"
	"strings"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

const runStepConflictLabel = "Step result"

// commitRunSteps pushes the journalled step results of a plain execution.
func (e *Engine) commitRunSteps(ctx context.Context, profileID string, rows []testrepo.PendingChange, result *CommitResult) {
	manager, ok := e.backend.(backend.RunStepResultManager)
	if !ok {
		// The capability said yes and the backend does not implement the
		// interface, which is a wiring mistake rather than something the user
		// did. Reported like any other unwritable row so the change is kept.
		e.skipRows(rows, skipReasonRunStepWrites, result)
		return
	}
	for _, change := range rows {
		identity, ok := splitRunStepEntityKey(change.EntityKey)
		if !ok {
			result.Failed = append(result.Failed, FailedCommit{TestKey: change.EntityKey, Error: "malformed step result key"})
			continue
		}
		execKey, testKey, stepResultID := identity[0], identity[1], identity[2]

		remoteSteps, err := manager.GetRunStepResults(ctx, execKey, testKey)
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: testKey, Error: "read the step results before writing: " + sanitizeError(err.Error())})
			continue
		}
		remote, found := runStepRemoteStatus(remoteSteps, stepResultID)
		if !found {
			// The step was deleted from the test, or the execution was
			// recreated. Either way the id addresses nothing and no local
			// action can change that, so the row is reported and kept rather
			// than failed on every Commit (#364).
			result.Skipped = append(result.Skipped, SkippedCommit{
				EntityKey:  change.EntityKey,
				EntityType: change.EntityType,
				Reason:     skipReasonRunStepGone,
			})
			continue
		}
		if remote != change.BeforeVal && remote != change.AfterVal {
			result.Conflicted = append(result.Conflicted, Conflict{
				TestKey: testKey,
				Fields: []ConflictField{{
					PendingID: change.ID, EntityType: change.EntityType, EntityKey: change.EntityKey,
					Field: change.Field, Label: runStepConflictLabel,
					Base: change.BeforeVal, Remote: remote, Mine: change.AfterVal,
				}},
			})
			continue
		}

		if remote != change.AfterVal {
			if err := manager.SetRunStepStatus(ctx, execKey, testKey, stepResultID, change.AfterVal); err != nil {
				result.Failed = append(result.Failed, FailedCommit{TestKey: testKey, Error: "set the step result: " + sanitizeError(err.Error())})
				continue
			}
		}
		if err := e.repo.CommitPendingChanges(profileID, []int64{change.ID}); err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: testKey, Error: "the backend accepted the step result but local cleanup failed: " + err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, testKey)
	}
}

// splitRunStepEntityKey parses the key testrepo builds: the execution, the
// test and the step result's own id. An empty part means a malformed key,
// which would otherwise write by a blank identifier.
func splitRunStepEntityKey(entityKey string) ([]string, bool) {
	parts := strings.Split(entityKey, ":")
	if len(parts) != 3 {
		return nil, false
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, false
		}
	}
	return parts, true
}

func runStepRemoteStatus(steps []backend.RunStepResult, stepResultID string) (string, bool) {
	for _, step := range steps {
		if step.ID == stepResultID {
			return step.Status, true
		}
	}
	return "", false
}
