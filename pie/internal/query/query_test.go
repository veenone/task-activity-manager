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
	"testing"

	corejira "agile-suite/core/jira"
)

// jiraWith serves /rest/api/2/search from total issues, at most serverCap per
// page however many were asked for, because Jira imposes its own limit.
func jiraWith(t *testing.T, total, serverCap int, seen *[]string) *corejira.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if seen != nil {
			*seen = append(*seen, q.Get("jql"))
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

func TestRunMapsTheFieldsAnAnalystReads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":1,"issues":[{"key":"A-1","fields":{
			"summary":"Fails","project":{"key":"A"},"issuetype":{"name":"Bug"},
			"status":{"name":"Open"},"priority":{"name":"High"},"updated":"2026-11-05T10:00:00.000+0700"}}]}`))
	}))
	defer srv.Close()
	res, err := Run(context.Background(), corejira.NewClient(srv.URL, "t"), "x")
	if err != nil {
		t.Fatal(err)
	}
	want := Issue{Key: "A-1", Project: "A", Type: "Bug", Summary: "Fails", Status: "Open", Priority: "High",
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
	res, err := Explore(context.Background(), jiraWith(t, AskAbove+1, 100, &seen), "x", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedsConfirm || res.Total != AskAbove+1 || len(res.Issues) != 0 || len(seen) != 1 {
		t.Fatalf("confirm %v, total %d, rows %d, searches %d", res.NeedsConfirm, res.Total, len(res.Issues), len(seen))
	}

	res, err = Explore(context.Background(), jiraWith(t, AskAbove+1, 100, nil), "x", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.NeedsConfirm || len(res.Issues) != AskAbove+1 {
		t.Fatalf("after confirming: confirm %v, rows %d", res.NeedsConfirm, len(res.Issues))
	}
}

func TestExploreDoesNotAskAtTheThreshold(t *testing.T) {
	res, err := Explore(context.Background(), jiraWith(t, AskAbove, 100, nil), "x", false, "")
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
	res, err := Explore(context.Background(), corejira.NewClient(srv.URL, "t"), "x", true, "customfield_1")
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
