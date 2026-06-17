# Defect (Bug) Tracking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users file a Bug-type Jira issue from a failed test in a Test Execution (local-first, committed on sync), list every bug linked to the profile's tests in a Bugs panel inside the Containers view, and show a test's linked bugs (incl. cross-project) as browser hyperlinks on the test detail.

**Architecture:** This feature is a near-exact mirror of the existing **requirements** feature (cross-project Jira issues, cached locally in twin tables, discovered via `issuelinks`, shown on test detail + a dedicated list). Reuse those proven patterns: where a mechanical twin exists, the steps say "copy `requirements.go:X-Y`, rename `requirement`→`bug`"; novel logic (create-from-failed-test, the commit pass, demo data, the three UI pieces) is given in full. Bugs are created local-first (placeholder `NEW-BUG-N` + `bug_create` pending change) and POSTed on Commit, exactly like new Tests.

**Tech Stack:** Go 1.25 (no ORM, raw SQL via `modernc.org/sqlite`), Wails v2 generated bindings, React 18 + TypeScript, Vite. **No JS test runner exists** (only Go has tests) — per-frontend-task verification is `npx tsc --noEmit`; final verification is `npm run build` + `wails build` + a demo click-through. The real Jira write/discovery calls are wired but carry `NOTE(xtm)` live-verify markers and demo-no-op, exactly as the requirements feature does today.

**Reference twins (read these first):** `internal/testrepo/requirements.go`, `internal/testrepo/requirementcrud.go`, `internal/testrepo/createtest.go` + `importcsv.go` (temp-key + `RenameTest`), `internal/jira/requirements.go`, `internal/syncer/engine.go` (`syncRequirements`), `internal/syncer/commit.go` (`commitTestCreates`, `commitRequirements`), `internal/testrepo/testrepo.go` (entity constants ~3998, `DiscardPendingChange` switch ~3328), `frontend/src/components/RequirementsView.tsx`, `frontend/src/components/TestDetail.tsx` (Requirements section + `openInJira`), `frontend/src/components/ContainersView.tsx`, `frontend/src/components/CreateBugModal`-analog `NewTestPanel.tsx`/`RequirementSourcesModal.tsx`.

---

## File Structure

- **Modify** `internal/store/store.go` — bump `schemaVersion`; add `bug` + `test_bug` tables + 2 indexes to baseSchema.
- **Create** `internal/testrepo/bugs.go` — types + read/replace methods (`ReplaceAllBugs`, `ReplaceAllBugLinks`, `ListBugsWithTests`, `GetTestBugs`).
- **Create** `internal/testrepo/bugcrud.go` — `CreateBugForTest`, `RenameBug`, the `bug_create` discard snapshot.
- **Modify** `internal/testrepo/testrepo.go` — `entityBugCreate` constant + `DiscardPendingChange` case.
- **Create** `internal/jira/bugs.go` — `ListBugs`, `CreateBug`, `CreateBugLink`, `demoBugs`.
- **Modify** `internal/syncer/engine.go` — `syncBugs` pass.
- **Modify** `internal/syncer/commit.go` — collect `bug_create` rows + `commitBugCreates`.
- **Modify** `app.go` — `CreateBugForTest`, `ListBugsWithTests`, `GetTestBugs` bindings.
- **Modify** `frontend/src/api.ts` — interfaces + binding re-exports.
- **Create** `frontend/src/components/CreateBugModal.tsx` — the create form.
- **Create** `frontend/src/components/BugsPanel.tsx` — the bugs list.
- **Modify** `frontend/src/components/TestDetail.tsx` — read-only Bugs section.
- **Modify** `frontend/src/components/ContainersView.tsx` — `[Containers | Bugs]` toggle + 🐞 action on failed rows.
- **Modify** `frontend/src/components/PendingChangesModal.tsx` — `bug_create` description.
- **Modify** `frontend/src/App.tsx` — pass `jiraUrl` to `ContainersView`.
- **Modify** `frontend/src/App.css` — styles for the new pieces.

---

## Task 1: Store schema + Go types

**Files:** Modify `internal/store/store.go`; Create `internal/testrepo/bugs.go`.

- [ ] **Step 1: Bump schemaVersion and add tables**

In `internal/store/store.go`, find `const schemaVersion = N` and increment it by 1. In the baseSchema string (the `CREATE TABLE IF NOT EXISTS` block, next to the `requirement` / `test_requirement` definitions), add:

```sql
CREATE TABLE IF NOT EXISTS bug (
    profile_id  TEXT NOT NULL,
    jira_key    TEXT NOT NULL,
    project_key TEXT NOT NULL DEFAULT '',
    issue_type  TEXT NOT NULL DEFAULT '',
    summary     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT '',
    priority    TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (profile_id, jira_key)
);

CREATE TABLE IF NOT EXISTS test_bug (
    profile_id TEXT NOT NULL,
    test_key   TEXT NOT NULL,
    bug_key    TEXT NOT NULL,
    link_id    TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (profile_id, test_key, bug_key)
);
```

In the indexSchema block (next to `idx_test_requirement_*`), add:

```sql
CREATE INDEX IF NOT EXISTS idx_test_bug_test ON test_bug(profile_id, test_key);
CREATE INDEX IF NOT EXISTS idx_test_bug_bug  ON test_bug(profile_id, bug_key);
```

- [ ] **Step 2: Create the types + read/replace methods**

Create `internal/testrepo/bugs.go`:

```go
package testrepo

import "fmt"

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
			`INSERT INTO bug (profile_id, jira_key, project_key, issue_type, summary, status, priority, updated_at)
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
```

- [ ] **Step 3: Build + commit**

Run: `cd /c/projects/xray-test-manager && go build ./...`
Expected: builds clean.

```bash
git add internal/store/store.go internal/testrepo/bugs.go
git commit -m "Bug tracking: store schema (bug, test_bug) + read/replace methods"
```

---

## Task 2: Local create / rename / discard (bugcrud.go + entity constant)

**Files:** Create `internal/testrepo/bugcrud.go`; Modify `internal/testrepo/testrepo.go`.

- [ ] **Step 1: Add the entity constant**

In `internal/testrepo/testrepo.go`, in the entity-type constant block (near `entityRequirementSet`), add:

```go
	entityBugCreate = "bug_create"
```

- [ ] **Step 2: Write bugcrud.go**

Create `internal/testrepo/bugcrud.go`. `bugCreatePayload` is the `after_val`; `bugCreateSnapshot` is the `before_val` for discard.

```go
package testrepo

import (
	"encoding/json"
	"fmt"
)

// bugCreatePayload is the after_val of a bug_create pending change: everything
// needed to POST the Bug issue and link it to the test on commit.
type bugCreatePayload struct {
	ProjectKey  string   `json:"projectKey"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
	TestKey     string   `json:"testKey"`
}

// CreateBugForTest queues a brand-new local Bug (temp "NEW-BUG-N" key) linked to
// a failed Test, committed to Jira on the next sync (mirrors CreateTest). execKey
// is recorded only for the audit note. Returns the temp key.
func (r *Repository) CreateBugForTest(profileID, testKey, execKey string, d BugDraft) (string, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	tempKey, err := nextNewBugKey(tx, profileID)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(
		`INSERT INTO bug (profile_id, jira_key, project_key, issue_type, summary, status, priority, updated_at)
		 VALUES (?, ?, ?, ?, ?, '(new)', ?, '')`,
		profileID, tempKey, d.ProjectKey, "Bug", d.Summary, d.Priority,
	); err != nil {
		return "", fmt.Errorf("insert local bug: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO test_bug (profile_id, test_key, bug_key, link_id) VALUES (?, ?, ?, '')`,
		profileID, testKey, tempKey,
	); err != nil {
		return "", fmt.Errorf("insert local bug link: %w", err)
	}

	payload, _ := json.Marshal(bugCreatePayload{
		ProjectKey: d.ProjectKey, Summary: d.Summary, Description: d.Description,
		Priority: d.Priority, Labels: d.Labels, TestKey: testKey,
	})
	if err := upsertPendingChange(
		tx, profileID, entityBugCreate, tempKey, "bug", "", string(payload), "",
	); err != nil {
		return "", err
	}
	if err := writeAudit(
		tx, profileID, entityBugCreate, tempKey, "create-bug-local", "bug", "", d.Summary, execKey,
	); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit create bug: %w", err)
	}
	return tempKey, nil
}

// RenameBug repoints a cached bug + its Test links from the temporary key to the
// real key Jira assigned at commit (mirrors RenameTest, scoped to bug tables).
func (r *Repository) RenameBug(profileID, oldKey, newKey string) error {
	if newKey == "" || newKey == oldKey {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`UPDATE bug SET jira_key = ? WHERE profile_id = ? AND jira_key = ?`,
		newKey, profileID, oldKey,
	); err != nil {
		return fmt.Errorf("rename bug: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE test_bug SET bug_key = ? WHERE profile_id = ? AND bug_key = ?`,
		newKey, profileID, oldKey,
	); err != nil {
		return fmt.Errorf("rename bug link: %w", err)
	}
	return tx.Commit()
}

// nextNewBugKey allocates an unused "NEW-BUG-N" placeholder (mirrors the temp-key
// probe loop in importcsv.go's reserveTempKey, namespaced for bugs).
func nextNewBugKey(tx *sql.Tx, profileID string) (string, error) {
	for n := 1; ; n++ {
		key := fmt.Sprintf("NEW-BUG-%d", n)
		var one int
		err := tx.QueryRow(
			`SELECT 1 FROM bug WHERE profile_id = ? AND jira_key = ?`, profileID, key,
		).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return key, nil
		}
		if err != nil {
			return "", fmt.Errorf("probe temp bug key: %w", err)
		}
	}
}
```

Add the imports `"database/sql"` and `"errors"` to the import block (needed by `nextNewBugKey`).

- [ ] **Step 3: Add the discard case**

In `internal/testrepo/testrepo.go`, in the `DiscardPendingChange` switch (near `case entityTestCreate:` ~line 3328), add:

```go
	case entityBugCreate:
		// entity_key is the temporary bug key; discarding removes the
		// not-yet-created bug and its Test link.
		if _, err := tx.Exec(
			`DELETE FROM bug WHERE profile_id = ? AND jira_key = ?`,
			profileID, entityKey,
		); err != nil {
			return fmt.Errorf("remove local bug: %w", err)
		}
		if _, err := tx.Exec(
			`DELETE FROM test_bug WHERE profile_id = ? AND bug_key = ?`,
			profileID, entityKey,
		); err != nil {
			return fmt.Errorf("remove local bug link: %w", err)
		}
```

- [ ] **Step 4: Write tests**

Create `internal/testrepo/bugcrud_test.go` (mirror `requirementcrud_test.go` / `createtest_test.go` test scaffolding — use the same `newRepo(t)` + `UpsertTests` helpers):

```go
package testrepo_test

import (
	"testing"

	"xray-test-manager/internal/testrepo"
)

func TestCreateBugForTestQueuesAndDiscardRestores(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", ID: "1", Summary: "Login"}}); err != nil {
		t.Fatalf("seed test: %v", err)
	}

	key, err := repo.CreateBugForTest("p1", "QA-1", "QA-TE-1", testrepo.BugDraft{
		ProjectKey: "QA", Summary: "Login crashes", Description: "...", Priority: "High",
		Labels: []string{"regression"},
	})
	if err != nil {
		t.Fatalf("create bug: %v", err)
	}
	if len(key) < 8 || key[:8] != "NEW-BUG-" {
		t.Fatalf("temp key = %q, want NEW-BUG-*", key)
	}

	bugs, _ := repo.GetTestBugs("p1", "QA-1")
	if len(bugs) != 1 || bugs[0].Key != key || bugs[0].Summary != "Login crashes" {
		t.Fatalf("GetTestBugs = %+v, want one new bug", bugs)
	}
	changes, _ := repo.ListPendingChanges("p1")
	var id int64
	var n int
	for _, c := range changes {
		if c.EntityType == "bug_create" && c.EntityKey == key {
			n++
			id = c.ID
		}
	}
	if n != 1 {
		t.Fatalf("bug_create rows = %d, want 1", n)
	}

	if err := repo.DiscardPendingChange("p1", id); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if bugs, _ := repo.GetTestBugs("p1", "QA-1"); len(bugs) != 0 {
		t.Errorf("after discard GetTestBugs = %+v, want none", bugs)
	}
}

func TestRenameBugRepointsCacheAndLinks(t *testing.T) {
	repo := newRepo(t)
	_ = repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", ID: "1"}})
	key, _ := repo.CreateBugForTest("p1", "QA-1", "QA-TE-1", testrepo.BugDraft{ProjectKey: "QA", Summary: "x"})

	if err := repo.RenameBug("p1", key, "QA-500"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	bugs, _ := repo.GetTestBugs("p1", "QA-1")
	if len(bugs) != 1 || bugs[0].Key != "QA-500" {
		t.Errorf("after rename = %+v, want QA-500", bugs)
	}
}
```

- [ ] **Step 5: Run tests + commit**

Run: `cd /c/projects/xray-test-manager && go test ./internal/testrepo/ -run 'Bug' -v`
Expected: PASS.

```bash
git add internal/testrepo/bugcrud.go internal/testrepo/bugcrud_test.go internal/testrepo/testrepo.go
git commit -m "Bug tracking: local create/rename + bug_create discard"
```

---

## Task 3: Jira client (demo + real-path stubs)

**Files:** Create `internal/jira/bugs.go`.

- [ ] **Step 1: Write bugs.go**

Create `internal/jira/bugs.go`. Mirror `internal/jira/requirements.go` for structure (`isDemoURL` short-circuit, the `NOTE(xtm)` real-path markers). `demoBugs` must generate cross-project bugs (a separate `BUGS` project) linked to a few demo test keys so the panel + detail have data.

```go
package jira

import (
	"context"
	"fmt"
)

// Bug is a defect issue (possibly cross-project) linked to Tests.
type Bug struct {
	Key        string
	ProjectKey string
	IssueType  string
	Summary    string
	Status     string
	Priority   string
	Updated    string
}

// BugLink is a Test <-> Bug link.
type BugLink struct {
	TestKey string
	BugKey  string
	LinkID  string
}

// ListBugs returns the defect issues linked to the given Tests, plus the links.
// Demo URLs generate a deterministic cross-project set; the real path is empty
// until verified on a live instance.
//
// TODO(xtm): real path — read each synced Test's issuelinks (already fetched
// during the test sync) and keep links whose target issuetype is in
// {"Bug","Defect"}; batch-fetch those issues by key via
// /rest/api/2/search?jql=key in (...) so cross-project bugs resolve. Verify the
// link direction and issuetype names on a live Xray Server 8.4.0 instance.
func (c *Client) ListBugs(ctx context.Context, testProjectKey string, testKeys []string, onProgress func(done, total int)) ([]Bug, []BugLink, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		bugs, links := demoBugs(testProjectKey)
		if onProgress != nil {
			onProgress(len(bugs), len(bugs))
		}
		return bugs, links, nil
	}
	_ = testKeys
	return []Bug{}, []BugLink{}, nil
}

// CreateBug creates a Bug-type issue and returns its key. Demo URLs return a
// synthetic key.
//
// Maps to POST /rest/api/2/issue with fields {project, issuetype:{name:"Bug"},
// summary, description, priority, labels}. NOTE(xtm): verify the project's Bug
// issuetype + required fields on a live instance.
func (c *Client) CreateBug(ctx context.Context, projectKey, summary, description, priority string, labels []string) (string, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return fmt.Sprintf("%s-BUG-DEMO", projectKey), nil
	}
	_ = summary
	_ = description
	_ = priority
	_ = labels
	return "", fmt.Errorf("creating bugs on a live Jira instance is not yet verified")
}

// CreateBugLink links a Test to a Bug. Demo URLs no-op.
//
// Maps to POST /rest/api/2/issueLink. NOTE(xtm): resolve the defect link type
// once and verify direction on a live instance (same open item as requirement
// links).
func (c *Client) CreateBugLink(ctx context.Context, testKey, bugKey string) error {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return nil
	}
	_ = testKey
	_ = bugKey
	return nil
}

const demoBugProject = "BUGS"

var demoBugStatuses = []string{"Open", "In Progress", "Reopened", "Done"}
var demoBugPriorities = []string{"High", "Medium", "Critical", "Low"}

// demoBugs generates a handful of Bug issues in a separate project, linked to the
// demo profile's lower-numbered tests (which the demo marks FAILED), with one
// bug affecting two tests — so the panel and detail section have cross-project
// data.
func demoBugs(testProjectKey string) ([]Bug, []BugLink) {
	if testProjectKey == "" {
		testProjectKey = "DEMO"
	}
	const count = 6
	bugs := make([]Bug, 0, count)
	links := make([]BugLink, 0, count+1)
	for i := 1; i <= count; i++ {
		key := fmt.Sprintf("%s-%d", demoBugProject, i)
		bugs = append(bugs, Bug{
			Key:        key,
			ProjectKey: demoBugProject,
			IssueType:  "Bug",
			Summary:    fmt.Sprintf("Defect found in %s-%d", testProjectKey, i),
			Status:     demoBugStatuses[i%len(demoBugStatuses)],
			Priority:   demoBugPriorities[i%len(demoBugPriorities)],
		})
		links = append(links, BugLink{
			TestKey: fmt.Sprintf("%s-%d", testProjectKey, i),
			BugKey:  key,
			LinkID:  fmt.Sprintf("bl-%d", i),
		})
	}
	// One bug affects a second test, to exercise the multi-test panel row.
	links = append(links, BugLink{
		TestKey: fmt.Sprintf("%s-7", testProjectKey),
		BugKey:  fmt.Sprintf("%s-1", demoBugProject),
		LinkID:  "bl-extra",
	})
	return bugs, links
}
```

- [ ] **Step 2: Build + test + commit**

Run: `cd /c/projects/xray-test-manager && go build ./... && go test ./internal/jira/ -run Bug`
Expected: builds; tests pass (or no bug-specific jira test yet — build is the gate).

```bash
git add internal/jira/bugs.go
git commit -m "Bug tracking: jira client (demo ListBugs/CreateBug + real-path stubs)"
```

---

## Task 4: Sync + commit passes

**Files:** Modify `internal/syncer/engine.go`, `internal/syncer/commit.go`.

- [ ] **Step 1: Add the syncBugs pass**

In `internal/syncer/engine.go`, right after the `syncRequirements` call site (search `syncRequirements`), add a sibling best-effort call:

```go
	emitStage(onProgress, "Syncing bugs")
	if err := e.syncBugs(ctx, profileID, projectKey, onProgress); err != nil {
		log.Printf("xtm: bug sync failed (continuing): %v", err)
	}
```

Then add the method (mirror `syncRequirements`, but bugs need the synced Test keys, not requirement sources):

```go
// syncBugs discovers the defect issues linked to the profile's Tests and
// reconciles the local cache. Best-effort: failures log and continue.
func (e *Engine) syncBugs(ctx context.Context, profileID, projectKey string, onProgress func(Progress)) error {
	testKeys, err := e.repo.AllTestKeys(profileID)
	if err != nil {
		return err
	}
	bugs, links, err := e.client.ListBugs(ctx, projectKey, testKeys, nil)
	if err != nil {
		return err
	}
	repoBugs := make([]testrepo.Bug, 0, len(bugs))
	for _, b := range bugs {
		repoBugs = append(repoBugs, testrepo.Bug{
			Key: b.Key, ProjectKey: b.ProjectKey, IssueType: b.IssueType,
			Summary: b.Summary, Status: b.Status, Priority: b.Priority, Updated: b.Updated,
		})
	}
	if err := e.repo.ReplaceAllBugs(profileID, repoBugs); err != nil {
		return err
	}
	repoLinks := make([]testrepo.BugLink, 0, len(links))
	for _, l := range links {
		repoLinks = append(repoLinks, testrepo.BugLink{TestKey: l.TestKey, BugKey: l.BugKey, LinkID: l.LinkID})
	}
	return e.repo.ReplaceAllBugLinks(profileID, repoLinks)
}
```

If `e.repo.AllTestKeys` does not exist, add it to `internal/testrepo/testrepo.go` next to similar list helpers:

```go
// AllTestKeys returns every cached Test key for a profile.
func (r *Repository) AllTestKeys(profileID string) ([]string, error) {
	rows, err := r.db.Query(`SELECT jira_key FROM test_case WHERE profile_id = ?`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list test keys: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
```

(First grep `func (r \*Repository) AllTestKeys` — if it already exists, skip adding it.)

- [ ] **Step 2: Add the commit pass**

In `internal/syncer/commit.go`: (a) in the pending-row classification loop (where `requirement_set` rows are collected), add a collector:

```go
		if c.EntityType == "bug_create" {
			bugCreateRows = append(bugCreateRows, c)
			continue
		}
```

declare `bugCreateRows := make([]testrepo.PendingChange, 0)` alongside the other row slices, and call `e.commitBugCreates(ctx, profileID, bugCreateRows, &result)` next to the other commit-pass calls (e.g. after `commitRequirements`).

(b) Add the pass (mirror `commitTestCreates`):

```go
// commitBugCreates creates each queued Bug issue, repoints the placeholder key
// to the real one, then links it to its Test. Reported under the test key.
func (e *Engine) commitBugCreates(ctx context.Context, profileID string, rows []testrepo.PendingChange, result *CommitResult) {
	for _, c := range rows {
		var p struct {
			ProjectKey  string   `json:"projectKey"`
			Summary     string   `json:"summary"`
			Description string   `json:"description"`
			Priority    string   `json:"priority"`
			Labels      []string `json:"labels"`
			TestKey     string   `json:"testKey"`
		}
		if err := json.Unmarshal([]byte(c.AfterVal), &p); err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: c.EntityKey, Error: "malformed bug payload: " + err.Error()})
			continue
		}
		realKey, err := e.client.CreateBug(ctx, p.ProjectKey, p.Summary, p.Description, p.Priority, p.Labels)
		if err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: p.TestKey, Error: "create bug: " + sanitizeError(err.Error())})
			continue
		}
		key := c.EntityKey
		if realKey != "" && realKey != c.EntityKey {
			if rErr := e.repo.RenameBug(profileID, c.EntityKey, realKey); rErr != nil {
				_ = rErr // remote create already succeeded; a cache-rename hiccup must not fail the commit
			}
			key = realKey
		}
		if err := e.client.CreateBugLink(ctx, p.TestKey, key); err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: p.TestKey, Error: "link bug: " + sanitizeError(err.Error())})
			continue
		}
		if err := e.repo.CommitPendingChanges(profileID, []int64{c.ID}); err != nil {
			result.Failed = append(result.Failed, FailedCommit{TestKey: key, Error: "Jira created the bug but local cleanup failed: " + err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, key)
	}
}
```

Confirm `json`, `sanitizeError`, `FailedCommit`, `CommitResult` are already imported/defined in commit.go (they are — used by `commitTestCreates`).

- [ ] **Step 3: Build + test + commit**

Run: `cd /c/projects/xray-test-manager && go build ./... && go test ./internal/syncer/ ./internal/testrepo/`
Expected: builds; all pass.

```bash
git add internal/syncer/engine.go internal/syncer/commit.go internal/testrepo/testrepo.go
git commit -m "Bug tracking: syncBugs pass + commitBugCreates"
```

---

## Task 5: App bindings + api.ts

**Files:** Modify `app.go`, `frontend/src/api.ts`.

- [ ] **Step 1: Add the bindings**

In `app.go`, next to the requirement bindings, add (each guards with `a.requireStore()` like its neighbors):

```go
// CreateBugForTest queues a new Bug issue linked to a failed Test, committed to
// Jira on the next sync. Returns the placeholder key.
func (a *App) CreateBugForTest(profileID, testKey, execKey, summary, description, priority string, labels []string) (string, error) {
	if err := a.requireStore(); err != nil {
		return "", err
	}
	return a.repo.CreateBugForTest(profileID, testKey, execKey, testrepo.BugDraft{
		ProjectKey: a.profileProjectKey(profileID), Summary: summary,
		Description: description, Priority: priority, Labels: labels,
	})
}

// ListBugsWithTests returns every cached bug with the Tests it affects.
func (a *App) ListBugsWithTests(profileID string) ([]testrepo.BugWithTests, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListBugsWithTests(profileID)
}

// GetTestBugs returns the bugs linked to a Test (for the detail section).
func (a *App) GetTestBugs(profileID, testKey string) ([]testrepo.TestBug, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.GetTestBugs(profileID, testKey)
}
```

For `a.profileProjectKey(profileID)`: grep `app.go` for how an existing binding resolves a profile's project key (e.g. `a.profiles.Get(profileID)` returning `.ProjectKey`). If no helper exists, inline:

```go
func (a *App) profileProjectKey(profileID string) string {
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return ""
	}
	return p.ProjectKey
}
```

(Grep first — `a.profiles.Get` is used elsewhere in app.go, e.g. ExportProfile.)

- [ ] **Step 2: Regenerate bindings + add to api.ts**

Run a build to regenerate the wailsjs bindings (Task 8 does the full build; here just add the TS surface). In `frontend/src/api.ts`, add to the `export { ... } from "../wailsjs/go/main/App"` block: `CreateBugForTest`, `ListBugsWithTests`, `GetTestBugs`. Add interfaces:

```typescript
export interface BugWithTests {
  key: string;
  projectKey: string;
  summary: string;
  status: string;
  priority: string;
  testKeys: string[];
}

export interface TestBug {
  key: string;
  projectKey: string;
  summary: string;
  status: string;
  priority: string;
}
```

If the wailsjs bindings aren't regenerated yet, add the three functions manually to `frontend/wailsjs/go/main/App.js` and `App.d.ts` (mirror an existing 3-arg + array binding) so `tsc` passes; the Task 8 `wails build` will confirm/overwrite them.

- [ ] **Step 3: Build + typecheck + commit**

Run: `cd /c/projects/xray-test-manager && go build ./... && cd frontend && npx tsc --noEmit`
Expected: both clean.

```bash
git add app.go frontend/src/api.ts frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts
git commit -m "Bug tracking: app bindings + api.ts surface"
```

---

## Task 6: Test-detail Bugs section (Part 3)

**Files:** Modify `frontend/src/components/TestDetail.tsx`, `frontend/src/App.css`.

- [ ] **Step 1: Load the test's bugs**

In `TestDetail.tsx`: import `GetTestBugs` and the `TestBug` type from `../api`. Add state `const [bugs, setBugs] = useState<TestBug[]>([]);`. In the effect that loads the detail's linked data (where `GetTestRequirements` is called — grep `GetTestRequirements`), also load bugs in the same `Promise.all`, e.g. add `GetTestBugs(profileId, testKey)` and `setBugs(...)`. Reset to `[]` for `NEW-` keys (new tests have none).

- [ ] **Step 2: Render the read-only section**

After the Requirements section (grep the `<h4>Requirements` block, ends ~line 953), add. `openBugInJira` reuses the existing `jiraUrl`/demo guard already in this file (see `openInJira`):

```tsx
            <h4 className="detail-section-title">Bugs</h4>
            {bugs.length === 0 ? (
              <p className="muted">No linked bugs.</p>
            ) : (
              <ul className="pre-list bug-link-list">
                {bugs.map((b) => (
                  <li key={b.key}>
                    {canLinkToJira ? (
                      <button
                        className="mono bug-link-key"
                        onClick={() => openBugInJira(b.key)}
                        title={`Open ${b.key} in Jira (browser)`}
                      >
                        {b.key}
                      </button>
                    ) : (
                      <span className="mono">{b.key}</span>
                    )}
                    <span className="muted req-link-project">{b.projectKey}</span>
                    <span className="req-link-summary">{b.summary}</span>
                    {b.status && (
                      <span className="status-pill req-link-status">{b.status}</span>
                    )}
                  </li>
                ))}
              </ul>
            )}
```

Add the handler near `openInJira`:

```tsx
  function openBugInJira(bugKey: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base) BrowserOpenURL(`${base}/browse/${bugKey}`);
  }
```

(`BrowserOpenURL` is already imported in this file — confirm via grep; if not, add it to the `../api` import.)

- [ ] **Step 3: Style + typecheck + commit**

Append to `frontend/src/App.css`:

```css
.bug-link-key {
  background: none;
  border: none;
  padding: 0;
  color: var(--accent, #3884de);
  cursor: pointer;
  text-decoration: underline;
  font: inherit;
}
.bug-link-key:hover {
  color: var(--accent-strong, #2b6cb0);
}
```

Run: `cd frontend && npx tsc --noEmit` → clean.

```bash
git add frontend/src/components/TestDetail.tsx frontend/src/App.css
git commit -m "Bug tracking: linked-bugs section on test detail (hyperlinks, cross-project)"
```

---

## Task 7: Create-bug modal + execution-board action (Part 1)

**Files:** Create `frontend/src/components/CreateBugModal.tsx`; Modify `frontend/src/components/ContainersView.tsx`, `frontend/src/App.css`.

- [ ] **Step 1: Build CreateBugModal**

Create `frontend/src/components/CreateBugModal.tsx`. Uses the existing `.modal-overlay`/`.modal` shell. Props give the failed row's context; `onCreated` is called after a successful create.

```tsx
import { useState } from "react";
import { CreateBugForTest, errMsg } from "../api";

interface Props {
  profileId: string;
  testKey: string;
  testSummary: string;
  execKey: string;
  onClose: () => void;
  onCreated: () => void;
}

const PRIORITIES = ["Highest", "High", "Medium", "Low", "Lowest"];

// CreateBugModal files a Bug-type Jira issue against a test marked FAILED in an
// execution. Local-first: the bug is queued and pushed on the next Commit.
export function CreateBugModal({
  profileId,
  testKey,
  testSummary,
  execKey,
  onClose,
  onCreated,
}: Props) {
  const [summary, setSummary] = useState("");
  const [priority, setPriority] = useState("Medium");
  const [labels, setLabels] = useState("");
  const [description, setDescription] = useState(
    `Found while executing ${execKey}.\nTest ${testKey} "${testSummary}" was marked FAILED.\n\nSteps / actual result:\n`,
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function create() {
    if (!summary.trim()) return;
    setBusy(true);
    setError("");
    try {
      await CreateBugForTest(
        profileId,
        testKey,
        execKey,
        summary.trim(),
        description,
        priority,
        labels.trim() ? labels.trim().split(/[\s,]+/) : [],
      );
      onCreated();
      onClose();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="pending-head">
          <h2>Create bug for {testKey}</h2>
          <button className="btn btn-ghost" onClick={onClose} title="Cancel" aria-label="Cancel">
            ✕
          </button>
        </div>
        <div className="bug-form">
          <label>
            Summary
            <input
              value={summary}
              autoFocus
              onChange={(e) => setSummary(e.target.value)}
              placeholder="Short defect title"
            />
          </label>
          <label>
            Priority
            <select value={priority} onChange={(e) => setPriority(e.target.value)}>
              {PRIORITIES.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </select>
          </label>
          <label>
            Labels (space or comma separated)
            <input
              value={labels}
              onChange={(e) => setLabels(e.target.value)}
              placeholder="regression login"
            />
          </label>
          <label>
            Description
            <textarea
              rows={6}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </label>
          {error && <div className="error-text">{error}</div>}
        </div>
        <div className="pending-actions">
          <button className="btn" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button
            className="btn btn-primary"
            onClick={create}
            disabled={busy || !summary.trim()}
          >
            {busy ? "Filing…" : "Create bug"}
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Wire the 🐞 action into the execution board**

In `ContainersView.tsx`: import `CreateBugModal`. Add state `const [bugFor, setBugFor] = useState<{ testKey: string; summary: string } | null>(null);`. The view needs `execKey` (the selected container key) and `profileId` (already a prop) and `jiraUrl` (Task 8 passes it; not needed for create). In the board row rendering for executions (grep the `kind === "testexec"` run-status cell / the remove-cell `board-remove`), add — only when the row's run status is a failure — a button before/after the remove ✕:

```tsx
                {kind === "testexec" &&
                  /^fail/i.test(r.runStatus || "") && (
                    <button
                      className="btn btn-ghost board-bug"
                      title="Create a bug for this failed test"
                      onClick={() => setBugFor({ testKey: r.testKey, summary: r.summary })}
                    >
                      🐞
                    </button>
                  )}
```

Place it inside the existing actions cell (the same `<td>` as the remove button) so the table columns don't shift. Then render the modal near the view's other modals:

```tsx
      {bugFor && (
        <CreateBugModal
          profileId={profileId}
          testKey={bugFor.testKey}
          testSummary={bugFor.summary}
          execKey={selectedContainer}
          onClose={() => setBugFor(null)}
          onCreated={() => {
            setBugFor(null);
            onChanged?.();
          }}
        />
      )}
```

Use whatever the view's selected-execution-key variable is actually named (grep — likely `selected` / `selectedContainer` / `containerKey`) and the existing refresh callback (grep `onChanged` / `refreshKey` — ContainersView already triggers a refresh after run-status edits; reuse that path).

- [ ] **Step 3: Style + typecheck + commit**

Append to `App.css`:

```css
.bug-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
}
.bug-form label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
}
.board-bug {
  padding: 0 4px;
}
```

Run: `cd frontend && npx tsc --noEmit` → clean.

```bash
git add frontend/src/components/CreateBugModal.tsx frontend/src/components/ContainersView.tsx frontend/src/App.css
git commit -m "Bug tracking: create-bug modal + 🐞 action on failed execution rows"
```

---

## Task 8: Bugs panel + Containers toggle + pending description + full verify (Part 2)

**Files:** Create `frontend/src/components/BugsPanel.tsx`; Modify `frontend/src/components/ContainersView.tsx`, `frontend/src/components/PendingChangesModal.tsx`, `frontend/src/App.tsx`, `frontend/src/App.css`.

- [ ] **Step 1: Build BugsPanel**

Create `frontend/src/components/BugsPanel.tsx`:

```tsx
import { useEffect, useMemo, useState } from "react";
import { ListBugsWithTests, BrowserOpenURL, errMsg } from "../api";
import type { BugWithTests } from "../api";

interface Props {
  profileId: string;
  refreshKey: number;
  jiraUrl: string;
  onOpenTest: (testKey: string) => void;
}

// BugsPanel lists every bug linked to the profile's tests, with the tests each
// affects. Bug keys open in the browser; test keys open the test detail.
export function BugsPanel({ profileId, refreshKey, jiraUrl, onOpenTest }: Props) {
  const [bugs, setBugs] = useState<BugWithTests[]>([]);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    ListBugsWithTests(profileId)
      .then((bs) => {
        if (!cancelled) setBugs(bs ?? []);
      })
      .catch((e) => {
        if (!cancelled) setError(errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey]);

  const isDemo = /^(demo$|demo:|mock:)/i.test((jiraUrl ?? "").trim());
  const canLink = !!jiraUrl && !isDemo;
  function openBug(key: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base && canLink && !key.startsWith("NEW-")) BrowserOpenURL(`${base}/browse/${key}`);
  }

  const shown = useMemo(() => {
    const f = filter.trim().toLowerCase();
    if (!f) return bugs;
    return bugs.filter(
      (b) =>
        b.key.toLowerCase().includes(f) ||
        b.summary.toLowerCase().includes(f) ||
        b.projectKey.toLowerCase().includes(f) ||
        b.status.toLowerCase().includes(f),
    );
  }, [bugs, filter]);

  return (
    <div className="bugs-panel">
      {error && <div className="error-text">{error}</div>}
      <input
        className="search"
        placeholder="Filter bugs by key, summary, project, status…"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
      />
      {shown.length === 0 ? (
        <p className="muted">
          {bugs.length === 0
            ? "No bugs linked to this profile's tests. File one from a failed test in a Test Execution, or sync a demo profile."
            : "No bugs match the filter."}
        </p>
      ) : (
        <table className="board-table bugs-table">
          <thead>
            <tr>
              <th>Bug</th>
              <th>Project</th>
              <th>Summary</th>
              <th>Status</th>
              <th>Priority</th>
              <th>Affects</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((b) => (
              <tr key={b.key}>
                <td>
                  {canLink && !b.key.startsWith("NEW-") ? (
                    <button className="mono bug-link-key" onClick={() => openBug(b.key)} title={`Open ${b.key} in Jira`}>
                      {b.key}
                    </button>
                  ) : (
                    <span className="mono">{b.key}</span>
                  )}
                </td>
                <td className="muted">{b.projectKey}</td>
                <td>{b.summary}</td>
                <td>{b.status && <span className="status-pill">{b.status}</span>}</td>
                <td>{b.priority}</td>
                <td>
                  {b.testKeys.map((tk, i) => (
                    <span key={tk}>
                      {i > 0 && ", "}
                      <button className="mono bug-link-key" onClick={() => onOpenTest(tk)} title={`Open ${tk}`}>
                        {tk}
                      </button>
                    </span>
                  ))}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Add the [Containers | Bugs] toggle**

In `ContainersView.tsx`: add `const [mode, setMode] = useState<"containers" | "bugs">("containers");`. At the top of the view's returned JSX (above the kind selector), add a segmented toggle:

```tsx
        <div className="containers-mode">
          <button
            className={`seg-btn${mode === "containers" ? " seg-btn-active" : ""}`}
            onClick={() => setMode("containers")}
          >
            Containers
          </button>
          <button
            className={`seg-btn${mode === "bugs" ? " seg-btn-active" : ""}`}
            onClick={() => setMode("bugs")}
          >
            Bugs
          </button>
        </div>
```

Wrap the existing container UI so it renders only when `mode === "containers"`, and render `<BugsPanel .../>` when `mode === "bugs"`:

```tsx
      {mode === "bugs" ? (
        <BugsPanel
          profileId={profileId}
          refreshKey={refreshKey}
          jiraUrl={jiraUrl}
          onOpenTest={onOpenTest}
        />
      ) : (
        /* ...existing kind selector + container list + board... */
      )}
```

`ContainersView` must receive `jiraUrl` and `onOpenTest` props — add them to its Props interface. `refreshKey` is likely already a prop (grep). For `onOpenTest`, App.tsx already has a way to select a test + switch to Browse (grep how other views open a test, e.g. RequirementsView's covering-test click or `setSelectedKey`/`setView`).

- [ ] **Step 3: App.tsx wiring**

In `App.tsx`, find where `<ContainersView ... />` is rendered and pass `jiraUrl={activeProfile?.jiraUrl ?? ""}` and `onOpenTest={(k) => { setSelectedKey(k); setView("browse"); }}` (use the actual setters present in App.tsx — grep `setSelectedKey` / `setView`).

- [ ] **Step 4: PendingChangesModal description**

In `frontend/src/components/PendingChangesModal.tsx`, in the `describeChange` switch, add a case (mirror the `test_create` case shape):

```tsx
    case "bug_create":
      return { field: "new bug", before: "", after: stepActionLike(c.afterVal, "summary") };
```

(Confirm `stepActionLike` exists in that file and extracts a JSON field — it's used by `test_create`/`precondition_add`. If the helper differs, mirror exactly what `case "test_create"` does.)

- [ ] **Step 5: Styles**

Append to `App.css`:

```css
.containers-mode {
  display: inline-flex;
  gap: 0;
  margin-bottom: 10px;
  border: 1px solid var(--border, #2a3a4f);
  border-radius: 6px;
  overflow: hidden;
}
.seg-btn {
  background: none;
  border: none;
  padding: 6px 14px;
  cursor: pointer;
  color: var(--text, inherit);
  font-size: 13px;
}
.seg-btn-active {
  background: var(--accent-soft, rgba(56, 132, 222, 0.18));
  font-weight: 600;
}
.bugs-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.bugs-table td {
  vertical-align: top;
}
```

- [ ] **Step 6: Full build + manual verification**

Run: `cd frontend && npm run build` → tsc + vite clean.
Run (PowerShell): `Set-Location C:\projects\xray-test-manager; $env:Path += ";$env:USERPROFILE\go\bin"; wails build` → `Built ...xray-test-manager.exe`.

Manual demo click-through (launch `build\bin\xray-test-manager.exe`, open/sync a profile whose Jira URL is `demo`):
  1. Sync → the demo generates `BUGS-*` bugs linked to `DEMO-1..6` (+ `DEMO-7`).
  2. Containers → **Bugs** toggle → the bug list shows cross-project `BUGS-*` rows with their affected test keys; clicking a test key opens its detail.
  3. Open `DEMO-1` detail → **Bugs** section lists `BUGS-1` as a hyperlink (no-op in demo; real profiles open the browser); project column shows `BUGS`.
  4. Containers → Test Executions → pick an execution with a FAILED test → click 🐞 on that row → fill the form → Create bug → it appears in **Pending Changes** as "new bug — <summary>", and on that test's detail under Bugs with a `NEW-BUG-1` key.
  5. Discard that pending change → the bug disappears from the detail.

- [ ] **Step 7: Commit (restore binding noise first)**

`wails build` regenerates `frontend/wailsjs/go/*` + `frontend/package.json.md5` + may flip `go.mod` line endings. Keep the NEW bindings (the three bug methods) in `App.d.ts`/`App.js`; restore pure-noise files (`git restore frontend/package.json.md5 go.mod` and `models.ts` if only line-endings changed — verify with `git diff`).

```bash
git add frontend/src/components/BugsPanel.tsx frontend/src/components/ContainersView.tsx frontend/src/components/PendingChangesModal.tsx frontend/src/App.tsx frontend/src/App.css frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/main/App.js
git commit -m "Bug tracking: Bugs panel + Containers toggle + pending-change label"
```

---

## Self-Review notes

- **Spec coverage:** schema/types (T1) · local create/discard (T2) · jira demo+stubs (T3) · sync+commit (T4) · bindings/api (T5) · detail section / Part 3 (T6) · create-from-failed-test / Part 1 (T7) · Bugs panel / Part 2 + pending label (T8). All spec sections map to a task.
- **No new requirement-style source config** (YAGNI per spec) — discovery is by test links + issuetype set, handled in `jira/bugs.go`.
- **Type consistency:** Go `Bug`/`BugLink`/`BugWithTests`/`TestBug`/`BugDraft` (T1) match the jira `Bug`/`BugLink` (T3) mapped in `syncBugs` (T4) and the TS `BugWithTests`/`TestBug` (T5). Entity `"bug_create"` is identical across `CreateBugForTest` (T2), the discard case (T2), the commit collector + `commitBugCreates` (T4), and `describeChange` (T8). `CreateBugForTest` binding arg order (profileId, testKey, execKey, summary, description, priority, labels) matches the modal call (T7).
- **Grep-before-edit reminders** are inline where a twin symbol's exact name/location must be confirmed against the current files (`AllTestKeys`, `a.profiles.Get`, ContainersView's selected-key + refresh + open-test, `stepActionLike`, `BrowserOpenURL` import).
