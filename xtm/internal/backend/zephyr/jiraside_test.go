package zephyr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

// TestTheDelegatedMethodsOnlyTalkToJira is the boundary this file's whole
// approach rests on.
//
// Half of backend.Backend is a plain Jira call on a Zephyr profile, and
// internal/backend/xray is where those calls are already adapted to the
// neutral DTOs. Delegating to it is only safe while the delegated methods
// touch Jira's own API and nothing else: an Xray endpoint under /rest/raven
// does not exist on a Zephyr instance, and a method that quietly grew one
// would answer 404 for a user and look like a Zephyr fault.
//
// So this test drives every delegated method against a recording server and
// asserts what they asked for. It fails if any of them reaches outside
// /rest/api, which is what makes the shortcut a decision rather than a bet.
func TestTheDelegatedMethodsOnlyTalkToJira(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.URL.Path] = true
		mu.Unlock()
		// Enough of a body for every shape these calls decode into. The
		// answers do not matter here; the paths do.
		switch {
		case strings.HasSuffix(r.URL.Path, "/search"):
			_, _ = w.Write([]byte(`{"issues":[],"total":0}`))
		case strings.Contains(r.URL.Path, "/issuetype") || strings.Contains(r.URL.Path, "/issueLinkType"):
			_, _ = w.Write([]byte(`[]`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	a := newTestAdapter(srv)
	ctx := context.Background()

	// Every method this package delegates, with the smallest arguments that
	// still make the call. Errors are ignored on purpose: a decode failure
	// against a stub body is not what is under test.
	_, _, _ = a.SearchTestsPage(ctx, "PROJ", "", "", 0, 1)
	_, _ = a.ListTestsBasic(ctx, []string{"PROJ-101"})
	_, _ = a.GetTestFields(ctx, "PROJ-101")
	_, _, _ = a.SearchTestsAcrossProjects(ctx, []string{"PROJ"}, "login", 0, 1)
	_, _ = a.GetTestMeta(ctx, "PROJ-101")
	_, _ = a.CreateTest(ctx, "PROJ", "a test", "", "", nil, nil, nil)
	_ = a.UpdateIssue(ctx, "PROJ-101", map[string]any{"summary": "a test"})
	_, _ = a.RemoteVersion(ctx, "test", "PROJ-101")
	_, _ = a.GetTransitions(ctx, "PROJ-101", "Open")
	_ = a.PostTransition(ctx, "PROJ-101", "11", nil)
	_, _ = a.ListStatuses(ctx, "PROJ")
	_, _ = a.ListPriorities(ctx, "PROJ")
	_, _ = a.ProjectComponents(ctx, "PROJ")
	_, _ = a.ProjectVersions(ctx, "PROJ")
	_ = a.AddComment(ctx, "PROJ-101", "a comment")
	_, _, _ = a.ListRequirements(ctx, "PROJ", nil, nil)
	_ = a.UpdateTestRequirements(ctx, "PROJ-101", []string{"PROJ-1"}, nil)
	_, _ = a.ListIssueLinkTypes(ctx)
	_, _ = a.ListIssueLinkTypeDetails(ctx)
	_, _ = a.CreateRequirement(ctx, "PROJ", "Story", "a story", "", "", "", "", nil)
	_, _ = a.GetRequirementCreateFields(ctx, "PROJ", "Story")
	_, _ = a.GetTestCreateFields(ctx, "PROJ")
	_ = a.DeleteRequirement(ctx, "PROJ-1")
	_ = a.UpdateRequirementLinks(ctx, "PROJ-1", []string{"PROJ-2"}, nil)
	_, _ = a.ListReqToReqLinks(ctx, []string{"PROJ-1"})
	_, _, _ = a.ListBugs(ctx, "PROJ", []string{"PROJ-101"}, "Bug", nil)
	_, _ = a.ListProjectBugs(ctx, "PROJ", "Bug")
	_, _ = a.ListBugsByKeys(ctx, []string{"PROJ-900"})
	_, _ = a.GetFieldDefs(ctx, "PROJ", "Bug")
	_, _ = a.CreateBug(ctx, "PROJ", "Bug", "a defect", "", "", nil, nil)
	_ = a.CreateBugLink(ctx, "PROJ-101", "PROJ-900")
	_, _ = a.GetBugDetail(ctx, "PROJ-900")
	// The Jira half of the custom fields. ListCustomFields is deliberately not
	// here: it reads Zephyr's own namespaces too, which is the whole point of
	// customfields.go, so it would fail this test by design.
	_, _ = a.GetTestCustomFields(ctx, "PROJ-101")
	_, _, _ = a.CustomFieldValue(ctx, "customfield_10102", "Automated")

	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("none of the delegated methods made a request, so nothing was proved")
	}
	var outside []string
	for path := range seen {
		if !strings.HasPrefix(path, "/rest/api/") {
			outside = append(outside, path)
		}
	}
	sort.Strings(outside)
	if len(outside) > 0 {
		t.Errorf("delegated methods asked for %v, which a Zephyr instance does not serve", outside)
	}
}

// TestGetTestFieldsReturnsTheIssueItRead keeps one delegation honest about its
// result rather than only its path, so a delegation that returned the zero
// value would not pass on paths alone.
func TestGetTestFieldsReturnsTheIssueItRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/PROJ-101") {
			_, _ = w.Write([]byte(`{"id":"10501","key":"PROJ-101","fields":{"summary":"Sign in with a valid account"}}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	a := newTestAdapter(srv)

	test, err := a.GetTestFields(context.Background(), "PROJ-101")
	if err != nil {
		t.Fatalf("GetTestFields: %v", err)
	}
	if test.Key != "PROJ-101" || test.Summary != "Sign in with a valid account" {
		t.Errorf("read %+v, want the issue the server returned", test)
	}
}

// TestRemoteAheadComparesTheJiraTimestamps replaces Phase 1's placeholder,
// which called any difference a conflict. A test whose remote copy is older
// than the local base is not a conflict, and reporting one blocked a commit
// that had nothing wrong with it.
func TestRemoteAheadComparesTheJiraTimestamps(t *testing.T) {
	a := New(nil, nil)
	const earlier = "2026-10-30T09:00:00.000+0000"
	const later = "2026-10-30T10:00:00.000+0000"

	if !a.RemoteAhead(earlier, later) {
		t.Error("a later remote timestamp is not reported as ahead")
	}
	if a.RemoteAhead(later, earlier) {
		t.Error("an older remote timestamp is reported as ahead, which manufactures a conflict")
	}
	if a.RemoteAhead(later, later) {
		t.Error("an unchanged timestamp is reported as ahead")
	}
}
