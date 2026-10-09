package testrepo_test

import (
	"reflect"
	"strings"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func TestSetTestComponentsQueuesOneEdit(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", Summary: "a", Components: []string{"API"}}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetTestComponents("p1", "QA-1", []string{"User Management", "API", "API"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	tc, _ := repo.GetTest("p1", "QA-1")
	if !reflect.DeepEqual(tc.Components, []string{"User Management", "API"}) {
		t.Fatalf("components %v", tc.Components)
	}
	pending, _ := repo.ListPendingChanges("p1")
	if len(pending) != 1 || pending[0].Field != "components" ||
		pending[0].BeforeVal != "\nAPI\n" || pending[0].AfterVal != "\nUser Management\nAPI\n" {
		t.Fatalf("pending %+v", pending)
	}
	// Same set again: nothing new queued.
	if err := repo.SetTestComponents("p1", "QA-1", []string{"User Management", "API"}); err != nil {
		t.Fatal(err)
	}
	if again, _ := repo.ListPendingChanges("p1"); len(again) != 1 {
		t.Fatalf("unchanged set queued another edit: %+v", again)
	}
}

func TestSetTestComponentsRejectsBadNames(t *testing.T) {
	repo := newRepo(t)
	_ = repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", Summary: "a"}})
	for _, bad := range []string{"  ", "two\nlines", strings.Repeat("x", 256)} {
		if err := repo.SetTestComponents("p1", "QA-1", []string{bad}); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func seedBulkComponents(t *testing.T, repo *testrepo.Repository) {
	t.Helper()
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", Summary: "a", Components: []string{"API", "Core"}},
		{Key: "QA-2", Summary: "b", Components: []string{"Core"}},
		{Key: "QA-3", Summary: "c"},
	}); err != nil {
		t.Fatal(err)
	}
}

func compsOf(t *testing.T, repo *testrepo.Repository, key string) []string {
	t.Helper()
	tc, err := repo.GetTest("p1", key)
	if err != nil {
		t.Fatal(err)
	}
	return tc.Components
}

func TestBulkEditComponentsAddRemove(t *testing.T) {
	repo := newRepo(t)
	seedBulkComponents(t, repo)
	res, err := repo.BulkEditComponents("p1", []string{"QA-1", "QA-2", "QA-3", "QA-9"},
		[]string{"User Management"}, []string{"Core"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Succeeded, []string{"QA-1", "QA-2", "QA-3"}) ||
		len(res.Failed) != 1 || res.Failed[0].TestKey != "QA-9" || res.Failed[0].Error != "not found" {
		t.Fatalf("result %+v", res)
	}
	if got := compsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"API", "User Management"}) {
		t.Fatalf("QA-1 %v", got)
	}
	if got := compsOf(t, repo, "QA-3"); !reflect.DeepEqual(got, []string{"User Management"}) {
		t.Fatalf("QA-3 %v", got)
	}
}

func TestBulkEditComponentsReplaceAndClear(t *testing.T) {
	repo := newRepo(t)
	seedBulkComponents(t, repo)
	if _, err := repo.BulkEditComponents("p1", []string{"QA-1", "QA-3"}, []string{"Core"}, nil, true); err != nil {
		t.Fatal(err)
	}
	if got := compsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"Core"}) {
		t.Fatalf("QA-1 %v", got)
	}
	if _, err := repo.BulkEditComponents("p1", []string{"QA-1", "QA-2"}, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	if got := compsOf(t, repo, "QA-2"); len(got) != 0 {
		t.Fatalf("QA-2 should be cleared, got %v", got)
	}
	pending, _ := repo.ListPendingChanges("p1")
	if len(pending) != 3 { // QA-1 (one row, updated twice), QA-2, QA-3
		t.Fatalf("pending %d: %+v", len(pending), pending)
	}
}

func TestBulkEditComponentsRejects(t *testing.T) {
	repo := newRepo(t)
	seedBulkComponents(t, repo)
	cases := []struct {
		name        string
		add, remove []string
		replace     bool
	}{
		{"nothing", nil, nil, false},
		{"overlap", []string{"Core"}, []string{"Core"}, false},
		{"replace with remove", []string{"A"}, []string{"Core"}, true},
		{"bad name", []string{"a\nb"}, nil, false},
	}
	for _, c := range cases {
		if _, err := repo.BulkEditComponents("p1", []string{"QA-1"}, c.add, c.remove, c.replace); err == nil {
			t.Errorf("%s: want error", c.name)
		}
	}
}

func TestListTestComponents(t *testing.T) {
	repo := newRepo(t)
	seedBulkComponents(t, repo)
	got, err := repo.ListTestComponents("p1", []string{"QA-2", "QA-3", "QA-9"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"QA-2": {"Core"}, "QA-3": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}
