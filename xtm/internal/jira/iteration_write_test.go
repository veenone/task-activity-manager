package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Writing an iteration's status (#336). The status goes in the body: the
// .../status?status=PASS route the run resource suggests does not exist, which
// #267 Phase 4 established against a live instance.
func TestSetIterationStatusPutsTheStatusInTheBody(t *testing.T) {
	var method, path string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		// Xray answers with the id alone, not the updated iteration.
		_, _ = w.Write(readFixture(t, "testrun_iteration_written.json"))
	}))
	defer srv.Close()

	if err := newTestClient(srv).SetIterationStatus(context.Background(), "3279153", 11868, "PASS"); err != nil {
		t.Fatalf("SetIterationStatus: %v", err)
	}
	if method != http.MethodPut {
		t.Errorf("method = %s, want PUT", method)
	}
	if path != "/rest/raven/2.0/api/testrun/3279153/iteration/11868" {
		t.Errorf("path = %s", path)
	}
	if body["status"] != "PASS" {
		t.Errorf("body = %v, want the status in it", body)
	}
	// No query string: the status is not a parameter on this resource.
	if len(body) != 1 {
		t.Errorf("body carries more than the status: %v", body)
	}
}

// A refused write is reported rather than swallowed, so the view can leave the
// status where it was instead of showing one Xray did not accept.
func TestSetIterationStatusReportsARefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Invalid status"}`))
	}))
	defer srv.Close()

	err := newTestClient(srv).SetIterationStatus(context.Background(), "3279153", 11868, "NOPE")
	if err == nil {
		t.Fatal("want an error for a refused write")
	}
}

// Demo profiles never reach the wire.
func TestSetIterationStatusOnDemoDoesNothing(t *testing.T) {
	asked := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		asked = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.baseURL = "demo"
	if err := c.SetIterationStatus(context.Background(), "1", 1, "PASS"); err != nil {
		t.Fatalf("demo write: %v", err)
	}
	if asked {
		t.Error("a demo profile reached the network")
	}
}
