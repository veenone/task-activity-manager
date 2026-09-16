package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// TestCreateFilterReturnsTheIDAndSendsTheProjectShare pins the two things a
// board's filter needs: the id Jira assigned, echoed back on the body, and
// the project share, sent by default so the board it backs is visible to
// the whole team rather than only its creator.
func TestCreateFilterReturnsTheIDAndSendsTheProjectShare(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"10050","name":"Sprint board filter"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	id, err := c.CreateFilter(context.Background(), "Sprint board filter", `project = "PLAT"`, "", "10000")
	if err != nil {
		t.Fatalf("create filter: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/rest/api/2/filter" {
		t.Errorf("path = %s", gotPath)
	}
	wantBody := map[string]any{
		"name": "Sprint board filter",
		"jql":  `project = "PLAT"`,
		"sharePermissions": []any{
			map[string]any{"type": "project", "project": map[string]any{"id": "10000"}},
		},
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("body = %+v, want %+v", gotBody, wantBody)
	}
	if id != "10050" {
		t.Errorf("id = %q, want 10050", id)
	}
}

// TestCreateBoardReturnsTheBoardIDAndSendsLocation pins the id Jira handed
// back and the location object, which is what ties the board to its project
// rather than leaving it to whatever project the filter's JQL happens to
// mention.
func TestCreateBoardReturnsTheBoardIDAndSendsLocation(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42,"name":"PLAT board"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	id, err := c.CreateBoard(context.Background(), "PLAT board", "scrum", "10050", "PLAT")
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/rest/agile/1.0/board" {
		t.Errorf("path = %s", gotPath)
	}
	wantBody := map[string]any{
		"name":     "PLAT board",
		"type":     "scrum",
		"filterId": float64(10050),
		"location": map[string]any{"type": "project", "projectKeyOrId": "PLAT"},
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("body = %+v, want %+v", gotBody, wantBody)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
}

// TestAddToBoardBacklogBatchesAt20 pins the batch width the brief calls for:
// a planning session's worth of adds must not ride in one request that a
// single bad key can take down whole.
func TestAddToBoardBacklogBatchesAt20(t *testing.T) {
	var requests [][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Issues []string `json:"issues"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		requests = append(requests, body.Issues)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	keys := make([]string, 25)
	for i := range keys {
		keys[i] = fmt.Sprintf("PLAT-%d", i+1)
	}
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.AddToBoardBacklog(context.Background(), 7, keys); err != nil {
		t.Fatalf("add to backlog: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2 batches for 25 keys", len(requests))
	}
	if len(requests[0]) != 20 || len(requests[1]) != 5 {
		t.Errorf("batch sizes = %d, %d, want 20 then 5", len(requests[0]), len(requests[1]))
	}
	if requests[0][0] != "PLAT-1" || requests[1][0] != "PLAT-21" {
		t.Errorf("batches = %v, want the keys split in order", requests)
	}
}

// TestAddToBoardBacklogA207FailsTheWholeBatch proves this call goes through
// the shared bulkWrite, which already treats 207 Multi-Status as a failure
// of the whole batch: a second 207 path here would be the bug the brief
// warns against.
func TestAddToBoardBacklogA207FailsTheWholeBatch(t *testing.T) {
	srv := multiStatusServer(t, `{"errorMessages":["Issue already on a board"]}`)
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	err := c.AddToBoardBacklog(context.Background(), 7, []string{"PLAT-1", "PLAT-2"})
	if err == nil {
		t.Fatal("want an error for a 207")
	}
	if !strings.Contains(err.Error(), "add to board backlog") {
		t.Errorf("err = %v, want it to name the operation, which proves this went through bulkWrite", err)
	}
	if !strings.Contains(err.Error(), "Issue already on a board") {
		t.Errorf("err = %v, want Jira's own message kept", err)
	}
}
