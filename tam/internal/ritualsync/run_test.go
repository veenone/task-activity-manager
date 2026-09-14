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
