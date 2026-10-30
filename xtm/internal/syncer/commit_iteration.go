package syncer

import (
	"context"
	"errors"
	"strings"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

const iterationConflictLabel = "Dataset iteration result"

// commitIterations checks the current result by stable server id before it
// writes. A changed remote result is held as a conflict, while one already at
// the requested value is treated as satisfied and only clears the local row.
func (e *Engine) commitIterations(ctx context.Context, profileID string, rows []testrepo.PendingChange, caps backend.Capabilities, result *CommitResult) {
	for _, change := range rows {
		identity, ok := parseIterationEntityKey(change)
		if !ok {
			result.Failed = append(result.Failed, FailedCommit{TestKey: change.EntityKey, Error: "malformed parameterized result key"})
			continue
		}

		// A step result the backend cannot address is skipped before anything is
		// compared. Its remote side is the iteration's own status, which matches
		// neither end of the edit whenever the iteration moved, so the conflict
		// check below would hold the Test on a value no resolution can change
		// and the same conflict would return on every Commit (#369).
		if identity.stepResultID != "" && !caps.SupportsParameterizedRunStepWrites {
			result.Skipped = append(result.Skipped, SkippedCommit{
				EntityKey:  change.EntityKey,
				EntityType: change.EntityType,
				Reason:     skipReasonIterationStepWrites,
			})
			continue
		}

		run, err := e.backend.GetParameterizedRun(ctx, identity.execKey, identity.testKey)
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: identity.testKey, Error: "read parameterized run before writing: " + sanitizeError(err.Error())})
			continue
		}
		remote, found := iterationRemoteStatus(run, identity)
		if !found {
			result.Failed = append(result.Failed, FailedCommit{TestKey: identity.testKey, Error: "parameterized result no longer exists on the backend"})
			continue
		}
		if remote != change.BeforeVal && remote != change.AfterVal {
			result.Conflicted = append(result.Conflicted, Conflict{
				TestKey: identity.testKey,
				Fields: []ConflictField{{
					PendingID: change.ID, EntityType: change.EntityType, EntityKey: change.EntityKey,
					Field: change.Field, Label: iterationConflictLabel,
					Base: change.BeforeVal, Remote: remote, Mine: change.AfterVal,
				}},
			})
			continue
		}

		if remote != change.AfterVal {
			if identity.stepResultID == "" {
				err = e.backend.SetIterationStatus(ctx, identity.execKey, identity.testKey, identity.iterationID, change.AfterVal)
			} else {
				err = e.backend.SetIterationStepStatus(ctx, identity.execKey, identity.testKey, identity.iterationID, identity.stepResultID, change.AfterVal)
			}
			// A step result the backend cannot express is reported and left
			// pending, not failed: nothing the user does will make it land, so
			// an error every Commit would be noise rather than news (#364).
			if errors.Is(err, backend.ErrUnsupported) && identity.stepResultID != "" {
				result.Skipped = append(result.Skipped, SkippedCommit{
					EntityKey:  change.EntityKey,
					EntityType: change.EntityType,
					Reason:     skipReasonIterationStepWrites,
				})
				continue
			}
			if err != nil {
				result.Failed = append(result.Failed, FailedCommit{TestKey: identity.testKey, Error: "set parameterized result: " + sanitizeError(err.Error())})
				continue
			}
		}

		if err := e.repo.CommitPendingChanges(profileID, []int64{change.ID}); err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: identity.testKey, Error: "backend accepted the parameterized result but local cleanup failed: " + err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, identity.testKey)
	}
}

type iterationIdentity struct {
	execKey, testKey, iterationID, stepResultID string
}

func parseIterationEntityKey(change testrepo.PendingChange) (iterationIdentity, bool) {
	parts := strings.Split(change.EntityKey, ":")
	want := 3
	if change.EntityType == "test_run_iteration_step" {
		want = 4
	} else if change.EntityType != "test_run_iteration" {
		return iterationIdentity{}, false
	}
	if len(parts) != want {
		return iterationIdentity{}, false
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return iterationIdentity{}, false
		}
	}
	identity := iterationIdentity{execKey: parts[0], testKey: parts[1], iterationID: parts[2]}
	if want == 4 {
		identity.stepResultID = parts[3]
	}
	return identity, true
}

func iterationRemoteStatus(run backend.ParameterizedRun, identity iterationIdentity) (string, bool) {
	if run.ExecutionKey != identity.execKey || run.TestKey != identity.testKey {
		return "", false
	}
	for _, iteration := range run.Iterations {
		if iteration.ID != identity.iterationID {
			continue
		}
		if identity.stepResultID == "" {
			return iteration.Status, true
		}
		for _, step := range iteration.Steps {
			if step.ID == identity.stepResultID {
				return step.Status, true
			}
		}
		return "", false
	}
	return "", false
}
