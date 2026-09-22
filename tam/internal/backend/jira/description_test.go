package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

// The sync reads the description with the rest of the row, so the detail
// panel can draw it from the local store with no call of its own.
func TestTheSyncsSearchAsksForTheDescription(t *testing.T) {
	b, f := newBackend(t, twoFields)
	if _, _, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", backend.AllTypes, 0, 50); err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(f.searches) != 1 {
		t.Fatalf("searches = %v", f.searches)
	}
	fields := f.searches[0][strings.Index(f.searches[0], "| fields=")+len("| fields="):]
	if !strings.Contains(fields, "description") {
		t.Errorf("search fields = %q, want the description among them", fields)
	}
}

// The three shapes the field arrives in, and the one that must not be
// flattened into the others: text, Jira's null for an issue with no
// description, and a response that did not carry the key at all.
func TestADescriptionJiraDidNotSendIsNotAnEmptyOne(t *testing.T) {
	b, f := newBackend(t, twoFields)
	page, _, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", backend.AllTypes, 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if page[0].Description == nil || *page[0].Description != "Apply the code at the payment step." {
		t.Errorf("PLAT-412 description = %v, want the text Jira sent", page[0].Description)
	}
	if page[1].Description == nil || *page[1].Description != "" {
		t.Errorf("PLAT-388 description = %v, want an empty string: Jira answered null, which means the issue has none", page[1].Description)
	}

	f.searchBody = `{"total":1,"issues":[{"id":"1","key":"PLAT-412","fields":{"summary":"Promo","status":{"name":"In Progress"},"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[]}}]}`
	silent, _, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", backend.AllTypes, 0, 50)
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if silent[0].Description != nil {
		t.Errorf("description = %q for a response that carried no description key, want nil: nothing was read, which is not the same as reading nothing", *silent[0].Description)
	}
}

// GetIssue parses the same row the search does, so the refresh after a
// commit brings the description back with it rather than dropping the
// column the panel reads.
func TestGetIssueCarriesTheDescription(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	iss, err := b.GetIssue(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.Description == nil || *iss.Description != "As a shopper" {
		t.Errorf("description = %v", iss.Description)
	}
}
