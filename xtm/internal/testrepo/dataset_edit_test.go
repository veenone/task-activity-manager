package testrepo

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func datasetOf(t *testing.T, encoded string) Dataset {
	t.Helper()
	var ds Dataset
	if err := json.Unmarshal([]byte(encoded), &ds); err != nil {
		t.Fatalf("pending value is not a dataset: %v\n%s", err, encoded)
	}
	return ds
}

func pendingDatasetRows(t *testing.T, r *Repository, profileID string) []PendingChange {
	t.Helper()
	all, err := r.ListPendingChanges(profileID)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	out := []PendingChange{}
	for _, c := range all {
		if c.EntityType == "test_dataset" {
			out = append(out, c)
		}
	}
	return out
}

var twoByTwo = Dataset{
	Parameters: []DatasetParameter{{Name: "USERNAME"}, {Name: "REGION"}},
	Rows:       [][]string{{"alice", "eu-west"}, {"bob", "us-east"}},
}

// A local dataset edit shows immediately and commits later, like every other
// edit in XTM: the cache carries the new table and the journal carries the
// whole table, before and after (#277 W1).
func TestEditTestDatasetCachesTheTableAndQueuesIt(t *testing.T) {
	r := newTestRepo(t)
	if err := r.EditTestDataset("p1", "QA-1", twoByTwo); err != nil {
		t.Fatalf("edit: %v", err)
	}

	cached, found, err := r.GetTestDataset("p1", "QA-1", "")
	if err != nil || !found {
		t.Fatalf("cache: found=%v err=%v", found, err)
	}
	if !reflect.DeepEqual(cached, twoByTwo) {
		t.Errorf("cached dataset = %+v, want %+v", cached, twoByTwo)
	}

	rows := pendingDatasetRows(t, r, "p1")
	if len(rows) != 1 {
		t.Fatalf("%d pending dataset rows, want 1", len(rows))
	}
	if rows[0].EntityKey != "QA-1" || rows[0].Field != "dataset" {
		t.Errorf("pending row = %s/%s, want QA-1/dataset", rows[0].EntityKey, rows[0].Field)
	}
	// The whole table travels in the row, so the commit needs no second read
	// and the user can see what will be sent.
	if got := datasetOf(t, rows[0].AfterVal); !reflect.DeepEqual(got, twoByTwo) {
		t.Errorf("queued dataset = %+v, want %+v", got, twoByTwo)
	}
	// A test that had none starts from an empty table, not from nothing, so
	// the commit's conflict check has something to compare against.
	if got := datasetOf(t, rows[0].BeforeVal); len(got.Parameters) != 0 || len(got.Rows) != 0 {
		t.Errorf("before value = %+v, want an empty dataset", got)
	}
}

// Editing the same dataset twice leaves one pending row, still holding the
// table as it was before the first edit. Two rows would commit twice and the
// second would overwrite with a stale before value.
func TestEditTestDatasetCoalescesRepeatedEdits(t *testing.T) {
	r := newTestRepo(t)
	if err := r.SaveTestDataset("p1", "QA-1", "", twoByTwo); err != nil {
		t.Fatalf("seed: %v", err)
	}

	withRow := twoByTwo
	withRow.Rows = append([][]string{}, append(twoByTwo.Rows, []string{"carol", "ap-south"})...)
	if err := r.EditTestDataset("p1", "QA-1", withRow); err != nil {
		t.Fatalf("first edit: %v", err)
	}
	renamed := Dataset{
		Parameters: []DatasetParameter{{Name: "USERNAME"}, {Name: "ZONE"}},
		Rows:       withRow.Rows,
	}
	if err := r.EditTestDataset("p1", "QA-1", renamed); err != nil {
		t.Fatalf("second edit: %v", err)
	}

	rows := pendingDatasetRows(t, r, "p1")
	if len(rows) != 1 {
		t.Fatalf("%d pending dataset rows after two edits, want 1", len(rows))
	}
	if got := datasetOf(t, rows[0].AfterVal); !reflect.DeepEqual(got, renamed) {
		t.Errorf("queued dataset = %+v, want the second edit %+v", got, renamed)
	}
	if got := datasetOf(t, rows[0].BeforeVal); !reflect.DeepEqual(got, twoByTwo) {
		t.Errorf("before value = %+v, want the table as synced %+v", got, twoByTwo)
	}
}

// Editing back to what Jira has is not a change. The row goes, so Commit has
// nothing to push and the Pending changes dialog does not list a no-op.
func TestEditTestDatasetBackToTheSyncedTableDropsTheRow(t *testing.T) {
	r := newTestRepo(t)
	if err := r.SaveTestDataset("p1", "QA-1", "", twoByTwo); err != nil {
		t.Fatalf("seed: %v", err)
	}

	trimmed := Dataset{Parameters: twoByTwo.Parameters, Rows: twoByTwo.Rows[:1]}
	if err := r.EditTestDataset("p1", "QA-1", trimmed); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if err := r.EditTestDataset("p1", "QA-1", twoByTwo); err != nil {
		t.Fatalf("revert: %v", err)
	}

	if rows := pendingDatasetRows(t, r, "p1"); len(rows) != 0 {
		t.Fatalf("%d pending dataset rows after reverting, want none", len(rows))
	}
	cached, _, err := r.GetTestDataset("p1", "QA-1", "")
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	if !reflect.DeepEqual(cached, twoByTwo) {
		t.Errorf("cached dataset = %+v, want the synced table back", cached)
	}
}

// Xray enforces its own limits, and finding out at commit time means the user
// loses the edit they already made. Nothing invalid is journalled.
func TestEditTestDatasetRefusesATableXrayWouldReject(t *testing.T) {
	wide := make([]DatasetParameter, 21)
	for i := range wide {
		wide[i] = DatasetParameter{Name: "P" + strconv.Itoa(i)}
	}

	for _, tc := range []struct {
		name string
		ds   Dataset
		want string
	}{
		{"too many parameters", Dataset{Parameters: wide}, "20 parameters"},
		{
			"value too long",
			Dataset{
				Parameters: []DatasetParameter{{Name: "USERNAME"}},
				Rows:       [][]string{{strings.Repeat("a", 101)}},
			},
			"100 characters",
		},
		{
			"duplicate parameter name",
			Dataset{Parameters: []DatasetParameter{{Name: "USERNAME"}, {Name: "USERNAME"}}},
			"more than once",
		},
		{
			"name starting with a digit",
			Dataset{Parameters: []DatasetParameter{{Name: "1ST_USER"}}},
			"1ST_USER",
		},
		{"empty name", Dataset{Parameters: []DatasetParameter{{Name: ""}}}, "needs a name"},
		{
			"duplicate rows",
			Dataset{
				Parameters: []DatasetParameter{{Name: "USERNAME"}},
				Rows:       [][]string{{"alice"}, {"alice"}},
			},
			"rows 1 and 2",
		},
		{
			"row wider than the parameters",
			Dataset{
				Parameters: []DatasetParameter{{Name: "USERNAME"}},
				Rows:       [][]string{{"alice", "eu-west"}},
			},
			"more cells",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			err := r.EditTestDataset("p1", "QA-1", tc.ds)
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
			if rows := pendingDatasetRows(t, r, "p1"); len(rows) != 0 {
				t.Errorf("a rejected table was journalled anyway: %+v", rows)
			}
			if _, found, _ := r.GetTestDataset("p1", "QA-1", ""); found {
				t.Error("a rejected table reached the cache")
			}
		})
	}
}

// The limits are limits, not margins: a table sitting exactly on them is fine.
func TestEditTestDatasetAcceptsTheLimits(t *testing.T) {
	r := newTestRepo(t)
	params := make([]DatasetParameter, 20)
	row := make([]string, 20)
	for i := range params {
		params[i] = DatasetParameter{Name: "P" + strconv.Itoa(i)}
		row[i] = strings.Repeat("a", 100)
	}
	// A name with the punctuation Xray allows, at its full length.
	params[0] = DatasetParameter{Name: "_" + strings.Repeat("a", 63)}
	params[1] = DatasetParameter{Name: "user name - 2"}

	if err := r.EditTestDataset("p1", "QA-1", Dataset{
		Parameters: params,
		Rows:       [][]string{row},
	}); err != nil {
		t.Fatalf("a table on the limits was refused: %v", err)
	}
}

// Discarding the pending change puts the synced table back. Dropping the row
// alone would leave the editor showing an edit that will never be sent.
func TestDiscardingADatasetEditRestoresTheSyncedTable(t *testing.T) {
	r := newTestRepo(t)
	if err := r.SaveTestDataset("p1", "QA-1", "", twoByTwo); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := r.EditTestDataset("p1", "QA-1", Dataset{
		Parameters: []DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}},
	}); err != nil {
		t.Fatalf("edit: %v", err)
	}

	rows := pendingDatasetRows(t, r, "p1")
	if len(rows) != 1 {
		t.Fatalf("%d pending dataset rows, want 1", len(rows))
	}
	if err := r.DiscardPendingChange("p1", rows[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	cached, found, err := r.GetTestDataset("p1", "QA-1", "")
	if err != nil || !found {
		t.Fatalf("cache: found=%v err=%v", found, err)
	}
	if !reflect.DeepEqual(cached, twoByTwo) {
		t.Errorf("cached dataset = %+v, want the synced table %+v", cached, twoByTwo)
	}
	if left := pendingDatasetRows(t, r, "p1"); len(left) != 0 {
		t.Errorf("%d pending dataset rows survive the discard, want none", len(left))
	}
}

// A dataset given to a test that has not been created in Jira yet is keyed by
// its placeholder key. The commit renames the test first, and the dataset has
// to travel with it or the write lands on a key Jira has never heard of.
func TestRenameTestCarriesTheDatasetAndItsPendingRow(t *testing.T) {
	r := newTestRepo(t)
	if err := r.EditTestDataset("p1", "new-test-1", twoByTwo); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if err := r.RenameTest("p1", "new-test-1", "QA-9"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	cached, found, err := r.GetTestDataset("p1", "QA-9", "")
	if err != nil || !found {
		t.Fatalf("dataset did not follow the rename: found=%v err=%v", found, err)
	}
	if !reflect.DeepEqual(cached, twoByTwo) {
		t.Errorf("cached dataset = %+v, want %+v", cached, twoByTwo)
	}
	rows := pendingDatasetRows(t, r, "p1")
	if len(rows) != 1 {
		t.Fatalf("%d pending dataset rows, want 1", len(rows))
	}
	if rows[0].EntityKey != "QA-9" {
		t.Errorf("pending row still keyed %s, want QA-9", rows[0].EntityKey)
	}
}
