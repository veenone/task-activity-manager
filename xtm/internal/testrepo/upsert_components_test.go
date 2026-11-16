package testrepo_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// A sync must not undo a components edit that has not been committed. Every
// other editable column had that guard in UpsertTests; components did not
// (#486).
func TestUpsertTestsPreservesPendingComponentsOnResync(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Components: []string{"API"}},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetTestComponents("p1", "QA-1", []string{"Billing"}); err != nil {
		t.Fatalf("set components: %v", err)
	}

	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Components: []string{"API", "Frontend"}},
	}); err != nil {
		t.Fatalf("resync: %v", err)
	}

	got, err := repo.GetTest("p1", "QA-1")
	if err != nil {
		t.Fatalf("read test: %v", err)
	}
	if !reflect.DeepEqual(got.Components, []string{"Billing"}) {
		t.Errorf("components after resync = %v, want the pending [Billing]", got.Components)
	}
}

func TestUpsertTestsOverwritesComponentsWhenNoPending(t *testing.T) {
	repo := newRepo(t)
	for _, comps := range [][]string{{"API"}, {"API", "Frontend"}} {
		if err := repo.UpsertTests("p1", []testrepo.TestCase{
			{Key: "QA-1", ID: "1", Summary: "X", Components: comps},
		}); err != nil {
			t.Fatalf("sync %v: %v", comps, err)
		}
	}

	got, err := repo.GetTest("p1", "QA-1")
	if err != nil {
		t.Fatalf("read test: %v", err)
	}
	if !reflect.DeepEqual(got.Components, []string{"API", "Frontend"}) {
		t.Errorf("components after resync = %v, want Jira's [API Frontend]", got.Components)
	}
}
