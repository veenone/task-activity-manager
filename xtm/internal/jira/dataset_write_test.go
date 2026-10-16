package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// datasetWriteServer records the write it was asked to make and answers with
// status and body. Xray answers a successful write with a plain sentence under
// a JSON content type, which the fixtures in testdata hold.
type datasetWrite struct {
	method string
	path   string
	query  string
	body   []byte
}

// datasetWriteTestID is the id the stub Jira hands back for every key, so an
// assertion on the query proves the write used the resolved id and not the key.
const datasetWriteTestID = "2474288"

func datasetWriteServer(t *testing.T, got *datasetWrite, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The dataset resource takes a numeric entity id, so a write is two
		// requests: resolve the key, then post the table.
		if strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + datasetWriteTestID + `","key":"QA-1"}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		*got = datasetWrite{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: b}
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// decodeParams reads the request body back into the shape Xray expects, so the
// assertions below talk about parameters and values rather than about JSON.
type wroteParam struct {
	Param struct {
		ID            *int   `json:"id"`
		Name          string `json:"name"`
		Rank          int    `json:"rank"`
		Combinatorial bool   `json:"combinatorial"`
		Info          struct {
			Type string `json:"type"`
		} `json:"info"`
	} `json:"param"`
	Values []struct {
		Value string `json:"value"`
		Rank  int    `json:"rank"`
	} `json:"values"`
}

func decodeParams(t *testing.T, body []byte) []wroteParam {
	t.Helper()
	var payload struct {
		Params []wroteParam `json:"params"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("request body is not the bare params object Xray wants: %v\n%s", err, body)
	}
	return payload.Params
}

// A dataset is sent to the JSON API keyed by the test's numeric issue id, not
// its key: /rest/raven/1.0/dataset is the only route that accepts a write, and
// it has no key parameter. Callers hold keys, so the id is resolved first.
func TestSetDatasetPostsToTheJSONAPIByNumericID(t *testing.T) {
	var got datasetWrite
	srv := datasetWriteServer(t, &got, http.StatusOK, "Dataset Updated with success.")
	defer srv.Close()

	err := newTestClient(srv).SetDataset(context.Background(), "QA-1", Dataset{
		Parameters: []DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}},
	})
	if err != nil {
		t.Fatalf("set dataset: %v", err)
	}

	if got.method != http.MethodPost {
		t.Errorf("method = %s, want POST", got.method)
	}
	if got.path != "/rest/raven/1.0/dataset" {
		t.Errorf("path = %s, want the raven/1.0 dataset resource", got.path)
	}
	if got.query != "entityId=2474288&type=TEST" {
		t.Errorf("query = %s, want the numeric id and TEST type", got.query)
	}
}

// Every parameter in a dataset Xray does not yet own is announced with a
// negative id. Omitting the id, or sending zero, is refused by the server with
// "Parameters (0) not associated with the dataset", so this is not cosmetic.
func TestSetDatasetMarksEveryNewParameterWithANegativeID(t *testing.T) {
	var got datasetWrite
	srv := datasetWriteServer(t, &got, http.StatusOK, "Dataset Updated with success.")
	defer srv.Close()

	err := newTestClient(srv).SetDataset(context.Background(), "QA-1", Dataset{
		Parameters: []DatasetParameter{{Name: "USERNAME"}, {Name: "REGION"}},
		Rows:       [][]string{{"alice", "eu-west"}},
	})
	if err != nil {
		t.Fatalf("set dataset: %v", err)
	}

	for _, p := range decodeParams(t, got.body) {
		if p.Param.ID == nil {
			t.Errorf("parameter %q carries no id; Xray refuses that", p.Param.Name)
			continue
		}
		if *p.Param.ID >= 0 {
			t.Errorf("parameter %q has id %d, want a negative id to mean new",
				p.Param.Name, *p.Param.ID)
		}
	}
}

// The dataset is rows of cells to everything above this layer, and columns of
// values to Xray. The transpose is this method's real work, and a row with
// fewer cells than there are parameters must still line up.
func TestSetDatasetTransposesRowsIntoParameterValues(t *testing.T) {
	var got datasetWrite
	srv := datasetWriteServer(t, &got, http.StatusOK, "Dataset Updated with success.")
	defer srv.Close()

	err := newTestClient(srv).SetDataset(context.Background(), "QA-1", Dataset{
		Parameters: []DatasetParameter{
			{Name: "USERNAME"},
			{Name: "REGION", Combinatorial: true},
		},
		Rows: [][]string{
			{"alice", "eu-west"},
			{"bob"}, // short on purpose
		},
	})
	if err != nil {
		t.Fatalf("set dataset: %v", err)
	}

	params := decodeParams(t, got.body)
	if len(params) != 2 {
		t.Fatalf("wrote %d parameters, want 2", len(params))
	}

	if params[0].Param.Name != "USERNAME" || params[0].Param.Rank != 0 {
		t.Errorf("first parameter = %q rank %d, want USERNAME rank 0",
			params[0].Param.Name, params[0].Param.Rank)
	}
	if params[1].Param.Rank != 1 {
		t.Errorf("second parameter rank = %d, want 1", params[1].Param.Rank)
	}
	// Combinatorial is a field here, not the asterisk the CSV export uses.
	if params[0].Param.Combinatorial || !params[1].Param.Combinatorial {
		t.Error("combinatorial flag did not survive the transpose")
	}
	if params[0].Param.Info.Type != "TEXT" {
		t.Errorf("parameter type = %q, want TEXT", params[0].Param.Info.Type)
	}

	gotFirst := []string{params[0].Values[0].Value, params[0].Values[1].Value}
	if !reflect.DeepEqual(gotFirst, []string{"alice", "bob"}) {
		t.Errorf("USERNAME values = %q, want alice then bob", gotFirst)
	}
	// The short row contributes an empty cell rather than dropping the column.
	gotSecond := []string{params[1].Values[0].Value, params[1].Values[1].Value}
	if !reflect.DeepEqual(gotSecond, []string{"eu-west", ""}) {
		t.Errorf("REGION values = %q, want eu-west then an empty cell", gotSecond)
	}
	for i, v := range params[1].Values {
		if v.Rank != i {
			t.Errorf("value %d has rank %d, want %d", i, v.Rank, i)
		}
	}
}

// Clearing a dataset is an empty params array. Xray publishes no DELETE on
// either dataset API and answers 405 to one, so this is the only way.
func TestSetDatasetClearsWithAnEmptyParamsArray(t *testing.T) {
	var got datasetWrite
	srv := datasetWriteServer(t, &got, http.StatusOK, "Dataset Updated with success.")
	defer srv.Close()

	if err := newTestClient(srv).SetDataset(context.Background(), "QA-1", Dataset{}); err != nil {
		t.Fatalf("clear dataset: %v", err)
	}

	params := decodeParams(t, got.body)
	if len(params) != 0 {
		t.Errorf("wrote %d parameters, want none", len(params))
	}
	// An absent key would make Xray answer 500 on getParams(); it must be [].
	if !jsonHasEmptyParamsArray(got.body) {
		t.Errorf("body = %s, want an explicit empty params array", got.body)
	}
}

func jsonHasEmptyParamsArray(b []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return false
	}
	v, ok := raw["params"]
	return ok && string(v) == "[]"
}

// A successful write answers Content-Type application/json with a body that is
// not JSON. Decoding it is the natural thing to write and fails only on
// success, so SetDataset must not look at the body at all when the status is
// good.
func TestSetDatasetIgnoresTheNonJSONSuccessBody(t *testing.T) {
	var got datasetWrite
	srv := datasetWriteServer(t, &got, http.StatusOK, "Dataset Updated with success.")
	defer srv.Close()

	if err := newTestClient(srv).SetDataset(context.Background(), "QA-1", Dataset{
		Parameters: []DatasetParameter{{Name: "A"}},
		Rows:       [][]string{{"1"}},
	}); err != nil {
		t.Fatalf("a successful write was reported as an error: %v", err)
	}
}

// A refusal carries its reason in a JSON body, and the caller needs it: the
// most likely refusal is the unassociated-parameter one, which tells a
// developer they sent the wrong id.
func TestSetDatasetSurfacesTheServerReasonOnRefusal(t *testing.T) {
	var got datasetWrite
	srv := datasetWriteServer(t, &got, http.StatusBadRequest,
		`{"error":"Parameters (0) not associated with the dataset."}`)
	defer srv.Close()

	err := newTestClient(srv).SetDataset(context.Background(), "QA-1", Dataset{
		Parameters: []DatasetParameter{{Name: "A"}},
		Rows:       [][]string{{"1"}},
	})
	if err == nil {
		t.Fatal("a 400 was reported as success")
	}
	if !strings.Contains(err.Error(), "not associated with the dataset") {
		t.Errorf("error = %q, want it to carry the server's reason", err)
	}
}
