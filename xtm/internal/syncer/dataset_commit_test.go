package syncer_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

// datasetInstance is a stub Xray: it resolves a key to an issue id, serves the
// dataset export (404 when remoteCSV is empty, as a test with no dataset does)
// and records the write.
type datasetInstance struct {
	remoteCSV string
	exported  int
	wrote     string
	writes    int
}

func (d *datasetInstance) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"4242","key":"QA-1"}`))
		case r.URL.Path == "/rest/raven/2.0/api/dataset/export":
			d.exported++
			if d.remoteCSV == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"Dataset not found"}`))
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(d.remoteCSV))
		case r.URL.Path == "/rest/raven/1.0/dataset":
			d.writes++
			b, _ := io.ReadAll(r.Body)
			d.wrote = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("Dataset Updated with success."))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func datasetRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st)
}

func pendingDatasets(t *testing.T, repo *testrepo.Repository) []testrepo.PendingChange {
	t.Helper()
	all, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	out := []testrepo.PendingChange{}
	for _, c := range all {
		if c.EntityType == "test_dataset" {
			out = append(out, c)
		}
	}
	return out
}

// The queued table reaches Xray and the pending row clears. The test here had
// no dataset, which is the case W1 exists for.
func TestCommitDatasetCreatesTheTableAndClearsThePendingRow(t *testing.T) {
	repo := datasetRepo(t)
	inst := &datasetInstance{}
	srv := inst.server(t)
	defer srv.Close()

	if err := repo.EditTestDataset("p1", "QA-1", testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}, {Name: "REGION"}},
		Rows:       [][]string{{"alice", "eu-west"}},
	}); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}

	res, err := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo).
		CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Failed) != 0 || len(res.Conflicted) != 0 {
		t.Fatalf("failed=%+v conflicted=%+v, want neither", res.Failed, res.Conflicted)
	}
	if len(res.Succeeded) != 1 || res.Succeeded[0] != "QA-1" {
		t.Errorf("succeeded = %v, want [QA-1]", res.Succeeded)
	}
	if inst.writes != 1 {
		t.Fatalf("%d dataset writes, want 1", inst.writes)
	}
	for _, want := range []string{`"name":"USERNAME"`, `"name":"REGION"`, `"value":"eu-west"`} {
		if !strings.Contains(inst.wrote, want) {
			t.Errorf("write body %s is missing %s", inst.wrote, want)
		}
	}
	if left := pendingDatasets(t, repo); len(left) != 0 {
		t.Errorf("%d pending dataset rows after a successful commit, want none", len(left))
	}
}

// A dataset given to a test that does not exist in Jira yet is keyed by the
// placeholder. The commit creates the test first, so the write has to land
// under the key Jira assigned and never under "new-test-N".
func TestCommitDatasetOfANewTestUsesTheRealKey(t *testing.T) {
	repo := datasetRepo(t)
	tempKey, err := repo.CreateTest("p1", testrepo.TestDraft{Summary: "New login test"})
	if err != nil {
		t.Fatalf("CreateTest: %v", err)
	}
	if err := repo.EditTestDataset("p1", tempKey, testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}},
	}); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}

	var seen []string
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path+"?"+r.URL.RawQuery)
		switch {
		case r.URL.Path == "/rest/api/2/issuetype":
			_, _ = w.Write([]byte(`[{"id":"5","name":"Test"}]`))
		case r.URL.Path == "/rest/api/2/issue":
			_, _ = w.Write([]byte(`{"key":"QA-900"}`))
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/"):
			// Serves both the conflict pre-check (fields.updated) and the id
			// the dataset resource is addressed by.
			_, _ = w.Write([]byte(`{"id":"4242","key":"QA-900","fields":{"updated":""}}`))
		case r.URL.Path == "/rest/raven/2.0/api/dataset/export":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Dataset not found"}`))
		case r.URL.Path == "/rest/raven/1.0/dataset":
			writes++
			_, _ = w.Write([]byte("Dataset Updated with success."))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	res, err := syncer.New(xray.New(jira.NewClient(srv.URL, "t")), repo).
		CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Failed) != 0 || len(res.Conflicted) != 0 {
		t.Fatalf("failed=%+v conflicted=%+v, want neither", res.Failed, res.Conflicted)
	}
	if writes != 1 {
		t.Fatalf("%d dataset writes, want 1", writes)
	}
	for _, path := range seen {
		if strings.Contains(path, tempKey) {
			t.Errorf("request %s still names the placeholder key %s", path, tempKey)
		}
	}
	if !slices.Contains(seen, "/rest/raven/2.0/api/dataset/export?testIssueKey=QA-900") {
		t.Errorf("the dataset was never read back under the real key: %v", seen)
	}
}

// Someone else changed the dataset between the local edit and the commit. The
// write must not go, because it replaces the whole table and would take their
// change with it.
func TestCommitDatasetHoldsBackWhenTheRemoteTableMoved(t *testing.T) {
	repo := datasetRepo(t)
	synced := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}},
	}
	if err := repo.SaveTestDataset("p1", "QA-1", "", synced); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.EditTestDataset("p1", "QA-1", testrepo.Dataset{
		Parameters: synced.Parameters,
		Rows:       [][]string{{"alice"}, {"bob"}},
	}); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}

	// Jira now holds a third version: a column the local edit never saw.
	inst := &datasetInstance{remoteCSV: "USERNAME,REGION\nalice,eu-west\n"}
	srv := inst.server(t)
	defer srv.Close()

	res, err := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo).
		CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if inst.writes != 0 {
		t.Fatalf("the dataset was overwritten despite the remote change (%d writes)", inst.writes)
	}
	if len(res.Conflicted) != 1 {
		t.Fatalf("conflicted = %+v, want one conflict", res.Conflicted)
	}
	c := res.Conflicted[0]
	if c.TestKey != "QA-1" || len(c.Fields) != 1 {
		t.Fatalf("conflict = %+v, want QA-1 with one field", c)
	}
	f := c.Fields[0]
	if f.EntityType != "test_dataset" || f.Field != "dataset" {
		t.Errorf("conflict field = %s/%s, want test_dataset/dataset", f.EntityType, f.Field)
	}
	// The three sides are what the resolution dialog shows, so each has to
	// carry a table rather than an empty string.
	for name, side := range map[string]string{"base": f.Base, "remote": f.Remote, "mine": f.Mine} {
		if !strings.Contains(side, `"parameters"`) {
			t.Errorf("%s side = %q, want the table as JSON", name, side)
		}
	}
	if !strings.Contains(f.Remote, "REGION") {
		t.Errorf("remote side = %q, want the column Jira has now", f.Remote)
	}
	if left := pendingDatasets(t, repo); len(left) != 1 {
		t.Errorf("%d pending dataset rows after a conflict, want the edit kept", len(left))
	}
}

// Keeping mine after a conflict has to re-base what the next commit compares
// against, or the same conflict is reported forever.
func TestResolvingADatasetConflictWithMineLetsTheNextCommitThrough(t *testing.T) {
	repo := datasetRepo(t)
	mine := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}, {"bob"}},
	}
	if err := repo.EditTestDataset("p1", "QA-1", mine); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}
	inst := &datasetInstance{remoteCSV: "USERNAME,REGION\nalice,eu-west\n"}
	srv := inst.server(t)
	defer srv.Close()
	eng := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo)

	res, err := eng.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Conflicted) != 1 {
		t.Fatalf("conflicted = %+v, want one conflict", res.Conflicted)
	}
	f := res.Conflicted[0].Fields[0]
	if err := repo.ResolveConflictMerge("p1", "QA-1", res.Conflicted[0].RemoteVersion,
		[]testrepo.ConflictDecision{{
			PendingID:   f.PendingID,
			EntityType:  f.EntityType,
			EntityKey:   f.EntityKey,
			Field:       f.Field,
			Choice:      "mine",
			RemoteValue: f.Remote,
		}}); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	res, err = eng.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("second CommitChanges: %v", err)
	}
	if len(res.Conflicted) != 0 {
		t.Fatalf("still conflicted after keeping mine: %+v", res.Conflicted)
	}
	if inst.writes != 1 {
		t.Fatalf("%d dataset writes after keeping mine, want 1", inst.writes)
	}
	if !strings.Contains(inst.wrote, `"value":"bob"`) {
		t.Errorf("write body %s does not carry my table", inst.wrote)
	}
}

// Taking theirs puts the remote table in the cache and drops the edit, so the
// panel stops showing a change that is not going anywhere.
func TestResolvingADatasetConflictWithTheirsTakesTheRemoteTable(t *testing.T) {
	repo := datasetRepo(t)
	if err := repo.EditTestDataset("p1", "QA-1", testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}, {"bob"}},
	}); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}
	inst := &datasetInstance{remoteCSV: "USERNAME,REGION\nalice,eu-west\n"}
	srv := inst.server(t)
	defer srv.Close()
	eng := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo)

	res, err := eng.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Conflicted) != 1 {
		t.Fatalf("conflicted = %+v, want one conflict", res.Conflicted)
	}
	f := res.Conflicted[0].Fields[0]
	if err := repo.ResolveConflictMerge("p1", "QA-1", "", []testrepo.ConflictDecision{{
		PendingID:   f.PendingID,
		EntityType:  f.EntityType,
		EntityKey:   f.EntityKey,
		Field:       f.Field,
		Choice:      "theirs",
		RemoteValue: f.Remote,
	}}); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if left := pendingDatasets(t, repo); len(left) != 0 {
		t.Errorf("%d pending dataset rows after taking theirs, want none", len(left))
	}
	cached, found, err := repo.GetTestDataset("p1", "QA-1", "")
	if err != nil || !found {
		t.Fatalf("cache: found=%v err=%v", found, err)
	}
	if len(cached.Parameters) != 2 || cached.Parameters[1].Name != "REGION" {
		t.Errorf("cached dataset = %+v, want the remote table", cached)
	}
}
