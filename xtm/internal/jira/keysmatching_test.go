package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// KeysMatching asks Jira which of the given keys also match the analyst's
// JQL, so the member table can keep only those (#446).
func TestKeysMatchingAsksForTheKeysAndTheJQLTogether(t *testing.T) {
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jql := r.URL.Query().Get("jql")
		sent = append(sent, jql)
		if strings.Contains(jql, "bad") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages":["Field 'bad' does not exist."]}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total": 1, "issues": []map[string]any{{"key": "QA-2"}}})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "t")

	got, err := c.KeysMatching(context.Background(), []string{"QA-1", "QA-2"}, "labels = smoke")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"QA-2"}) {
		t.Fatalf("keys = %v", got)
	}
	if len(sent) != 1 || sent[0] != `key in ("QA-1", "QA-2") AND (labels = smoke) ORDER BY key ASC` {
		t.Fatalf("jql sent = %q", sent)
	}

	if _, err := c.KeysMatching(context.Background(), []string{"QA-1"}, "bad = 1"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("a refused JQL gave %v", err)
	}
}

func TestKeysMatchingRefusesTheDemoProfile(t *testing.T) {
	if _, err := NewClient("demo", "t").KeysMatching(context.Background(), []string{"DEMO-1"}, "labels = x"); err == nil {
		t.Fatal("the demo profile answered a JQL it cannot run")
	}
}
