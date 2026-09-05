package jira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// DefaultRequirementType is the Jira issue type name TAM assumes for
// requirements when the profile has not set one.
const DefaultRequirementType = "Requirement"

// baseFields are the search fields the grid needs, before the custom ones.
var baseFields = []string{"summary", "status", "assignee", "reporter", "priority", "labels", "issuetype", "project", "parent", "created", "updated"}

// fieldIDs are the discovered custom field ids. Any may be empty when the
// instance lacks the field.
type fieldIDs struct {
	Sprint   string
	Points   string
	EpicLink string
	Rank     string
}

func (f fieldIDs) list() []string {
	var out []string
	for _, id := range []string{f.Sprint, f.Points, f.EpicLink, f.Rank} {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

// logicalType maps a Jira issue type name to one of the five TAM types, or
// "" when it is none of them.
func logicalType(name, requirementType string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "task":
		return backend.TypeTask
	case "epic":
		return backend.TypeEpic
	case "story":
		return backend.TypeStory
	case "bug":
		return backend.TypeBug
	}
	if n != "" && n == strings.ToLower(strings.TrimSpace(requirementType)) {
		return backend.TypeRequirement
	}
	return ""
}

// jiraTypeNames turns logical types into the Jira names the JQL quotes.
func jiraTypeNames(types []string, requirementType string) []string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		switch t {
		case backend.TypeTask:
			names = append(names, "Task")
		case backend.TypeEpic:
			names = append(names, "Epic")
		case backend.TypeStory:
			names = append(names, "Story")
		case backend.TypeBug:
			names = append(names, "Bug")
		case backend.TypeRequirement:
			names = append(names, requirementType)
		}
	}
	return names
}

// buildJQL is the sync scope: the project, the type list, the profile's
// scope JQL in parentheses when set, the incremental clause when since
// parses, and a stable order by key so paging never skips an issue.
func buildJQL(projectKey, scopeJQL, since string, typeNames []string) string {
	quoted := make([]string, len(typeNames))
	for i, n := range typeNames {
		quoted[i] = strconv.Quote(n)
	}
	jql := fmt.Sprintf("project = %s AND issuetype in (%s)", projectKey, strings.Join(quoted, ", "))
	if s := strings.TrimSpace(scopeJQL); s != "" {
		jql += " AND (" + s + ")"
	}
	if extra := sinceClause(since); extra != "" {
		jql += " AND " + extra
	}
	return jql + " ORDER BY key ASC"
}

// sinceClause backdates the cut-off by an hour, as XTM does, so clock skew
// between the app and Jira cannot hide an update. An unparseable value
// yields no clause, which makes the sync a full pull rather than a wrong one.
func sinceClause(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`updated >= "%s"`, t.UTC().Add(-time.Hour).Format("2006-01-02 15:04"))
}

type named struct {
	Name string `json:"name"`
}

type userRef struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type keyed struct {
	Key string `json:"key"`
}

// parseIssue maps one raw search hit onto the grid row.
func parseIssue(raw corejira.RawIssue, ids fieldIDs, requirementType string) backend.Issue {
	iss := backend.Issue{Key: raw.Key, ID: raw.ID, Labels: []string{}}
	f := raw.Fields
	_ = json.Unmarshal(f["summary"], &iss.Summary)
	_ = json.Unmarshal(f["created"], &iss.Created)
	_ = json.Unmarshal(f["updated"], &iss.Updated)
	var labels []string
	if err := json.Unmarshal(f["labels"], &labels); err == nil && labels != nil {
		iss.Labels = labels
	}
	var status, priority, issueType named
	if err := json.Unmarshal(f["status"], &status); err == nil {
		iss.Status = status.Name
	}
	if err := json.Unmarshal(f["priority"], &priority); err == nil {
		iss.Priority = priority.Name
	}
	if err := json.Unmarshal(f["issuetype"], &issueType); err == nil {
		iss.Type = logicalType(issueType.Name, requirementType)
	}
	var assignee, reporter *userRef
	if err := json.Unmarshal(f["assignee"], &assignee); err == nil && assignee != nil {
		iss.Assignee = displayName(*assignee)
	}
	if err := json.Unmarshal(f["reporter"], &reporter); err == nil && reporter != nil {
		iss.Reporter = displayName(*reporter)
	}
	var project, parent *keyed
	if err := json.Unmarshal(f["project"], &project); err == nil && project != nil {
		iss.Project = project.Key
	}
	if err := json.Unmarshal(f["parent"], &parent); err == nil && parent != nil && parent.Key != "" {
		iss.ParentKey = parent.Key
	} else if ids.EpicLink != "" {
		var epic string
		if err := json.Unmarshal(f[ids.EpicLink], &epic); err == nil {
			iss.ParentKey = epic
		}
	}
	if ids.Sprint != "" {
		iss.SprintID, iss.SprintName = parseSprint(f[ids.Sprint])
	}
	if ids.Points != "" {
		var pts *float64
		if err := json.Unmarshal(f[ids.Points], &pts); err == nil && pts != nil {
			iss.StoryPoints = pts
		}
	}
	if ids.Rank != "" {
		_ = json.Unmarshal(f[ids.Rank], &iss.Rank)
	}
	return iss
}

func displayName(u userRef) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Name
}

// Legacy Sprint values are toString dumps of the GreenHopper sprint object.
var (
	legacySprintID   = regexp.MustCompile(`\bid=(\d+)`)
	legacySprintName = regexp.MustCompile(`\bname=([^,\]]*)`)
)

// parseSprint reads the Sprint custom field in either shape Jira DC uses
// (an array of objects on Jira Software 8.x and later, an array of
// toString dumps before that; a single object also occurs) and returns the
// last entry, which is the sprint the issue is in now.
//
// NOTE(tam): both shapes come from documentation and XTM's field notes, not
// from a live instance. Verify against a real Jira DC before Phase 3 builds
// sprint entities on top of this.
func parseSprint(raw json.RawMessage) (id, name string) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", ""
	}
	type sprintObj struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	}
	var objs []sprintObj
	if err := json.Unmarshal(raw, &objs); err == nil {
		if len(objs) == 0 {
			return "", ""
		}
		last := objs[len(objs)-1]
		return last.ID.String(), last.Name
	}
	var one sprintObj
	if err := json.Unmarshal(raw, &one); err == nil && (one.ID != "" || one.Name != "") {
		return one.ID.String(), one.Name
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil && len(strs) > 0 {
		last := strs[len(strs)-1]
		if m := legacySprintID.FindStringSubmatch(last); m != nil {
			id = m[1]
		}
		if m := legacySprintName.FindStringSubmatch(last); m != nil {
			name = strings.TrimSpace(m[1])
		}
		return id, name
	}
	return "", ""
}

// parseLinks flattens Jira's issuelinks array: each entry names the link
// type and carries either an inwardIssue or an outwardIssue.
func parseLinks(raw json.RawMessage) []backend.Link {
	out := []backend.Link{}
	if len(raw) == 0 {
		return out
	}
	type linked struct {
		Key    string `json:"key"`
		Fields struct {
			Summary   string `json:"summary"`
			IssueType named  `json:"issuetype"`
		} `json:"fields"`
	}
	var entries []struct {
		Type    named   `json:"type"`
		Inward  *linked `json:"inwardIssue"`
		Outward *linked `json:"outwardIssue"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return out
	}
	for _, e := range entries {
		if e.Inward != nil {
			out = append(out, backend.Link{Direction: "inward", Type: e.Type.Name, Key: e.Inward.Key, Summary: e.Inward.Fields.Summary, IssueType: e.Inward.Fields.IssueType.Name})
		}
		if e.Outward != nil {
			out = append(out, backend.Link{Direction: "outward", Type: e.Type.Name, Key: e.Outward.Key, Summary: e.Outward.Fields.Summary, IssueType: e.Outward.Fields.IssueType.Name})
		}
	}
	return out
}
