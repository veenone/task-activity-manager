package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// TestListIssuesFiltersByAssigneeName pins the "assigned to me" match: a row
// with a non-empty assignee_name is matched on that alone, case-insensitive,
// and a row whose assignee_name is still empty (cached before schema 14)
// falls back to the assignee display name. An empty AssigneeName leaves the
// query exactly as it was before this filter existed.
func TestListIssuesFiltersByAssigneeName(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Now()
	page := sample()
	page[0].AssigneeName = "ranand" // PLAT-412, Assignee display name "R. Anand"
	page[2].AssigneeName = "jdoe"   // PLAT-350, Assignee display name "PO"
	// PLAT-409 (page[1]) and PLAT-347 (page[3]) keep AssigneeName empty; the
	// latter's display name is "S. Kim".
	if err := r.UpsertPage(ctx, "p1", page, now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	cases := []struct {
		name string
		q    issuerepo.IssueQuery
		want []string
	}{
		{
			// The caller always sends the display name alongside the username
			// (the connected user's own identity), so a realistic case names
			// both even where the primary branch is the one expected to match.
			"matches by assignee_name",
			issuerepo.IssueQuery{AssigneeName: "ranand", AssigneeDisplayName: "R. Anand"},
			[]string{"PLAT-412"},
		},
		{
			"case-insensitive",
			issuerepo.IssueQuery{AssigneeName: "RaNaNd", AssigneeDisplayName: "R. Anand"},
			[]string{"PLAT-412"},
		},
		{
			"empty assignee_name falls back to the display name",
			issuerepo.IssueQuery{AssigneeName: "skim", AssigneeDisplayName: "S. Kim"},
			[]string{"PLAT-347"},
		},
		{
			// PLAT-350 has assignee_name "jdoe" but display name "PO"; a query
			// for "PO" must not match it through the fallback, because the
			// fallback only applies when assignee_name is empty.
			"non-empty non-matching assignee_name is excluded despite a matching display name",
			issuerepo.IssueQuery{AssigneeName: "po", AssigneeDisplayName: "PO"},
			nil,
		},
		{
			// With no display name there is no fallback at all. Comparing
			// assignee against "" would match every unassigned row, so the
			// list would fill with work belonging to nobody: PLAT-409 carries
			// neither an assignee nor an assignee_name. Only the username
			// branch may match here.
			"no display name means no fallback, and unassigned rows stay out",
			issuerepo.IssueQuery{AssigneeName: "ranand"},
			[]string{"PLAT-412"},
		},
		{
			"empty AssigneeName leaves the Backlog unfiltered",
			issuerepo.IssueQuery{},
			[]string{"PLAT-409", "PLAT-412", "PLAT-347", "PLAT-350"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := r.ListIssues(ctx, "p1", c.q)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(got.Issues) != len(c.want) {
				t.Fatalf("keys = %v, want %v", keysOf(got.Issues), c.want)
			}
			for i, k := range c.want {
				if got.Issues[i].Key != k {
					t.Errorf("row %d = %s, want %s", i, got.Issues[i].Key, k)
				}
			}
		})
	}
}

// TestListIssuesFiltersByAssigneeNameIncludesDraftsAndPendingReassignment
// covers the two cases beyond the plain column match: a draft only ever
// carries a username (CreateDraft writes it into both assignee and
// assignee_name), and a pending reassignment is a direct write to the row
// (writeField, via EditField's "assignee" path, which AssigneePicker
// actually sends), so the filter sees it immediately, before any Commit.
func TestListIssuesFiltersByAssigneeNameIncludesDraftsAndPendingReassignment(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	page := sample()[:2]
	page[0].AssigneeName = "alice" // PLAT-412: will be reassigned to me
	page[1].AssigneeName = "me"    // PLAT-409: will be reassigned away
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	draftKey, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Draft one", Assignee: "me"})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if err := r.EditField(ctx, "p1", "PLAT-412", "assignee", "me"); err != nil {
		t.Fatalf("reassign to me: %v", err)
	}
	if err := r.EditField(ctx, "p1", "PLAT-409", "assignee", "bob"); err != nil {
		t.Fatalf("reassign away: %v", err)
	}

	got, err := r.ListIssues(ctx, "p1", issuerepo.IssueQuery{AssigneeName: "me"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	keys := keysOf(got.Issues)
	want := map[string]bool{draftKey: true, "PLAT-412": true}
	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for _, k := range keys {
		if !want[k] {
			t.Errorf("unexpected key %s in %v", k, keys)
		}
		if k == "PLAT-409" {
			t.Errorf("PLAT-409 was reassigned away from me and should not be listed")
		}
	}
}

func keysOf(issues []backend.Issue) []string {
	out := make([]string, len(issues))
	for i, iss := range issues {
		out[i] = iss.Key
	}
	return out
}
