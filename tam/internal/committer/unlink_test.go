package committer_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

func blocksLink(id, direction, to string) backend.Link {
	return backend.Link{ID: id, Direction: direction, Type: "Blocks", Key: to}
}

// A journaled removal reaches Jira on Commit as a delete by the link's id
// (#612), and the link leaves both ends of the cache once Jira took it.
func TestCommitDeletesARemovedLinkByItsIDAndDropsTheCachedRows(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Links: []backend.Link{blocksLink("10100", "outward", "PLAT-2")}}, time.Now())
	_ = repo.WriteDetail(ctx, "p1", "PLAT-2", backend.IssueDetail{Links: []backend.Link{blocksLink("10100", "inward", "PLAT-1")}}, time.Now())
	if err := repo.RemoveLink(ctx, "p1", "PLAT-1", blocksLink("10100", "outward", "PLAT-2")); err != nil {
		t.Fatalf("RemoveLink: %v", err)
	}
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.unlinked, ",") != "10100" || len(f.links) != 0 {
		t.Errorf("deleted %v, created %v; want only the delete of 10100", f.unlinked, f.links)
	}
	if len(res.Linked) != 1 || !res.Linked[0].Removed || res.Linked[0].ToKey != "PLAT-2" || res.Remaining != 0 || len(res.Failures) != 0 {
		t.Errorf("result: %+v", res)
	}
	for _, key := range []string{"PLAT-1", "PLAT-2"} {
		if links, _ := repo.ListLinks(ctx, "p1", key); len(links) != 0 {
			t.Errorf("%s still caches %+v", key, links)
		}
	}
}

// A retype deletes the old link before it creates the new one, and the
// source is read back from Jira so the new link is cached with its id.
func TestCommitRetypeDeletesThenCreatesAndCachesTheNewLink(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Links: []backend.Link{blocksLink("10100", "outward", "PLAT-2")}}, time.Now())
	if err := repo.RetypeLink(ctx, "p1", "PLAT-1", blocksLink("10100", "outward", "PLAT-2"), "Relates"); err != nil {
		t.Fatalf("RetypeLink: %v", err)
	}
	// What Jira holds once the commit lands, which the refresh reads.
	row := f.rows["PLAT-1"]
	row.Links = []backend.Link{{ID: "10200", Direction: "outward", Type: "Relates", Key: "PLAT-2"}}
	f.rows["PLAT-1"] = row
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.unlinked, ",") != "10100" || strings.Join(f.links, ",") != "PLAT-1 outward Relates PLAT-2" || len(res.Linked) != 2 {
		t.Errorf("deleted %v, created %v, result %+v", f.unlinked, f.links, res)
	}
	links, _ := repo.ListLinks(ctx, "p1", "PLAT-1")
	if len(links) != 1 || links[0].ID != "10200" || links[0].Type != "Relates" {
		t.Errorf("PLAT-1 caches %+v, want the new Relates link with its id", links)
	}
}
