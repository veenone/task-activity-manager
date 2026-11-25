package demo_test

import (
	"testing"

	"agile-suite/tam/internal/demo"
)

func TestDemoIssuesNameOnlyCatalogIDs(t *testing.T) {
	versions := map[int]bool{}
	unreleasedUsed := map[int]bool{}
	for _, v := range demo.Versions("DEMO") {
		versions[v.ID] = true
		if !v.Released {
			unreleasedUsed[v.ID] = false
		}
	}
	components := map[int]bool{}
	for _, c := range demo.Components("DEMO") {
		components[c.ID] = true
	}
	if len(versions) == 0 || len(components) == 0 || len(unreleasedUsed) == 0 {
		t.Fatalf("catalog has %d versions (%d unreleased) and %d components", len(versions), len(unreleasedUsed), len(components))
	}
	for _, iss := range demo.Issues("DEMO") {
		for _, id := range iss.FixVersions {
			if !versions[id] {
				t.Errorf("%s names fix version %d, not in the catalog", iss.Key, id)
			}
			if _, ok := unreleasedUsed[id]; ok {
				unreleasedUsed[id] = true
			}
		}
		for _, id := range iss.Components {
			if !components[id] {
				t.Errorf("%s names component %d, not in the catalog", iss.Key, id)
			}
		}
	}
	for id, used := range unreleasedUsed {
		if !used {
			t.Errorf("unreleased version %d has no issue", id)
		}
	}
	vs := demo.Versions("DEMO")
	vs[0].Name = "changed"
	if demo.Versions("DEMO")[0].Name == "changed" {
		t.Error("Versions hands out its own slice")
	}
}
