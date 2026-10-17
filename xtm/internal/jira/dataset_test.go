package jira

import (
	"reflect"
	"testing"
)

// The CSV parser, exercised directly.
//
// It used to be reached through GetDataset, because a test's dataset was read
// from the 2.0 export. That read goes to the JSON model now (#283) and the
// export is gone, but the parser stays: it reads the files a user brings back
// from a spreadsheet (#277 W2). These are the cases about the format itself;
// dataset_fixture_test.go reads what a live instance actually sent.

// The header row names the parameters and each later row is one iteration.
func TestParseDatasetCSVReadsParametersAndRows(t *testing.T) {
	ds := ParseDatasetCSV([]byte("USERNAME,PASSWORD\nalice,hunter2\nbob,s3cret\n"))

	wantNames := []string{"USERNAME", "PASSWORD"}
	if got := parameterNames(ds.Parameters); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("parameters = %v, want %v", got, wantNames)
	}
	wantRows := [][]string{{"alice", "hunter2"}, {"bob", "s3cret"}}
	if !reflect.DeepEqual(ds.Rows, wantRows) {
		t.Fatalf("rows = %v, want %v", ds.Rows, wantRows)
	}
}

// Xray marks a combinatorial parameter with an asterisk suffix on the column
// name. The flag is ours to keep; the asterisk is not part of the name.
func TestParseDatasetCSVStripsTheCombinatorialMarker(t *testing.T) {
	ds := ParseDatasetCSV([]byte("BROWSER,QUALITY*\nchrome,high\n"))

	if len(ds.Parameters) != 2 {
		t.Fatalf("parameters = %v, want 2", ds.Parameters)
	}
	if ds.Parameters[0].Combinatorial {
		t.Errorf("BROWSER must not be combinatorial")
	}
	if got := ds.Parameters[1]; got.Name != "QUALITY" || !got.Combinatorial {
		t.Errorf("parameter 1 = %+v, want name QUALITY and combinatorial", got)
	}
}

// A quoted cell may hold the delimiter. encoding/csv handles this; the test
// pins it so a hand-rolled splitter can never replace the parser.
func TestParseDatasetCSVKeepsACommaInsideAQuotedCell(t *testing.T) {
	ds := ParseDatasetCSV([]byte("CITY,NOTE\n\"Paris, France\",capital\n"))

	if len(ds.Rows) != 1 || ds.Rows[0][0] != "Paris, France" {
		t.Fatalf("rows = %q, want the comma kept inside the cell", ds.Rows)
	}
}

// A dataset with no rows is a header and nothing else.
func TestParseDatasetCSVHandlesAHeaderWithNoRows(t *testing.T) {
	ds := ParseDatasetCSV([]byte("USERNAME\n"))

	if len(ds.Parameters) != 1 || len(ds.Rows) != 0 {
		t.Fatalf("dataset = %+v, want one parameter and no rows", ds)
	}
}

// An empty body is not a parse failure. Nothing to show is a valid answer,
// and an import refuses it in its own words rather than crashing here.
func TestParseDatasetCSVHandlesAnEmptyBody(t *testing.T) {
	if ds := ParseDatasetCSV(nil); len(ds.Parameters) != 0 {
		t.Fatalf("parameters = %v, want none", ds.Parameters)
	}
}

// A row with fewer cells than the header is padded rather than dropped, so a
// short row still lines up under its parameters.
func TestParseDatasetCSVPadsAShortRow(t *testing.T) {
	ds := ParseDatasetCSV([]byte("A,B,C\n1,2\n"))

	want := [][]string{{"1", "2", ""}}
	if !reflect.DeepEqual(ds.Rows, want) {
		t.Fatalf("rows = %v, want %v", ds.Rows, want)
	}
}

// parameterNames is a test helper: the parameter names in order.
func parameterNames(ps []DatasetParameter) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}
