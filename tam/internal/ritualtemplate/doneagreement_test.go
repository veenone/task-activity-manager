package ritualtemplate

import (
	"strings"
	"testing"
	"time"
)

// The done agreement is the team's own statement of what finished means. It is
// a kind the bindings accept, and deliberately not one of Types: Types is what
// a sprint gets without asking, and a board's agreement is one document for
// the board rather than a sixth page per sprint.
func TestTheDoneAgreementIsAKindOfItsOwnAndNotOneOfTheFive(t *testing.T) {
	// The literal is the value stored in ritual_document.ritual_type, so it is
	// spelled out here rather than read from the constant that names it.
	if !Known("doneagreement") {
		t.Error("the bindings would refuse a done agreement save")
	}
	if got := Label("doneagreement"); got != "Done agreement" {
		t.Errorf("label = %q", got)
	}
	for _, typ := range Types {
		if typ == "doneagreement" {
			t.Fatal("the done agreement is in Types, so Ensure writes one for every sprint")
		}
	}
}

func TestADocumentWithNoSprintIsTitledAfterItsBoard(t *testing.T) {
	if got := Title("doneagreement", SprintInfo{BoardName: "PLAT board"}); got != "PLAT board · Done agreement" {
		t.Errorf("board title = %q", got)
	}
	if got := Title("doneagreement", SprintInfo{ID: 14, Name: "Sprint 14", BoardName: "PLAT board"}); got != "Sprint 14 · Done agreement" {
		t.Errorf("sprint title = %q", got)
	}
}

// run.go decides whether a page was written in by comparing its stored body
// against a fresh render, so a render that moves with the clock or the zone
// turns untouched pages into conflicts (#74). The done agreement reads
// neither, which this asserts rather than trusts.
func TestTheDoneAgreementRendersATaskListAndReadsNoZone(t *testing.T) {
	board := SprintInfo{BoardName: "PLAT board"}
	body := Render("doneagreement", board, time.UTC)
	if !strings.Contains(body, taskList) {
		t.Errorf("the items are not a task list: %s", body)
	}
	for _, loc := range []*time.Location{nil, time.FixedZone("WIB", 7*60*60), time.FixedZone("behind", -11*60*60)} {
		if got := Render("doneagreement", board, loc); got != body {
			t.Errorf("the render moved with the zone:\n%s\n%s", body, got)
		}
	}
	additions := Render("doneagreement", SprintInfo{ID: 14, Name: "Sprint 14", BoardName: "PLAT board"}, time.UTC)
	if additions == body {
		t.Error("a sprint's additions render as the board's standing agreement")
	}
	if !strings.Contains(additions, "Sprint 14") {
		t.Errorf("the additions page does not say which sprint it adds to: %s", additions)
	}
	if !strings.Contains(additions, taskList) {
		t.Errorf("the additions are not a task list: %s", additions)
	}
}

// The sprint's own pages point at the agreement the team argues against, by
// title. A page id or a URL would be the shape #74 rejected on the retro
// template: an id is empty until the page is published and changes
// afterwards, so publishing the agreement would make an untouched Review page
// render differently and read as one somebody wrote in.
func TestReviewAndPlanningPointAtTheDoneAgreementByTitle(t *testing.T) {
	include := `<ac:structured-macro ac:name="include"><ac:parameter ac:name=""><ac:link>` +
		`<ri:page ri:content-title="PLAT board · Done agreement"/></ac:link></ac:parameter></ac:structured-macro>`
	for _, typ := range []string{Review, Planning} {
		body := Render(typ, golden, time.UTC)
		if !strings.Contains(body, include) {
			t.Errorf("%s does not include the board's agreement by title: %s", typ, body)
		}
		// The effective agreement is the board's items plus this sprint's, so
		// the sprint's own page is named as well.
		if !strings.Contains(body, "Sprint 14 · Done agreement") {
			t.Errorf("%s does not name this sprint's additions: %s", typ, body)
		}
		for _, forbidden := range []string{"ri:content-id", "pageId", "viewpage.action", "ac:macro-id"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s references a page by %s, which is not stable when that page is published: %s", typ, forbidden, body)
			}
		}
	}
}
