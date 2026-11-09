package report

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"agile-suite/core/confluence"
	"agile-suite/core/store"
	"agile-suite/pie/internal/cases"
	"agile-suite/pie/internal/cycles"
	"agile-suite/pie/internal/saved"
)

var at = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

// space is a Confluence space in memory: pages by id, titles unique.
type space struct {
	pages map[string]*confluence.StoredPage
	fail  error
}

func newSpace() *space { return &space{pages: map[string]*confluence.StoredPage{}} }

func (s *space) GetPageStorage(_ context.Context, id string) (confluence.StoredPage, error) {
	p, ok := s.pages[id]
	if !ok {
		return confluence.StoredPage{}, &confluence.HTTPError{Code: 404, Status: "404 Not Found"}
	}
	return *p, nil
}

func (s *space) FindPageByTitle(_ context.Context, _, title string) (confluence.StoredPage, bool, error) {
	for _, p := range s.pages {
		if p.Title == title {
			return *p, true, nil
		}
	}
	return confluence.StoredPage{}, false, nil
}

func (s *space) CreatePage(_ context.Context, _, parentID, title, body string) (confluence.StoredPage, error) {
	if s.fail != nil {
		return confluence.StoredPage{}, s.fail
	}
	p := &confluence.StoredPage{ID: strconv.Itoa(len(s.pages) + 100), Title: title, Version: 1, Body: body, AncestorIDs: []string{parentID}}
	s.pages[p.ID] = p
	return *p, nil
}

func (s *space) UpdatePage(_ context.Context, id, title, body string, version int) (confluence.StoredPage, error) {
	p := s.pages[id]
	if version != p.Version+1 {
		return confluence.StoredPage{}, &confluence.HTTPError{Code: 409, Status: "409 Conflict"}
	}
	p.Title, p.Body, p.Version = title, body, version
	return *p, nil
}

func (s *space) AttachFile(context.Context, string, string, string, []byte) (confluence.Attachment, error) {
	return confluence.Attachment{}, errors.New("not used")
}

func setup(t *testing.T) (*cycles.Store, *saved.Store, int64) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pie.db"), saved.Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cs := cases.New(db.DB())
	if err := cs.Open("p1", []string{"ACME-1"}, at); err != nil {
		t.Fatal(err)
	}
	list, _ := cs.List("p1")
	if err := cs.AddProjects("p1", list[0].ID, []string{"ALPHA"}); err != nil {
		t.Fatal(err)
	}
	if err := cs.SetVerdict("p1", list[0].ID, "ALPHA", cases.Affected, "ships the <old> parser", at); err != nil {
		t.Fatal(err)
	}
	cy := cycles.New(db.DB(), cs)
	c, err := cy.Take("p1", "Week 45", at)
	if err != nil {
		t.Fatal(err)
	}
	return cy, saved.New(db.DB()), c.ID
}

func publish(cy *cycles.Store, sp *space, id int64, confirmed bool) (Outcome, error) {
	return Publish(context.Background(), sp, cy, Target{Profile: "p1", Space: "SEC", Parent: "7"}, id, confirmed, at)
}

func TestPublishWritesThePageAndMarksTheCyclePublished(t *testing.T) {
	cy, _, id := setup(t)
	sp := newSpace()
	got, err := publish(cy, sp, id, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ask != "" || got.Title != "Impact report: Week 45" || got.PageID == "" {
		t.Fatalf("outcome = %+v", got)
	}
	page := sp.pages[got.PageID]
	if page.AncestorIDs[0] != "7" || !strings.Contains(page.Body, "ships the &lt;old&gt; parser") {
		t.Fatalf("page = %+v", page)
	}
	d, _ := cy.Get("p1", id)
	if d.PublishedAt != "2026-11-09T10:00:00Z" {
		t.Fatalf("published at %q", d.PublishedAt)
	}

	again, err := publish(cy, sp, id, false)
	if err != nil || again.Ask != "" || again.PageID != got.PageID || len(sp.pages) != 1 || page.Version != 2 {
		t.Fatalf("publishing again = %+v, %v; %d pages, version %d", again, err, len(sp.pages), page.Version)
	}
}

func TestAFirstPublishAsksBeforeTakingOverAPageWithItsTitle(t *testing.T) {
	cy, _, id := setup(t)
	sp := newSpace()
	sp.pages["50"] = &confluence.StoredPage{ID: "50", Title: "Impact report: Week 45", Version: 3, Body: "someone's page"}
	got, err := publish(cy, sp, id, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Ask, "already exists in SEC") || sp.pages["50"].Body != "someone's page" {
		t.Fatalf("outcome = %+v, body %q", got, sp.pages["50"].Body)
	}
	if d, _ := cy.Get("p1", id); d.PublishedAt != "" {
		t.Fatal("a publish that only asked marked the cycle published")
	}
	got, err = publish(cy, sp, id, true)
	if err != nil || got.PageID != "50" || sp.pages["50"].Version != 4 {
		t.Fatalf("confirmed = %+v, %v", got, err)
	}
}

func TestPublishingAgainAsksWhenThePageWasEditedInConfluence(t *testing.T) {
	cy, _, id := setup(t)
	sp := newSpace()
	first, _ := publish(cy, sp, id, false)
	sp.pages[first.PageID].Version = 5
	sp.pages[first.PageID].Body = "an analyst's note"
	got, err := publish(cy, sp, id, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Ask, "edited") || sp.pages[first.PageID].Body != "an analyst's note" {
		t.Fatalf("outcome = %+v", got)
	}
	if got, err = publish(cy, sp, id, true); err != nil || sp.pages[first.PageID].Version != 6 {
		t.Fatalf("confirmed = %+v, %v", got, err)
	}
}

func TestAPageDeletedInConfluenceIsWrittenAgain(t *testing.T) {
	cy, _, id := setup(t)
	sp := newSpace()
	first, _ := publish(cy, sp, id, false)
	delete(sp.pages, first.PageID)
	got, err := publish(cy, sp, id, false)
	if err != nil || got.Ask != "" || sp.pages[got.PageID] == nil {
		t.Fatalf("outcome = %+v, %v", got, err)
	}
}

func TestAFailedPublishLeavesTheCycleUnpublished(t *testing.T) {
	cy, _, id := setup(t)
	sp := newSpace()
	sp.fail = &confluence.HTTPError{Code: 403, Status: "403 Forbidden", Message: "no permission"}
	_, err := publish(cy, sp, id, false)
	if err == nil || !strings.Contains(err.Error(), "no permission") {
		t.Fatalf("err = %v", err)
	}
	if d, _ := cy.Get("p1", id); d.PublishedAt != "" {
		t.Fatal("a failed publish marked the cycle published")
	}
}

func TestPurgeRemovesThePageRecord(t *testing.T) {
	cy, sv, id := setup(t)
	sp := newSpace()
	if _, err := publish(cy, sp, id, false); err != nil {
		t.Fatal(err)
	}
	if err := sv.Purge("p1"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := cy.Page("p1", id); ok || err != nil {
		t.Fatalf("purge left the page record: %v, %v", ok, err)
	}
}

func TestForgettingPagesSendsTheNextPublishThroughTheTitleCheck(t *testing.T) {
	cy, _, id := setup(t)
	sp := newSpace()
	first, _ := publish(cy, sp, id, false)
	if err := cy.ForgetPages("p1"); err != nil {
		t.Fatal(err)
	}
	got, err := publish(cy, sp, id, false)
	if err != nil || !strings.Contains(got.Ask, "already exists") || sp.pages[first.PageID].Version != 1 {
		t.Fatalf("after forgetting, publish = %+v, %v", got, err)
	}
}
