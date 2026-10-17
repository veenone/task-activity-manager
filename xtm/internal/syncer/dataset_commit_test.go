package syncer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

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
		case r.URL.Path == "/rest/raven/1.0/dataset" && r.Method == http.MethodGet:
			// The new test has no dataset yet, which this route answers with
			// an empty params array rather than a 404.
			_, _ = w.Write([]byte(`{"entityDataset":{"params":[]},` +
				`"resolvedDataset":{"params":[]},"resolutionMessage":""}`))
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
	if !slices.Contains(seen, "/rest/api/2/issue/QA-900?fields=summary") {
		t.Errorf("the dataset was never resolved under the real key: %v", seen)
	}
}

// Demo mode is how XTM is seen without a Jira to point it at, so a dataset
// edited there has to commit like any other change. Every other write already
// treats a demo commit as a no-op success (#295).
func TestCommitDatasetOnADemoProfileClearsThePendingRow(t *testing.T) {
	repo := datasetRepo(t)
	edited := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}, {"bob"}},
	}
	// QA-2 carries no demo dataset, so this is the create case: the table
	// the user builds is the whole change.
	if err := repo.EditTestDataset("p1", "QA-2", edited); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}

	res, err := syncer.New(xray.New(jira.NewClient("demo", "tok")), repo).
		CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Failed) != 0 || len(res.Conflicted) != 0 {
		t.Fatalf("failed=%+v conflicted=%+v, want neither", res.Failed, res.Conflicted)
	}
	if len(res.Succeeded) != 1 || res.Succeeded[0] != "QA-2" {
		t.Errorf("succeeded = %v, want [QA-2]", res.Succeeded)
	}
	if left := pendingDatasets(t, repo); len(left) != 0 {
		t.Errorf("%d pending dataset rows after a demo commit, want none", len(left))
	}
	// What the user saved is what they keep looking at.
	cached, found, err := repo.GetTestDataset("p1", "QA-2", "")
	if err != nil || !found {
		t.Fatalf("cache: found=%v err=%v", found, err)
	}
	if len(cached.Rows) != 2 {
		t.Errorf("cached dataset = %+v, want the table that was saved", cached)
	}
}

// Xray added the dataset API in 5.1. An older instance answers 404 to the
// write, which on its own reads as "the test is gone".
func TestCommitDatasetNamesAnInstanceWithNoDatasetAPI(t *testing.T) {
	repo := datasetRepo(t)
	if err := repo.EditTestDataset("p1", "QA-1", testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}},
	}); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/"):
			_, _ = w.Write([]byte(`{"id":"4242","key":"QA-1"}`))
		default:
			// Jersey's answer for a route the plugin does not publish.
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<status><status-code>404</status-code></status>`))
		}
	}))
	defer srv.Close()

	res, err := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo).
		CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Failed) != 1 {
		t.Fatalf("failed = %+v, want one failure naming the missing API", res.Failed)
	}
	if !strings.Contains(res.Failed[0].Error, "dataset API") {
		t.Errorf("error = %q, want it to say the instance has no dataset API", res.Failed[0].Error)
	}
	if left := pendingDatasets(t, repo); len(left) != 1 {
		t.Errorf("%d pending dataset rows, want the edit kept for a newer instance", len(left))
	}
}

// A list parameter has to reach Jira with everything the read found, or Xray
// rebuilds it as free text and its allowed values are gone (#283).
func TestCommitDatasetKeepsAListParameterAList(t *testing.T) {
	repo := datasetRepo(t)
	synced := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{
			Name: "TIER", ID: 2741, Type: "LIST",
			Options: []string{"gold", "silver"}, ListScope: "PARAMETER_AD_HOC", ListID: 162,
		}},
		Rows: [][]string{{"gold"}},
	}
	if err := repo.SaveTestDataset("p1", "QA-1", "", synced); err != nil {
		t.Fatalf("seed: %v", err)
	}
	edited := testrepo.Dataset{Parameters: synced.Parameters, Rows: [][]string{{"gold"}, {"silver"}}}
	if err := repo.EditTestDataset("p1", "QA-1", edited); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}

	inst := &datasetInstance{columns: []string{
		listColumn(2741, "TIER", 162, []string{"gold", "silver"}, "gold"),
	}}
	srv := inst.server(t)
	defer srv.Close()

	res, err := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo).
		CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Failed) != 0 || len(res.Conflicted) != 0 {
		t.Fatalf("failed=%+v conflicted=%+v, want neither", res.Failed, res.Conflicted)
	}
	if inst.writes != 1 {
		t.Fatalf("%d writes, want 1", inst.writes)
	}
	for _, want := range []string{
		`"id":2741`, `"type":"LIST"`, `"options":["gold","silver"]`,
		`"paramTestListScope":"PARAMETER_AD_HOC"`, `"listId":162`,
	} {
		if !strings.Contains(inst.wrote, want) {
			t.Errorf("write body %s is missing %s", inst.wrote, want)
		}
	}
}

// Xray gives a parameter a new id whenever it is rebuilt, and rebuilds the
// list behind it on every edit through its own UI. None of that is a change
// the user made, so it must neither raise a conflict nor be written back
// stale: ids a write sends come from the table Jira holds now.
func TestCommitDatasetTakesCurrentIdsAndIgnoresThemWhenComparing(t *testing.T) {
	repo := datasetRepo(t)
	synced := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{
			Name: "TIER", ID: 2741, Type: "LIST",
			Options: []string{"gold", "silver"}, ListScope: "PARAMETER_AD_HOC", ListID: 162,
		}},
		Rows: [][]string{{"gold"}},
	}
	if err := repo.SaveTestDataset("p1", "QA-1", "", synced); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.EditTestDataset("p1", "QA-1", testrepo.Dataset{
		Parameters: synced.Parameters,
		Rows:       [][]string{{"silver"}},
	}); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}

	// Same table to look at, rebuilt underneath: new parameter id, new list.
	inst := &datasetInstance{columns: []string{
		listColumn(2999, "TIER", 177, []string{"gold", "silver"}, "gold"),
	}}
	srv := inst.server(t)
	defer srv.Close()

	res, err := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo).
		CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Conflicted) != 0 {
		t.Fatalf("conflicted on bookkeeping alone: %+v", res.Conflicted)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("failed = %+v", res.Failed)
	}
	if !strings.Contains(inst.wrote, `"id":2999`) || !strings.Contains(inst.wrote, `"listId":177`) {
		t.Errorf("write body %s does not carry the ids Jira has now", inst.wrote)
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
	inst := &datasetInstance{columns: []string{
		textColumn(2726, "USERNAME", "alice"),
		textColumn(2727, "REGION", "eu-west"),
	}}
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
