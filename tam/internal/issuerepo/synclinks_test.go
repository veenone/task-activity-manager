package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

// blocks is a link as the sync reads it: Jira's id, the type, and the far end.
func blocks(id, direction, to string) backend.Link {
	return backend.Link{ID: id, Direction: direction, Type: "Blocks", Key: to, Summary: "Summary of " + to, IssueType: "Story"}
}

func linkedIssue(key string, links []backend.Link) backend.Issue {
	return backend.Issue{Key: key, ID: key, Project: "PLAT", Type: backend.TypeTask, Summary: key, Status: "To Do", Labels: []string{}, Links: links}
}

// The sync stores every pulled issue's links with their Jira ids (#612), so
// the Dependencies view does not depend on which issues had their detail open.
func TestUpsertPageStoresEveryIssuesLinksWithTheirIDs(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	page := []backend.Issue{
		linkedIssue("PLAT-1", []backend.Link{blocks("10100", "outward", "PLAT-2")}),
		linkedIssue("PLAT-2", []backend.Link{blocks("10100", "inward", "PLAT-1")}),
	}
	if err := repo.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.UpsertPage(ctx, "p2", []backend.Issue{linkedIssue("PLAT-1", []backend.Link{blocks("9", "outward", "PLAT-7")})}, time.Now(), false); err != nil {
		t.Fatalf("upsert p2: %v", err)
	}
	links, err := repo.ListLinks(ctx, "p1", "PLAT-1")
	if err != nil || len(links) != 1 || links[0].ID != "10100" || links[0].Key != "PLAT-2" || links[0].Direction != "outward" {
		t.Fatalf("PLAT-1 links = %+v, %v", links, err)
	}

	// A later page replaces what the issue had: Jira dropped the link to
	// PLAT-2 and added one to PLAT-3.
	if err := repo.UpsertPage(ctx, "p1", []backend.Issue{linkedIssue("PLAT-1", []backend.Link{blocks("10200", "outward", "PLAT-3")})}, time.Now(), false); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	links, _ = repo.ListLinks(ctx, "p1", "PLAT-1")
	if len(links) != 1 || links[0].ID != "10200" || links[0].Key != "PLAT-3" {
		t.Errorf("after the second page = %+v, want only the link to PLAT-3", links)
	}
	// An issue whose response did not carry links keeps what it had.
	if err := repo.UpsertPage(ctx, "p1", []backend.Issue{linkedIssue("PLAT-1", nil)}, time.Now(), false); err != nil {
		t.Fatalf("third upsert: %v", err)
	}
	if links, _ = repo.ListLinks(ctx, "p1", "PLAT-1"); len(links) != 1 {
		t.Errorf("a page without links cleared them: %+v", links)
	}
	// The other profile's link is untouched.
	if other, _ := repo.ListLinks(ctx, "p2", "PLAT-1"); len(other) != 1 || other[0].Key != "PLAT-7" {
		t.Errorf("p2 links = %+v", other)
	}
}

// A full sync used to clear every cached link at its end. Now that the pages
// carry them, it keeps the links of the issues it synced and drops only the
// links of issues Jira no longer returns.
func TestFinishFullSyncKeepsTheLinksItsPagesStored(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	old := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	if err := repo.UpsertPage(ctx, "p1", []backend.Issue{linkedIssue("PLAT-9", []backend.Link{blocks("1", "outward", "PLAT-1")})}, old, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	now := old.Add(time.Hour)
	if err := repo.UpsertPage(ctx, "p1", []backend.Issue{linkedIssue("PLAT-1", []backend.Link{blocks("2", "outward", "PLAT-2")})}, now, false); err != nil {
		t.Fatalf("page: %v", err)
	}
	if err := repo.FinishFullSync(ctx, "p1", now); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if links, _ := repo.ListLinks(ctx, "p1", "PLAT-1"); len(links) != 1 || links[0].ID != "2" {
		t.Errorf("synced issue's links = %+v, want the one its page stored", links)
	}
	if links, _ := repo.ListLinks(ctx, "p1", "PLAT-9"); len(links) != 0 {
		t.Errorf("the gone issue's links = %+v, want none", links)
	}
}
