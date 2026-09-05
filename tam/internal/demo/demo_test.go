package demo_test

import (
	"reflect"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

func TestIssuesAreDeterministicAndWellFormed(t *testing.T) {
	a := demo.Issues(demo.ProjectKey)
	b := demo.Issues(demo.ProjectKey)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two calls returned different datasets")
	}
	if len(a) != 60 {
		t.Fatalf("len = %d, want 60", len(a))
	}
	seen := map[string]bool{}
	types := map[string]int{}
	for _, iss := range a {
		if seen[iss.Key] {
			t.Errorf("duplicate key %s", iss.Key)
		}
		seen[iss.Key] = true
		types[iss.Type]++
		if iss.Project != "PLAT" || iss.Summary == "" || iss.Status == "" || iss.Rank == "" || iss.Updated == "" || iss.ID == "" {
			t.Errorf("incomplete issue %+v", iss)
		}
		if iss.Labels == nil {
			t.Errorf("%s has nil labels; want an empty slice", iss.Key)
		}
	}
	for _, typ := range backend.AllTypes {
		if types[typ] == 0 {
			t.Errorf("no issues of type %s", typ)
		}
	}
}

func TestCuratedIssuesMatchTheMockup(t *testing.T) {
	by := map[string]backend.Issue{}
	for _, iss := range demo.Issues(demo.ProjectKey) {
		by[iss.Key] = iss
	}
	s := by["PLAT-412"]
	if s.Type != "story" || s.Summary != "Checkout: apply promo code at payment step" || s.Status != "In Progress" ||
		s.Assignee != "R. Anand" || s.SprintID != "12" || s.SprintName != "Sprint 12" || s.StoryPoints == nil || *s.StoryPoints != 5 ||
		s.ParentKey != "PLAT-350" || len(s.Labels) != 2 {
		t.Errorf("PLAT-412 = %+v", s)
	}
	if r := by["PLAT-388"]; r.Type != "requirement" || r.Status != "Approved" || r.SprintID != "" || r.StoryPoints != nil {
		t.Errorf("PLAT-388 = %+v", r)
	}
	if e := by["PLAT-350"]; e.Type != "epic" || e.StoryPoints == nil || *e.StoryPoints != 21 {
		t.Errorf("PLAT-350 = %+v", e)
	}
	if b := by["PLAT-401"]; b.Type != "bug" || b.Assignee != "M. Ortiz" {
		t.Errorf("PLAT-401 = %+v", b)
	}
}

func TestProjectKeyIsSubstituted(t *testing.T) {
	issues := demo.Issues("DEMO")
	if issues[0].Key != "DEMO-412" || issues[0].Project != "DEMO" || issues[0].ParentKey != "DEMO-350" {
		t.Errorf("first issue = %+v", issues[0])
	}
	d, ok := demo.Detail("DEMO", "DEMO-412")
	if !ok || d.Key != "DEMO-412" {
		t.Errorf("detail = %+v, ok=%v", d, ok)
	}
}

func TestDetailCarriesLinksForTheCuratedStory(t *testing.T) {
	d, ok := demo.Detail(demo.ProjectKey, "PLAT-412")
	if !ok {
		t.Fatal("no detail for PLAT-412")
	}
	if d.Description == "" {
		t.Error("empty description")
	}
	tested := 0
	for _, l := range d.Links {
		if l.Type == "Tested By" && l.IssueType == "Test" {
			tested++
		}
	}
	if tested != 2 {
		t.Errorf("PLAT-412 has %d Tested By links, want 2: %+v", tested, d.Links)
	}
	if _, ok := demo.Detail(demo.ProjectKey, "PLAT-260"); !ok {
		t.Error("filler issues have a detail too")
	}
	if _, ok := demo.Detail(demo.ProjectKey, "PLAT-9999"); ok {
		t.Error("unknown key should not have a detail")
	}
}
