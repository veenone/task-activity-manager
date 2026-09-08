package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// multiStatusServer answers every request with 207 and the given body, the
// way Jira reports a bulk write it only partly accepted.
func multiStatusServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestRankIssueRejectsEvery207 is the rule the three bulk writes rest on:
// Jira answers 204 when every issue landed and 207 only when at least one
// did not, so a 207 is a failure whatever body came with it. Each case here
// is a body a schema-matching parser would fail to recognise and wave
// through: the entries array Atlassian documents for this endpoint, an
// errorMessages body, a bare {}, an empty body, and a body that is not JSON
// at all. None of them may return nil, and each must leave something in the
// message a person can act on.
func TestRankIssueRejectsEvery207(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		contains []string
	}{
		{
			name:     "the documented entries array",
			body:     `{"entries":[{"issueId":10001,"status":400,"errors":["The issue could not be ranked"]}]}`,
			contains: []string{"10001", "400", "The issue could not be ranked"},
		},
		{
			name:     "an errors object keyed by issue key",
			body:     `{"errors":{"PLAT-412":"The issue is already ranked there"}}`,
			contains: []string{"PLAT-412", "already ranked"},
		},
		{
			name:     "an errorMessages body",
			body:     `{"errorMessages":["Rank field is not on the screen"]}`,
			contains: []string{"Rank field is not on the screen"},
		},
		{
			name:     "a bare empty object",
			body:     `{}`,
			contains: []string{"{}"},
		},
		{
			name:     "no body at all",
			body:     ``,
			contains: []string{"no response body"},
		},
		{
			name:     "a body that is not JSON",
			body:     `<html><body>Gateway trouble</body></html>`,
			contains: []string{"Gateway trouble"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := multiStatusServer(t, tc.body)
			c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
			err := c.RankIssue(context.Background(), "PLAT-412", "PLAT-409", true)
			if err == nil {
				t.Fatalf("err = nil, want a failure: a 207 means Jira refused at least one issue, body %q", tc.body)
			}
			for _, want := range tc.contains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want it to mention %q", err, want)
				}
			}
			// Fix 3: the message ends up in a journal row, so it has to say
			// which write was refused, and which issues it was made for.
			if !strings.Contains(err.Error(), "rank issue") {
				t.Errorf("err = %v, want it to name the operation", err)
			}
			if !strings.Contains(err.Error(), "/rest/agile/1.0/issue/rank") {
				t.Errorf("err = %v, want it to name the path", err)
			}
			// The rank endpoint reports rejections by numeric issue id, so
			// the key the caller ranked has to be in the message too.
			if !strings.Contains(err.Error(), "PLAT-412") {
				t.Errorf("err = %v, want it to name the issue the call was made with", err)
			}
		})
	}
}

// TestMoveToSprintRejectsOn207 keeps the same rule on the sprint move.
// Atlassian documents this endpoint as 204 or 4xx, so a 207 here is an
// instance doing something undocumented, which is all the more reason not
// to record the move as landed.
func TestMoveToSprintRejectsOn207(t *testing.T) {
	srv := multiStatusServer(t, `{"errors":{"PLAT-409":"The issue is already in this sprint"}}`)
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	err := c.MoveToSprint(context.Background(), "12", []string{"PLAT-412", "PLAT-409"})
	if err == nil {
		t.Fatal("want an error for a 207 rejecting one of two issues")
	}
	if !strings.Contains(err.Error(), "PLAT-409") {
		t.Errorf("err = %v, want it to name the rejected issue PLAT-409", err)
	}
	if !strings.Contains(err.Error(), "move to sprint") {
		t.Errorf("err = %v, want it to name the operation", err)
	}
}

// TestMoveToBacklogRejectsOn207WithAnUnknownBody pins the fail-closed rule
// on the third bulk write, with a body no parser in this package models.
func TestMoveToBacklogRejectsOn207WithAnUnknownBody(t *testing.T) {
	srv := multiStatusServer(t, `{"failures":[{"key":"PLAT-412"}]}`)
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	err := c.MoveToBacklog(context.Background(), []string{"PLAT-412"})
	if err == nil {
		t.Fatal("want an error for a 207 whose body this package does not model")
	}
	if !strings.Contains(err.Error(), "move to backlog") || !strings.Contains(err.Error(), "PLAT-412") {
		t.Errorf("err = %v, want the operation and the issue key", err)
	}
}

// TestBulkWritesAcceptAPlain2xx is the other half of the rule: a 204 is
// success and nothing about the body is examined, so a proxy that answers a
// 204 or a 200 with a non-JSON body cannot turn a landed write into a
// failure.
func TestBulkWritesAcceptAPlain2xx(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "204 with no body", status: http.StatusNoContent},
		{name: "200 with a non-JSON body", status: http.StatusOK, body: "<html>ok</html>"},
		{name: "200 with an empty JSON object", status: http.StatusOK, body: `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
			if err := c.RankIssue(context.Background(), "PLAT-412", "PLAT-409", true); err != nil {
				t.Errorf("rank issue: %v", err)
			}
			if err := c.MoveToSprint(context.Background(), "12", []string{"PLAT-412"}); err != nil {
				t.Errorf("move to sprint: %v", err)
			}
			if err := c.MoveToBacklog(context.Background(), []string{"PLAT-412"}); err != nil {
				t.Errorf("move to backlog: %v", err)
			}
		})
	}
}

// TestBulkWriteSurfacesA4xx keeps the ordinary failure path readable: the
// status, the path, Jira's message, and the operation that was refused.
func TestBulkWriteSurfacesA4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["Sprint does not exist"]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	err := c.MoveToSprint(context.Background(), "12", []string{"PLAT-412"})
	if err == nil {
		t.Fatal("want an error for a 400 on the sprint move")
	}
	for _, want := range []string{"move to sprint", "400", "Sprint does not exist", "/rest/agile/1.0/sprint/12/issue"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to mention %q", err, want)
		}
	}
}
