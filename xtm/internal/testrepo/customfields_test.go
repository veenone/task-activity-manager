package testrepo_test

import (
	"path/filepath"
	"testing"

	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/testrepo"
)

func newCFRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st)
}

func seedCustomFields(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newCFRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X"},
	}); err != nil {
		t.Fatalf("seed test: %v", err)
	}
	if err := repo.UpsertCustomFields("p1", []testrepo.CustomFieldDef{
		{FieldID: "customfield_10100", Name: "Test Type", Type: "option"},
		{FieldID: "customfield_10102", Name: "Component", Type: "string"},
	}); err != nil {
		t.Fatalf("seed defs: %v", err)
	}
	return repo
}

func TestListTestCustomFieldsIncludesAllDefs(t *testing.T) {
	repo := seedCustomFields(t)
	if err := repo.SetTestCustomFields("p1", "QA-1", map[string]string{
		"customfield_10100": "Manual",
	}); err != nil {
		t.Fatalf("set values: %v", err)
	}

	fields, err := repo.ListTestCustomFields("p1", "QA-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("got %d fields, want 2 (both defs, even the value-less one)", len(fields))
	}
}

func TestEditTestCustomFieldQueuesChange(t *testing.T) {
	repo := seedCustomFields(t)

	if err := repo.EditTestCustomField("p1", "QA-1", "customfield_10102", "Backend"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	fields, _ := repo.ListTestCustomFields("p1", "QA-1")
	var got string
	for _, f := range fields {
		if f.FieldID == "customfield_10102" {
			got = f.Value
		}
	}
	if got != "Backend" {
		t.Errorf("value = %q, want Backend", got)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "custom_field" || changes[0].EntityKey != "QA-1:customfield_10102" {
		t.Fatalf("pending = %+v, want one custom_field for QA-1:customfield_10102", changes)
	}
}

func TestDiscardCustomFieldEditRevertsValue(t *testing.T) {
	repo := seedCustomFields(t)
	if err := repo.SetTestCustomFields("p1", "QA-1", map[string]string{
		"customfield_10102": "Frontend",
	}); err != nil {
		t.Fatalf("set values: %v", err)
	}
	if err := repo.EditTestCustomField("p1", "QA-1", "customfield_10102", "Backend"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	fields, _ := repo.ListTestCustomFields("p1", "QA-1")
	for _, f := range fields {
		if f.FieldID == "customfield_10102" && f.Value != "Frontend" {
			t.Errorf("value = %q after discard, want Frontend (reverted)", f.Value)
		}
	}
}

func TestEditUnknownCustomFieldErrors(t *testing.T) {
	repo := seedCustomFields(t)
	if err := repo.EditTestCustomField("p1", "QA-1", "customfield_99999", "x"); err == nil {
		t.Error("editing an unknown custom field should error")
	}
}
