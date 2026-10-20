package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Jira 9 removed GET /rest/api/2/issue/createmeta. Once it is gone the path
// matches /rest/api/2/issue/{issueIdOrKey} instead, with "createmeta" read as
// the key, so the instance answers 404 "Issue Does Not Exist" (#313). This
// handler is that instance: the classic endpoint is a 404 in Jira's own
// wording, and /rest/api/2/field still works.
func createmetaRemoved(t *testing.T, fieldCalled *bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/issue/createmeta":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errorMessages": []string{"Issue Does Not Exist"}, "errors": map[string]any{},
			})
		case "/rest/api/2/field":
			if fieldCalled != nil {
				*fieldCalled = true
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "summary", "name": "Summary", "custom": false},
				{"id": "customfield_10101", "name": "Automation Status", "custom": true,
					"schema": map[string]any{"type": "option"}},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// The sync aborted on this: syncCustomFields returns the error and the whole
// run fails, reported to the user as a wrong Jira URL. The fallback to
// /rest/api/2/field already exists for instances that answer createmeta with
// no Test fields, and a removed endpoint is the same situation.
func TestListCustomFieldsSurvivesCreatemetaRemoval(t *testing.T) {
	var fieldCalled bool
	srv := createmetaRemoved(t, &fieldCalled)
	defer srv.Close()

	defs, err := newTestClient(srv).ListCustomFields(context.Background(), "QA")
	if err != nil {
		t.Fatalf("ListCustomFields: %v", err)
	}
	if !fieldCalled {
		t.Error("the /rest/api/2/field fallback was never reached")
	}
	if len(defs) != 1 || defs[0].ID != "customfield_10101" || defs[0].Type != "option" {
		t.Errorf("defs = %#v, want the one custom field /rest/api/2/field reported", defs)
	}
}

// The same call backs the Create Bug and Create Requirement forms, where it is
// only asked which extra fields are required. Its contract already treats an
// instance without the fields expansion as "no extra fields" rather than an
// error, so a removed endpoint reads the same way and the form still opens.
func TestGetCreateScreenFieldsSurvivesCreatemetaRemoval(t *testing.T) {
	srv := createmetaRemoved(t, nil)
	defer srv.Close()

	fields, err := newTestClient(srv).GetCreateScreenFields(
		context.Background(), "QA", "", "Bug", []string{"summary"})
	if err != nil {
		t.Fatalf("GetCreateScreenFields: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("fields = %#v, want none so the form opens without them", fields)
	}
}
