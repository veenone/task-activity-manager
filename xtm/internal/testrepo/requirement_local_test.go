package testrepo_test

import (
	"strings"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// A requirement created in XTM carries a "NEW-REQ-N" placeholder until a commit
// creates it in Jira. These cover what happens to it, and to the pending rows
// that name it, before that commit (#482).

// seedLocalRequirement creates a local requirement and stages an edit, a
// coverage link from QA-3 and an outbound link to PRD-12 against it.
func seedLocalRequirement(t *testing.T) (*testrepo.Repository, string) {
	t.Helper()
	repo := seedReqRepo(t)
	key, err := repo.CreateRequirement("p1", "PRD", "Story", "Lockout policy", "", "", "", "", nil)
	if err != nil {
		t.Fatalf("create requirement: %v", err)
	}
	if err := repo.EditRequirementField("p1", key, "summary", "Account lockout policy"); err != nil {
		t.Fatalf("edit local requirement: %v", err)
	}
	if err := repo.SetTestRequirements("p1", "QA-3", []string{"PRD-11", key}); err != nil {
		t.Fatalf("link test to local requirement: %v", err)
	}
	if err := repo.SetRequirementLinks("p1", key, "Relates", []string{"PRD-12"}); err != nil {
		t.Fatalf("link local requirement: %v", err)
	}
	return repo, key
}

func TestDeleteLocalRequirementCancelsItsCreate(t *testing.T) {
	repo, key := seedLocalRequirement(t)

	if err := repo.DeleteRequirement("p1", key); err != nil {
		t.Fatalf("delete: %v", err)
	}

	cov, err := repo.ListRequirementsWithCoverage("p1")
	if err != nil {
		t.Fatalf("list requirements: %v", err)
	}
	for _, c := range cov {
		if c.Key == key {
			t.Errorf("%s still listed after delete", key)
		}
	}
	// Nothing about it may reach Jira: no delete of a key Jira never had, no
	// create, and no edit or link naming the placeholder.
	left, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("pending after deleting a local requirement = %+v, want none", left)
	}
}

func TestRenameRequirementRewritesPendingRows(t *testing.T) {
	repo, key := seedLocalRequirement(t)
	if err := repo.SetRequirementLinks("p1", "PRD-10", "Relates", []string{key}); err != nil {
		t.Fatalf("link to local requirement: %v", err)
	}

	if err := repo.RenameRequirement("p1", key, "PRD-99"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	changes, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	got := map[string]testrepo.PendingChange{}
	for _, c := range changes {
		// The create row keeps the placeholder; the commit drops it next.
		if c.EntityType == "requirement_create" {
			continue
		}
		if c.EntityKey == key || strings.Contains(c.AfterVal, key) {
			t.Errorf("%s row for %s still names %s: after=%s", c.EntityType, c.EntityKey, key, c.AfterVal)
		}
		got[c.EntityType+" "+c.EntityKey] = c
	}
	if c := got["requirement_set QA-3"]; !strings.Contains(c.AfterVal, "PRD-99") {
		t.Errorf("QA-3 coverage set after = %q, want it to name PRD-99", c.AfterVal)
	}
	if c := got["requirement_edit PRD-99"]; c.AfterVal != "Account lockout policy" {
		t.Errorf("edit row for PRD-99 = %+v, want the staged summary", c)
	}
	if c := got["req_req_link_set PRD-99"]; !strings.Contains(c.AfterVal, "PRD-12") {
		t.Errorf("outbound link row for PRD-99 = %+v, want it to name PRD-12", c)
	}
	if c := got["req_req_link_set PRD-10"]; !strings.Contains(c.AfterVal, "PRD-99") {
		t.Errorf("PRD-10 link set after = %q, want it to name PRD-99", c.AfterVal)
	}
}
