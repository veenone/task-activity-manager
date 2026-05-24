package jira

import (
	"fmt"
	"strings"
	"time"
)

// Demo mode generates fake Test data so the UI can be exercised without a
// real Jira instance. A profile triggers demo mode when its Jira URL is
// "demo", "demo://…", or "mock://…" (case-insensitive). Auth tokens are
// ignored; TestConnection / SearchTestsPage / ListFolders / ListPreconditions
// short-circuit to the generators in this file.

// demoTestCount is the size of the fake dataset — enough to exercise the
// grid's pagination, search, filter and sort without being absurd.
const demoTestCount = 5000

// isDemoURL reports whether a profile's Jira URL selects demo mode.
func isDemoURL(baseURL string) bool {
	u := strings.ToLower(strings.TrimSpace(baseURL))
	return u == "demo" ||
		strings.HasPrefix(u, "demo:") ||
		strings.HasPrefix(u, "mock:")
}

// demoTestsPage returns the [startAt, startAt+maxResults) slice of the demo
// dataset plus the dataset total — matching SearchTestsPage's signature.
func demoTestsPage(projectKey string, startAt, maxResults int) ([]Test, int) {
	if projectKey == "" {
		projectKey = "DEMO"
	}
	if startAt >= demoTestCount {
		return nil, demoTestCount
	}
	end := startAt + maxResults
	if end > demoTestCount {
		end = demoTestCount
	}
	out := make([]Test, 0, end-startAt)
	for i := startAt; i < end; i++ {
		out = append(out, makeDemoTest(projectKey, i))
	}
	return out, demoTestCount
}

// Source vocabularies. Statuses and priorities are deliberately repeated to
// produce a weighted distribution — most tests are Approved/Done, a few are
// Blocked or Deprecated.

var demoFeatures = []string{
	"Login", "Logout", "User registration", "Password reset",
	"Search", "Filter results", "Sort results", "Pagination",
	"Checkout", "Cart", "Add to cart", "Remove from cart",
	"Profile update", "Settings", "Dashboard", "Notifications",
	"Payment", "Refund", "Reports", "Export to CSV",
	"Import data", "Admin console", "Permissions", "Audit log",
	"API rate limit", "File upload", "File download",
	"Multi-factor auth", "Session timeout", "Bulk operations",
}

var demoConditions = []string{
	"with valid input",
	"with invalid input",
	"from empty state",
	"after timeout",
	"with special characters",
	"on a slow network",
	"as an admin user",
	"as a guest user",
	"with maximum boundary values",
	"with minimum boundary values",
	"after page reload",
	"with concurrent users",
}

var demoStatuses = []string{
	"Approved", "Approved", "Approved", "Approved",
	"Done", "Done", "Done",
	"In Progress", "In Progress",
	"Open",
	"Blocked",
	"Deprecated",
}

var demoPriorities = []string{
	"Medium", "Medium", "Medium",
	"High", "High",
	"Low",
	"Critical",
}

var demoLabels = []string{
	"smoke", "regression", "p1", "p2", "p3",
	"api", "ui", "manual", "automated", "flaky",
	"security", "performance",
}

// demoFolderCategories defines the demo Test Repository hierarchy. Feature
// names match those in demoFeatures so each test slots into the matching
// leaf folder.
var demoFolderCategories = []struct {
	Name     string
	Features []string
}{
	{"Authentication", []string{"Login", "Logout", "User registration", "Password reset", "Multi-factor auth", "Session timeout"}},
	{"Browse", []string{"Search", "Filter results", "Sort results", "Pagination"}},
	{"Commerce", []string{"Checkout", "Cart", "Add to cart", "Remove from cart", "Payment", "Refund"}},
	{"User", []string{"Profile update", "Settings", "Notifications"}},
	{"Reporting", []string{"Dashboard", "Reports", "Export to CSV", "Import data"}},
	{"Admin", []string{"Admin console", "Permissions", "Audit log"}},
	{"System", []string{"API rate limit", "File upload", "File download", "Bulk operations"}},
}

// demoFolders returns the demo folder tree. Folder IDs are full paths so a
// folder is uniquely identified by its location in the tree
// ("/Authentication/Login").
func demoFolders(_ string) []Folder {
	out := make([]Folder, 0)
	for _, cat := range demoFolderCategories {
		catID := "/" + cat.Name
		out = append(out, Folder{ID: catID, ParentID: "", Name: cat.Name})
		for _, feat := range cat.Features {
			out = append(out, Folder{
				ID:       catID + "/" + feat,
				ParentID: catID,
				Name:     feat,
			})
		}
	}
	return out
}

// demoFolderForFeature returns the leaf folder ID holding tests for a given
// feature, or empty if the feature isn't mapped.
func demoFolderForFeature(feature string) string {
	for _, cat := range demoFolderCategories {
		for _, f := range cat.Features {
			if f == feature {
				return "/" + cat.Name + "/" + f
			}
		}
	}
	return ""
}

// preconditionDefs is the master list of distinct demo preconditions. Their
// indexes here are used by featurePreconditions to assign preconditions to
// tests by feature.
var preconditionDefs = []struct {
	Summary string
	Type    string
}{
	{"User account exists", "Manual"},
	{"User is logged in", "Manual"},
	{"Email service is available", "Manual"},
	{"MFA device enrolled", "Manual"},
	{"Search index is populated", "Manual"},
	{"Cart has items", "Manual"},
	{"Payment method on file", "Manual"},
	{"Product catalog is loaded", "Manual"},
	{"Completed order exists", "Manual"},
	{"Admin user is logged in", "Manual"},
	{"At least one report exists", "Manual"},
	{"Database has seed data", "Manual"},
	{"Network is available", "Manual"},
	{"File system has write access", "Manual"},
	{"Multiple users are logged in", "Manual"},
}

// featurePreconditions maps each feature in demoFeatures to indexes into
// preconditionDefs. Tests inherit these preconditions based on their feature.
var featurePreconditions = map[string][]int{
	"Login":             {0},
	"Logout":            {1},
	"User registration": {2},
	"Password reset":    {0, 2},
	"Multi-factor auth": {0, 3},
	"Session timeout":   {1},
	"Search":            {4},
	"Filter results":    {4},
	"Sort results":      {4},
	"Pagination":        {4},
	"Checkout":          {5, 6},
	"Cart":              {1},
	"Add to cart":       {7},
	"Remove from cart":  {5},
	"Profile update":    {1},
	"Settings":          {1},
	"Dashboard":         {1},
	"Notifications":     {1},
	"Payment":           {6},
	"Refund":            {8},
	"Reports":           {1, 10},
	"Export to CSV":     {1, 10},
	"Import data":       {9, 11},
	"Admin console":     {9},
	"Permissions":       {9},
	"Audit log":         {1, 9},
	"API rate limit":    {12},
	"File upload":       {1, 13},
	"File download":     {1, 13},
	"Bulk operations":   {9, 14},
}

// demoPreconditionsAndLinks returns the demo precondition master list plus
// the test-key → precondition-keys mapping. Keys use a "<project>-P-N"
// convention so they read like Jira keys without colliding with the test
// number range.
func demoPreconditionsAndLinks(projectKey string) ([]Precondition, map[string][]string, error) {
	if projectKey == "" {
		projectKey = "DEMO"
	}

	preconditions := make([]Precondition, 0, len(preconditionDefs))
	for i, def := range preconditionDefs {
		preconditions = append(preconditions, Precondition{
			Key:         fmt.Sprintf("%s-P-%d", projectKey, i+1),
			Summary:     def.Summary,
			Type:        def.Type,
			Description: fmt.Sprintf("(Demo precondition: %s)", def.Summary),
		})
	}

	links := make(map[string][]string, demoTestCount)
	for i := 0; i < demoTestCount; i++ {
		feature := demoFeatures[i%len(demoFeatures)]
		indexes, ok := featurePreconditions[feature]
		if !ok || len(indexes) == 0 {
			continue
		}
		testKey := fmt.Sprintf("%s-%d", projectKey, i+1)
		keys := make([]string, len(indexes))
		for j, idx := range indexes {
			keys[j] = fmt.Sprintf("%s-P-%d", projectKey, idx+1)
		}
		links[testKey] = keys
	}

	return preconditions, links, nil
}

// makeDemoTest builds a deterministic Test for index i, so repeated syncs of
// a demo profile are idempotent.
func makeDemoTest(projectKey string, i int) Test {
	feature := demoFeatures[i%len(demoFeatures)]
	condition := demoConditions[(i/len(demoFeatures))%len(demoConditions)]
	status := demoStatuses[i%len(demoStatuses)]
	priority := demoPriorities[(i*7+3)%len(demoPriorities)]

	// 1–3 labels, derived from i so the same Test always carries the same
	// labels. Duplicates collapse to a unique set.
	labelCount := (i % 3) + 1
	seen := make(map[string]struct{}, labelCount)
	labels := make([]string, 0, labelCount)
	for j := 0; j < labelCount; j++ {
		l := demoLabels[(i*(j+1)+11)%len(demoLabels)]
		if _, dup := seen[l]; dup {
			continue
		}
		seen[l] = struct{}{}
		labels = append(labels, l)
	}

	updated := time.Now().AddDate(0, 0, -(i % 365)).
		Format("2006-01-02T15:04:05.000-0700")

	summary := fmt.Sprintf("%s %s", feature, condition)
	description := fmt.Sprintf(
		"Given a user is on the %s screen\n"+
			"When they perform the action %s\n"+
			"Then the system should respond correctly\n\n"+
			"(Demo data — generated for UI testing.)",
		strings.ToLower(feature), condition)

	return Test{
		Key:         fmt.Sprintf("%s-%d", projectKey, i+1),
		ID:          fmt.Sprintf("%d", 10000+i),
		Summary:     summary,
		Description: description,
		Status:      status,
		Priority:    priority,
		Labels:      labels,
		Updated:     updated,
		FolderID:    demoFolderForFeature(feature),
	}
}
