package ritualsync

import (
	"testing"
	"time"

	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

// board is the info a board-level document renders and is titled from: no
// sprint, which is what its sprint id of 0 says.
var board = ritualtemplate.SprintInfo{BoardName: "PLAT board"}

// seedAgreement writes a done agreement row the way EnsureAgreement does,
// through the repository, so the sync tests below stand on their own.
func (h *harness) seedAgreement(info ritualtemplate.SprintInfo) {
	h.t.Helper()
	k := h.key(info.ID, ritualtemplate.DoneAgreement)
	title := ritualtemplate.Title(ritualtemplate.DoneAgreement, info)
	body := ritualtemplate.Render(ritualtemplate.DoneAgreement, info, time.UTC)
	if err := h.docs.WriteTemplate(h.ctx, k, title, body, "t"); err != nil {
		h.t.Fatal(err)
	}
}

// The board's standing agreement has no sprint, so the pass that walks
// sprints has to reach it by another path or it would never be created,
// pushed, pulled or conflicted, and the Rituals view would show a document
// that quietly never synced.
func TestABoardsDoneAgreementSyncsUnderTheRootWithNoSprint(t *testing.T) {
	h := newHarness(t)
	h.seedAgreement(board)
	res := h.run(sprint14)
	if res.Created != 6 {
		t.Errorf("created = %d, want the five sprint pages and the board's agreement", res.Created)
	}
	d := h.doc(0, ritualtemplate.DoneAgreement)
	if d.Status != ritualrepo.StatusSynced || d.Title != "PLAT board · Done agreement" {
		t.Fatalf("board agreement = %+v", d)
	}
	page, ok := h.fake.Page(d.PageID)
	if !ok || len(page.AncestorIDs) != 1 || page.AncestorIDs[0] != "root" {
		t.Fatalf("the board's agreement is not under the rituals root: %+v, %v", page, ok)
	}
}

// A sprint's additions are the same kind with the sprint's own id, so they
// belong under that sprint's overview page like its other documents.
func TestASprintsDoneAgreementAdditionsSyncUnderItsOverview(t *testing.T) {
	h := newHarness(t)
	h.seedAgreement(sprint14.Info)
	res := h.run(sprint14)
	if res.Created != 6 {
		t.Errorf("created = %d, want the five sprint pages and this sprint's additions", res.Created)
	}
	d := h.doc(14, ritualtemplate.DoneAgreement)
	overview := h.doc(14, ritualtemplate.Sprint)
	page, ok := h.fake.Page(d.PageID)
	if d.Status != ritualrepo.StatusSynced || !ok || page.AncestorIDs[len(page.AncestorIDs)-1] != overview.PageID {
		t.Fatalf("the additions are not under the sprint page: doc %+v, page %+v", d, page)
	}
	if d.Title != "Sprint 14 · Done agreement" {
		t.Errorf("title = %q", d.Title)
	}
}

// A local edit to either has to reach Confluence like any other page. This
// also holds the second half of the board-level path: reconcile, not only the
// first create.
func TestAnEditedDoneAgreementIsPushedOnTheNextPass(t *testing.T) {
	h := newHarness(t)
	h.seedAgreement(board)
	h.run(sprint14)
	h.save(0, ritualtemplate.DoneAgreement, "<p>ours, edited</p>")
	res := h.run(sprint14)
	if res.Pushed != 1 {
		t.Fatalf("pushed = %d", res.Pushed)
	}
	d := h.doc(0, ritualtemplate.DoneAgreement)
	page, _ := h.fake.Page(d.PageID)
	if d.Status != ritualrepo.StatusSynced || page.Body != "<p>ours, edited</p>" {
		t.Fatalf("doc = %+v, page = %+v", d, page)
	}
}

// Ensure is what a sprint gets without asking, and the done agreement is not
// part of it: a board has one agreement, not one per sprint.
func TestEnsureGivesASprintNoDoneAgreement(t *testing.T) {
	h := newHarness(t)
	h.ensure(sprint14)
	docs, err := h.docs.Documents(h.ctx, testProfile, testBoard, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 5 {
		t.Fatalf("documents = %d", len(docs))
	}
	for _, d := range docs {
		if d.RitualType == ritualtemplate.DoneAgreement {
			t.Fatal("Ensure wrote a done agreement nobody asked for")
		}
	}
}

// EnsureAgreement is the one writer of the row, and it is called only when
// somebody asks for the document. It fills a missing row and leaves an
// edited one exactly as it is.
func TestEnsureAgreementWritesOneOnRequestAndLeavesAnEditedOneAlone(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	if err := EnsureAgreement(h.ctx, h.docs, testProfile, testBoard, board, now); err != nil {
		t.Fatal(err)
	}
	d := h.doc(0, ritualtemplate.DoneAgreement)
	if d.Status != ritualrepo.StatusLocal || d.Title != "PLAT board · Done agreement" ||
		d.Body != ritualtemplate.Render(ritualtemplate.DoneAgreement, board, time.UTC) {
		t.Fatalf("board agreement = %+v", d)
	}
	h.save(0, ritualtemplate.DoneAgreement, "<p>what we agreed</p>")
	if err := EnsureAgreement(h.ctx, h.docs, testProfile, testBoard, board, now); err != nil {
		t.Fatal(err)
	}
	if got := h.doc(0, ritualtemplate.DoneAgreement).Body; got != "<p>what we agreed</p>" {
		t.Fatalf("a second request rewrote the agreement: %q", got)
	}
}

// #74's failure mode, guarded where it would actually bite. Review and
// Planning point at the board's done agreement, and place() decides whether a
// page was written in by comparing its stored body against a fresh render, so
// a reference that changed once the agreement was published would make both
// pages read as written in and raise a conflict every sprint.
func TestPublishingTheDoneAgreementLeavesTheSprintsPagesUntouched(t *testing.T) {
	h := newHarness(t)
	h.seedAgreement(board)
	if res := h.run(sprint14); res.Created != 6 {
		t.Fatalf("first pass = %+v", res)
	}
	for _, typ := range []string{ritualtemplate.Review, ritualtemplate.Planning} {
		d := h.doc(14, typ)
		if d.Body != ritualtemplate.Render(typ, sprint14.Info, time.UTC) {
			t.Errorf("the stored %s no longer matches a fresh render now the agreement is published:\n%s", typ, d.Body)
		}
		if d.Dirty() {
			t.Errorf("%s reads as written in: %+v", typ, d)
		}
	}
	if res := h.run(sprint14); res.Conflicts != 0 || res.Pushed != 0 || res.Created != 0 || len(res.Failed) != 0 {
		t.Fatalf("a second pass over untouched pages did something: %+v", res)
	}
}
