package testrepo

import (
	"fmt"
	"strings"
)

// Bug is a cached defect issue (possibly in another project) linked to Tests.
type Bug struct {
	Key        string `json:"key"`
	ProjectKey string `json:"projectKey"`
	IssueType  string `json:"issueType"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	Updated    string `json:"updated"`
}

// BugLink is a Test <-> Bug link.
type BugLink struct {
	TestKey string
	BugKey  string
	LinkID  string
}

// BugWithTests is a bug plus the Test keys it affects, for the Bugs panel.
type BugWithTests struct {
	Key        string   `json:"key"`
	ProjectKey string   `json:"projectKey"`
	Summary    string   `json:"summary"`
	Status     string   `json:"status"`
	Priority   string   `json:"priority"`
	TestKeys   []string `json:"testKeys"`
}

// TestBug is a bug linked to one Test, for the test-detail section.
type TestBug struct {
	Key        string `json:"key"`
	ProjectKey string `json:"projectKey"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
}

// BugDraft is the payload for creating a new bug from a failed test.
type BugDraft struct {
	ProjectKey  string   `json:"projectKey"`
	IssueType   string   `json:"issueType"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
}

// bugLinkSnap mirrors reqLinkSnap: a Test link snapshot for discard.
type bugLinkSnap struct {
	Key    string `json:"key"`
	LinkID string `json:"linkId"`
}

// ProfileBugIssueType returns the profile's configured defect issuetype,
// defaulting to "Bug". It reads the profiles table directly (same store) so the
// syncer can recognize the right defect type without depending on the profile
// manager.
func (r *Repository) ProfileBugIssueType(profileID string) string {
	var t string
	err := r.db.QueryRow(`SELECT bug_issue_type FROM profiles WHERE id = ?`, profileID).Scan(&t)
	if err != nil || strings.TrimSpace(t) == "" {
		return "Bug"
	}
	return t
}

// ReplaceAllBugs reconciles the cached bug issues for a profile (full replace on
// sync). Mirrors ReplaceAllRequirements.
func (r *Repository) ReplaceAllBugs(profileID string, bugs []Bug) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM bug WHERE profile_id = ?`, profileID); err != nil {
		return fmt.Errorf("clear bugs: %w", err)
	}
	for _, b := range bugs {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO bug (profile_id, jira_key, project_key, issue_type, summary, status, priority, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, b.Key, b.ProjectKey, b.IssueType, b.Summary, b.Status, b.Priority, b.Updated,
		); err != nil {
			return fmt.Errorf("insert bug: %w", err)
		}
	}
	return tx.Commit()
}

// ReplaceAllBugLinks reconciles the Test<->Bug links for a profile (full replace
// on sync). Mirrors ReplaceAllRequirementLinks.
func (r *Repository) ReplaceAllBugLinks(profileID string, links []BugLink) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM test_bug WHERE profile_id = ?`, profileID); err != nil {
		return fmt.Errorf("clear bug links: %w", err)
	}
	for _, l := range links {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO test_bug (profile_id, test_key, bug_key, link_id)
			 VALUES (?, ?, ?, ?)`,
			profileID, l.TestKey, l.BugKey, l.LinkID,
		); err != nil {
			return fmt.Errorf("insert bug link: %w", err)
		}
	}
	return tx.Commit()
}

// GetTestBugs returns the bugs linked to a Test (for the detail section),
// ordered by key.
func (r *Repository) GetTestBugs(profileID, testKey string) ([]TestBug, error) {
	rows, err := r.db.Query(
		`SELECT b.jira_key, b.project_key, b.summary, b.status, b.priority
		 FROM test_bug l
		 JOIN bug b ON b.profile_id = l.profile_id AND b.jira_key = l.bug_key
		 WHERE l.profile_id = ? AND l.test_key = ?
		 ORDER BY b.jira_key`, profileID, testKey)
	if err != nil {
		return nil, fmt.Errorf("get test bugs: %w", err)
	}
	defer rows.Close()
	out := []TestBug{}
	for rows.Next() {
		var b TestBug
		if err := rows.Scan(&b.Key, &b.ProjectKey, &b.Summary, &b.Status, &b.Priority); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListBugsWithTests returns every cached bug with the Test keys it affects, for
// the Bugs panel. Ordered by project then key.
func (r *Repository) ListBugsWithTests(profileID string) ([]BugWithTests, error) {
	rows, err := r.db.Query(
		`SELECT jira_key, project_key, summary, status, priority
		 FROM bug WHERE profile_id = ? ORDER BY project_key, jira_key`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list bugs: %w", err)
	}
	defer rows.Close()
	out := []BugWithTests{}
	idx := map[string]int{}
	for rows.Next() {
		var b BugWithTests
		if err := rows.Scan(&b.Key, &b.ProjectKey, &b.Summary, &b.Status, &b.Priority); err != nil {
			return nil, err
		}
		b.TestKeys = []string{}
		idx[b.Key] = len(out)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	lrows, err := r.db.Query(
		`SELECT bug_key, test_key FROM test_bug WHERE profile_id = ? ORDER BY test_key`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list bug links: %w", err)
	}
	defer lrows.Close()
	for lrows.Next() {
		var bugKey, testKey string
		if err := lrows.Scan(&bugKey, &testKey); err != nil {
			return nil, err
		}
		if i, ok := idx[bugKey]; ok {
			out[i].TestKeys = append(out[i].TestKeys, testKey)
		}
	}
	return out, lrows.Err()
}
