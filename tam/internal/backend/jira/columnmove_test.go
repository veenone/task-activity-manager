package jira_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

// transitionServer answers with the shape a real Data Center instance gave
// for an issue whose only forward move is Start Progress: the reachable
// "In Progress" is status 3, and 10004 is a status of the same board column
// that this issue's workflow cannot reach.
func transitionServer(t *testing.T, fired *string) *jirabackend.Backend {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body struct {
				Transition struct {
					ID string `json:"id"`
				} `json:"transition"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			*fired = body.Transition.ID
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"transitions":[
			{"id":"4","name":"Start Progress","to":{"id":"3","name":"In Progress"}},
			{"id":"5","name":"Resolve Issue","to":{"id":"5","name":"Corrected"}},
			{"id":"2","name":"Close Issue","to":{"id":"6","name":"Closed"}}
		]}`))
	}))
	t.Cleanup(srv.Close)
	return jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "")
}

// A board column collects several statuses and only one is reachable from
// where the card is now. Dropping on that column asks for the column, so the
// push takes the status of it the workflow actually offers, rather than
// refusing because the first id in the column happened to be unreachable.
func TestTransitionTakesTheReachableStatusOfTheColumn(t *testing.T) {
	var fired string
	b := transitionServer(t, &fired)
	// The "In Progress" column, as the board configuration lists it: the
	// migrated status first, the one this workflow reaches second.
	if err := b.Transition(context.Background(), "RND_P_4TFINT_05-124", []string{"10004", "3"}); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if fired != "4" {
		t.Errorf("fired transition %q, want 4 (Start Progress, to status 3)", fired)
	}
}

// The status actually dropped on stays first, so a column whose first status
// is reachable is not silently resolved to a sibling.
func TestTransitionPrefersTheStatusDroppedOn(t *testing.T) {
	var fired string
	b := transitionServer(t, &fired)
	if err := b.Transition(context.Background(), "RND_P_4TFINT_05-124", []string{"6", "3"}); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if fired != "2" {
		t.Errorf("fired transition %q, want 2 (Close Issue, to status 6)", fired)
	}
}

// A column with nothing reachable still refuses, and still names the target
// the user dropped on rather than the last sibling tried.
func TestTransitionRefusesWhenTheWholeColumnIsOutOfReach(t *testing.T) {
	var fired string
	b := transitionServer(t, &fired)
	err := b.Transition(context.Background(), "RND_P_4TFINT_05-124", []string{"10004", "10005"})
	if err == nil {
		t.Fatal("want a refusal")
	}
	var nt *backend.NoTransition
	if !strings.Contains(err.Error(), "10004") {
		t.Errorf("the refusal names the status dropped on: %v", err)
	}
	if ok := errors.As(err, &nt); !ok || nt.TargetStatusID != "10004" {
		t.Errorf("NoTransition target = %+v", nt)
	}
	if fired != "" {
		t.Errorf("nothing may be fired: %q", fired)
	}
}
