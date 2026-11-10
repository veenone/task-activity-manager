package query

import (
	"context"
	"sync"
	"testing"
)

func TestRunReportsEachPageAsItArrives(t *testing.T) {
	var mu sync.Mutex
	var got []Progress
	ctx := WithProgress(context.Background(), func(p Progress) {
		mu.Lock()
		got = append(got, p)
		mu.Unlock()
	})
	if _, err := Run(ctx, jiraWith(t, 250, 40, nil), "category = X"); err != nil {
		t.Fatal(err)
	}
	// One report per page: 7 pages of up to 40 for 250 issues.
	if len(got) != 7 {
		t.Fatalf("%d reports: %+v", len(got), got)
	}
	last := got[len(got)-1]
	if last.Stage != StageIssues || last.Done != 250 || last.Total != 250 {
		t.Fatalf("last report %+v", last)
	}
	for _, p := range got {
		if p.Done > p.Total {
			t.Fatalf("done past total: %+v", p)
		}
	}
}

func TestExploreReportsLinksWithoutRepeatingIssuePages(t *testing.T) {
	graph := map[string]node{
		"A-1": {links: []string{"B-1"}},
		"B-1": {links: []string{"C-1"}},
		"C-1": {},
	}
	var got []Progress
	ctx := WithProgress(context.Background(), func(p Progress) { got = append(got, p) })
	if _, err := Explore(ctx, linkedJira(t, graph, []string{"A-1"}, &[]string{}, ""), "project = A", true, Fields{}); err != nil {
		t.Fatal(err)
	}
	issues, links := 0, 0
	for _, p := range got {
		switch p.Stage {
		case StageIssues:
			issues++
		case StageLinks:
			links++
		}
	}
	// The query's own page, then one report for each of the two link hops; the
	// link searches' own pages are not reported as issue pages.
	if issues != 1 || links != 2 {
		t.Fatalf("issue reports %d, link reports %d: %+v", issues, links, got)
	}
	if last := got[len(got)-1]; last.Done != 3 {
		t.Fatalf("last report %+v", last)
	}
}

func TestRunWithoutAReporterStillRuns(t *testing.T) {
	if _, err := Run(context.Background(), jiraWith(t, 5, 40, nil), "x"); err != nil {
		t.Fatal(err)
	}
}
