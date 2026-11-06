package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
)

// node is one issue on the fake server: its status category and the keys it
// links to. Links are added in both directions, as Jira shows them.
type node struct {
	category string
	links    []string
}

var keyIn = regexp.MustCompile(`key in \(([^)]*)\)`)

// linkedJira serves matched for a plain query and any key of graph for
// key in (...). Every JQL it is sent is recorded in seen.
func linkedJira(t *testing.T, graph map[string]node, matched []string, seen *[]string, refuseKeyIn string) *corejira.Client {
	t.Helper()
	both := map[string][]string{}
	for k, n := range graph {
		for _, o := range n.links {
			both[k] = append(both[k], o)
			both[o] = append(both[o], k)
		}
	}
	render := func(k string) map[string]any {
		var links []any
		for _, o := range both[k] {
			links = append(links, map[string]any{
				"type":         map[string]any{"name": "Cloners", "outward": "clones", "inward": "is cloned by"},
				"outwardIssue": map[string]any{"key": o},
			})
		}
		return map[string]any{"key": k, "fields": map[string]any{
			"project":    map[string]any{"key": strings.Split(k, "-")[0]},
			"status":     map[string]any{"name": "S", "statusCategory": map[string]any{"key": graph[k].category}},
			"issuelinks": links,
		}}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jql := r.URL.Query().Get("jql")
		*seen = append(*seen, jql)
		keys := matched
		if m := keyIn.FindStringSubmatch(jql); m != nil {
			if refuseKeyIn != "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"errorMessages":[%q]}`, refuseKeyIn)
				return
			}
			keys = nil
			for _, k := range strings.Split(m[1], ",") {
				if _, ok := graph[strings.TrimSpace(k)]; ok {
					keys = append(keys, strings.TrimSpace(k))
				}
			}
		}
		var issues []any
		start := 0
		_, _ = fmt.Sscan(r.URL.Query().Get("startAt"), &start)
		for i := start; i < len(keys) && i < start+100; i++ {
			issues = append(issues, render(keys[i]))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total": len(keys), "issues": issues})
	}))
	t.Cleanup(srv.Close)
	return corejira.NewClient(srv.URL, "t")
}

func keysOf(issues []Issue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Key)
	}
	return out
}

func TestExploreFollowsLinksAcrossHopsAndEndsOnACycle(t *testing.T) {
	graph := map[string]node{
		"A-1": {"done", []string{"B-1"}},
		"B-1": {"new", []string{"C-1"}},
		"C-1": {"new", []string{"B-1"}}, // B and C link each other twice: a cycle
	}
	var seen []string
	res, err := Explore(context.Background(), linkedJira(t, graph, []string{"A-1"}, &seen, ""), "q", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := keysOf(res.Issues); !slices.Equal(got, []string{"A-1", "B-1", "C-1"}) {
		t.Fatalf("issues %v", got)
	}
	if !res.Issues[0].Matched || res.Issues[1].Matched || res.Issues[2].Matched {
		t.Error("only A-1 was matched by the query")
	}
	if len(seen) != 3 || seen[1] != "key in (B-1)" || seen[2] != "key in (C-1)" {
		t.Errorf("searches %q", seen)
	}
	if len(res.Sets) != 1 || !slices.Equal(res.Sets[0].Keys, []string{"A-1", "B-1", "C-1"}) || !res.Sets[0].OutOfStep {
		t.Errorf("sets %+v", res.Sets)
	}
	if !slices.Equal(res.Sets[0].Projects, []string{"A", "B", "C"}) {
		t.Errorf("projects %v", res.Sets[0].Projects)
	}
}

// A link inside one project is not what the views trace, and following them
// would fetch a project's whole web of related issues.
func TestExploreIgnoresLinksInsideOneProject(t *testing.T) {
	graph := map[string]node{"A-1": {"new", []string{"A-2"}}, "A-2": {"new", nil}}
	var seen []string
	res, err := Explore(context.Background(), linkedJira(t, graph, []string{"A-1"}, &seen, ""), "q", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || len(res.Issues) != 1 || len(res.Sets) != 0 {
		t.Fatalf("searches %q, issues %v, sets %+v", seen, keysOf(res.Issues), res.Sets)
	}
}

func TestExploreAsksForAHundredKeysAtATime(t *testing.T) {
	graph := map[string]node{"A-1": {"new", nil}}
	for i := range 150 {
		k := fmt.Sprintf("B-%d", i)
		graph[k] = node{"done", nil}
		n := graph["A-1"]
		n.links = append(n.links, k)
		graph["A-1"] = n
	}
	var seen []string
	res, err := Explore(context.Background(), linkedJira(t, graph, []string{"A-1"}, &seen, ""), "q", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 || strings.Count(seen[1], ",") != 99 || strings.Count(seen[2], ",") != 49 {
		t.Fatalf("searched %d times", len(seen))
	}
	if len(res.Issues) != 151 || len(res.Sets) != 1 || len(res.Sets[0].Keys) != 151 {
		t.Fatalf("issues %d, sets %d", len(res.Issues), len(res.Sets))
	}
}

func TestExploreStopsAtTheCeilingAndSaysSo(t *testing.T) {
	graph := map[string]node{}
	var matched []string
	for i := range MaxRows - 1 {
		k := fmt.Sprintf("A-%d", i)
		graph[k] = node{"new", nil}
		matched = append(matched, k)
	}
	graph["A-0"] = node{"new", []string{"B-1", "B-2", "B-3"}}
	for _, k := range []string{"B-1", "B-2", "B-3"} {
		graph[k] = node{"done", nil}
	}
	var seen []string
	res, err := Explore(context.Background(), linkedJira(t, graph, matched, &seen, ""), "q", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Issues) != MaxRows || !res.LinksTruncated {
		t.Fatalf("issues %d, links truncated %v", len(res.Issues), res.LinksTruncated)
	}
}

func TestExploreKeepsTheRowsWhenALinkSearchFails(t *testing.T) {
	const msg = "An issue with key 'B-1' does not exist for field 'key'."
	graph := map[string]node{"A-1": {"new", []string{"B-1"}}, "B-1": {"done", nil}}
	var seen []string
	res, err := Explore(context.Background(), linkedJira(t, graph, []string{"A-1"}, &seen, msg), "q", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.LinkError != msg || !slices.Equal(keysOf(res.Issues), []string{"A-1"}) {
		t.Fatalf("link error %q, issues %v", res.LinkError, keysOf(res.Issues))
	}
}

func TestSetsAreOutOfStepOnlyWhenFinishedMeetsUnfinished(t *testing.T) {
	issue := func(key, cat string, links ...string) Issue {
		is := Issue{Key: key, Project: strings.Split(key, "-")[0], StatusCategory: cat}
		for _, l := range links {
			is.Links = append(is.Links, Link{Key: l})
		}
		return is
	}
	sets := Sets([]Issue{
		issue("A-1", "done", "B-1"), issue("B-1", "done", "A-1"), // all finished
		issue("A-2", "done", "B-2"), issue("B-2", "indeterminate", "A-2"), // finished beside unfinished
		issue("A-3", "new", "B-3"), issue("B-3", "new", "A-3"), // none finished
		issue("A-4", "new", "C-9"), // links to an issue that was never fetched
	})
	var got []string
	for _, s := range sets {
		got = append(got, fmt.Sprintf("%s:%v", s.Keys[0], s.OutOfStep))
	}
	if !slices.Equal(got, []string{"A-1:false", "A-2:true", "A-3:false"}) {
		t.Fatalf("sets %v", got)
	}
}

// Jira words a link from the side it is read on: the issue holding an
// outwardIssue uses the type's outward words, the other end the inward ones.
func TestRunReadsDatesCategoryAndLinkWording(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":1,"issues":[{"key":"B-1","fields":{
			"status":{"name":"Corrected","statusCategory":{"key":"done"}},
			"created":"2026-01-02T10:00:00.000+0700","resolutiondate":null,
			"issuelinks":[
				{"type":{"name":"Cloners","outward":"clones","inward":"is cloned by"},"outwardIssue":{"key":"A-1"}},
				{"type":{"name":"Cloners","outward":"clones","inward":"is cloned by"},"inwardIssue":{"key":"C-1"}}]}}]}`))
	}))
	defer srv.Close()
	res, err := Run(context.Background(), corejira.NewClient(srv.URL, "t"), "x")
	if err != nil {
		t.Fatal(err)
	}
	got := res.Issues[0]
	want := []Link{{Key: "A-1", Type: "Cloners", Words: "clones"}, {Key: "C-1", Type: "Cloners", Words: "is cloned by"}}
	if got.StatusCategory != "done" || got.Created != "2026-01-02T10:00:00.000+0700" || got.Resolved != "" || !slices.Equal(got.Links, want) {
		t.Fatalf("got %+v", got)
	}
}
