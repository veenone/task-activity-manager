package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestLinkTypesAreReadOnceAndCached(t *testing.T) {
	b, f := newBackend(t, twoFields)
	types, err := b.LinkTypes(context.Background())
	if err != nil || len(types) != 2 || types[0].Name != "Blocks" || types[0].Inward != "is blocked by" || types[1].Outward != "relates to" {
		t.Fatalf("LinkTypes: %+v %v", types, err)
	}
	f.linkTypes = `{"issueLinkTypes":[]}`
	again, _ := b.LinkTypes(context.Background())
	if len(again) != 2 {
		t.Error("second call must come from the cache")
	}
}

func TestCreateLinkPutsTheSourceOnTheRightSide(t *testing.T) {
	b, f := newBackend(t, twoFields)
	ctx := context.Background()
	if err := b.CreateLink(ctx, "PLAT-412", backend.LinkDraft{Type: "Blocks", Direction: "outward", ToKey: "PAY-77"}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateLink(ctx, "PLAT-412", backend.LinkDraft{Type: "Blocks", Direction: "inward", ToKey: "PAY-78"}); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 2 || !strings.HasPrefix(f.writes[0], "POST /rest/api/2/issueLink ") {
		t.Fatalf("writes = %v", f.writes)
	}
	if !strings.Contains(f.writes[0], `"outwardIssue":{"key":"PLAT-412"}`) || !strings.Contains(f.writes[0], `"inwardIssue":{"key":"PAY-77"}`) || !strings.Contains(f.writes[0], `"type":{"name":"Blocks"}`) {
		t.Errorf("outward body: %s", f.writes[0])
	}
	if !strings.Contains(f.writes[1], `"outwardIssue":{"key":"PAY-78"}`) || !strings.Contains(f.writes[1], `"inwardIssue":{"key":"PLAT-412"}`) {
		t.Errorf("inward body: %s", f.writes[1])
	}
	if err := b.CreateLink(ctx, "PLAT-412", backend.LinkDraft{Type: "Blocks", Direction: "sideways", ToKey: "PAY-79"}); err == nil {
		t.Error("bad direction must be refused before any request")
	}
}
