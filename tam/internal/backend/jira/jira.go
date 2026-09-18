// Package jira is the IssueBackend for a live Jira Data Center, built on the
// suite's shared transport. It owns the JQL, the custom-field discovery, and
// the mapping from Jira's field shapes to TAM's issue.
package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// Backend talks to one Jira instance for one profile.
type Backend struct {
	c               *corejira.Client
	requirementType string

	mu         sync.Mutex
	ids        fieldIDs
	discovered bool

	// transitionResolution is the profile's `transition_resolution`
	// setting: the resolution name a transition into Done is given when it
	// asks for one and allows it. See transitions.go.
	transitionResolution string

	linkTypes       []backend.LinkType
	linkTypesLoaded bool

	// projectTypeCache holds each project's resolved level names. Both are
	// read from the project rather than assumed, because the instance names
	// them: the sub-task level defaults to "Sub-task" but one seen in the
	// field calls it "Technical task", and the plain task level defaults to
	// "Task" but the same instance calls it "Todo". Cached so a sync's every
	// page does not re-read the project.
	projectTypeCache map[string]projectTypes
}

// projectTypes are the Jira names one project gives the two levels TAM cannot
// assume. Either is "" when the project defines no such type. ids maps each
// type's lowercased name to its id, which the per-type create-meta endpoint
// is asked with.
type projectTypes struct {
	task    string
	subtask string
	ids     map[string]string
}

// taskAliases are the names an instance gives the plain task level, in the
// order they are preferred. "Task" is Jira's default; "Todo" is what an
// instance seen in the field calls it.
var taskAliases = []string{"task", "todo", "to do"}

// New builds the backend. requirementType is the profile's Jira name for
// requirements; empty means DefaultRequirementType.
func New(c *corejira.Client, requirementType string) *Backend {
	if strings.TrimSpace(requirementType) == "" {
		requirementType = DefaultRequirementType
	}
	return &Backend{
		c:                c,
		requirementType:  strings.TrimSpace(requirementType),
		projectTypeCache: map[string]projectTypes{},
	}
}

// TestConnection fetches the authenticated user.
func (b *Backend) TestConnection(ctx context.Context) (backend.User, error) {
	u, err := b.c.Myself(ctx)
	if err != nil {
		return backend.User{}, fmt.Errorf("connection test failed: %w", err)
	}
	return backend.User{Name: u.Name, DisplayName: u.DisplayName}, nil
}

// IsDemo is false: this backend always talks to a server.
func (b *Backend) IsDemo() bool { return false }

// discover resolves the custom field ids once. A field the instance does not
// have is logged once and left empty; a transport failure is logged and
// retried on the next call, so a flaky first request does not blank the
// columns for the rest of the session.
func (b *Backend) discover(ctx context.Context) fieldIDs {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.discovered {
		return b.ids
	}
	ids := fieldIDs{ambiguous: map[string][]string{}}
	ok := true
	for _, want := range []struct {
		name string
		dst  *string
	}{
		{"Sprint", &ids.Sprint},
		{pointsFieldName, &ids.Points},
		{"Epic Link", &ids.EpicLink},
		{"Rank", &ids.Rank},
		{"Epic Name", &ids.EpicName},
	} {
		id, err := b.c.CustomFieldID(ctx, want.name)
		var amb *corejira.AmbiguousFieldError
		switch {
		case err == nil:
			*want.dst = id
		case errors.Is(err, corejira.ErrFieldNotFound):
			log.Printf("tam: %s has no %q custom field; that column stays empty", b.c.BaseURL(), want.name)
		case errors.As(err, &amb):
			// Two fields of the same name identify neither, so the id stays
			// empty rather than being guessed. Cached like a missing field:
			// the instance will not stop having two of them while the app
			// runs, and re-asking would only repeat this line.
			ids.ambiguous[want.name] = amb.IDs
			log.Printf("tam: %s has more than one %q custom field (%s); TAM will not guess which one it means", b.c.BaseURL(), want.name, strings.Join(amb.IDs, " and "))
		default:
			log.Printf("tam: custom field discovery failed, syncing without custom columns this time: %v", err)
			ok = false
		}
	}
	if ok {
		b.ids = ids
		b.discovered = true
	}
	return ids
}

// SearchIssuesPage runs one page of the scope JQL and maps the rows.
func (b *Backend) SearchIssuesPage(ctx context.Context, projectKey, scopeJQL, since string, types []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	ids := b.discover(ctx)
	pt := b.typesOrEmpty(ctx, projectKey)
	jql := buildJQL(projectKey, scopeJQL, since, jiraTypeNames(types, b.requirementType, pt))
	fields := append(append([]string{}, baseFields...), ids.list()...)
	// No expand: this is the sync path. expand=changelog costs no second
	// request; it makes this same response carry every row's full history,
	// which Jira has to assemble at real cost and which a sync never reads,
	// so asking for it here would only make every page slower for nothing.
	page, err := b.c.SearchIssues(ctx, jql, fields, nil, startAt, maxResults)
	if err != nil {
		return nil, 0, err
	}
	issues := make([]backend.Issue, 0, len(page.Issues))
	for _, raw := range page.Issues {
		issues = append(issues, parseIssue(raw, ids, b.requirementType, pt))
	}
	return issues, page.Total, nil
}

// GetIssueDetail fetches the description, the links, and the discovered
// custom fields for one issue.
func (b *Backend) GetIssueDetail(ctx context.Context, key string) (backend.IssueDetail, error) {
	ids := b.discover(ctx)
	fields := append([]string{"description", "issuelinks"}, ids.list()...)
	raw, err := b.c.GetIssue(ctx, key, fields)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	d := backend.IssueDetail{Key: raw.Key, Links: parseLinks(raw.Fields["issuelinks"]), Fields: map[string]any{}}
	_ = json.Unmarshal(raw.Fields["description"], &d.Description)
	d.Comments, d.CommentTotal, d.CommentsTruncated = b.comments(ctx, key)
	for _, id := range ids.list() {
		if v, ok := raw.Fields[id]; ok && len(v) > 0 && string(v) != "null" {
			var decoded any
			if err := json.Unmarshal(v, &decoded); err == nil {
				d.Fields[id] = decoded
			}
		}
	}
	return d, nil
}

// commentPage is how many comments one request asks for and commentCap how
// many the detail keeps. The comments are not read off the issue itself:
// Data Center answers fields=comment with the whole list inline, however
// long it is, so the detail of a year-old bug would carry hundreds of
// bodies nobody asked for. The comment endpoint pages, and newest first is
// what makes the cap keep the half a reader wants.
const (
	commentPage = 100
	commentCap  = 500
)

// rawComment is Jira's own comment shape. author is null for an anonymous
// comment and for one whose author has been deleted since, and visibility is
// present only on a restricted comment.
type rawComment struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Created string `json:"created"`
	Updated string `json:"updated"`
	Author  *struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Visibility *struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"visibility"`
}

// comment maps one row, normalising both timestamps and keeping whatever
// restriction it carries.
func (c rawComment) comment() backend.Comment {
	out := backend.Comment{
		ID:      c.ID,
		Body:    c.Body,
		Created: commentTime(c.Created),
		Updated: commentTime(c.Updated),
	}
	if c.Author != nil {
		out.Author, out.AuthorName = c.Author.Name, c.Author.DisplayName
	}
	if c.Visibility != nil {
		out.Restriction = c.Visibility.Value
	}
	return out
}

// commentTime normalises one Jira timestamp to RFC 3339. Jira's offsets
// carry no colon, so this goes through sprintdate, the one place that
// reading lives; a value it cannot read is handed on exactly as it arrived,
// since a comment with an odd stamp is still a comment.
func commentTime(v string) string {
	t, err := sprintdate.Parse(v)
	if err != nil {
		return v
	}
	return t.UTC().Format(time.RFC3339)
}

// comments reads the issue's comments newest first and returns them oldest
// first, with Jira's own total and whether the list is short of it.
//
// Newest first is what the request asks for, so the cap drops the oldest
// rather than whatever Jira happened to send first; oldest first is what
// comes back, because that is the order the panel reads them in and the
// newest few it shows are then simply the last few.
//
// A failed page is not an error. The description, the links and the fields
// are what the panel is for, and losing all of them because the comment
// endpoint answered 500 would be the wrong trade; what was read is kept and
// the list is marked short, which is the same thing the cap says.
func (b *Backend) comments(ctx context.Context, key string) ([]backend.Comment, int, bool) {
	out := []backend.Comment{}
	total := 0
	// The bound is a count of comments, not of pages, and the walk advances
	// by what actually came back: an instance that clamps maxResults below
	// what was asked for answers every page short, and stepping on by the
	// size asked for would leave holes in the list that nothing downstream
	// could see. Same arithmetic as core/jira's own Agile paging.
	for startAt := 0; len(out) < commentCap; {
		var page struct {
			Total    int          `json:"total"`
			Comments []rawComment `json:"comments"`
		}
		path := fmt.Sprintf("/rest/api/2/issue/%s/comment?orderBy=-created&startAt=%d&maxResults=%d",
			url.PathEscape(key), startAt, commentPage)
		if err := b.c.Get(ctx, path, &page); err != nil {
			log.Printf("tam: read the comments of %s from %d: %v", key, startAt, err)
			if total < len(out) {
				total = len(out)
			}
			return oldestFirst(out), total, true
		}
		total = page.Total
		for _, c := range page.Comments {
			out = append(out, c.comment())
		}
		if len(page.Comments) == 0 || len(out) >= total {
			break
		}
		startAt += len(page.Comments)
	}
	// The last page can carry the count past the ceiling. Cutting from the
	// end keeps the newest, which is the half the cap is there to keep.
	if len(out) > commentCap {
		out = out[:commentCap]
	}
	return oldestFirst(out), total, len(out) < total
}

// oldestFirst reverses the newest-first list in place.
func oldestFirst(c []backend.Comment) []backend.Comment {
	for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
		c[i], c[j] = c[j], c[i]
	}
	return c
}

// IssueTypes lists the project's issue types.
// resolveTypes reads a project's issue types once and works out what it calls
// the two levels TAM cannot assume. A project usually defines exactly one
// sub-task type; when it defines several the first Jira lists is used, which
// is the order the project's own create dialog offers them in.
func (b *Backend) resolveTypes(ctx context.Context, projectKey string) (projectTypes, error) {
	b.mu.Lock()
	if pt, ok := b.projectTypeCache[projectKey]; ok {
		b.mu.Unlock()
		return pt, nil
	}
	b.mu.Unlock()

	types, err := b.c.IssueTypes(ctx, projectKey)
	if err != nil {
		return projectTypes{}, err
	}
	pt := projectTypes{ids: map[string]string{}}
	bestTask := len(taskAliases) // lower is a better match
	for _, t := range types {
		if _, seen := pt.ids[strings.ToLower(t.Name)]; !seen {
			pt.ids[strings.ToLower(t.Name)] = t.ID
		}
		if t.Subtask {
			if pt.subtask == "" {
				pt.subtask = t.Name
			}
			continue
		}
		n := strings.ToLower(strings.TrimSpace(t.Name))
		for i, alias := range taskAliases {
			if n == alias && i < bestTask {
				pt.task, bestTask = t.Name, i
			}
		}
	}
	b.mu.Lock()
	b.projectTypeCache[projectKey] = pt
	b.mu.Unlock()
	if pt.subtask == "" {
		log.Printf("tam: %s has no sub-task type in %s; sub-tasks are unavailable there", b.c.BaseURL(), projectKey)
	}
	if pt.task == "" {
		log.Printf("tam: %s has no task-level type in %s; the Task filter stays empty there", b.c.BaseURL(), projectKey)
	}
	return pt, nil
}

// SubtaskTypeName is the project's sub-task type name, "" when it has none.
func (b *Backend) SubtaskTypeName(ctx context.Context, projectKey string) (string, error) {
	pt, err := b.resolveTypes(ctx, projectKey)
	return pt.subtask, err
}

// typesOrEmpty is resolveTypes with the error swallowed, for the paths that
// only need to recognise a type and must not fail because the project lookup
// did.
func (b *Backend) typesOrEmpty(ctx context.Context, projectKey string) projectTypes {
	pt, err := b.resolveTypes(ctx, projectKey)
	if err != nil {
		log.Printf("tam: resolve the issue types for %s: %v", projectKey, err)
	}
	return pt
}

func (b *Backend) IssueTypes(ctx context.Context, projectKey string) ([]backend.IssueType, error) {
	types, err := b.c.IssueTypes(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	out := make([]backend.IssueType, 0, len(types))
	for _, t := range types {
		out = append(out, backend.IssueType{ID: t.ID, Name: t.Name})
	}
	return out, nil
}
