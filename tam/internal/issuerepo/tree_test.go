package issuerepo_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func seedTree(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	rows := []backend.Issue{
		{Key: "PLAT-350", Type: backend.TypeEpic, Summary: "Promotions", Status: "In Progress", Rank: "0|a", Labels: []string{}},
		{Key: "PLAT-320", Type: backend.TypeEpic, Summary: "Checkout", Status: "Done", Rank: "0|b", Labels: []string{}},
		{Key: "PLAT-412", Type: backend.TypeStory, Summary: "Apply promo", Status: "In Progress", ParentKey: "PLAT-350", SprintID: "12", StoryPoints: pts(8), Labels: []string{}},
		{Key: "PLAT-385", Type: backend.TypeTask, Summary: "Analytics event", Status: "Done", ParentKey: "PLAT-350", SprintID: "11", StoryPoints: pts(2), Labels: []string{}},
		{Key: "PLAT-331", Type: backend.TypeStory, Summary: "Guest checkout", Status: "Closed", ParentKey: "PLAT-320", StoryPoints: pts(5), Labels: []string{}},
		{Key: "PLAT-409", Type: backend.TypeTask, Summary: "Rotate keys", Status: "To Do", ParentKey: "", SprintID: "12", StoryPoints: pts(2), Labels: []string{}},
		{Key: "PLAT-500", Type: backend.TypeBug, Summary: "Stale parent", Status: "To Do", ParentKey: "PLAT-999", Labels: []string{}},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
}

func TestEpicTreeGroupsAndCountsAllChildren(t *testing.T) {
	repo := newRepo(t)
	seedTree(t, repo)
	tree, err := repo.EpicTree(context.Background(), "p1", issuerepo.TreeQuery{ShowDone: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Epics) != 2 || tree.Epics[0].Issue.Key != "PLAT-350" {
		t.Fatalf("epics: %+v", tree.Epics)
	}
	promo := tree.Epics[0]
	if promo.Total != 2 || promo.Done != 1 || promo.Points != 10 || promo.DonePoints != 2 || len(promo.Children) != 2 {
		t.Errorf("promo: %+v", promo)
	}
	// The two issues under no epic are no longer in the tree at all; the
	// no-epic tab reads them through ListIssues (#341). The tree reports
	// how many there are so the tab can show a total.
	if tree.OrphanTotal != 2 {
		t.Errorf("OrphanTotal counts PLAT-409 and PLAT-500: %d", tree.OrphanTotal)
	}
}

func TestEpicTreeKeepsSubtasksUnderTheirIssueAndFindsThemByText(t *testing.T) {
	r := newRepo(t)
	seedTree(t, r)
	ctx := context.Background()
	key, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeSubtask, ParentKey: "PLAT-412", Summary: "Wire validation"})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []issuerepo.TreeQuery{{ShowDone: true}, {Text: "validation"}, {SprintID: "12"}} {
		tree, err := r.EpicTree(ctx, "p1", q)
		if err != nil {
			t.Fatal(err)
		}
		if len(tree.Epics) == 0 {
			t.Fatalf("no epic for %+v", q)
		}
		node := tree.Epics[0]
		found := false
		for i, child := range node.Children {
			if child.Key == key {
				found = i > 0 && node.Children[i-1].Key == "PLAT-412"
			}
		}
		if !found {
			t.Fatalf("subtask detached: %+v", node)
		}
		wantTotal := 2
		if q.SprintID != "" {
			wantTotal = 1
		}
		if node.Total != wantTotal {
			t.Fatalf("subtask inflated epic total: %+v", node)
		}
		if tree.OrphanTotal != 2 {
			t.Fatalf("an attached subtask belongs to its epic, not the no-epic side: %d", tree.OrphanTotal)
		}
	}
}

func TestEpicTreeFilters(t *testing.T) {
	repo := newRepo(t)
	seedTree(t, repo)
	ctx := context.Background()
	hidden, _ := repo.EpicTree(ctx, "p1", issuerepo.TreeQuery{})
	if len(hidden.Epics) != 1 || hidden.Epics[0].Issue.Key != "PLAT-350" || len(hidden.Epics[0].Children) != 1 || hidden.Epics[0].Total != 2 {
		t.Errorf("show done off hides done children and fully done epics, counts stay: %+v", hidden.Epics)
	}
	if hidden.OrphanTotal != 2 {
		t.Errorf("OrphanTotal is a count of the cache, not of what Show done draws: %d", hidden.OrphanTotal)
	}
	byText, _ := repo.EpicTree(ctx, "p1", issuerepo.TreeQuery{Text: "analytics", ShowDone: true})
	if len(byText.Epics) != 1 || len(byText.Epics[0].Children) != 1 || byText.Epics[0].Children[0].Key != "PLAT-385" || byText.OrphanTotal != 2 {
		t.Errorf("a matching child keeps its epic and only itself: %+v", byText)
	}
	byEpic, _ := repo.EpicTree(ctx, "p1", issuerepo.TreeQuery{Text: "promotions", ShowDone: true})
	if len(byEpic.Epics) != 1 || len(byEpic.Epics[0].Children) != 2 {
		t.Errorf("a matching epic keeps every child: %+v", byEpic)
	}
	bySprint, _ := repo.EpicTree(ctx, "p1", issuerepo.TreeQuery{SprintID: "12", ShowDone: true})
	if len(bySprint.Epics) != 1 || len(bySprint.Epics[0].Children) != 1 || bySprint.OrphanTotal != 2 {
		t.Errorf("sprint filter: %+v", bySprint)
	}
	epics, _ := repo.ListEpics(ctx, "p1")
	if len(epics) != 2 || epics[0].Key != "PLAT-350" {
		t.Errorf("ListEpics: %+v", epics)
	}
}

// seedOrphanHeavy builds the shape #338 was filed for: a modest epic tree
// behind enough issues with no epic to fill a capped read on their own. The
// orphans rank first so one read in rank order never reaches the children.
func seedOrphanHeavy(t *testing.T, repo *issuerepo.Repository, children, orphans int) {
	t.Helper()
	rows := []backend.Issue{{Key: "PLAT-1", Type: backend.TypeEpic, Summary: "Epic", Rank: "0|m", Labels: []string{}}}
	for i := 1; i <= orphans; i++ {
		rows = append(rows, backend.Issue{
			Key: fmt.Sprintf("ORPH-%d", i), Type: backend.TypeTask, Summary: "No epic", Status: "To Do",
			Rank: fmt.Sprintf("0|a%05d", i), Labels: []string{},
		})
	}
	for i := 1; i <= children; i++ {
		status := "To Do"
		if i%2 == 0 {
			status = "Done"
		}
		rows = append(rows, backend.Issue{
			Key: fmt.Sprintf("PLAT-%d", 100+i), Type: backend.TypeStory, Summary: "Child", Status: status,
			ParentKey: "PLAT-1", Rank: fmt.Sprintf("0|z%05d", i), StoryPoints: pts(2), Labels: []string{},
		})
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
}

// An epic keeps its children and its counts however many issues without an
// epic sit in front of them (#338). Before the read was split, the cap fell
// across one rank-ordered pass and the orphans ate it, so these epics drew
// as "0 of 0" and opened empty.
func TestEpicTreeKeepsEpicChildrenWhenOrphansFillTheCap(t *testing.T) {
	repo := newRepo(t)
	seedOrphanHeavy(t, repo, 40, 5100)
	tree, err := repo.EpicTree(context.Background(), "p1", issuerepo.TreeQuery{ShowDone: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Epics) != 1 {
		t.Fatalf("the epic must survive the orphans: %d epics", len(tree.Epics))
	}
	if got := tree.Epics[0].Total; got != 40 {
		t.Errorf("Total is the children the epic really has: got %d, want 40", got)
	}
	if got := tree.Epics[0].Done; got != 20 {
		t.Errorf("Done counts every done child, not the ones a cap let through: got %d, want 20", got)
	}
	if got := len(tree.Epics[0].Children); got != 40 {
		t.Errorf("expanding the epic shows its children: got %d, want 40", got)
	}
}

// The notice over the tree needs the number it reports, not just the fact
// that something was dropped (#338).
func TestEpicTreeReportsHowManyIssuesHaveNoEpic(t *testing.T) {
	repo := newRepo(t)
	seedOrphanHeavy(t, repo, 40, 5100)
	tree, err := repo.EpicTree(context.Background(), "p1", issuerepo.TreeQuery{ShowDone: true})
	if err != nil {
		t.Fatal(err)
	}
	// 5,100 issues with no epic no longer reach the tree, so they cannot
	// truncate it; the no-epic tab pages them on its own (#341). What the
	// tree owes that tab is the total for its label.
	if tree.Truncated {
		t.Error("the issues with no epic are not in the tree and cannot cut it short")
	}
	if got := tree.OrphanTotal; got != 5100 {
		t.Errorf("OrphanTotal counts every issue with no epic: got %d, want 5100", got)
	}
	if got := tree.IssueTotal; got != 40 {
		t.Errorf("IssueTotal counts the work under the epics: got %d, want 40", got)
	}
}

// A profile under the cap says nothing was dropped, so the view can keep the
// notice hidden (#338).
func TestEpicTreeUnderTheCapIsNotTruncated(t *testing.T) {
	repo := newRepo(t)
	seedTree(t, repo)
	tree, err := repo.EpicTree(context.Background(), "p1", issuerepo.TreeQuery{ShowDone: true})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Truncated {
		t.Error("a small profile drops nothing")
	}
	if got := tree.OrphanTotal; got != 2 {
		t.Errorf("OrphanTotal counts PLAT-409 and PLAT-500: got %d, want 2", got)
	}
}

func TestEpicTreeCapsAtFiveThousandRows(t *testing.T) {
	repo := newRepo(t)
	rows := []backend.Issue{{Key: "PLAT-1", Type: backend.TypeEpic, Summary: "Big epic", Rank: "0|a", Labels: []string{}}}
	for i := 1; i <= 5002; i++ {
		rows = append(rows, backend.Issue{
			Key: fmt.Sprintf("PLAT-%d", i+1), Type: backend.TypeTask, Summary: "Child", Status: "To Do",
			ParentKey: "PLAT-1", Rank: fmt.Sprintf("0|%05d", i), Labels: []string{},
		})
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	tree, err := repo.EpicTree(context.Background(), "p1", issuerepo.TreeQuery{ShowDone: true})
	if err != nil {
		t.Fatal(err)
	}
	if !tree.Truncated {
		t.Error("5,002 children under one epic fill the epic read and must set EpicsTruncated")
	}
	if tree.Epics[0].Total != 5000 {
		t.Errorf("Total counts the capped rows, the one case where it cannot be whole: %d", tree.Epics[0].Total)
	}
}

// An issue of a type TAM does not model reaches the tree now that the sync
// fetches every type (#68). It is placed by its parent like any other
// non-epic issue: under the epic it belongs to, and in the "No epic" group
// when it belongs to none, which is the tree's own way of saying it could
// not be placed under an epic. It counts towards its epic's progress, since
// it is work the epic is waiting on like any other.
func TestEpicTreePlacesATypeTAMDoesNotModel(t *testing.T) {
	repo := newRepo(t)
	seedTree(t, repo)
	rows := []backend.Issue{
		{Key: "PLAT-600", Type: "Improvement", Summary: "Trim the bundle", Status: "To Do", ParentKey: "PLAT-350", Rank: "0|c", Labels: []string{}},
		{Key: "PLAT-601", Type: "Todo", Summary: "Chase the vendor", Status: "To Do", ParentKey: "", Rank: "0|d", Labels: []string{}},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	tree, err := repo.EpicTree(context.Background(), "p1", issuerepo.TreeQuery{ShowDone: true})
	if err != nil {
		t.Fatal(err)
	}
	promo := tree.Epics[0]
	if promo.Issue.Key != "PLAT-350" {
		t.Fatalf("first epic = %s", promo.Issue.Key)
	}
	under := false
	for _, c := range promo.Children {
		under = under || c.Key == "PLAT-600"
	}
	if !under || promo.Total != 3 {
		t.Errorf("promo = %+v, want PLAT-600 among its children and counted", promo)
	}
	// PLAT-601 belongs to no epic, so it is counted on the no-epic side
	// rather than drawn in the tree (#341).
	if tree.OrphanTotal != 3 {
		t.Errorf("OrphanTotal = %d, want PLAT-601 counted among the three with no epic", tree.OrphanTotal)
	}
}

// The no-epic tab reads through ListIssues rather than the tree, so it pages
// like the Backlog does (#341). The filter is the tree's own underEpic
// predicate negated, so the two tabs partition the profile between them.
func TestListIssuesNoEpicIsTheOtherHalfOfTheTree(t *testing.T) {
	repo := newRepo(t)
	seedTree(t, repo)
	ctx := context.Background()
	page, err := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{NoEpic: true, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, iss := range page.Issues {
		got[iss.Key] = true
	}
	// PLAT-409 has no parent at all and PLAT-500 hangs off a key that was
	// never synced. Neither sits under an epic, and nothing else in the
	// seed qualifies.
	if page.Total != 2 || !got["PLAT-409"] || !got["PLAT-500"] {
		t.Errorf("issues with no epic = %d %v, want PLAT-409 and PLAT-500", page.Total, got)
	}
	under, err := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	// Every non-epic issue is on exactly one side. The unfiltered list
	// counts the epics too, so they come off the total before comparing.
	// PLAT-412, PLAT-385 and PLAT-331 are the three the seed puts under an
	// epic, against the two with none.
	epics, _ := repo.ListEpics(ctx, "p1")
	if under.Total-len(epics) != page.Total+3 {
		t.Errorf("the two sides must partition the non-epic issues: %d non-epic, %d with no epic", under.Total-len(epics), page.Total)
	}
}

// A profile whose issues all sit under epics has an empty no-epic side
// rather than a broken query (#341).
func TestListIssuesNoEpicIsEmptyWhenEverythingHasAnEpic(t *testing.T) {
	repo := newRepo(t)
	rows := []backend.Issue{
		{Key: "PLAT-1", Type: backend.TypeEpic, Summary: "Epic", Rank: "0|a", Labels: []string{}},
		{Key: "PLAT-2", Type: backend.TypeStory, Summary: "Child", Status: "To Do", ParentKey: "PLAT-1", Rank: "0|b", Labels: []string{}},
		{Key: "PLAT-3", Type: backend.TypeSubtask, Summary: "Sub", Status: "To Do", ParentKey: "PLAT-2", Rank: "0|c", Labels: []string{}},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	page, err := repo.ListIssues(context.Background(), "p1", issuerepo.IssueQuery{NoEpic: true, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Errorf("nothing is outside the epics here: %d, %+v", page.Total, page.Issues)
	}
}
