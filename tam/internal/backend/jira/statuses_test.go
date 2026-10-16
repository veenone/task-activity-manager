package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

// TestStatusesReadEachStatusesCategory reads /rest/api/2/status, keeping
// Jira's three category keys and dropping one it does not define (I1).
func TestStatusesReadEachStatusesCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/status" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
			{"id":"1","name":"To Do","statusCategory":{"key":"new"}},
			{"id":"3","name":"In Progress","statusCategory":{"key":"indeterminate"}},
			{"id":"10400","name":"Odd","statusCategory":{"key":"future"}}
		]`))
	}))
	t.Cleanup(srv.Close)
	b := jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "Requirement")

	var sb backend.StatusBackend = b
	got, err := sb.Statuses(context.Background())
	if err != nil {
		t.Fatalf("Statuses: %v", err)
	}
	want := []backend.Status{{ID: "1", Name: "To Do", Category: "new"}, {ID: "3", Name: "In Progress", Category: "indeterminate"}, {ID: "10400", Name: "Odd"}}
	if len(got) != len(want) {
		t.Fatalf("statuses = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("status %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
