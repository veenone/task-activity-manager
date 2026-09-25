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
// description is among them so the sync caches it on the row: it is the one
// field the detail panel used to pay a round trip for on every selection.
var baseFields = []string{"summary", "description", "status", "assignee", "reporter", "priority", "labels", "issuetype", "project", "parent", "created", "updated"}

// fieldIDs are the discovered custom field ids. Any may be empty when the
// instance lacks the field.
type fieldIDs struct {
	Sprint   string
	Points   string
	EpicLink string
	EpicName string
	Rank     string

	// ambiguous holds the ids of every field answering to a name more than
	// one field carries, keyed by the name TAM asked for. The id is left
	// empty in that case, because picking one of two fields with the same
	// name is how an estimate ends up on a field nobody chose; this is what
	// lets a failure say which two.
	ambiguous map[string][]string
}

// pointsFieldName is the name TAM looks for when no board says which field a
// project estimates in.
const pointsFieldName = "Story Points"

func (f fieldIDs) list() []string {
	var out []string
	for _, id := range []string{f.Sprint, f.Points, f.EpicLink, f.Rank, f.EpicName} {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

// logicalType maps a Jira issue type name to one of TAM's types, or "" when
// it is none of them. subtaskType is the project's own name for its sub-task
// level, discovered rather than assumed: the default is "Sub-task", but an
// instance may call it anything, and one seen in the field calls it
// "Technical task".
func logicalType(name, requirementType string, pt projectTypes) string {
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
	// A built-in name above keeps its meaning, so an instance that reuses one
	// for a level of its own does not shadow it here.
	if n != "" && n == strings.ToLower(strings.TrimSpace(pt.subtask)) {
		return backend.TypeSubtask
	}
	// The project's own word for the task level, "Todo" on an instance seen
	// in the field. "Task" is already handled above, so this is what makes a
	// renamed task level read as one.
	if n != "" && n == strings.ToLower(strings.TrimSpace(pt.task)) {
		return backend.TypeTask
	}
	if n != "" && n == strings.ToLower(strings.TrimSpace(requirementType)) {
		return backend.TypeRequirement
	}
	return ""
}

// jiraTypeNames turns logical types into the Jira names the JQL quotes. A
// name the project does not define is dropped rather than quoted: Jira
// rejects the whole query when an issuetype in it does not exist, so asking
// for sub-tasks on a project without them would fail the sync instead of
// returning nothing.
func jiraTypeNames(types []string, requirementType string, pt projectTypes) []string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		switch t {
		case backend.TypeTask:
			// The project's own name for the level, so an instance that calls
			// it "Todo" is asked for the type it actually has. The default
			// only stands in when the project could not be read.
			if pt.task != "" {
				names = append(names, pt.task)
			} else {
				names = append(names, "Task")
			}
		case backend.TypeEpic:
			names = append(names, "Epic")
		case backend.TypeStory:
			names = append(names, "Story")
		case backend.TypeBug:
			names = append(names, "Bug")
		case backend.TypeRequirement:
			names = append(names, requirementType)
		case backend.TypeSubtask:
			if strings.TrimSpace(pt.subtask) != "" {
				names = append(names, pt.subtask)
			}
		}
	}
	return names
}

// xrayPluginKey is the plugin whose issue types belong to XTM, not to TAM.
// Xray's types cannot be recognised by name: an instance renames and
// localises them, which is why XTM discovers its own type names rather than
// hardcoding them (xtm/internal/jira/client.go). The icon a project serves
// for a type is the only field in that response that says which plugin
// defined it, because a plugin's types are drawn from its own bundled
// resources under this key.
const xrayPluginKey = "com.xpandit.plugins.xray"

// isXrayType says whether a project's issue type is one of Xray's, by the
// only signal the project endpoint carries.
//
// ponytail: icon heuristic with two failure modes, both tested. An instance
// that replaces an Xray type's icon with an uploaded avatar is not
// recognised, so its Tests sync into TAM; a type of the project's own given
// an Xray icon is excluded from the sync. Upgrade path: ask the instance
// for the plugin that owns each type, if Jira ever answers that, or share
// XTM's own type resolution through core.
func isXrayType(iconURL string) bool {
	return strings.Contains(strings.ToLower(iconURL), xrayPluginKey)
}

// buildJQL is the sync scope: the project, the types to keep out of it, the
// profile's scope JQL in parentheses when set, the incremental clause when
// since parses, and a stable order by key so paging never skips an issue.
//
// The type clause excludes rather than enumerates. TAM used to name the six
// types it models, so a project whose work is mostly other types came back
// almost empty and reported it as a success (#68). Excluding leaves a type
// the project adds later inside the scope without TAM being taught its name.
func buildJQL(projectKey, scopeJQL, since string, excludeTypes []string) string {
	jql := fmt.Sprintf("project = %s", strconv.Quote(projectKey))
	if len(excludeTypes) > 0 {
		quoted := make([]string, len(excludeTypes))
		for i, n := range excludeTypes {
			quoted[i] = strconv.Quote(n)
		}
		jql += fmt.Sprintf(" AND issuetype not in (%s)", strings.Join(quoted, ", "))
	}
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

// named is Jira's {"id": ..., "name": ...} object, the shape status,
// priority, and issue type all arrive in. The id is decoded because the
// board matches cards to columns by status id; it comes in the same object
// as the name, so reading it asks Jira for nothing extra.
type named struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type userRef struct {
	Name        string `json:"name"`
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

type keyed struct {
	Key string `json:"key"`
}

// parseIssue maps one raw search hit onto the grid row.
func parseIssue(raw corejira.RawIssue, ids fieldIDs, requirementType string, pt projectTypes) backend.Issue {
	iss := backend.Issue{Key: raw.Key, ID: raw.ID, Labels: []string{}}
	f := raw.Fields
	_ = json.Unmarshal(f["summary"], &iss.Summary)
	// Present and null is an issue with no description, which decodes to the
	// empty string and is a fact. Absent is a response that did not carry the
	// field at all, which is not: the pointer stays nil so nothing downstream
	// reports "no description" about an issue nobody has read.
	if raw, ok := f["description"]; ok {
		var text string
		_ = json.Unmarshal(raw, &text)
		iss.Description = &text
	}
	_ = json.Unmarshal(f["created"], &iss.Created)
	_ = json.Unmarshal(f["updated"], &iss.Updated)
	var labels []string
	if err := json.Unmarshal(f["labels"], &labels); err == nil && labels != nil {
		iss.Labels = labels
	}
	var priority, issueType named
	// The status carries one thing the other two do not: the category Jira
	// files it under, which is the same three keys on every instance while
	// the name is whatever that instance calls it.
	var status struct {
		named
		StatusCategory struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	}
	if err := json.Unmarshal(f["status"], &status); err == nil {
		iss.Status = status.Name
		iss.StatusID = status.ID
		// I1: the key is the server's word. Only the three Jira defines are
		// kept, so a key from a future version does not reach the store and
		// come out as a colour nothing maps.
		switch status.StatusCategory.Key {
		case "new", "indeterminate", "done":
			iss.StatusCategory = status.StatusCategory.Key
		}
	}
	if err := json.Unmarshal(f["priority"], &priority); err == nil {
		iss.Priority = priority.Name
	}
	if err := json.Unmarshal(f["issuetype"], &issueType); err == nil {
		// A type TAM has no logical type for keeps the project's own name for
		// it, the rule #67 set for the New issue dialog. The row is real work
		// either way, and the empty string is what the sync dropped it for.
		if iss.Type = logicalType(issueType.Name, requirementType, pt); iss.Type == "" {
			iss.Type = issueType.Name
		}
	}
	var assignee, reporter *userRef
	if err := json.Unmarshal(f["assignee"], &assignee); err == nil && assignee != nil {
		iss.Assignee = displayName(*assignee)
		iss.AssigneeName = assigneeName(*assignee)
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

// assigneeName is the username sync caches for the "assigned to me" match:
// name ordinarily, falling back to key, since Data Center in GDPR mode
// populates that instead for some users.
func assigneeName(u userRef) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Key
}

// Legacy Sprint values are toString dumps of the GreenHopper sprint object.
//
// The name runs to the next ",key=" pair or to the dump's closing bracket,
// not to the first "," or "]": a sprint is routinely named for its team in
// brackets ("[SGRS] Sprint 12"), and stopping at the first "]" cut that to
// "[SGRS". Lazy up to the delimiter is what keeps a bracket, and a comma
// that is not followed by a key, inside the name.
var (
	legacySprintID   = regexp.MustCompile(`\bid=(\d+)`)
	legacySprintName = regexp.MustCompile(`\bname=(.*?)(?:,[A-Za-z][A-Za-z0-9]*=|\]\s*$)`)
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
