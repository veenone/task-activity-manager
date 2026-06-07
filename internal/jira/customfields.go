package jira

import (
	"context"
	"fmt"
)

// CustomFieldDef describes a Jira custom field configured for the Test issue
// type (FR-2.6): its Jira id (e.g. "customfield_10001"), display name and a
// coarse type hint.
type CustomFieldDef struct {
	ID   string
	Name string
	Type string
}

// ListCustomFields returns the custom fields configured for the project's Test
// issue type. Demo URLs short-circuit to a fixed set; the real-Jira call is a
// best-effort no-op pending verification against an actual instance.
//
// TODO(xtm): derive from /rest/api/2/issue/createmeta (or editmeta) filtered to
// the Test issue type once the response can be verified on a live instance.
func (c *Client) ListCustomFields(ctx context.Context, projectKey string) ([]CustomFieldDef, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return demoCustomFieldDefs(), nil
	}
	return nil, nil
}

// GetTestCustomFields returns a Test's custom field values, keyed by field id
// (FR-2.6). Demo URLs generate deterministic values; the real-Jira call is a
// best-effort no-op pending verification.
//
// TODO(xtm): read from /rest/api/2/issue/{key}?fields=<ids> and stringify each
// value per its type once verified on a live instance.
func (c *Client) GetTestCustomFields(ctx context.Context, testKey string) (map[string]string, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return demoTestCustomFields(testKey), nil
	}
	return map[string]string{}, nil
}

// demoCustomFieldDefs is the fixed set of custom fields demo mode exposes on
// the Test issue type.
func demoCustomFieldDefs() []CustomFieldDef {
	return []CustomFieldDef{
		{ID: "customfield_10100", Name: "Test Type", Type: "option"},
		{ID: "customfield_10101", Name: "Automation Status", Type: "option"},
		{ID: "customfield_10102", Name: "Component", Type: "string"},
		{ID: "customfield_10103", Name: "Estimated Duration (min)", Type: "number"},
	}
}

var demoTestTypes = []string{"Manual", "Generic", "Cucumber"}
var demoAutomationStatuses = []string{"Not Automated", "In Progress", "Automated"}
var demoComponents = []string{"Frontend", "Backend", "API", "Database", "Auth"}

// demoTestCustomFields produces deterministic custom field values for a Test so
// repeated opens are stable.
func demoTestCustomFields(testKey string) map[string]string {
	h := 0
	for _, r := range testKey {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return map[string]string{
		"customfield_10100": demoTestTypes[h%len(demoTestTypes)],
		"customfield_10101": demoAutomationStatuses[(h/3)%len(demoAutomationStatuses)],
		"customfield_10102": demoComponents[(h/7)%len(demoComponents)],
		"customfield_10103": fmt.Sprintf("%d", 5+(h%55)),
	}
}
