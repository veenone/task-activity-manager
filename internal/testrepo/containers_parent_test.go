package testrepo

import "testing"

func TestUpsertAndListContainersCarryParent(t *testing.T) {
	r := newTestRepo(t) // shared helper in sankey_crossproject_test.go
	const p = "p1"

	in := []Container{
		{Key: "DEMO-TE-1", Kind: "testexec", Summary: "Standalone", Status: "Open"},
		{Key: "DEMO-STE-1", Kind: "testexec", Summary: "Sub", Status: "Open",
			ParentKey: "DEMO-S-1", IssueType: "Sub Test Execution"},
	}
	if err := r.UpsertContainers(p, in); err != nil {
		t.Fatalf("UpsertContainers: %v", err)
	}
	got, err := r.ListContainers(p, "testexec")
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	byKey := map[string]Container{}
	for _, c := range got {
		byKey[c.Key] = c
	}
	if byKey["DEMO-TE-1"].ParentKey != "" {
		t.Errorf("standalone exec should have empty ParentKey, got %q", byKey["DEMO-TE-1"].ParentKey)
	}
	sub := byKey["DEMO-STE-1"]
	if sub.ParentKey != "DEMO-S-1" || sub.IssueType != "Sub Test Execution" {
		t.Errorf("sub-task exec lost parent/issuetype: %+v", sub)
	}
}
