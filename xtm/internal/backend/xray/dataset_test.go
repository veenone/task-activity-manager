package xray

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

// Xray stores parameterized test inputs as datasets, so the capability is on
// and the adapter forwards the read (#267 Phase 1).
func TestXrayReportsDatasetSupport(t *testing.T) {
	if !(&Adapter{}).Capabilities().SupportsDatasets {
		t.Fatal("Xray must report SupportsDatasets")
	}
}

// Reading a dataset and writing one are separate capabilities: an instance
// below Xray 5.1 serves neither, but a backend can model datasets and have no
// route to change them, which is what Kiwi does (#277 W1).
func TestXrayReportsDatasetWrites(t *testing.T) {
	if !(&Adapter{}).Capabilities().SupportsDatasetWrites {
		t.Fatal("Xray must report SupportsDatasetWrites")
	}
}

// The write resource takes the test's numeric issue id, so the adapter hands
// the key down and the client resolves it. This proves the table reached the
// dataset resource under the id Jira gave for that key.
func TestXraySetDatasetWritesTheTable(t *testing.T) {
	var wrotePath, wroteQuery string
	var wroteBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") {
			_, _ = w.Write([]byte(`{"id":"2474288","key":"QA-1"}`))
			return
		}
		wrotePath, wroteQuery = r.URL.Path, r.URL.RawQuery
		wroteBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte("Dataset Updated with success."))
	}))
	defer srv.Close()

	err := New(jira.NewClient(srv.URL, "tok")).SetDataset(context.Background(), "QA-1", backend.Dataset{
		Parameters: []backend.DatasetParameter{{Name: "USERNAME", Combinatorial: true}},
		Rows:       [][]string{{"alice"}},
	})
	if err != nil {
		t.Fatalf("set dataset: %v", err)
	}
	if wrotePath != "/rest/raven/1.0/dataset" {
		t.Errorf("wrote to %s, want the raven/1.0 dataset resource", wrotePath)
	}
	if wroteQuery != "entityId=2474288&type=TEST" {
		t.Errorf("query = %s, want the resolved numeric id", wroteQuery)
	}
	// The neutral DTO's own fields have to survive the hop into the Xray shape.
	body := string(wroteBody)
	for _, want := range []string{`"name":"USERNAME"`, `"combinatorial":true`, `"value":"alice"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s is missing %s", body, want)
		}
	}
}

// A demo profile has no instance to write to. Saying so beats a connection
// error naming a host that does not exist (#277 T7 will make demo persist).
func TestXrayDemoRefusesADatasetWrite(t *testing.T) {
	err := New(jira.NewClient("demo", "")).SetDataset(context.Background(), "QA-1", backend.Dataset{
		Parameters: []backend.DatasetParameter{{Name: "A"}},
		Rows:       [][]string{{"1"}},
	})
	if err == nil {
		t.Fatal("a demo profile reported a dataset write as done")
	}
	if !strings.Contains(err.Error(), "demo profile") {
		t.Errorf("error = %q, want it to name the demo profile", err)
	}
}

// Demo mode answers without a network call, so a demo adapter still returns a
// usable dataset for the keys that carry one.
func TestXrayDemoReturnsADatasetForEveryTwentiethTest(t *testing.T) {
	a := New(jira.NewClient("demo", ""))
	withData, err := a.GetDataset(context.Background(), "QA-1")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(withData.Parameters) == 0 {
		t.Fatalf("QA-1 should carry a demo dataset, got %+v", withData)
	}
	none, err := a.GetDataset(context.Background(), "QA-2")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(none.Parameters) != 0 {
		t.Fatalf("QA-2 should carry no dataset, got %+v", none)
	}
}
