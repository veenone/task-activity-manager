// Package backend is the seam between Task Activity Manager and the system
// that holds its issues. IssueBackend is deliberately small: what the read
// path needs, and nothing the write path will add later. The Jira
// implementation lives in backend/jira, the offline one in backend/demo.
package backend

import "context"

// Logical issue types. Every issue TAM manages is one of these five; the
// Jira names they map to are the backend's business.
const (
	TypeTask        = "task"
	TypeEpic        = "epic"
	TypeStory       = "story"
	TypeBug         = "bug"
	TypeRequirement = "requirement"
)

// AllTypes is the five logical types in display order.
var AllTypes = []string{TypeTask, TypeEpic, TypeStory, TypeBug, TypeRequirement}

// Issue is one row of the Backlog: the columns the grid shows plus what sync
// needs to keep it current. StoryPoints is nil when the issue has none.
type Issue struct {
	Key         string   `json:"key"`
	ID          string   `json:"id"`
	Project     string   `json:"project"`
	Type        string   `json:"type"`
	Summary     string   `json:"summary"`
	Status      string   `json:"status"`
	Assignee    string   `json:"assignee"`
	Reporter    string   `json:"reporter"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
	SprintID    string   `json:"sprintId"`
	SprintName  string   `json:"sprintName"`
	ParentKey   string   `json:"parentKey"`
	StoryPoints *float64 `json:"storyPoints"`
	Rank        string   `json:"rank"`
	Created     string   `json:"created"`
	Updated     string   `json:"updated"`
}

// Link is one issue link seen from the issue that owns it.
type Link struct {
	Direction string `json:"direction"` // "outward" or "inward"
	Type      string `json:"type"`      // the Jira link type name, e.g. "Tested By"
	Key       string `json:"key"`       // the other issue
	Summary   string `json:"summary"`
	IssueType string `json:"issueType"` // the other issue's Jira type name
}

// IssueDetail is what the detail panel shows beyond the grid columns.
// Fields holds the decoded custom fields keyed by field id, so a later
// phase's edit form can build on the same shape.
type IssueDetail struct {
	Key         string         `json:"key"`
	Description string         `json:"description"`
	Links       []Link         `json:"links"`
	Fields      map[string]any `json:"fields"`
}

// IssueType is one issue type a project offers.
type IssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// User is the authenticated user, from the connection test.
type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// IssueBackend is what the read path needs from the issue system.
type IssueBackend interface {
	TestConnection(ctx context.Context) (User, error)
	IsDemo() bool
	// SearchIssuesPage returns one page of issues in projectKey whose logical
	// type is in types, narrowed by scopeJQL when non-empty and by
	// updated >= since (RFC3339) when non-empty, plus the total match count.
	SearchIssuesPage(ctx context.Context, projectKey, scopeJQL, since string, types []string, startAt, maxResults int) ([]Issue, int, error)
	// GetIssueDetail fetches what the grid does not carry: description,
	// links, and the custom fields.
	GetIssueDetail(ctx context.Context, key string) (IssueDetail, error)
	IssueTypes(ctx context.Context, projectKey string) ([]IssueType, error)
}
