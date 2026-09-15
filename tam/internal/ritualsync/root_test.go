package ritualsync

import (
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/ritualtemplate"
)

func TestCreateRootCreatesAtTheTopOfTheSpace(t *testing.T) {
	h := newHarness(t)
	root, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", " PLAT Rituals ", false)
	if err != nil || root.Outcome != RootCreated || root.Title != "PLAT Rituals" || root.SpaceKey != "PLAT" || !root.TopLevel {
		t.Fatalf("root = %+v, %v", root, err)
	}
	page, ok := h.fake.Page(root.PageID)
	if !ok || len(page.AncestorIDs) != 0 || page.Body != ritualtemplate.RootBody("PLAT") || page.Title != "PLAT Rituals" {
		t.Fatalf("page = %+v", page)
	}
}

func TestCreateRootRefusesAnEmptyTitle(t *testing.T) {
	h := newHarness(t)
	if _, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "  ", false); err == nil || err.Error() != "The root page needs a title" {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateRootReportsATokenThatMayNotCreate(t *testing.T) {
	h := newHarness(t)
	h.fake.DenyCreate()
	root, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "PLAT Rituals", false)
	if err != nil || root.Outcome != RootForbidden || root.PageID != "" {
		t.Fatalf("root = %+v, %v", root, err)
	}
}

func TestCreateRootFindsATakenTitleAndSaysWhereItSits(t *testing.T) {
	h := newHarness(t)
	top := h.fake.Seed("", "PLAT Rituals", "<p>by hand</p>")
	root, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "PLAT Rituals", false)
	if err != nil || root.Outcome != RootTitleTaken || root.PageID != top || !root.TopLevel {
		t.Fatalf("top-level taken = %+v, %v", root, err)
	}
	nested := h.fake.Seed("root", "Nested rituals", "<p/>")
	root, err = CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "Nested rituals", false)
	if err != nil || root.Outcome != RootTitleTaken || root.PageID != nested || root.TopLevel {
		t.Fatalf("nested taken = %+v, %v", root, err)
	}
}

func TestCreateRootAdoptsOnlyATopLevelPage(t *testing.T) {
	h := newHarness(t)
	top := h.fake.Seed("", "PLAT Rituals", "<p>by hand</p>")
	root, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "PLAT Rituals", true)
	if err != nil || root.Outcome != RootAdopted || root.PageID != top || !root.TopLevel {
		t.Fatalf("adopted = %+v, %v", root, err)
	}
	nested := h.fake.Seed("root", "Nested rituals", "<p/>")
	root, err = CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "Nested rituals", true)
	if err != nil || root.Outcome != RootTitleTaken || root.PageID != nested || root.TopLevel {
		t.Fatalf("nested adoption = %+v, %v", root, err)
	}
	if _, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "Nobody wrote this", true); err == nil || !strings.HasPrefix(err.Error(), `No page titled "Nobody wrote this" is in PLAT any more`) {
		t.Fatalf("adopting a missing page = %v", err)
	}
}

func TestCreateRootPassesATransportFailureBack(t *testing.T) {
	h := newHarness(t)
	h.fake.FailNext("create", "PLAT Rituals", errors.New("dial tcp: connection refused"))
	if _, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "PLAT Rituals", false); err == nil || err.Error() != "dial tcp: connection refused" {
		t.Fatalf("err = %v", err)
	}
}
