package ritualtemplate

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var golden = SprintInfo{
	ID: 14, Name: "Sprint 14", Goal: "Ship promo codes & the VAT fix",
	StartDate: "2026-09-14T09:00:00.000+0000", EndDate: "2026-09-25T17:00:00.000+0000",
	BoardName: "PLAT board",
}

// The golden files are the frontend's round-trip corpus as well, so a change
// to a template is a change the storage converter is re-tested against.
// Regenerate with: UPDATE_GOLDEN=1 go test ./internal/ritualtemplate/
func TestRenderMatchesTheGoldenFiles(t *testing.T) {
	for _, typ := range Types {
		got := Render(typ, golden, time.UTC)
		path := filepath.Join("testdata", strings.TrimPrefix(typ, "_")+".xml")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run with UPDATE_GOLDEN=1 once)", path, err)
		}
		if got != string(want) {
			t.Errorf("%s drifted from its golden file:\n got %s\nwant %s", typ, got, string(want))
		}
	}
}

// Adoption compares a stored body against a fresh render, so the same sprint
// must always render the same bytes.
func TestRenderIsDeterministic(t *testing.T) {
	for _, typ := range Types {
		if Render(typ, golden, time.UTC) != Render(typ, golden, time.UTC) {
			t.Errorf("%s rendered differently twice", typ)
		}
	}
}

func TestRenderIsWellFormedOnceItsPrefixesAreDeclared(t *testing.T) {
	for _, typ := range Types {
		doc := `<root xmlns:ac="http://atlassian.com/content" xmlns:ri="http://atlassian.com/resource/identifier">` +
			Render(typ, golden, time.UTC) + `</root>`
		dec := xml.NewDecoder(strings.NewReader(doc))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s is not well formed: %v", typ, err)
			}
		}
	}
}

func TestRenderEmitsOnlyTheThreeJQLForms(t *testing.T) {
	cases := map[string][]string{
		Planning: {"sprint = 14 ORDER BY Rank"},
		Standup:  {"sprint = 14 AND statusCategory != Done"},
		Review:   {"sprint = 14 AND statusCategory = Done", "sprint = 14 AND statusCategory != Done"},
		Retro:    {"sprint = 14 AND statusCategory != Done"},
	}
	for typ, wants := range cases {
		body := Render(typ, golden, time.UTC)
		for _, want := range wants {
			if !strings.Contains(body, `<ac:parameter ac:name="jqlQuery">`+want+`</ac:parameter>`) {
				t.Errorf("%s is missing %q", typ, want)
			}
		}
	}
}

func TestRenderEscapesAndCarriesTheSprintFacts(t *testing.T) {
	body := Render(Sprint, golden, time.UTC)
	for _, want := range []string{"Ship promo codes &amp; the VAT fix", "PLAT board", "14 Sep 2026 to 25 Sep 2026", `ac:name="children"`} {
		if !strings.Contains(body, want) {
			t.Errorf("overview missing %q in %s", want, body)
		}
	}
	undated := golden
	undated.StartDate, undated.EndDate = "", ""
	if !strings.Contains(Render(Sprint, undated, time.UTC), "Dates not set") {
		t.Error("an undated sprint should say its dates are not set")
	}
	if strings.Contains(Render(Standup, undated, time.UTC), "<h3>") {
		t.Error("an undated standup should seed no daily entry")
	}
}

func TestTitlesAndLabels(t *testing.T) {
	if got := Title(Sprint, golden); got != "Sprint 14" {
		t.Errorf("overview title = %q", got)
	}
	if got := Title(Retro, golden); got != "Sprint 14 · Retrospective" {
		t.Errorf("retro title = %q", got)
	}
	if got := Title(Planning, SprintInfo{ID: 9}); got != "Sprint 9 · Planning" {
		t.Errorf("nameless title = %q", got)
	}
	if !Known(" Review ") || Known("party") || Label(Sprint) != "Overview" {
		t.Error("Known/Label disagree with the type list")
	}
}

func TestStandupEntryIsTheDayHeadingAndThreeSections(t *testing.T) {
	entry := StandupEntry(time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	if !strings.HasPrefix(entry, "<h3>Tue 15 Sep 2026</h3>") {
		t.Fatalf("entry = %s", entry)
	}
	for _, want := range []string{"<strong>Yesterday</strong>", "<strong>Today</strong>", "<strong>Blockers</strong>", "<ac:task-list>"} {
		if !strings.Contains(entry, want) {
			t.Errorf("entry missing %q", want)
		}
	}
}

func TestEarlierNotesKeepsOnlyWhatSomebodyWrote(t *testing.T) {
	if EarlierNotes("  ", []Note{{Key: "PLAT-1"}}) != "" {
		t.Error("nothing written should add nothing")
	}
	got := EarlierNotes("short sprint", []Note{{Key: "PLAT-1", Remark: "demoed"}, {Key: "PLAT-2"}})
	want := "<h2>Earlier draft notes</h2><p>short sprint</p><ul><li>PLAT-1: demoed</li></ul>"
	if got != want {
		t.Errorf("notes = %s", got)
	}
}

func TestParseJQLReadsTheThreeFormsAndNothingElse(t *testing.T) {
	for _, f := range []Filter{All, Done, NotDone} {
		id, got, ok := ParseJQL(JQL(14, f))
		if !ok || id != 14 || got != f {
			t.Errorf("round trip of %v = %d %v %v", f, id, got, ok)
		}
	}
	if _, _, ok := ParseJQL("  SPRINT = 3  and statuscategory != done "); !ok {
		t.Error("case and spacing should not matter")
	}
	for _, other := range []string{"project = PLAT", "sprint = 14 AND assignee = currentUser()", "sprint in openSprints()"} {
		if _, _, ok := ParseJQL(other); ok {
			t.Errorf("%q should not be previewable", other)
		}
	}
}
