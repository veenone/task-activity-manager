// Package backend is the seam between Task Activity Manager and the system
// that holds its issues. IssueBackend carries the read path and the write
// path plan 1b added: version checks, edits, and issue creation. The Jira
// implementation lives in backend/jira, the offline one in backend/demo.
package backend

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

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

	// Pending and Draft are computed by the repository's reads, never
	// stored: Pending says the journal holds a change for this key, Draft
	// says the key is a local placeholder Commit has not yet created.
	Pending bool `json:"pending"`
	Draft   bool `json:"draft"`
}

// IssueDraft is a new issue as the form captured it. Type is the logical
// type (task, story, bug). Extra carries the create-meta required fields
// the form rendered as text, keyed by Jira field id; the backend sends each
// as a string, or as {"name": value} when the field takes an option.
type IssueDraft struct {
	Type        string   `json:"type"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
	Assignee    string   `json:"assignee"`
	StoryPoints *float64 `json:"storyPoints"`

	// ParentKey is the epic (or parent) the draft belongs under. The Jira
	// backend sends it through the Epic Link field when it exists.
	ParentKey string `json:"parentKey"`

	Extra map[string]string `json:"extra"`
}

// SplitLabels turns the comma list the form and the journal use back into
// Jira's label slice, trimming blanks.
func SplitLabels(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, ",") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// FormatPoints renders story points the way the journal and the forms
// show them: a plain number, or empty for none.
func FormatPoints(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

// ParsePoints reads the text form back; blank means none.
func ParsePoints(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, errors.New("story points must be a number")
	}
	return &v, nil
}

// FieldSpec is one create-meta field the New issue form must ask for
// because Jira requires it and the form does not already carry it. Type is
// string, option, number, date, or array; option and array fields list
// their AllowedValues.
type FieldSpec struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	Required      bool          `json:"required"`
	AllowedValues []FieldOption `json:"allowedValues"`
}

// FieldOption is one allowed value of an option field.
type FieldOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
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
	// GetIssue reads one issue's row fields, for the version check before a
	// commit and the refresh after it.
	GetIssue(ctx context.Context, key string) (Issue, error)
	// UpdateIssue pushes edited fields, keyed by the logical field name
	// (summary, description, priority, labels, storyPoints, assignee) with
	// the text form the journal holds.
	UpdateIssue(ctx context.Context, key string, fields map[string]string) error
	// CreateIssue creates the draft and returns the key Jira assigned.
	CreateIssue(ctx context.Context, projectKey string, d IssueDraft) (string, error)
	// CreateFields lists the required create-meta fields of a logical type
	// that the New issue form does not already carry.
	CreateFields(ctx context.Context, projectKey, logicalType string) ([]FieldSpec, error)
}
