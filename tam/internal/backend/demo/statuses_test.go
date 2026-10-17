package demo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

// TestDemoStatusesNameEveryDemoStatusesCategory gives the offline sprint
// report's People section its In Progress bucket.
func TestDemoStatusesNameEveryDemoStatusesCategory(t *testing.T) {
	var sb backend.StatusBackend = demobackend.New("PLAT")
	got, err := sb.Statuses(context.Background())
	if err != nil {
		t.Fatalf("Statuses: %v", err)
	}
	cat := map[string]string{}
	for _, s := range got {
		if s.ID != demobackend.StatusID(s.Name) {
			t.Errorf("%s has id %q, want the boards' %q", s.Name, s.ID, demobackend.StatusID(s.Name))
		}
		cat[s.Name] = s.Category
	}
	if cat["To Do"] != "new" || cat["In Progress"] != "indeterminate" || cat["Done"] != "done" {
		t.Errorf("categories = %v", cat)
	}
}
