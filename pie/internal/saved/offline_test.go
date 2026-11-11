package saved

import "testing"

// Keep links for offline is a per-query choice, off until set (#464).
func TestKeepOfflineIsPerQueryAndGoesWithTheQuery(t *testing.T) {
	s := open(t)
	a, err := s.Save("p1", "A", "x")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Save("p1", "B", "y")
	if err != nil {
		t.Fatal(err)
	}
	if a.KeepOffline {
		t.Fatal("a new query keeps links for offline")
	}
	if err := s.SetKeepOffline("p1", a.ID, true); err != nil {
		t.Fatal(err)
	}
	if on, _ := s.KeepOffline("p1", a.ID); !on {
		t.Fatal("the choice did not stick")
	}
	list, _ := s.List("p1")
	if !list[0].KeepOffline || list[1].KeepOffline {
		t.Fatalf("list %+v", list)
	}
	if err := s.SetKeepOffline("p1", b.ID+99, true); err == nil {
		t.Fatal("set it on a query that does not exist")
	}
	if err := s.Delete("p1", a.ID); err != nil {
		t.Fatal(err)
	}
	if on, _ := s.KeepOffline("p1", a.ID); on {
		t.Fatal("the choice outlived its query")
	}
	if err := s.SetKeepOffline("p1", b.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetKeepOffline("p1", b.ID, false); err != nil {
		t.Fatal(err)
	}
	if on, _ := s.KeepOffline("p1", b.ID); on {
		t.Fatal("it could not be turned off")
	}
}
