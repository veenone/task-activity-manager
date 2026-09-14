package demo_test

import (
	"context"
	"sort"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

// sprint11Start is the same instant boards.go's curated Sprint 11 carries as
// its StartDate ("2026-08-04T09:00:00Z"), written here in the changelog
// wire's own offset form instead of the Agile API's: Jira's two endpoints
// really do format a timestamp differently, so the two literals are meant
// to differ and are not a typo to reconcile. If Sprint 11's start ever
// moves, this constant has to move with it.
const sprint11Start = "2026-08-04T09:00:00.000+0000"

func TestSearchIssuesWithHistoryFindsExactlyWhatSearchIssuesPageFinds(t *testing.T) {
	b := demobackend.New("PLAT")
	withHistory, historyTotal, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("history search: %v", err)
	}
	page, pageTotal, err := b.SearchIssuesPage(context.Background(), "PLAT", "sprint = 11", "", nil, 0, 50)
	if err != nil {
		t.Fatalf("page search: %v", err)
	}
	if historyTotal != pageTotal {
		t.Fatalf("history total %d, page total %d, want the same scope", historyTotal, pageTotal)
	}
	pageKeys := map[string]bool{}
	for _, iss := range page {
		pageKeys[iss.Key] = true
	}
	if len(withHistory) != len(pageKeys) {
		t.Fatalf("history rows %d, page rows %d", len(withHistory), len(pageKeys))
	}
	for _, h := range withHistory {
		if !pageKeys[h.Issue.Key] {
			t.Errorf("%s came back from SearchIssuesWithHistory but not from SearchIssuesPage for the same jql", h.Issue.Key)
		}
	}

	// The curated cards this task's report cases are built on must all be
	// present: each one's cached sprint is 11 today, which is the only
	// reason a "sprint = 11" search, real or demo, can still see it.
	byKey := map[string]backend.IssueHistory{}
	for _, h := range withHistory {
		byKey[h.Issue.Key] = h
	}
	for _, key := range []string{"PLAT-331", "PLAT-385", "PLAT-347", "PLAT-401"} {
		if _, ok := byKey[key]; !ok {
			t.Errorf("%s missing from sprint = 11, got keys %v", key, keysOf(withHistory))
		}
	}
}

func TestACardThatLeftTheSprintAndNeverReturnedIsInvisibleToTheHistorySearch(t *testing.T) {
	b := demobackend.New("PLAT")
	out, _, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	// PLAT-398's curated history shows it passing through Sprint 11 before
	// moving on to Sprint 13 for good, exactly the case a real instance
	// cannot see either: `sprint = 11` only returns whoever is in the
	// sprint now, so a card that left mid-flight is never fetched and its
	// changelog is never read. This dataset has the history on file and
	// still does not return it, which is the point.
	for _, h := range out {
		if h.Issue.Key == "PLAT-398" {
			t.Fatalf("PLAT-398 left Sprint 11 and never came back; it must not be in the result, got %+v", h)
		}
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
	if done.Changes[1].At >= sprint11Start {
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

	// PLAT-347: added to the sprint after it had already started, and it
	// stays, which is what keeps the scope increase visible at all.
	add := byKey["PLAT-347"]
	if len(add.Changes) != 1 || add.Changes[0].Field != "sprint" || add.Changes[0].From != "" || add.Changes[0].To != "Sprint 11" {
		t.Fatalf("PLAT-347 changes = %+v", add.Changes)
	}
	if add.Changes[0].At <= sprint11Start {
		t.Errorf("PLAT-347 should be added after the sprint starts, added at %s", add.Changes[0].At)
	}

	// PLAT-401: re-estimated mid-sprint, and it stays too.
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
