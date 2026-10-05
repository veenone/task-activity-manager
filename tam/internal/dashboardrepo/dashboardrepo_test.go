package dashboardrepo_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/dashboardrepo"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *dashboardrepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return dashboardrepo.New(db.DB())
}

func pts(v float64) *float64 { return &v }
func secs(v int) *int        { return &v }

// issues is a filter's worth of rows, spanning two projects on purpose:
// a saved filter is not bound to the profile's own project.
func issues() []backend.Issue {
	return []backend.Issue{
		{Key: "PLAT-1", Type: "story", Status: "In Progress", StatusCategory: "indeterminate", Assignee: "R. Anand",
			StoryPoints: pts(5), OriginalEstimateSeconds: secs(28800), TimeSpentSeconds: secs(7200)},
		{Key: "PLAT-2", Type: "bug", Status: "Done", StatusCategory: "done", Assignee: "R. Anand", StoryPoints: pts(3)},
		{Key: "OPS-9", Type: "task", Status: "To Do", StatusCategory: "new", Assignee: "",
			OriginalEstimateSeconds: secs(3600), TimeSpentSeconds: secs(1800)},
	}
}

// searcher is a backend that answers one page of a filter's JQL.
type searcher struct {
	jql   string
	pages [][]backend.Issue
	calls int
	err   error
}

func (s *searcher) SearchByJQL(_ context.Context, jql string, startAt, _ int) ([]backend.Issue, int, error) {
	s.jql = jql
	s.calls++
	if s.err != nil {
		return nil, 0, s.err
	}
	total := 0
	for _, p := range s.pages {
		total += len(p)
	}
	if startAt >= len(s.pages) {
		return nil, total, nil
	}
	return s.pages[startAt], total, nil
}

// A dashboard is kept, so it is still there with its numbers after a
// restart. The store is the only thing between the two.
func TestADashboardAndItsNumbersSurviveAReopen(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	d, err := r.Create(ctx, "p1", "My open bugs", "10100", "assignee = currentUser()")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.SaveSnapshot(ctx, "p1", d.ID, dashboardrepo.Summarise(issues()), "2026-10-02T09:00:00Z"); err != nil {
		t.Fatalf("save: %v", err)
	}
	list, err := r.List(ctx, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("dashboards = %+v", list)
	}
	got := list[0]
	if got.Name != "My open bugs" || got.FilterID != "10100" || got.JQL != "assignee = currentUser()" {
		t.Errorf("dashboard = %+v", got)
	}
	if got.RefreshedAt != "2026-10-02T09:00:00Z" {
		t.Errorf("refreshed at = %q, want the stamp the snapshot was taken at", got.RefreshedAt)
	}
	if got.Snapshot.Total != 3 || got.Snapshot.Points != 8 {
		t.Errorf("snapshot = %+v, want the three issues and their eight points", got.Snapshot)
	}
}

// The panels are counts, and they come off the rows the filter returned,
// whatever project those rows are in.
func TestSummariseCountsEveryRowTheFilterReturned(t *testing.T) {
	s := dashboardrepo.Summarise(issues())
	if s.Total != 3 {
		t.Errorf("total = %d", s.Total)
	}
	if s.Points != 8 {
		t.Errorf("points = %v, want the two estimated rows", s.Points)
	}
	if s.EstimateSeconds != 32400 || s.SpentSeconds != 9000 {
		t.Errorf("time = %d estimated, %d spent", s.EstimateSeconds, s.SpentSeconds)
	}
	// Largest bucket first, so the panel reads as a ranking.
	if len(s.ByType) != 3 || s.ByStatus[0].Count != 1 {
		t.Errorf("by type = %+v, by status = %+v", s.ByType, s.ByStatus)
	}
	// An unassigned row is counted as unassigned rather than dropped: the
	// panel is about where the work sits, and nobody is a place.
	var unassigned bool
	for _, b := range s.ByAssignee {
		if b.Name == dashboardrepo.Unassigned && b.Count == 1 {
			unassigned = true
		}
	}
	if !unassigned {
		t.Errorf("by assignee = %+v, want the unassigned row counted", s.ByAssignee)
	}
	// The busiest assignee leads.
	if s.ByAssignee[0].Name != "R. Anand" || s.ByAssignee[0].Count != 2 {
		t.Errorf("by assignee = %+v, want the busiest first", s.ByAssignee)
	}
}

// Refresh re-runs the dashboard's own JQL and writes what came back over
// the snapshot, stamped with when it was taken.
func TestRefreshRunsTheFiltersJQLAndStampsWhatItWrote(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	d, err := r.Create(ctx, "p1", "Platform", "10101", "project in (PLAT, OPS)")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	b := &searcher{pages: [][]backend.Issue{issues()}}
	got, err := r.Refresh(ctx, b, "p1", d.ID, "2026-10-02T10:00:00Z")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if b.jql != "project in (PLAT, OPS)" {
		t.Errorf("searched %q, want the dashboard's own jql", b.jql)
	}
	if got.Snapshot.Total != 3 || got.RefreshedAt != "2026-10-02T10:00:00Z" {
		t.Errorf("refreshed = %+v", got)
	}
}

// A refresh that cannot reach Jira leaves the last snapshot exactly where
// it was. A dashboard that blanked itself the moment the network dropped
// would be worse than one that says how old its numbers are.
func TestAFailedRefreshKeepsTheLastSnapshot(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	d, _ := r.Create(ctx, "p1", "Platform", "10101", "project = PLAT")
	if err := r.SaveSnapshot(ctx, "p1", d.ID, dashboardrepo.Summarise(issues()), "2026-10-02T09:00:00Z"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := r.Refresh(ctx, &searcher{err: errors.New("no route to host")}, "p1", d.ID, "2026-10-02T11:00:00Z"); err == nil {
		t.Fatal("refresh against an unreachable Jira must report the failure")
	}
	list, _ := r.List(ctx, "p1")
	if list[0].Snapshot.Total != 3 || list[0].RefreshedAt != "2026-10-02T09:00:00Z" {
		t.Errorf("after a failed refresh = %+v, want the morning's snapshot untouched", list[0])
	}
}

// Deleting one leaves the others, and a dashboard belongs to the profile
// it was made under.
func TestDashboardsAreScopedToTheirProfileAndCanBeDeleted(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	mine, _ := r.Create(ctx, "p1", "Mine", "1", "project = PLAT")
	_, _ = r.Create(ctx, "p1", "Second", "2", "project = OPS")
	_, _ = r.Create(ctx, "p2", "Theirs", "3", "project = XT")

	if err := r.Delete(ctx, "p1", mine.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	p1, _ := r.List(ctx, "p1")
	if len(p1) != 1 || p1[0].Name != "Second" {
		t.Errorf("p1 = %+v", p1)
	}
	p2, _ := r.List(ctx, "p2")
	if len(p2) != 1 {
		t.Errorf("p2 = %+v, want the other profile untouched", p2)
	}
}

// A dashboard with no name has nothing to be found by, and one with no JQL
// counts nothing. Both are refused at the boundary rather than stored.
func TestADashboardNeedsANameAndSomethingToCount(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.Create(ctx, "p1", "  ", "10100", "project = PLAT"); err == nil {
		t.Error("a dashboard with no name was accepted")
	}
	if _, err := r.Create(ctx, "p1", "Nameless", "", "   "); err == nil {
		t.Error("a dashboard with no jql was accepted")
	}
}
