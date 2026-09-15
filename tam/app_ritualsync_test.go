package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualsync"
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
	d, err := a.SaveRitualBody(p.ID, 1, 14, "planning", "<p>mine</p>", 0, "")
	if err != nil || d.Body != "<p>mine</p>" || d.Status != ritualrepo.StatusLocal {
		t.Fatalf("saved = %+v, %v", d, err)
	}
	if _, err := a.SaveRitualBody(p.ID, 1, 14, "party", "<p/>", 0, ""); err == nil {
		t.Fatal("an unknown ritual type should be refused")
	}
	if err := a.ResolveRitualConflict(p.ID, 1, 14, "planning", "both"); err == nil {
		t.Fatal("an unknown choice should be refused")
	}
	if err := a.ResolveRitualConflict(p.ID, 1, 14, "planning", "mine"); err == nil {
		t.Fatal("a row with no conflict should be refused")
	}
}

func ritualDoc(t *testing.T, a *App, profileID, ritualType string) ritualrepo.Document {
	t.Helper()
	docs, err := a.ListRitualDocuments(profileID, 1, 14)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if d.RitualType == ritualType {
			return d
		}
	}
	t.Fatalf("no %s document", ritualType)
	return ritualrepo.Document{}
}

// An editor opened before a Sync created the page still holds version 0 and
// no page id; its save must be refused, not written over the synced page.
func TestSaveRitualBodyRefusesAnEditorOpenedBeforeASync(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	before := ritualDoc(t, a, p.ID, "planning")
	if _, err := a.SaveRitualBody(p.ID, 1, 14, "planning", "<p>stale</p>", 0, ""); !errors.Is(err, ritualrepo.ErrChangedUnderEditor) {
		t.Fatalf("err = %v", err)
	}
	if after := ritualDoc(t, a, p.ID, "planning"); after.Body != before.Body || after.Status != ritualrepo.StatusSynced {
		t.Fatalf("a refused save changed the row: %+v", after)
	}
	if d, err := a.SaveRitualBody(p.ID, 1, 14, "planning", "<p>current</p>", before.Version, before.PageID); err != nil || d.Status != ritualrepo.StatusUnsynced {
		t.Fatalf("current save = %+v, %v", d, err)
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
	planning := ritualDoc(t, a, p.ID, "planning")
	if _, err := a.SaveRitualBody(p.ID, 1, 14, "planning", "<p>after restart</p>", planning.Version, planning.PageID); err != nil {
		t.Fatal(err)
	}
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil || res.Gone != 0 || res.Pushed != 1 {
		t.Fatalf("after restart = %+v, %v", res, err)
	}
}

// A row in conflict knows the remote as its conflict body and version, not
// its base: an adopted page never had a base, so rebuilding from base_body
// put it back at version 0 with an empty body, and Keep mine then pushed
// against a version Confluence never had.
func TestADemoConflictIsRebuiltFromTheRemoteItKnows(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.EnsureSprintRituals(p.ID, 1, 14); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveRitualBody(p.ID, 1, 14, "_sprint", "<p>mine</p>", 0, ""); err != nil {
		t.Fatal(err)
	}
	space := a.demoSpace(p.ID, profile.ConfluenceConfig{SpaceKey: "DEMO", RootPageID: "demo-root"})
	id := space.Seed("demo-root", "Sprint 14", "<p>theirs</p>")
	if res, err := a.SyncRituals(p.ID, 1); err != nil || res.Conflicts != 1 {
		t.Fatalf("adopt = %+v, %v", res, err)
	}

	a.demoConfluence = nil
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	if page, ok := a.demoConfluence[p.ID].Page(id); !ok || page.Body != "<p>theirs</p>" || page.Version != 1 {
		t.Fatalf("rebuilt page = %+v, %v", page, ok)
	}
	if err := a.ResolveRitualConflict(p.ID, 1, 14, "_sprint", "mine"); err != nil {
		t.Fatal(err)
	}
	if res, err := a.SyncRituals(p.ID, 1); err != nil || res.Pushed != 1 || len(res.Failed) != 0 {
		t.Fatalf("after keep mine = %+v, %v", res, err)
	}
}

// TestAGonePageStaysGoneAfterARestart guards against demoSpace resurrecting a
// page a real Sync already marked gone: ProfileDocuments filters only on
// confluence_page_id being non-empty, and MarkGone never clears that column,
// so a naive rebuild restores the gone page under its old id and title the
// moment the app restarts, before anyone has forgotten it. ForgetRitualPage
// then clears the local page id, and the next Sync's place() does
// FindPageByTitle before creating: against the resurrected page still
// sitting in the rebuilt space, it adopts (or conflicts with) it instead of
// creating a genuinely new one. The rebuild has to run once while the row is
// still gone-with-a-page-id (the sync right after the restart) for the
// resurrection to land in the space at all; a Sync run only after
// ForgetRitualPage has already cleared the page id would never ask
// ProfileDocuments for this row in the first place, fix or no fix.
func TestAGonePageStaysGoneAfterARestart(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	docs, err := a.ListRitualDocuments(p.ID, 1, 14)
	if err != nil {
		t.Fatal(err)
	}
	var oldPageID string
	for _, d := range docs {
		if d.RitualType == "planning" {
			oldPageID = d.PageID
		}
	}
	if oldPageID == "" {
		t.Fatal("planning page was not created")
	}

	a.demoConfluence[p.ID].Remove(oldPageID)
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	docs, err = a.ListRitualDocuments(p.ID, 1, 14)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if d.RitualType == "planning" && d.Status != ritualrepo.StatusGone {
			t.Fatalf("planning status = %q, want gone", d.Status)
		}
	}

	// Restart: the in-memory space is gone, but the gone row (page id still
	// set) is not. The very next Sync is what rebuilds the space, and it has
	// to run before the row is forgotten for a naive rebuild to resurrect it.
	a.demoConfluence = nil
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}

	if err := a.ForgetRitualPage(p.ID, 1, 14, "planning"); err != nil {
		t.Fatal(err)
	}
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil || res.Created != 1 {
		t.Fatalf("result = %+v, %v", res, err)
	}
	docs, err = a.ListRitualDocuments(p.ID, 1, 14)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if d.RitualType == "planning" && d.PageID == oldPageID {
			t.Fatalf("planning page id = %q, want a new id, not the resurrected old one", d.PageID)
		}
	}
}

// newMissingRootApp is newRitualSyncApp pointed at the demo space's staged
// missing root, the configuration a stale root page id leaves behind.
func newMissingRootApp(t *testing.T) (*App, profile.Profile) {
	t.Helper()
	a, p := newRitualSyncApp(t)
	if err := a.profiles.SetConfluenceConfig(p.ID, profile.ConfluenceConfig{BaseURL: "demo", SpaceKey: "DEMO", RootPageID: demo.StagedMissingRootID}); err != nil {
		t.Fatal(err)
	}
	return a, p
}

func storedRoot(t *testing.T, a *App, profileID string) profile.ConfluenceConfig {
	t.Helper()
	c, err := a.profiles.ConfluenceConfig(profileID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSyncRitualsReportsAMissingRootWithoutRecordingASync(t *testing.T) {
	a, p := newMissingRootApp(t)
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := ritualsync.RootMissing{PageID: demo.StagedMissingRootID, SpaceKey: "DEMO", CanCreate: true, SuggestedTitle: "PLAT Rituals"}
	if res.RootMissing == nil || *res.RootMissing != want {
		t.Fatalf("root missing = %+v", res.RootMissing)
	}
	if last, _ := a.LastRitualSync(p.ID, 1); last != "" {
		t.Fatalf("a pass that found no root recorded a sync at %q", last)
	}
	if _, ok := a.busy[p.ID]; ok {
		t.Fatal("the lock was not released")
	}
}

func TestCreateRitualRootSavesTheNewRootAndSyncsUnderIt(t *testing.T) {
	a, p := newMissingRootApp(t)
	out, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false)
	if err != nil || out.Root.Outcome != ritualsync.RootCreated || out.Sync == nil || out.Sync.Created != 5 || out.SyncError != "" {
		t.Fatalf("out = %+v, %v", out, err)
	}
	stored := storedRoot(t, a, p.ID)
	if stored.RootPageID != out.Root.PageID || stored.BaseURL != "demo" || stored.SpaceKey != "DEMO" {
		t.Fatalf("stored = %+v", stored)
	}
	overview := ritualDoc(t, a, p.ID, "_sprint")
	page, ok := a.demoSpace(p.ID, stored).Page(overview.PageID)
	if !ok || len(page.AncestorIDs) != 1 || page.AncestorIDs[0] != out.Root.PageID {
		t.Fatalf("overview page = %+v", page)
	}
	if last, _ := a.LastRitualSync(p.ID, 1); last == "" {
		t.Fatal("the Sync after the create should record its time")
	}
	again, err := a.SyncRituals(p.ID, 1)
	if err != nil || again.RootMissing != nil || again.Created != 0 {
		t.Fatalf("second sync = %+v, %v", again, err)
	}
	if _, ok := a.busy[p.ID]; ok {
		t.Fatal("the lock was not released")
	}
}

// A Sync that fails after the root was created must not hide the create: the
// root is saved by then, so the failure travels as SyncError beside it.
func TestCreateRitualRootReportsAFailedSyncBesideTheSavedRoot(t *testing.T) {
	a, p := newMissingRootApp(t)
	space := a.demoSpace(p.ID, storedRoot(t, a, p.ID))
	space.After("create", "PLAT Rituals", func() {
		created, _, _ := space.FindPageByTitle(context.Background(), "DEMO", "PLAT Rituals")
		space.FailNext("get", created.ID, errors.New("503 Service Unavailable\n<html><body>down</body></html>"))
	})
	out, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false)
	if err != nil || out.Root.Outcome != ritualsync.RootCreated || out.Sync != nil {
		t.Fatalf("out = %+v, %v", out, err)
	}
	if out.SyncError == "" || strings.Contains(out.SyncError, "\n") {
		t.Fatalf("sync error = %q, want one line", out.SyncError)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != out.Root.PageID {
		t.Fatalf("root page id = %q, want %q", got, out.Root.PageID)
	}
	if _, ok := a.busy[p.ID]; ok {
		t.Fatal("the lock was not released")
	}
}

func TestCreateRitualRootLeavesTheProfileAloneWhenTheTokenMayNotCreate(t *testing.T) {
	a, p := newMissingRootApp(t)
	a.demoSpace(p.ID, storedRoot(t, a, p.ID)).DenyCreate()
	out, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false)
	if err != nil || out.Root.Outcome != ritualsync.RootForbidden || out.Sync != nil {
		t.Fatalf("out = %+v, %v", out, err)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != demo.StagedMissingRootID {
		t.Fatalf("root page id = %q", got)
	}
}

func TestCreateRitualRootAdoptsATopLevelPageOnlyWhenAsked(t *testing.T) {
	a, p := newMissingRootApp(t)
	existing := a.demoSpace(p.ID, storedRoot(t, a, p.ID)).Seed("", "PLAT Rituals", "<p>by hand</p>")
	out, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false)
	if err != nil || out.Root.Outcome != ritualsync.RootTitleTaken || out.Root.PageID != existing || !out.Root.TopLevel || out.Sync != nil {
		t.Fatalf("taken = %+v, %v", out, err)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != demo.StagedMissingRootID {
		t.Fatalf("a taken title saved root %q", got)
	}
	out, err = a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", true)
	if err != nil || out.Root.Outcome != ritualsync.RootAdopted || out.Sync == nil || out.Sync.Created != 5 {
		t.Fatalf("adopted = %+v, %v", out, err)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != existing {
		t.Fatalf("root page id = %q, want %q", got, existing)
	}
}

func TestCreateRitualRootIsRefusedWhileAnotherOperationHoldsTheLock(t *testing.T) {
	a, p := newMissingRootApp(t)
	a.busy[p.ID] = "sync"
	if _, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false); err == nil || err.Error() != "a sync is already running for this profile" {
		t.Fatalf("err = %v", err)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != demo.StagedMissingRootID {
		t.Fatalf("a refused call saved root %q", got)
	}
}
