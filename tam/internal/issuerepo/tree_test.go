package issuerepo_test

import (
	"context"
	"fmt"
	"strings"
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
	keys := []string{}
	for _, o := range tree.Orphans {
		keys = append(keys, o.Key)
	}
	if strings.Join(keys, ",") != "PLAT-409,PLAT-500" {
		t.Errorf("orphans (no parent and unknown parent): %v", keys)
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
	if len(hidden.Orphans) != 2 {
		t.Errorf("orphans: %+v", hidden.Orphans)
	}
	byText, _ := repo.EpicTree(ctx, "p1", issuerepo.TreeQuery{Text: "analytics", ShowDone: true})
	if len(byText.Epics) != 1 || len(byText.Epics[0].Children) != 1 || byText.Epics[0].Children[0].Key != "PLAT-385" || len(byText.Orphans) != 0 {
		t.Errorf("a matching child keeps its epic and only itself: %+v", byText)
	}
	byEpic, _ := repo.EpicTree(ctx, "p1", issuerepo.TreeQuery{Text: "promotions", ShowDone: true})
	if len(byEpic.Epics) != 1 || len(byEpic.Epics[0].Children) != 2 {
		t.Errorf("a matching epic keeps every child: %+v", byEpic)
	}
	bySprint, _ := repo.EpicTree(ctx, "p1", issuerepo.TreeQuery{SprintID: "12", ShowDone: true})
	if len(bySprint.Epics) != 1 || len(bySprint.Epics[0].Children) != 1 || len(bySprint.Orphans) != 1 || bySprint.Orphans[0].Key != "PLAT-409" {
		t.Errorf("sprint filter: %+v", bySprint)
	}
	epics, _ := repo.ListEpics(ctx, "p1")
	if len(epics) != 2 || epics[0].Key != "PLAT-350" {
		t.Errorf("ListEpics: %+v", epics)
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
		t.Error("5,002 non-epic rows must set Truncated")
	}
	if tree.Epics[0].Total != 5000 {
		t.Errorf("Total counts the capped rows, not the whole cache: %d", tree.Epics[0].Total)
	}
}
