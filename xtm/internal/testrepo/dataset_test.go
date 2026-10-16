package testrepo

import (
	"reflect"
	"strings"
	"testing"
)

// A dataset survives the round trip with its parameters, its combinatorial
// flags and its rows intact.
func TestDatasetRoundTrip(t *testing.T) {
	r := newTestRepo(t)
	want := Dataset{
		Parameters: []DatasetParameter{
			{Name: "USERNAME"},
			{Name: "PRIORITY", Combinatorial: true},
		},
		Rows: [][]string{{"alice", "high"}, {"bob", "low"}},
	}
	if err := r.SaveTestDataset("p1", "QA-1", "", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok, err := r.GetTestDataset("p1", "QA-1", "")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatal("a saved dataset must be found")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dataset = %+v, want %+v", got, want)
	}
}

// Most tests have no dataset. Storing that fact is what stops the detail panel
// asking Jira again every time such a test is opened, so an empty dataset must
// come back as found rather than as absent.
func TestDatasetRecordsThatATestHasNone(t *testing.T) {
	r := newTestRepo(t)
	if err := r.SaveTestDataset("p1", "QA-2", "", Dataset{}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok, err := r.GetTestDataset("p1", "QA-2", "")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatal("an empty dataset must still be recorded as fetched")
	}
	if len(got.Parameters) != 0 || len(got.Rows) != 0 {
		t.Fatalf("dataset = %+v, want empty", got)
	}
}

// A test never looked at is absent, which is what makes the caller fetch.
func TestDatasetIsAbsentBeforeItIsFetched(t *testing.T) {
	r := newTestRepo(t)
	if _, ok, err := r.GetTestDataset("p1", "QA-3", ""); err != nil || ok {
		t.Fatalf("found = %v, err = %v, want not found and no error", ok, err)
	}
}

// Re-saving replaces the stored table rather than accumulating rows, so a
// Refresh after the dataset shrank in Jira does not leave stale rows behind.
func TestDatasetSaveReplaces(t *testing.T) {
	r := newTestRepo(t)
	first := Dataset{
		Parameters: []DatasetParameter{{Name: "A"}},
		Rows:       [][]string{{"1"}, {"2"}, {"3"}},
	}
	if err := r.SaveTestDataset("p1", "QA-1", "", first); err != nil {
		t.Fatalf("save: %v", err)
	}
	second := Dataset{
		Parameters: []DatasetParameter{{Name: "A"}},
		Rows:       [][]string{{"9"}},
	}
	if err := r.SaveTestDataset("p1", "QA-1", "", second); err != nil {
		t.Fatalf("resave: %v", err)
	}
	got, _, err := r.GetTestDataset("p1", "QA-1", "")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !reflect.DeepEqual(got.Rows, second.Rows) {
		t.Fatalf("rows = %v, want %v", got.Rows, second.Rows)
	}
}

// Datasets at different levels coexist: a container's dataset overrides the
// test's own in Xray, so storing one must not evict the other.
func TestDatasetKeepsTestAndContextLevelsApart(t *testing.T) {
	r := newTestRepo(t)
	testLevel := Dataset{Parameters: []DatasetParameter{{Name: "A"}}, Rows: [][]string{{"1"}}}
	planLevel := Dataset{Parameters: []DatasetParameter{{Name: "B"}}, Rows: [][]string{{"2"}}}
	if err := r.SaveTestDataset("p1", "QA-1", "", testLevel); err != nil {
		t.Fatalf("save test level: %v", err)
	}
	if err := r.SaveTestDataset("p1", "QA-1", "QA-99", planLevel); err != nil {
		t.Fatalf("save plan level: %v", err)
	}
	got, _, err := r.GetTestDataset("p1", "QA-1", "")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !reflect.DeepEqual(got, testLevel) {
		t.Fatalf("test level = %+v, want %+v", got, testLevel)
	}
}

// Deleting a profile must take its datasets with it. The table is
// profile-keyed, so it belongs in the purge list.
func TestPurgeProfileSweepsDatasets(t *testing.T) {
	r := newTestRepo(t)
	ds := Dataset{Parameters: []DatasetParameter{{Name: "A"}}, Rows: [][]string{{"1"}}}
	if err := r.SaveTestDataset("p1", "QA-1", "", ds); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := r.PurgeProfile("p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, ok, err := r.GetTestDataset("p1", "QA-1", ""); err != nil || ok {
		t.Fatalf("found = %v after purge, want the row gone", ok)
	}
}

// The purge list is the contract the store schema names. Checking membership
// directly keeps a future table from being added to the schema and forgotten
// here, which is the defect the TAM gate exists to catch and XTM has no gate
// for yet.
func TestDatasetTableIsInThePurgeList(t *testing.T) {
	if !strings.Contains(strings.Join(profileScopedTables, ","), "test_dataset") {
		t.Fatalf("test_dataset missing from profileScopedTables: %v", profileScopedTables)
	}
}
