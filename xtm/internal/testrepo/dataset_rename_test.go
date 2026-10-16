package testrepo

import (
	"testing"
)

// seedParameterizedTest stores a test whose steps and definition refer to two
// dataset parameters, so a rename has somewhere to land.
func seedParameterizedTest(t *testing.T, r *Repository) {
	t.Helper()
	if err := r.UpsertTests("p1", []TestCase{{
		Key: "QA-1", ID: "1", Summary: "Log in as ${USER}", Status: "Open",
		Description:       "Sign in with ${USER} in ${REGION}.",
		GenericDefinition: "run --user ${USER}",
		Updated:           "2026-01-01T00:00:00Z",
	}}); err != nil {
		t.Fatalf("seed test: %v", err)
	}
	if err := r.SetTestSteps("p1", "QA-1", []Step{
		{XrayID: "11", Index: 1, Action: "Open the ${USER} page", Data: "${USER}", Expected: "Welcome ${USER}"},
		{XrayID: "12", Index: 2, Action: "Pick ${REGION}", Data: "", Expected: "Shown in ${REGION}"},
	}); err != nil {
		t.Fatalf("seed steps: %v", err)
	}
}

func pendingFields(t *testing.T, r *Repository) map[string]string {
	t.Helper()
	rows, err := r.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	out := map[string]string{}
	for _, c := range rows {
		out[c.EntityType+":"+c.EntityKey+":"+c.Field] = c.AfterVal
	}
	return out
}

// Renaming a dataset parameter leaves every ${placeholder} pointing at a name
// that no longer exists. The rewrite is offered rather than forced, and when
// it runs it goes through the ordinary step and field edits, so it commits and
// discards like anything else (#277 W2).
func TestRenameDatasetPlaceholderRewritesStepsAndDefinition(t *testing.T) {
	r := newTestRepo(t)
	seedParameterizedTest(t, r)

	changed, err := r.RenameDatasetPlaceholder("p1", "QA-1", "USER", "ACCOUNT")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	// Step 11's three fields, the description and the generic definition.
	if changed != 5 {
		t.Errorf("rewrote %d fields, want 5", changed)
	}

	queued := pendingFields(t, r)
	for key, want := range map[string]string{
		"test_step:QA-1:11:action":          "Open the ${ACCOUNT} page",
		"test_step:QA-1:11:data":            "${ACCOUNT}",
		"test_step:QA-1:11:expected":        "Welcome ${ACCOUNT}",
		"test_case:QA-1:description":        "Sign in with ${ACCOUNT} in ${REGION}.",
		"test_case:QA-1:generic_definition": "run --user ${ACCOUNT}",
	} {
		if got := queued[key]; got != want {
			t.Errorf("%s queued as %q, want %q", key, got, want)
		}
	}

	// The other parameter's placeholder is untouched, and so is a step that
	// never mentioned the renamed one.
	if _, touched := queued["test_step:QA-1:12:action"]; touched {
		t.Error("a step that does not mention the parameter was edited")
	}

	// The cache shows the rewrite immediately, like every other local edit.
	steps, err := r.ListTestSteps("p1", "QA-1")
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if steps[0].Action != "Open the ${ACCOUNT} page" {
		t.Errorf("cached step action = %q, want the rewritten one", steps[0].Action)
	}
}

// A parameter nothing refers to is renamed without touching the test. The
// count is what the caller reports, so it has to be honest.
func TestRenameDatasetPlaceholderReportsNothingToRewrite(t *testing.T) {
	r := newTestRepo(t)
	seedParameterizedTest(t, r)

	changed, err := r.RenameDatasetPlaceholder("p1", "QA-1", "TIER", "LEVEL")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if changed != 0 {
		t.Errorf("rewrote %d fields, want none", changed)
	}
	if rows := pendingFields(t, r); len(rows) != 0 {
		t.Errorf("queued %v, want nothing", rows)
	}
}

// A name that is a prefix of another must not be rewritten inside it: ${USER}
// and ${USER_ID} are different parameters.
func TestRenameDatasetPlaceholderLeavesALongerNameAlone(t *testing.T) {
	r := newTestRepo(t)
	if err := r.UpsertTests("p1", []TestCase{{
		Key: "QA-1", ID: "1", Summary: "s", Status: "Open",
		Description: "${USER} and ${USER_ID}", Updated: "2026-01-01T00:00:00Z",
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := r.RenameDatasetPlaceholder("p1", "QA-1", "USER", "ACCOUNT"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got := pendingFields(t, r)["test_case:QA-1:description"]; got != "${ACCOUNT} and ${USER_ID}" {
		t.Errorf("description = %q, want only the exact name rewritten", got)
	}
}
