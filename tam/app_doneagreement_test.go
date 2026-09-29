package main

import (
	"strings"
	"testing"

	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

// seedBoardAgreement writes the board's standing agreement straight through
// the repository, so the tests below stand on the store rather than on the
// binding that normally writes it.
func seedBoardAgreement(t *testing.T, a *App, profileID string) {
	t.Helper()
	k := ritualrepo.Key{ProfileID: profileID, BoardID: 1, RitualType: ritualtemplate.DoneAgreement}
	if err := a.rituals.WriteTemplate(a.ctx, k, "PLAT board · Done agreement", "<p>what we agreed</p>", "t"); err != nil {
		t.Fatal(err)
	}
}

func boardAgreement(t *testing.T, a *App, profileID string) ritualrepo.Document {
	t.Helper()
	docs, err := a.ListRitualDocuments(profileID, 1, 14)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if d.RitualType == ritualtemplate.DoneAgreement && d.SprintID == 0 {
			return d
		}
	}
	t.Fatal("the sprint's documents do not carry the board's done agreement")
	return ritualrepo.Document{}
}

// The board's agreement has no sprint of its own, so a view that asks for a
// sprint's documents would never see it. It travels with them instead, which
// is what lets the Rituals view, the editor and the conflict banners reach it
// through the paths they already use.
func TestASprintsDocumentsCarryTheBoardsDoneAgreement(t *testing.T) {
	a, p := newRitualSyncApp(t)
	seedBoardAgreement(t, a, p.ID)
	docs, err := a.EnsureSprintRituals(p.ID, 1, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 6 {
		t.Fatalf("documents = %d, want the sprint's five and the board's agreement", len(docs))
	}
	if d := boardAgreement(t, a, p.ID); d.Body != "<p>what we agreed</p>" {
		t.Fatalf("board agreement = %+v", d)
	}
}

// Both documents are created only when asked for: sprint id 0 is the board's
// standing agreement and a sprint's own id is that sprint's additions.
func TestCreateDoneAgreementWritesEitherDocumentOnRequest(t *testing.T) {
	a, p := newRitualSyncApp(t)
	d, err := a.CreateDoneAgreement(p.ID, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d.SprintID != 0 || d.RitualType != ritualtemplate.DoneAgreement ||
		d.Title != "PLAT board · Done agreement" || d.Status != ritualrepo.StatusLocal {
		t.Fatalf("board agreement = %+v", d)
	}
	if !strings.Contains(d.Body, "<ac:task-list>") {
		t.Errorf("the items are not a task list: %s", d.Body)
	}

	// The view has opened the sprint before it can ask for its additions, so
	// its five pages are already written by then.
	if _, err := a.EnsureSprintRituals(p.ID, 1, 14); err != nil {
		t.Fatal(err)
	}
	additions, err := a.CreateDoneAgreement(p.ID, 1, 14)
	if err != nil {
		t.Fatal(err)
	}
	if additions.SprintID != 14 || additions.Title != "Sprint 14 · Done agreement" {
		t.Fatalf("additions = %+v", additions)
	}
	docs, err := a.ListRitualDocuments(p.ID, 1, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 7 {
		t.Fatalf("documents = %d, want the sprint's five, its additions and the board's agreement", len(docs))
	}
	if _, err := a.CreateDoneAgreement(p.ID, 1, 99); err == nil {
		t.Fatal("a sprint the cache does not hold should be refused")
	}
}

// The demo space is rebuilt from stored pages by hanging each document under
// its sprint's overview page. A document with no sprint has no overview, so
// it was dropped from the rebuild and the first Sync after a restart called
// the page gone.
func TestABoardsDoneAgreementSurvivesADemoRestart(t *testing.T) {
	a, p := newRitualSyncApp(t)
	seedBoardAgreement(t, a, p.ID)
	if res, err := a.SyncRituals(p.ID, 1); err != nil || res.Created != 6 {
		t.Fatalf("first sync = %+v, %v", res, err)
	}
	a.demoConfluence = nil
	d := boardAgreement(t, a, p.ID)
	if _, err := a.SaveRitualBody(p.ID, 1, 0, ritualtemplate.DoneAgreement, "<p>after restart</p>", d.Version, d.PageID); err != nil {
		t.Fatal(err)
	}
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil || res.Gone != 0 || res.Pushed != 1 {
		t.Fatalf("after restart = %+v, %v", res, err)
	}
	if got := boardAgreement(t, a, p.ID); got.Status != ritualrepo.StatusSynced {
		t.Fatalf("board agreement = %+v", got)
	}
}
