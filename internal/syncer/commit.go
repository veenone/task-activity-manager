package syncer

import (
	"context"
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
// for a non-conflict reason (network error, Jira validation, etc).
type FailedCommit struct {
	TestKey string `json:"testKey"`
	Error   string `json:"error"`
}

// CommitChanges pushes a profile's pending changes to Jira (FR-1.5). For
// each Test:
//
//  1. Fetch the current remote `updated` (FR-1.4 conflict pre-check).
//  2. If the remote has moved since the oldest pending edit's base_version,
//     report a conflict and skip the PUT.
//  3. Otherwise PUT the batched field updates and delete the pending rows,
//     writing a "commit" audit entry for each.
//
// Failures and conflicts both leave pending rows in place so the user can
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

	// Group by Test, preserving the order pending changes were returned in
	// so the commit run is deterministic.
	byTest := make(map[string][]testrepo.PendingChange)
	order := make([]string, 0)
	for _, c := range changes {
		if c.EntityType != "test_case" {
			continue
		}
		if _, seen := byTest[c.EntityKey]; !seen {
			order = append(order, c.EntityKey)
		}
		byTest[c.EntityKey] = append(byTest[c.EntityKey], c)
	}

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

		updates := make(map[string]string, len(testChanges))
		for _, c := range testChanges {
			updates[c.Field] = c.AfterVal
		}
		if err := e.client.UpdateIssue(ctx, testKey, jira.FieldsForJira(updates)); err != nil {
			result.Failed = append(result.Failed, FailedCommit{
				TestKey: testKey,
				Error:   sanitizeError(err.Error()),
			})
			continue
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
