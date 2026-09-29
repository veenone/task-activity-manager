package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

// TestWorklogsPagesToTheEnd is why this read is its own call rather than
// fields=worklog on a search: the search caps an issue's worklogs at 20 and
// says nothing about the rest. The endpoint pages, so the reader has to walk
// it, and an issue with more entries than one page holds must come back
// whole.
func TestWorklogsPagesToTheEnd(t *testing.T) {
	var starts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/PLAT-1/worklog" {
			t.Errorf("path = %s", r.URL.Path)
		}
		starts = append(starts, r.URL.Query().Get("startAt"))
		start, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
		// Three entries in all, two on the first page, one on the second.
		entries := []string{
			`{"id":"1","timeSpent":"2h","timeSpentSeconds":7200,"started":"2026-09-29T13:00:00.000+0700","author":{"name":"jdoe","displayName":"Jane Doe"},"comment":"Pairing"}`,
			`{"id":"2","timeSpent":"30m","timeSpentSeconds":1800,"started":"2026-09-29T16:00:00.000+0700","author":{"name":"jdoe","displayName":"Jane Doe"}}`,
			`{"id":"3","timeSpent":"1d","timeSpentSeconds":28800,"started":"2026-09-28T09:00:00.000+0700","author":{"name":"ash","displayName":"Ash Khan"}}`,
		}
		page := entries[:2]
		if start > 0 {
			page = entries[2:]
		}
		body := fmt.Sprintf(`{"startAt":%d,"maxResults":2,"total":3,"worklogs":[%s]}`, start, join(page))
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	got, err := c.Worklogs(context.Background(), "PLAT-1")
	if err != nil {
		t.Fatalf("worklogs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("read %d worklogs, want all 3 across both pages: %+v", len(got), got)
	}
	if !reflect.DeepEqual(starts, []string{"0", "2"}) {
		t.Errorf("startAt values = %v, want the second page asked for from 2", starts)
	}
	if got[0].TimeSpent != "2h" || got[0].TimeSpentSeconds != 7200 || got[0].Comment != "Pairing" {
		t.Errorf("first entry = %+v", got[0])
	}
	if got[0].Author.DisplayName != "Jane Doe" {
		t.Errorf("first author = %+v, want the display name decoded", got[0].Author)
	}
	// The offset comes back exactly as Jira sent it: whoever renders the day
	// needs the zone the work was logged in, not a normalised instant.
	if got[2].Started != "2026-09-28T09:00:00.000+0700" {
		t.Errorf("third started = %q, want the offset kept", got[2].Started)
	}
}

// TestWorklogsStopsOnAnEmptyPage guards the instance that reports a total it
// will not serve: the loop has to end on an empty page rather than ask for
// the missing rows for ever.
func TestWorklogsStopsOnAnEmptyPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 4 {
			t.Fatalf("still asking after %d pages; the loop does not end on an empty page", calls)
		}
		start, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
		if start == 0 {
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":1,"total":9,"worklogs":[{"id":"1","timeSpent":"1h","timeSpentSeconds":3600,"started":"2026-09-29T13:00:00.000+0700"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"startAt":1,"maxResults":1,"total":9,"worklogs":[]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	got, err := c.Worklogs(context.Background(), "PLAT-1")
	if err != nil {
		t.Fatalf("worklogs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d worklogs, want the one the instance actually served", len(got))
	}
	if calls != 2 {
		t.Errorf("made %d requests, want 2: the first page and the empty one that ended it", calls)
	}
}

// TestAddWorklogSendsStartedTimeSpentAndComment is the write half. The
// started stamp goes over the wire verbatim, offset and all: Jira dates the
// entry by it, and stripping the offset moves the work to another day in
// every report that groups by day.
func TestAddWorklogSendsStartedTimeSpentAndComment(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"10001"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	err := c.AddWorklog(context.Background(), "PLAT-1", "2026-09-29T01:00:00.000+0700", "2h 30m", "Pairing on the migration")
	if err != nil {
		t.Fatalf("add worklog: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/rest/api/2/issue/PLAT-1/worklog" {
		t.Errorf("path = %s", gotPath)
	}
	wantBody := map[string]any{
		"started":   "2026-09-29T01:00:00.000+0700",
		"timeSpent": "2h 30m",
		"comment":   "Pairing on the migration",
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("body = %+v, want %+v", gotBody, wantBody)
	}
}

// TestAddWorklogOmitsAnEmptyComment: Jira takes a worklog with no comment,
// and sending "" writes an empty comment onto the entry.
func TestAddWorklogOmitsAnEmptyComment(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"10002"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.AddWorklog(context.Background(), "PLAT-1", "2026-09-29T01:00:00.000+0700", "90m", ""); err != nil {
		t.Fatalf("add worklog: %v", err)
	}
	if _, ok := gotBody["comment"]; ok {
		t.Errorf("body = %+v, want no comment key at all", gotBody)
	}
}

// join is strings.Join for the JSON fragments above, kept local so the test
// reads as one page of entries rather than a string build.
func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}
