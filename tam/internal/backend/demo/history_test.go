package demo_test

import (
	"context"
	"sort"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

func TestSearchIssuesWithHistoryFindsEveryCardThatEverPassedThroughTheClosedSprint(t *testing.T) {
	b := demobackend.New("PLAT")
	// The filler dataset is seeded but still assigns some of its own rows
	// to Sprint 11, so the total is not a fixed four: what matters here is
	// that the four curated cards are found, two of them (PLAT-347 and
	// PLAT-401) only findable through their history since the dataset's
	// own current fields have already moved them on.
	out, total, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != len(out) {
		t.Fatalf("total %d, rows %d, want a page covering the whole scope", total, len(out))
	}
	byKey := map[string]backend.IssueHistory{}
	for _, h := range out {
		byKey[h.Issue.Key] = h
	}
	for _, key := range []string{"PLAT-331", "PLAT-385", "PLAT-347", "PLAT-401"} {
		if _, ok := byKey[key]; !ok {
			t.Errorf("%s missing from sprint = 11, got keys %v", key, keysOf(out))
		}
	}

	// PLAT-347 and PLAT-401 are cached with a later sprint today; the scope
	// still finds them because their curated history says they were once in
	// Sprint 11, the same as Jira's own multi-valued Sprint field would.
	if byKey["PLAT-347"].Issue.SprintID == "11" {
		t.Errorf("PLAT-347's current sprint should already have moved on")
	}
	if byKey["PLAT-401"].Issue.SprintID == "11" {
		t.Errorf("PLAT-401's current sprint should already have moved on")
	}
}

func TestTheCuratedHistoryCoversAnAddALeaveAndReturnAReestimateAndAnEarlyFinish(t *testing.T) {
	b := demobackend.New("PLAT")
	out, _, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	byKey := map[string]backend.IssueHistory{}
	for _, h := range out {
		byKey[h.Issue.Key] = h
	}

	// PLAT-331: done before the sprint's own start date (2026-08-04).
	done := byKey["PLAT-331"]
	if len(done.Changes) != 2 || done.Changes[1].Field != "status" || done.Changes[1].To != "Done" {
		t.Fatalf("PLAT-331 changes = %+v", done.Changes)
	}
	if done.Changes[1].At >= "2026-08-04T09:00:00.000+0000" {
		t.Errorf("PLAT-331 should finish before the sprint starts, finished at %s", done.Changes[1].At)
	}

	// PLAT-385: left the sprint and came back before finishing.
	leftAndBack := byKey["PLAT-385"]
	var sprintMoves []backend.Change
	for _, c := range leftAndBack.Changes {
		if c.Field == "sprint" {
			sprintMoves = append(sprintMoves, c)
		}
	}
	if len(sprintMoves) != 3 {
		t.Fatalf("PLAT-385 sprint moves = %+v, want three: in, out, back in", sprintMoves)
	}
	if sprintMoves[0].To != "Sprint 11" || sprintMoves[1].To != "" || sprintMoves[2].To != "Sprint 11" {
		t.Errorf("PLAT-385 sprint moves = %+v", sprintMoves)
	}

	// PLAT-347: added to the sprint after it had already started.
	add := byKey["PLAT-347"]
	if len(add.Changes) == 0 || add.Changes[0].Field != "sprint" || add.Changes[0].From != "" || add.Changes[0].To != "Sprint 11" {
		t.Fatalf("PLAT-347 changes = %+v", add.Changes)
	}
	if add.Changes[0].At <= "2026-08-04T09:00:00.000+0000" {
		t.Errorf("PLAT-347 should be added after the sprint starts, added at %s", add.Changes[0].At)
	}

	// PLAT-401: re-estimated mid-sprint.
	reestimate := byKey["PLAT-401"]
	var points *backend.Change
	for i, c := range reestimate.Changes {
		if c.Field == "storyPoints" {
			points = &reestimate.Changes[i]
		}
	}
	if points == nil || points.From != "2" || points.To != "3" {
		t.Fatalf("PLAT-401 story points change = %+v", points)
	}
}

func TestHistoryLooksUpTheCuratedChangesUnderARekeyedProject(t *testing.T) {
	b := demobackend.New("DEMO")
	out, _, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var found bool
	for _, h := range out {
		if h.Issue.Key != "DEMO-331" {
			continue
		}
		found = true
		if len(h.Changes) != 2 {
			t.Errorf("DEMO-331 changes = %+v, want the same two PLAT-331 carries", h.Changes)
		}
	}
	if !found {
		t.Fatalf("DEMO-331 not found, got keys %v", keysOf(out))
	}
}

func TestAScopeOutsideSprintOrHistoryIsIgnoredJustAsSearchIssuesPageIgnoresIt(t *testing.T) {
	b := demobackend.New("PLAT")
	scoped, scopedTotal, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("scoped search: %v", err)
	}
	all, allTotal, err := b.SearchIssuesWithHistory(context.Background(), "", 0, 100)
	if err != nil {
		t.Fatalf("unscoped search: %v", err)
	}
	if allTotal <= scopedTotal {
		t.Fatalf("unscoped total %d should exceed the sprint's own %d", allTotal, scopedTotal)
	}
	if len(scoped) != scopedTotal || len(all) != allTotal {
		t.Fatalf("page sizes = %d/%d and %d/%d", len(scoped), scopedTotal, len(all), allTotal)
	}
}

func keysOf(out []backend.IssueHistory) []string {
	keys := make([]string, len(out))
	for i, h := range out {
		keys[i] = h.Issue.Key
	}
	sort.Strings(keys)
	return keys
}
