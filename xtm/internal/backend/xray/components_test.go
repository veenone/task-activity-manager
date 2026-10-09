package xray

import (
	"context"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

func TestAdapterManagesComponentsInDemo(t *testing.T) {
	a := New(jira.NewClient("demo", "t"))
	var b backend.Backend = a
	cm, ok := b.(backend.ComponentManager)
	if !ok {
		t.Fatal("xray adapter does not implement ComponentManager")
	}
	if !a.Capabilities().SupportsComponentAdmin {
		t.Fatal("SupportsComponentAdmin is false")
	}
	created, err := cm.CreateComponent(context.Background(), "DEMO",
		backend.ComponentInput{Name: "AdapterTest", AssigneeType: "PROJECT_DEFAULT"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = cm.DeleteComponent(context.Background(), created.ID, "") })
	list, err := cm.ProjectComponentDetails(context.Background(), "DEMO")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, c := range list {
		if c.ID == created.ID && c.Name == "AdapterTest" && c.AssigneeType == "PROJECT_DEFAULT" {
			found = true
		}
	}
	if !found {
		t.Fatalf("created component missing from %+v", list)
	}
}
