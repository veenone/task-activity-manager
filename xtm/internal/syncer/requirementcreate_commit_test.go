package syncer_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

// A test linked to a requirement that was created in the same commit has to be
// linked to the key Jira assigned. The link used to go out first, carrying the
// "NEW-REQ-1" placeholder (#482).
func TestCommitLinksTestToRequirementCreatedInSameCommit(t *testing.T) {
	var linked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/2/issue" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"key": "PRD-99"})
		case r.URL.Path == "/rest/api/2/issueLink" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var link struct {
				OutwardIssue struct {
					Key string `json:"key"`
				} `json:"outwardIssue"`
			}
			_ = json.Unmarshal(body, &link)
			linked = append(linked, link.OutwardIssue.Key)
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/rest/api/2/issueLinkType":
			_ = json.NewEncoder(w).Encode(map[string]any{"issueLinkTypes": []map[string]string{
				{"id": "1", "name": "Tests", "inward": "tested by", "outward": "tests"},
			}})
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	repo := testrepo.NewRepository(st)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", ID: "1", Summary: "Login"}}); err != nil {
		t.Fatalf("seed test: %v", err)
	}
	key, err := repo.CreateRequirement("p1", "PRD", "Story", "Lockout policy", "", "", "", "", nil)
	if err != nil {
		t.Fatalf("create requirement: %v", err)
	}
	if err := repo.SetTestRequirements("p1", "QA-1", []string{key}); err != nil {
		t.Fatalf("link test: %v", err)
	}

	res, err := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo).
		CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("commit failed: %+v", res.Failed)
	}
	if len(linked) != 1 || linked[0] != "PRD-99" {
		t.Errorf("requirement keys linked in Jira = %v, want [PRD-99]", linked)
	}
	left, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("pending after commit = %+v, want none", left)
	}
}
