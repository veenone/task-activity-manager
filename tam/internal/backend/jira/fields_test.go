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
		"Business Requirement": "requirement", "Requirement": "",
		// The sub-task type is whatever the project calls it, so the generic
		// name is not one unless the project says so.
		"Technical task": "subtask", "TECHNICAL TASK": "subtask", "Sub-task": "",
		// The project's own word for the task level, alongside the built-in.
		"Todo": "task", "TODO": "task",
	}
	for name, want := range cases {
		if got := logicalType(name, "Business Requirement", projectTypes{task: "Todo", subtask: "Technical task"}); got != want {
			t.Errorf("logicalType(%q) = %q, want %q", name, got, want)
		}
	}
	// A project with no sub-task type maps nothing to one.
	if got := logicalType("Technical task", "Requirement", projectTypes{}); got != "" {
		t.Errorf("no sub-task type = %q, want empty", got)
	}
	// A built-in name keeps its meaning even if the instance reuses it.
	if got := logicalType("Bug", "Requirement", projectTypes{subtask: "Bug"}); got != backend.TypeBug {
		t.Errorf("a built-in name keeps its meaning: %q", got)
	}
}

func TestBuildJQLExcludesRatherThanEnumerates(t *testing.T) {
	// With nothing to exclude the project alone is the scope. A type the
	// project gains tomorrow is in it without TAM being taught the name,
	// which is the whole difference from the enumerated list that fetched
	// 38 issues of 2,943 (#68).
	if got := buildJQL("PLAT", "", "", nil); got != `project = "PLAT" ORDER BY key ASC` {
		t.Errorf("bare jql = %s", got)
	}
	got := buildJQL("PLAT", " labels = promo ", "2026-09-05T10:42:00Z", []string{"Test", "Test Set"})
	want := `project = "PLAT" AND issuetype not in ("Test", "Test Set") AND (labels = promo) AND updated >= "2026-09-05 09:42" ORDER BY key ASC`
	if got != want {
		t.Errorf("jql = %s, want %s", got, want)
	}
	if got := buildJQL("PLAT", "", "not a time", nil); got != `project = "PLAT" ORDER BY key ASC` {
		t.Errorf("an unparseable since is dropped: %s", got)
	}
}

// The ceiling of the icon heuristic, written as cases rather than as a
// sentence nobody runs. Xray's types are drawn from the plugin's own
// bundled resources, so their icon URL carries the plugin key; nothing else
// in the project response says where a type came from.
func TestXrayTypesAreRecognisedByTheirPluginIcon(t *testing.T) {
	cases := map[string]bool{
		"https://jira.example/download/resources/com.xpandit.plugins.xray:xray-issue-type-resources/images/test.png": true,
		"https://jira.example/download/resources/com.xpandit.plugins.xray:xray-resources/images/precondition.png":    true,
		"HTTPS://JIRA.EXAMPLE/DOWNLOAD/RESOURCES/COM.XPANDIT.PLUGINS.XRAY:XRAY-ISSUE-TYPE-RESOURCES/IMAGES/TEST.PNG": true,
		"https://jira.example/secure/viewavatar?size=xsmall&avatarId=10318&avatarType=issuetype":                     false,
		"": false,
		// The ceiling. An instance that replaces an Xray type's icon with an
		// uploaded avatar looks like any other type here, and its Tests are
		// synced into TAM. There is no second signal in this response to
		// check it against.
		"https://jira.example/secure/viewavatar?size=xsmall&avatarId=10999&avatarType=issuetype#Test": false,
		// The other half of the same ceiling: a type of the project's own
		// that was given an Xray icon is excluded from the sync.
		"https://jira.example/download/resources/com.xpandit.plugins.xray:xray-issue-type-resources/images/test.png?id=own": true,
	}
	for icon, want := range cases {
		if got := isXrayType(icon); got != want {
			t.Errorf("isXrayType(%q) = %v, want %v", icon, got, want)
		}
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
	iss := parseIssue(raw, ids, "Requirement", projectTypes{task: "Task", subtask: "Technical task"})
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
	iss := parseIssue(raw, fieldIDs{EpicLink: "customfield_10014"}, "Requirement", projectTypes{task: "Task", subtask: "Technical task"})
	if iss.ParentKey != "PLAT-412" {
		t.Errorf("parent = %q, want the parent field to win", iss.ParentKey)
	}
	// "Sub-task" is not this project's word for its sub-task level, so TAM
	// has no logical type for it and the row keeps the name Jira gave it.
	if iss.Type != "Sub-task" {
		t.Errorf("type = %q, want the Jira name TAM does not model", iss.Type)
	}
	if iss.Assignee != "" || iss.Labels == nil || len(iss.Labels) != 0 || iss.StoryPoints != nil {
		t.Errorf("issue = %+v", iss)
	}
	if iss.AssigneeName != "" {
		t.Errorf("assignee name = %q, want empty when there is no assignee", iss.AssigneeName)
	}
}

// TestParseIssueAssigneeNameFromTheUsername is the ordinary case: an
// assignee's own name field is the username the write path and the
// "assigned to me" match both need, distinct from the display name Assignee
// already carries.
func TestParseIssueAssigneeNameFromTheUsername(t *testing.T) {
	raw := corejira.RawIssue{ID: "1", Key: "PLAT-412", Fields: map[string]json.RawMessage{
		"summary":  json.RawMessage(`"x"`),
		"assignee": json.RawMessage(`{"name":"ranand","displayName":"R. Anand"}`),
		"labels":   json.RawMessage(`null`),
	}}
	iss := parseIssue(raw, fieldIDs{}, "Requirement", projectTypes{})
	if iss.Assignee != "R. Anand" {
		t.Errorf("assignee = %q, want the display name unchanged", iss.Assignee)
	}
	if iss.AssigneeName != "ranand" {
		t.Errorf("assignee name = %q, want the username", iss.AssigneeName)
	}
}

// TestParseIssueAssigneeNameFallsBackToKey covers Data Center in GDPR mode,
// which populates key rather than name for some users.
func TestParseIssueAssigneeNameFallsBackToKey(t *testing.T) {
	raw := corejira.RawIssue{ID: "1", Key: "PLAT-412", Fields: map[string]json.RawMessage{
		"summary":  json.RawMessage(`"x"`),
		"assignee": json.RawMessage(`{"key":"ranand-key","displayName":"R. Anand"}`),
		"labels":   json.RawMessage(`null`),
	}}
	iss := parseIssue(raw, fieldIDs{}, "Requirement", projectTypes{})
	if iss.AssigneeName != "ranand-key" {
		t.Errorf("assignee name = %q, want the key when name is empty", iss.AssigneeName)
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

func TestParseIssueCarriesTheStatusNameAndItsID(t *testing.T) {
	raw := corejira.RawIssue{ID: "1", Key: "PLAT-412", Fields: map[string]json.RawMessage{
		"summary":   json.RawMessage(`"Checkout: apply promo code"`),
		"status":    json.RawMessage(`{"id":"3","name":"In Progress"}`),
		"issuetype": json.RawMessage(`{"id":"7","name":"Story"}`),
		"project":   json.RawMessage(`{"key":"PLAT"}`),
		"labels":    json.RawMessage(`["promo"]`),
	}}
	iss := parseIssue(raw, fieldIDs{}, "Requirement", projectTypes{task: "Task", subtask: "Technical task"})
	if iss.Status != "In Progress" {
		t.Errorf("status = %q, want In Progress", iss.Status)
	}
	if iss.StatusID != "3" {
		t.Errorf("status id = %q, want 3: the board's whole join is by status id", iss.StatusID)
	}
	if iss.Type != backend.TypeStory {
		t.Errorf("type = %q", iss.Type)
	}

	// A status Jira sent without an id leaves the column empty rather than
	// failing the parse.
	raw.Fields["status"] = json.RawMessage(`{"name":"In Progress"}`)
	if iss := parseIssue(raw, fieldIDs{}, "Requirement", projectTypes{task: "Task", subtask: "Technical task"}); iss.StatusID != "" || iss.Status != "In Progress" {
		t.Errorf("status without an id = %q, %q", iss.Status, iss.StatusID)
	}
}

// A type TAM has no logical type for keeps the project's own name for it,
// the rule #67 set for the New issue dialog, so the grid, the filter and
// the tree all get a value they can show and filter on rather than the
// empty string the sync used to drop the row for.
func TestParseIssueKeepsATypeTAMDoesNotModel(t *testing.T) {
	raw := corejira.RawIssue{ID: "1", Key: "PLAT-1", Fields: map[string]json.RawMessage{
		"summary":   json.RawMessage(`"Trim the bundle"`),
		"issuetype": json.RawMessage(`{"name":"Improvement"}`),
	}}
	if iss := parseIssue(raw, fieldIDs{}, "Requirement", projectTypes{task: "Task"}); iss.Type != "Improvement" {
		t.Errorf("type = %q, want the project's own name", iss.Type)
	}
}

// The status object already carries its category, so reading it costs no
// extra request. I1: the key is a value from the server, so only the three
// keys Jira defines are kept. Anything else leaves the column empty, which
// reads the same as a row synced before the column existed, and the chip
// falls back to guessing from the name.
func TestParseIssueKeepsTheStatusCategoryJiraSends(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{"new", `{"id":"1","name":"Zu erledigen","statusCategory":{"id":2,"key":"new","name":"To Do"}}`, "new"},
		{"indeterminate", `{"id":"3","name":"En cours","statusCategory":{"id":4,"key":"indeterminate","name":"In Progress"}}`, "indeterminate"},
		{"done", `{"id":"5","name":"Erledigt","statusCategory":{"id":3,"key":"done","name":"Done"}}`, "done"},
		{"a key this Jira invented", `{"id":"9","name":"Blocked","statusCategory":{"id":7,"key":"stalled"}}`, ""},
		{"no category at all", `{"id":"9","name":"Blocked"}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := corejira.RawIssue{ID: "1", Key: "PLAT-412", Fields: map[string]json.RawMessage{
				"summary": json.RawMessage(`"Checkout: apply promo code"`),
				"status":  json.RawMessage(c.status),
			}}
			iss := parseIssue(raw, fieldIDs{}, "Requirement", projectTypes{})
			if iss.StatusCategory != c.want {
				t.Errorf("status category = %q, want %q", iss.StatusCategory, c.want)
			}
		})
	}
}
