package demo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

func TestDemoBackendPagesTheWholeDataset(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	if !b.IsDemo() {
		t.Fatal("IsDemo = false")
	}
	u, err := b.TestConnection(ctx)
	if err != nil || u.Name != "demo" {
		t.Fatalf("connection = %+v, %v", u, err)
	}
	var got []backend.Issue
	total := -1
	for start := 0; total < 0 || start < total; {
		page, n, err := b.SearchIssuesPage(ctx, "PLAT", "", "", backend.AllTypes, start, 25)
		if err != nil {
			t.Fatalf("page at %d: %v", start, err)
		}
		total = n
		if len(page) == 0 {
			break
		}
		got = append(got, page...)
		start += len(page)
	}
	if total != 60 || len(got) != 60 {
		t.Errorf("total %d, fetched %d, want 60 and 60", total, len(got))
	}
}

func TestDemoBackendFiltersByTypeAndIgnoresScopeAndSince(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	page, total, err := b.SearchIssuesPage(ctx, "PLAT", "labels = nothing", "2030-01-01T00:00:00Z", []string{backend.TypeEpic}, 0, 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 4 || len(page) != 4 {
		t.Errorf("epics: total %d, rows %d, want 4 (scope and since are ignored)", total, len(page))
	}
	for _, iss := range page {
		if iss.Type != backend.TypeEpic {
			t.Errorf("non-epic in result: %+v", iss)
		}
	}
}

func TestDemoBackendDetailAndTypes(t *testing.T) {
	b := demobackend.New("DEMO")
	ctx := context.Background()
	d, err := b.GetIssueDetail(ctx, "DEMO-412")
	if err != nil || len(d.Links) != 3 {
		t.Errorf("detail = %+v, %v", d, err)
	}
	if _, err := b.GetIssueDetail(ctx, "DEMO-9999"); err == nil {
		t.Error("unknown key should fail")
	}
	types, err := b.IssueTypes(ctx, "DEMO")
	if err != nil || len(types) != 5 || types[4].Name != "Requirement" {
		t.Errorf("types = %+v, %v", types, err)
	}
}
