package main

import (
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// newTestAppWithDemoProfile builds an App on a demo Xray profile, so the whole
// dataset path runs without a network.
func newTestAppWithDemoProfile(t *testing.T) (*App, string) {
	t.Helper()
	a := newTestApp(t)
	p, err := a.CreateProfile("Demo", "demo", "QA", "", "", "", "", "tok", "", false, "xray")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return a, p.ID
}

// Saving a dataset is local: the table shows immediately, one pending change
// is queued, and nothing is sent until Commit (#277 W1).
func TestSaveTestDatasetQueuesTheTableLocally(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)
	want := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}, {"bob"}},
	}

	if err := a.SaveTestDataset(profileID, "QA-2", want); err != nil {
		t.Fatalf("save dataset: %v", err)
	}

	got, err := a.GetTestDataset(profileID, "QA-2", false)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(got.Parameters) != 1 || got.Parameters[0].Name != "USERNAME" || len(got.Rows) != 2 {
		t.Errorf("dataset = %+v, want the table just saved", got)
	}
	pending, err := a.repo.ListPendingChanges(profileID)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	queued := 0
	for _, c := range pending {
		if c.EntityType == "test_dataset" && c.EntityKey == "QA-2" {
			queued++
		}
	}
	if queued != 1 {
		t.Errorf("%d queued dataset changes, want 1 (pending=%+v)", queued, pending)
	}
}

// The limits are enforced before anything is stored, so the editor can show
// the reason and the user still has their table in front of them.
func TestSaveTestDatasetRefusesAnInvalidTable(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	err := a.SaveTestDataset(profileID, "QA-2", testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}, {Name: "USERNAME"}},
	})
	if err == nil {
		t.Fatal("a dataset with two parameters of the same name was accepted")
	}
	if _, found, _ := a.repo.GetTestDataset(profileID, "QA-2", ""); found {
		t.Error("the rejected table was cached anyway")
	}
}

// Renaming a parameter is offered with the rewrite its references need, and
// the rewrite is journalled like any other edit (#277 W2).
func TestRenameDatasetPlaceholderQueuesTheRewrite(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)
	if err := a.repo.UpsertTests(profileID, []testrepo.TestCase{{
		Key: "QA-7", ID: "7", Summary: "s", Status: "Open",
		Description: "Sign in as ${USER}", Updated: "2026-01-01T00:00:00Z",
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	changed, err := a.RenameDatasetPlaceholder(profileID, "QA-7", "USER", "ACCOUNT")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if changed != 1 {
		t.Fatalf("rewrote %d fields, want 1", changed)
	}
	tc, err := a.repo.GetTest(profileID, "QA-7")
	if err != nil {
		t.Fatalf("read test: %v", err)
	}
	if tc.Description != "Sign in as ${ACCOUNT}" {
		t.Errorf("description = %q, want the rewritten one", tc.Description)
	}
}

// A spreadsheet round trip is the point of the CSV: out, edited, back in.
// What comes back is the file's table, with the identity of the parameters it
// still has recovered from the table it replaces (#277 W2).
func TestImportDatasetCSVReadsTheFileAndKeepsWhatIsKnown(t *testing.T) {
	a, _ := newTestAppWithDemoProfile(t)
	current := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{
			{Name: "TIER", ID: 2741, Type: "LIST", Options: []string{"gold", "silver"}, ListID: 162},
		},
		Rows: [][]string{{"gold"}},
	}

	got, err := a.ImportDatasetCSV("TIER,ROUND*\nsilver,1\ngold,2\n", current)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(got.Parameters) != 2 || got.Parameters[1].Name != "ROUND" {
		t.Fatalf("parameters = %+v, want TIER and ROUND", got.Parameters)
	}
	if !got.Parameters[1].Combinatorial {
		t.Error("the asterisk on ROUND did not come through as combinatorial")
	}
	if got.Parameters[0].ID != 2741 || got.Parameters[0].ListID != 162 {
		t.Errorf("TIER = %+v, want the id and list it already had", got.Parameters[0])
	}
	if len(got.Rows) != 2 || got.Rows[1][1] != "2" {
		t.Errorf("rows = %q, want both rows of the file", got.Rows)
	}
}

// A file that is not a dataset should say so rather than quietly emptying the
// editor, and one Xray would refuse should be refused here too.
func TestImportDatasetCSVRefusesWhatItCannotUse(t *testing.T) {
	a, _ := newTestAppWithDemoProfile(t)

	if _, err := a.ImportDatasetCSV("", testrepo.Dataset{}); err == nil {
		t.Error("an empty file was accepted")
	}
	if _, err := a.ImportDatasetCSV("A,A\n1,2\n", testrepo.Dataset{}); err == nil {
		t.Error("a file with two columns of the same name was accepted")
	}
}

// An execution's override is cached under its own key, so reopening a member
// of a container costs nothing and the panel can say which level it is
// showing without asking again (#277 W3).
func TestGetContainerDatasetPrefersTheCache(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)
	override := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "TIER"}},
		Rows:       [][]string{{"run-only"}},
	}
	if err := a.repo.SaveTestDataset(profileID, "QA-1", "EXEC-1", override); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := a.GetContainerDataset(profileID, "EXEC-1", "QA-1", false)
	if err != nil {
		t.Fatalf("container dataset: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "run-only" {
		t.Errorf("dataset = %+v, want the cached override", got)
	}
	// The test's own dataset is a different row and is not disturbed by it.
	own, _, err := a.repo.GetTestDataset(profileID, "QA-1", "")
	if err != nil {
		t.Fatalf("test level: %v", err)
	}
	if len(own.Parameters) != 0 {
		t.Errorf("test-level dataset = %+v, want it untouched", own)
	}
}

// Iterations belong to a run, so there is nothing to ask for without an
// execution, and nothing for a test that exists only here (#267 Phase 4).
func TestGetRunIterationsAsksNothingWithoutARun(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	for _, tc := range []struct{ container, test string }{
		{"", "QA-1"},
		{"EXEC-1", "new-test-1"},
		// A demo profile has runs but no parameterized ones.
		{"EXEC-1", "QA-1"},
	} {
		iters, err := a.GetRunIterations(profileID, tc.container, tc.test)
		if err != nil {
			t.Fatalf("container=%q test=%q: %v", tc.container, tc.test, err)
		}
		if len(iters) != 0 {
			t.Errorf("container=%q test=%q gave %+v, want none", tc.container, tc.test, iters)
		}
	}
}

// Nothing to ask about: no container selected, or a test that exists only
// here. Both answer empty without reaching for a backend.
func TestGetContainerDatasetAsksNothingWithoutAContainer(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	for _, tc := range []struct{ container, test string }{
		{"", "QA-1"},
		{"EXEC-1", "new-test-1"},
	} {
		ds, err := a.GetContainerDataset(profileID, tc.container, tc.test, false)
		if err != nil {
			t.Fatalf("container=%q test=%q: %v", tc.container, tc.test, err)
		}
		if len(ds.Parameters) != 0 {
			t.Errorf("container=%q test=%q gave %+v, want nothing", tc.container, tc.test, ds)
		}
	}
}

// A parameterized test's dataset reaches the caller with its parameters and
// rows intact.
func TestGetTestDatasetReadsAParameterizedTest(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	ds, err := a.GetTestDataset(profileID, "QA-1", false)
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(ds.Parameters) == 0 || len(ds.Rows) == 0 {
		t.Fatalf("QA-1 should carry a demo dataset, got %+v", ds)
	}
}

// The fix this method exists for. A test with NO dataset must be recorded as
// fetched, so the next open is answered from the store instead of Jira. Most
// tests are not parameterized, so without this the detail panel pays for a
// round trip on nearly every test anyone clicks.
func TestGetTestDatasetRecordsThatATestHasNoDataset(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	if _, ok, err := a.repo.GetTestDataset(profileID, "QA-2", ""); err != nil || ok {
		t.Fatalf("precondition: QA-2 should not be cached yet (found=%v, err=%v)", ok, err)
	}

	ds, err := a.GetTestDataset(profileID, "QA-2", false)
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(ds.Parameters) != 0 {
		t.Fatalf("QA-2 should have no dataset, got %+v", ds)
	}

	_, ok, err := a.repo.GetTestDataset(profileID, "QA-2", "")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !ok {
		t.Fatal("a test with no dataset must still be recorded as fetched, or every open refetches")
	}
}

// A cached dataset is returned without consulting the backend. Seeding a value
// the demo backend would never produce proves the read came from the store.
func TestGetTestDatasetPrefersTheCache(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	seeded := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "FROM_CACHE"}},
		Rows:       [][]string{{"yes"}},
	}
	if err := a.repo.SaveTestDataset(profileID, "QA-1", "", seeded); err != nil {
		t.Fatalf("seed: %v", err)
	}

	ds, err := a.GetTestDataset(profileID, "QA-1", false)
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(ds.Parameters) != 1 || ds.Parameters[0].Name != "FROM_CACHE" {
		t.Fatalf("dataset = %+v, want the cached FROM_CACHE parameter", ds)
	}
}

// Refresh goes past the cache and overwrites it, so a dataset changed in Jira
// can be pulled without clearing anything by hand.
func TestGetTestDatasetRefreshBypassesTheCache(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	stale := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "STALE"}},
		Rows:       [][]string{{"old"}},
	}
	if err := a.repo.SaveTestDataset(profileID, "QA-1", "", stale); err != nil {
		t.Fatalf("seed: %v", err)
	}

	ds, err := a.GetTestDataset(profileID, "QA-1", true)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	for _, p := range ds.Parameters {
		if p.Name == "STALE" {
			t.Fatalf("refresh returned the cached dataset: %+v", ds)
		}
	}

	cached, _, err := a.repo.GetTestDataset(profileID, "QA-1", "")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	for _, p := range cached.Parameters {
		if p.Name == "STALE" {
			t.Fatal("refresh did not overwrite the cached dataset")
		}
	}
}

// An uncommitted local test has a placeholder key Jira would reject, so it is
// answered from the store alone, the same rule GetTestSteps follows.
func TestGetTestDatasetNeverCallsJiraForALocalTest(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	ds, err := a.GetTestDataset(profileID, "NEW-1", false)
	if err != nil {
		t.Fatalf("local test: %v", err)
	}
	if len(ds.Parameters) != 0 {
		t.Fatalf("a local test has no dataset, got %+v", ds)
	}
}
