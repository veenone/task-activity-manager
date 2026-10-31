package testrepo_test

import (
	"path/filepath"
	"strings"
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

// TestCustomFieldScopeRoundTrips covers the column added for #371 Phase 3. A
// Zephyr profile has custom fields on an execution and on a step as well as on
// the test issue, and only the issue ones can be edited here, so the scope has
// to survive the store rather than being re-derived from the field id.
func TestCustomFieldScopeRoundTrips(t *testing.T) {
	repo := seedCustomFields(t)
	if err := repo.UpsertCustomFields("p1", []testrepo.CustomFieldDef{
		{FieldID: "1", Name: "Build number", Type: "TEXT", Scope: "execution"},
		{FieldID: "2", Name: "Evidence", Type: "LARGE_TEXT", Scope: "step"},
	}); err != nil {
		t.Fatalf("upsert scoped defs: %v", err)
	}

	defs, err := repo.ListCustomFieldDefs("p1")
	if err != nil {
		t.Fatalf("list defs: %v", err)
	}
	want := map[string]string{
		// The two seeded by seedCustomFields pass no scope at all, which is
		// every existing caller: they have to read back as issue fields or an
		// Xray profile's fields would stop being editable.
		"customfield_10100": "issue",
		"customfield_10102": "issue",
		"1":                 "execution",
		"2":                 "step",
	}
	for _, d := range defs {
		if w, ok := want[d.FieldID]; ok && d.Scope != w {
			t.Errorf("field %s scope = %q, want %q", d.FieldID, d.Scope, w)
		}
		delete(want, d.FieldID)
	}
	if len(want) > 0 {
		t.Errorf("definitions missing from the list: %v", want)
	}

	fields, err := repo.ListTestCustomFields("p1", "QA-1")
	if err != nil {
		t.Fatalf("list test fields: %v", err)
	}
	for _, f := range fields {
		if f.FieldID == "1" && f.Scope != "execution" {
			t.Errorf("the detail panel reads scope %q for an execution field, want execution", f.Scope)
		}
	}
}

// TestEditTestCustomFieldRefusesAFieldThatIsNotOnTheIssue is the guard the
// scope exists for. A custom field edit commits as a Jira issue field update,
// so staging one against a Zephyr execution or step field would queue a change
// that can only fail at commit, against a field Jira has never heard of.
func TestEditTestCustomFieldRefusesAFieldThatIsNotOnTheIssue(t *testing.T) {
	repo := seedCustomFields(t)
	if err := repo.UpsertCustomFields("p1", []testrepo.CustomFieldDef{
		{FieldID: "1", Name: "Build number", Type: "TEXT", Scope: "execution"},
	}); err != nil {
		t.Fatalf("upsert scoped def: %v", err)
	}

	err := repo.EditTestCustomField("p1", "QA-1", "1", "42")
	if err == nil {
		t.Fatal("editing an execution-scoped field was accepted")
	}
	if !strings.Contains(err.Error(), "Build number") {
		t.Errorf("error %q does not name the field the user clicked", err)
	}
	if changes, _ := repo.ListPendingChanges("p1"); len(changes) != 0 {
		t.Errorf("a pending change was queued anyway: %+v", changes)
	}
}
