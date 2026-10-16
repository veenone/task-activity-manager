package jira

import (
	"reflect"
	"strings"
	"testing"
)

// A table written out and read back is the same table. The fixture is a real
// export from the #277 W0 spike, so the format this writes is the format Xray
// itself produces and a spreadsheet round trip lands back where it started.
func TestDatasetCSVRoundTripsALiveExport(t *testing.T) {
	original := ParseDatasetCSV(readFixture(t, "dataset_export_list_combinatorial.csv"))
	if len(original.Parameters) != 3 {
		t.Fatalf("fixture parsed as %d parameters, want 3", len(original.Parameters))
	}

	again := ParseDatasetCSV(DatasetCSV(original))
	if !reflect.DeepEqual(again, original) {
		t.Errorf("round trip changed the table:\n got %+v\nwant %+v", again, original)
	}
}

// Xray marks a combinatorial parameter with an asterisk on the column name,
// and writing one without it would quietly drop the flag on the way back in.
func TestDatasetCSVMarksACombinatorialColumn(t *testing.T) {
	out := string(DatasetCSV(Dataset{
		Parameters: []DatasetParameter{
			{Name: "USERNAME"},
			{Name: "REGION", Combinatorial: true},
		},
		Rows: [][]string{{"alice", "eu-west"}},
	}))

	header := strings.SplitN(out, "\n", 2)[0]
	if strings.TrimSpace(header) != "USERNAME,REGION*" {
		t.Errorf("header = %q, want the asterisk on the combinatorial column", header)
	}
	if !strings.Contains(out, "alice,eu-west") {
		t.Errorf("output %q is missing the row", out)
	}
}

// A value with a comma, a quote or a newline in it has to survive, which is
// the entire reason this goes through encoding/csv rather than strings.Join.
func TestDatasetCSVQuotesAwkwardValues(t *testing.T) {
	awkward := Dataset{
		Parameters: []DatasetParameter{{Name: "NOTE"}},
		Rows:       [][]string{{`a, b`}, {`say "hi"`}, {"two\nlines"}},
	}

	back := ParseDatasetCSV(DatasetCSV(awkward))
	if !reflect.DeepEqual(back.Rows, awkward.Rows) {
		t.Errorf("rows = %q, want %q", back.Rows, awkward.Rows)
	}
}

// An empty table writes nothing rather than a header of no columns, so an
// export of a test with no dataset is an empty file instead of a stray line.
func TestDatasetCSVOfAnEmptyTableIsEmpty(t *testing.T) {
	if out := DatasetCSV(Dataset{}); len(out) != 0 {
		t.Errorf("empty dataset wrote %q, want nothing", out)
	}
}
