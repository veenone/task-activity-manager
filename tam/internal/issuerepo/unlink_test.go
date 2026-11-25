package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// seedChain caches PLAT-1 blocks PLAT-2 on both ends with Jira's id 10100,
// and the same link under a second profile.
func seedChain(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	for _, p := range []string{"p1", "p2"} {
		page := []backend.Issue{
			linkedIssue("PLAT-1", []backend.Link{blocks("10100", "outward", "PLAT-2")}),
			linkedIssue("PLAT-2", []backend.Link{blocks("10100", "inward", "PLAT-1")}),
		}
		if err := repo.UpsertPage(context.Background(), p, page, time.Now(), false); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}
}

// A removal is journaled like an add (#612): one link row naming Jira's id,
// so Commit can delete it, and the cached link stays until Commit lands.
func TestRemoveLinkJournalsTheRemovalByJiraID(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedChain(t, repo)
	if err := repo.RemoveLink(ctx, "p1", "PLAT-1", blocks("10100", "outward", "PLAT-2")); err != nil {
		t.Fatalf("RemoveLink: %v", err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(pend) != 1 || pend[0].EntityType != issuerepo.EntityLink || pend[0].Field != "-|Blocks|outward|PLAT-2" {
		t.Fatalf("journal row: %+v", pend)
	}
	links, err := repo.PendingLinks(ctx, "p1", "PLAT-1")
	if err != nil || len(links) != 1 || !links[0].Removal || links[0].ID != "10100" || !links[0].Pending || links[0].Key != "PLAT-2" {
		t.Errorf("PendingLinks: %+v %v", links, err)
	}
	if cached, _ := repo.ListLinks(ctx, "p1", "PLAT-1"); len(cached) != 1 {
		t.Errorf("the cached link stays until Commit: %+v", cached)
	}
	if err := repo.RemoveLink(ctx, "p1", "PLAT-1", blocks("10100", "outward", "PLAT-2")); err == nil {
		t.Error("the same removal twice should fail")
	}
}

func TestRemoveLinkRefusesALinkItCannotDelete(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedChain(t, repo)
	cases := map[string]backend.Link{
		"no id":      blocks("", "outward", "PLAT-2"),
		"not cached": blocks("999", "outward", "PLAT-3"),
	}
	for name, l := range cases {
		if err := repo.RemoveLink(ctx, "p1", "PLAT-1", l); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1"); len(pend) != 0 {
		t.Errorf("nothing journaled: %+v", pend)
	}
}

// A retype is a removal plus an add, journaled together, removal first so
// Commit deletes before it creates.
func TestRetypeLinkJournalsARemovalThenAnAdd(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedChain(t, repo)
	if err := repo.RetypeLink(ctx, "p1", "PLAT-1", blocks("10100", "outward", "PLAT-2"), "Relates"); err != nil {
		t.Fatalf("RetypeLink: %v", err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(pend) != 2 || pend[0].Field != "-|Blocks|outward|PLAT-2" || pend[1].Field != "Relates|outward|PLAT-2" || pend[0].ID > pend[1].ID {
		t.Fatalf("journal rows: %+v", pend)
	}
	if err := repo.RetypeLink(ctx, "p1", "PLAT-1", blocks("10100", "outward", "PLAT-2"), "Blocks"); err == nil {
		t.Error("retyping to the same type should fail")
	}
}

// DropCachedLink is what Commit runs once Jira deleted a link: both ends'
// rows go, the other profile's copy stays.
func TestDropCachedLinkRemovesBothEndsOfOneProfile(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedChain(t, repo)
	if err := repo.DropCachedLink(ctx, "p1", "10100"); err != nil {
		t.Fatalf("DropCachedLink: %v", err)
	}
	for _, key := range []string{"PLAT-1", "PLAT-2"} {
		if links, _ := repo.ListLinks(ctx, "p1", key); len(links) != 0 {
			t.Errorf("p1 %s links = %+v, want none", key, links)
		}
		if links, _ := repo.ListLinks(ctx, "p2", key); len(links) != 1 {
			t.Errorf("p2 %s links = %+v, want the one", key, links)
		}
	}
}
