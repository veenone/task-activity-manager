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

// unknownCreateJira is a fake Jira for #607. Each create POST is answered by
// the next entry of answers: "drop" hangs up without a response after
// creating the issue, "lost" hangs up without creating anything, "400"
// refuses it, and anything else (or running out) creates it and answers.
// search lists the issues it returns to the duplicate check, on top of every
// issue a "drop" created.
type unknownCreateJira struct {
	mu       sync.Mutex
	answers  []string
	search   []map[string]string
	creates  int
	steps    int
	members  []string
	searches []string
}

func (f *unknownCreateJira) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.URL.Path == "/rest/api/2/issuetype":
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "5", "name": "Test"}})
	case r.URL.Path == "/rest/api/2/issue" && r.Method == http.MethodPost:
		f.creates++
		var body struct {
			Fields struct {
				Summary string `json:"summary"`
			} `json:"fields"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		answer := ""
		if len(f.answers) > 0 {
			answer, f.answers = f.answers[0], f.answers[1:]
		}
		switch answer {
		case "drop", "lost":
			if answer == "drop" {
				f.search = append(f.search, map[string]string{"key": "QA-900", "summary": body.Fields.Summary})
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		case "400":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages":["Summary is required"]}`))
		default:
			_ = json.NewEncoder(w).Encode(map[string]string{"key": "QA-901"})
		}
	case r.URL.Path == "/rest/api/2/search":
		f.searches = append(f.searches, r.URL.Query().Get("jql"))
		issues := make([]map[string]any, 0, len(f.search))
		for _, s := range f.search {
			issues = append(issues, map[string]any{"key": s["key"], "fields": map[string]any{"summary": s["summary"]}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total": len(issues), "issues": issues})
	case strings.Contains(r.URL.Path, "/step"):
		f.steps++
		_ = json.NewEncoder(w).Encode(map[string]any{"id": f.steps})
	case strings.HasPrefix(r.URL.Path, "/rest/raven/2.0/api/testset/"):
		f.members = append(f.members, r.URL.Path)
		_, _ = w.Write([]byte(`{}`))
	default:
		_, _ = w.Write([]byte(`{}`))
	}
}

func newUnknownCreateRig(t *testing.T, f *unknownCreateJira) (*syncer.Engine, *testrepo.Repository, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	repo := testrepo.NewRepository(st)
	tempKey, err := repo.CreateTest("p1", testrepo.TestDraft{
		Summary: "Login works",
		Steps:   []testrepo.StepDraft{{Action: "Open the login page"}},
	})
	if err != nil {
		t.Fatalf("CreateTest: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return syncer.New(xray.New(jira.NewClient(srv.URL, "t")), repo), repo, tempKey
}

func commitOnce(t *testing.T, e *syncer.Engine) syncer.CommitResult {
	t.Helper()
	res, err := e.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	return res
}

func pendingCreate(t *testing.T, repo *testrepo.Repository) *testrepo.PendingChange {
	t.Helper()
	rows, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("ListPendingChanges: %v", err)
	}
	for i := range rows {
		if rows[i].EntityType == "test_create" {
			return &rows[i]
		}
	}
	return nil
}

// A create Jira carried out but never answered is found on the next commit
// and adopted: no second create, and the steps the first attempt never got
// to are created on the adopted key.
func TestCommitAdoptsATestATimedOutCreateMade(t *testing.T) {
	f := &unknownCreateJira{answers: []string{"drop"}}
	engine, repo, tempKey := newUnknownCreateRig(t, f)

	first := commitOnce(t, engine)
	if len(first.Failed) != 1 || !strings.Contains(first.Failed[0].Error, "next commit checks") {
		t.Fatalf("first commit failures = %+v, want one that says the next commit checks", first.Failed)
	}
	row := pendingCreate(t, repo)
	if row == nil || !strings.Contains(row.AfterVal, `"createAttempt"`) {
		t.Fatalf("pending create after a dropped answer = %+v, want a recorded attempt", row)
	}

	second := commitOnce(t, engine)
	if f.creates != 1 {
		t.Fatalf("create POSTs = %d, want 1 (the second commit must adopt, not create)", f.creates)
	}
	if len(second.Failed) != 0 {
		t.Fatalf("second commit failures = %+v", second.Failed)
	}
	if len(second.Created) != 1 || second.Created[0].TempKey != tempKey || second.Created[0].Key != "QA-900" {
		t.Fatalf("created = %+v, want %s -> QA-900", second.Created, tempKey)
	}
	if f.steps != 1 {
		t.Fatalf("step POSTs = %d, want 1 on the adopted Test", f.steps)
	}
	if pendingCreate(t, repo) != nil {
		t.Fatal("the create row is still pending after adopting QA-900")
	}
	jql := f.searches[0]
	for _, want := range []string{`project = "QA"`, "issuetype = 5", "reporter = currentUser()", `created >= "-`} {
		if !strings.Contains(jql, want) {
			t.Errorf("duplicate check JQL %q lacks %q", jql, want)
		}
	}
}

// A timed-out create that made nothing is created once on the next commit.
// An issue whose summary only resembles the draft's is not taken for it.
func TestCommitRecreatesWhenATimedOutCreateMadeNothing(t *testing.T) {
	f := &unknownCreateJira{
		answers: []string{"lost"},
		search:  []map[string]string{{"key": "QA-7", "summary": "Login works again"}},
	}
	engine, repo, _ := newUnknownCreateRig(t, f)

	commitOnce(t, engine)
	second := commitOnce(t, engine)
	if f.creates != 2 {
		t.Fatalf("create POSTs = %d, want 2 (one lost, one real)", f.creates)
	}
	if len(second.Failed) != 0 || len(second.Created) != 1 || second.Created[0].Key != "QA-901" {
		t.Fatalf("second commit = %+v, want QA-901 created", second)
	}
	if pendingCreate(t, repo) != nil {
		t.Fatal("the create row is still pending after it was created")
	}
}

// Two issues that could each be the timed-out create: nothing is created and
// the row fails naming both, so the user decides.
func TestCommitDoesNotGuessBetweenTwoPossibleCreates(t *testing.T) {
	f := &unknownCreateJira{
		answers: []string{"drop"},
		search:  []map[string]string{{"key": "QA-899", "summary": "Login works"}},
	}
	engine, repo, _ := newUnknownCreateRig(t, f)

	commitOnce(t, engine)
	second := commitOnce(t, engine)
	if f.creates != 1 {
		t.Fatalf("create POSTs = %d, want 1", f.creates)
	}
	if len(second.Failed) != 1 || !strings.Contains(second.Failed[0].Error, "QA-899") || !strings.Contains(second.Failed[0].Error, "QA-900") {
		t.Fatalf("second commit failures = %+v, want one naming QA-899 and QA-900", second.Failed)
	}
	if pendingCreate(t, repo) == nil {
		t.Fatal("the create row was cleared although nothing was adopted")
	}
}

// A create Jira refused with a status is a known failure: no attempt is
// recorded, and the retry creates without searching first.
func TestCommitDoesNotMarkARefusedCreateUnknown(t *testing.T) {
	f := &unknownCreateJira{answers: []string{"400"}}
	engine, repo, _ := newUnknownCreateRig(t, f)

	first := commitOnce(t, engine)
	if len(first.Failed) != 1 || strings.Contains(first.Failed[0].Error, "next commit checks") {
		t.Fatalf("first commit failures = %+v, want the plain 400", first.Failed)
	}
	if row := pendingCreate(t, repo); row == nil || strings.Contains(row.AfterVal, "createAttempt") {
		t.Fatalf("pending create after a 400 = %+v, want no recorded attempt", row)
	}
	commitOnce(t, engine)
	if len(f.searches) != 0 || f.creates != 2 {
		t.Fatalf("searches = %v, creates = %d; want no search and a second create", f.searches, f.creates)
	}
}

// The other create paths share the check. A Test Set whose create got no
// answer is adopted on the next commit and still gets its members.
func TestCommitAdoptsATestSetATimedOutCreateMade(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	repo := testrepo.NewRepository(st)
	if _, err := repo.CreateContainerAllocation("p1", "QA", "testset", "Smoke set", []string{"QA-1"}); err != nil {
		t.Fatalf("CreateContainerAllocation: %v", err)
	}
	f := &unknownCreateJira{answers: []string{"drop"}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	engine := syncer.New(xray.New(jira.NewClient(srv.URL, "t")), repo)

	commitOnce(t, engine)
	second := commitOnce(t, engine)
	if f.creates != 1 {
		t.Fatalf("create POSTs = %d, want 1", f.creates)
	}
	if len(second.Failed) != 0 {
		t.Fatalf("second commit failures = %+v", second.Failed)
	}
	if len(f.members) != 1 || !strings.Contains(f.members[0], "/QA-900/") {
		t.Fatalf("member adds = %v, want one on QA-900", f.members)
	}
	if len(f.searches) != 1 || !strings.Contains(f.searches[0], `issuetype = "Test Set"`) {
		t.Fatalf("duplicate check JQL = %v, want the Test Set type", f.searches)
	}
}
