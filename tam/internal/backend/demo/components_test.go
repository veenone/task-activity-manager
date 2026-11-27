package demo_test

import (
	"context"
	"slices"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

func componentByID(t *testing.T, b *demobackend.Backend, id int) (backend.Component, bool) {
	t.Helper()
	cs, err := b.ProjectComponents(context.Background(), "ACME")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.ID == id {
			return c, true
		}
	}
	return backend.Component{}, false
}

func TestDemoComponentLifecycle(t *testing.T) {
	b := demobackend.New("ACME")
	var cb backend.ComponentBackend = b
	ctx := context.Background()
	made, err := cb.CreateComponent(ctx, "ACME", backend.ComponentInput{Name: "Reports", LeadName: "ranand", AssigneeType: "COMPONENT_LEAD"})
	if err != nil || made.ID <= 10004 || made.Name != "Reports" {
		t.Fatalf("made %+v %v", made, err)
	}
	if c, ok := componentByID(t, b, made.ID); !ok || c.LeadName != "ranand" {
		t.Fatalf("listed %+v %v", c, ok)
	}
	if _, err := cb.UpdateComponent(ctx, made.ID, backend.ComponentInput{Name: "Reporting", AssigneeType: "PROJECT_DEFAULT"}); err != nil {
		t.Fatal(err)
	}
	if c, _ := componentByID(t, b, made.ID); c.Name != "Reporting" || c.LeadName != "" {
		t.Fatalf("updated %+v", c)
	}
	// Deleting Accounts moves its issues to Reporting.
	if err := cb.DeleteComponent(ctx, 10000, made.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := componentByID(t, b, 10000); ok {
		t.Fatal("Accounts still listed")
	}
	issues, _, err := b.SearchIssuesPage(ctx, "", "", "", 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	moved := 0
	for _, iss := range issues {
		if slices.Contains(iss.Components, 10000) {
			t.Fatalf("%s still under Accounts", iss.Key)
		}
		if slices.Contains(iss.Components, made.ID) {
			moved++
		}
	}
	if moved == 0 {
		t.Fatal("no issue moved to Reporting")
	}
}
