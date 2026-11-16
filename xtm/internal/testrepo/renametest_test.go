package testrepo_test

import (
	"strings"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// A test created in XTM is "NEW-N" until a commit creates it in Jira and
// RenameTest swaps in the real key. Everything attached to the test before
// that commit has to follow it to the new key (#484).

// seedRenamedLocalTest creates a local test, attaches a covered requirement, a
// custom field value, a review and a new bug to it, then renames it to QA-50.
func seedRenamedLocalTest(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := seedReqRepo(t)
	if err := repo.UpsertCustomFields("p1", []testrepo.CustomFieldDef{
		{FieldID: "customfield_10100", Name: "Feature", Type: "string"},
	}); err != nil {
		t.Fatalf("seed custom field: %v", err)
	}
	key, err := repo.CreateTest("p1", testrepo.TestDraft{Summary: "Lockout after five attempts"})
	if err != nil {
		t.Fatalf("create test: %v", err)
	}
	if err := repo.SetTestRequirements("p1", key, []string{"PRD-12"}); err != nil {
		t.Fatalf("link requirement: %v", err)
	}
	if err := repo.EditTestCustomField("p1", key, "customfield_10100", "Lockout"); err != nil {
		t.Fatalf("edit custom field: %v", err)
	}
	if err := repo.SetTestReview("p1", key, "approved", "ana", "ok"); err != nil {
		t.Fatalf("review: %v", err)
	}
	if _, err := repo.CreateBugForTest("p1", key, "", testrepo.BugDraft{
		ProjectKey: "QA", IssueType: "Bug", Summary: "Counter resets early",
	}); err != nil {
		t.Fatalf("create bug: %v", err)
	}
	if err := repo.RenameTest("p1", key, "QA-50"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	return repo
}

func TestRenameTestCarriesAttachedDataToTheNewKey(t *testing.T) {
	repo := seedRenamedLocalTest(t)

	reqs, err := repo.GetTestRequirements("p1", "QA-50")
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	if len(reqs) != 1 || reqs[0].Key != "PRD-12" {
		t.Errorf("requirements of QA-50 = %+v, want PRD-12", reqs)
	}
	fields, err := repo.ListTestCustomFields("p1", "QA-50")
	if err != nil {
		t.Fatalf("read custom fields: %v", err)
	}
	if len(fields) != 1 || fields[0].Value != "Lockout" {
		t.Errorf("custom fields of QA-50 = %+v, want Feature = Lockout", fields)
	}
	review, err := repo.GetTestReview("p1", "QA-50")
	if err != nil {
		t.Fatalf("read review: %v", err)
	}
	if review.Verdict != "approved" {
		t.Errorf("review of QA-50 = %+v, want approved", review)
	}
	bugs, err := repo.GetTestBugs("p1", "QA-50")
	if err != nil {
		t.Fatalf("read bugs: %v", err)
	}
	if len(bugs) != 1 || bugs[0].Summary != "Counter resets early" {
		t.Errorf("bugs of QA-50 = %+v, want the new bug", bugs)
	}
}

func TestRenameTestRewritesPendingRowsThatNameTheTest(t *testing.T) {
	repo := seedRenamedLocalTest(t)

	changes, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	keys := map[string]string{}
	for _, c := range changes {
		keys[c.EntityType] = c.EntityKey
		if c.EntityType == "bug_create" && !strings.Contains(c.AfterVal, `"testKey":"QA-50"`) {
			t.Errorf("bug_create payload = %s, want testKey QA-50", c.AfterVal)
		}
	}
	if keys["requirement_set"] != "QA-50" {
		t.Errorf("requirement_set key = %q, want QA-50", keys["requirement_set"])
	}
	if keys["custom_field"] != "QA-50:customfield_10100" {
		t.Errorf("custom_field key = %q, want QA-50:customfield_10100", keys["custom_field"])
	}
	if _, ok := keys["bug_create"]; !ok {
		t.Error("no bug_create row left to check")
	}
}
