package ritualsync

import (
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

func TestEnsureWritesTheFiveTemplatesLocally(t *testing.T) {
	h := newHarness(t)
	h.ensure(sprint14)
	docs, _ := h.docs.Documents(h.ctx, testProfile, testBoard, 14)
	if len(docs) != 5 {
		t.Fatalf("documents = %d", len(docs))
	}
	for _, typ := range ritualtemplate.Types {
		d := h.doc(14, typ)
		if d.Status != ritualrepo.StatusLocal || d.Title != ritualtemplate.Title(typ, sprint14.Info) ||
			d.Body != ritualtemplate.Render(typ, sprint14.Info, time.UTC) {
			t.Errorf("%s = %+v", typ, d)
		}
	}
}

func TestEnsureTwiceLeavesAnEditedPageAlone(t *testing.T) {
	h := newHarness(t)
	h.ensure(sprint14)
	h.save(14, "planning", "<p>mine</p>")
	h.ensure(sprint14)
	if got := h.doc(14, "planning").Body; got != "<p>mine</p>" {
		t.Fatalf("planning = %q", got)
	}
}

func TestEnsureCarriesTheWizardsRemarksForward(t *testing.T) {
	h := newHarness(t)
	if _, err := h.docs.DB().ExecContext(h.ctx, `INSERT INTO ritual_document
		(profile_id, board_id, sprint_id, ritual_type, remark, issues_json)
		VALUES ('p1', 1, 14, 'review', 'short sprint', '[{"key":"PLAT-1","remark":"demoed"}]')`); err != nil {
		t.Fatal(err)
	}
	h.ensure(sprint14)
	body := h.doc(14, "review").Body
	if !strings.HasPrefix(body, ritualtemplate.Render("review", sprint14.Info, time.UTC)) ||
		!strings.HasSuffix(body, "<h2>Earlier draft notes</h2><p>short sprint</p><ul><li>PLAT-1: demoed</li></ul>") {
		t.Fatalf("review = %s", body)
	}
}

func TestEnsureGivesAClosedSprintNothing(t *testing.T) {
	h := newHarness(t)
	closed := sprint14
	closed.State = "closed"
	h.ensure(closed)
	if docs, _ := h.docs.Documents(h.ctx, testProfile, testBoard, 14); len(docs) != 0 {
		t.Fatalf("a closed sprint got %d documents", len(docs))
	}
}

func TestSprintsCoversOpenSprintsAndClosedOnesThatHavePages(t *testing.T) {
	cached := []boardrepo.Sprint{
		{ID: 14, Name: "Sprint 14", State: "active"},
		{ID: 15, Name: "Sprint 15", State: "future"},
		{ID: 12, Name: "Sprint 12", State: "closed"},
		{ID: 11, Name: "Sprint 11", State: "closed"},
	}
	got := Sprints(cached, "PLAT board", []int{12})
	var ids []int
	for _, s := range got {
		ids = append(ids, s.Info.ID)
		if s.Info.BoardName != "PLAT board" {
			t.Errorf("board name = %q", s.Info.BoardName)
		}
	}
	if len(ids) != 3 || ids[0] != 14 || ids[1] != 15 || ids[2] != 12 {
		t.Fatalf("sprints = %v", ids)
	}
}

// The retro template as it stood before issue #72, kept literally so the test
// keeps asking the question it was written for once the template moves again.
const retroBeforeIssue72 = `<h2>What went well</h2><ul><li></li></ul><h2>What did not</h2><ul><li></li></ul>` +
	`<h2>What we will try</h2><ul><li></li></ul><h2>Carried over</h2>` +
	`<ac:structured-macro ac:name="jira"><ac:parameter ac:name="jqlQuery">sprint = 14 AND statusCategory != Done</ac:parameter>` +
	`<ac:parameter ac:name="columns">key,summary,type,status,assignee</ac:parameter>` +
	`<ac:parameter ac:name="maximumIssues">50</ac:parameter></ac:structured-macro>` +
	`<h2>Action items</h2><ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body></ac:task-body></ac:task></ac:task-list>`

// A retrospective written under an older template is never rewritten by a
// newer one: Ensure only fills a document that has neither a body nor a page.
// The one place the change shows is an adoption, where a body TAM can no
// longer recognise as its own untouched template stops being pulled over
// silently and becomes a conflict the user decides, with both sides kept.
func TestARetroFromTheOldTemplateIsKeptAndItsAdoptionAsks(t *testing.T) {
	h := newHarness(t)
	overview := h.fake.Seed("root", ritualtemplate.Title(ritualtemplate.Sprint, sprint14.Info), "<p>theirs</p>")
	h.fake.Seed(overview, ritualtemplate.Title(ritualtemplate.Retro, sprint14.Info), "<p>written in Confluence</p>")
	h.ensure(sprint14)
	h.save(14, ritualtemplate.Retro, retroBeforeIssue72)
	h.ensure(sprint14)
	if got := h.doc(14, ritualtemplate.Retro).Body; got != retroBeforeIssue72 {
		t.Fatalf("Ensure rewrote a retro written under the old template: %s", got)
	}
	res := h.run(sprint14)
	d := h.doc(14, ritualtemplate.Retro)
	if res.Conflicts != 1 || d.Status != ritualrepo.StatusConflict ||
		d.Body != retroBeforeIssue72 || d.ConflictBody != "<p>written in Confluence</p>" {
		t.Fatalf("result = %+v, retro = %+v", res, d)
	}
}
