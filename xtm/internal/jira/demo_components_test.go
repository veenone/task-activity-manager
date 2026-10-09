package jira

import (
	"context"
	"sync"
	"testing"
)

func demoNames(t *testing.T, c *Client) []string {
	t.Helper()
	list, err := c.ProjectComponents(context.Background(), "DEMO")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return list
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestDemoComponentsCreateRenameDelete(t *testing.T) {
	resetDemoComponents()
	t.Cleanup(resetDemoComponents)
	ctx := context.Background()

	// A fresh client per call, as app.go does, must see earlier writes.
	created, err := NewClient("demo", "t").CreateComponent(ctx, ComponentInput{Project: "DEMO", Name: "Search"})
	if err != nil || created.ID == "" {
		t.Fatalf("create %+v %v", created, err)
	}
	if !contains(demoNames(t, NewClient("demo", "t")), "Search") {
		t.Fatal("created component not listed")
	}

	if _, err := NewClient("demo", "t").UpdateComponent(ctx, created.ID, ComponentInput{Name: "Find"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	names := demoNames(t, NewClient("demo", "t"))
	if contains(names, "Search") || !contains(names, "Find") {
		t.Fatalf("rename not applied: %v", names)
	}

	if err := NewClient("demo", "t").DeleteComponent(ctx, created.ID, ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if contains(demoNames(t, NewClient("demo", "t")), "Find") {
		t.Fatal("deleted component still listed")
	}
}

func TestDemoComponentListMatchesDemoTests(t *testing.T) {
	resetDemoComponents()
	t.Cleanup(resetDemoComponents)
	names := demoNames(t, NewClient("demo", "t"))
	for i := 0; i < 30; i++ {
		for _, n := range demoComponentsForIndex(i) {
			if !contains(names, n) {
				t.Fatalf("demo test component %q is not in the project list %v", n, names)
			}
		}
	}
}

func TestDemoComponentsConcurrentDeletes(t *testing.T) {
	resetDemoComponents()
	t.Cleanup(resetDemoComponents)
	ctx := context.Background()
	list, _ := NewClient("demo", "t").ProjectComponentDetails(ctx, "DEMO")
	var wg sync.WaitGroup
	for _, comp := range list[:2] {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_ = NewClient("demo", "t").DeleteComponent(ctx, id, "")
		}(comp.ID)
	}
	wg.Wait()
	after, _ := NewClient("demo", "t").ProjectComponentDetails(ctx, "DEMO")
	if len(after) != len(list)-2 {
		t.Fatalf("want %d components, got %d", len(list)-2, len(after))
	}
}
