package cases

import (
	"reflect"
	"testing"
)

func TestBaselinesHoldTheirDescendants(t *testing.T) {
	s, sv := open(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.AddBaseline("p1", "BASE"))
	must(s.AddBaseline("p1", "AURA"))
	must(s.AddDescendants("p1", "BASE", []string{"DELTA", "ECHO"}))
	must(s.AddDescendants("p1", "BASE", []string{"ECHO"})) // already there: no change
	must(s.AddBaseline("p2", "OTHER"))

	got, err := s.Baselines("p1")
	must(err)
	want := []Baseline{{Key: "AURA", Descendants: []string{}}, {Key: "BASE", Descendants: []string{"DELTA", "ECHO"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("baselines %+v, want %+v", got, want)
	}

	for name, err := range map[string]error{
		"a second baseline for a descendant": s.AddDescendants("p1", "AURA", []string{"DELTA"}),
		"a baseline as a descendant":         s.AddDescendants("p1", "BASE", []string{"AURA"}),
		"a descendant as a baseline":         s.AddBaseline("p1", "ECHO"),
		"the same baseline twice":            s.AddBaseline("p1", "BASE"),
		"descendants of no baseline":         s.AddDescendants("p1", "NONE", []string{"FOX"}),
		"a key that is not a project":        s.AddBaseline("p1", "no"),
	} {
		if err == nil {
			t.Errorf("allowed %s", name)
		}
	}

	must(s.RemoveDescendant("p1", "DELTA"))
	if got, _ := s.Baselines("p1"); !reflect.DeepEqual(got[1].Descendants, []string{"ECHO"}) {
		t.Fatalf("after removing a descendant %+v", got)
	}
	must(s.RemoveBaseline("p1", "BASE"))
	if got, _ := s.Baselines("p1"); len(got) != 1 || got[0].Key != "AURA" {
		t.Fatalf("after removing a baseline %+v", got)
	}
	// ECHO went with BASE, so it is free for another baseline.
	must(s.AddDescendants("p1", "AURA", []string{"ECHO"}))

	must(sv.Purge("p1"))
	if got, _ := s.Baselines("p1"); len(got) != 0 {
		t.Fatalf("purge left %+v", got)
	}
	if got, _ := s.Baselines("p2"); len(got) != 1 {
		t.Fatalf("purge reached p2: %+v", got)
	}
}
