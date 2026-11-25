package demo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

// linksOf finds key's links in a full search page, as the sync stores them.
func linksOf(t *testing.T, b *demobackend.Backend, key string) []backend.Link {
	t.Helper()
	page, _, err := b.SearchIssuesPage(context.Background(), "DEMO", "", "", 0, 1000)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, iss := range page {
		if iss.Key == key {
			return iss.Links
		}
	}
	t.Fatalf("%s not in the page", key)
	return nil
}

func find(links []backend.Link, typ, direction, key string) (backend.Link, bool) {
	for _, l := range links {
		if l.Type == typ && l.Direction == direction && l.Key == key {
			return l, true
		}
	}
	return backend.Link{}, false
}

// The demo's search carries each issue's links, as Jira's does once the sync
// asks for issuelinks (#612), and a link shows from both of its ends with
// one id, so the Dependencies view has a chain to draw in demo mode.
func TestDemoSearchCarriesEachLinkOnBothEnds(t *testing.T) {
	b := demobackend.New("DEMO")
	in, ok := find(linksOf(t, b, "DEMO-412"), "Blocks", "inward", "DEMO-409")
	if !ok || in.ID == "" || in.Summary != "Rotate payment gateway API keys" {
		t.Fatalf("DEMO-412 is blocked by DEMO-409: %+v %v", in, ok)
	}
	out, ok := find(linksOf(t, b, "DEMO-409"), "Blocks", "outward", "DEMO-412")
	if !ok || out.ID != in.ID {
		t.Errorf("DEMO-409 blocks DEMO-412 with the same id: %+v %v", out, ok)
	}
	if _, ok := find(linksOf(t, b, "DEMO-412"), "Relates", "outward", "DEMO-388"); !ok {
		t.Error("the curated detail links are carried too")
	}

	if err := b.CreateLink(context.Background(), "DEMO-390", backend.LinkDraft{Type: "Blocks", Direction: "outward", ToKey: "DEMO-398"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	made, ok := find(linksOf(t, b, "DEMO-398"), "Blocks", "inward", "DEMO-390")
	if !ok || made.ID == "" {
		t.Errorf("a created link shows from its far end with an id: %+v %v", made, ok)
	}
}

// GetIssue carries the links too, as Jira's issue read does once it asks
// for issuelinks, so Commit's read-back caches a link it just created.
func TestDemoGetIssueCarriesTheLinks(t *testing.T) {
	b := demobackend.New("DEMO")
	if err := b.CreateLink(context.Background(), "DEMO-401", backend.LinkDraft{Type: "Blocks", Direction: "outward", ToKey: "DEMO-409"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	iss, err := b.GetIssue(context.Background(), "DEMO-401")
	if err != nil {
		t.Fatal(err)
	}
	if l, ok := find(iss.Links, "Blocks", "outward", "DEMO-409"); !ok || l.ID == "" {
		t.Errorf("links = %+v, want the new Blocks link with an id", iss.Links)
	}
}
