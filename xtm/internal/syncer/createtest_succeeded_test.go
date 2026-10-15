package syncer_test

import (
	"context"
	"encoding/json"
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

// A new Test created into a folder commits as one create and one folder move.
// Both passes succeed for the same Test, and Succeeded is a set of Test keys,
// so the Test is listed once. Listed twice, "Committed N tests" counts it
// twice (#263).
func TestCommitListsACreatedTestWithAFolderOnce(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	repo := testrepo.NewRepository(st)
	if _, err := st.DB().Exec(
		`INSERT INTO test_folder (profile_id, id, parent_id, name, xray_id) VALUES ('p1', '/Commerce/Refund', '/Commerce', 'Refund', '42')`,
	); err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	if _, err := repo.CreateTest("p1", testrepo.TestDraft{Summary: "Refund to the original card", FolderID: "/Commerce/Refund"}); err != nil {
		t.Fatalf("CreateTest: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/2/issuetype":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "5", "name": "Test"}})
		case r.URL.Path == "/rest/api/2/issue":
			_ = json.NewEncoder(w).Encode(map[string]string{"key": "QA-900"})
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
	if len(result.Succeeded) != 1 || result.Succeeded[0] != "QA-900" {
		t.Fatalf("Succeeded = %v, want [QA-900]", result.Succeeded)
	}
}
