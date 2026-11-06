package cases

import "testing"

func TestGroupsAreNamedListsOfProjects(t *testing.T) {
	s, _ := open(t)
	g, err := s.SaveGroup("p1", " Readers ", []string{"BETA", "ALPHA"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveGroup("p1", "Readers", []string{"GAMMA"}); err == nil {
		t.Error("saved a second group with the same name")
	}
	for _, bad := range []struct {
		name string
		keys []string
	}{{" ", []string{"A"}}, {"Empty", nil}, {"Bad", []string{"no"}}} {
		if _, err := s.SaveGroup("p1", bad.name, bad.keys); err == nil {
			t.Errorf("saved %+v", bad)
		}
	}
	_, _ = s.SaveGroup("p2", "Other", []string{"X"})
	got, err := s.Groups("p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != g.ID || got[0].Name != "Readers" || len(got[0].Projects) != 2 || got[0].Projects[0] != "ALPHA" {
		t.Fatalf("groups %+v", got)
	}
	if err := s.DeleteGroup("p2", g.ID); err == nil {
		t.Error("deleted p1's group as p2")
	}
	if err := s.DeleteGroup("p1", g.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Groups("p1"); len(got) != 0 {
		t.Fatalf("delete left %+v", got)
	}
}
