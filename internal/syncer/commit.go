package syncer

import (
	"context"
	"strings"

	"xray-test-manager/internal/jira"
	"xray-test-manager/internal/testrepo"
)

// CommitResult reports the outcome of pushing pending changes to Jira,
// per-Test. The Succeeded and Failed slices are disjoint sets of Test keys.
type CommitResult struct {
	Succeeded []string       `json:"succeeded"`
	Failed    []FailedCommit `json:"failed"`
}

// FailedCommit is one Test whose pending changes could not be committed.
type FailedCommit struct {
	TestKey string `json:"testKey"`
	Error   string `json:"error"`
}

// CommitChanges pushes a profile's pending changes to Jira (FR-1.5). Changes
// for the same Test are batched into a single PUT. Successful PUTs delete
// the local pending rows and append "commit" audit entries; failures leave
// the rows in place and are reported per-Test.
//
// TODO(xtm): conflict detection (FR-1.4) — re-fetch each Test's current
// `updated` and compare against base_version before PUT.
func (e *Engine) CommitChanges(ctx context.Context, profileID string) (CommitResult, error) {
	result := CommitResult{
		Succeeded: []string{},
		Failed:    []FailedCommit{},
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

// sanitizeError trims long Jira error responses so the UI shows a short,
// single-line message in the per-Test failure list.
func sanitizeError(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
