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

func TestDemoBackendWritesInMemoryAndStagesOneConflict(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	conflictKey := b.ConflictKey()
	if conflictKey != "ACME-412" {
		t.Fatalf("conflict key = %s", conflictKey)
	}
	before, err := b.GetIssue(ctx, "ACME-409")
	if err != nil || before.Summary != "Rotate payment gateway API keys" {
		t.Fatalf("GetIssue: %+v %v", before, err)
	}
	if err := b.UpdateIssue(ctx, "ACME-409", map[string]string{"summary": "Rotate keys", "labels": "security, ops", "storyPoints": "5", "description": "Body"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	after, _ := b.GetIssue(ctx, "ACME-409")
	if after.Summary != "Rotate keys" || len(after.Labels) != 2 || *after.StoryPoints != 5 || after.Updated <= before.Updated {
		t.Errorf("after update: %+v", after)
	}
	d, _ := b.GetIssueDetail(ctx, "ACME-409")
	if d.Description != "Body" {
		t.Errorf("description overlay: %+v", d)
	}
	page, _, _ := b.SearchIssuesPage(ctx, "ACME", "", "", nil, 0, 100)
	seen := false
	for _, iss := range page {
		if iss.Key == "ACME-409" && iss.Summary == "Rotate keys" {
			seen = true
		}
	}
	if !seen {
		t.Error("search must reflect the overlay")
	}
	if err := b.UpdateIssue(ctx, "ACME-999", map[string]string{"summary": "x"}); err == nil {
		t.Error("unknown key must fail")
	}

	key, err := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeTask, Summary: "New one", Labels: []string{"x"}, StoryPoints: pts(2)})
	if err != nil || key != "ACME-500" {
		t.Fatalf("CreateIssue: %q %v", key, err)
	}
	key2, _ := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeBug, Summary: "Second"})
	if key2 != "ACME-501" {
		t.Errorf("keys count up: %s", key2)
	}
	created, err := b.GetIssue(ctx, key)
	if err != nil || created.Type != backend.TypeTask || created.Status != "To Do" || created.Project != "ACME" || *created.StoryPoints != 2 {
		t.Errorf("created: %+v %v", created, err)
	}

	// The staged conflict fires once: the first GetIssue of the key reports
	// a version later than the search showed, and the next read agrees with
	// the first.
	var base backend.Issue
	for _, iss := range page {
		if iss.Key == conflictKey {
			base = iss
		}
	}
	first, _ := b.GetIssue(ctx, conflictKey)
	second, _ := b.GetIssue(ctx, conflictKey)
	if first.Updated <= base.Updated || second.Updated != first.Updated {
		t.Errorf("staged conflict: base=%s first=%s second=%s", base.Updated, first.Updated, second.Updated)
	}

	specs, err := b.CreateFields(ctx, "ACME", backend.TypeBug)
	if err != nil || len(specs) != 1 || specs[0].Type != "option" || len(specs[0].AllowedValues) != 3 {
		t.Errorf("bug create fields: %+v %v", specs, err)
	}
	if specs, _ := b.CreateFields(ctx, "ACME", backend.TypeTask); len(specs) != 0 {
		t.Errorf("tasks need nothing extra: %+v", specs)
	}
}

func pts(v float64) *float64 { return &v }
