package testrepo_test

import (
	"strings"
	"testing"

	"xray-test-manager/internal/testrepo"
)

const importCSV = "Summary,Description,Priority,Labels,Folder\n" +
	"Login works,Verify login,High,smoke,/Auth\n" +
	",missing summary,Low,,\n" +
	"Logout works,Verify logout,Medium,regression,/Auth\n"

func TestParseImportPreviewReportsHeadersAndRows(t *testing.T) {
	repo := newRepo(t)
	pv, err := repo.ParseImportPreview(importCSV)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(pv.Headers) != 5 || pv.Headers[0] != "Summary" {
		t.Errorf("headers = %v, want 5 starting with Summary", pv.Headers)
	}
	if pv.RowCount != 3 {
		t.Errorf("RowCount = %d, want 3", pv.RowCount)
	}
}

func TestImportTestsDryRunReportsErrorsWithoutCreating(t *testing.T) {
	repo := newRepo(t)
	mapping := testrepo.ImportMapping{Summary: "Summary", Description: "Description", Priority: "Priority", Labels: "Labels", Folder: "Folder"}

	res, err := repo.ImportTests("p1", importCSV, mapping, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if res.Created != 2 {
		t.Errorf("Created = %d, want 2 valid rows", res.Created)
	}
	if len(res.Errors) != 1 || res.Errors[0].Row != 3 {
		t.Errorf("errors = %+v, want one for row 3 (empty summary)", res.Errors)
	}

	page, _ := repo.ListTests("p1", testrepo.Query{})
	if page.Total != 0 {
		t.Errorf("dry run must not create tests; got %d", page.Total)
	}
}

func TestImportTestsCreatesPendingTests(t *testing.T) {
	repo := newRepo(t)
	mapping := testrepo.ImportMapping{Summary: "Summary", Description: "Description", Priority: "Priority", Labels: "Labels", Folder: "Folder"}

	res, err := repo.ImportTests("p1", importCSV, mapping, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Created != 2 {
		t.Errorf("Created = %d, want 2", res.Created)
	}

	page, _ := repo.ListTests("p1", testrepo.Query{})
	if page.Total != 2 {
		t.Fatalf("expected 2 created tests, got %d", page.Total)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 2 {
		t.Errorf("want 2 test_create pending rows, got %d", len(changes))
	}
	for _, c := range changes {
		if c.EntityType != "test_create" || !strings.HasPrefix(c.EntityKey, "NEW-") {
			t.Errorf("change = %+v, want test_create with NEW-* key", c)
		}
	}
}

func TestImportTestsRequiresSummaryMapping(t *testing.T) {
	repo := newRepo(t)
	_, err := repo.ImportTests("p1", importCSV, testrepo.ImportMapping{}, true)
	if err == nil {
		t.Error("importing without a Summary mapping should error")
	}
}

func TestDiscardImportedTestRemovesIt(t *testing.T) {
	repo := newRepo(t)
	mapping := testrepo.ImportMapping{Summary: "Summary"}
	if _, err := repo.ImportTests("p1", "Summary\nOne test\n", mapping, false); err != nil {
		t.Fatalf("import: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	page, _ := repo.ListTests("p1", testrepo.Query{})
	if page.Total != 0 {
		t.Errorf("discarding the import should remove the test; got %d", page.Total)
	}
}
