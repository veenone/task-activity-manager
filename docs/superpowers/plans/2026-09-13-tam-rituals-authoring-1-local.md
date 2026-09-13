# TAM Rituals authoring, part 1: local drafts and the wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ritual documents authorable inside TAM: a draft per sprint and ritual type, scaffolded four at a time, carrying an author remark, a chosen set of the sprint's Jira issues, and a remark on each of them.

**Architecture:** `tam/internal/ritualrepo` already stores ritual documents in `tam.db` and is reachable only from `SeedDemo`. This plan widens its row to carry per-issue remarks, adds a package that picks each ritual type's default issue set, exposes five App methods so the frontend can reach the repository at all, and builds the slots and wizard in `RitualsView`. Nothing here talks to Confluence: publishing is part 2, and every draft this plan creates ends at `status = "draft"`.

**Tech Stack:** Go 1.27 (toolchain auto-downloads; run tooling with `C:/Python313/python.exe` only for scripts, not builds), SQLite via `core/store`, Wails v2.15.0 bindings, React 19 + TypeScript, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-09-13-tam-rituals-authoring-design.md`

## Global Constraints

- Schema version becomes **10**. A column on an existing table needs a migration entry in the `AddColumnIfMissing` shape used by versions 7 and 8.
- `status` is one of exactly `"draft"`, `"queued"`, `"published"`, `"stale"`. This plan only ever writes `"draft"`.
- Issue order in `issues_json` **is** the published order. Never re-sort it from a query result.
- No clock value is ever written into rendered content. Timestamps belong in `updated_at` and `published_at`.
- Remarks are plain text. No rich text editor, no HTML authoring in TAM.
- Every new App method takes `profileID` first and calls `a.requireStore()` before touching anything, matching `tam/app_rituals.go`.
- Go test names read as sentences, matching `TestPurgeProfileClearsTheFourBoardTables`.
- Counts to beat, never lower: 46 tests in `frontend/core`, 159 in `xtm`, 485 in `tam` frontend.
- Do not modify generated files. `tam/frontend/wailsjs/**` is regenerated with `wails generate module`, never hand-edited.

---

### Task 1: Schema version 10 and the `issues_json` column

**Files:**
- Modify: `tam/internal/tamstore/tamstore.go:36` (version), the `ritual_document` DDL at `:256`, and the migration list ending at `:127`
- Test: `tam/internal/tamstore/tamstore_test.go`

**Interfaces:**
- Consumes: `store.AddColumnIfMissing(db *sql.DB, table, columnDDL string) error` from `core/store/store.go:126`
- Produces: a `ritual_document.issues_json` column, `TEXT NOT NULL DEFAULT '[]'`, holding a JSON array of `{"key": string, "remark": string}` objects. Old `issue_keys_json` stays in place and unread.

- [ ] **Step 1: Write the failing test**

Add to `tam/internal/tamstore/tamstore_test.go`:

```go
func TestVersionTenAddsIssuesJSONAndConvertsOldKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tam.db")

	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.DB().Exec(`INSERT INTO ritual_document
		(profile_id, board_id, sprint_id, ritual_type, issue_keys_json, issues_json)
		VALUES ('p1', 1, 12, 'review', '["PLAT-14","PLAT-22"]', '[]')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := db.DB().Exec(`UPDATE schema_version SET version = 9`); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	var got string
	if err := reopened.DB().QueryRow(`SELECT issues_json FROM ritual_document
		WHERE profile_id = 'p1' AND board_id = 1 AND sprint_id = 12 AND ritual_type = 'review'`).Scan(&got); err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := `[{"key":"PLAT-14","remark":""},{"key":"PLAT-22","remark":""}]`
	if got != want {
		t.Fatalf("issues_json = %s, want %s", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tam && go test ./internal/tamstore/ -run TestVersionTenAddsIssuesJSONAndConvertsOldKeys -count=1 -v`
Expected: FAIL. The insert errors with `table ritual_document has no column named issues_json`.

- [ ] **Step 3: Add the column to Base and bump the version**

In `tam/internal/tamstore/tamstore.go`, change `Version: 9` at line 36 to `Version: 10`.

In the `ritual_document` DDL beginning at line 256, add the column after `issue_keys_json`:

```sql
	issue_keys_json   TEXT NOT NULL DEFAULT '[]',
	issues_json       TEXT NOT NULL DEFAULT '[]',
```

- [ ] **Step 4: Add the migration**

Append to the migration slice, directly after the `Version: 8` entry that ends at line 127:

```go
	}, {
		Version: 10,
		// ritual_document arrived whole at version 9, so Base created it and
		// no migration was needed. issues_json is a column on a table that now
		// exists, which CREATE TABLE IF NOT EXISTS will not touch, so it needs
		// the same treatment goal and complete_date got at versions 7 and 8.
		//
		// The old issue_keys_json is left in place and unread rather than
		// dropped: SQLite makes column removal a table rebuild, and a rebuild
		// here would risk a user's unpublished drafts to reclaim nothing.
		Apply: func(db *sql.DB) error {
			if err := store.AddColumnIfMissing(db, "ritual_document", "issues_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
				return err
			}
			return convertRitualIssueKeys(db)
		},
	}},
```

- [ ] **Step 5: Write the conversion helper**

Add to `tam/internal/tamstore/tamstore.go`, below the migration slice:

```go
// convertRitualIssueKeys rewrites version 9's bare key array into version 10's
// objects, so a draft written before per-issue remarks existed keeps its issues
// and their order. Only rows that have not already been converted are touched.
func convertRitualIssueKeys(db *sql.DB) error {
	rows, err := db.Query(`SELECT profile_id, board_id, sprint_id, ritual_type, issue_keys_json
		FROM ritual_document WHERE issues_json = '[]' AND issue_keys_json <> '[]'`)
	if err != nil {
		return fmt.Errorf("read ritual issue keys: %w", err)
	}
	type target struct {
		profileID  string
		boardID    int
		sprintID   int
		ritualType string
		issuesJSON string
	}
	var targets []target
	for rows.Next() {
		var t target
		var keysJSON string
		if err := rows.Scan(&t.profileID, &t.boardID, &t.sprintID, &t.ritualType, &keysJSON); err != nil {
			rows.Close()
			return fmt.Errorf("scan ritual issue keys: %w", err)
		}
		var keys []string
		if err := json.Unmarshal([]byte(keysJSON), &keys); err != nil {
			// A row nobody can parse is left exactly as it is rather than
			// emptied: the draft is the user's unpublished work.
			continue
		}
		converted := make([]struct {
			Key    string `json:"key"`
			Remark string `json:"remark"`
		}, 0, len(keys))
		for _, k := range keys {
			converted = append(converted, struct {
				Key    string `json:"key"`
				Remark string `json:"remark"`
			}{Key: k})
		}
		encoded, err := json.Marshal(converted)
		if err != nil {
			rows.Close()
			return fmt.Errorf("encode ritual issues: %w", err)
		}
		t.issuesJSON = string(encoded)
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate ritual issue keys: %w", err)
	}
	rows.Close()

	for _, t := range targets {
		if _, err := db.Exec(`UPDATE ritual_document SET issues_json = ?
			WHERE profile_id = ? AND board_id = ? AND sprint_id = ? AND ritual_type = ?`,
			t.issuesJSON, t.profileID, t.boardID, t.sprintID, t.ritualType); err != nil {
			return fmt.Errorf("write ritual issues for %s: %w", t.ritualType, err)
		}
	}
	return nil
}
```

Add `"encoding/json"` to the file's imports if it is not already there.

- [ ] **Step 6: Run test to verify it passes**

Run: `cd tam && go test ./internal/tamstore/ -count=1`
Expected: PASS, including every existing migration test.

- [ ] **Step 7: Commit**

```bash
git add tam/internal/tamstore/tamstore.go tam/internal/tamstore/tamstore_test.go
git commit -m "feat(tam): a ritual issue that can carry a remark"
```

---

### Task 2: The repository learns issues and lists a sprint

**Files:**
- Modify: `tam/internal/ritualrepo/ritualrepo.go`
- Test: `tam/internal/ritualrepo/ritualrepo_test.go`

**Interfaces:**
- Consumes: `ritual_document.issues_json` from Task 1.
- Produces:
  - `type Issue struct { Key string \`json:"key"\`; Remark string \`json:"remark"\` }`
  - `Draft.IssuesJSON string` replacing `Draft.IssueKeysJSON`
  - `func EncodeIssues(issues []Issue) (string, error)`
  - `func DecodeIssues(encoded string) ([]Issue, error)`
  - `func (r *Repository) ListDrafts(ctx context.Context, profileID string, boardID, sprintID int) ([]Draft, error)` ordered by `ritual_type`

- [ ] **Step 1: Write the failing test**

Add to `tam/internal/ritualrepo/ritualrepo_test.go`:

```go
func TestIssuesRoundTripThroughADraft(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)

	issues := []ritualrepo.Issue{
		{Key: "PLAT-14", Remark: "demoed, docs follow-up"},
		{Key: "PLAT-22", Remark: "blocked on infra"},
	}
	encoded, err := ritualrepo.EncodeIssues(issues)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := r.Upsert(ctx, ritualrepo.Draft{
		ProfileID: "p1", BoardID: 1, SprintID: 12, RitualType: "review",
		IssuesJSON: encoded, Status: "draft",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := r.Get(ctx, "p1", 1, 12, "review")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	back, err := ritualrepo.DecodeIssues(got.IssuesJSON)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(back) != 2 || back[0].Key != "PLAT-14" || back[1].Remark != "blocked on infra" {
		t.Fatalf("issues = %#v, want the two seeded in order", back)
	}
}

func TestListDraftsReturnsOneSprintsRitualsInTypeOrder(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)

	for _, ritualType := range []string{"review", "planning", "standup"} {
		if err := r.Upsert(ctx, ritualrepo.Draft{
			ProfileID: "p1", BoardID: 1, SprintID: 12, RitualType: ritualType, Status: "draft",
		}); err != nil {
			t.Fatalf("upsert %s: %v", ritualType, err)
		}
	}
	// A different sprint, which must not appear.
	if err := r.Upsert(ctx, ritualrepo.Draft{
		ProfileID: "p1", BoardID: 1, SprintID: 13, RitualType: "planning", Status: "draft",
	}); err != nil {
		t.Fatalf("upsert other sprint: %v", err)
	}

	drafts, err := r.ListDrafts(ctx, "p1", 1, 12)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(drafts) != 3 {
		t.Fatalf("got %d drafts, want 3", len(drafts))
	}
	if drafts[0].RitualType != "planning" || drafts[2].RitualType != "standup" {
		t.Fatalf("order = %s, %s, %s; want planning, review, standup",
			drafts[0].RitualType, drafts[1].RitualType, drafts[2].RitualType)
	}
}
```

If `newTestRepo` does not already exist in that file, add it:

```go
func newTestRepo(t *testing.T) *ritualrepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return ritualrepo.New(db.DB())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tam && go test ./internal/ritualrepo/ -count=1`
Expected: FAIL to compile, with `undefined: ritualrepo.Issue`, `undefined: ritualrepo.EncodeIssues`, and `IssuesJSON` not a field of `Draft`.

- [ ] **Step 3: Add the issue type and its codec**

In `tam/internal/ritualrepo/ritualrepo.go`, above `Draft`:

```go
// Issue is one Jira issue chosen for a ritual, with the remark the team made
// about it. The slice order is the order the published table uses, so it is
// preserved exactly as stored and never re-sorted from a query result.
type Issue struct {
	Key    string `json:"key"`
	Remark string `json:"remark"`
}

// EncodeIssues renders the chosen issues for storage. A nil slice encodes as an
// empty array rather than null, so the column's default and a cleared selection
// are the same value.
func EncodeIssues(issues []Issue) (string, error) {
	if issues == nil {
		issues = []Issue{}
	}
	encoded, err := json.Marshal(issues)
	if err != nil {
		return "", fmt.Errorf("encode ritual issues: %w", err)
	}
	return string(encoded), nil
}

// DecodeIssues reads stored issues. An empty column decodes as an empty slice.
func DecodeIssues(encoded string) ([]Issue, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return []Issue{}, nil
	}
	var issues []Issue
	if err := json.Unmarshal([]byte(trimmed), &issues); err != nil {
		return nil, fmt.Errorf("decode ritual issues: %w", err)
	}
	if issues == nil {
		issues = []Issue{}
	}
	return issues, nil
}
```

Add `"encoding/json"` and `"strings"` to the imports.

- [ ] **Step 4: Swap the Draft field and the SQL**

In `Draft`, replace the `IssueKeysJSON` field with:

```go
	IssuesJSON        string `json:"issuesJson"`
```

In `selectDraftSQL`, replace `issue_keys_json` with `issues_json`.

In `upsertDraftSQL`, replace `issue_keys_json` in the column list with `issues_json`, and replace the `issue_keys_json = excluded.issue_keys_json` line with `issues_json = excluded.issues_json`.

In `Get`, change `&draft.IssueKeysJSON` to `&draft.IssuesJSON`. In `Upsert`, change `draft.IssueKeysJSON` to `draft.IssuesJSON`.

- [ ] **Step 5: Add ListDrafts**

Append to `tam/internal/ritualrepo/ritualrepo.go`:

```go
const listDraftsSQL = `
	SELECT profile_id, board_id, sprint_id, ritual_type, title, remark, body,
		issues_json, confluence_page_id, confluence_version, status,
		updated_at, published_at
	FROM ritual_document
	WHERE profile_id = ? AND board_id = ? AND sprint_id = ?
	ORDER BY ritual_type`

// ListDrafts returns every ritual document stored for one sprint, ordered by
// ritual type so the view's slots do not reshuffle between reads.
func (r *Repository) ListDrafts(ctx context.Context, profileID string, boardID, sprintID int) ([]Draft, error) {
	rows, err := r.db.QueryContext(ctx, listDraftsSQL, profileID, boardID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("list rituals for board %d sprint %d: %w", boardID, sprintID, err)
	}
	defer rows.Close()

	drafts := []Draft{}
	for rows.Next() {
		var draft Draft
		if err := rows.Scan(
			&draft.ProfileID, &draft.BoardID, &draft.SprintID, &draft.RitualType,
			&draft.Title, &draft.Remark, &draft.Body, &draft.IssuesJSON,
			&draft.ConfluencePageID, &draft.ConfluenceVersion, &draft.Status,
			&draft.UpdatedAt, &draft.PublishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ritual for board %d sprint %d: %w", boardID, sprintID, err)
		}
		drafts = append(drafts, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rituals for board %d sprint %d: %w", boardID, sprintID, err)
	}
	return drafts, nil
}
```

- [ ] **Step 6: Fix the demo seed**

`tam/internal/ritualrepo/demo_seed.go` sets `IssueKeysJSON`. Change each assignment to `IssuesJSON`, and where it wrote a bare key array, build the value with `EncodeIssues`:

```go
	issues, err := EncodeIssues([]Issue{{Key: projectKey + "-1"}, {Key: projectKey + "-2"}})
	if err != nil {
		return err
	}
```

Use `issues` wherever the old key JSON string was passed.

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd tam && go build ./... && go test ./internal/ritualrepo/ ./internal/tamstore/ -count=1`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add tam/internal/ritualrepo/
git commit -m "feat(tam): a sprint's rituals, and the remarks on their issues"
```

---

### Task 3: What each ritual asks for

**Files:**
- Create: `tam/internal/ritualdefaults/ritualdefaults.go`
- Test: `tam/internal/ritualdefaults/ritualdefaults_test.go`

**Interfaces:**
- Consumes: `backend.Issue` (`tam/internal/backend/backend.go:40`, fields `Key string`, `Status string`, `StoryPoints *float64`), `backend.IsDone(status string) bool` (`tam/internal/backend/status.go:11`), `ritualrepo.Issue` from Task 2.
- Produces:
  - `var Types = []string{"planning", "standup", "review", "retro"}`
  - `func Select(ritualType string, issues []backend.Issue) []ritualrepo.Issue`
  - `func Title(ritualType, sprintName string) string`

This lives in its own package rather than inside `ritualrepo` because it is a policy about rituals, not a query about rows, and both the scaffold and the wizard need it.

- [ ] **Step 1: Write the failing test**

Create `tam/internal/ritualdefaults/ritualdefaults_test.go`:

```go
package ritualdefaults_test

import (
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/ritualdefaults"
)

func points(v float64) *float64 { return &v }

func sample() []backend.Issue {
	return []backend.Issue{
		{Key: "PLAT-1", Status: "To Do", StoryPoints: points(3)},
		{Key: "PLAT-2", Status: "In Progress", StoryPoints: nil},
		{Key: "PLAT-3", Status: "Done", StoryPoints: points(5)},
		{Key: "PLAT-4", Status: "Blocked", StoryPoints: points(2)},
	}
}

func keys(issues []ritualdefaults.Selected) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Key)
	}
	return out
}

func TestPlanningTakesTheWholeSprintUnestimatedFirst(t *testing.T) {
	got := keys(ritualdefaults.Select("planning", sample()))
	want := []string{"PLAT-2", "PLAT-1", "PLAT-3", "PLAT-4"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestStandupTakesOnlyWorkInFlight(t *testing.T) {
	got := keys(ritualdefaults.Select("standup", sample()))
	if len(got) != 2 || got[0] != "PLAT-2" || got[1] != "PLAT-4" {
		t.Fatalf("got %v, want the in-progress and blocked issues", got)
	}
}

func TestReviewTakesOnlyWhatIsDone(t *testing.T) {
	got := keys(ritualdefaults.Select("review", sample()))
	if len(got) != 1 || got[0] != "PLAT-3" {
		t.Fatalf("got %v, want only the done issue", got)
	}
}

func TestRetroLeadsWithWhatDidNotFinish(t *testing.T) {
	got := keys(ritualdefaults.Select("retro", sample()))
	want := []string{"PLAT-1", "PLAT-2", "PLAT-4", "PLAT-3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestAnUnknownRitualTypeSelectsNothing(t *testing.T) {
	if got := ritualdefaults.Select("retrospective-party", sample()); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestTitleNamesTheSprintAndTheRitual(t *testing.T) {
	if got := ritualdefaults.Title("retro", "Sprint 14"); got != "Sprint 14 Retrospective" {
		t.Fatalf("title = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tam && go test ./internal/ritualdefaults/ -count=1`
Expected: FAIL, `no required module provides package agile-suite/tam/internal/ritualdefaults`.

- [ ] **Step 3: Write the implementation**

Create `tam/internal/ritualdefaults/ritualdefaults.go`:

```go
// Package ritualdefaults decides which of a sprint's issues each ritual starts
// with, so the same four documents appear each cycle without anyone choosing.
package ritualdefaults

import (
	"sort"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/ritualrepo"
)

// Types is the four rituals a sprint gets, in the order they happen.
var Types = []string{"planning", "standup", "review", "retro"}

// Selected is one chosen issue. It is ritualrepo.Issue under another name so
// this package can be read without knowing where drafts are stored.
type Selected = ritualrepo.Issue

// labels name each ritual in a page title.
var labels = map[string]string{
	"planning": "Planning",
	"standup":  "Standup",
	"review":   "Review",
	"retro":    "Retrospective",
}

// Select returns the issues a ritual opens with, in the order they should be
// published. An unknown ritual type selects nothing rather than guessing.
//
// The rules are deliberately derivable from a cached issue alone. Nothing here
// reads a sprint report, so a scaffold works before a report has ever been
// built.
func Select(ritualType string, issues []backend.Issue) []Selected {
	switch strings.TrimSpace(strings.ToLower(ritualType)) {
	case "planning":
		// Everything in the sprint, with unestimated work first, because the
		// unestimated issues are what planning is for.
		return order(issues, func(a, b backend.Issue) bool {
			ae, be := a.StoryPoints == nil, b.StoryPoints == nil
			if ae != be {
				return ae
			}
			return false
		})
	case "standup":
		return pick(issues, func(i backend.Issue) bool {
			return !backend.IsDone(i.Status) && inFlight(i.Status)
		})
	case "review":
		return pick(issues, func(i backend.Issue) bool { return backend.IsDone(i.Status) })
	case "retro":
		// Unfinished work first: that is what a retrospective opens on.
		return order(issues, func(a, b backend.Issue) bool {
			ad, bd := backend.IsDone(a.Status), backend.IsDone(b.Status)
			if ad != bd {
				return !ad
			}
			return false
		})
	default:
		return []Selected{}
	}
}

// Title names a ritual's page.
func Title(ritualType, sprintName string) string {
	label, ok := labels[strings.TrimSpace(strings.ToLower(ritualType))]
	if !ok {
		return strings.TrimSpace(sprintName)
	}
	name := strings.TrimSpace(sprintName)
	if name == "" {
		return label
	}
	return name + " " + label
}

// inFlight is true for work somebody has started or is stuck on. It matches on
// the status name, the same shape backend.IsDone uses, because a cached issue
// carries the name and not the board's column.
func inFlight(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return strings.Contains(s, "progress") || strings.Contains(s, "block") ||
		strings.Contains(s, "review") || strings.Contains(s, "testing")
}

func pick(issues []backend.Issue, keep func(backend.Issue) bool) []Selected {
	out := []Selected{}
	for _, i := range issues {
		if keep(i) {
			out = append(out, Selected{Key: i.Key})
		}
	}
	return out
}

// order keeps every issue, sorted by less, preserving the incoming order within
// a group. SliceStable matters: the incoming order is the board's rank, and two
// issues that tie must not swap between runs or the render stops being
// deterministic.
func order(issues []backend.Issue, less func(a, b backend.Issue) bool) []Selected {
	sorted := make([]backend.Issue, len(issues))
	copy(sorted, issues)
	sort.SliceStable(sorted, func(i, j int) bool { return less(sorted[i], sorted[j]) })
	return pick(sorted, func(backend.Issue) bool { return true })
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd tam && go test ./internal/ritualdefaults/ -count=1 -v`
Expected: PASS, all six tests.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/ritualdefaults/
git commit -m "feat(tam): what each ritual asks a sprint for"
```

---

### Task 4: The app seam reaches the repository

**Files:**
- Modify: `tam/app.go:36-58` (add the field), `tam/app_rituals.go` (add methods)
- Test: `tam/app_rituals_test.go` (create if absent)

**Interfaces:**
- Consumes: `ritualrepo.Repository` from Task 2, `a.requireStore()` (`tam/app.go:167`), `a.local.DB()`.
- Produces, all on `*App`:
  - `ListRitualDrafts(profileID string, boardID, sprintID int) ([]ritualrepo.Draft, error)`
  - `GetRitualDraft(profileID string, boardID, sprintID int, ritualType string) (ritualrepo.Draft, error)`
  - `SaveRitualDraft(profileID string, draft ritualrepo.Draft) error`
  - `DeleteRitualDraft(profileID string, boardID, sprintID int, ritualType string) error`

- [ ] **Step 1: Write the failing test**

Create `tam/app_rituals_test.go`:

```go
package main

import (
	"testing"

	"agile-suite/tam/internal/ritualrepo"
)

func TestSavingARitualDraftStoresItAgainstTheSprint(t *testing.T) {
	a, p := newTestAppWithProfile(t)

	issues, err := ritualrepo.EncodeIssues([]ritualrepo.Issue{{Key: "PLAT-14", Remark: "demoed"}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{
		BoardID: 1, SprintID: 12, RitualType: "review",
		Title: "Sprint 12 Review", Remark: "short sprint", IssuesJSON: issues,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := a.GetRitualDraft(p.ID, 1, 12, "review")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Remark != "short sprint" {
		t.Fatalf("remark = %q", got.Remark)
	}
	if got.Status != "draft" {
		t.Fatalf("status = %q, want draft; saving must never publish", got.Status)
	}
	if got.UpdatedAt == "" {
		t.Fatal("updatedAt was not stamped")
	}
}

func TestARitualDraftNeedsAKnownRitualType(t *testing.T) {
	a, p := newTestAppWithProfile(t)

	err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{BoardID: 1, SprintID: 12, RitualType: "party"})
	if err == nil {
		t.Fatal("expected an unknown ritual type to be refused")
	}
}

func TestDeletingARitualDraftLeavesItsSiblings(t *testing.T) {
	a, p := newTestAppWithProfile(t)

	for _, ritualType := range []string{"planning", "review"} {
		if err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{
			BoardID: 1, SprintID: 12, RitualType: ritualType,
		}); err != nil {
			t.Fatalf("save %s: %v", ritualType, err)
		}
	}
	if err := a.DeleteRitualDraft(p.ID, 1, 12, "review"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	drafts, err := a.ListRitualDrafts(p.ID, 1, 12)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(drafts) != 1 || drafts[0].RitualType != "planning" {
		t.Fatalf("drafts = %#v, want only planning", drafts)
	}
}
```

If `newTestAppWithProfile` does not exist, find the helper the other `tam/app_*_test.go` files use to build an `*App` with a seeded profile and reuse it by name rather than writing a second one.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tam && go test . -run TestSavingARitualDraftStoresItAgainstTheSprint -count=1`
Expected: FAIL to compile, `a.SaveRitualDraft undefined`.

- [ ] **Step 3: Hold the repository on App**

In `tam/app.go`, add the field to the `App` struct beside `boards` at line 44:

```go
	rituals   *ritualrepo.Repository
```

Wherever `a.boards = boardrepo.New(...)` is assigned after the local store opens, assign beside it:

```go
	a.rituals = ritualrepo.New(a.local.DB())
```

`tam/app.go` already imports `ritualrepo` at line 22 for the demo seed, so no import changes.

- [ ] **Step 4: Write the four methods**

Append to `tam/app_rituals.go`:

```go
// requireRituals guards the ritual document store the way requireStore guards
// the profile store: a profile that was never opened has no local database.
func (a *App) requireRituals() error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if a.rituals == nil {
		return errors.New("local ritual store not initialised")
	}
	return nil
}

// knownRitualType keeps an unrecognised type out of the table, because the
// composite primary key would otherwise accept any string and the view would
// grow a slot nothing can render.
func knownRitualType(ritualType string) bool {
	want := strings.TrimSpace(strings.ToLower(ritualType))
	for _, t := range ritualdefaults.Types {
		if t == want {
			return true
		}
	}
	return false
}

func (a *App) ListRitualDrafts(profileID string, boardID, sprintID int) ([]ritualrepo.Draft, error) {
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return nil, err
	}
	return a.rituals.ListDrafts(a.ctx, profileID, boardID, sprintID)
}

func (a *App) GetRitualDraft(profileID string, boardID, sprintID int, ritualType string) (ritualrepo.Draft, error) {
	if err := a.requireRituals(); err != nil {
		return ritualrepo.Draft{}, err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return ritualrepo.Draft{}, err
	}
	return a.rituals.Get(a.ctx, profileID, boardID, sprintID, ritualType)
}

// SaveRitualDraft writes the editable fields and nothing else. It always lands
// as a draft: publishing is a separate, deliberate press, and no save should
// ever reach Confluence.
func (a *App) SaveRitualDraft(profileID string, draft ritualrepo.Draft) error {
	if err := a.requireRituals(); err != nil {
		return err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return err
	}
	if !knownRitualType(draft.RitualType) {
		return errors.New("unknown ritual type")
	}
	if _, err := ritualrepo.DecodeIssues(draft.IssuesJSON); err != nil {
		return err
	}

	// The publication fields belong to the publish path, so they are carried
	// from the stored row rather than trusted from the caller.
	stored, err := a.rituals.Get(a.ctx, profileID, draft.BoardID, draft.SprintID, draft.RitualType)
	if err != nil {
		return err
	}
	draft.ProfileID = profileID
	draft.RitualType = strings.TrimSpace(strings.ToLower(draft.RitualType))
	draft.ConfluencePageID = stored.ConfluencePageID
	draft.ConfluenceVersion = stored.ConfluenceVersion
	draft.Body = stored.Body
	draft.PublishedAt = stored.PublishedAt
	draft.Status = "draft"
	draft.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(draft.IssuesJSON) == "" {
		draft.IssuesJSON = "[]"
	}
	return a.rituals.Upsert(a.ctx, draft)
}

// DeleteRitualDraft removes the local document only. Any Confluence page it was
// published to is left exactly where it is, because deleting a team's meeting
// notes is not a side effect a list tidy should have.
func (a *App) DeleteRitualDraft(profileID string, boardID, sprintID int, ritualType string) error {
	if err := a.requireRituals(); err != nil {
		return err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return err
	}
	return a.rituals.Delete(a.ctx, profileID, boardID, sprintID, ritualType)
}
```

Add `"agile-suite/tam/internal/ritualdefaults"` and `"agile-suite/tam/internal/ritualrepo"` to the imports of `tam/app_rituals.go`. `errors`, `strings` and `time` are already imported.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd tam && go test . -run TestRitual -count=1 -v && go test . -run "TestSavingARitual|TestDeletingARitual|TestARitualDraft" -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tam/app.go tam/app_rituals.go tam/app_rituals_test.go
git commit -m "feat(tam): the ritual store the app could not reach"
```

---

### Task 5: Scaffolding a sprint's four rituals

**Files:**
- Modify: `tam/app_rituals.go`
- Test: `tam/app_rituals_test.go`

**Interfaces:**
- Consumes: `ritualdefaults.Types`, `ritualdefaults.Select`, `ritualdefaults.Title` from Task 3; `a.repo.ListIssues(ctx, profileID, issuerepo.IssueQuery) (issuerepo.IssuePage, error)` (`tam/internal/issuerepo/issues.go:154`); `a.boards` for the sprint's name.
- Produces: `func (a *App) ScaffoldSprintRituals(profileID string, boardID, sprintID int) ([]ritualrepo.Draft, error)`

- [ ] **Step 1: Write the failing test**

Add to `tam/app_rituals_test.go`:

```go
func TestScaffoldingASprintCreatesTheFourRituals(t *testing.T) {
	a, p := newTestAppWithProfile(t)
	seedSprintIssues(t, a, p.ID, 12)

	drafts, err := a.ScaffoldSprintRituals(p.ID, 1, 12)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if len(drafts) != 4 {
		t.Fatalf("got %d drafts, want 4", len(drafts))
	}
	for _, d := range drafts {
		if d.Status != "draft" {
			t.Fatalf("%s status = %q, want draft", d.RitualType, d.Status)
		}
		if d.Title == "" {
			t.Fatalf("%s has no title", d.RitualType)
		}
	}
}

func TestScaffoldingTwiceDoesNotOverwriteEditedRituals(t *testing.T) {
	a, p := newTestAppWithProfile(t)
	seedSprintIssues(t, a, p.ID, 12)

	if _, err := a.ScaffoldSprintRituals(p.ID, 1, 12); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	if err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{
		BoardID: 1, SprintID: 12, RitualType: "review",
		Title: "Sprint 12 Review", Remark: "written by a human",
	}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if _, err := a.ScaffoldSprintRituals(p.ID, 1, 12); err != nil {
		t.Fatalf("second scaffold: %v", err)
	}

	got, err := a.GetRitualDraft(p.ID, 1, 12, "review")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Remark != "written by a human" {
		t.Fatalf("remark = %q; scaffolding must not overwrite an existing ritual", got.Remark)
	}
}
```

Add the seed helper to the same file:

```go
// seedSprintIssues puts four issues in a sprint so the defaults have something
// to choose between.
func seedSprintIssues(t *testing.T, a *App, profileID string, sprintID int) {
	t.Helper()
	issues := []backend.Issue{
		{Key: "PLAT-1", Project: "PLAT", Type: "story", Summary: "Checkout", Status: "To Do", SprintID: "12"},
		{Key: "PLAT-2", Project: "PLAT", Type: "story", Summary: "Payments", Status: "In Progress", SprintID: "12"},
		{Key: "PLAT-3", Project: "PLAT", Type: "bug", Summary: "Timeout", Status: "Done", SprintID: "12"},
		{Key: "PLAT-4", Project: "PLAT", Type: "story", Summary: "Search", Status: "Blocked", SprintID: "12"},
	}
	if err := a.repo.UpsertPage(a.ctx, profileID, issues, time.Now().UTC(), false); err != nil {
		t.Fatalf("seed issues: %v", err)
	}
}
```

`UpsertPage` is the only issue write on `issuerepo.Repository`
(`tam/internal/issuerepo/issues.go:123`); its signature is
`UpsertPage(ctx context.Context, profileID string, page []backend.Issue, syncedAt time.Time, clearFirst bool) error`.
Pass `false` for `clearFirst` so the seed adds to whatever the test app already
holds. Add `"time"` and `"agile-suite/tam/internal/backend"` to the test file's
imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tam && go test . -run TestScaffoldingASprintCreatesTheFourRituals -count=1`
Expected: FAIL to compile, `a.ScaffoldSprintRituals undefined`.

- [ ] **Step 3: Write the implementation**

Append to `tam/app_rituals.go`:

```go
// ScaffoldSprintRituals creates whatever of a sprint's four rituals does not
// exist yet, each with the issues its type asks for. It never touches a ritual
// that is already there: running it twice must be safe, because the button sits
// next to documents somebody has been editing.
func (a *App) ScaffoldSprintRituals(profileID string, boardID, sprintID int) ([]ritualrepo.Draft, error) {
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return nil, err
	}

	page, err := a.repo.ListIssues(a.ctx, profileID, issuerepo.IssueQuery{
		SprintID: strconv.Itoa(sprintID),
		Limit:    500,
	})
	if err != nil {
		return nil, err
	}

	// A sprint whose name cannot be read still gets its rituals, titled by
	// ritual alone, because a scaffold must not fail on a cosmetic lookup.
	sprintName, err := a.boards.SprintName(a.ctx, profileID, strconv.Itoa(sprintID))
	if err != nil {
		sprintName = ""
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, ritualType := range ritualdefaults.Types {
		existing, err := a.rituals.Get(a.ctx, profileID, boardID, sprintID, ritualType)
		if err != nil {
			return nil, err
		}
		if existing.RitualType != "" {
			continue
		}
		issues, err := ritualrepo.EncodeIssues(ritualdefaults.Select(ritualType, page.Issues))
		if err != nil {
			return nil, err
		}
		if err := a.rituals.Upsert(a.ctx, ritualrepo.Draft{
			ProfileID:  profileID,
			BoardID:    boardID,
			SprintID:   sprintID,
			RitualType: ritualType,
			Title:      ritualdefaults.Title(ritualType, sprintName),
			IssuesJSON: issues,
			Status:     "draft",
			UpdatedAt:  now,
		}); err != nil {
			return nil, err
		}
	}
	return a.rituals.ListDrafts(a.ctx, profileID, boardID, sprintID)
}
```

Add `"strconv"` and `"agile-suite/tam/internal/issuerepo"` to the imports.

`SprintName` is at `tam/internal/boardrepo/boards.go:154` with the signature
`SprintName(ctx context.Context, profileID, sprintID string) (string, error)`.
It takes the sprint id as a string and does not take a board id, which is why
`strconv.Itoa` appears twice in this function.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd tam && go test . -run TestScaffolding -count=1 -v`
Expected: PASS, both tests.

- [ ] **Step 5: Commit**

```bash
git add tam/app_rituals.go tam/app_rituals_test.go
git commit -m "feat(tam): a sprint's rituals, drawn up in one press"
```

---

### Task 6: A deleted profile takes its rituals with it

**Files:**
- Modify: `tam/internal/boardrepo/boardrepo.go:84`
- Test: `tam/internal/boardrepo/boardrepo_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `PurgeProfile` also clears `ritual_document`.

The Phase 4 plan required every new table to join this list, and the comment on `PurgeProfile` predicted this exact failure. `ritual_document` arrived at version 9 without joining it.

- [ ] **Step 1: Write the failing test**

Add to `tam/internal/boardrepo/boardrepo_test.go`:

```go
func TestPurgeProfileClearsRitualDocuments(t *testing.T) {
	ctx := context.Background()
	r, db := newTestRepoWithDB(t)

	for _, profileID := range []string{"p1", "p2"} {
		if _, err := db.Exec(`INSERT INTO ritual_document
			(profile_id, board_id, sprint_id, ritual_type, status)
			VALUES (?, 1, 12, 'review', 'draft')`, profileID); err != nil {
			t.Fatalf("seed %s: %v", profileID, err)
		}
	}

	if err := r.PurgeProfile(ctx, "p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ritual_document WHERE profile_id = 'p1'`).Scan(&remaining); err != nil {
		t.Fatalf("count p1: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("p1 left %d ritual rows behind", remaining)
	}

	var untouched int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ritual_document WHERE profile_id = 'p2'`).Scan(&untouched); err != nil {
		t.Fatalf("count p2: %v", err)
	}
	if untouched != 1 {
		t.Fatalf("p2 has %d ritual rows, want 1", untouched)
	}
}
```

Follow whatever helper `TestPurgeProfileClearsTheFourBoardTables` at line 259 uses to get a repository and a handle; if it only returns the repository, add a sibling helper that also returns the `*sql.DB` rather than changing the existing one.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tam && go test ./internal/boardrepo/ -run TestPurgeProfileClearsRitualDocuments -count=1`
Expected: FAIL with `p1 left 1 ritual rows behind`.

- [ ] **Step 3: Add the table to the purge list**

At `tam/internal/boardrepo/boardrepo.go:84`, extend the slice:

```go
		for _, table := range []string{"board", "board_column", "board_issue", "sprint", "sprint_report", "ritual_document"} {
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd tam && go test ./internal/boardrepo/ -count=1`
Expected: PASS, including the existing four-table test.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/boardrepo/
git commit -m "fix(tam): a deleted profile takes its rituals with it"
```

---

### Task 7: The frontend can reach a draft

**Files:**
- Modify: `tam/frontend/src/api.ts`
- Regenerate: `tam/frontend/wailsjs/**` via `wails generate module`
- Test: `tam/frontend/src/queries/rituals.test.tsx` (create)

**Interfaces:**
- Consumes: the five App methods from Tasks 4 and 5.
- Produces, from `src/api.ts`:
  - `export interface RitualIssue { key: string; remark: string }`
  - `export interface RitualDraft { profileId: string; boardId: number; sprintId: number; ritualType: string; title: string; remark: string; body: string; issuesJson: string; confluencePageId: string; confluenceVersion: number; status: string; updatedAt: string; publishedAt: string }`
  - `ListRitualDrafts`, `GetRitualDraft`, `SaveRitualDraft`, `DeleteRitualDraft`, `ScaffoldSprintRituals`
  - `parseRitualIssues(json: string): RitualIssue[]` and `encodeRitualIssues(issues: RitualIssue[]): string`

- [ ] **Step 1: Regenerate the bindings**

Run: `cd tam && wails generate module`

This rewrites `tam/frontend/wailsjs/go/main/App.js`, `App.d.ts` and `models.ts`. Do not hand-edit them. Confirm the new methods appear:

Run: `grep -c "RitualDraft\|ScaffoldSprintRituals" tam/frontend/wailsjs/go/main/App.d.ts`
Expected: at least `5`.

- [ ] **Step 2: Write the failing test**

Create `tam/frontend/src/queries/rituals.test.tsx`:

```tsx
import { describe, expect, it } from "vitest";
import { encodeRitualIssues, parseRitualIssues } from "../api";

describe("ritual issues", () => {
  it("round-trips keys and remarks in order", () => {
    const issues = [
      { key: "PLAT-14", remark: "demoed" },
      { key: "PLAT-22", remark: "blocked on infra" },
    ];
    expect(parseRitualIssues(encodeRitualIssues(issues))).toEqual(issues);
  });

  it("reads an empty column as no issues", () => {
    expect(parseRitualIssues("")).toEqual([]);
    expect(parseRitualIssues("[]")).toEqual([]);
  });

  it("returns no issues rather than throwing on malformed stored data", () => {
    expect(parseRitualIssues("{not json")).toEqual([]);
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd tam/frontend && npx vitest run src/queries/rituals.test.tsx`
Expected: FAIL, `parseRitualIssues is not exported`.

- [ ] **Step 4: Add the types and wrappers**

In `tam/frontend/src/api.ts`, beside the existing `RitualAssociation` interface at line 45:

```ts
export interface RitualIssue { key: string; remark: string }
export type RitualDraft = ritualrepo.Draft;

// parseRitualIssues reads the issues_json column. Stored data that cannot be
// parsed yields no issues rather than throwing, because a draft with a damaged
// column must still open in the wizard to be repaired.
export function parseRitualIssues(json: string): RitualIssue[] {
  const trimmed = (json ?? "").trim();
  if (!trimmed) return [];
  try {
    const parsed = JSON.parse(trimmed);
    if (!Array.isArray(parsed)) return [];
    return parsed.map((i) => ({ key: String(i?.key ?? ""), remark: String(i?.remark ?? "") })).filter((i) => i.key);
  } catch {
    return [];
  }
}

export function encodeRitualIssues(issues: RitualIssue[]): string {
  return JSON.stringify(issues.map((i) => ({ key: i.key, remark: i.remark ?? "" })));
}
```

Beside the existing ritual exports at line 1088:

```ts
export const ListRitualDrafts: (profileId: string, boardId: number, sprintId: number) => Promise<RitualDraft[]> = App.ListRitualDrafts as any;
export const GetRitualDraft: (profileId: string, boardId: number, sprintId: number, ritualType: string) => Promise<RitualDraft> = App.GetRitualDraft as any;
export const SaveRitualDraft = (profileId: string, draft: RitualDraft): Promise<void> =>
  App.SaveRitualDraft(profileId, ritualrepo.Draft.createFrom(draft));
export const DeleteRitualDraft: (profileId: string, boardId: number, sprintId: number, ritualType: string) => Promise<void> = App.DeleteRitualDraft as any;
export const ScaffoldSprintRituals: (profileId: string, boardId: number, sprintId: number) => Promise<RitualDraft[]> = App.ScaffoldSprintRituals as any;
```

Add `ritualrepo` to the model import at the top of `api.ts`, alongside `confluence` and `profile`. The exact namespace name is whatever `wails generate module` emitted in `models.ts` for the `ritualrepo` package; check it rather than assuming.

- [ ] **Step 5: Run tests and typecheck**

Run: `cd tam/frontend && npx vitest run src/queries/rituals.test.tsx && npx tsc --noEmit`
Expected: PASS, and typecheck exits 0.

- [ ] **Step 6: Commit**

```bash
git add tam/frontend/src/api.ts tam/frontend/src/queries/rituals.test.tsx tam/frontend/wailsjs/
git commit -m "feat(tam): the binding a ritual draft needs"
```

---

### Task 8: Slots and the wizard

**Files:**
- Create: `tam/frontend/src/components/RitualWizard.tsx`
- Modify: `tam/frontend/src/components/RitualsView.tsx`, `tam/frontend/src/App.css`
- Test: `tam/frontend/src/components/RitualWizard.test.tsx` (create), `tam/frontend/src/components/RitualsView.test.tsx` (create if absent)

**Interfaces:**
- Consumes: everything from Task 7.
- Produces: `export function RitualWizard({ profileId, boardId, sprintId, ritualType, sprintIssues, onSaved, onCancel })`

- [ ] **Step 1: Write the failing tests**

Create `tam/frontend/src/components/RitualWizard.test.tsx`:

```tsx
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "../api";
import { RitualWizard } from "./RitualWizard";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetRitualDraft: vi.fn(), SaveRitualDraft: vi.fn() };
});

const ISSUES = [
  { key: "PLAT-1", summary: "Checkout", status: "To Do" },
  { key: "PLAT-3", summary: "Timeout", status: "Done" },
];

function renderWizard(ritualType = "review") {
  return render(
    <RitualWizard
      profileId="p1"
      boardId={1}
      sprintId={12}
      ritualType={ritualType}
      sprintIssues={ISSUES as never}
      onSaved={vi.fn()}
      onCancel={vi.fn()}
    />,
  );
}

beforeEach(() => {
  vi.mocked(api.GetRitualDraft).mockResolvedValue({
    profileId: "p1", boardId: 1, sprintId: 12, ritualType: "review",
    title: "Sprint 12 Review", remark: "", body: "", issuesJson: '[{"key":"PLAT-3","remark":""}]',
    confluencePageId: "", confluenceVersion: 0, status: "draft", updatedAt: "", publishedAt: "",
  } as never);
  vi.mocked(api.SaveRitualDraft).mockResolvedValue();
});

describe("RitualWizard", () => {
  it("opens on the stored selection rather than the whole sprint", async () => {
    renderWizard();
    const selected = await screen.findByRole("checkbox", { name: /PLAT-3/ });
    expect(selected).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /PLAT-1/ })).not.toBeChecked();
  });

  it("saves the author remark and a remark for each chosen issue", async () => {
    const user = userEvent.setup();
    renderWizard();

    await user.type(await screen.findByLabelText("Remark"), "short sprint");
    const row = screen.getByRole("group", { name: /PLAT-3/ });
    await user.type(within(row).getByLabelText("Issue remark"), "demoed");
    await user.click(screen.getByRole("button", { name: "Save draft" }));

    expect(api.SaveRitualDraft).toHaveBeenCalled();
    const draft = vi.mocked(api.SaveRitualDraft).mock.calls[0][1];
    expect(draft.remark).toBe("short sprint");
    expect(JSON.parse(draft.issuesJson)).toEqual([{ key: "PLAT-3", remark: "demoed" }]);
  });

  it("does not publish when saving", async () => {
    const user = userEvent.setup();
    renderWizard();
    await user.click(await screen.findByRole("button", { name: "Save draft" }));
    expect(screen.queryByRole("button", { name: /Publish/ })).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd tam/frontend && npx vitest run src/components/RitualWizard.test.tsx`
Expected: FAIL, cannot resolve `./RitualWizard`.

- [ ] **Step 3: Write the wizard**

Create `tam/frontend/src/components/RitualWizard.tsx`:

```tsx
import { useEffect, useState } from "react";
import type { Issue } from "../api";
import { encodeRitualIssues, GetRitualDraft, parseRitualIssues, RitualIssue, SaveRitualDraft } from "../api";
import type { RitualDraft } from "../api";

// RitualWizard edits one ritual document. It saves a draft and never publishes:
// reaching Confluence is a separate, deliberate press elsewhere, so nobody
// pushes a page by finishing a form.
export function RitualWizard({
  profileId, boardId, sprintId, ritualType, sprintIssues, onSaved, onCancel,
}: {
  profileId: string; boardId: number; sprintId: number; ritualType: string;
  sprintIssues: Issue[]; onSaved: () => void; onCancel: () => void;
}) {
  const [draft, setDraft] = useState<RitualDraft | null>(null);
  const [remark, setRemark] = useState("");
  const [chosen, setChosen] = useState<RitualIssue[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let live = true;
    void GetRitualDraft(profileId, boardId, sprintId, ritualType)
      .then((d) => {
        if (!live) return;
        setDraft(d);
        setRemark(d.remark ?? "");
        setChosen(parseRitualIssues(d.issuesJson));
      })
      .catch((e) => live && setError(String(e)));
    return () => { live = false; };
  }, [profileId, boardId, sprintId, ritualType]);

  function toggle(key: string) {
    setChosen((current) =>
      current.some((i) => i.key === key)
        ? current.filter((i) => i.key !== key)
        : [...current, { key, remark: "" }]);
  }

  function setIssueRemark(key: string, value: string) {
    setChosen((current) => current.map((i) => (i.key === key ? { ...i, remark: value } : i)));
  }

  function save() {
    if (!draft) return;
    setSaving(true); setError("");
    void SaveRitualDraft(profileId, { ...draft, remark, issuesJson: encodeRitualIssues(chosen) })
      .then(onSaved)
      .catch((e) => setError(String(e)))
      .finally(() => setSaving(false));
  }

  if (!draft) return <p className="muted">{error || "Loading the ritual..."}</p>;

  return (
    <section className="ritual-wizard" aria-label={`Edit ${ritualType}`}>
      <h3>{draft.title || ritualType}</h3>

      <label className="field">
        <span>Remark</span>
        <textarea value={remark} onChange={(e) => setRemark(e.target.value)} rows={3} />
      </label>

      <h4>Issues</h4>
      <ul className="ritual-issues">
        {sprintIssues.map((issue) => {
          const picked = chosen.find((i) => i.key === issue.key);
          return (
            <li key={issue.key} role="group" aria-label={`${issue.key} ${issue.summary}`}>
              <label>
                <input type="checkbox" checked={!!picked} onChange={() => toggle(issue.key)} />
                <span>{issue.key} {issue.summary}</span>
                <span className="muted small">{issue.status}</span>
              </label>
              {picked && (
                <label className="field">
                  <span>Issue remark</span>
                  <input value={picked.remark} onChange={(e) => setIssueRemark(issue.key, e.target.value)} />
                </label>
              )}
            </li>
          );
        })}
      </ul>

      {error && <p className="warn-text small">{error}</p>}
      <div className="row">
        <button onClick={save} disabled={saving}>Save draft</button>
        <button className="ghost" onClick={onCancel}>Cancel</button>
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd tam/frontend && npx vitest run src/components/RitualWizard.test.tsx`
Expected: PASS, all three tests.

- [ ] **Step 5: Add the slots to RitualsView**

In `tam/frontend/src/components/RitualsView.tsx`, alongside the existing association list, render one slot per ritual type for the selected sprint. Load them with `ListRitualDrafts`, offer `ScaffoldSprintRituals` when none exist, and open `RitualWizard` for a slot the user picks.

Keep the existing association path untouched: linking a page somebody wrote by hand is a different job from authoring one, and Phase 5 shipped it.

Add to `tam/frontend/src/App.css`, beside the existing `.report-*` rules:

```css
.ritual-slots { display: grid; gap: var(--gap-2); }
.ritual-slot { display: flex; justify-content: space-between; align-items: center; gap: var(--gap-2); }
.ritual-slot .status { font-size: 0.85em; opacity: 0.75; }
.ritual-wizard .ritual-issues { list-style: none; padding: 0; display: grid; gap: var(--gap-1); }
```

If those custom properties are not what `App.css` uses, match whatever the neighbouring `.report-*` rules use rather than introducing new names.

- [ ] **Step 6: Test the slots**

Add to `tam/frontend/src/components/RitualsView.test.tsx`, mocking `ListRitualDrafts` and `ScaffoldSprintRituals` the way the wizard test mocks its two:

```tsx
it("offers to draw up the sprint's rituals when none exist", async () => {
  vi.mocked(api.ListRitualDrafts).mockResolvedValue([]);
  renderView();
  expect(await screen.findByRole("button", { name: /Set up .* rituals/ })).toBeInTheDocument();
});

it("shows a slot per stored ritual with its status", async () => {
  vi.mocked(api.ListRitualDrafts).mockResolvedValue([
    { ritualType: "planning", title: "Sprint 12 Planning", status: "draft" },
    { ritualType: "review", title: "Sprint 12 Review", status: "draft" },
  ] as never);
  renderView();
  expect(await screen.findByText("Sprint 12 Planning")).toBeInTheDocument();
  expect(screen.getAllByText("draft")).toHaveLength(2);
});
```

- [ ] **Step 7: Run the whole gate**

```bash
cd core && go vet ./... && go test ./... -count=1 && cd ..
cd xtm && go vet ./... && go test ./internal/... -count=1 && cd ..
cd tam && go vet ./... && go test ./... -count=1 && cd ..
npm run typecheck --workspaces --if-present
npm test --workspaces --if-present
cd tam && wails build && cd ..
git status --short --untracked-files=no
```

Expected: every suite green, typecheck clean, build succeeds. Frontend counts must be at or above 46, 159 and 485. Do not lower an assertion to make a test pass.

- [ ] **Step 8: Commit**

```bash
git add tam/frontend/src/components/ tam/frontend/src/App.css
git commit -m "feat(tam): rituals a sprint can be walked through"
```

---

## What part 2 covers

Publishing, and nothing in this plan should anticipate it. Part 2 begins with the marker probe the spec requires, then adds `CreatePage` and `UpdatePage` to `core/confluence`, journals a publish as one `PendingChange` per changed block, and teaches the committer to group a ritual's blocks into a single page write with `BaseVersion + 1`.

Two seams here exist for it and are deliberately inert until then: `Draft.Body` is written by nothing and reserved for the last published render, and `status` only ever holds `"draft"`.
