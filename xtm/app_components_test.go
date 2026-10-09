package main

import (
	"errors"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

func TestComponentBindingsInDemo(t *testing.T) {
	a := newTestApp(t)
	p, err := a.CreateProfile("Demo", "demo", "DEMO", "", "", "", "", "tok", "", false, "xray")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	created, err := a.CreateComponent(p.ID, backend.ComponentInput{Name: "BindingTest"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = a.DeleteComponent(p.ID, created.ID, "") })
	opts, err := a.ListProjectComponents(p.ID, "DEMO")
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	found := false
	for _, o := range opts {
		if o == "BindingTest" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cached options %v lack the new component", opts)
	}
}

func TestComponentBindingsUnsupportedOnKiwi(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)
	_, err := a.CreateComponent(profileID, backend.ComponentInput{Name: "X"})
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}

func TestSetTestComponentsBindingQueuesEdit(t *testing.T) {
	a := newTestApp(t)
	p, err := a.CreateProfile("Demo", "demo", "DEMO", "", "", "", "", "tok", "", false, "xray")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.repo.UpsertTests(p.ID, []testrepo.TestCase{{Key: "DEMO-1", Summary: "a"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.SetTestComponents(p.ID, "DEMO-1", []string{"User Management"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := a.ListTestComponents(p.ID, []string{"DEMO-1"})
	if err != nil || len(got["DEMO-1"]) != 1 || got["DEMO-1"][0] != "User Management" {
		t.Fatalf("got %v err %v", got, err)
	}
}
