# Task Activity Manager issues, plan 1c: the write features

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** TAM imports issues from an Excel or CSV file as drafts, creates cross-project links from the Links tab through the journal, and creates requirements, all pushed to Jira by the existing Commit.

**Architecture:** `core/importfile` is XTM's file parser lifted out with XTM delegating. `tam/internal/importer` maps columns to draft fields, validates rows, and creates drafts in one transaction through a new `CreateDrafts`. A pending link is a journal row of entity type `link` whose JSON the committer pushes with `POST /rest/api/2/issueLink` after edits; the repository merges pending links into the detail cache so the Links tab shows them. The requirement type joins the creatable set; the backends and the New issue dialog already handle it. Frontend: an Import issues dialog, an Add link form and pending links on the Links tab, link cards in the Pending changes dialog, link wording on the Activity tab.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, `github.com/xuri/excelize/v2` v2.10.1, React 19, TanStack Query 5, Vite 8, Vitest 4, npm workspaces.

**Spec:** [`../specs/2026-09-05-tam-issues-design.md`](../specs/2026-09-05-tam-issues-design.md), section 14. **Mockup:** [`../specs/assets/2026-09-06-tam-import-and-links.svg`](../specs/assets/2026-09-06-tam-import-and-links.svg).

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, `agile-suite/tam`; each app module carries `replace agile-suite/core => ../core`. Run Go commands from inside the module directory.
- XTM is edited only in Task 1: in `xtm/internal/testrepo/importcsv.go`, `ParseRecords`, `ParseImportPreview`, `readCSV`, `parseXLSX`, and `stripUTF8BOM` become one-line delegators to `core/importfile`, `ImportPreview` becomes a type alias of `importfile.Preview`, and the `utf8BOMBytes` variable goes. Every error string XTM produced stays byte-identical. Nothing else under `xtm/` changes. Task 1 lands as its own PR gated by XTM's Go suite (`go test ./internal/...` inside `xtm/`) and XTM's Vitest suite (159 tests).
- Every later task leaves XTM's Go suite, every Vitest workspace, and `npm run typecheck --workspaces --if-present` green. `frontend/core` is not edited by this plan.
- Import produces drafts only; it never talks to Jira. A dry run validates and creates nothing. An import creates every valid row's draft in one transaction and skips the rows with errors, listing them. Row numbers in errors are file rows (the header is row 1, the first data row is row 2).
- Import fields are exactly type, summary, description, priority, labels, assignee, story points, parent key. Type blank or unmapped means task; otherwise task, story, bug, requirement, or the profile's requirement type name, case-insensitively. A parent key must exist in the profile's cache.
- Creatable types are now `task`, `story`, `bug`, `requirement`. The requirement draft is created under the profile's `requirement_issue_type` name (the backend's `jiraTypeNames` already maps it). Story points are hidden for requirements in the New issue dialog.
- A pending link is a journal row: entity type `link`, `entity_key` the source issue, `field` `<type name>|<direction>|<target key>`, `after_val` the link as JSON; direction is `outward` or `inward`. Links have no base version and never conflict. The Jira POST puts the source as `outwardIssue` for an outward link and as `inwardIssue` for an inward one.
- Link creation only. No link removal, no links in the bulk sync, no epic creation, no subtask parents.
- Bound method signatures (Task 5) are exactly: `PreviewImport(contentB64 string, isXlsx bool) (importfile.Preview, error)`, `AutoMapImport(headers []string) importer.Mapping`, `ImportIssues(profileID, contentB64 string, isXlsx bool, fileName string, mapping importer.Mapping, dryRun bool) (importer.Result, error)`, `SaveImportTemplate() (string, error)`, `GetLinkTypes(profileID string) ([]backend.LinkType, error)`, `LookupIssue(profileID, key string) (backend.Issue, error)`, `AddLink(profileID, key string, link backend.LinkDraft) error`. Import runs under the `busy` guard as `"import"`, so it and Commit and Sync exclude each other.
- The PAT stays in the Jira client's Authorization header only. No token in SQLite, logs, or JSON to the frontend.
- UI text uses no em dashes. No AI attribution or mentions in any commit message, PR, file, or comment. Run the humanizer pass over prose, including code comments.
- Commit messages use the repo's conventional prefixes with a scope where one applies, and no trailers.
- The working tree holds untracked local tooling files (`agentdb.rvf`, `.claude/`, `reference/`, `.superpowers/` and similar). Never add, commit, or delete them. Check state with `git status --short --untracked-files=no`. Wails may rewrite line endings under `tam/frontend/wailsjs/runtime`, `tam/frontend/package.json.md5`, and `tam/go.mod`; revert those with `git checkout --`.

## Decisions

1. **`core/importfile` exports the three parser pieces XTM used privately** (`StripUTF8BOM`, `ReadCSV`, `ParseXLSX`) besides `ParseRecords` and `ParsePreview`, so XTM's private helpers can delegate one-for-one and the package is testable piece by piece.
2. **`CreateDrafts` is the transaction; `CreateDraft` calls it with one draft.** The importer validates every row itself, then hands the valid drafts to `CreateDrafts`, which validates again (cheaply) and inserts them all or nothing. The audit note carries the file name.
3. **The parent key goes to Jira through the Epic Link custom field** when discovered and the draft is not an epic; otherwise it is dropped from the create with a log line. Jira DC has no `parent` on standard issue types; subtasks are out of scope.
4. **Link rows are pushed in their own pass after creates and edits**, read fresh from the journal, so a link added from a draft is pushed under the real key (`Rekey` moves its journal row). `commitCreate` marks only the create row committed, never the link rows.
5. **`Link` gains `Pending` and `PendingID`.** The Links tab discards a pending link with the existing `DiscardPendingChange(id)`; `PendingLinks` fills the id from the journal row.
6. **`LookupIssue` is `GetIssue` on the backend** (any project key). The demo answers it for the `XT-` keys its curated details reference with synthetic rows (type empty, status Done), so a cross-project link can be checked and created offline.
7. **File contents travel base64** from a browser file input, the way XTM's `PreviewImport` works, and the template is saved through the Wails save dialog; no Wails open dialog is needed.
8. **Duplicate links are refused at Add time**: a pending row with the same field, or an existing `issue_link` row with the same type, direction, and target, returns an error the form shows.
9. **The demo's `CreateFields` for requirements asks for one required string, Source**, so the New issue dialog's create-meta path can be seen for the new type offline.

## File structure

**Created**

- `core/importfile/importfile.go`, `core/importfile/importfile_test.go` (Task 1).
- `tam/internal/importer/importer.go`, `importer_test.go` (Task 4).
- `tam/frontend/src/components/ImportIssuesModal.tsx`, `ImportIssuesModal.test.tsx` (Task 6).
- `tam/frontend/src/components/AddLinkForm.tsx`, `AddLinkForm.test.tsx` (Task 7).

**Modified**

- `xtm/internal/testrepo/importcsv.go`, `core/go.mod`, `core/go.sum`, possibly `go.work.sum` (Task 1).
- `tam/internal/backend/backend.go`; `tam/internal/issuerepo/writes.go`, `writes_test.go`; `tam/internal/backend/jira/writes.go`, `writes_test.go`, `jira_test.go`; `tam/internal/backend/demo/demo.go`, `demo_test.go` (Task 2).
- `tam/internal/backend/backend.go`; `tam/internal/issuerepo/pending.go`, `links.go` (new), `links_test.go` (new), `detail.go`, `writes.go`; `tam/internal/backend/jira/links.go` (new), `links_test.go` (new), `jira_test.go`; `tam/internal/backend/demo/demo.go`, `demo_test.go`; `tam/internal/demo/demo.go`; `tam/internal/syncer/syncer_test.go`; `tam/internal/committer/committer_test.go` (Task 3).
- `tam/internal/committer/committer.go`, `committer_test.go`; `tam/app_writes.go`, `tam/app_imports.go` (new); `tam/frontend/wailsjs/**` regenerated (Task 5).
- `tam/frontend/src/api.ts`, `modals.ts`, `components/BacklogView.tsx`, `BacklogView.test.tsx`, `App.test.tsx`, `App.css` (Task 6).
- `tam/frontend/src/api.ts`, `queries/keys.ts`, `queries/pending.ts`, `components/IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx`, `PendingChangesModal.tsx`, `PendingChangesModal.test.tsx`, `ActivityTab.tsx`, `NewIssueModal.tsx`, `NewIssueModal.test.tsx`, `App.css` (Task 7).
- `tam/CLAUDE.md`, `README.md`, the spec's 14.11 (Task 8).

---

### Task 1: `core/importfile`, lifted from XTM

Move the CSV and XLSX parsing XTM's importer uses into a shared package; XTM delegates. This task is its own PR.

**Files:**
- Create: `core/importfile/importfile.go`, `core/importfile/importfile_test.go`
- Modify: `core/go.mod`, `core/go.sum` (excelize), `xtm/internal/testrepo/importcsv.go`
- Test: `go test ./importfile/` inside `core/`; `go build ./... && go vet ./... && go test ./internal/... -count=1` inside `xtm/`

**Interfaces:**
- Produces (package `agile-suite/core/importfile`): `type Preview struct{Headers []string; RowCount int}` (JSON `headers`, `rowCount`), `ParsePreview(records [][]string) (Preview, error)`, `ParseRecords(data []byte, isXlsx bool) ([][]string, error)`, `StripUTF8BOM(data []byte) []byte`, `ReadCSV(content string) ([][]string, error)`, `ParseXLSX(data []byte) ([][]string, error)`.

- [ ] **Step 1: Write the failing tests**

Create `core/importfile/importfile_test.go`:

```go
package importfile_test

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"agile-suite/core/importfile"
)

func TestParseRecordsCSVStripsTheBOMAndAllowsRaggedRows(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("Summary,Type\nFirst,Task\nSecond\n")...)
	recs, err := importfile.ParseRecords(data, false)
	if err != nil {
		t.Fatalf("ParseRecords: %v", err)
	}
	if len(recs) != 3 || recs[0][0] != "Summary" || len(recs[2]) != 1 || recs[2][0] != "Second" {
		t.Errorf("records = %v", recs)
	}
}

func TestParseRecordsXLSXReadsTheFirstSheet(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "Summary")
	_ = f.SetCellValue("Sheet1", "B1", "Points")
	_ = f.SetCellValue("Sheet1", "A2", "Add a retry")
	_ = f.SetCellValue("Sheet1", "B2", 3)
	_, _ = f.NewSheet("Other")
	_ = f.SetCellValue("Other", "A1", "ignored")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	recs, err := importfile.ParseRecords(buf.Bytes(), true)
	if err != nil {
		t.Fatalf("ParseRecords xlsx: %v", err)
	}
	if len(recs) != 2 || recs[0][1] != "Points" || recs[1][0] != "Add a retry" || recs[1][1] != "3" {
		t.Errorf("records = %v", recs)
	}
	if _, err := importfile.ParseRecords([]byte("not a workbook"), true); err == nil || !strings.Contains(err.Error(), "open xlsx") {
		t.Errorf("bad xlsx: %v", err)
	}
}

func TestParsePreview(t *testing.T) {
	pv, err := importfile.ParsePreview([][]string{{"A", "B"}, {"1", "2"}, {"3"}})
	if err != nil || len(pv.Headers) != 2 || pv.RowCount != 2 {
		t.Errorf("preview = %+v %v", pv, err)
	}
	if _, err := importfile.ParsePreview(nil); err == nil || err.Error() != "the file is empty" {
		t.Errorf("empty: %v", err)
	}
}
```

- [ ] **Step 2: Run them to see the failure**

Run (inside `core/`): `go test ./importfile/`
Expected: FAIL, the package does not exist (and excelize is not yet a dependency of `core`).

- [ ] **Step 3: The dependency and the package**

Inside `core/`: `go get github.com/xuri/excelize/v2@v2.10.1` (the version XTM pins). Then create `core/importfile/importfile.go`; the bodies are XTM's, error strings included:

```go
// Package importfile turns an uploaded CSV or XLSX file into rows of cells,
// the first step of every spreadsheet import in the suite. It was lifted
// from XTM's testrepo importer, which now delegates here; mapping columns
// to fields and validating rows stays with each app.
package importfile

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Preview is what a freshly parsed file looks like before mapping: its
// column headers and the number of data rows.
type Preview struct {
	Headers  []string `json:"headers"`
	RowCount int      `json:"rowCount"`
}

// ParsePreview reads the header row and counts the data rows.
func ParsePreview(records [][]string) (Preview, error) {
	if len(records) == 0 {
		return Preview{}, fmt.Errorf("the file is empty")
	}
	return Preview{Headers: records[0], RowCount: len(records) - 1}, nil
}

// ParseRecords parses raw file bytes into rows, CSV or XLSX. For XLSX the
// first worksheet is used.
func ParseRecords(data []byte, isXlsx bool) ([][]string, error) {
	if isXlsx {
		return ParseXLSX(data)
	}
	return ReadCSV(string(StripUTF8BOM(data)))
}

// utf8BOM is the UTF-8 byte-order mark (EF BB BF) that Excel and Windows
// editors prepend to saved CSVs. Left in place it fuses onto the first
// header cell, so column auto-mapping no longer recognizes "Summary".
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// StripUTF8BOM removes a leading UTF-8 BOM so the first column header maps
// cleanly.
func StripUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, utf8BOM)
}

// ReadCSV parses CSV content leniently (variable field counts allowed).
func ReadCSV(content string) ([][]string, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	return records, nil
}

// ParseXLSX reads the first worksheet of an XLSX file into rows.
func ParseXLSX(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("the workbook has no sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("read xlsx rows: %w", err)
	}
	return rows, nil
}
```

Run `go mod tidy` inside `core/` and check `git status --short --untracked-files=no`: `core/go.mod` and `core/go.sum` change; if `go.work.sum` at the root changes too, it is part of this commit.

- [ ] **Step 4: Run the core tests**

Run (inside `core/`): `go vet ./importfile/ && go test ./importfile/ -v`
Expected: PASS, three tests.

- [ ] **Step 5: XTM delegates**

In `xtm/internal/testrepo/importcsv.go`, add the import `"agile-suite/core/importfile"`. Replace the `ImportPreview` type with an alias, keeping its doc comment:

```go
// ImportPreview is what a freshly-parsed import file looks like before mapping
// (FR-10.5): its column headers and the number of data rows.
type ImportPreview = importfile.Preview
```

Replace the bodies of the five functions with delegators, keeping their signatures and doc comments:

```go
func ParseImportPreview(records [][]string) (ImportPreview, error) {
	return importfile.ParsePreview(records)
}
```

```go
func ParseRecords(data []byte, isXlsx bool) ([][]string, error) {
	return importfile.ParseRecords(data, isXlsx)
}
```

```go
func stripUTF8BOM(data []byte) []byte { return importfile.StripUTF8BOM(data) }
```

```go
func readCSV(content string) ([][]string, error) { return importfile.ReadCSV(content) }
```

```go
func parseXLSX(data []byte) ([][]string, error) { return importfile.ParseXLSX(data) }
```

Delete the `utf8BOMBytes` variable and its comment (the package now owns it). Then run `go build ./...` inside `xtm/`; remove whichever of `bytes`, `encoding/csv`, and `github.com/xuri/excelize/v2` the compiler reports as unused in `importcsv.go` (other files in the package may still use them; only remove from this file's import block). XTM's `go.mod` keeps excelize because other XTM packages use it; do not run `go mod tidy` in `xtm/`.

- [ ] **Step 6: Prove XTM did not move**

Run, inside `xtm/`:

```bash
go build ./... && go vet ./... && go test ./internal/... -count=1
```

Expected: every package `ok`; `TestParseImportPreviewReportsHeadersAndRows` and the gap-analysis tests that call `ParseRecords` pass unchanged. From the repo root, `npm test --workspace xtm/frontend 2>&1 | grep "Tests "` still reports 159 passing.

- [ ] **Step 7: Commit and open the PR**

```bash
git add core/importfile core/go.mod core/go.sum xtm/internal/testrepo/importcsv.go
git status --short --untracked-files=no
```

If `go.work.sum` is listed as modified, add it too. Then:

```bash
git commit -m "refactor(core): lift the spreadsheet parser into core/importfile"
```

Push the branch and open a PR titled "Lift the spreadsheet parser into core/importfile" whose description says the five XTM functions now delegate with their error strings unchanged, that `ImportPreview` is an alias, and which XTM suites were run. The remaining tasks continue on a branch from this one and do not wait for the merge, but this PR merges first.

---

### Task 2: Drafts in bulk, the parent key, and the requirement type

`CreateDrafts` inserts many drafts in one transaction with an audit note; drafts carry a parent key that the Jira backend sends through Epic Link; requirements become creatable.

**Files:**
- Modify: `tam/internal/backend/backend.go` (`IssueDraft.ParentKey`)
- Modify: `tam/internal/issuerepo/writes.go` (`draftTypes`, `CreateDrafts`, `CreateDraft`), `tam/internal/issuerepo/writes_test.go`
- Modify: `tam/internal/backend/jira/writes.go` (`CreateIssue`), `tam/internal/backend/jira/writes_test.go`, `tam/internal/backend/jira/jira_test.go` (a field list with Epic Link)
- Modify: `tam/internal/backend/demo/demo.go` (`CreateIssue`, `CreateFields`), `tam/internal/backend/demo/demo_test.go`
- Test: `go test ./internal/issuerepo/ ./internal/backend/... -count=1` inside `tam/`

**Interfaces:**
- Consumes: `issuerepo.CreateDraft`, `journal.Put`, `journal.Audit`, `fieldIDs.EpicLink` (already discovered by the Jira backend), `jiraTypeNames`.
- Produces: `backend.IssueDraft.ParentKey string` (JSON `parentKey`); `(*Repository).CreateDrafts(ctx, profileID, projectKey string, drafts []backend.IssueDraft, note string) ([]string, error)`; `CreateDraft` unchanged in signature, now accepting `requirement`; demo `CreateFields(ctx, _, "requirement")` returns the Source field.

- [ ] **Step 1: The draft's parent**

In `tam/internal/backend/backend.go`, add to `IssueDraft` after `StoryPoints`:

```go
	// ParentKey is the epic (or parent) the draft belongs under. The Jira
	// backend sends it through the Epic Link field when it exists.
	ParentKey string `json:"parentKey"`
```

- [ ] **Step 2: Repository tests**

Append to `tam/internal/issuerepo/writes_test.go`:

```go
func TestCreateDraftsIsOneTransactionWithParentsAndANote(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	drafts := []backend.IssueDraft{
		{Type: backend.TypeTask, Summary: "First", ParentKey: "PLAT-1", Labels: []string{"a"}},
		{Type: backend.TypeRequirement, Summary: "Second", Priority: "High"},
		{Type: backend.TypeBug, Summary: "Third", StoryPoints: pts(2)},
	}
	keys, err := repo.CreateDrafts(ctx, "p1", "PLAT", drafts, "imported from backlog.xlsx")
	if err != nil {
		t.Fatalf("CreateDrafts: %v", err)
	}
	if strings.Join(keys, ",") != "TAM-NEW-1,TAM-NEW-2,TAM-NEW-3" {
		t.Errorf("keys = %v", keys)
	}
	first, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-1")
	if first.ParentKey != "PLAT-1" || !first.Draft || !first.Pending {
		t.Errorf("first: %+v", first)
	}
	second, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-2")
	if second.Type != backend.TypeRequirement || second.Priority != "High" {
		t.Errorf("second: %+v", second)
	}
	act, _ := repo.ListActivity(ctx, "p1", "TAM-NEW-3", 0)
	if len(act) != 1 || act[0].Action != "create" || act[0].Note != "imported from backlog.xlsx" {
		t.Errorf("audit note: %+v", act)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "TAM-NEW-1")
	var stored backend.IssueDraft
	_ = json.Unmarshal([]byte(pend[0].AfterVal), &stored)
	if stored.ParentKey != "PLAT-1" {
		t.Errorf("create JSON keeps the parent: %s", pend[0].AfterVal)
	}

	// One bad draft fails the whole call and leaves nothing behind.
	bad := []backend.IssueDraft{{Type: backend.TypeTask, Summary: "Ok"}, {Type: backend.TypeEpic, Summary: "Nope"}}
	if _, err := repo.CreateDrafts(ctx, "p1", "PLAT", bad, ""); err == nil {
		t.Fatal("an epic draft must be refused")
	}
	if _, err := repo.GetIssue(ctx, "p1", "TAM-NEW-4"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("nothing from the failed batch: %v", err)
	}
	if _, err := repo.CreateDrafts(ctx, "p1", "PLAT", nil, ""); err == nil {
		t.Error("an empty batch is refused")
	}
	// The single-draft path still works and now accepts requirements.
	k, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeRequirement, Summary: "Req"})
	if err != nil || k != "TAM-NEW-4" {
		t.Errorf("CreateDraft requirement: %q %v", k, err)
	}
}
```

Update `TestCreateDraftNumbersPerProfileAndEditsUpdateItsJSON`: the assertion that an epic draft is refused stays; the message check (if any) on "plan 1b creates tasks, stories, and bugs" changes to the new text below.

- [ ] **Step 3: Run them**

Run (inside `tam/`): `go test ./internal/issuerepo/ -run CreateDrafts`
Expected: compile failure (no `CreateDrafts`, no `ParentKey`).

- [ ] **Step 4: `CreateDrafts`**

In `tam/internal/issuerepo/writes.go`, change `draftTypes` and replace `CreateDraft`:

```go
// draftTypes are the logical types a draft may have.
var draftTypes = map[string]bool{backend.TypeTask: true, backend.TypeStory: true, backend.TypeBug: true, backend.TypeRequirement: true}
```

```go
// CreateDraft inserts one draft. See CreateDrafts.
func (r *Repository) CreateDraft(ctx context.Context, profileID, projectKey string, d backend.IssueDraft) (string, error) {
	keys, err := r.CreateDrafts(ctx, profileID, projectKey, []backend.IssueDraft{d}, "")
	if err != nil {
		return "", err
	}
	return keys[0], nil
}

// CreateDrafts inserts placeholder rows under the next temporary keys, one
// create row each holding the draft as JSON, in one transaction: any
// invalid draft fails the whole batch. note lands on every audit entry
// (the import puts the file name there). It returns the temporary keys in
// order.
func (r *Repository) CreateDrafts(ctx context.Context, profileID, projectKey string, drafts []backend.IssueDraft, note string) ([]string, error) {
	if len(drafts) == 0 {
		return nil, errors.New("nothing to create")
	}
	for i := range drafts {
		d := &drafts[i]
		if !draftTypes[d.Type] {
			return nil, fmt.Errorf("type %q cannot be created here; tasks, stories, bugs, and requirements can", d.Type)
		}
		if strings.TrimSpace(d.Summary) == "" {
			return nil, errors.New("summary cannot be empty")
		}
		d.Summary = strings.TrimSpace(d.Summary)
		d.ParentKey = strings.TrimSpace(d.ParentKey)
		if d.Labels == nil {
			d.Labels = []string{}
		}
		if d.Extra == nil {
			d.Extra = map[string]string{}
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var last int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(CAST(SUBSTR(key, ?) AS INTEGER)), 0) FROM issue WHERE profile_id = ? AND key LIKE ?`,
		len(DraftPrefix)+1, profileID, DraftPrefix+"%").Scan(&last); err != nil {
		return nil, fmt.Errorf("next draft key: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	keys := make([]string, 0, len(drafts))
	for _, d := range drafts {
		last++
		key := fmt.Sprintf("%s%d", DraftPrefix, last)
		encoded, err := json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("encode draft: %w", err)
		}
		labels, _ := json.Marshal(d.Labels)
		var points sql.NullFloat64
		if d.StoryPoints != nil {
			points = sql.NullFloat64{Float64: *d.StoryPoints, Valid: true}
		}
		detail, _ := json.Marshal(backend.IssueDetail{Key: key, Description: d.Description, Fields: map[string]any{}})
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO issue (profile_id, key, id, project, type, summary, status, assignee, reporter, priority, labels,
				sprint_id, sprint_name, parent_key, story_points, rank, created, updated, synced_at, detail_json, detail_fetched_at)
			VALUES (?, ?, '', ?, ?, ?, ?, ?, '', ?, ?, '', '', ?, ?, '', ?, '', '', ?, ?)`,
			profileID, key, projectKey, d.Type, d.Summary, StatusDraft, d.Assignee, d.Priority, string(labels),
			d.ParentKey, points, now, string(detail), now); err != nil {
			return nil, fmt.Errorf("insert draft: %w", err)
		}
		if err := journal.Put(tx, profileID, EntityIssueCreate, key, FieldCreate, "", string(encoded), ""); err != nil {
			return nil, err
		}
		if err := journal.Audit(tx, profileID, EntityIssue, key, "create", "", "", d.Summary, note); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return keys, nil
}
```

The `INSERT` now has 14 placeholders: the `parent_key` column takes `d.ParentKey` where the old statement had `''`.

Run `go test ./internal/issuerepo/ -count=1` inside `tam/`. Expected: PASS, including the earlier draft tests (numbering and JSON rewriting are unchanged).

- [ ] **Step 5: Jira sends the parent through Epic Link; the demo stores it**

In `tam/internal/backend/jira/jira_test.go`, add a field list beside `twoFields`:

```go
const threeFields = `[{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true},{"id":"customfield_10014","name":"Epic Link","custom":true}]`
```

Append to `tam/internal/backend/jira/writes_test.go`:

```go
func TestCreateIssueSendsTheParentThroughEpicLinkWhenItExists(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "PLAT-502"
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Under an epic", ParentKey: "PLAT-350"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.writes[len(f.writes)-1], `"customfield_10014":"PLAT-350"`) {
		t.Errorf("epic link missing: %s", f.writes[len(f.writes)-1])
	}
	noEpic, f2 := newBackend(t, twoFields)
	f2.createKey = "PLAT-503"
	if _, err := noEpic.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "No field", ParentKey: "PLAT-350"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f2.writes[len(f2.writes)-1], "PLAT-350") {
		t.Errorf("the parent must be dropped when the field is missing: %s", f2.writes[len(f2.writes)-1])
	}
}
```

In `tam/internal/backend/jira/writes.go`, inside `CreateIssue` after the story points block, add:

```go
	if d.ParentKey != "" {
		if ids.EpicLink != "" && d.Type != backend.TypeEpic {
			fields[ids.EpicLink] = d.ParentKey
		} else {
			log.Printf("tam: %s has no Epic Link field; parent %s dropped from the create of %q", b.c.BaseURL(), d.ParentKey, d.Summary)
		}
	}
```

Add `"log"` to the file's imports.

In `tam/internal/backend/demo/demo.go`, in `CreateIssue`, set `ParentKey: d.ParentKey` in the `backend.Issue` literal, and replace `CreateFields`:

```go
// CreateFields asks for one extra field on bugs and one on requirements,
// so the New issue dialog's create-meta section can be seen offline for
// both an option field and a text field.
func (b *Backend) CreateFields(_ context.Context, _, logicalType string) ([]backend.FieldSpec, error) {
	switch logicalType {
	case backend.TypeBug:
		return []backend.FieldSpec{{
			ID: "customfield_10050", Name: "Severity", Type: "option", Required: true,
			AllowedValues: []backend.FieldOption{{ID: "1", Value: "Minor"}, {ID: "2", Value: "Major"}, {ID: "3", Value: "Critical"}},
		}}, nil
	case backend.TypeRequirement:
		return []backend.FieldSpec{{ID: "customfield_10060", Name: "Source", Type: "string", Required: true, AllowedValues: []backend.FieldOption{}}}, nil
	}
	return []backend.FieldSpec{}, nil
}
```

In `tam/internal/backend/demo/demo_test.go`, extend `TestDemoBackendWritesInMemoryAndStagesOneConflict` at its end:

```go
	req, _ := b.CreateFields(ctx, "ACME", backend.TypeRequirement)
	if len(req) != 1 || req[0].Name != "Source" || req[0].Type != "string" {
		t.Errorf("requirement create fields: %+v", req)
	}
	withParent, _ := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeStory, Summary: "Child", ParentKey: "ACME-350"})
	if got, _ := b.GetIssue(ctx, withParent); got.ParentKey != "ACME-350" {
		t.Errorf("parent stored: %+v", got)
	}
```

- [ ] **Step 6: Run the suites**

Run (inside `tam/`):

```bash
go vet ./... && go test ./internal/... -count=1
```

Expected: PASS. The committer suite still passes: its fake `CreateIssue` ignores the new field.

- [ ] **Step 7: Commit**

```bash
git add tam/internal/backend tam/internal/issuerepo
git commit -m "feat(tam): bulk drafts with parents and an audit note, requirements creatable"
```

---

### Task 3: Journaled links in the store and both backends

A pending link is a journal row; the repository adds and lists them, merges them into the cached detail, and discards them; the backends list link types, create links, and (demo) answer lookups for foreign keys.

**Files:**
- Modify: `tam/internal/backend/backend.go` (`Link.Pending`, `Link.PendingID`, `LinkType`, `LinkDraft`, the interface)
- Modify: `tam/internal/issuerepo/pending.go` (`EntityLink`), `tam/internal/issuerepo/detail.go` (`ReadDetail` merge, `ClearDetail`), `tam/internal/issuerepo/writes.go` (`discardOne`)
- Create: `tam/internal/issuerepo/links.go`, `tam/internal/issuerepo/links_test.go`
- Create: `tam/internal/backend/jira/links.go`, `tam/internal/backend/jira/links_test.go`; modify `tam/internal/backend/jira/jira_test.go` (fake endpoints)
- Modify: `tam/internal/backend/demo/demo.go`, `demo_test.go`; `tam/internal/demo/demo.go` (`ForeignIssues`)
- Modify: `tam/internal/syncer/syncer_test.go` and `tam/internal/committer/committer_test.go` (the fakes gain the two methods)
- Test: `go build ./... && go vet ./... && go test ./internal/... -count=1` inside `tam/`

**Interfaces:**
- Consumes: `journal.Put`, `journal.ListForKey`, `journal.Audit`, `journal.Delete`; `corejira.Client.Get`, `Post`.
- Produces: `backend.Link.Pending bool` (`pending`), `backend.Link.PendingID int64` (`pendingId`); `backend.LinkType{Name, Inward, Outward string}`; `backend.LinkDraft{Type, Direction, ToKey, ToSummary, ToType string}`; on `IssueBackend`: `LinkTypes(ctx) ([]LinkType, error)`, `CreateLink(ctx, fromKey string, d LinkDraft) error`; `issuerepo.EntityLink = "link"`; `issuerepo.LinkField(d LinkDraft) string`; on `*Repository`: `AddLink(ctx, profileID, key string, d backend.LinkDraft) error`, `PendingLinks(ctx, profileID, key string) ([]backend.Link, error)`, `ClearDetail(ctx, profileID, key string) error`; `demo.ForeignIssues(projectKey string) []backend.Issue`.

- [ ] **Step 1: Shapes and the interface**

In `tam/internal/backend/backend.go`, extend `Link`:

```go
type Link struct {
	Direction string `json:"direction"` // "outward" or "inward"
	Type      string `json:"type"`      // the Jira link type name, e.g. "Tested By"
	Key       string `json:"key"`       // the other issue
	Summary   string `json:"summary"`
	IssueType string `json:"issueType"` // the other issue's Jira type name
	// Pending marks a link the journal holds and Commit has not pushed;
	// PendingID is its journal row, for Discard.
	Pending   bool  `json:"pending"`
	PendingID int64 `json:"pendingId"`
}
```

Add after `FieldOption`:

```go
// LinkType is one of Jira's issue link types with its two phrasings, for
// example Blocks: "blocks" outward, "is blocked by" inward.
type LinkType struct {
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// LinkDraft is a link the user asked for: the type, which side the source
// issue is on, and the target with the summary and type the lookup showed.
type LinkDraft struct {
	Type      string `json:"type"`
	Direction string `json:"direction"` // "outward" or "inward"
	ToKey     string `json:"toKey"`
	ToSummary string `json:"toSummary"`
	ToType    string `json:"toType"`
}
```

Extend `IssueBackend` after `CreateFields`:

```go
	// LinkTypes lists the issue link types the instance defines.
	LinkTypes(ctx context.Context) ([]LinkType, error)
	// CreateLink links fromKey to the draft's target with the draft's type
	// and direction.
	CreateLink(ctx context.Context, fromKey string, d LinkDraft) error
```

Add the stubs to both fakes in `tam/internal/syncer/syncer_test.go` (`fake` and `cancelOnSearch`):

```go
func (f *fake) LinkTypes(context.Context) ([]backend.LinkType, error) { return nil, errors.New("not used") }
func (f *fake) CreateLink(context.Context, string, backend.LinkDraft) error { return errors.New("not used") }
```

(and the same two on `cancelOnSearch`). In `tam/internal/committer/committer_test.go`, give `fake` a `links []string` field and:

```go
func (f *fake) LinkTypes(context.Context) ([]backend.LinkType, error) {
	return []backend.LinkType{{Name: "Relates", Inward: "relates to", Outward: "relates to"}}, nil
}
func (f *fake) CreateLink(_ context.Context, fromKey string, d backend.LinkDraft) error {
	if f.linkErr != nil {
		return f.linkErr
	}
	f.links = append(f.links, fromKey+" "+d.Direction+" "+d.Type+" "+d.ToKey)
	return nil
}
```

with `linkErr error` added to the struct. Run `go build ./...` inside `tam/`: the two real backends now fail the interface; Steps 5 and 7 fix that.

- [ ] **Step 2: Repository tests**

Create `tam/internal/issuerepo/links_test.go`:

```go
package issuerepo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func relates(to string) backend.LinkDraft {
	return backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: to, ToSummary: "Summary of " + to, ToType: "Test"}
}

func TestAddLinkJournalsAndMergesIntoTheDetail(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "d", Links: []backend.Link{{Direction: "inward", Type: "Tested By", Key: "XT-1", Summary: "Existing", IssueType: "Test"}}}, time.Now())
	if err := repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-9")); err != nil {
		t.Fatalf("AddLink: %v", err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(pend) != 1 || pend[0].EntityType != issuerepo.EntityLink || pend[0].Field != "Relates|outward|XT-9" || pend[0].BaseVersion != "" {
		t.Fatalf("journal row: %+v", pend)
	}
	links, err := repo.PendingLinks(ctx, "p1", "PLAT-1")
	if err != nil || len(links) != 1 || !links[0].Pending || links[0].PendingID != pend[0].ID || links[0].Key != "XT-9" || links[0].Summary != "Summary of XT-9" || links[0].IssueType != "Test" {
		t.Errorf("PendingLinks: %+v %v", links, err)
	}
	d, _, ok, err := repo.ReadDetail(ctx, "p1", "PLAT-1")
	if err != nil || !ok || len(d.Links) != 2 || d.Links[0].Pending || !d.Links[1].Pending {
		t.Errorf("detail merges pending links after the cached ones: %+v %v %v", d.Links, ok, err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !iss.Pending {
		t.Error("a pending link marks the row")
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 1 || act[0].Action != "link" || act[0].AfterVal != "XT-9" || act[0].Field != "Relates|outward|XT-9" {
		t.Errorf("audit: %+v", act)
	}
}

func TestAddLinkRefusesDuplicatesAndBadInput(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Links: []backend.Link{{Direction: "outward", Type: "Relates", Key: "XT-1"}}}, time.Now())
	cases := map[string]backend.LinkDraft{
		"already linked":  relates("XT-1"),
		"itself":          relates("PLAT-1"),
		"empty target":    relates(""),
		"bad direction":   {Type: "Relates", Direction: "sideways", ToKey: "XT-2"},
		"empty type":      {Direction: "outward", ToKey: "XT-2"},
	}
	for name, d := range cases {
		if err := repo.AddLink(ctx, "p1", "PLAT-1", d); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if err := repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-2")); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-2")); err == nil || !strings.Contains(err.Error(), "already pending") {
		t.Errorf("duplicate pending: %v", err)
	}
	if err := repo.AddLink(ctx, "p1", "PLAT-9", relates("XT-2")); err == nil {
		t.Error("unknown source issue must be refused")
	}
}

func TestDiscardingALinkDropsTheRowOnly(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine")
	_ = repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-2"))
	links, _ := repo.PendingLinks(ctx, "p1", "PLAT-1")
	if err := repo.DiscardPendingChange(ctx, "p1", links[0].PendingID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Summary != "mine" || !iss.Pending {
		t.Errorf("the edit is untouched: %+v", iss)
	}
	if rest, _ := repo.PendingLinks(ctx, "p1", "PLAT-1"); len(rest) != 0 {
		t.Errorf("link gone: %+v", rest)
	}
	_ = repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-3"))
	if n, err := repo.DiscardAllPendingChanges(ctx, "p1"); err != nil || n != 2 {
		t.Errorf("discard all counts the link: %d %v", n, err)
	}
}

func TestClearDetailDropsTheCache(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "d"}, time.Now())
	if err := repo.ClearDetail(ctx, "p1", "PLAT-1"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, _ := repo.ReadDetail(ctx, "p1", "PLAT-1"); ok {
		t.Error("detail must be gone")
	}
}
```

- [ ] **Step 3: Run them**

Run (inside `tam/`): `go test ./internal/issuerepo/ -run 'AddLink|DiscardingALink|ClearDetail'`
Expected: compile failure.

- [ ] **Step 4: `links.go`, the detail merge, the discard branch**

In `tam/internal/issuerepo/pending.go`, add to the constants block:

```go
	// EntityLink is the journal entity type of a link to create. The row's
	// field is LinkField(d) and its after_val the LinkDraft as JSON.
	EntityLink = "link"
```

Create `tam/internal/issuerepo/links.go`:

```go
package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// LinkField is the journal field of a link row: type, direction, and
// target, so the journal's uniqueness refuses the same link twice.
func LinkField(d backend.LinkDraft) string {
	return d.Type + "|" + d.Direction + "|" + d.ToKey
}

// AddLink journals a link from key to the draft's target. The source must
// be a cached issue (a draft counts), the target another key, and the same
// link must not exist yet, cached or pending.
func (r *Repository) AddLink(ctx context.Context, profileID, key string, d backend.LinkDraft) error {
	d.Type = strings.TrimSpace(d.Type)
	d.ToKey = strings.TrimSpace(d.ToKey)
	switch {
	case d.Type == "":
		return errors.New("link type is empty")
	case d.Direction != "outward" && d.Direction != "inward":
		return fmt.Errorf("link direction %q is neither outward nor inward", d.Direction)
	case d.ToKey == "":
		return errors.New("target issue key is empty")
	case strings.EqualFold(d.ToKey, key):
		return errors.New("an issue cannot link to itself")
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode link: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM issue WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("check %s exists: %w", key, err)
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM issue_link WHERE profile_id = ? AND from_key = ? AND link_type = ? AND direction = ? AND to_key = ?`,
		profileID, key, d.Type, d.Direction, d.ToKey).Scan(&exists); err == nil {
		return fmt.Errorf("%s is already linked to %s that way", key, d.ToKey)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check link exists: %w", err)
	}
	field := LinkField(d)
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, EntityLink, key, field).Scan(&exists); err == nil {
		return fmt.Errorf("a link from %s to %s is already pending", key, d.ToKey)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check pending link: %w", err)
	}
	if err := journal.Put(tx, profileID, EntityLink, key, field, "", string(encoded), ""); err != nil {
		return err
	}
	if err := journal.Audit(tx, profileID, EntityLink, key, "link", field, "", d.ToKey, ""); err != nil {
		return err
	}
	return tx.Commit()
}

// PendingLinks returns the links the journal holds for key, as Link rows
// flagged pending with their journal ids.
func (r *Repository) PendingLinks(ctx context.Context, profileID, key string) ([]backend.Link, error) {
	rows, err := journal.ListForKey(r.db, profileID, key)
	if err != nil {
		return nil, err
	}
	links := []backend.Link{}
	for _, p := range rows {
		if p.EntityType != EntityLink {
			continue
		}
		var d backend.LinkDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			return nil, fmt.Errorf("decode pending link %d: %w", p.ID, err)
		}
		links = append(links, backend.Link{
			Direction: d.Direction, Type: d.Type, Key: d.ToKey, Summary: d.ToSummary, IssueType: d.ToType,
			Pending: true, PendingID: p.ID,
		})
	}
	return links, nil
}
```

In `tam/internal/issuerepo/detail.go`, in `ReadDetail` after `links, err := r.ListLinks(ctx, profileID, key)` and its error check, merge the pending ones:

```go
	pending, err := r.PendingLinks(ctx, profileID, key)
	if err != nil {
		return backend.IssueDetail{}, time.Time{}, false, err
	}
	links = append(links, pending...)
```

Also add to `detail.go`:

```go
// ClearDetail drops the cached detail so the next panel open refetches it,
// which is what a pushed link needs: Jira now has a link the cache lacks.
func (r *Repository) ClearDetail(ctx context.Context, profileID, key string) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE issue SET detail_json = NULL, detail_fetched_at = NULL WHERE profile_id = ? AND key = ?`, profileID, key); err != nil {
		return fmt.Errorf("clear detail for %s: %w", key, err)
	}
	return nil
}
```

In `tam/internal/issuerepo/writes.go`, in `discardOne`, add a branch so a link row has nothing to revert:

```go
	switch {
	case p.EntityType == EntityIssueCreate:
		// existing draft-drop branch, unchanged
	case p.EntityType == EntityLink:
		// A link that was never pushed: nothing on the row to revert.
	default:
		// existing exists-check and writeField branch, unchanged
	}
```

Rewrite the function's `if ... else` into that `switch` keeping the two existing bodies verbatim.

Run `go test ./internal/issuerepo/ -count=1` inside `tam/`. Expected: PASS.

- [ ] **Step 5: Jira link types and link creation**

In `tam/internal/backend/jira/jira_test.go`, add to `fakeJira` a `linkTypes string` field (the `/rest/api/2/issueLinkType` body) and, in the handler's switch before the write case, a read case:

```go
		case r.URL.Path == "/rest/api/2/issueLinkType" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(f.linkTypes))
```

and in `newBackend` initialise it: `f := &fakeJira{fields: fields, linkTypes: `{"issueLinkTypes":[{"id":"10000","name":"Blocks","inward":"is blocked by","outward":"blocks"},{"id":"10003","name":"Relates","inward":"relates to","outward":"relates to"}]}`}`. The existing write case already records `POST /rest/api/2/issueLink` bodies into `f.writes` and answers 204.

Create `tam/internal/backend/jira/links_test.go`:

```go
package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestLinkTypesAreReadOnceAndCached(t *testing.T) {
	b, f := newBackend(t, twoFields)
	types, err := b.LinkTypes(context.Background())
	if err != nil || len(types) != 2 || types[0].Name != "Blocks" || types[0].Inward != "is blocked by" || types[1].Outward != "relates to" {
		t.Fatalf("LinkTypes: %+v %v", types, err)
	}
	f.linkTypes = `{"issueLinkTypes":[]}`
	again, _ := b.LinkTypes(context.Background())
	if len(again) != 2 {
		t.Error("second call must come from the cache")
	}
}

func TestCreateLinkPutsTheSourceOnTheRightSide(t *testing.T) {
	b, f := newBackend(t, twoFields)
	ctx := context.Background()
	if err := b.CreateLink(ctx, "PLAT-412", backend.LinkDraft{Type: "Blocks", Direction: "outward", ToKey: "PAY-77"}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateLink(ctx, "PLAT-412", backend.LinkDraft{Type: "Blocks", Direction: "inward", ToKey: "PAY-78"}); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 2 || !strings.HasPrefix(f.writes[0], "POST /rest/api/2/issueLink ") {
		t.Fatalf("writes = %v", f.writes)
	}
	if !strings.Contains(f.writes[0], `"outwardIssue":{"key":"PLAT-412"}`) || !strings.Contains(f.writes[0], `"inwardIssue":{"key":"PAY-77"}`) || !strings.Contains(f.writes[0], `"type":{"name":"Blocks"}`) {
		t.Errorf("outward body: %s", f.writes[0])
	}
	if !strings.Contains(f.writes[1], `"outwardIssue":{"key":"PAY-78"}`) || !strings.Contains(f.writes[1], `"inwardIssue":{"key":"PLAT-412"}`) {
		t.Errorf("inward body: %s", f.writes[1])
	}
	if err := b.CreateLink(ctx, "PLAT-412", backend.LinkDraft{Type: "Blocks", Direction: "sideways", ToKey: "PAY-79"}); err == nil {
		t.Error("bad direction must be refused before any request")
	}
}
```

Create `tam/internal/backend/jira/links.go`:

```go
package jira

import (
	"context"
	"fmt"

	"agile-suite/tam/internal/backend"
)

// linkTypesResponse is Jira's /rest/api/2/issueLinkType answer.
type linkTypesResponse struct {
	IssueLinkTypes []struct {
		Name    string `json:"name"`
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"issueLinkTypes"`
}

// LinkTypes reads the instance's link types once per backend.
func (b *Backend) LinkTypes(ctx context.Context) ([]backend.LinkType, error) {
	b.mu.Lock()
	if b.linkTypesLoaded {
		out := append([]backend.LinkType{}, b.linkTypes...)
		b.mu.Unlock()
		return out, nil
	}
	b.mu.Unlock()
	var resp linkTypesResponse
	if err := b.c.Get(ctx, "/rest/api/2/issueLinkType", &resp); err != nil {
		return nil, err
	}
	types := make([]backend.LinkType, 0, len(resp.IssueLinkTypes))
	for _, t := range resp.IssueLinkTypes {
		types = append(types, backend.LinkType{Name: t.Name, Inward: t.Inward, Outward: t.Outward})
	}
	b.mu.Lock()
	b.linkTypes = types
	b.linkTypesLoaded = true
	b.mu.Unlock()
	return append([]backend.LinkType{}, types...), nil
}

// CreateLink POSTs an issue link. For an outward link the source is the
// outward issue ("PLAT-1 blocks PAY-7"); for an inward one it is the inward
// issue ("PLAT-1 is blocked by PAY-7").
func (b *Backend) CreateLink(ctx context.Context, fromKey string, d backend.LinkDraft) error {
	var outward, inward string
	switch d.Direction {
	case "outward":
		outward, inward = fromKey, d.ToKey
	case "inward":
		outward, inward = d.ToKey, fromKey
	default:
		return fmt.Errorf("link direction %q is neither outward nor inward", d.Direction)
	}
	body := map[string]any{
		"type":         map[string]string{"name": d.Type},
		"inwardIssue":  map[string]string{"key": inward},
		"outwardIssue": map[string]string{"key": outward},
	}
	return b.c.Post(ctx, "/rest/api/2/issueLink", body)
}
```

Add to the `Backend` struct in `tam/internal/backend/jira/jira.go`, after `discovered bool`:

```go
	linkTypes       []backend.LinkType
	linkTypesLoaded bool
```

Run `go test ./internal/backend/jira/ -count=1` inside `tam/`. Expected: PASS.

- [ ] **Step 6: Demo foreign issues**

In `tam/internal/demo/demo.go`, add after `Detail`:

```go
// ForeignIssues are the issues outside the project that the curated
// details link to (the XT test keys), as minimal rows so a lookup for a
// cross-project link has something to find offline.
func ForeignIssues(projectKey string) []backend.Issue {
	seen := map[string]bool{}
	out := []backend.Issue{}
	for _, d := range curatedDetails {
		for _, l := range d.Links {
			if strings.HasPrefix(l.Key, ProjectKey+"-") || seen[l.Key] {
				continue
			}
			seen[l.Key] = true
			project := l.Key
			if i := strings.LastIndex(l.Key, "-"); i > 0 {
				project = l.Key[:i]
			}
			out = append(out, backend.Issue{
				Key: l.Key, ID: l.Key, Project: project, Type: "", Summary: l.Summary, Status: "Done",
				Reporter: "PO", Labels: []string{}, Updated: base.Format(time.RFC3339),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
```

Add `"sort"` to that file's imports if missing (`strings` and `time` are already there). The `_ = projectKey` is not needed: the parameter is kept for symmetry with `Issues` and used for nothing; drop the parameter instead if `go vet` complains about it (it does not; unused parameters are fine).

- [ ] **Step 7: Demo link types, links, and lookups**

In `tam/internal/backend/demo/demo.go`, add `links map[string][]backend.Link` to the struct and initialise it in `New` (`links: map[string][]backend.Link{}`). Change `find` to fall back to the foreign issues:

```go
func (b *Backend) find(key string) (backend.Issue, bool) {
	for _, iss := range b.issues() {
		if iss.Key == key {
			return iss, true
		}
	}
	for _, iss := range demo.ForeignIssues(b.project) {
		if iss.Key == key {
			return iss, true
		}
	}
	return backend.Issue{}, false
}
```

In `GetIssueDetail`, before `return d, nil`, merge created links:

```go
	d.Links = append(d.Links, b.links[key]...)
```

Add the two methods:

```go
// demoLinkTypes are the three link types the demo defines.
var demoLinkTypes = []backend.LinkType{
	{Name: "Relates", Inward: "relates to", Outward: "relates to"},
	{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
	{Name: "Tested By", Inward: "is tested by", Outward: "tests"},
}

func (b *Backend) LinkTypes(context.Context) ([]backend.LinkType, error) {
	return append([]backend.LinkType{}, demoLinkTypes...), nil
}

// CreateLink records the link so the detail shows it from now on.
func (b *Backend) CreateLink(_ context.Context, fromKey string, d backend.LinkDraft) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(fromKey); !ok {
		return fmt.Errorf("demo: no issue %s", fromKey)
	}
	if _, ok := b.find(d.ToKey); !ok {
		return fmt.Errorf("demo: no issue %s", d.ToKey)
	}
	b.links[fromKey] = append(b.links[fromKey], backend.Link{
		Direction: d.Direction, Type: d.Type, Key: d.ToKey, Summary: d.ToSummary, IssueType: d.ToType,
	})
	return nil
}
```

`GetIssue` already uses `find`, so `GetIssue(ctx, "XT-1018")` now returns the synthetic row.

Append to `tam/internal/backend/demo/demo_test.go`:

```go
func TestDemoBackendLinks(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	types, _ := b.LinkTypes(ctx)
	if len(types) != 3 || types[1].Inward != "is blocked by" {
		t.Errorf("link types: %+v", types)
	}
	foreign, err := b.GetIssue(ctx, "XT-1018")
	if err != nil || foreign.Summary != "Promo code applies discount" || foreign.Project != "XT" {
		t.Errorf("foreign lookup: %+v %v", foreign, err)
	}
	if err := b.CreateLink(ctx, "ACME-409", backend.LinkDraft{Type: "Blocks", Direction: "inward", ToKey: "XT-1018", ToSummary: "Promo code applies discount", ToType: "Test"}); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	d, _ := b.GetIssueDetail(ctx, "ACME-409")
	found := false
	for _, l := range d.Links {
		if l.Key == "XT-1018" && l.Type == "Blocks" && l.Direction == "inward" {
			found = true
		}
	}
	if !found {
		t.Errorf("created link missing from the detail: %+v", d.Links)
	}
	if err := b.CreateLink(ctx, "ACME-409", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "NOPE-1"}); err == nil {
		t.Error("unknown target must fail")
	}
}
```

- [ ] **Step 8: Run everything**

Run (inside `tam/`):

```bash
gofmt -l ./internal/issuerepo/links.go ./internal/backend/jira/links.go ./internal/backend/demo ./internal/demo && go build ./... && go vet ./... && go test ./internal/... -count=1
```

Expected: `gofmt` silent, every package PASS. `go build ./...` at the module root compiles `app_issues.go`, which assigns both backends to the interface; it must be clean.

- [ ] **Step 9: Commit**

```bash
git add tam/internal
git commit -m "feat(tam): journaled links in the store, link types and link creation in both backends"
```

---

### Task 4: The importer

`tam/internal/importer` maps columns to draft fields, validates rows, and creates drafts in one transaction.

**Files:**
- Create: `tam/internal/importer/importer.go`, `tam/internal/importer/importer_test.go`
- Test: `go test ./internal/importer/` inside `tam/`

**Interfaces:**
- Consumes: `importfile.ParseRecords` (Task 1), `issuerepo.CreateDrafts` and `GetIssue` (Task 2), `backend.ParsePoints`, `backend.SplitLabels`.
- Produces (package `agile-suite/tam/internal/importer`): `Mapping{Type, Summary, Description, Priority, Labels, Assignee, StoryPoints, ParentKey string}` (JSON `type`, `summary`, `description`, `priority`, `labels`, `assignee`, `storyPoints`, `parentKey`); `RowError{Row int; Message string}` (`row`, `message`); `Result{Rows int; Created []string; Errors []RowError}` (`rows`, `created`, `errors`); `AutoMap(headers []string) Mapping`; `Run(ctx, repo *issuerepo.Repository, profileID, projectKey, requirementType string, records [][]string, mapping Mapping, fileName string, dryRun bool) (Result, error)`; `TemplateCSV() []byte`.

- [ ] **Step 1: Tests**

Create `tam/internal/importer/importer_test.go`:

```go
package importer_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/importer"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *issuerepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := issuerepo.New(db.DB())
	rows := []backend.Issue{{Key: "PLAT-350", Type: backend.TypeEpic, Summary: "Promotions", Labels: []string{}, Updated: "2026-09-01T00:00:00Z"}}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestAutoMapMatchesHeadersLooselyAndBySynonym(t *testing.T) {
	m := importer.AutoMap([]string{"Issue Type", "Summary", "Description", "priority", "Labels", "Assignee", "Story_Points", "Epic Link", "Comment"})
	want := importer.Mapping{Type: "Issue Type", Summary: "Summary", Description: "Description", Priority: "priority", Labels: "Labels", Assignee: "Assignee", StoryPoints: "Story_Points", ParentKey: "Epic Link"}
	if m != want {
		t.Errorf("AutoMap = %+v, want %+v", m, want)
	}
	m = importer.AutoMap([]string{"Title", "Points", "Parent"})
	if m.Summary != "Title" || m.StoryPoints != "Points" || m.ParentKey != "Parent" || m.Type != "" {
		t.Errorf("synonyms: %+v", m)
	}
}

func records() [][]string {
	return [][]string{
		{"Issue Type", "Summary", "Description", "Priority", "Labels", "Assignee", "Points", "Epic Link"},
		{"Story", "Apply promo at payment", "As a shopper", "High", "checkout, promo", "ranand", "5", "PLAT-350"},
		{"", "Rotate keys", "", "", "security", "", "", ""},
		{"Bug", "", "no summary", "", "", "", "", ""},
		{"Epic", "Not creatable", "", "", "", "", "", ""},
		{"Task", "Bad points", "", "", "", "", "eight", ""},
		{"Task", "Unknown parent", "", "", "", "", "", "PLAT-999"},
		{"Business Requirement", "Single-use promo codes", "", "", "promo", "", "", ""},
	}
}

func TestRunDryRunValidatesEveryRuleAndCreatesNothing(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	m := importer.AutoMap(records()[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "Business Requirement", records(), m, "backlog.csv", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Rows != 7 || len(res.Created) != 0 || len(res.Errors) != 4 {
		t.Fatalf("result: %+v", res)
	}
	got := map[int]string{}
	for _, e := range res.Errors {
		got[e.Row] = e.Message
	}
	for row, want := range map[int]string{4: "Summary is empty", 5: `Type "Epic" cannot be created`, 6: `Story points "eight" is not a number`, 7: "Parent PLAT-999 is not in the cache"} {
		if !strings.Contains(got[row], want) {
			t.Errorf("row %d: %q lacks %q", row, got[row], want)
		}
	}
	if page, _ := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{}); page.Total != 1 {
		t.Errorf("dry run created rows: %d", page.Total)
	}
}

func TestRunImportsTheValidRowsAsDrafts(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	m := importer.AutoMap(records()[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "Business Requirement", records(), m, "backlog.csv", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(res.Created, ",") != "TAM-NEW-1,TAM-NEW-2,TAM-NEW-3" || len(res.Errors) != 4 {
		t.Fatalf("result: %+v", res)
	}
	first, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-1")
	if first.Type != backend.TypeStory || first.Summary != "Apply promo at payment" || first.Priority != "High" || strings.Join(first.Labels, "|") != "checkout|promo" || first.Assignee != "ranand" || *first.StoryPoints != 5 || first.ParentKey != "PLAT-350" {
		t.Errorf("first: %+v", first)
	}
	second, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-2")
	if second.Type != backend.TypeTask || second.StoryPoints != nil {
		t.Errorf("blank type means task, blank points mean none: %+v", second)
	}
	third, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-3")
	if third.Type != backend.TypeRequirement {
		t.Errorf("the profile's requirement type name maps to requirement: %+v", third)
	}
	d, _, _, _ := repo.ReadDetail(ctx, "p1", "TAM-NEW-1")
	if d.Description != "As a shopper" {
		t.Errorf("description: %+v", d)
	}
	act, _ := repo.ListActivity(ctx, "p1", "TAM-NEW-1", 0)
	if len(act) != 1 || act[0].Note != "imported from backlog.csv" {
		t.Errorf("audit note: %+v", act)
	}
}

func TestRunRefusesAMappingWithoutSummaryOrWithAMissingColumn(t *testing.T) {
	repo := newRepo(t)
	if _, err := importer.Run(context.Background(), repo, "p1", "PLAT", "", records(), importer.Mapping{Type: "Issue Type"}, "f.csv", true); err == nil || !strings.Contains(err.Error(), "Summary") {
		t.Errorf("no summary mapping: %v", err)
	}
	if _, err := importer.Run(context.Background(), repo, "p1", "PLAT", "", records(), importer.Mapping{Summary: "Nope"}, "f.csv", true); err == nil || !strings.Contains(err.Error(), `"Nope"`) {
		t.Errorf("missing column: %v", err)
	}
	if _, err := importer.Run(context.Background(), repo, "p1", "PLAT", "", [][]string{{"Summary"}}, importer.Mapping{Summary: "Summary"}, "f.csv", true); err == nil {
		t.Error("a file with only a header has nothing to import")
	}
}

func TestTemplateCSVRoundTripsThroughAutoMap(t *testing.T) {
	data := importer.TemplateCSV()
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "Type,Summary,Description,Priority,Labels,Assignee,Story Points,Parent") {
		t.Errorf("template: %q", string(data))
	}
	m := importer.AutoMap(strings.Split(lines[0], ","))
	if m.Type == "" || m.Summary == "" || m.StoryPoints == "" || m.ParentKey == "" {
		t.Errorf("template headers must auto-map: %+v", m)
	}
}
```

- [ ] **Step 2: Run them**

Run (inside `tam/`): `go test ./internal/importer/`
Expected: compile failure, the package does not exist.

- [ ] **Step 3: The importer**

Create `tam/internal/importer/importer.go`:

```go
// Package importer turns spreadsheet rows into drafts: a column mapping,
// per-row validation with file row numbers, a dry run that only validates,
// and an import that creates every valid row's draft in one transaction.
package importer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Mapping names the column header for each draft field; empty means
// unmapped. Summary is required.
type Mapping struct {
	Type        string `json:"type"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Labels      string `json:"labels"`
	Assignee    string `json:"assignee"`
	StoryPoints string `json:"storyPoints"`
	ParentKey   string `json:"parentKey"`
}

// RowError is one row that was skipped. Row is the file row: the header is
// row 1, the first data row is row 2.
type RowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// Result is what a dry run or an import found and did.
type Result struct {
	Rows    int        `json:"rows"`
	Created []string   `json:"created"`
	Errors  []RowError `json:"errors"`
}

// synonyms are the normalised header names each field accepts, first
// match wins.
var synonyms = map[string][]string{
	"type":        {"type", "issuetype"},
	"summary":     {"summary", "title"},
	"description": {"description"},
	"priority":    {"priority"},
	"labels":      {"labels", "label"},
	"assignee":    {"assignee"},
	"storyPoints": {"storypoints", "points", "estimate"},
	"parentKey":   {"parent", "parentkey", "epic", "epiclink"},
}

// normalize lowercases a header and drops spaces, underscores, and hyphens.
func normalize(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	return strings.NewReplacer(" ", "", "_", "", "-", "").Replace(h)
}

// AutoMap picks a column for each field by header name; unmatched fields
// stay empty. A header serves one field only, in field order.
func AutoMap(headers []string) Mapping {
	used := map[int]bool{}
	pick := func(field string) string {
		for _, want := range synonyms[field] {
			for i, h := range headers {
				if !used[i] && normalize(h) == want {
					used[i] = true
					return h
				}
			}
		}
		return ""
	}
	return Mapping{
		Type:        pick("type"),
		Summary:     pick("summary"),
		Description: pick("description"),
		Priority:    pick("priority"),
		Labels:      pick("labels"),
		Assignee:    pick("assignee"),
		StoryPoints: pick("storyPoints"),
		ParentKey:   pick("parentKey"),
	}
}

// columns resolves the mapping to header indexes; -1 means unmapped.
type columns struct {
	typ, summary, description, priority, labels, assignee, points, parent int
}

func resolve(headers []string, m Mapping) (columns, error) {
	index := func(name string) (int, error) {
		if name == "" {
			return -1, nil
		}
		for i, h := range headers {
			if h == name {
				return i, nil
			}
		}
		return -1, fmt.Errorf("column %q is not in the file", name)
	}
	var c columns
	var err error
	if m.Summary == "" {
		return c, errors.New("a Summary column must be mapped")
	}
	fields := []struct {
		name string
		dst  *int
	}{
		{m.Type, &c.typ}, {m.Summary, &c.summary}, {m.Description, &c.description}, {m.Priority, &c.priority},
		{m.Labels, &c.labels}, {m.Assignee, &c.assignee}, {m.StoryPoints, &c.points}, {m.ParentKey, &c.parent},
	}
	for _, f := range fields {
		if *f.dst, err = index(f.name); err != nil {
			return c, err
		}
	}
	return c, nil
}

func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// logicalType maps a type cell to a creatable logical type. Blank means
// task; the profile's requirement type name counts as requirement.
func logicalType(cell, requirementType string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(cell))
	switch n {
	case "", backend.TypeTask:
		return backend.TypeTask, nil
	case backend.TypeStory, backend.TypeBug, backend.TypeRequirement:
		return n, nil
	}
	if requirementType != "" && n == strings.ToLower(strings.TrimSpace(requirementType)) {
		return backend.TypeRequirement, nil
	}
	return "", fmt.Errorf("Type %q cannot be created; use Task, Story, Bug, or %s", strings.TrimSpace(cell), requirementLabel(requirementType))
}

func requirementLabel(requirementType string) string {
	if strings.TrimSpace(requirementType) == "" {
		return "Requirement"
	}
	return strings.TrimSpace(requirementType)
}

// Run validates every data row and, unless dryRun, creates the valid ones
// as drafts in one transaction. Rows with errors are skipped and listed.
func Run(ctx context.Context, repo *issuerepo.Repository, profileID, projectKey, requirementType string, records [][]string, m Mapping, fileName string, dryRun bool) (Result, error) {
	if len(records) < 2 {
		return Result{}, errors.New("the file has a header row but no data rows")
	}
	c, err := resolve(records[0], m)
	if err != nil {
		return Result{}, err
	}
	res := Result{Rows: len(records) - 1, Created: []string{}, Errors: []RowError{}}
	var drafts []backend.IssueDraft
	parents := map[string]bool{}
	for i, row := range records[1:] {
		fileRow := i + 2
		fail := func(msg string) { res.Errors = append(res.Errors, RowError{Row: fileRow, Message: msg}) }
		summary := cell(row, c.summary)
		if summary == "" {
			fail("Summary is empty.")
			continue
		}
		typ, err := logicalType(cell(row, c.typ), requirementType)
		if err != nil {
			fail(err.Error() + ".")
			continue
		}
		points, err := backend.ParsePoints(cell(row, c.points))
		if err != nil {
			fail(fmt.Sprintf("Story points %q is not a number.", cell(row, c.points)))
			continue
		}
		parent := cell(row, c.parent)
		if parent != "" {
			known, seen := parents[parent]
			if !seen {
				_, err := repo.GetIssue(ctx, profileID, parent)
				switch {
				case errors.Is(err, issuerepo.ErrNotFound):
					known = false
				case err != nil:
					return Result{}, err
				default:
					known = true
				}
				parents[parent] = known
			}
			if !known {
				fail(fmt.Sprintf("Parent %s is not in the cache. Sync first or clear the cell.", parent))
				continue
			}
		}
		drafts = append(drafts, backend.IssueDraft{
			Type:        typ,
			Summary:     summary,
			Description: cell(row, c.description),
			Priority:    cell(row, c.priority),
			Labels:      backend.SplitLabels(cell(row, c.labels)),
			Assignee:    cell(row, c.assignee),
			StoryPoints: points,
			ParentKey:   parent,
			Extra:       map[string]string{},
		})
	}
	if dryRun || len(drafts) == 0 {
		return res, nil
	}
	keys, err := repo.CreateDrafts(ctx, profileID, projectKey, drafts, "imported from "+fileName)
	if err != nil {
		return res, err
	}
	res.Created = keys
	return res, nil
}

// TemplateCSV is a starter file with the supported columns and one example
// row.
func TemplateCSV() []byte {
	return []byte("Type,Summary,Description,Priority,Labels,Assignee,Story Points,Parent\n" +
		"Story,Apply promo code at payment step,As a shopper I can enter a promo code,High,\"checkout, promo\",ranand,5,PLAT-350\n")
}
```

- [ ] **Step 4: Run the suite**

Run (inside `tam/`): `gofmt -l ./internal/importer && go vet ./internal/importer/ && go test ./internal/importer/ -count=1 -v`
Expected: five tests PASS. If the dry-run test reports three errors instead of four, check that the `Epic` row fails on type before anything else and that `PLAT-999` is not in the cache (the test seeds only `PLAT-350`).

- [ ] **Step 5: Commit**

```bash
git add tam/internal/importer
git commit -m "feat(tam): the importer, rows to drafts with a dry run"
```

---

### Task 5: Links in the commit pass, the seven bound methods, regenerated bindings

The committer pushes link rows after edits; the app binds import, link types, lookup, and add link; the Wails bindings are regenerated.

**Files:**
- Modify: `tam/internal/committer/committer.go`, `committer_test.go`
- Create: `tam/app_imports.go`
- Modify: `tam/app_writes.go` (`GetLinkTypes`, `LookupIssue`, `AddLink`)
- Regenerate: `tam/frontend/wailsjs/go/main/App.js`, `App.d.ts`, `tam/frontend/wailsjs/go/models.ts`
- Test: `go test ./... -count=1` inside `tam/`, then `wails generate module`

**Interfaces:**
- Consumes: Tasks 1 to 4.
- Produces: `committer.Result.Linked []Linked` (`linked`), `committer.Linked{Key, ToKey, Type string}`; the bound methods named in Global Constraints; generated TypeScript for `importfile.Preview`, `importer.Mapping`, `importer.Result`, `backend.LinkType`, `backend.LinkDraft`.

- [ ] **Step 1: Committer tests**

Append to `tam/internal/committer/committer_test.go`:

```go
func TestCommitPushesLinksAfterEditsAndClearsTheDetail(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "cached"}, time.Now())
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno")
	_ = repo.AddLink(ctx, "p1", "PLAT-1", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "XT-9", ToSummary: "A test", ToType: "Test"})
	_ = repo.AddLink(ctx, "p1", "PLAT-2", backend.LinkDraft{Type: "Relates", Direction: "inward", ToKey: "PAY-1"})
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Committed, ",") != "PLAT-1" || len(res.Linked) != 2 || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
	if len(f.links) != 2 || f.links[0] != "PLAT-1 outward Relates XT-9" || f.links[1] != "PLAT-2 inward Relates PAY-1" {
		t.Errorf("links pushed in key order after the edit: %v", f.links)
	}
	if len(f.updates) != 1 {
		t.Errorf("the edit was pushed once: %v", f.updates)
	}
	if _, _, ok, _ := repo.ReadDetail(ctx, "p1", "PLAT-1"); ok {
		t.Error("the detail cache is dropped after a link push")
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-2", 0)
	if len(act) != 2 || act[0].Action != "commit" || act[0].EntityType != issuerepo.EntityLink {
		t.Errorf("link commit audited: %+v", act)
	}
}

func TestCommitLinksFromADraftFollowTheRealKey(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Draft with a link"})
	_ = repo.AddLink(ctx, "p1", temp, backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "XT-9"})
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 || len(res.Linked) != 1 || res.Linked[0].Key != "PLAT-501" || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
	if len(f.links) != 1 || f.links[0] != "PLAT-501 outward Relates XT-9" {
		t.Errorf("the link is pushed under the real key: %v", f.links)
	}
}

func TestLinkFailureKeepsTheRow(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.AddLink(ctx, "p1", "PLAT-1", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "XT-9"})
	f.linkErr = errors.New("POST failed: 404 XT-9 does not exist")
	res, _ := eng.Commit(ctx, "p1", "PLAT")
	if len(res.Failures) != 1 || res.Failures[0].Key != "PLAT-1" || !strings.Contains(res.Failures[0].Error, "XT-9") || res.Remaining != 1 {
		t.Errorf("result: %+v", res)
	}
	if links, _ := repo.PendingLinks(ctx, "p1", "PLAT-1"); len(links) != 1 {
		t.Errorf("row kept: %+v", links)
	}
}
```

- [ ] **Step 2: Run them**

Run (inside `tam/`): `go test ./internal/committer/ -run 'Links|LinkFailure'`
Expected: compile failure (`res.Linked`), then, once it compiles, failures because links are pushed as if they were edits.

- [ ] **Step 3: The link pass**

In `tam/internal/committer/committer.go`:

Add the type and the field:

```go
// Linked is a link Commit created.
type Linked struct {
	Key   string `json:"key"`
	ToKey string `json:"toKey"`
	Type  string `json:"type"`
}
```

`Result` gains `Linked []Linked \`json:"linked"\`` after `Created`, and `Commit`'s initial literal sets `Linked: []Linked{}`.

In `Commit`, skip link rows when grouping (they are handled by their own pass), so the loop over `all` begins:

```go
	for _, p := range all {
		if p.EntityType == issuerepo.EntityLink {
			continue
		}
```

After the edits loop and before the `left` count, add:

```go
	e.commitLinks(ctx, profileID, &res)
```

And the pass itself:

```go
// commitLinks pushes every link row, read fresh so a link added from a
// draft carries the key the create pass gave it. Each push is its own
// journal delete, and the source's detail cache is dropped so the panel
// refetches the links Jira now holds.
func (e *Engine) commitLinks(ctx context.Context, profileID string, res *Result) {
	all, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		res.Failures = append(res.Failures, Failure{Key: "", Error: "the journal could not be read for links: " + err.Error()})
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	for _, p := range all {
		if p.EntityType != issuerepo.EntityLink {
			continue
		}
		var d backend.LinkDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			res.Failures = append(res.Failures, Failure{Key: p.EntityKey, Error: "the link could not be decoded: " + err.Error()})
			continue
		}
		if err := e.b.CreateLink(ctx, p.EntityKey, d); err != nil {
			res.Failures = append(res.Failures, Failure{Key: p.EntityKey, Error: err.Error()})
			continue
		}
		if err := e.repo.MarkCommitted(ctx, profileID, []journal.PendingChange{p}); err != nil {
			res.Failures = append(res.Failures, Failure{Key: p.EntityKey, Error: "linked in Jira but the journal could not be cleared: " + err.Error()})
			continue
		}
		if err := e.repo.ClearDetail(ctx, profileID, p.EntityKey); err != nil {
			log.Printf("tam: clear detail for %s after a link push: %v", p.EntityKey, err)
		}
		res.Linked = append(res.Linked, Linked{Key: p.EntityKey, ToKey: d.ToKey, Type: d.Type})
	}
}
```

`commitCreate` already marks only the rows it was given; since link rows are no longer in `byKey`, a draft's link row survives `Rekey` under the real key and the link pass finds it. Confirm by reading `commitCreate`: it receives `byKey[tempKey]`, which now holds only the create row.

Run `go test ./internal/committer/ -count=1` inside `tam/`. Expected: PASS, nine tests.

- [ ] **Step 4: Bound methods**

Create `tam/app_imports.go`:

```go
package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/core/importfile"
	"agile-suite/tam/internal/importer"
)

// decodeImport base64-decodes an uploaded file and parses it into rows.
func decodeImport(contentB64 string, isXlsx bool) ([][]string, error) {
	data, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		return nil, fmt.Errorf("decode upload: %w", err)
	}
	return importfile.ParseRecords(data, isXlsx)
}

// PreviewImport parses an uploaded file's header row and counts its data
// rows so the dialog can offer column mapping.
func (a *App) PreviewImport(contentB64 string, isXlsx bool) (importfile.Preview, error) {
	if err := a.requireStore(); err != nil {
		return importfile.Preview{}, err
	}
	records, err := decodeImport(contentB64, isXlsx)
	if err != nil {
		return importfile.Preview{}, err
	}
	return importfile.ParsePreview(records)
}

// AutoMapImport guesses the column for each draft field from the headers.
func (a *App) AutoMapImport(headers []string) importer.Mapping {
	return importer.AutoMap(headers)
}

// ImportIssues validates the file against the mapping and, unless dryRun,
// creates a draft per valid row. It refuses while a sync or commit runs.
func (a *App) ImportIssues(profileID, contentB64 string, isXlsx bool, fileName string, mapping importer.Mapping, dryRun bool) (importer.Result, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return importer.Result{}, err
	}
	if err := a.acquire(p.ID, "import"); err != nil {
		return importer.Result{}, err
	}
	defer a.release(p.ID)
	records, err := decodeImport(contentB64, isXlsx)
	if err != nil {
		return importer.Result{}, err
	}
	reqType, err := a.repo.ProfileSetting(a.ctx, p.ID, settingRequirementType)
	if err != nil {
		return importer.Result{}, err
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = "an uploaded file"
	}
	res, err := importer.Run(a.ctx, a.repo, p.ID, p.ProjectKey, reqType, records, mapping, fileName, dryRun)
	if err != nil {
		return res, err
	}
	if !dryRun {
		log.Printf("tam: imported %d drafts from %s into %s (%d rows skipped)", len(res.Created), fileName, p.ProjectKey, len(res.Errors))
	}
	return res, nil
}

// SaveImportTemplate writes the starter CSV where the user chooses and
// returns the path, or "" when the dialog was cancelled.
func (a *App) SaveImportTemplate() (string, error) {
	if a.ctx == nil {
		return "", errors.New("the window is not ready")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save import template",
		DefaultFilename: "tam-import-template.csv",
		Filters:         []runtime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, importer.TemplateCSV(), 0o644); err != nil {
		return "", fmt.Errorf("write template: %w", err)
	}
	return path, nil
}
```

`ProfileSetting` returns the default requirement type name when unset only if the repository does so; check `tam/internal/issuerepo/state.go`: if it returns `""` for an unset key, that is fine, since `importer.logicalType` treats an empty requirement type as "Requirement" only for the error text and still maps the literal `requirement`. If the Jira backend's default (`DefaultRequirementType = "Requirement"`) should also be accepted, pass `jirabackend.DefaultRequirementType` when `reqType` is empty: add `if reqType == "" { reqType = jirabackend.DefaultRequirementType }` (import `jirabackend "agile-suite/tam/internal/backend/jira"`, already imported in `app_issues.go`).

Append to `tam/app_writes.go`:

```go
// GetLinkTypes lists the link types the profile's Jira defines.
func (a *App) GetLinkTypes(profileID string) ([]backend.LinkType, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	b, err := a.backendFor(p)
	if err != nil {
		return nil, err
	}
	types, err := b.LinkTypes(a.ctx)
	if err != nil {
		return nil, err
	}
	if types == nil {
		types = []backend.LinkType{}
	}
	return types, nil
}

// LookupIssue reads one issue from Jira by key, any project, so the Add
// link form can confirm a target and show its summary.
func (a *App) LookupIssue(profileID, key string) (backend.Issue, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return backend.Issue{}, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return backend.Issue{}, errors.New("issue key is empty")
	}
	b, err := a.backendFor(p)
	if err != nil {
		return backend.Issue{}, err
	}
	return b.GetIssue(a.ctx, key)
}

// AddLink journals a link from key to the draft's target.
func (a *App) AddLink(profileID, key string, link backend.LinkDraft) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	return a.repo.AddLink(a.ctx, p.ID, strings.TrimSpace(key), link)
}
```

- [ ] **Step 5: Build, vet, test, generate**

Run (inside `tam/`):

```bash
gofmt -l . ./internal && go build ./... && go vet ./... && go test ./... -count=1
wails generate module
grep -n "PreviewImport\|ImportIssues\|SaveImportTemplate\|GetLinkTypes\|LookupIssue\|AddLink\|AutoMapImport" frontend/wailsjs/go/main/App.d.ts
grep -n "^export namespace" frontend/wailsjs/go/models.ts
grep -n "class Preview\|class Mapping\|class Result\|class LinkType\|class LinkDraft\|class Linked" frontend/wailsjs/go/models.ts
git checkout -- frontend/wailsjs/runtime frontend/package.json.md5 go.mod
git status --short --untracked-files=no
```

Expected: gates clean; the seven methods in `App.d.ts`; namespaces now include `importfile` and `importer`; the classes present (note `importer.Result` and `committer.Result` are distinct namespaces). The status shows only `app_writes.go`, `app_imports.go`, the committer files, and the three generated files under `frontend/wailsjs/go`.

From the repo root: `npm run typecheck --workspace tam/frontend && npm test --workspace tam/frontend 2>&1 | grep "Tests "`. Expected: clean and 56 passing (no frontend source changed).

- [ ] **Step 6: Commit**

```bash
git add tam/internal/committer tam/app_writes.go tam/app_imports.go tam/frontend/wailsjs/go
git commit -m "feat(tam): links in the commit pass, import and link bindings"
```

---

### Task 6: The Import issues dialog

File pick, preview, column mapping, dry run, import, template.

**Files:**
- Modify: `tam/frontend/src/api.ts`, `modals.ts`, `components/BacklogView.tsx`, `BacklogView.test.tsx`, `App.test.tsx`, `App.css`
- Create: `tam/frontend/src/components/ImportIssuesModal.tsx`, `ImportIssuesModal.test.tsx`
- Test: `npm test --workspace tam/frontend -- ImportIssuesModal BacklogView App` from the repo root

**Interfaces:**
- Consumes: the generated bindings (Task 5), `invalidateWrites`, `useSync().status`, `useNotice`.
- Produces (in `api.ts`): `ImportPreview{headers, rowCount}`, `ImportMapping{type, summary, description, priority, labels, assignee, storyPoints, parentKey}`, `ImportRowError{row, message}`, `ImportResult{rows, created, errors}`, `IMPORT_FIELDS`, the four bound functions; `ModalId` gains `"import"`; `ImportIssuesModal({ onClose, onImported })`; `readFileAsBase64(file): Promise<string>`.

- [ ] **Step 1: `api.ts` and `modals.ts`**

In `tam/frontend/src/api.ts`, after `CommitResult`, add:

```ts
export interface ImportPreview {
  headers: string[];
  rowCount: number;
}

export interface ImportMapping {
  type: string;
  summary: string;
  description: string;
  priority: string;
  labels: string;
  assignee: string;
  storyPoints: string;
  parentKey: string;
}

export interface ImportRowError {
  row: number;
  message: string;
}

export interface ImportResult {
  rows: number;
  created: string[];
  errors: ImportRowError[];
}

// IMPORT_FIELDS are the draft fields a column can feed, in dialog order.
export const IMPORT_FIELDS: { id: keyof ImportMapping; label: string }[] = [
  { id: "type", label: "Type" },
  { id: "summary", label: "Summary" },
  { id: "description", label: "Description" },
  { id: "priority", label: "Priority" },
  { id: "labels", label: "Labels" },
  { id: "assignee", label: "Assignee" },
  { id: "storyPoints", label: "Story points" },
  { id: "parentKey", label: "Parent key" },
];

// readFileAsBase64 reads a browser File into the base64 the import
// bindings take (the data URL's payload, after the comma).
export function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("The file could not be read."));
    reader.onload = () => {
      const url = String(reader.result ?? "");
      resolve(url.slice(url.indexOf(",") + 1));
    };
    reader.readAsDataURL(file);
  });
}
```

After `ListActivity`, add the bindings:

```ts
export const PreviewImport: (contentB64: string, isXlsx: boolean) => Promise<ImportPreview> = App.PreviewImport;
export const AutoMapImport: (headers: string[]) => Promise<ImportMapping> = App.AutoMapImport;
export const ImportIssues = (
  profileId: string,
  contentB64: string,
  isXlsx: boolean,
  fileName: string,
  mapping: ImportMapping,
  dryRun: boolean,
): Promise<ImportResult> =>
  App.ImportIssues(profileId, contentB64, isXlsx, fileName, importer.Mapping.createFrom(mapping), dryRun) as Promise<ImportResult>;
export const SaveImportTemplate: () => Promise<string> = App.SaveImportTemplate;
```

Add `importer` to the models import: `import { backend, importer, issuerepo } from "../wailsjs/go/models";`.

`tam/frontend/src/modals.ts`:

```ts
export type ModalId = "profiles" | "about" | "pending" | "newIssue" | "import";
```

- [ ] **Step 2: Dialog test**

Create `tam/frontend/src/components/ImportIssuesModal.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { ImportIssuesModal } from "./ImportIssuesModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    PreviewImport: vi.fn(),
    AutoMapImport: vi.fn(),
    ImportIssues: vi.fn(),
    SaveImportTemplate: vi.fn(),
  };
});

vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ status: "idle" }) }));

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderModal(onImported = vi.fn(), onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <Loader />
          <ImportIssuesModal onClose={onClose} onImported={onImported} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onImported, onClose };
}

const csv = "Issue Type,Summary,Points\nStory,Apply promo,5\nTask,,\n";
const mapping: api.ImportMapping = { type: "Issue Type", summary: "Summary", description: "", priority: "", labels: "", assignee: "", storyPoints: "Points", parentKey: "" };

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.PreviewImport).mockResolvedValue({ headers: ["Issue Type", "Summary", "Points"], rowCount: 2 });
  vi.mocked(api.AutoMapImport).mockResolvedValue(mapping);
  vi.mocked(api.SaveImportTemplate).mockResolvedValue("C:/tam-import-template.csv");
});

async function pickFile(user: ReturnType<typeof userEvent.setup>) {
  const input = await screen.findByLabelText("File");
  await user.upload(input, new File([csv], "backlog.csv", { type: "text/csv" }));
  await waitFor(() => expect(api.PreviewImport).toHaveBeenCalled());
}

describe("ImportIssuesModal", () => {
  it("previews the file, pre-fills the mapping, and dry runs", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue({ rows: 2, created: [], errors: [{ row: 3, message: "Summary is empty." }] });
    renderModal();
    await pickFile(user);
    const [b64, isXlsx] = vi.mocked(api.PreviewImport).mock.calls[0];
    expect(atob(b64)).toBe(csv);
    expect(isXlsx).toBe(false);
    expect(await screen.findByText("3 columns, 2 rows")).toBeInTheDocument();
    expect(screen.getByLabelText("Summary")).toHaveValue("Summary");
    expect(screen.getByLabelText("Story points")).toHaveValue("Points");
    expect(screen.getByLabelText("Assignee")).toHaveValue("");
    await user.click(screen.getByRole("button", { name: "Dry run" }));
    await waitFor(() => expect(api.ImportIssues).toHaveBeenCalledWith("p1", b64, false, "backlog.csv", mapping, true));
    expect(await screen.findByText("Dry run: 1 row would become a draft, 1 would be skipped.")).toBeInTheDocument();
    expect(screen.getByText("Row 3")).toBeInTheDocument();
    expect(screen.getByText("Summary is empty.")).toBeInTheDocument();
  });

  it("imports with the edited mapping and reports the drafts", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue({ rows: 2, created: ["TAM-NEW-1"], errors: [{ row: 3, message: "Summary is empty." }] });
    const { onImported } = renderModal();
    await pickFile(user);
    await user.selectOptions(await screen.findByLabelText("Priority"), "Points");
    await user.click(screen.getByRole("button", { name: "Import" }));
    await waitFor(() => expect(api.ImportIssues).toHaveBeenCalledWith("p1", expect.any(String), false, "backlog.csv", { ...mapping, priority: "Points" }, false));
    expect(await screen.findByText("Imported 1 draft; 1 row was skipped.")).toBeInTheDocument();
    expect(onImported).toHaveBeenCalledWith(["TAM-NEW-1"]);
  });

  it("shows a parse error and offers the template", async () => {
    const user = userEvent.setup();
    vi.mocked(api.PreviewImport).mockRejectedValue(new Error("open xlsx: zip: not a valid zip file"));
    renderModal();
    const input = await screen.findByLabelText("File");
    await user.upload(input, new File(["junk"], "bad.xlsx", { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" }));
    expect(await screen.findByText(/not a valid zip file/)).toBeInTheDocument();
    expect(vi.mocked(api.PreviewImport).mock.calls[0][1]).toBe(true);
    await user.click(screen.getByRole("button", { name: "Download template" }));
    await waitFor(() => expect(api.SaveImportTemplate).toHaveBeenCalled());
    expect(await screen.findByText("Template saved to C:/tam-import-template.csv")).toBeInTheDocument();
  });

  it("refuses to import without a Summary column", async () => {
    const user = userEvent.setup();
    vi.mocked(api.AutoMapImport).mockResolvedValue({ ...mapping, summary: "" });
    renderModal();
    await pickFile(user);
    await user.click(await screen.findByRole("button", { name: "Import" }));
    expect(await screen.findByText("Map a Summary column first.")).toBeInTheDocument();
    expect(api.ImportIssues).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 3: Run it**

Run from the repo root: `npm test --workspace tam/frontend -- ImportIssuesModal`
Expected: FAIL, the component does not exist.

- [ ] **Step 4: The dialog**

Create `tam/frontend/src/components/ImportIssuesModal.tsx`:

```tsx
import { useState } from "react";
import type { ChangeEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, call, errMsg, useProfile } from "@agile-suite/core";
import { AutoMapImport, IMPORT_FIELDS, ImportIssues, PreviewImport, SaveImportTemplate, readFileAsBase64 } from "../api";
import type { ImportMapping, ImportPreview, ImportResult, Profile, Settings } from "../api";
import { invalidateWrites } from "../queries/invalidate";
import { useSync } from "../contexts/SyncContext";

interface Props {
  onClose: () => void;
  onImported: (keys: string[]) => void;
}

interface Picked {
  name: string;
  b64: string;
  isXlsx: boolean;
}

const EMPTY: ImportMapping = { type: "", summary: "", description: "", priority: "", labels: "", assignee: "", storyPoints: "", parentKey: "" };

function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

// resultLine words a dry run or an import.
export function resultLine(r: ImportResult, dryRun: boolean): string {
  const skipped = r.errors.length;
  if (dryRun) {
    const ok = r.rows - skipped;
    return `Dry run: ${ok} ${ok === 1 ? "row would become a draft" : "rows would become drafts"}, ${skipped} would be skipped.`;
  }
  return `Imported ${plural(r.created.length, "draft", "drafts")}; ${plural(skipped, "row was", "rows were")} skipped.`;
}

// ImportIssuesModal turns a CSV or XLSX into drafts: pick, preview, map,
// dry run, import. The file's bytes go to the backend base64-encoded.
export function ImportIssuesModal({ onClose, onImported }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const { status } = useSync();
  const [picked, setPicked] = useState<Picked | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [mapping, setMapping] = useState<ImportMapping>(EMPTY);
  const [result, setResult] = useState<{ r: ImportResult; dryRun: boolean } | null>(null);
  const [error, setError] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const locked = busy || status !== "idle";

  async function onFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setError("");
    setResult(null);
    setPreview(null);
    setBusy(true);
    try {
      const b64 = await readFileAsBase64(file);
      const isXlsx = /\.xlsx$/i.test(file.name);
      const pv = await call(() => PreviewImport(b64, isXlsx));
      setPicked({ name: file.name, b64, isXlsx });
      setPreview(pv);
      setMapping(await call(() => AutoMapImport(pv.headers)));
    } catch (err) {
      setPicked(null);
      setError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  async function run(dryRun: boolean) {
    if (!picked) return;
    if (!mapping.summary) {
      setError("Map a Summary column first.");
      return;
    }
    setError("");
    setBusy(true);
    try {
      const r = await call(() => ImportIssues(activeId, picked.b64, picked.isXlsx, picked.name, mapping, dryRun));
      setResult({ r, dryRun });
      if (!dryRun && r.created.length > 0) {
        invalidateWrites(qc, activeId);
        onImported(r.created);
      }
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  async function template() {
    try {
      const path = await call(() => SaveImportTemplate());
      setNote(path ? `Template saved to ${path}` : "");
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <Modal onClose={onClose} className="modal import-modal" labelledBy="import-title">
      <div className="pending-head">
        <h2 id="import-title">Import issues</h2>
        <span className="muted">{activeProfile ? `into ${activeProfile.projectKey}` : ""}</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <div className="import-file">
        <label className="edit-row" htmlFor="import-file">
          <span className="muted small">File</span>
          <input id="import-file" type="file" accept=".csv,.xlsx" onChange={(e) => void onFile(e)} disabled={locked} />
        </label>
        {preview && picked && (
          <p className="muted small">{picked.name}: {`${plural(preview.headers.length, "column", "columns")}, ${plural(preview.rowCount, "row", "rows")}`}</p>
        )}
        <button type="button" className="btn btn-ghost" onClick={() => void template()}>Download template</button>
        {note && <span className="muted small" role="status">{note}</span>}
      </div>

      {preview && (
        <div className="import-mapping">
          <div className="import-mapping-head"><span className="muted small b">Field</span><span className="muted small b">Column</span></div>
          {IMPORT_FIELDS.map((f) => (
            <label key={f.id} className="edit-row" htmlFor={`map-${f.id}`}>
              <span className="muted small">{f.label}</span>
              <select id={`map-${f.id}`} className="detail-input" value={mapping[f.id]} onChange={(e) => setMapping((m) => ({ ...m, [f.id]: e.target.value }))} disabled={locked}>
                <option value="">(not mapped)</option>
                {preview.headers.map((h, i) => (
                  <option key={`${i}-${h}`} value={h}>{h || `(column ${i + 1})`}</option>
                ))}
              </select>
            </label>
          ))}
        </div>
      )}

      {result && (
        <div className={`pending-banner${result.r.errors.length ? " pending-banner-warn" : ""}`} role="status">
          <p className="b">{resultLine(result.r, result.dryRun)}</p>
          {result.r.errors.map((e) => (
            <p key={`${e.row}-${e.message}`} className="small">
              <span className="danger-text">{`Row ${e.row}`}</span>{" "}
              <span>{e.message}</span>
            </p>
          ))}
          <p className="muted small">Types default to Task. Drafts join the Backlog now; Commit creates them in Jira.</p>
        </div>
      )}

      {error && <p className="error-text small" role="alert">{error}</p>}

      <div className="pending-footer">
        <span className="muted small">Import runs in one transaction; skipped rows stay in the file for a second pass.</span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" disabled={!picked || locked} onClick={() => void run(true)}>Dry run</button>
          <button type="button" className="btn btn-primary" disabled={!picked || locked} onClick={() => void run(false)}>
            {result?.dryRun ? `Import ${result.r.rows - result.r.errors.length}` : "Import"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
```

The Import button reads "Import" until a dry run has run, then "Import N"; the tests click it before any dry run, so its name is "Import" there.

- [ ] **Step 5: The Backlog button and the shell test**

In `tam/frontend/src/components/BacklogView.tsx`, import `ImportIssuesModal` and, just before the New button, add:

```tsx
        <button type="button" className="btn filter-import" disabled={!activeId} onClick={() => openModal("import")}>
          Import
        </button>
```

and, next to the `NewIssueModal` render:

```tsx
      {isOpen("import") && (
        <ImportIssuesModal
          onClose={closeModal}
          onImported={(keys) => {
            setPage(0);
            setSelectedKey(keys[0] ?? "");
          }}
        />
      )}
```

In `BacklogView.test.tsx`, add to the api mock `PreviewImport: vi.fn(), AutoMapImport: vi.fn(), ImportIssues: vi.fn(), SaveImportTemplate: vi.fn()`, and a test:

```tsx
  it("opens the Import issues dialog from the toolbar", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "Import" }));
    expect(await screen.findByRole("dialog", { name: "Import issues" })).toBeInTheDocument();
  });
```

If `BacklogView.test.tsx` does not already mock `../contexts/SyncContext` (Task 8 of plan 1b added `vi.mock("../contexts/SyncContext", ...)` returning `{ status: "idle" }` for the panel), the dialog's `useSync` needs it: reuse that mock. In `App.test.tsx`, add the same four bindings to the `./api` mock so the shell still compiles under the mock.

- [ ] **Step 6: CSS**

Append to `tam/frontend/src/App.css`:

```css
/* Plan 1c: import */
.filter-import { margin-left: auto; }
.filter-import + .filter-new { margin-left: 8px; }
.import-modal { width: min(640px, 92vw); max-height: 88vh; overflow: auto; }
.import-file { display: flex; flex-direction: column; gap: 6px; margin-top: 8px; }
.import-mapping { display: flex; flex-direction: column; gap: 6px; margin-top: 12px; }
.import-mapping-head { display: grid; grid-template-columns: 96px 1fr; gap: 8px; border-bottom: 1px solid var(--border); padding-bottom: 4px; }
```

The `.filter-new { margin-left: auto; }` rule from plan 1b now belongs to `.filter-import`; remove `margin-left: auto` from `.filter-new`.

- [ ] **Step 7: Run everything**

From the repo root:

```bash
npm run typecheck --workspace tam/frontend && npm test --workspace tam/frontend 2>&1 | grep -E "Tests |FAIL"
```

Expected: clean; 56 plus five new tests pass.

- [ ] **Step 8: Commit**

```bash
git add tam/frontend/src
git commit -m "feat(tam): Import issues dialog with mapping, dry run, and template"
```

---

### Task 7: Links on the panel, in the Pending changes dialog, on the Activity tab; the requirement type

**Files:**
- Modify: `tam/frontend/src/api.ts`, `queries/keys.ts`, `queries/pending.ts`, `components/IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx`, `PendingChangesModal.tsx`, `PendingChangesModal.test.tsx`, `ActivityTab.tsx`, `NewIssueModal.tsx`, `NewIssueModal.test.tsx`, `App.css`
- Create: `tam/frontend/src/components/AddLinkForm.tsx`, `AddLinkForm.test.tsx`
- Test: `npm test --workspace tam/frontend` from the repo root

**Interfaces:**
- Consumes: `GetLinkTypes`, `LookupIssue`, `AddLink`, `DiscardPendingChange` bindings; `groupPending`; `describe`.
- Produces (in `api.ts`): `LinkType`, `LinkDraft`, `Link.pending?`, `Link.pendingId?`, `CommitResult.linked`, `linkPhrase(type, direction, key)`; `keys.linkTypes(profileId)`; `useLinkTypes`, `useAddLink`, `useDiscardById`; `PendingGroup.links`; `AddLinkForm({ profileId, issueKey, onAdded })`.

- [ ] **Step 1: `api.ts` shapes and bindings**

Extend `Link`:

```ts
export interface Link {
  direction: "inward" | "outward" | string;
  type: string;
  key: string;
  summary: string;
  issueType: string;
  pending?: boolean;
  pendingId?: number;
}
```

After `Conflict`, add:

```ts
export interface LinkType {
  name: string;
  inward: string;
  outward: string;
}

export interface LinkDraft {
  type: string;
  direction: "outward" | "inward";
  toKey: string;
  toSummary: string;
  toType: string;
}

// linkPhrase words a link the way Jira does: "PLAT-1 blocks PAY-7" reads
// from the type's outward wording, "is blocked by" from its inward one.
export function linkPhrase(types: LinkType[], type: string, direction: string): string {
  const t = types.find((x) => x.name === type);
  if (!t) return direction === "inward" ? `${type} (inward)` : type;
  return direction === "inward" ? t.inward : t.outward;
}
```

`CommitResult` gains `linked: { key: string; toKey: string; type: string }[];` after `created`.

After `SaveImportTemplate`, add:

```ts
export const GetLinkTypes: (profileId: string) => Promise<LinkType[]> = App.GetLinkTypes;
export const LookupIssue: (profileId: string, key: string) => Promise<Issue> = App.LookupIssue;
export const AddLink = (profileId: string, key: string, link: LinkDraft): Promise<void> =>
  App.AddLink(profileId, key, backend.LinkDraft.createFrom(link));
```

- [ ] **Step 2: Keys, hooks, grouping**

`queries/keys.ts` gains `linkTypes: (profileId: string) => [profileId, "linkTypes"] as const`.

In `queries/pending.ts`, add hooks and extend the grouping:

```ts
export function useLinkTypes(profileId: string) {
  return useQuery({
    queryKey: keys.linkTypes(profileId),
    queryFn: () => call(() => GetLinkTypes(profileId)),
    enabled: !!profileId,
    staleTime: 10 * 60 * 1000,
  });
}

export function useAddLink(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, link }: { key: string; link: LinkDraft }) => call(() => AddLink(profileId, key, link)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

// useDiscardById discards a journal row known only by id and issue key,
// which is what the Links tab has for a pending link.
export function useDiscardById(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: { id: number; key: string }) => call(() => DiscardPendingChange(profileId, id)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}
```

(import `AddLink`, `GetLinkTypes`, and the `LinkDraft` type from `../api`). `PendingGroup` gains `links: { row: PendingChange; link: LinkDraft }[]`, initialised to `[]` in `groupPending`, and the loop gets a branch before the `else`:

```ts
    } else if (row.entityType === "link") {
      try {
        g.links.push({ row, link: JSON.parse(row.afterVal) as LinkDraft });
      } catch {
        g.edits.push(row);
      }
    } else {
```

- [ ] **Step 3: Tests for the form, the panel, the dialog, the activity, the New dialog**

Create `tam/frontend/src/components/AddLinkForm.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import { AddLinkForm } from "./AddLinkForm";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetLinkTypes: vi.fn(), LookupIssue: vi.fn(), AddLink: vi.fn() };
});

function renderForm(onAdded = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <AddLinkForm profileId="p1" issueKey="PLAT-412" onAdded={onAdded} />
    </QueryClientProvider>,
  );
  return onAdded;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetLinkTypes).mockResolvedValue([
    { name: "Blocks", inward: "is blocked by", outward: "blocks" },
    { name: "Relates", inward: "relates to", outward: "relates to" },
  ]);
  vi.mocked(api.LookupIssue).mockResolvedValue({
    key: "PAY-77", id: "9", project: "PAY", type: "task", summary: "Rotate gateway signing keys", status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
  });
  vi.mocked(api.AddLink).mockResolvedValue();
});

describe("AddLinkForm", () => {
  it("lists both phrasings of each type once, checks the target, and adds", async () => {
    const user = userEvent.setup();
    const onAdded = renderForm();
    const select = await screen.findByLabelText("Link");
    const labels = Array.from(select.querySelectorAll("option")).map((o) => o.textContent);
    expect(labels).toEqual(["blocks", "is blocked by", "relates to"]);
    await user.selectOptions(select, "Blocks|inward");
    await user.type(screen.getByLabelText("Issue key"), "pay-77");
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Check" }));
    expect(await screen.findByText("PAY-77, Task, Rotate gateway signing keys")).toBeInTheDocument();
    expect(api.LookupIssue).toHaveBeenCalledWith("p1", "PAY-77");
    await user.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() => expect(api.AddLink).toHaveBeenCalledWith("p1", "PLAT-412", {
      type: "Blocks", direction: "inward", toKey: "PAY-77", toSummary: "Rotate gateway signing keys", toType: "task",
    }));
    expect(onAdded).toHaveBeenCalled();
    expect(screen.getByLabelText("Issue key")).toHaveValue("");
  });

  it("shows lookup and add errors", async () => {
    const user = userEvent.setup();
    vi.mocked(api.LookupIssue).mockRejectedValueOnce(new Error("GET failed: 404"));
    renderForm();
    await screen.findByLabelText("Link");
    await user.type(screen.getByLabelText("Issue key"), "NOPE-1");
    await user.click(screen.getByRole("button", { name: "Check" }));
    expect(await screen.findByText(/404/)).toBeInTheDocument();
    await user.clear(screen.getByLabelText("Issue key"));
    await user.type(screen.getByLabelText("Issue key"), "PAY-77");
    await user.click(screen.getByRole("button", { name: "Check" }));
    await screen.findByText("PAY-77, Task, Rotate gateway signing keys");
    vi.mocked(api.AddLink).mockRejectedValueOnce(new Error("a link from PLAT-412 to PAY-77 is already pending"));
    await user.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findByText(/already pending/)).toBeInTheDocument();
  });
});
```

In `IssueDetailPanel.test.tsx`, add `DiscardPendingChange: vi.fn()` and `GetLinkTypes: vi.fn()` to the mock (with `GetLinkTypes` resolving `[]` in `beforeEach`), and a test:

```tsx
  it("marks a pending link on the Links tab and discards it", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetIssueDetail).mockResolvedValue({
      key: "PLAT-412", description: "d", fields: {},
      links: [
        { direction: "inward", type: "Tested By", key: "XT-1018", summary: "Promo code applies discount", issueType: "Test" },
        { direction: "outward", type: "Relates", key: "XT-1031", summary: "Retried payment is not charged twice", issueType: "Test", pending: true, pendingId: 41 },
      ],
    });
    vi.mocked(api.DiscardPendingChange).mockResolvedValue();
    renderPanel();
    await user.click(await screen.findByRole("tab", { name: "Links" }));
    const row = (await screen.findByText("XT-1031")).closest("li")!;
    expect(row).toHaveTextContent("pending");
    await user.click(within(row).getByRole("button", { name: "Discard link to XT-1031" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 41));
    expect(screen.getByRole("heading", { name: "Add link" })).toBeInTheDocument();
  });
```

In `PendingChangesModal.test.tsx`, add a test:

```tsx
  it("shows a link card and counts pushed links in the banner", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce([
      { id: 9, entityType: "link", entityKey: "PLAT-412", field: "Relates|outward|XT-1018", beforeVal: "", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ type: "Relates", direction: "outward", toKey: "XT-1018", toSummary: "Promo code applies discount", toType: "Test" }) },
    ]).mockResolvedValue([]);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({ committed: [], created: [], linked: [{ key: "PLAT-412", toKey: "XT-1018", type: "Relates" }], conflicts: [], failures: [], remaining: 0 });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    const card = await within(dialog).findByRole("group", { name: "PLAT-412" });
    expect(card).toHaveTextContent("Relates (outward) XT-1018 Promo code applies discount");
    expect(within(card).getByRole("button", { name: "Discard link to XT-1018" })).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Commit (1)" }));
    expect(await within(dialog).findByText("Last commit: 1 link pushed.")).toBeInTheDocument();
  });
```

Every `CommitPendingChanges` mock in that file gains `linked: []` (the type now requires it). In `IssueDetailPanel.test.tsx`'s Activity fixture, add a link row and assert its sentence: `{ id: 3, occurredAt: "2026-09-06T10:06:00Z", actor: "araha", entityType: "link", entityKey: "PLAT-412", action: "link", field: "Relates|outward|XT-1018", beforeVal: "", afterVal: "XT-1018", note: "" }` and `expect(items[0]).toHaveTextContent("araha added a link: Relates (outward) to XT-1018")` (the item order puts the newest first, so adjust the earlier indexes by one).

In `NewIssueModal.test.tsx`, add:

```tsx
  it("offers Requirement and hides story points for it", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    expect(within(dialog).getByLabelText("Story points")).toBeInTheDocument();
    await user.selectOptions(within(dialog).getByLabelText("Type"), "requirement");
    expect(within(dialog).queryByLabelText("Story points")).not.toBeInTheDocument();
    await user.type(within(dialog).getByLabelText("Summary"), "Single-use promo codes");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    await waitFor(() => expect(api.CreateIssue).toHaveBeenCalled());
    const draft = vi.mocked(api.CreateIssue).mock.calls[0][1];
    expect(draft.type).toBe("requirement");
    expect(draft.storyPoints).toBeNull();
  });
```

- [ ] **Step 4: Run them**

Run from the repo root: `npm test --workspace tam/frontend -- AddLinkForm IssueDetailPanel PendingChangesModal NewIssueModal`
Expected: the new tests FAIL.

- [ ] **Step 5: `AddLinkForm.tsx`**

```tsx
import { useState } from "react";
import type { FormEvent } from "react";
import { call, errMsg } from "@agile-suite/core";
import { LookupIssue } from "../api";
import type { Issue, LinkDraft } from "../api";
import { useAddLink, useLinkTypes } from "../queries/pending";

interface Props {
  profileId: string;
  issueKey: string;
  onAdded: () => void;
}

// AddLinkForm journals a link from the panel's issue to any key. The
// phrasing select lists each type's outward and inward wording (once when
// they read the same); Check confirms the target through Jira and shows
// its summary; Add journals it for the next Commit.
export function AddLinkForm({ profileId, issueKey, onAdded }: Props) {
  const types = useLinkTypes(profileId);
  const add = useAddLink(profileId);
  const [choice, setChoice] = useState("");
  const [key, setKey] = useState("");
  const [target, setTarget] = useState<Issue | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");

  const options: { value: string; label: string }[] = [];
  for (const t of types.data ?? []) {
    options.push({ value: `${t.name}|outward`, label: t.outward });
    if (t.inward !== t.outward) options.push({ value: `${t.name}|inward`, label: t.inward });
  }
  const selected = choice || options[0]?.value || "";

  async function check() {
    const k = key.trim().toUpperCase();
    setError("");
    setTarget(null);
    if (!k) {
      setError("Enter an issue key.");
      return;
    }
    setChecking(true);
    try {
      setTarget(await call(() => LookupIssue(profileId, k)));
      setKey(k);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setChecking(false);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!target || !selected) return;
    const [type, direction] = selected.split("|") as [string, "outward" | "inward"];
    const link: LinkDraft = { type, direction, toKey: target.key, toSummary: target.summary, toType: target.type };
    setError("");
    try {
      await add.mutateAsync({ key: issueKey, link });
      setKey("");
      setTarget(null);
      onAdded();
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <form className="add-link" onSubmit={(e) => void onSubmit(e)} aria-labelledby="add-link-title">
      <h3 id="add-link-title">Add link</h3>
      <label className="edit-row" htmlFor="link-type">
        <span className="muted small">Link</span>
        <select id="link-type" className="detail-input" value={selected} onChange={(e) => setChoice(e.target.value)} disabled={types.isPending || options.length === 0}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      </label>
      <label className="edit-row" htmlFor="link-key">
        <span className="muted small">Issue key</span>
        <span className="add-link-key">
          <input id="link-key" className="detail-input" type="text" value={key} placeholder="Any project, for example PAY-77" onChange={(e) => { setKey(e.target.value); setTarget(null); }} />
          <button type="button" className="btn" onClick={() => void check()} disabled={checking || !key.trim()}>{checking ? "Checking" : "Check"}</button>
        </span>
      </label>
      {target && (
        <p className="small">{`${target.key}, ${target.type ? target.type[0].toUpperCase() + target.type.slice(1) : "Issue"}, ${target.summary}`}</p>
      )}
      {types.isError && <p className="error-text small">Link types could not be read: {types.error.message}</p>}
      {error && <p className="error-text small" role="alert">{error}</p>}
      <div className="edit-actions">
        <button type="submit" className="btn btn-primary" disabled={!target || !selected || add.isPending}>Add</button>
        <span className="muted small">Journaled now, pushed on Commit.</span>
      </div>
    </form>
  );
}
```

- [ ] **Step 6: The Links tab, the dialog card, the activity wording, the New dialog**

In `IssueDetailPanel.tsx`, import `AddLinkForm` and `useDiscardById`, and render the form under the links (inside the Links tab panel, after the `LinkGroups`/empty-state block):

```tsx
          <AddLinkForm profileId={profileId} issueKey={issue.key} onAdded={() => void detail.refetch()} />
```

Change `LinkGroups` to take `profileId` and mark pending links; the discard mutation lives in the panel and is passed down:

```tsx
function LinkGroups({ links, onDiscard }: { links: Link[]; onDiscard: (id: number, key: string) => void }) {
  ...
              <li key={`${l.direction}-${l.key}`} className="linked-row">
                <span className="accent-text linked-key">{l.key}</span>
                <span>{l.summary}</span>
                <span className="muted small">{l.issueType}</span>
                {l.pending && (
                  <>
                    <span className="pending-dot" role="img" aria-label="Pending changes" />
                    <span className="muted small">pending</span>
                    <button type="button" className="btn btn-ghost" aria-label={`Discard link to ${l.key}`} onClick={() => onDiscard(l.pendingId ?? 0, l.key)}>Discard</button>
                  </>
                )}
              </li>
```

with `const discardLink = useDiscardById(profileId);` in the panel and `<LinkGroups links={detail.data.links} onDiscard={(id, key) => discardLink.mutate({ id, key: issue.key }, { onError: () => undefined })} />`. The `key` passed to the mutation is the panel's issue (the row to refresh), not the link's target; drop the unused second argument if the linter objects.

In `PendingChangesModal.tsx`, inside the edit card (the `g.draft ? ... : <ul className="pending-rows">` branch), render link rows before the edit rows; a group with links and no edits and no draft also takes this branch:

```tsx
                    {g.links.map(({ row, link }) => (
                      <li key={row.id} className="pending-row pending-row-link">
                        <span className="muted pending-field">Link</span>{" "}
                        <span className="b">{`${link.type} (${link.direction})`}</span>{" "}
                        <span className="accent-text">{link.toKey}</span>{" "}
                        <span>{link.toSummary}</span>{" "}
                        <button type="button" className="btn btn-ghost" disabled={busy} aria-label={`Discard link to ${link.toKey}`} onClick={() => discardOne.mutate(row, { onError: onDiscardError })}>
                          Discard
                        </button>
                      </li>
                    ))}
```

In `bannerLine`, after the created part, add `if (r.linked.length) parts.push(plural(r.linked.length, "link pushed", "links pushed"));` and make the "nothing pushed" test include links: `if (!r.committed.length && !r.created.length && !r.linked.length)`. In `summaryLine` nothing changes (a link row counts as a change).

In `ActivityTab.tsx` `describe`, add before the `issue_create` branch:

```tsx
  if (a.entityType === "link") {
    const [type = "", direction = "", key = ""] = a.field.split("|");
    const what = `${type} (${direction}) to ${key}`;
    switch (a.action) {
      case "link":
        return `${a.actor} added a link: ${what}`;
      case "commit":
        return `${a.actor} pushed the link ${what}`;
      case "discard":
        return `${a.actor} discarded the link ${what}`;
    }
  }
```

In `NewIssueModal.tsx`: `const CREATABLE: IssueType[] = ["task", "story", "bug", "requirement"];`, update its comment (epics arrive later), wrap the Story points label in `{type !== "requirement" && ( ... )}`, and in `onSubmit` compute `storyPoints: type === "requirement" || points.trim() === "" ? null : Number(points.trim())`.

- [ ] **Step 7: CSS and the gates**

Append to `App.css`:

```css
/* Plan 1c: links */
.add-link { margin-top: 16px; padding-top: 12px; border-top: 1px solid var(--border); display: flex; flex-direction: column; gap: 8px; }
.add-link h3 { margin: 0; font-size: 13px; }
.add-link-key { display: flex; gap: 6px; }
.add-link-key .detail-input { flex: 1; }
.linked-row .pending-dot { margin-left: 4px; }
.pending-row-link { grid-template-columns: 110px auto auto 1fr auto; }
```

From the repo root:

```bash
npm run typecheck --workspace tam/frontend && npm test --workspace tam/frontend 2>&1 | grep -E "Tests |FAIL"
```

Expected: clean and green (61 plus six new tests).

- [ ] **Step 8: Commit**

```bash
git add tam/frontend/src
git commit -m "feat(tam): Add link form, pending links, link cards and activity, requirement drafts"
```

---

### Task 8: Docs and the whole-plan verification

**Files:**
- Modify: `tam/CLAUDE.md`, `README.md`, `docs/superpowers/specs/2026-09-05-tam-issues-design.md` (14.11)
- Test: every suite, then `wails build` inside `tam/`

- [ ] **Step 1: Docs**

In `tam/CLAUDE.md`, after the write-path section, add:

```markdown
## The write features (plan 1c)

Import: the Backlog's Import button takes a CSV or XLSX (parsed by
`core/importfile`, XTM's parser lifted out), maps columns to the eight draft
fields (`internal/importer`), validates rows with file row numbers, and
creates the valid rows as drafts in one transaction (`CreateDrafts`, audited
"imported from <file>"). Links: the Links tab's Add link form journals a
link (entity type `link`, field `<type>|<direction>|<target>`); the
repository merges pending links into the cached detail; the committer pushes
link rows after edits with `POST /rest/api/2/issueLink` and drops the
source's detail cache. Requirements are creatable; the demo asks for a
Source field on them and answers lookups for the `XT-` keys its curated
details reference. Link removal, links in the bulk sync, epics, and
subtask parents are not in scope.
```

Update the Layout section with `app_imports.go`, `internal/importer/`, `ImportIssuesModal`, and `AddLinkForm`. In `README.md`, add one sentence after the plan 1b one: "Plan 1c adds Excel import to drafts, cross-project links, and requirement creation." Append to the spec after 14.10:

```markdown
### 14.11 Implementation notes

Recorded when plan 1c was written. `core/importfile` exports the parser pieces XTM used privately so the delegators are one-for-one. `CreateDrafts` is the one transaction; `CreateDraft` calls it with a single draft. The parent key reaches Jira through the discovered Epic Link field only. Link rows are pushed in their own pass, read fresh after creates so a link from a draft follows the real key; `Link` carries `pending` and `pendingId` so the Links tab discards with the existing `DiscardPendingChange`. `LookupIssue` is the backend's `GetIssue`; the demo answers it for foreign `XT-` keys with synthetic rows.
```

- [ ] **Step 2: Whole-plan verification**

```bash
cd core && go vet ./... && go test ./... -count=1 && cd ..
cd xtm && go vet ./... && go test ./internal/... -count=1 && cd ..
cd tam && gofmt -l ./internal/importer ./internal/issuerepo/links.go ./internal/backend/jira/links.go && go vet ./... && go test ./... -count=1 && cd ..
npm run typecheck --workspaces --if-present
npm test --workspaces --if-present 2>&1 | grep -E "Tests |FAIL"
cd tam && wails build && cd ..
git status --short --untracked-files=no
```

Expected: every Go package `ok`; typecheck clean; Vitest core 46, XTM 159, TAM 67; `wails build` produces the exe; only wails line-ending churn to revert.

Then the offline walk-through with `wails dev` inside `tam/` on the demo profile:

1. Import, Download template, then pick the saved file: preview shows 8 columns and 1 row, the mapping is pre-filled, Dry run says 1 row would become a draft, Import creates `TAM-NEW-1` under PLAT-350 and the Backlog selects it.
2. Open PLAT-412, Links tab: choose "is blocked by", key `XT-1018`, Check shows "XT-1018, Issue, Promo code applies discount", Add. The link appears with the dot and Discard; the chip counts 2.
3. New, type Requirement: Story points is hidden and Source is required; create it.
4. Commit (3): the banner says 2 created (with the key mapping) and 1 link pushed; PLAT-412's Links tab refetches and shows the link without the dot.
5. Discard a pending link from the Links tab before a commit and see it go.

- [ ] **Step 3: Commit**

```bash
git add tam/CLAUDE.md README.md docs/superpowers/specs/2026-09-05-tam-issues-design.md
git commit -m "docs(tam): plan 1c notes for import, links, and requirement creation"
```

Then open the PR against `main` titled "Task Activity Manager issues, plan 1c: the write features", listing the eight tasks, the gates run, and the walk-through result. No AI attribution anywhere in it.

---

## Risky spots for the implementer

- **Task 1** copies XTM's bodies; the XLSX test builds a workbook with excelize inside the test, so no fixture file is needed.
- **Task 3**'s `discardOne` rewrite must keep the two existing branches byte for byte; the link branch only skips the revert.
- **Task 5**: `commitCreate` must not see link rows (the grouping skips them), or a draft's link would be deleted unpushed.
- **Task 6**: `readFileAsBase64` under jsdom works with `FileReader`; the test decodes with `atob` to check the payload.
- **Task 7**: `bannerLine` now has three "pushed" parts; keep the "nothing pushed" wording for the all-failed case.
