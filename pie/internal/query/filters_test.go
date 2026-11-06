package query

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	corejira "agile-suite/core/jira"
)

func filterJira(t *testing.T) *corejira.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/filter/favourite":
			_, _ = w.Write([]byte(`[{"id":"10","name":"Weekly","jql":"project = A"}]`))
		case "/rest/api/2/filter/20":
			_, _ = w.Write([]byte(`{"id":"20","name":"Not starred","jql":"project = B"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["The selected filter is not available to you."]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return corejira.NewClient(srv.URL, "t")
}

func TestStarredListsTheAccountsFilters(t *testing.T) {
	got, err := Starred(context.Background(), filterJira(t))
	if err != nil || len(got) != 1 || got[0].ID != "10" || got[0].JQL != "project = A" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestFilterReachesOneNobodyStarred(t *testing.T) {
	got, err := Filter(context.Background(), filterJira(t), " 20 ")
	if err != nil || got.Name != "Not starred" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestFilterSaysWhyAnIdIsRefused(t *testing.T) {
	for id, want := range map[string]string{
		"30":  "Jira has no filter 30 that this account can see.",
		"abc": "A filter id is a number, as in the filter's Jira address.",
		"":    "A filter id is a number, as in the filter's Jira address.",
	} {
		if _, err := Filter(context.Background(), filterJira(t), id); err == nil || err.Error() != want {
			t.Errorf("Filter(%q) = %v, want %q", id, err, want)
		}
	}
}
