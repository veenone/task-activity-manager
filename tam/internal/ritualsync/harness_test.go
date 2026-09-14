package ritualsync

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/demo"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
	"agile-suite/tam/internal/tamstore"
)

const (
	testProfile = "p1"
	testBoard   = 1
)

var sprint14 = Sprint{State: "active", Info: ritualtemplate.SprintInfo{
	ID: 14, Name: "Sprint 14", Goal: "Ship promo codes",
	StartDate: "2026-09-14T09:00:00.000+0000", EndDate: "2026-09-25T17:00:00.000+0000", BoardName: "PLAT board",
}}

type harness struct {
	t    *testing.T
	ctx  context.Context
	docs *ritualrepo.Repository
	fake *demo.Confluence
	cfg  Config
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &harness{
		t: t, ctx: context.Background(), docs: ritualrepo.New(db.DB()),
		fake: demo.NewConfluence("PLAT", "root", false),
		cfg: Config{SpaceKey: "PLAT", RootID: "root", Location: time.UTC,
			Now: func() time.Time { return time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC) }},
	}
}

func (h *harness) key(sprintID int, ritualType string) ritualrepo.Key {
	return ritualrepo.Key{ProfileID: testProfile, BoardID: testBoard, SprintID: sprintID, RitualType: ritualType}
}

func (h *harness) doc(sprintID int, ritualType string) ritualrepo.Document {
	h.t.Helper()
	d, ok, err := h.docs.Document(h.ctx, h.key(sprintID, ritualType))
	if err != nil || !ok {
		h.t.Fatalf("document %d %s: ok=%v err=%v", sprintID, ritualType, ok, err)
	}
	return d
}

func (h *harness) ensure(s Sprint) {
	h.t.Helper()
	if err := Ensure(h.ctx, h.docs, testProfile, testBoard, s, time.UTC, h.cfg.Now()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) save(sprintID int, ritualType, body string) {
	h.t.Helper()
	if _, err := h.docs.SaveBody(h.ctx, h.key(sprintID, ritualType), body, "t"); err != nil {
		h.t.Fatal(err)
	}
}
