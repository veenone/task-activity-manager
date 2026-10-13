package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGetTransitionsReturnsEachTransitionsFieldsInOneRequest(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/rest/api/2/issue/QA-1/transitions" || r.URL.Query().Get("expand") != "transitions.fields" {
			t.Errorf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []any{
			map[string]any{"id": "11", "name": "Start", "to": map[string]any{"name": "In Progress"}},
			map[string]any{"id": "31", "name": "Close", "to": map[string]any{"name": "Done"},
				"fields": map[string]any{
					"resolution": map[string]any{
						"name": "Resolution", "required": true,
						"schema":        map[string]any{"type": "resolution"},
						"allowedValues": []any{map[string]any{"id": "1", "name": "Fixed"}},
					},
					"comment": map[string]any{
						"name": "Comment", "required": false,
						"schema": map[string]any{"type": "comments-page"},
					},
				}},
		}})
	}))
	defer srv.Close()

	ts, err := newTestClient(srv).GetTransitions(context.Background(), "QA-1", "Open")
	if err != nil {
		t.Fatalf("transitions: %v", err)
	}
	if requests != 1 {
		t.Fatalf("made %d requests, want 1", requests)
	}
	if len(ts[0].Fields) != 0 {
		t.Fatalf("Start fields %+v, want none", ts[0].Fields)
	}
	want := []FieldDef{
		{ID: "comment", Name: "Comment", Required: false, Type: "text", AllowedValues: []FieldOption{}},
		{ID: "resolution", Name: "Resolution", Required: true, Type: "text",
			AllowedValues: []FieldOption{{ID: "1", Value: "Fixed"}}},
	}
	if !reflect.DeepEqual(ts[1].Fields, want) {
		t.Fatalf("Close fields\n got %+v\nwant %+v", ts[1].Fields, want)
	}
}

func TestPostTransitionSendsFieldsInTheSameRequest(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		bodies = append(bodies, b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := newTestClient(srv)

	if err := c.PostTransition(context.Background(), "QA-1", "31",
		map[string]any{"resolution": map[string]any{"id": "1"}}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if err := c.PostTransition(context.Background(), "QA-1", "11", nil); err != nil {
		t.Fatalf("post without fields: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("requests %d", len(bodies))
	}
	fields, _ := bodies[0]["fields"].(map[string]any)
	if res, _ := fields["resolution"].(map[string]any); res["id"] != "1" {
		t.Fatalf("first body %v", bodies[0])
	}
	if _, has := bodies[1]["fields"]; has {
		t.Fatalf("a transition without fields sent %v", bodies[1])
	}
}
