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
