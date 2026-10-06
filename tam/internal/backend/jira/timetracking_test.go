package jira_test

import (
	"context"
	"strings"
	"testing"
)

// The sync asks for the time fields with the rest of the row, so an estimate
// is cached beside the story points instead of costing a call per issue.
func TestTheSyncsSearchAsksForTheTimeTrackingFields(t *testing.T) {
	b, f := newBackend(t, twoFields)
	if _, _, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", 0, 50); err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(f.searches) != 1 {
		t.Fatalf("searches = %v", f.searches)
	}
	fields := f.searches[0][strings.Index(f.searches[0], "| fields=")+len("| fields="):]
	for _, want := range []string{
		"timeoriginalestimate", "timeestimate", "timespent",
		"aggregatetimeoriginalestimate", "aggregatetimeestimate", "aggregatetimespent",
	} {
		if !strings.Contains(fields, want) {
			t.Errorf("search fields = %q, want %s among them", fields, want)
		}
	}
}

// Jira sends the four as seconds, and sends nothing at all for an issue that
// carries no estimate. Nothing is nil, not zero: an unestimated issue must
// not read back as one estimated at no time.
func TestTimeTrackingIsReadInSecondsAndIsNilWhenJiraSendsNone(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	page, _, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	estimated := page[0]
	if estimated.OriginalEstimateSeconds == nil || *estimated.OriginalEstimateSeconds != 28800 {
		t.Errorf("PLAT-412 original estimate = %v, want the eight hours Jira sent", estimated.OriginalEstimateSeconds)
	}
	if estimated.RemainingEstimateSeconds == nil || *estimated.RemainingEstimateSeconds != 7200 {
		t.Errorf("PLAT-412 remaining estimate = %v", estimated.RemainingEstimateSeconds)
	}
	if estimated.TimeSpentSeconds == nil || *estimated.TimeSpentSeconds != 21600 {
		t.Errorf("PLAT-412 time spent = %v", estimated.TimeSpentSeconds)
	}
	// The family's three, which is where everything sits for an issue
	// estimated through its sub-tasks (#142).
	if estimated.AggregateEstimateSeconds == nil || *estimated.AggregateEstimateSeconds != 144000 {
		t.Errorf("PLAT-412 aggregate estimate = %v, want the family's 40h", estimated.AggregateEstimateSeconds)
	}
	if estimated.AggregateRemainingSeconds == nil || *estimated.AggregateRemainingSeconds != 100800 {
		t.Errorf("PLAT-412 aggregate remaining = %v", estimated.AggregateRemainingSeconds)
	}
	if estimated.AggregateTimeSpentSeconds == nil || *estimated.AggregateTimeSpentSeconds != 43200 {
		t.Errorf("PLAT-412 aggregate time spent = %v", estimated.AggregateTimeSpentSeconds)
	}
	unestimated := page[1]
	if unestimated.OriginalEstimateSeconds != nil || unestimated.RemainingEstimateSeconds != nil ||
		unestimated.TimeSpentSeconds != nil || unestimated.AggregateEstimateSeconds != nil ||
		unestimated.AggregateRemainingSeconds != nil || unestimated.AggregateTimeSpentSeconds != nil {
		t.Errorf("PLAT-388 = %+v, want no time tracking at all", unestimated)
	}
}
