package jira

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The fixtures in testdata are raw responses captured from a live Xray
// Server/DC instance during the #277 W0 spike. They exist so a change to the
// parser is measured against bytes Xray really sent, rather than against a
// reading of the documentation, which was wrong about the media type and about
// list parameters.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestParseDatasetCSVReadsALiveTextExport(t *testing.T) {
	d := parseDatasetCSV(readFixture(t, "dataset_export_text.csv"))

	if len(d.Parameters) != 1 || d.Parameters[0].Name != "dataset sample - 1" {
		t.Fatalf("parameters = %+v, want one named %q", d.Parameters, "dataset sample - 1")
	}
	if d.Parameters[0].Combinatorial {
		t.Error("parameter marked combinatorial; the export carries no asterisk")
	}
	want := [][]string{{"text combi - 1"}, {"text combi -2"}}
	if len(d.Rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(d.Rows), len(want))
	}
	for i, row := range want {
		if d.Rows[i][0] != row[0] {
			t.Errorf("row %d = %q, want %q", i, d.Rows[i][0], row[0])
		}
	}
}

func TestParseDatasetCSVMarksTheCombinatorialColumnAndPadsAShortRow(t *testing.T) {
	d := parseDatasetCSV(readFixture(t, "dataset_export_list_combinatorial.csv"))

	names := make([]string, len(d.Parameters))
	combi := make([]bool, len(d.Parameters))
	for i, p := range d.Parameters {
		names[i], combi[i] = p.Name, p.Combinatorial
	}
	wantNames := []string{
		"dataset sample - 1",
		"dataset sample - 2 - list",
		"dataset sample - 2 - list combi",
	}
	if strings.Join(names, "|") != strings.Join(wantNames, "|") {
		t.Fatalf("parameters = %q, want %q", names, wantNames)
	}
	// Only the asterisk-suffixed column is combinatorial, and the asterisk is
	// stripped from the name it marks.
	if combi[0] || combi[1] || !combi[2] {
		t.Errorf("combinatorial flags = %v, want [false false true]", combi)
	}

	// The last row of this export is ",,list 3 option 3": two empty leading
	// cells. Every row must still line up under its parameters.
	if len(d.Rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(d.Rows))
	}
	last := d.Rows[2]
	if len(last) != len(d.Parameters) {
		t.Fatalf("last row has %d cells, want %d", len(last), len(d.Parameters))
	}
	if last[0] != "" || last[1] != "" || last[2] != "list 3 option 3" {
		t.Errorf("last row = %q, want two empty cells then %q", last, "list 3 option 3")
	}
}

// TestDatasetAbsentIsDistinguishableFromEndpointAbsent pins the discriminator
// the write path needs. Both answers are 404, so the status alone cannot say
// whether the test has no dataset or the instance has no dataset API.
func TestDatasetAbsentIsDistinguishableFromEndpointAbsent(t *testing.T) {
	noDataset := readFixture(t, "dataset_export_absent.json")
	noEndpoint := readFixture(t, "dataset_endpoint_absent.xml")

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(noDataset, &body); err != nil {
		t.Fatalf("a missing dataset no longer answers with JSON: %v", err)
	}
	if body.Error != "Dataset not found" {
		t.Errorf("error = %q, want %q", body.Error, "Dataset not found")
	}

	if json.Valid(noEndpoint) {
		t.Error("a missing endpoint answers with JSON too; the two are no longer distinguishable")
	}
	if !strings.Contains(string(noEndpoint), "<status-code>404</status-code>") {
		t.Errorf("missing-endpoint body is not Jersey's XML status: %q", noEndpoint)
	}
}

// TestDatasetJSONModelNamesItsLevel pins what the raven/1.0 API gives a reader
// that the CSV export cannot: which level the dataset comes from, whether this
// level overrides, and a list parameter's allowed values.
func TestDatasetJSONModelNamesItsLevel(t *testing.T) {
	type paramInfo struct {
		Type    string   `json:"type"`
		Options []string `json:"options"`
	}
	type param struct {
		Name          string    `json:"name"`
		Rank          int       `json:"rank"`
		Combinatorial bool      `json:"combinatorial"`
		Info          paramInfo `json:"info"`
	}
	type column struct {
		Param  param `json:"param"`
		Values []struct {
			Value string `json:"value"`
			Rank  int    `json:"rank"`
		} `json:"values"`
	}
	type model struct {
		EntityDataset     struct{ Params []column } `json:"entityDataset"`
		ResolvedDataset   struct{ Params []column } `json:"resolvedDataset"`
		ResolutionMessage string                    `json:"resolutionMessage"`
	}

	var atTest model
	if err := json.Unmarshal(readFixture(t, "dataset_model_test_level.json"), &atTest); err != nil {
		t.Fatal(err)
	}
	if len(atTest.EntityDataset.Params) != 3 {
		t.Fatalf("test level has %d columns, want 3", len(atTest.EntityDataset.Params))
	}
	// The CSV export of this same dataset loses the allowed values. The JSON
	// keeps them, which is why a list parameter is modellable here and not there.
	list := atTest.EntityDataset.Params[1].Param
	if list.Info.Type != "LIST" {
		t.Errorf("second parameter type = %q, want LIST", list.Info.Type)
	}
	if len(list.Info.Options) != 3 {
		t.Errorf("list parameter carries %d allowed values, want 3", len(list.Info.Options))
	}
	if !strings.Contains(atTest.ResolutionMessage, "defined in the Test") {
		t.Errorf("resolution message = %q, want it to name the Test level", atTest.ResolutionMessage)
	}

	// A run that defines nothing of its own still reports what will run, and
	// still names where it came from. That is the pair a provenance display
	// needs: an empty entityDataset means "no override", not "not fetched".
	var atRun model
	if err := json.Unmarshal(readFixture(t, "dataset_model_run_inherited.json"), &atRun); err != nil {
		t.Fatal(err)
	}
	if len(atRun.EntityDataset.Params) != 0 {
		t.Errorf("run defines %d columns, want 0", len(atRun.EntityDataset.Params))
	}
	if len(atRun.ResolvedDataset.Params) == 0 {
		t.Error("run resolves to no dataset; it should inherit the Test's")
	}
	if !strings.Contains(atRun.ResolutionMessage, "defined in the Test") {
		t.Errorf("resolution message = %q, want it to name the inherited level", atRun.ResolutionMessage)
	}
}

// TestImportContractNamesNoOverwriteFlag records why the write path cannot use
// the 2.0 import alone: the resource takes no flag saying what to do with an
// existing dataset. The flags live on raven/1.0's parseDataset instead.
func TestImportContractNamesNoOverwriteFlag(t *testing.T) {
	wadl := string(readFixture(t, "dataset_import_v2.wadl.xml"))
	if !strings.Contains(wadl, `name="POST"`) {
		t.Fatal("the 2.0 import resource no longer accepts POST")
	}
	for _, flag := range []string{"overwriteValues", "createNewParameters"} {
		if strings.Contains(wadl, flag) {
			t.Errorf("the 2.0 import now takes %s; the write path can use it directly", flag)
		}
	}

	v1 := string(readFixture(t, "dataset_v1.wadl.xml"))
	for _, want := range []string{
		"parseDataset", "overwriteValues", "createNewParameters", "hasDataset",
	} {
		if !strings.Contains(v1, want) {
			t.Errorf("raven/1.0 no longer publishes %s", want)
		}
	}
}
