package worklogweek_test

import (
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/worklogweek"
)

func entry(key, started string, pending bool) backend.LoggedWork {
	return backend.LoggedWork{Key: key, Summary: "from Jira", Log: backend.Worklog{Started: started, Seconds: 3600, Pending: pending}}
}

func TestBuildListsOpenAssignedIssuesThenTheOthersLoggedOn(t *testing.T) {
	assigned := []backend.Issue{
		{Key: "PLAT-1", Summary: "open", StatusCategory: "indeterminate"},
		{Key: "PLAT-2", Summary: "finished", StatusCategory: "done"},
		{Key: "PLAT-3", Summary: "finished, worked on this week", StatusCategory: "done"},
	}
	cache := map[string]backend.Issue{"PLAT-3": assigned[2]}
	lookup := func(key string) (backend.Issue, bool) { i, ok := cache[key]; return i, ok }
	remote := []backend.LoggedWork{
		entry("OPS-88", "2026-11-17T14:00:00.000+0700", false),
		entry("PLAT-1", "2026-11-16T09:00:00.000+0700", false),
	}
	pending := []backend.LoggedWork{
		entry("PLAT-3", "2026-11-18T09:00:00.000+0700", true),
		entry("PLAT-1", "2026-11-09T09:00:00.000+0700", true),
	}

	v := worklogweek.Build(assigned, lookup, remote, pending, "2026-11-16", "2026-11-22")

	if len(v.Rows) != 3 {
		t.Fatalf("rows = %+v, want the open one and the two logged on", v.Rows)
	}
	if r := v.Rows[0]; r.Issue.Key != "PLAT-1" || !r.Assigned || !r.Cached {
		t.Errorf("first row = %+v, want the open assigned issue", r)
	}
	// Jira knows OPS-88 and the cache does not, so the row is drawn from what
	// the search carried and says work cannot be journalled against it.
	if r := v.Rows[1]; r.Issue.Key != "OPS-88" || r.Issue.Summary != "from Jira" || r.Assigned || r.Cached {
		t.Errorf("second row = %+v, want OPS-88 uncached", r)
	}
	if r := v.Rows[2]; r.Issue.Key != "PLAT-3" || r.Assigned || !r.Cached {
		t.Errorf("third row = %+v, want the finished issue that was logged on", r)
	}
	// Oldest first, and the pending entry from the week before is left out.
	if len(v.Entries) != 3 || v.Entries[0].Key != "PLAT-1" || v.Entries[1].Key != "OPS-88" || !v.Entries[2].Log.Pending {
		t.Errorf("entries = %+v, want the three of the week in start order", v.Entries)
	}
}
