package jira

import (
	"fmt"
	"strings"
	"time"
)

// Demo mode generates fake Test data so the UI can be exercised without a
// real Jira instance. A profile triggers demo mode when its Jira URL is
// "demo", "demo://…", or "mock://…" (case-insensitive). Auth tokens are
// ignored; TestConnection / SearchTestsPage / ListFolders short-circuit to
// the generators in this file.

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
