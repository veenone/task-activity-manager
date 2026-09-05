package jira

import (
	"encoding/json"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

func TestParseSprintHandlesBothShapes(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantID   string
		wantName string
	}{
		{"objects, last is current", `[{"id":11,"name":"Sprint 11","state":"CLOSED"},{"id":12,"name":"Sprint 12","state":"ACTIVE"}]`, "12", "Sprint 12"},
		{"legacy strings", `["com.atlassian.greenhopper.service.sprint.Sprint@1a2b[id=11,rapidViewId=3,state=CLOSED,name=Sprint 11,startDate=2026-08-01T00:00:00.000Z,goal=]","com.atlassian.greenhopper.service.sprint.Sprint@3c4d[id=12,rapidViewId=3,state=ACTIVE,name=Sprint 12 - Checkout polish,goal=]"]`, "12", "Sprint 12 - Checkout polish"},
		{"single object", `{"id":13,"name":"Sprint 13","state":"FUTURE"}`, "13", "Sprint 13"},
		{"null", `null`, "", ""},
		{"absent", ``, "", ""},
		{"empty list", `[]`, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, name := parseSprint(json.RawMessage(c.raw))
			if id != c.wantID || name != c.wantName {
				t.Errorf("parseSprint = %q, %q; want %q, %q", id, name, c.wantID, c.wantName)
			}
		})
	}
}

func TestLogicalTypeIsCaseInsensitiveAndUsesTheRequirementName(t *testing.T) {
	cases := map[string]string{
		"Task": "task", "task": "task", "EPIC": "epic", "Story": "story", "Bug": "bug",
		"Business Requirement": "requirement", "Requirement": "", "Sub-task": "",
	}
	for name, want := range cases {
		if got := logicalType(name, "Business Requirement"); got != want {
			t.Errorf("logicalType(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestBuildJQL(t *testing.T) {
	names := jiraTypeNames(backend.AllTypes, "Requirement")
	got := buildJQL("PLAT", " labels = promo ", "2026-09-05T10:42:00Z", names)
	want := `project = "PLAT" AND issuetype in ("Task", "Epic", "Story", "Bug", "Requirement") AND (labels = promo) AND updated >= "2026-09-05 09:42" ORDER BY key ASC`
	if got != want {
		t.Errorf("jql =\n%s\nwant\n%s", got, want)
	}
	got = buildJQL("PLAT", "", "", jiraTypeNames([]string{backend.TypeBug}, "Requirement"))
	want = `project = "PLAT" AND issuetype in ("Bug") ORDER BY key ASC`
	if got != want {
		t.Errorf("minimal jql = %s", got)
	}
	if got := buildJQL("PLAT", "", "not a time", names); got != `project = "PLAT" AND issuetype in ("Task", "Epic", "Story", "Bug", "Requirement") ORDER BY key ASC` {
		t.Errorf("an unparseable since is dropped: %s", got)
	}
}

func TestParseIssueMapsEveryColumn(t *testing.T) {
	raw := corejira.RawIssue{ID: "10412", Key: "PLAT-412", Fields: map[string]json.RawMessage{
		"summary":           json.RawMessage(`"Checkout: apply promo code at payment step"`),
		"status":            json.RawMessage(`{"name":"In Progress"}`),
		"assignee":          json.RawMessage(`{"name":"ranand","displayName":"R. Anand"}`),
		"reporter":          json.RawMessage(`{"name":"po","displayName":"Product Owner"}`),
		"priority":          json.RawMessage(`{"name":"High"}`),
		"labels":            json.RawMessage(`["checkout","promo"]`),
		"issuetype":         json.RawMessage(`{"name":"Story"}`),
		"project":           json.RawMessage(`{"key":"PLAT"}`),
		"created":           json.RawMessage(`"2026-08-01T09:00:00.000+0000"`),
		"updated":           json.RawMessage(`"2026-09-05T09:58:00.000+0000"`),
		"customfield_10020": json.RawMessage(`[{"id":12,"name":"Sprint 12","state":"ACTIVE"}]`),
		"customfield_10016": json.RawMessage(`5`),
		"customfield_10014": json.RawMessage(`"PLAT-350"`),
		"customfield_10019": json.RawMessage(`"0|i0002:"`),
	}}
	ids := fieldIDs{Sprint: "customfield_10020", Points: "customfield_10016", EpicLink: "customfield_10014", Rank: "customfield_10019"}
	iss := parseIssue(raw, ids, "Requirement")
	if iss.Key != "PLAT-412" || iss.ID != "10412" || iss.Project != "PLAT" || iss.Type != "story" ||
		iss.Summary != "Checkout: apply promo code at payment step" || iss.Status != "In Progress" ||
		iss.Assignee != "R. Anand" || iss.Reporter != "Product Owner" || iss.Priority != "High" ||
		len(iss.Labels) != 2 || iss.SprintID != "12" || iss.SprintName != "Sprint 12" ||
		iss.ParentKey != "PLAT-350" || iss.StoryPoints == nil || *iss.StoryPoints != 5 ||
		iss.Rank != "0|i0002:" || iss.Created != "2026-08-01T09:00:00.000+0000" {
		t.Errorf("issue = %+v", iss)
	}
}

func TestParseIssuePrefersParentOverEpicLinkAndToleratesMissingFields(t *testing.T) {
	raw := corejira.RawIssue{ID: "1", Key: "PLAT-1", Fields: map[string]json.RawMessage{
		"summary":           json.RawMessage(`"x"`),
		"issuetype":         json.RawMessage(`{"name":"Sub-task"}`),
		"parent":            json.RawMessage(`{"key":"PLAT-412"}`),
		"customfield_10014": json.RawMessage(`"PLAT-350"`),
		"assignee":          json.RawMessage(`null`),
		"labels":            json.RawMessage(`null`),
	}}
	iss := parseIssue(raw, fieldIDs{EpicLink: "customfield_10014"}, "Requirement")
	if iss.ParentKey != "PLAT-412" {
		t.Errorf("parent = %q, want the parent field to win", iss.ParentKey)
	}
	if iss.Type != "" {
		t.Errorf("type = %q, want empty for an unknown Jira type", iss.Type)
	}
	if iss.Assignee != "" || iss.Labels == nil || len(iss.Labels) != 0 || iss.StoryPoints != nil {
		t.Errorf("issue = %+v", iss)
	}
}

func TestParseLinksSeesBothDirections(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":{"name":"Tested By","inward":"is tested by","outward":"tests"},"inwardIssue":{"key":"XT-1018","fields":{"summary":"Promo code applies discount","issuetype":{"name":"Test"}}}},
		{"type":{"name":"Relates"},"outwardIssue":{"key":"PLAT-388","fields":{"summary":"Promo codes must be single-use","issuetype":{"name":"Requirement"}}}}
	]`)
	links := parseLinks(raw)
	if len(links) != 2 {
		t.Fatalf("links = %+v", links)
	}
	if links[0] != (backend.Link{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "Promo code applies discount", IssueType: "Test"}) {
		t.Errorf("first = %+v", links[0])
	}
	if links[1] != (backend.Link{Direction: "outward", Type: "Relates", Key: "PLAT-388", Summary: "Promo codes must be single-use", IssueType: "Requirement"}) {
		t.Errorf("second = %+v", links[1])
	}
	if got := parseLinks(nil); got == nil || len(got) != 0 {
		t.Errorf("nil raw should give an empty slice, got %#v", got)
	}
}
