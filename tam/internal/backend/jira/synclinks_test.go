package jira

import (
	"encoding/json"
	"slices"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// The sync reads every issue's links, with Jira's link id, so the store can
// draw the project's dependencies and remove a link later (#612).
func TestTheSyncAsksForAndKeepsEveryIssuesLinks(t *testing.T) {
	if !slices.Contains(baseFields, "issuelinks") {
		t.Fatalf("baseFields = %v, want issuelinks among them", baseFields)
	}
	raw := corejira.RawIssue{ID: "1", Key: "PLAT-1", Fields: map[string]json.RawMessage{
		"summary":    json.RawMessage(`"Checkout"`),
		"issuelinks": json.RawMessage(`[{"id":"10431","type":{"name":"Blocks"},"outwardIssue":{"key":"PLAT-2","fields":{"summary":"Pay","issuetype":{"name":"Story"}}}}]`),
	}}
	iss := parseIssue(raw, fieldIDs{}, "Requirement", projectTypes{})
	want := []backend.Link{{ID: "10431", Direction: "outward", Type: "Blocks", Key: "PLAT-2", Summary: "Pay", IssueType: "Story"}}
	if !slices.Equal(iss.Links, want) {
		t.Errorf("links = %+v, want %+v", iss.Links, want)
	}
	// A response without the field says nothing about the issue's links.
	none := parseIssue(corejira.RawIssue{Key: "PLAT-3", Fields: map[string]json.RawMessage{}}, fieldIDs{}, "Requirement", projectTypes{})
	if none.Links != nil {
		t.Errorf("links without the field = %#v, want nil", none.Links)
	}
}
