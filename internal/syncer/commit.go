package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"xray-test-manager/internal/jira"
	"xray-test-manager/internal/testrepo"
)

// CommitResult reports the outcome of pushing pending changes to Jira,
// per-Test. Succeeded, Conflicted and Failed are disjoint sets of Test keys.
type CommitResult struct {
	Succeeded  []string       `json:"succeeded"`
	Conflicted []Conflict     `json:"conflicted"`
	Failed     []FailedCommit `json:"failed"`
}

// Conflict means the remote `updated` has moved since the user's earliest
// pending edit for this Test (FR-1.4). The PUT is held back so the user can
// resolve — sync to pull in the remote change and either re-commit or
// discard.
type Conflict struct {
	TestKey       string `json:"testKey"`
	BaseVersion   string `json:"baseVersion"`
	RemoteVersion string `json:"remoteVersion"`
}

// FailedCommit is one Test whose pending changes could not be committed
// for a non-conflict reason (network error, Jira validation, missing
// transition, etc.).
type FailedCommit struct {
	TestKey string `json:"testKey"`
	Error   string `json:"error"`
}

// CommitChanges pushes a profile's pending changes to Jira (FR-1.5). For
// each Test:
//
//  1. Fetch the current remote `updated` (FR-1.4 conflict pre-check). If
//     the remote has moved since the oldest pending edit's base_version,
//     skip the Test and surface a conflict.
//  2. PUT any non-status field updates.
//  3. POST a workflow transition (FR-4.2) if a status change is queued.
//  4. DELETE removed steps, then PUT step field updates, then POST new
//     steps (FR-2.5).
//  5. Delete the pending rows and audit "commit".
//
// Failures and conflicts leave pending rows in place so the user can
// resolve and retry.
func (e *Engine) CommitChanges(ctx context.Context, profileID string) (CommitResult, error) {
	result := CommitResult{
		Succeeded:  []string{},
		Conflicted: []Conflict{},
		Failed:     []FailedCommit{},
	}

	changes, err := e.repo.ListPendingChanges(profileID)
	if err != nil {
		return result, err
	}

	// Group by parent Test key, preserving discovery order so the commit
	// run is deterministic. Step entity_keys are "<testKey>:<xrayID>" — we
	// strip the suffix to put step changes under the same Test bucket as
	// field changes on that Test.
	byTest := make(map[string][]testrepo.PendingChange)
	order := make([]string, 0)
	for _, c := range changes {
		testKey, ok := parentTestKey(c)
		if !ok {
			continue
		}
		if _, seen := byTest[testKey]; !seen {
			order = append(order, testKey)
		}
		byTest[testKey] = append(byTest[testKey], c)
	}

testLoop:
	for _, testKey := range order {
		testChanges := byTest[testKey]

		// Conflict pre-check.
		remoteUpdated, err := e.client.GetIssueUpdated(ctx, testKey)
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{
				TestKey: testKey,
				Error:   "conflict pre-check failed: " + sanitizeError(err.Error()),
			})
			continue
		}
		if remoteUpdated != "" {
			oldest := oldestBaseVersion(testChanges)
			if oldest != "" && isRemoteAhead(remoteUpdated, oldest) {
				result.Conflicted = append(result.Conflicted, Conflict{
					TestKey:       testKey,
					BaseVersion:   oldest,
					RemoteVersion: remoteUpdated,
				})
				continue
			}
		}

		// Split the bucket into:
		//   - one status transition (at most)
		//   - test_case field updates (summary, description, priority, labels)
		//   - per-step field updates, keyed by xrayID
		//   - step deletions, keyed by xrayID
		var statusChange *testrepo.PendingChange
		fieldChanges := make([]testrepo.PendingChange, 0, len(testChanges))
		stepChanges := make(map[string][]testrepo.PendingChange)
		stepDeletes := make([]string, 0)
		stepAdds := make([]testrepo.Step, 0)
		for i := range testChanges {
			c := testChanges[i]
			switch c.EntityType {
			case "test_case":
				if c.Field == "status" {
					cc := c
					statusChange = &cc
				} else {
					fieldChanges = append(fieldChanges, c)
				}
			case "test_step":
				_, xrayID, ok := parseStepKey(c.EntityKey)
				if !ok {
					result.Failed = append(result.Failed, FailedCommit{
						TestKey: testKey,
						Error:   fmt.Sprintf("malformed step entity_key %q", c.EntityKey),
					})
					continue testLoop
				}
				stepChanges[xrayID] = append(stepChanges[xrayID], c)
			case "test_step_delete":
				_, xrayID, ok := parseStepKey(c.EntityKey)
				if !ok {
					result.Failed = append(result.Failed, FailedCommit{
						TestKey: testKey,
						Error:   fmt.Sprintf("malformed step entity_key %q", c.EntityKey),
					})
					continue testLoop
				}
				stepDeletes = append(stepDeletes, xrayID)
			case "test_step_add":
				_, tempID, ok := parseStepKey(c.EntityKey)
				if !ok {
					result.Failed = append(result.Failed, FailedCommit{
						TestKey: testKey,
						Error:   fmt.Sprintf("malformed step entity_key %q", c.EntityKey),
					})
					continue testLoop
				}
				var s testrepo.Step
				if err := json.Unmarshal([]byte(c.AfterVal), &s); err != nil {
					result.Failed = append(result.Failed, FailedCommit{
						TestKey: testKey,
						Error:   fmt.Sprintf("malformed step_add payload for %q: %s", c.EntityKey, err),
					})
					continue testLoop
				}
				s.XrayID = tempID
				stepAdds = append(stepAdds, s)
			}
		}

		// PUT non-status field updates.
		if len(fieldChanges) > 0 {
			updates := make(map[string]string, len(fieldChanges))
			for _, c := range fieldChanges {
				updates[c.Field] = c.AfterVal
			}
			if err := e.client.UpdateIssue(ctx, testKey, jira.FieldsForJira(updates)); err != nil {
				result.Failed = append(result.Failed, FailedCommit{
					TestKey: testKey,
					Error:   sanitizeError(err.Error()),
				})
				continue
			}
		}

		// POST workflow transition if a status change is pending.
		if statusChange != nil {
			if err := e.applyTransition(ctx, testKey, statusChange); err != nil {
				result.Failed = append(result.Failed, FailedCommit{
					TestKey: testKey,
					Error:   err.Error(),
				})
				continue
			}
		}

		// DELETE removed steps first — Xray validates the rest of the
		// commit against whatever steps remain, so removing before edits
		// avoids touching a step a parallel commit might also be removing.
		for _, xrayID := range stepDeletes {
			if err := e.client.DeleteTestStep(ctx, testKey, xrayID); err != nil {
				result.Failed = append(result.Failed, FailedCommit{
					TestKey: testKey,
					Error:   fmt.Sprintf("delete step %s: %s", xrayID, sanitizeError(err.Error())),
				})
				continue testLoop
			}
		}

		// PUT each step that has pending edits, batching the step's changes
		// into one body. The first step to fail aborts further step PUTs
		// for this Test — the user resolves and retries.
		for xrayID, changes := range stepChanges {
			fields := make(map[string]string, len(changes))
			for _, c := range changes {
				fields[c.Field] = c.AfterVal
			}
			if err := e.client.UpdateTestStep(ctx, testKey, xrayID, fields); err != nil {
				result.Failed = append(result.Failed, FailedCommit{
					TestKey: testKey,
					Error:   fmt.Sprintf("update step %s: %s", xrayID, sanitizeError(err.Error())),
				})
				continue testLoop
			}
		}

		// POST new steps last, in index order — Xray appends each created
		// step to the end of the list, so creating them ascending preserves
		// the order the user arranged. On success we rename the local "new-N"
		// placeholder to the real id Xray returned.
		sort.SliceStable(stepAdds, func(i, j int) bool {
			return stepAdds[i].Index < stepAdds[j].Index
		})
		for _, s := range stepAdds {
			newID, err := e.client.CreateTestStep(ctx, testKey, s.Action, s.Data, s.Expected)
			if err != nil {
				result.Failed = append(result.Failed, FailedCommit{
					TestKey: testKey,
					Error:   fmt.Sprintf("add step: %s", sanitizeError(err.Error())),
				})
				continue testLoop
			}
			if err := e.repo.RenameTestStepID(profileID, testKey, s.XrayID, newID); err != nil {
				// The remote create already succeeded; a cache-rename hiccup
				// must not fail the commit. The stale placeholder reconciles
				// on the next steps refresh.
				continue
			}
		}

		ids := make([]int64, len(testChanges))
		for i, c := range testChanges {
			ids[i] = c.ID
		}
		if err := e.repo.CommitPendingChanges(profileID, ids); err != nil {
			result.Failed = append(result.Failed, FailedCommit{
				TestKey: testKey,
				Error:   "Jira accepted update but local cleanup failed: " + err.Error(),
			})
			continue
		}
		result.Succeeded = append(result.Succeeded, testKey)
	}

	return result, nil
}

// parentTestKey extracts the parent Test key for a pending change so
// CommitChanges can group test_case and test_step changes together per
// Test. Returns false for unrecognised entity types.
func parentTestKey(c testrepo.PendingChange) (string, bool) {
	switch c.EntityType {
	case "test_case":
		return c.EntityKey, true
	case "test_step", "test_step_delete", "test_step_add":
		k, _, ok := parseStepKey(c.EntityKey)
		return k, ok
	}
	return "", false
}

// parseStepKey splits a "<testKey>:<xrayID>" pending entity_key. Mirrors
// testrepo.parseStepEntityKey but lives here too so the syncer doesn't
// depend on an exported helper for a one-line split.
func parseStepKey(s string) (testKey, xrayID string, ok bool) {
	i := strings.Index(s, ":")
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// applyTransition resolves the transition ID by target status name and POSTs
// it. The current Jira status is the pending change's BeforeVal — that's
// what Jira holds until our commit lands.
func (e *Engine) applyTransition(ctx context.Context, testKey string, change *testrepo.PendingChange) error {
	transitions, err := e.client.GetTransitions(ctx, testKey, change.BeforeVal)
	if err != nil {
		return fmt.Errorf("fetch transitions: %s", sanitizeError(err.Error()))
	}
	var transitionID string
	for _, t := range transitions {
		if t.To == change.AfterVal {
			transitionID = t.ID
			break
		}
	}
	if transitionID == "" {
		return fmt.Errorf(
			"no transition available to status %q from %q",
			change.AfterVal, change.BeforeVal,
		)
	}
	if err := e.client.PostTransition(ctx, testKey, transitionID); err != nil {
		return fmt.Errorf("post transition: %s", sanitizeError(err.Error()))
	}
	return nil
}

// oldestBaseVersion returns the earliest base_version among a Test's pending
// changes, ignoring empty values. The oldest is used for the conflict
// pre-check so any field that was edited before a remote update triggers a
// conflict on the whole Test.
func oldestBaseVersion(changes []testrepo.PendingChange) string {
	oldest := ""
	for _, c := range changes {
		if c.BaseVersion == "" {
			continue
		}
		if oldest == "" || c.BaseVersion < oldest {
			oldest = c.BaseVersion
		}
	}
	return oldest
}

// isRemoteAhead returns true if remote's timestamp is strictly later than
// base's. Both arguments are timestamps as Jira returns them — typically
// "yyyy-MM-ddTHH:mm:ss.SSS-HHMM" but RFC 3339 variants are also accepted.
// On parse failure the function is permissive (returns false) so a malformed
// remote string can't manufacture a phantom conflict.
func isRemoteAhead(remote, base string) bool {
	rt, ok1 := parseJiraTime(remote)
	bt, ok2 := parseJiraTime(base)
	if !ok1 || !ok2 {
		return false
	}
	return rt.After(bt)
}

var jiraTimeFormats = []string{
	"2006-01-02T15:04:05.000-0700",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05.000Z07:00",
	time.RFC3339Nano,
	time.RFC3339,
}

func parseJiraTime(s string) (time.Time, bool) {
	for _, f := range jiraTimeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// sanitizeError trims long Jira error responses so the UI shows a short,
// single-line message in the per-Test failure list.
func sanitizeError(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
