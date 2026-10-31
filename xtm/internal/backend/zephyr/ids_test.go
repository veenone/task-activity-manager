package zephyr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestJiraIssueIDIsReadOncePerKey matters because every step call needs the
// issue id and a test's steps are read repeatedly while the user browses. A
// resolver without a cache turns one step read into two requests, every time.
func TestJiraIssueIDIsReadOncePerKey(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/PROJ-101") {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"id":"10501","key":"PROJ-101","fields":{}}`))
	}))
	defer srv.Close()
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())

	for i := 0; i < 3; i++ {
		id, err := c.JiraIssueID(context.Background(), "PROJ-101")
		if err != nil {
			t.Fatalf("JiraIssueID: %v", err)
		}
		if id != "10501" {
			t.Fatalf("issue id = %q, want the id Jira reported", id)
		}
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Errorf("Jira was asked for the same key %d times, want 1", n)
	}
}

// TestJiraIssueIDNamesTheKeyItCouldNotResolve keeps the failure readable: the
// step read that fails is the symptom, and the key is what the user can act on.
func TestJiraIssueIDNamesTheKeyItCouldNotResolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
	}))
	defer srv.Close()
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())

	_, err := c.JiraIssueID(context.Background(), "PROJ-404")
	if err == nil {
		t.Fatal("JiraIssueID returned no error for an issue Jira does not have")
	}
	if !strings.Contains(err.Error(), "PROJ-404") {
		t.Errorf("error does not name the key: %v", err)
	}
}

// TestConnectionRefusesAnInstanceWhoseTestTypeIsNamedSomethingElse covers the
// one way a Zephyr profile can connect cleanly and still show nothing. The
// Jira searches look for an issue type named Test; Zephyr reports the id of
// whichever type it uses. When the two disagree the project reads as empty,
// which sends the user looking for a sync problem that is not there.
func TestConnectionRefusesAnInstanceWhoseTestTypeIsNamedSomethingElse(t *testing.T) {
	srv := zephyrServer(t, map[string]func(http.ResponseWriter){
		"/rest/api/2/issuetype": func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`[{"id":"1","name":"Bug"},{"id":"9","name":"Test Case"}]`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	_, err := a.TestConnection(context.Background())
	if err == nil {
		t.Fatal("TestConnection accepted an instance whose test issue type is not named Test")
	}
	if !strings.Contains(err.Error(), "Test Case") {
		t.Errorf("error does not name what the type is actually called: %v", err)
	}
}

// TestConnectionAcceptsAnInstanceThatDoesNotListTheType guards against the
// check refusing a working instance. Reading the issue type list needs
// permission a plain user may not have, and an id that is missing from the
// list proves nothing either way, so silence is not evidence of a problem.
func TestConnectionAcceptsAnInstanceThatDoesNotListTheType(t *testing.T) {
	srv := zephyrServer(t, map[string]func(http.ResponseWriter){
		"/rest/api/2/issuetype": func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errorMessages":["You do not have permission"]}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	if _, err := a.TestConnection(context.Background()); err != nil {
		t.Errorf("TestConnection refused an instance that only withheld its issue type list: %v", err)
	}
}
