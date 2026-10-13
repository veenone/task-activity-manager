package syncer_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

// TestCommitPostsTransitionFieldsWithTheTransition proves the field values
// queued with a transition reach Jira in the same POST .../transitions
// request, where Jira takes them, rather than as a separate edit.
func TestCommitPostsTransitionFieldsWithTheTransition(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	repo := testrepo.NewRepository(st)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", Summary: "a", Status: "Open"}}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.TransitionTestWithFields("p1", "QA-1", "Done",
		map[string]any{"resolution": map[string]any{"id": "1"}}); err != nil {
		t.Fatalf("transition: %v", err)
	}

	var posts []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/2/issue/QA-1/transitions" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []any{
				map[string]any{"id": "31", "name": "Close", "to": map[string]any{"name": "Done"}},
			}})
		case r.URL.Path == "/rest/api/2/issue/QA-1/transitions" && r.Method == http.MethodPost:
			raw, _ := io.ReadAll(r.Body)
			var b map[string]any
			_ = json.Unmarshal(raw, &b)
			posts = append(posts, b)
			w.WriteHeader(http.StatusNoContent)
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
	if len(posts) != 1 {
		t.Fatalf("transition POSTs %d, want 1", len(posts))
	}
	tr, _ := posts[0]["transition"].(map[string]any)
	fields, _ := posts[0]["fields"].(map[string]any)
	res, _ := fields["resolution"].(map[string]any)
	if tr["id"] != "31" || res["id"] != "1" {
		t.Fatalf("posted %v", posts[0])
	}
}
