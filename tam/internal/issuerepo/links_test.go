package issuerepo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func relates(to string) backend.LinkDraft {
	return backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: to, ToSummary: "Summary of " + to, ToType: "Test"}
}

func TestAddLinkJournalsAndMergesIntoTheDetail(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "d", Links: []backend.Link{{Direction: "inward", Type: "Tested By", Key: "XT-1", Summary: "Existing", IssueType: "Test"}}}, time.Now())
	if err := repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-9")); err != nil {
		t.Fatalf("AddLink: %v", err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(pend) != 1 || pend[0].EntityType != issuerepo.EntityLink || pend[0].Field != "Relates|outward|XT-9" || pend[0].BaseVersion != "" {
		t.Fatalf("journal row: %+v", pend)
	}
	links, err := repo.PendingLinks(ctx, "p1", "PLAT-1")
	if err != nil || len(links) != 1 || !links[0].Pending || links[0].PendingID != pend[0].ID || links[0].Key != "XT-9" || links[0].Summary != "Summary of XT-9" || links[0].IssueType != "Test" {
		t.Errorf("PendingLinks: %+v %v", links, err)
	}
	d, _, ok, err := repo.ReadDetail(ctx, "p1", "PLAT-1")
	if err != nil || !ok || len(d.Links) != 2 || d.Links[0].Pending || !d.Links[1].Pending {
		t.Errorf("detail merges pending links after the cached ones: %+v %v %v", d.Links, ok, err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !iss.Pending {
		t.Error("a pending link marks the row")
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 1 || act[0].Action != "link" || act[0].AfterVal != "XT-9" || act[0].Field != "Relates|outward|XT-9" {
		t.Errorf("audit: %+v", act)
	}
}

func TestAddLinkRefusesDuplicatesAndBadInput(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Links: []backend.Link{{Direction: "outward", Type: "Relates", Key: "XT-1"}}}, time.Now())
	cases := map[string]backend.LinkDraft{
		"already linked": relates("XT-1"),
		"itself":         relates("PLAT-1"),
		"empty target":   relates(""),
		"bad direction":  {Type: "Relates", Direction: "sideways", ToKey: "XT-2"},
		"empty type":     {Direction: "outward", ToKey: "XT-2"},
	}
	for name, d := range cases {
		if err := repo.AddLink(ctx, "p1", "PLAT-1", d); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if err := repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-2")); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-2")); err == nil || !strings.Contains(err.Error(), "already pending") {
		t.Errorf("duplicate pending: %v", err)
	}
	if err := repo.AddLink(ctx, "p1", "PLAT-9", relates("XT-2")); err == nil {
		t.Error("unknown source issue must be refused")
	}
}

func TestDiscardingALinkDropsTheRowOnly(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine")
	_ = repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-2"))
	links, _ := repo.PendingLinks(ctx, "p1", "PLAT-1")
	if err := repo.DiscardPendingChange(ctx, "p1", links[0].PendingID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Summary != "mine" || !iss.Pending {
		t.Errorf("the edit is untouched: %+v", iss)
	}
	if rest, _ := repo.PendingLinks(ctx, "p1", "PLAT-1"); len(rest) != 0 {
		t.Errorf("link gone: %+v", rest)
	}
	_ = repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-3"))
	if n, err := repo.DiscardAllPendingChanges(ctx, "p1"); err != nil || n != 2 {
		t.Errorf("discard all counts the link: %d %v", n, err)
	}
}

func TestWriteDetailRefreshLeavesThePendingLinkInPlace(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "d", Links: []backend.Link{{Direction: "inward", Type: "Tested By", Key: "XT-1", Summary: "Existing", IssueType: "Test"}}}, time.Now())
	if err := repo.AddLink(ctx, "p1", "PLAT-1", relates("XT-9")); err != nil {
		t.Fatalf("AddLink: %v", err)
	}
	// A refresh (what a re-fetch after a miss or a stale cache does) must not
	// drop the link the journal still holds.
	if err := repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "d2", Links: []backend.Link{{Direction: "inward", Type: "Tested By", Key: "XT-1", Summary: "Existing", IssueType: "Test"}}}, time.Now()); err != nil {
		t.Fatalf("WriteDetail refresh: %v", err)
	}
	d, _, ok, err := repo.ReadDetail(ctx, "p1", "PLAT-1")
	if err != nil || !ok || len(d.Links) != 2 || d.Links[0].Pending || !d.Links[1].Pending || d.Links[1].Key != "XT-9" {
		t.Errorf("the pending link survives a refresh, after the cached ones: %+v %v %v", d.Links, ok, err)
	}
}

func TestClearDetailDropsTheCache(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "d"}, time.Now())
	if err := repo.ClearDetail(ctx, "p1", "PLAT-1"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, _ := repo.ReadDetail(ctx, "p1", "PLAT-1"); ok {
		t.Error("detail must be gone")
	}
}
