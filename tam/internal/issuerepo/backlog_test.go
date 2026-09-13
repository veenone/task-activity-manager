package issuerepo_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func TestBacklogKeepsSubtasksWithParentsAcrossPagesAndFilters(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	rows := []backend.Issue{
		{Key: "P-1", Type: "story", Summary: "Parent", Rank: "a"},
		{Key: "P-2", Type: "bug", Summary: "Another parent", Rank: "b"},
		{Key: "P-3", Type: "subtask", ParentKey: "P-1", Summary: "Child needle", Rank: "z", SprintID: "12"},
		{Key: "P-4", Type: "subtask", ParentKey: "P-1", Summary: "Sibling", Rank: "c"},
		{Key: "P-5", Type: "subtask", ParentKey: "P-404", Summary: "Uncached parent", Rank: "d"},
	}
	if err := r.UpsertPage(ctx, "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	// A parent cached only in another profile must not hide an orphan.
	if err := r.UpsertPage(ctx, "p2", []backend.Issue{{Key: "P-404", Type: "task"}}, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		query issuerepo.IssueQuery
		keys  []string
		total int
	}{
		{"first family", issuerepo.IssueQuery{Limit: 1}, []string{"P-1", "P-4", "P-3"}, 3},
		{"next page", issuerepo.IssueQuery{Limit: 1, Offset: 1}, []string{"P-2"}, 3},
		{"orphan stays visible", issuerepo.IssueQuery{Limit: 1, Offset: 2}, []string{"P-5"}, 3},
		{"child search includes family", issuerepo.IssueQuery{Text: "needle"}, []string{"P-1", "P-4", "P-3"}, 1},
		{"child sprint includes family", issuerepo.IssueQuery{SprintID: "12"}, []string{"P-1", "P-4", "P-3"}, 1},
		{"parent type includes children", issuerepo.IssueQuery{Types: []string{"story"}}, []string{"P-1", "P-4", "P-3"}, 1},
		{"sort keeps family together", issuerepo.IssueQuery{Sort: "key", Desc: true, Limit: 1, Offset: 2}, []string{"P-1", "P-4", "P-3"}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.query.GroupSubtasks = true
			page, err := r.ListIssues(ctx, "p1", tc.query)
			if err != nil {
				t.Fatal(err)
			}
			var keys []string
			for _, iss := range page.Issues {
				keys = append(keys, iss.Key)
			}
			if page.Total != tc.total || !reflect.DeepEqual(keys, tc.keys) {
				t.Fatalf("keys = %v, total = %d; want %v, %d", keys, page.Total, tc.keys, tc.total)
			}
		})
	}
}
