package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func findEdge(g issuerepo.DependencyGraph, typ, outward, inward string) (issuerepo.DependencyLink, bool) {
	for _, l := range g.Links {
		if l.Type == typ && l.Outward == outward && l.Inward == inward {
			return l, true
		}
	}
	return issuerepo.DependencyLink{}, false
}

// The graph is every cached link once, however many ends cache it, plus the
// links the journal holds, and the issues they join (#612).
func TestDependencyGraphDrawsEachLinkOnceWithItsIssues(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	page := []backend.Issue{
		linkedIssue("PLAT-1", []backend.Link{blocks("10100", "outward", "PLAT-2"), {ID: "10300", Direction: "inward", Type: "Blocks", Key: "OPS-9", Summary: "Firewall rule", IssueType: "Task"}}),
		linkedIssue("PLAT-2", []backend.Link{blocks("10100", "inward", "PLAT-1")}),
		linkedIssue("PLAT-3", []backend.Link{}),
		linkedIssue("PLAT-4", []backend.Link{}),
	}
	if err := repo.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.AddLink(ctx, "p1", "PLAT-3", backend.LinkDraft{Type: "Blocks", Direction: "inward", ToKey: "PLAT-2"}); err != nil {
		t.Fatalf("AddLink: %v", err)
	}
	if err := repo.RemoveLink(ctx, "p1", "PLAT-2", blocks("10100", "inward", "PLAT-1")); err != nil {
		t.Fatalf("RemoveLink: %v", err)
	}
	g, err := repo.DependencyGraph(ctx, "p1")
	if err != nil {
		t.Fatalf("DependencyGraph: %v", err)
	}
	if len(g.Links) != 3 {
		t.Fatalf("links = %+v, want three", g.Links)
	}
	cached, ok := findEdge(g, "Blocks", "PLAT-1", "PLAT-2")
	if !ok || cached.ID != "10100" || !cached.Pending || !cached.Removal || cached.PendingID == 0 {
		t.Errorf("the cached link, removal pending: %+v %v", cached, ok)
	}
	added, ok := findEdge(g, "Blocks", "PLAT-2", "PLAT-3")
	if !ok || !added.Pending || added.Removal || added.ID != "" || added.Owner != "PLAT-3" || added.Direction != "inward" {
		t.Errorf("the pending add, drawn outward to inward: %+v %v", added, ok)
	}
	if ext, ok := findEdge(g, "Blocks", "OPS-9", "PLAT-1"); !ok || ext.Pending {
		t.Errorf("the link to another project: %+v %v", ext, ok)
	}
	keys := map[string]bool{}
	for _, iss := range g.Issues {
		keys[iss.Key] = true
	}
	if len(keys) != 3 || !keys["PLAT-1"] || !keys["PLAT-2"] || !keys["PLAT-3"] {
		t.Errorf("issues = %v, want the three linked cached ones and not PLAT-4", keys)
	}
	if len(g.External) != 1 || g.External[0].Key != "OPS-9" || g.External[0].Summary != "Firewall rule" {
		t.Errorf("external = %+v, want OPS-9 from the link row", g.External)
	}
}
