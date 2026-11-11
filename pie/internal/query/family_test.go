package query

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
)

// The issue graph draws parent and sub-task edges, and draws a type worded
// the same both ways without an arrow (#464).
func TestRunReadsParentSubtasksAndSymmetricLinks(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"total":1,"issues":[{"key":"B-2","fields":{
			"parent":{"key":"B-1"},
			"subtasks":[{"key":"B-3"},{"key":"B-4"}],
			"issuelinks":[
				{"type":{"name":"Relates","outward":"relates to","inward":"relates to"},"outwardIssue":{"key":"A-1"}},
				{"type":{"name":"Blocks","outward":"blocks","inward":"is blocked by"},"inwardIssue":{"key":"C-1"}}]}}]}`))
	}))
	defer srv.Close()
	res, err := Run(context.Background(), corejira.NewClient(srv.URL, "t"), "x")
	if err != nil {
		t.Fatal(err)
	}
	got := res.Issues[0]
	if got.Parent != "B-1" || !slices.Equal(got.Subtasks, []string{"B-3", "B-4"}) {
		t.Fatalf("parent %q, subtasks %v", got.Parent, got.Subtasks)
	}
	if !got.Links[0].Symmetric || got.Links[1].Symmetric {
		t.Fatalf("links %+v", got.Links)
	}
	for _, f := range []string{"parent", "subtasks"} {
		if !slices.Contains(strings.Split(asked, ","), f) {
			t.Fatalf("fields asked %q lack %s", asked, f)
		}
	}
}

func TestIssuesFetchesKeysInBatches(t *testing.T) {
	graph := map[string]node{}
	keys := make([]string, 0, 150)
	for i := range 150 {
		k := "A-" + strconv.Itoa(i+1)
		graph[k] = node{}
		keys = append(keys, k)
	}
	var seen []string
	got, err := Issues(context.Background(), linkedJira(t, graph, nil, &seen, ""), keys, Fields{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 150 || len(seen) != 2 {
		t.Fatalf("%d issues in %d searches", len(got), len(seen))
	}
}

// Jira refuses a whole key in (...) search when one key is gone or hidden.
// Issues then asks key by key and leaves the refused ones out (#464).
func TestIssuesLeavesOutAKeyJiraRefuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jql := r.URL.Query().Get("jql")
		if strings.Contains(jql, "GONE-1") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages":["An issue with key 'GONE-1' does not exist for field 'key'."]}`))
			return
		}
		var issues []string
		for _, k := range []string{"A-1", "A-2"} {
			if strings.Contains(jql, k) {
				issues = append(issues, `{"key":"`+k+`","fields":{}}`)
			}
		}
		_, _ = w.Write([]byte(`{"total":` + strconv.Itoa(len(issues)) + `,"issues":[` + strings.Join(issues, ",") + `]}`))
	}))
	defer srv.Close()
	got, err := Issues(context.Background(), corejira.NewClient(srv.URL, "t"), []string{"A-1", "GONE-1", "A-2"}, Fields{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Key != "A-1" || got[1].Key != "A-2" {
		t.Fatalf("got %+v", got)
	}
}
