package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corejira "agile-suite/core/jira"
)

// jiraWith serves /rest/api/2/search from total issues, at most serverCap per
// page however many were asked for, because Jira imposes its own limit.
func jiraWith(t *testing.T, total, serverCap int, seen *[]string) *corejira.Client {
	t.Helper()
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if seen != nil {
			mu.Lock()
			*seen = append(*seen, q.Get("jql"))
			mu.Unlock()
		}
		start, _ := strconv.Atoi(q.Get("startAt"))
		var issues []map[string]any
		for i := start; i < total && i < start+serverCap; i++ {
			issues = append(issues, map[string]any{"key": fmt.Sprintf("A-%d", i), "fields": map[string]any{}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total": total, "issues": issues})
	}))
	t.Cleanup(srv.Close)
	return corejira.NewClient(srv.URL, "token")
}

func TestRunFetchesEveryPageWhenTheServerCapsThem(t *testing.T) {
	var seen []string
	res, err := Run(context.Background(), jiraWith(t, 250, 40, &seen), "category = X")
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 250 || len(res.Issues) != 250 || res.Truncated {
		t.Fatalf("total %d, rows %d, truncated %v", res.Total, len(res.Issues), res.Truncated)
	}
	if res.Issues[249].Key != "A-249" {
		t.Fatalf("last row is %s", res.Issues[249].Key)
	}
	if seen[0] != "category = X" {
		t.Fatalf("sent %q", seen[0])
	}
}

// inFlight counts the requests a server is answering at once and keeps the
// highest count it saw. Each request waits a little, so overlapping ones do.
type inFlight struct {
	now, most atomic.Int32
}

func (f *inFlight) wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := f.now.Add(1)
		for m := f.most.Load(); n > m && !f.most.CompareAndSwap(m, n); m = f.most.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		h(w, r)
		f.now.Add(-1)
	}
}

func TestRunFetchesTheRemainingPagesInParallelAndInOrder(t *testing.T) {
	var f inFlight
	srv := httptest.NewServer(f.wrap(func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
		var issues []map[string]any
		for i := start; i < 950 && i < start+100; i++ {
			issues = append(issues, map[string]any{"key": fmt.Sprintf("A-%d", i), "fields": map[string]any{}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total": 950, "issues": issues})
	}))
	t.Cleanup(srv.Close)
	res, err := Run(context.Background(), corejira.NewClient(srv.URL, "token"), "category = X")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Issues) != 950 || res.Truncated {
		t.Fatalf("rows %d, truncated %v", len(res.Issues), res.Truncated)
	}
	for i, is := range res.Issues {
		if is.Key != fmt.Sprintf("A-%d", i) {
			t.Fatalf("row %d is %s", i, is.Key)
		}
	}
	if m := f.most.Load(); m < 2 || m > workers {
		t.Fatalf("at most %d requests at once, want 2 to %d", m, workers)
	}
}

func TestRunMapsTheFieldsAnAnalystReads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":1,"issues":[{"key":"A-1","fields":{
			"summary":"Fails","project":{"key":"A","name":"Alpha Platform"},"issuetype":{"name":"Bug"},
			"status":{"name":"Open"},"priority":{"name":"High"},"updated":"2026-11-05T10:00:00.000+0700"}}]}`))
	}))
	defer srv.Close()
	res, err := Run(context.Background(), corejira.NewClient(srv.URL, "t"), "x")
	if err != nil {
		t.Fatal(err)
	}
	want := Issue{Key: "A-1", Project: "A", ProjectName: "Alpha Platform", Type: "Bug", Summary: "Fails", Status: "Open", Priority: "High",
		Updated: "2026-11-05T10:00:00.000+0700", Matched: true}
	if !reflect.DeepEqual(res.Issues[0], want) {
		t.Fatalf("got %+v", res.Issues[0])
	}
}

// Priority is optional on an instance, and a field of the wrong shape is
// routine at a trust boundary (I1): the row still has to come through.
func TestRunKeepsARowWhoseFieldsAreMissingOrMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":1,"issues":[{"key":"A-1","fields":{"priority":null,"status":"odd","summary":7}}]}`))
	}))
	defer srv.Close()
	res, err := Run(context.Background(), corejira.NewClient(srv.URL, "t"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Issues[0], Issue{Key: "A-1", Matched: true}) {
		t.Fatalf("got %+v", res.Issues[0])
	}
}

func TestRunStopsAtTheRowLimitAndSaysSo(t *testing.T) {
	res, err := Run(context.Background(), jiraWith(t, MaxRows+500, 100, nil), "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Issues) != MaxRows || res.Total != MaxRows+500 || !res.Truncated {
		t.Fatalf("rows %d, total %d, truncated %v", len(res.Issues), res.Total, res.Truncated)
	}
}

func TestExploreAsksBeforeALargeFetch(t *testing.T) {
	var seen []string
	res, err := Explore(context.Background(), jiraWith(t, AskAbove+1, 100, &seen), "x", false, Fields{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedsConfirm || res.Total != AskAbove+1 || len(res.Issues) != 0 || len(seen) != 1 {
		t.Fatalf("confirm %v, total %d, rows %d, searches %d", res.NeedsConfirm, res.Total, len(res.Issues), len(seen))
	}

	res, err = Explore(context.Background(), jiraWith(t, AskAbove+1, 100, nil), "x", true, Fields{})
	if err != nil {
		t.Fatal(err)
	}
	if res.NeedsConfirm || len(res.Issues) != AskAbove+1 {
		t.Fatalf("after confirming: confirm %v, rows %d", res.NeedsConfirm, len(res.Issues))
	}
}

func TestExploreDoesNotAskAtTheThreshold(t *testing.T) {
	res, err := Explore(context.Background(), jiraWith(t, AskAbove, 100, nil), "x", false, Fields{})
	if err != nil {
		t.Fatal(err)
	}
	if res.NeedsConfirm || len(res.Issues) != AskAbove {
		t.Fatalf("confirm %v, rows %d", res.NeedsConfirm, len(res.Issues))
	}
}

// A server that claims more than it will serve must end the walk, not spin.
func TestRunEndsWhenAPageComesBackEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":10,"issues":[]}`))
	}))
	defer srv.Close()
	res, err := Run(context.Background(), corejira.NewClient(srv.URL, "t"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || len(res.Issues) != 0 {
		t.Fatalf("truncated %v, rows %d", res.Truncated, len(res.Issues))
	}
}

// The case the team's own query hits: a saved filter this account cannot see
// fails the whole search, and only Jira's sentence says which one.
func TestRunShowsJirasOwnMessageForARefusedQuery(t *testing.T) {
	const msg = "A value with ID '12345' does not exist for the field 'filter'."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"errorMessages":[%q],"errors":{}}`, msg)
	}))
	defer srv.Close()
	_, err := Run(context.Background(), corejira.NewClient(srv.URL, "t"), "filter = 12345")
	if err == nil || err.Error() != msg {
		t.Fatalf("got %v", err)
	}
}

func TestRunExplainsARejectedToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := Run(context.Background(), corejira.NewClient(srv.URL, "t"), "x")
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("got %v", err)
	}
}

func TestRunRefusesEmptyJQLWithoutAskingJira(t *testing.T) {
	var seen []string
	if _, err := Run(context.Background(), jiraWith(t, 1, 1, &seen), "  \n"); err == nil {
		t.Fatal("no error")
	}
	if len(seen) != 0 {
		t.Fatalf("asked Jira %d times", len(seen))
	}
}

// A product base field may be text, one option, several options or empty.
// Each option is a base of its own, so an issue can count towards two.
func TestExploreReadsTheProductBaseField(t *testing.T) {
	var fields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"total":4,"issues":[
			{"key":"A-1","fields":{"customfield_1":{"value":"One"}}},
			{"key":"A-2","fields":{"customfield_1":[{"value":"One"},{"value":" Two "}]}},
			{"key":"A-3","fields":{"customfield_1":"Plain"}},
			{"key":"A-4","fields":{"customfield_1":null}}]}`))
	}))
	defer srv.Close()
	res, err := Explore(context.Background(), corejira.NewClient(srv.URL, "t"), "x", true, Fields{Base: "customfield_1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(fields, ",customfield_1") {
		t.Fatalf("asked for fields %q", fields)
	}
	want := [][]string{{"One"}, {"One", "Two"}, {"Plain"}, nil}
	for i, is := range res.Issues {
		if !reflect.DeepEqual(is.Bases, want[i]) {
			t.Errorf("%s bases %q, want %q", is.Key, is.Bases, want[i])
		}
	}
}

// Client visibility is an option and client impact free text; both are read
// as text, an option giving its value.
func TestExploreReadsTheClientFields(t *testing.T) {
	var fields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"total":2,"issues":[
			{"key":"A-1","fields":{"customfield_2":{"value":"Visible"},"customfield_3":"Seen at two sites."}},
			{"key":"A-2","fields":{"customfield_2":null,"customfield_3":null}}]}`))
	}))
	defer srv.Close()
	res, err := Explore(context.Background(), corejira.NewClient(srv.URL, "t"), "x", true, Fields{Visibility: "customfield_2", Impact: "customfield_3"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(fields, ",customfield_2,customfield_3") {
		t.Fatalf("asked for fields %q", fields)
	}
	if got := res.Issues[0]; got.ClientVisibility != "Visible" || got.ClientImpact != "Seen at two sites." {
		t.Fatalf("A-1 = %+v", got)
	}
	if got := res.Issues[1]; got.ClientVisibility != "" || got.ClientImpact != "" {
		t.Fatalf("A-2 = %+v", got)
	}
}
