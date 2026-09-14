package demo

import (
	"context"
	"errors"
	"testing"

	"agile-suite/core/confluence"
)

func TestTheDemoSpaceTracksAncestorsAndRefusesDuplicateTitles(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	sprint, err := c.CreatePage(ctx, "DEMO", "root", "Sprint 14", "<p/>")
	if err != nil || sprint.Version != 1 {
		t.Fatalf("create = %+v, %v", sprint, err)
	}
	child, _ := c.CreatePage(ctx, "DEMO", sprint.ID, "Sprint 14 · Planning", "<p>plan</p>")
	found, ok, err := c.FindPageByTitle(ctx, "DEMO", "Sprint 14 · Planning")
	if err != nil || !ok || found.ID != child.ID || len(found.AncestorIDs) != 2 || found.AncestorIDs[0] != "root" || found.AncestorIDs[1] != sprint.ID {
		t.Fatalf("found = %+v", found)
	}
	if _, err := c.CreatePage(ctx, "DEMO", "root", "Sprint 14", "<p/>"); err == nil {
		t.Fatal("a duplicate title should be refused")
	}
	if _, err := c.CreatePage(ctx, "DEMO", "nope", "Orphan", "<p/>"); !errors.Is(err, confluence.ErrNotFound) {
		t.Fatalf("missing parent = %v", err)
	}
}

func TestTheDemoSpaceEnforcesVersionPlusOne(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	p, _ := c.CreatePage(ctx, "DEMO", "root", "Sprint 14", "<p>v1</p>")
	if _, err := c.UpdatePage(ctx, p.ID, "Sprint 14", "<p>stale</p>", 1); !errors.Is(err, confluence.ErrVersionConflict) {
		t.Fatalf("stale update = %v", err)
	}
	u, err := c.UpdatePage(ctx, p.ID, "Sprint 14", "<p>v2</p>", 2)
	if err != nil || u.Version != 2 || u.Body != "<p>v2</p>" {
		t.Fatalf("update = %+v, %v", u, err)
	}
	c.Remove(p.ID)
	if _, err := c.GetPageStorage(ctx, p.ID); !errors.Is(err, confluence.ErrNotFound) {
		t.Fatalf("removed = %v", err)
	}
}

func TestFailNextAndAfterFireOnce(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	boom := errors.New("boom")
	c.FailNext("get", "root", boom)
	if _, err := c.GetPageStorage(ctx, "root"); !errors.Is(err, boom) {
		t.Fatalf("first get = %v", err)
	}
	if _, err := c.GetPageStorage(ctx, "root"); err != nil {
		t.Fatalf("second get = %v", err)
	}
	calls := 0
	c.After("get", "root", func() { calls++ })
	_, _ = c.GetPageStorage(ctx, "root")
	_, _ = c.GetPageStorage(ctx, "root")
	if calls != 1 {
		t.Fatalf("after ran %d times", calls)
	}
	findCalls := 0
	c.After("find", "Sprint 14", func() { findCalls++ })
	_, _, _ = c.FindPageByTitle(ctx, "DEMO", "Sprint 14")
	_, _, _ = c.FindPageByTitle(ctx, "DEMO", "Sprint 14")
	if findCalls != 1 {
		t.Fatalf("find after ran %d times", findCalls)
	}
}

func TestTheStagedConflictBumpsOnlyTheFirstStandup(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", true)
	first, _ := c.CreatePage(ctx, "DEMO", "root", "Sprint 14 · Standup", "<p>mine</p>")
	if first.Version != 1 {
		t.Fatalf("the create itself answers version 1, got %d", first.Version)
	}
	if p, _ := c.Page(first.ID); p.Version != 2 {
		t.Fatalf("the staged edit should leave version 2, got %d", p.Version)
	}
	second, _ := c.CreatePage(ctx, "DEMO", "root", "Sprint 15 · Standup", "<p>mine</p>")
	if p, _ := c.Page(second.ID); p.Version != 1 {
		t.Fatalf("only the first standup is staged, got %d", p.Version)
	}
}

func TestRestoreRebuildsAPageAndKeepsIdsUnique(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	c.Restore("1500", "root", "Sprint 14", "<p>kept</p>", 4)
	p, ok := c.Page("1500")
	if !ok || p.Version != 4 || p.Body != "<p>kept</p>" {
		t.Fatalf("restored = %+v", p)
	}
	next, _ := c.CreatePage(ctx, "DEMO", "root", "Sprint 15", "<p/>")
	if next.ID == "1500" {
		t.Fatal("a new page reused a restored id")
	}
}
