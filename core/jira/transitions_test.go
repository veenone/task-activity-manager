package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestTransitionsDecodesFieldsWithNamesAndAllowedValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/PLAT-412/transitions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("expand"); got != "transitions.fields" {
			t.Errorf("expand = %q", got)
		}
		_, _ = w.Write([]byte(`{"transitions":[
			{"id":"21","name":"In Progress","to":{"id":"3","name":"In Progress"},"fields":{}},
			{"id":"31","name":"Done","to":{"id":"5","name":"Done"},"fields":{
				"resolution":{"name":"Resolution","required":true,"allowedValues":[
					{"id":"1","name":"Fixed"},
					{"id":"2","name":"Won't Fix"}
				]}
			}}
		]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	transitions, err := c.Transitions(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("transitions: %v", err)
	}
	if len(transitions) != 2 {
		t.Fatalf("transitions = %+v", transitions)
	}

	inProgress := transitions[0]
	if inProgress.ID != "21" || inProgress.Name != "In Progress" || inProgress.To.ID != "3" {
		t.Errorf("transition 0 = %+v", inProgress)
	}
	if len(inProgress.Fields) != 0 {
		t.Errorf("transition 0 fields = %+v, want none", inProgress.Fields)
	}

	done := transitions[1]
	if done.ID != "31" || done.Name != "Done" || done.To.ID != "5" || done.To.Name != "Done" {
		t.Errorf("transition 1 = %+v", done)
	}
	// The map is keyed by field id, so a caller checking a required field
	// has the name to put in an error message and not just the id.
	res, ok := done.Fields["resolution"]
	if !ok {
		t.Fatalf("done.Fields = %+v, want a resolution entry", done.Fields)
	}
	if res.Name != "Resolution" || !res.Required {
		t.Errorf("resolution field = %+v", res)
	}
	wantValues := []RawAllowedValue{{ID: "1", Name: "Fixed"}, {ID: "2", Name: "Won't Fix"}}
	if !reflect.DeepEqual(res.AllowedValues, wantValues) {
		t.Errorf("resolution allowed values = %+v, want %+v", res.AllowedValues, wantValues)
	}
}

// TestDoTransitionSendsOnlyTheTransitionIDWhenFieldsIsEmpty covers both
// ways a caller can say "no fields": a nil map and an empty one. Both have
// to produce {"transition":{"id":"31"}} with no fields key, since a Data
// Center workflow with a resolution screen can refuse a request that always
// sends "fields" as an empty object.
func TestDoTransitionSendsOnlyTheTransitionIDWhenFieldsIsEmpty(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]any
	}{
		{name: "nil map", fields: nil},
		{name: "empty map", fields: map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]json.RawMessage
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/rest/api/2/issue/PLAT-412/transitions" {
					t.Errorf("path = %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("method = %s", r.Method)
				}
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
			if err := c.DoTransition(context.Background(), "PLAT-412", "31", tc.fields); err != nil {
				t.Fatalf("do transition: %v", err)
			}

			if len(gotBody) != 1 {
				t.Fatalf("body = %+v, want only a transition key", gotBody)
			}
			if _, ok := gotBody["fields"]; ok {
				t.Errorf("body = %+v, want no fields key at all", gotBody)
			}
			var transition struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(gotBody["transition"], &transition); err != nil {
				t.Fatalf("unmarshal transition: %v", err)
			}
			if transition.ID != "31" {
				t.Errorf("transition id = %q, want 31", transition.ID)
			}
		})
	}
}

// TestDoTransitionSendsFieldsWhenPresent pins down the shape a resolution
// screen needs: {"transition":{"id":...},"fields":{...}} with fields
// carried through as given, not just the transition id most Data Center
// workflows will refuse for a transition into Done.
func TestDoTransitionSendsFieldsWhenPresent(t *testing.T) {
	var gotBody struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
		Fields map[string]any `json:"fields"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	fields := map[string]any{"resolution": map[string]any{"id": "1"}}
	if err := c.DoTransition(context.Background(), "PLAT-412", "31", fields); err != nil {
		t.Fatalf("do transition: %v", err)
	}
	if gotBody.Transition.ID != "31" {
		t.Errorf("transition id = %q, want 31", gotBody.Transition.ID)
	}
	res, ok := gotBody.Fields["resolution"].(map[string]any)
	if !ok || res["id"] != "1" {
		t.Errorf("fields = %+v, want resolution id 1", gotBody.Fields)
	}
}

// TestTransitionsAndDoTransitionEscapeTheKey uses a key that actually
// changes under escaping, the way TestBoardIssueKeysEscapesSprintID does for
// the sprint id, so a dropped url.PathEscape cannot pass unnoticed.
func TestTransitionsAndDoTransitionEscapeTheKey(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.EscapedPath())
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"transitions":[]}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	key := "PLAT 412/x"
	if _, err := c.Transitions(context.Background(), key); err != nil {
		t.Fatalf("transitions: %v", err)
	}
	if err := c.DoTransition(context.Background(), key, "31", nil); err != nil {
		t.Fatalf("do transition: %v", err)
	}

	want := "/rest/api/2/issue/PLAT%20412%2Fx/transitions"
	if len(gotPaths) != 2 {
		t.Fatalf("gotPaths = %v, want 2 requests", gotPaths)
	}
	for _, p := range gotPaths {
		if p != want {
			t.Errorf("path = %q, want %q", p, want)
		}
	}
}
