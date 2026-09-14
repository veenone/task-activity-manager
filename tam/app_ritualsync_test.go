package main

import (
	"strings"
	"testing"

	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/ritualrepo"
)

// newRitualSyncApp is a non-demo Jira profile whose Confluence URL is "demo",
// which is what routes its pages to the in-memory space, with one scrum board
// and one active sprint in the cache.
func newRitualSyncApp(t *testing.T) (*App, profile.Profile) {
	t.Helper()
	a, p := newTestAppWithRituals(t)
	if err := a.profiles.SetConfluenceConfig(p.ID, profile.ConfluenceConfig{BaseURL: "demo", SpaceKey: "DEMO", RootPageID: "demo-root"}); err != nil {
		t.Fatal(err)
	}
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT board", Type: "scrum"}, nil,
		[]backend.Sprint{{ID: 14, BoardID: 1, Name: "Sprint 14", State: "active",
			StartDate: "2026-09-14T09:00:00.000+0000", EndDate: "2026-09-25T17:00:00.000+0000"}}, nil); err != nil {
		t.Fatal(err)
	}
	return a, p
}

func TestEnsureSprintRitualsWritesFiveLocalPagesWithoutConfluence(t *testing.T) {
	a, p := newRitualSyncApp(t)
	docs, err := a.EnsureSprintRituals(p.ID, 1, 14)
	if err != nil || len(docs) != 5 {
		t.Fatalf("docs = %d, %v", len(docs), err)
	}
	for _, d := range docs {
		if d.Status != ritualrepo.StatusLocal {
			t.Errorf("%s = %s", d.RitualType, d.Status)
		}
	}
	if len(a.demoConfluence) != 0 {
		t.Fatal("ensuring pages must not touch a Confluence transport")
	}
	if _, err := a.EnsureSprintRituals(p.ID, 1, 99); err == nil {
		t.Fatal("a sprint the cache does not hold should be refused")
	}
}

func TestSyncRitualsCreatesTheTreeAndRecordsWhen(t *testing.T) {
	a, p := newRitualSyncApp(t)
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil || res.Created != 5 {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if last, err := a.LastRitualSync(p.ID, 1); err != nil || last != res.SyncedAt {
		t.Fatalf("last sync = %q, %v", last, err)
	}
	if _, ok := a.busy[p.ID]; ok {
		t.Fatal("the lock was not released")
	}
}

func TestSyncRitualsIsRefusedWhileAnotherOperationHoldsTheLock(t *testing.T) {
	a, p := newRitualSyncApp(t)
	a.busy[p.ID] = "sync"
	if _, err := a.SyncRituals(p.ID, 1); err == nil || err.Error() != "a sync is already running for this profile" {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncRitualsIsRefusedWithoutConfluence(t *testing.T) {
	a, p := newTestAppWithRituals(t)
	if _, err := a.SyncRituals(p.ID, 1); err == nil || err.Error() != "Confluence is not configured for this profile" {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveAndResolveThroughTheBindings(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.EnsureSprintRituals(p.ID, 1, 14); err != nil {
		t.Fatal(err)
	}
	d, err := a.SaveRitualBody(p.ID, 1, 14, "planning", "<p>mine</p>")
	if err != nil || d.Body != "<p>mine</p>" || d.Status != ritualrepo.StatusLocal {
		t.Fatalf("saved = %+v, %v", d, err)
	}
	if _, err := a.SaveRitualBody(p.ID, 1, 14, "party", "<p/>"); err == nil {
		t.Fatal("an unknown ritual type should be refused")
	}
	if err := a.ResolveRitualConflict(p.ID, 1, 14, "planning", "both"); err == nil {
		t.Fatal("an unknown choice should be refused")
	}
	if err := a.ResolveRitualConflict(p.ID, 1, 14, "planning", "mine"); err == nil {
		t.Fatal("a row with no conflict should be refused")
	}
}

func TestRitualMacroIssuesPreviewsTheThreeFormsFromTheCache(t *testing.T) {
	a, p := newRitualSyncApp(t)
	seedSprintIssues(t, a, p.ID, 12) // To Do, In Progress, Done, Blocked, all in sprint 12
	for jql, want := range map[string]int{
		"sprint = 12 ORDER BY Rank":              4,
		"sprint = 12 AND statusCategory = Done":  1,
		"sprint = 12 AND statusCategory != Done": 3,
	} {
		got, err := a.RitualMacroIssues(p.ID, jql)
		if err != nil || !got.Supported || len(got.Issues) != want {
			t.Errorf("%s = %d issues, supported %v, %v", jql, len(got.Issues), got.Supported, err)
		}
	}
	if got, _ := a.RitualMacroIssues(p.ID, "project = PLAT"); got.Supported || len(got.Issues) != 0 {
		t.Fatalf("other JQL = %+v", got)
	}
}

func TestStandupEntryReadsADayInput(t *testing.T) {
	a, _ := newRitualSyncApp(t)
	entry, err := a.StandupEntry("2026-09-15")
	if err != nil || !strings.HasPrefix(entry, "<h3>Tue 15 Sep 2026</h3>") {
		t.Fatalf("entry = %q, %v", entry, err)
	}
	if _, err := a.StandupEntry("tomorrow"); err == nil {
		t.Fatal("a malformed day should be refused")
	}
}

// The demo space lives in memory, so a restart empties it. Without rebuilding
// it from the pages the store already knows, every demo page would read as
// gone after reopening the app.
func TestDemoPagesSurviveARestart(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	a.demoConfluence = nil
	if _, err := a.SaveRitualBody(p.ID, 1, 14, "planning", "<p>after restart</p>"); err != nil {
		t.Fatal(err)
	}
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil || res.Gone != 0 || res.Pushed != 1 {
		t.Fatalf("after restart = %+v, %v", res, err)
	}
}
