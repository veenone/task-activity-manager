package cases

import (
	"testing"
	"time"
)

func caseFor(t *testing.T, s *Store, key string) int64 {
	t.Helper()
	if err := s.Open("p1", []string{key}, at); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List("p1")
	for _, c := range list {
		if c.Key == key {
			return c.ID
		}
	}
	t.Fatalf("no case for %s", key)
	return 0
}

func TestAProjectStartsNotAssessedAndTakesAVerdict(t *testing.T) {
	s, _ := open(t)
	id := caseFor(t, s, "A-1")
	if err := s.AddProjects("p1", id, []string{"BETA", "ALPHA", "BETA"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetVerdict("p1", id, "BETA", Affected, " Same chip ", at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	d, err := s.Get("p1", id)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Projects) != 2 || d.Projects[0] != (Project{Key: "ALPHA", Verdict: NotAssessed}) {
		t.Fatalf("projects %+v", d.Projects)
	}
	if b := d.Projects[1]; b.Verdict != Affected || b.Reason != "Same chip" || b.AssessedAt != "2026-11-06T11:00:00Z" {
		t.Fatalf("BETA = %+v", b)
	}
	list, _ := s.List("p1")
	if list[0].Assessed != 1 || list[0].Affected != 1 {
		t.Fatalf("counts %+v", list[0])
	}
}

func TestVerdictsAndProjectsAreChecked(t *testing.T) {
	s, _ := open(t)
	id := caseFor(t, s, "A-1")
	_ = s.AddProjects("p1", id, []string{"ALPHA"})
	if err := s.SetVerdict("p1", id, "ALPHA", "maybe", "", at); err == nil {
		t.Error("stored an unknown verdict")
	}
	if err := s.SetVerdict("p1", id, "GAMMA", Affected, "", at); err == nil {
		t.Error("set a verdict for a project not on the case")
	}
	if err := s.AddProjects("p1", id, []string{"bad key"}); err == nil {
		t.Error("added a malformed project key")
	}
	if err := s.AddProjects("p1", id+99, []string{"ALPHA"}); err == nil {
		t.Error("added a project to a case that does not exist")
	}
	if _, err := s.Get("p2", id); err == nil {
		t.Error("read p1's case as p2")
	}
}

func TestNotesAndRemovingAProject(t *testing.T) {
	s, _ := open(t)
	id := caseFor(t, s, "A-1")
	_ = s.AddProjects("p1", id, []string{"ALPHA", "BETA"})
	if err := s.SetNotes("p1", id, "Seen twice.\nAsk reader team."); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveProject("p1", id, "ALPHA"); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Get("p1", id)
	if d.Notes != "Seen twice.\nAsk reader team." || len(d.Projects) != 1 || d.Projects[0].Key != "BETA" {
		t.Fatalf("case %+v", d)
	}
}

func TestPurgeTakesProjectsAndGroups(t *testing.T) {
	s, sv := open(t)
	id := caseFor(t, s, "A-1")
	_ = s.AddProjects("p1", id, []string{"ALPHA"})
	if _, err := s.SaveGroup("p1", "Readers", []string{"ALPHA"}); err != nil {
		t.Fatal(err)
	}
	if err := sv.Purge("p1"); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM case_project) + (SELECT COUNT(*) FROM project_group) + (SELECT COUNT(*) FROM project_group_member)`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d rows left after purge", n)
	}
}
