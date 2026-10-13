package syncer_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

// TestCommitSendsCreateScreenFields proves the required create-screen values a
// dialog collects survive the journal and reach Jira in the create POST, for a
// new Test and a new Precondition alike.
func TestCommitSendsCreateScreenFields(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	repo := testrepo.NewRepository(st)

	team := map[string]any{"customfield_20000": map[string]any{"id": "3"}}
	if _, err := repo.CreateTest("p1", testrepo.TestDraft{Summary: "Login", Fields: team}); err != nil {
		t.Fatalf("CreateTest: %v", err)
	}
	if _, err := repo.CreatePrecondition("p1", "QA", "Net up", "Manual", "", team); err != nil {
		t.Fatalf("CreatePrecondition: %v", err)
	}

	var (
		mu    sync.Mutex
		posts = map[string]map[string]any{} // summary -> posted fields
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/2/issuetype":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "5", "name": "Test"},
				{"id": "77", "name": "Pre-Condition"},
			})
		case r.URL.Path == "/rest/api/2/field":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issue":
			raw, _ := io.ReadAll(r.Body)
			var body struct {
				Fields map[string]any `json:"fields"`
			}
			_ = json.Unmarshal(raw, &body)
			mu.Lock()
			posts[body.Fields["summary"].(string)] = body.Fields
			mu.Unlock()
			key := "QA-900"
			if body.Fields["summary"] == "Net up" {
				key = "QA-901"
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"key": key})
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{"updated": ""}})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	engine := syncer.New(xray.New(jira.NewClient(srv.URL, "t")), repo)
	result, err := engine.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("unexpected failures: %+v", result.Failed)
	}
	for _, summary := range []string{"Login", "Net up"} {
		f, ok := posts[summary]
		if !ok {
			t.Fatalf("no create POST for %q", summary)
		}
		got, _ := f["customfield_20000"].(map[string]any)
		if got["id"] != "3" {
			t.Errorf("%q posted customfield_20000 = %v, want {id: 3}", summary, f["customfield_20000"])
		}
	}
}
