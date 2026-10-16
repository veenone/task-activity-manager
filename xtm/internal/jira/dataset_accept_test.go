package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// acceptStrictDatasetServer behaves the way a real Xray instance does: the
// dataset export is served as application/octet-stream, and a request that
// will not accept that media type is refused.
//
// Every other test for this feature drives a server that ignores Accept, which
// is why #279 shipped. This one does not ignore it.
func acceptStrictDatasetServer(t *testing.T, gotAccept *string, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		*gotAccept = accept
		if !acceptsOctetStream(accept) {
			// Jersey reports its own 406 inside a 500, which is what the
			// instance was observed doing.
			w.Header().Set("Content-Type", "application/json;charset=UTF-8")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"HTTP 406 Not Acceptable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream;charset=UTF-8")
		_, _ = w.Write([]byte(body))
	}))
}

// acceptsOctetStream is the server's half of content negotiation: an empty
// header or one listing */* or application/octet-stream is served.
func acceptsOctetStream(accept string) bool {
	if strings.TrimSpace(accept) == "" {
		return true
	}
	for _, part := range strings.Split(accept, ",") {
		switch strings.TrimSpace(strings.Split(part, ";")[0]) {
		case "*/*", "application/*", "application/octet-stream":
			return true
		}
	}
	return false
}

// The bug: GetDataset asked for application/json and the export is not JSON,
// so a real instance refused every read. The export now serves the
// container-level read, since #283 moved a test's own dataset to the JSON
// model, and it is still the route that answers octet-stream.
func TestGetDatasetReadsAnInstanceThatRefusesJSONAccept(t *testing.T) {
	var accept string
	srv := acceptStrictDatasetServer(t, &accept, "USERNAME,REGION\nalice,eu-west\n")
	defer srv.Close()

	d, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "EXEC-1")
	if err != nil {
		t.Fatalf("GetDataset refused by a server that serves octet-stream: %v (Accept was %q)", err, accept)
	}
	if len(d.Parameters) != 2 {
		t.Fatalf("read %d parameters, want 2 (Accept was %q)", len(d.Parameters), accept)
	}
	if len(d.Rows) != 1 || d.Rows[0][0] != "alice" {
		t.Errorf("rows = %q, want one row starting alice", d.Rows)
	}
	if !acceptsOctetStream(accept) {
		t.Errorf("Accept = %q, which a real instance refuses", accept)
	}
}

// The other GETs fetch JSON and must keep asking for it: changing the shared
// default would be a far wider change than this bug needs.
func TestOtherGetsStillAskForJSON(t *testing.T) {
	var accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetTestSteps(context.Background(), "QA-1"); err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if accept != "application/json" {
		t.Errorf("Accept = %q, want application/json unchanged", accept)
	}
}
