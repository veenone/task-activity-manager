package testrepo_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

const lblProfile = "p1"

func seedLabelTests(t *testing.T, repo *testrepo.Repository) {
	t.Helper()
	tests := []testrepo.TestCase{
		{Key: "QA-1", Summary: "a", Labels: []string{"smoke", "login"}},
		{Key: "QA-2", Summary: "b", Labels: []string{"smoke"}},
		{Key: "QA-3", Summary: "c", Labels: []string{"Smoke"}},
		{Key: "QA-4", Summary: "d"},
	}
	if err := repo.UpsertTests(lblProfile, tests); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestListLabelsCountsCaseSensitive(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	got, err := repo.ListLabels(lblProfile)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []testrepo.Bucket{
		{Label: "Smoke", Count: 1},
		{Label: "login", Count: 1},
		{Label: "smoke", Count: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func labelsOf(t *testing.T, repo *testrepo.Repository, key string) []string {
	t.Helper()
	tc, err := repo.GetTest(lblProfile, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	return tc.Labels
}

func TestBulkEditLabelsAddsAndRemoves(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	res, err := repo.BulkEditLabels(lblProfile,
		[]string{"QA-1", "QA-2", "QA-4"},
		[]string{"regression", "login"},
		[]string{"smoke"})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if len(res.Failed) != 0 || len(res.Succeeded) != 3 {
		t.Fatalf("result %+v", res)
	}
	if got := labelsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"login", "regression"}) {
		t.Fatalf("QA-1 labels %v", got)
	}
	if got := labelsOf(t, repo, "QA-2"); !reflect.DeepEqual(got, []string{"regression", "login"}) {
		t.Fatalf("QA-2 labels %v", got)
	}
	if got := labelsOf(t, repo, "QA-4"); !reflect.DeepEqual(got, []string{"regression", "login"}) {
		t.Fatalf("QA-4 labels %v", got)
	}
}

func TestBulkEditLabelsSkipsUnchangedAndKeepsCase(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	// Removing "smoke" must not touch QA-3's "Smoke".
	res, err := repo.BulkEditLabels(lblProfile, []string{"QA-3"}, nil, []string{"smoke"})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if !reflect.DeepEqual(res.Succeeded, []string{"QA-3"}) {
		t.Fatalf("result %+v", res)
	}
	pending, err := repo.ListPendingChanges(lblProfile)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("unchanged test queued %d pending changes: %+v", len(pending), pending)
	}
}

func TestBulkEditLabelsReportsMissingKey(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	res, err := repo.BulkEditLabels(lblProfile, []string{"QA-1", "QA-99"}, []string{"x"}, nil)
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if !reflect.DeepEqual(res.Succeeded, []string{"QA-1"}) {
		t.Fatalf("succeeded %v", res.Succeeded)
	}
	if len(res.Failed) != 1 || res.Failed[0].TestKey != "QA-99" || res.Failed[0].Error != "not found" {
		t.Fatalf("failed %+v", res.Failed)
	}
}

func TestBulkEditLabelsRejectsBadInput(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	cases := []struct {
		name        string
		add, remove []string
	}{
		{"overlap", []string{"smoke"}, []string{"smoke"}},
		{"whitespace", []string{"two words"}, nil},
		{"empty", nil, nil},
	}
	for _, c := range cases {
		if _, err := repo.BulkEditLabels(lblProfile, []string{"QA-1"}, c.add, c.remove); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

func TestBulkEditTestsAddLabelAcceptsSeveral(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	_, err := repo.BulkEditTests(lblProfile, []string{"QA-2"},
		testrepo.BulkEdit{Operation: "add_label", Value: "a b smoke"})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if got := labelsOf(t, repo, "QA-2"); !reflect.DeepEqual(got, []string{"smoke", "a", "b"}) {
		t.Fatalf("labels %v", got)
	}
	_, err = repo.BulkEditTests(lblProfile, []string{"QA-2"},
		testrepo.BulkEdit{Operation: "remove_label", Value: "a smoke"})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if got := labelsOf(t, repo, "QA-2"); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("labels %v", got)
	}
}

func TestListTestLabelsReturnsRequestedKeys(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	got, err := repo.ListTestLabels(lblProfile, []string{"QA-1", "QA-4", "QA-99"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := map[string][]string{"QA-1": {"smoke", "login"}, "QA-4": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
