package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

// The New issue dialog offers the types the project has, so the type list
// has to say which of them TAM has a logical type for and which are the
// project's sub-task level (issue #65 item 2). PLAT is the shape the
// reporter's project has: a type TAM has no concept of, a requirement type
// under the instance's own name, and a sub-task level called something else
// again.
func TestIssueTypesCarryTheLogicalTypeAndTheSubtaskFlag(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	types, err := b.IssueTypes(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("issue types: %v", err)
	}
	want := []backend.IssueType{
		{ID: "1", Name: "Task", Logical: backend.TypeTask},
		// Nothing TAM knows. The empty logical type is what keeps it from
		// being offered, stored or drawn as a task.
		{ID: "4", Name: "Improvement"},
		{ID: "7", Name: "Business Requirement", Logical: backend.TypeRequirement},
		{ID: "19", Name: "Technical task", Subtask: true, Logical: backend.TypeSubtask},
	}
	if len(types) != len(want) {
		t.Fatalf("types = %+v, want %d of them", types, len(want))
	}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("types[%d] = %+v, want %+v", i, types[i], w)
		}
	}
}

// A draft of a type TAM has no logical type for carries the project's own
// name for it. The create sends that name only when the project really has
// it; a name nothing has heard of is refused by name rather than becoming
// the type nobody chose.
func TestCreateSendsAProjectTypeNameItCanFind(t *testing.T) {
	b, f := newBackend(t, twoFields)
	f.createKey = "PLAT-900"
	ctx := context.Background()
	key, _, err := b.CreateIssue(ctx, "PLAT", backend.IssueDraft{Type: "Improvement", Summary: "Trim the bundle"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if key != "PLAT-900" {
		t.Fatalf("create returned %q", key)
	}
	if len(f.writes) != 1 || !strings.Contains(f.writes[0], `"issuetype":{"name":"Improvement"}`) {
		t.Errorf("writes = %q, want one create naming Improvement", f.writes)
	}
	if _, _, err := b.CreateIssue(ctx, "PLAT", backend.IssueDraft{Type: "Spike", Summary: "x"}); err == nil {
		t.Error("a type the project does not have was created anyway")
	}
}

// Xray's test artefacts belong to XTM, so they are not offered as types
// here and not fetched by a sync. They are recognised by the plugin icon
// the project serves for them, never by the name "Test", which an instance
// renames and localises.
func TestIssueTypesDropsXraysOwnTypes(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	types, err := b.IssueTypes(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("issue types: %v", err)
	}
	for _, ty := range types {
		if ty.Name == "Test" {
			t.Errorf("types = %+v, want Xray's Test left out", types)
		}
	}
}
