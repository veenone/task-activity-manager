package ritualsync

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"agile-suite/core/confluence"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

func (h *harness) run(sprints ...Sprint) Result {
	h.t.Helper()
	res, err := Run(h.ctx, h.fake, h.docs, h.cfg, testProfile, testBoard, sprints)
	if err != nil {
		h.t.Fatalf("run: %v", err)
	}
	return res
}

func TestFirstSyncCreatesTheSprintPageThenItsRituals(t *testing.T) {
	h := newHarness(t)
	res := h.run(sprint14)
	if res.Created != 5 || len(res.Failed) != 0 || res.SyncedAt == "" {
		t.Fatalf("result = %+v", res)
	}
	overview := h.doc(14, ritualtemplate.Sprint)
	page, _ := h.fake.Page(overview.PageID)
	if page.Title != "Sprint 14" || len(page.AncestorIDs) != 1 || page.AncestorIDs[0] != "root" {
		t.Fatalf("overview page = %+v", page)
	}
	for _, typ := range ritualtemplate.Types[1:] {
		d := h.doc(14, typ)
		p, ok := h.fake.Page(d.PageID)
		if !ok || p.AncestorIDs[len(p.AncestorIDs)-1] != overview.PageID {
			t.Errorf("%s is not under its sprint page: %+v", typ, p)
		}
		if d.Status != ritualrepo.StatusSynced || d.Version != 1 || d.BaseBody != d.Body {
			t.Errorf("%s = %+v", typ, d)
		}
	}
}

func TestAPageAlreadyUnderTheRootIsAdoptedNotDuplicated(t *testing.T) {
	h := newHarness(t)
	id := h.fake.Seed("root", "Sprint 14", "<p>written by hand</p>")
	res := h.run(sprint14)
	if res.Created != 4 || res.Pulled != 1 {
		t.Fatalf("result = %+v", res)
	}
	d := h.doc(14, ritualtemplate.Sprint)
	if d.PageID != id || d.Body != "<p>written by hand</p>" || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("overview = %+v", d)
	}
}

func TestAdoptingOverLocalEditsIsAConflict(t *testing.T) {
	h := newHarness(t)
	h.ensure(sprint14)
	h.save(14, ritualtemplate.Sprint, "<p>mine</p>")
	id := h.fake.Seed("root", "Sprint 14", "<p>theirs</p>")
	res := h.run(sprint14)
	d := h.doc(14, ritualtemplate.Sprint)
	if res.Conflicts != 1 || d.Status != ritualrepo.StatusConflict || d.PageID != id ||
		d.Body != "<p>mine</p>" || d.ConflictBody != "<p>theirs</p>" {
		t.Fatalf("result = %+v, overview = %+v", res, d)
	}
	// The rituals still land under the adopted page.
	if res.Created != 4 {
		t.Fatalf("created = %d", res.Created)
	}
}

func TestATitleTakenOutsideTheRootIsRefusedAndItsRitualsWait(t *testing.T) {
	h := newHarness(t)
	h.fake.Seed("", "Sprint 14", "<p>another team</p>")
	res := h.run(sprint14)
	if res.Created != 0 || len(res.Failed) != 1 {
		t.Fatalf("result = %+v", res)
	}
	want := `A page titled "Sprint 14" already exists outside the rituals root. Rename one of them.`
	if f := res.Failed[0]; f.Reason != want || f.Title != "Sprint 14" || f.SprintName != "Sprint 14" {
		t.Fatalf("failure = %+v", f)
	}
	if d := h.doc(14, "planning"); d.PageID != "" {
		t.Fatalf("a ritual was placed without its sprint page: %+v", d)
	}
}

func TestClosedSprintsGetNoNewPages(t *testing.T) {
	h := newHarness(t)
	closed := sprint14
	closed.State = "closed"
	if res := h.run(closed); res.Created != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestAnUnreadableRootRefusesThePass(t *testing.T) {
	h := newHarness(t)
	h.fake.FailNext("get", "root", &confluence.HTTPError{Code: http.StatusForbidden, Status: "403 Forbidden"})
	_, err := Run(h.ctx, h.fake, h.docs, h.cfg, testProfile, testBoard, []Sprint{sprint14})
	if err == nil || !strings.HasPrefix(err.Error(), "The Confluence root page root could not be read: ") {
		t.Fatalf("err = %v", err)
	}
	if docs, _ := h.docs.Documents(h.ctx, testProfile, testBoard, 14); len(docs) != 0 {
		t.Fatal("a refused pass should not have started")
	}
}

func TestAFailedCreateIsReportedAndTheRestContinue(t *testing.T) {
	h := newHarness(t)
	h.fake.FailNext("create", "Sprint 14 · Review", errors.New("boom"))
	res := h.run(sprint14)
	if res.Created != 4 || len(res.Failed) != 1 || res.Failed[0].Reason != "boom" || res.Failed[0].Title != "Sprint 14 · Review" {
		t.Fatalf("result = %+v", res)
	}
	if d := h.doc(14, "review"); d.PageID != "" || d.Status != ritualrepo.StatusLocal {
		t.Fatalf("review = %+v", d)
	}
}

func TestAnUntouchedSyncedPageIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	res := h.run(sprint14)
	if res.Created+res.Pulled+res.Pushed+res.Conflicts+res.Gone != 0 || len(res.Failed) != 0 {
		t.Fatalf("second pass = %+v", res)
	}
}

func TestALocalEditIsPushedAtTheNextVersion(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	h.save(14, "planning", "<p>plan</p>")
	res := h.run(sprint14)
	d := h.doc(14, "planning")
	page, _ := h.fake.Page(d.PageID)
	if res.Pushed != 1 || page.Body != "<p>plan</p>" || page.Version != 2 ||
		d.Version != 2 || d.BaseBody != "<p>plan</p>" || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("result = %+v, doc = %+v, page = %+v", res, d, page)
	}
}

func TestARemoteEditIsPulledIntoACleanPage(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "retro").PageID
	h.fake.EditRemote(id, "<p>theirs</p>")
	res := h.run(sprint14)
	if d := h.doc(14, "retro"); res.Pulled != 1 || d.Body != "<p>theirs</p>" || d.Version != 2 || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
}

func TestARemoteEditAgainstLocalEditsIsAConflictAndIsNeverPushed(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "retro").PageID
	h.fake.EditRemote(id, "<p>theirs</p>")
	h.save(14, "retro", "<p>mine</p>")
	if res := h.run(sprint14); res.Conflicts != 1 {
		t.Fatalf("result = %+v", res)
	}
	h.fake.EditRemote(id, "<p>theirs again</p>")
	res := h.run(sprint14)
	d := h.doc(14, "retro")
	page, _ := h.fake.Page(id)
	if res.Conflicts != 1 || res.Pushed != 0 || d.ConflictVersion != 3 || d.ConflictBody != "<p>theirs again</p>" ||
		d.Body != "<p>mine</p>" || page.Body != "<p>theirs again</p>" {
		t.Fatalf("result = %+v, doc = %+v, page = %+v", res, d, page)
	}
	// Keep mine, and the next pass pushes over the newer version.
	if err := h.docs.ResolveMine(h.ctx, h.key(14, "retro"), "t"); err != nil {
		t.Fatal(err)
	}
	if res := h.run(sprint14); res.Pushed != 1 {
		t.Fatalf("after keep mine = %+v", res)
	}
	if page, _ := h.fake.Page(id); page.Body != "<p>mine</p>" || page.Version != 4 {
		t.Fatalf("page = %+v", page)
	}
}

func TestAPageDeletedInConfluenceGoesGoneAndIsNotRecreated(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	h.fake.Remove(h.doc(14, "review").PageID)
	if res := h.run(sprint14); res.Gone != 1 || h.doc(14, "review").Status != ritualrepo.StatusGone {
		t.Fatalf("result = %+v", res)
	}
	if res := h.run(sprint14); res.Gone != 0 || res.Created != 0 {
		t.Fatalf("a gone page must be skipped, not recreated: %+v", res)
	}
}

func TestAVersionRaceOnPushBecomesAConflict(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "planning").PageID
	h.save(14, "planning", "<p>mine</p>")
	// The teammate saves after the pass read version 1 and before it pushes.
	h.fake.After("get", id, func() { h.fake.EditRemote(id, "<p>raced</p>") })
	res := h.run(sprint14)
	d := h.doc(14, "planning")
	if res.Conflicts != 1 || res.Pushed != 0 || d.Status != ritualrepo.StatusConflict || d.ConflictBody != "<p>raced</p>" {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
}

// A 409 is not always a version race: an update can be refused for a title
// clash. When the page has not moved, the push failed; recording a conflict
// at the base version would claim a newer version that is not newer, and
// Keep mine would loop on it every Sync.
func TestA409WhosePageHasNotMovedIsAFailureNotAConflict(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "planning").PageID
	h.save(14, "planning", "<p>mine</p>")
	h.fake.FailNext("update", id, &confluence.HTTPError{Code: http.StatusConflict, Status: "409 Conflict", Message: "title clash"})
	res := h.run(sprint14)
	d := h.doc(14, "planning")
	if res.Conflicts != 0 || res.Pushed != 0 || len(res.Failed) != 1 || !strings.Contains(res.Failed[0].Reason, "title clash") {
		t.Fatalf("result = %+v", res)
	}
	if d.Status != ritualrepo.StatusUnsynced || d.ConflictVersion != 0 || d.ConflictBody != "" || d.Version != 1 || d.Body != "<p>mine</p>" {
		t.Fatalf("doc = %+v", d)
	}
}

func TestA409FollowedByA404MarksThePageGone(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "planning").PageID
	h.save(14, "planning", "<p>mine</p>")
	// The page is deleted after the pass read it, and the push is refused.
	h.fake.After("get", id, func() { h.fake.Remove(id) })
	h.fake.FailNext("update", id, &confluence.HTTPError{Code: http.StatusConflict, Status: "409 Conflict"})
	res := h.run(sprint14)
	if d := h.doc(14, "planning"); res.Gone != 1 || res.Conflicts != 0 || len(res.Failed) != 0 || d.Status != ritualrepo.StatusGone {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
}

func TestKeepMineAfterAnAdoptionConflictPushesOverTheRemote(t *testing.T) {
	h := newHarness(t)
	h.ensure(sprint14)
	h.save(14, ritualtemplate.Sprint, "<p>mine</p>")
	id := h.fake.Seed("root", "Sprint 14", "<p>theirs</p>")
	if res := h.run(sprint14); res.Conflicts != 1 {
		t.Fatalf("result = %+v", res)
	}
	if err := h.docs.ResolveMine(h.ctx, h.key(14, ritualtemplate.Sprint), "t"); err != nil {
		t.Fatal(err)
	}
	res := h.run(sprint14)
	page, _ := h.fake.Page(id)
	d := h.doc(14, ritualtemplate.Sprint)
	if res.Pushed != 1 || res.Conflicts != 0 || page.Body != "<p>mine</p>" || page.Version != 2 || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("result = %+v, doc = %+v, page = %+v", res, d, page)
	}
}

func TestKeepMineMeetsAConflictAgainWhenTheRemoteMovesFirst(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "retro").PageID
	h.fake.EditRemote(id, "<p>theirs</p>")
	h.save(14, "retro", "<p>mine</p>")
	h.run(sprint14)
	if err := h.docs.ResolveMine(h.ctx, h.key(14, "retro"), "t"); err != nil {
		t.Fatal(err)
	}
	h.fake.EditRemote(id, "<p>theirs again</p>")
	res := h.run(sprint14)
	d := h.doc(14, "retro")
	page, _ := h.fake.Page(id)
	if res.Conflicts != 1 || res.Pushed != 0 || d.Status != ritualrepo.StatusConflict || d.ConflictVersion != 3 ||
		d.ConflictBody != "<p>theirs again</p>" || page.Body != "<p>theirs again</p>" {
		t.Fatalf("result = %+v, doc = %+v, page = %+v", res, d, page)
	}
}

// A sprint page gone from Confluence has nothing new to be created under,
// but a ritual that already has its own page is still brought into step.
func TestARitualWithAPageStillSyncsWhenItsSprintPageIsGone(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	h.fake.Remove(h.doc(14, ritualtemplate.Sprint).PageID)
	h.save(14, "planning", "<p>plan</p>")
	res := h.run(sprint14)
	d := h.doc(14, "planning")
	page, _ := h.fake.Page(d.PageID)
	if res.Gone != 1 || res.Pushed != 1 || page.Body != "<p>plan</p>" || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("result = %+v, doc = %+v, page = %+v", res, d, page)
	}
}

func TestASaveDuringAPushStaysUnsynced(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "planning").PageID
	h.save(14, "planning", "<p>pushed</p>")
	h.fake.After("update", id, func() { h.save(14, "planning", "<p>typed during the push</p>") })
	res := h.run(sprint14)
	d := h.doc(14, "planning")
	if res.Pushed != 1 || d.Body != "<p>typed during the push</p>" || d.BaseBody != "<p>pushed</p>" || d.Status != ritualrepo.StatusUnsynced {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
	if res := h.run(sprint14); res.Pushed != 1 {
		t.Fatalf("the next pass should push what was typed: %+v", res)
	}
}

func TestASaveDuringAPullBecomesAConflictNotAnOverwrite(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "retro").PageID
	h.fake.EditRemote(id, "<p>theirs</p>")
	h.fake.After("get", id, func() { h.save(14, "retro", "<p>typed during the pull</p>") })
	res := h.run(sprint14)
	d := h.doc(14, "retro")
	if res.Conflicts != 1 || d.Body != "<p>typed during the pull</p>" || d.ConflictBody != "<p>theirs</p>" {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
}

func TestOneFailingPageLeavesTheOthersSynced(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	h.save(14, "planning", "<p>plan</p>")
	h.save(14, "review", "<p>review</p>")
	h.fake.FailNext("update", h.doc(14, "planning").PageID, errors.New("boom"))
	res := h.run(sprint14)
	if res.Pushed != 1 || len(res.Failed) != 1 || res.Failed[0].Title != "Sprint 14 · Planning" || res.Failed[0].Reason != "boom" {
		t.Fatalf("result = %+v", res)
	}
	if h.doc(14, "planning").Status != ritualrepo.StatusUnsynced || h.doc(14, "review").Status != ritualrepo.StatusSynced {
		t.Fatal("statuses after a partial pass are wrong")
	}
}

// pagesOnly hides the demo space's permission probe, the way a transport
// that cannot answer it would.
type pagesOnly struct{ confluence.Pages }

func TestAMissingRootIsReportedInTheResultNotAsAnError(t *testing.T) {
	h := newHarness(t)
	h.cfg.ProjectKey = "PLAT"
	h.fake.Remove("root")
	res, err := Run(h.ctx, h.fake, h.docs, h.cfg, testProfile, testBoard, []Sprint{sprint14})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := RootMissing{PageID: "root", SpaceKey: "PLAT", CanCreate: true, SuggestedTitle: "PLAT Rituals"}
	if res.RootMissing == nil || *res.RootMissing != want {
		t.Fatalf("root missing = %+v", res.RootMissing)
	}
	if res.Created != 0 || res.SyncedAt != "" || len(res.Failed) != 0 {
		t.Fatalf("a pass with no root must do nothing: %+v", res)
	}
	if docs, _ := h.docs.Documents(h.ctx, testProfile, testBoard, 14); len(docs) != 0 {
		t.Fatal("a pass with no root should not have written pages locally")
	}
}

func TestAMissingRootSaysWhenTheTokenCannotCreatePages(t *testing.T) {
	h := newHarness(t)
	h.fake.Remove("root")
	h.fake.DenyCreate()
	res, err := Run(h.ctx, h.fake, h.docs, h.cfg, testProfile, testBoard, []Sprint{sprint14})
	if err != nil || res.RootMissing == nil || res.RootMissing.CanCreate {
		t.Fatalf("result = %+v, %v", res.RootMissing, err)
	}
	if res.RootMissing.SuggestedTitle != "Rituals" {
		t.Fatalf("suggested title with no project key = %q", res.RootMissing.SuggestedTitle)
	}
}

func TestAMissingRootOnATransportWithNoProbeIsWorthTrying(t *testing.T) {
	h := newHarness(t)
	h.fake.Remove("root")
	h.fake.DenyCreate()
	res, err := Run(h.ctx, pagesOnly{h.fake}, h.docs, h.cfg, testProfile, testBoard, []Sprint{sprint14})
	if err != nil || res.RootMissing == nil || !res.RootMissing.CanCreate {
		t.Fatalf("result = %+v, %v", res.RootMissing, err)
	}
}

func TestSuggestedRootTitleNamesTheProject(t *testing.T) {
	for key, want := range map[string]string{"PLAT": "PLAT Rituals", " PLAT ": "PLAT Rituals", "": "Rituals", "  ": "Rituals"} {
		if got := SuggestedRootTitle(key); got != want {
			t.Errorf("SuggestedRootTitle(%q) = %q, want %q", key, got, want)
		}
	}
}
