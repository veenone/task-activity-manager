package jira_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

// PLAT's Bug screen comes from the classic call, which cannot say what the
// screen carries, so both fields go.
func TestCreateIssueSendsComponentsAndFixVersions(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "PLAT-90"
	_, leftOut, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeBug, Summary: "Login fails", Components: []int{10000, 10001}, FixVersions: []int{10100},
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := postOf(f)
	for _, want := range []string{`"components":[{"id":"10000"},{"id":"10001"}]`, `"fixVersions":[{"id":"10100"}]`} {
		if !strings.Contains(post, want) {
			t.Errorf("POST lacks %s: %s", want, post)
		}
	}
	if len(leftOut) != 0 {
		t.Errorf("leftOut = %v", leftOut)
	}
}

// TKT's Story screen is a per-type answer listing neither field, so the
// create goes without them and names both.
func TestCreateIssueLeavesOutCatalogFieldsOffTheScreen(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "TKT-78"
	_, leftOut, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo code", Components: []int{10000}, FixVersions: []int{10100},
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := postOf(f)
	if post == "" || strings.Contains(post, "components") || strings.Contains(post, "fixVersions") {
		t.Fatalf("POST = %s, want it sent without either field", post)
	}
	if !reflect.DeepEqual(leftOut, []string{"Components", "Fix versions"}) {
		t.Errorf("leftOut = %v", leftOut)
	}
}
