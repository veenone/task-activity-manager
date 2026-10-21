package issuerepo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// TestListIssuesFiltersByStatus pins the status filter: an empty list leaves
// the query alone, one status narrows to it, and several are an OR. Status
// names are compared as the sync stored them, which is how the grid shows
// them and how ListFilterOptions offers them.
func TestListIssuesFiltersByStatus(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	cases := []struct {
		name string
		q    issuerepo.IssueQuery
		want string
	}{
		{"no statuses leaves the query unfiltered", issuerepo.IssueQuery{}, "PLAT-409,PLAT-412,PLAT-347,PLAT-350"},
		{"one status", issuerepo.IssueQuery{Statuses: []string{"To Do"}}, "PLAT-409,PLAT-347"},
		{"several statuses are an OR", issuerepo.IssueQuery{Statuses: []string{"To Do", "In Progress"}}, "PLAT-409,PLAT-412,PLAT-347,PLAT-350"},
		{"a status no row carries matches nothing", issuerepo.IssueQuery{Statuses: []string{"Blocked"}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page, err := r.ListIssues(ctx, "p1", c.q)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if got := strings.Join(keysOf(page.Issues), ","); got != c.want {
				t.Errorf("keys = %q, want %q", got, c.want)
			}
			if page.Total != len(page.Issues) {
				t.Errorf("total %d counts something other than the %d rows the filter matched", page.Total, len(page.Issues))
			}
		})
	}
}

// TestListIssuesFiltersByAssignee pins the assignee the user picked, which is
// the display name the grid shows, not the username the "assigned to me" tab
// matches on. The empty string is its own choice: it means unassigned, which
// is why the field is a list and not a string that would read as no filter.
func TestListIssuesFiltersByAssignee(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	cases := []struct {
		name string
		q    issuerepo.IssueQuery
		want string
	}{
		{"no assignees leaves the query unfiltered", issuerepo.IssueQuery{}, "PLAT-409,PLAT-412,PLAT-347,PLAT-350"},
		{"one person", issuerepo.IssueQuery{Assignees: []string{"S. Kim"}}, "PLAT-347"},
		{"the empty name is unassigned, not everyone", issuerepo.IssueQuery{Assignees: []string{""}}, "PLAT-409"},
		{"a person and nobody together", issuerepo.IssueQuery{Assignees: []string{"S. Kim", ""}}, "PLAT-409,PLAT-347"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page, err := r.ListIssues(ctx, "p1", c.q)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if got := strings.Join(keysOf(page.Issues), ","); got != c.want {
				t.Errorf("keys = %q, want %q", got, c.want)
			}
		})
	}
}

// TestListIssuesCombinesTheFilters is the case a filter bar actually produces:
// the user narrows by more than one field at a time, and the conditions are an
// AND across fields while staying an OR inside one.
func TestListIssuesCombinesTheFilters(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	page, err := r.ListIssues(ctx, "p1", issuerepo.IssueQuery{
		Types:    []string{"task"},
		Statuses: []string{"To Do"},
		SprintID: "13",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := strings.Join(keysOf(page.Issues), ","); got != "PLAT-347" {
		t.Errorf("keys = %q, want PLAT-347: the three filters have to be an AND", got)
	}
}

// TestListFilterOptionsReadsTheCache pins what the filter bar offers. Like
// ListSprints it reads the cached rows rather than asking Jira, so an option
// is never offered for something the view cannot show. The empty assignee is
// left out: "Unassigned" is the bar's own option, and an empty string in this
// list would render as a blank row the user cannot tell from the All one.
func TestListFilterOptionsReadsTheCache(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := r.UpsertPage(ctx, "p2", sample()[:1], time.Now(), false); err != nil {
		t.Fatalf("upsert p2: %v", err)
	}
	opts, err := r.ListFilterOptions(ctx, "p1")
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	if got := strings.Join(opts.Statuses, ","); got != "In Progress,To Do" {
		t.Errorf("statuses = %q, want the distinct statuses sorted", got)
	}
	if got := strings.Join(opts.Assignees, ","); got != "PO,R. Anand,S. Kim" {
		t.Errorf("assignees = %q, want the distinct names sorted, with no empty entry", got)
	}
	// A second profile's cache is a different set of options, or the bar would
	// offer a status from someone else's project.
	two, err := r.ListFilterOptions(ctx, "p2")
	if err != nil {
		t.Fatalf("options p2: %v", err)
	}
	if got := strings.Join(two.Statuses, ","); got != "In Progress" {
		t.Errorf("p2 statuses = %q, want only its own", got)
	}
}

// TestEpicTreeFiltersByTypeStatusAndAssignee pins the Epics view's half. The
// tree reads at most treeCap rows before it groups them, so these filters run
// in SQL beside the sprint one: filtering in Go after the cap would narrow
// whichever 5,000 rows the cap happened to let through, and the truncation
// note would then describe a different set than the tree shows.
//
// An epic is kept when the filter left it a child, the rule the sprint filter
// already follows. An epic has no assignee or type of its own worth filtering
// on here: the tree is read for the work under it.
func TestEpicTreeFiltersByTypeStatusAndAssignee(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	seedTree(t, r)

	cases := []struct {
		name  string
		q     issuerepo.TreeQuery
		epics string
		under string
	}{
		{
			"type keeps only that type under the epic",
			issuerepo.TreeQuery{Types: []string{"task"}, ShowDone: true},
			"PLAT-350",
			"PLAT-385",
		},
		{
			"status narrows the children",
			issuerepo.TreeQuery{Statuses: []string{"In Progress"}, ShowDone: true},
			"PLAT-350",
			"PLAT-412",
		},
		{
			"an epic whose children are all filtered out is dropped",
			issuerepo.TreeQuery{Statuses: []string{"Blocked"}, ShowDone: true},
			"",
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree, err := r.EpicTree(ctx, "p1", c.q)
			if err != nil {
				t.Fatalf("tree: %v", err)
			}
			got := make([]string, 0, len(tree.Epics))
			for _, e := range tree.Epics {
				got = append(got, e.Issue.Key)
			}
			if strings.Join(got, ",") != c.epics {
				t.Fatalf("epics = %v, want %q", got, c.epics)
			}
			if c.epics == "" {
				return
			}
			if kept := strings.Join(keysOf(tree.Epics[0].Children), ","); kept != c.under {
				t.Errorf("children = %q, want %q", kept, c.under)
			}
		})
	}
}

// TestEpicTreeFiltersByAssignee is its own test because the seed tree has no
// assignees, so the rows it needs are written here rather than bending the
// shared seed every other tree test reads.
func TestEpicTreeFiltersByAssignee(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	seedTree(t, r)
	page := []backend.Issue{
		{Key: "PLAT-412", Type: backend.TypeStory, Summary: "Apply promo", Status: "In Progress", Assignee: "S. Kim", ParentKey: "PLAT-350", SprintID: "12", Labels: []string{}},
	}
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	tree, err := r.EpicTree(ctx, "p1", issuerepo.TreeQuery{Assignees: []string{"S. Kim"}, ShowDone: true})
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if len(tree.Epics) != 1 || tree.Epics[0].Issue.Key != "PLAT-350" {
		t.Fatalf("epics = %+v", tree.Epics)
	}
	if got := strings.Join(keysOf(tree.Epics[0].Children), ","); got != "PLAT-412" {
		t.Errorf("children = %q, want only the row assigned to S. Kim", got)
	}
	if len(tree.Orphans) != 0 {
		t.Errorf("orphans = %v, want none: no unparented row is assigned to S. Kim", keysOf(tree.Orphans))
	}
}
