package main

import (
	"context"
	"errors"
	"testing"

	"agile-suite/tam/internal/backend"
)

// filterBackend is a Jira holding two saved filters and one page of rows,
// which can be made unreachable mid-test.
type filterBackend struct {
	stubIssueBackend
	rows    []backend.Issue
	jql     []string
	offline bool
}

func (f *filterBackend) Filters(context.Context) ([]backend.Filter, error) {
	if f.offline {
		return nil, errors.New("no route to host")
	}
	return []backend.Filter{
		{ID: "10100", Name: "My open bugs", JQL: "type = Bug AND resolution = Unresolved"},
	}, nil
}

// byID is the filters this Jira will resolve by id, which is every filter
// the reader can see rather than only the starred ones.
var byID = map[string]backend.Filter{
	"25475": {ID: "25475", Name: "Release blockers", JQL: "project = PLAT AND priority = Blocker"},
}

func (f *filterBackend) FilterByID(_ context.Context, id string) (backend.Filter, error) {
	if f.offline {
		return backend.Filter{}, errors.New("no route to host")
	}
	got, ok := byID[id]
	if !ok {
		return backend.Filter{}, errors.New("jira: 404")
	}
	return got, nil
}

func (f *filterBackend) JiraDashboards(context.Context) ([]backend.JiraDashboard, error) {
	if f.offline {
		return nil, errors.New("no route to host")
	}
	return []backend.JiraDashboard{
		{ID: "25475", Name: "PR Management - IST R&D", URL: "https://jira.example/secure/Dashboard.jspa?selectPageId=25475"},
	}, nil
}

func (f *filterBackend) SearchByJQL(_ context.Context, jql string, startAt, maxResults int) ([]backend.Issue, int, error) {
	if f.offline {
		return nil, 0, errors.New("no route to host")
	}
	f.jql = append(f.jql, jql)
	if startAt >= len(f.rows) {
		return []backend.Issue{}, len(f.rows), nil
	}
	end := startAt + maxResults
	if end > len(f.rows) {
		end = len(f.rows)
	}
	return f.rows[startAt:end], len(f.rows), nil
}

func points(v float64) *float64 { return &v }

func seedDashboardApp(t *testing.T) (*App, string, *filterBackend) {
	t.Helper()
	a := newTestApp(t)
	p := newTestProfile(t, a)
	b := &filterBackend{rows: []backend.Issue{
		{Key: "PLAT-1", Project: "PLAT", Type: backend.TypeBug, Status: "To Do", Assignee: "R. Anand", StoryPoints: points(3)},
		{Key: "OPS-9", Project: "OPS", Type: backend.TypeTask, Status: "In Progress", Assignee: "M. Ortiz"},
	}}
	a.backends[p.ID] = b
	return a, p.ID, b
}

// Creating a dashboard fills it in at once: a reader who has just named
// one should not have to ask for its numbers as a second step.
func TestCreateDashboardRunsTheFilterStraightAway(t *testing.T) {
	a, profileID, b := seedDashboardApp(t)
	d, err := a.CreateDashboard(profileID, "My open bugs", "10100", "type = Bug")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if d.Snapshot.Total != 2 || d.RefreshedAt == "" {
		t.Errorf("dashboard = %+v, want it filled and stamped", d)
	}
	if len(b.jql) != 1 || b.jql[0] != "type = Bug" {
		t.Errorf("searched %v, want the filter's jql once", b.jql)
	}
	// Both projects are counted. A saved filter is not bound to the
	// profile's own project and the dashboard must not narrow it.
	var people int
	for _, bucket := range d.Snapshot.ByAssignee {
		people += bucket.Count
	}
	if people != 2 {
		t.Errorf("assignee buckets = %+v, want both rows counted", d.Snapshot.ByAssignee)
	}
}

// With Jira unreachable the stored figures still come back, with the
// stamp that says how old they are.
func TestDashboardsReadFromTheStoreWhenJiraIsUnreachable(t *testing.T) {
	a, profileID, b := seedDashboardApp(t)
	created, err := a.CreateDashboard(profileID, "My open bugs", "10100", "type = Bug")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	b.offline = true

	list, err := a.ListDashboards(profileID)
	if err != nil {
		t.Fatalf("list with Jira down must still answer: %v", err)
	}
	if len(list) != 1 || list[0].Snapshot.Total != 2 {
		t.Fatalf("list = %+v", list)
	}
	if list[0].RefreshedAt != created.RefreshedAt {
		t.Errorf("stamp = %q, want the one the snapshot was taken at", list[0].RefreshedAt)
	}
	// A refresh against an unreachable Jira says so and changes nothing.
	if _, err := a.RefreshDashboard(profileID, created.ID); err == nil {
		t.Error("a refresh with Jira down must report the failure")
	}
	after, _ := a.ListDashboards(profileID)
	if after[0].Snapshot.Total != 2 || after[0].RefreshedAt != created.RefreshedAt {
		t.Errorf("after a failed refresh = %+v, want the stored snapshot untouched", after[0])
	}
}

// Deleting a profile takes its dashboards with it. The gate checks the
// table is named in a purge list; this checks the rows actually go.
func TestDeletingAProfileRemovesItsDashboards(t *testing.T) {
	a, profileID, _ := seedDashboardApp(t)
	if _, err := a.CreateDashboard(profileID, "My open bugs", "10100", "type = Bug"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := a.repo.PurgeProfile(context.Background(), profileID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	list, err := a.ListDashboards(profileID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("dashboards after the purge = %+v, want none", list)
	}
}

// The saved filters come from Jira, and a connection that cannot read
// them says so rather than answering with an empty list, which would read
// as a person who has starred nothing.
func TestListJiraFiltersReportsAConnectionThatCannotAnswer(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = &stubIssueBackend{}
	if _, err := a.ListJiraFilters(p.ID); err == nil {
		t.Error("a backend with no filters must refuse rather than answer with none")
	}
}

// A filter the picker cannot offer is reached by its id, and the name and
// the JQL come from Jira rather than being typed beside it.
func TestAddDashboardByFilterIDTakesTheNameAndJQLFromJira(t *testing.T) {
	a, profileID, b := seedDashboardApp(t)
	d, err := a.AddDashboardByFilterID(profileID, " 25475 ")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if d.Name != "Release blockers" || d.FilterID != "25475" {
		t.Errorf("dashboard = %+v, want it named by Jira", d)
	}
	if d.JQL != "project = PLAT AND priority = Blocker" {
		t.Errorf("jql = %q, want the filter's own words", d.JQL)
	}
	// It is filled in on the way, the same as one pinned from the picker.
	if d.Snapshot.Total != 2 || d.RefreshedAt == "" {
		t.Errorf("snapshot = %+v, want it counted and stamped", d.Snapshot)
	}
	if len(b.jql) != 1 || b.jql[0] != "project = PLAT AND priority = Blocker" {
		t.Errorf("searched %v, want the resolved jql once", b.jql)
	}
}

// An id that does not resolve adds nothing. A dashboard named after an id
// nobody can see would count nothing and say nothing about why.
func TestAddDashboardByFilterIDAddsNothingWhenTheIDDoesNotResolve(t *testing.T) {
	a, profileID, _ := seedDashboardApp(t)
	if _, err := a.AddDashboardByFilterID(profileID, "999999"); err == nil {
		t.Fatal("want an error for an id this reader cannot see")
	}
	list, err := a.ListDashboards(profileID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("dashboards = %+v, want none added", list)
	}
}

// An empty box is a reader who pressed the button too early, and it must
// not reach Jira as a request for the filter called nothing.
func TestAddDashboardByFilterIDRefusesAnEmptyID(t *testing.T) {
	a, profileID, b := seedDashboardApp(t)
	if _, err := a.AddDashboardByFilterID(profileID, "   "); err == nil {
		t.Fatal("want an error for an empty id")
	}
	if len(b.jql) != 0 {
		t.Errorf("searched %v, want nothing sent", b.jql)
	}
}

// The Jira dashboards are a signpost: a name and the URL that opens it.
// TAM renders none of their gadgets, which Data Center cannot enumerate.
func TestListJiraDashboardsAnswersWithNamesAndURLs(t *testing.T) {
	a, profileID, _ := seedDashboardApp(t)
	boards, err := a.ListJiraDashboards(profileID)
	if err != nil {
		t.Fatalf("dashboards: %v", err)
	}
	if len(boards) != 1 || boards[0].Name != "PR Management - IST R&D" {
		t.Fatalf("dashboards = %+v", boards)
	}
	if boards[0].URL == "" {
		t.Error("want the URL that opens the dashboard")
	}
}

// A connection that cannot read them says so, the same way the filters do.
func TestListJiraDashboardsReportsAConnectionThatCannotAnswer(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = &stubIssueBackend{}
	if _, err := a.ListJiraDashboards(p.ID); err == nil {
		t.Error("a backend with no dashboards must refuse rather than answer with none")
	}
}
