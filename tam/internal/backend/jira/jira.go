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
	"strings"
	"sync"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
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
// assume. Either is "" when the project defines no such type.
type projectTypes struct {
	task    string
	subtask string
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
	var ids fieldIDs
	ok := true
	for _, want := range []struct {
		name string
		dst  *string
	}{
		{"Sprint", &ids.Sprint},
		{"Story Points", &ids.Points},
		{"Epic Link", &ids.EpicLink},
		{"Rank", &ids.Rank},
		{"Epic Name", &ids.EpicName},
	} {
		id, err := b.c.CustomFieldID(ctx, want.name)
		switch {
		case err == nil:
			*want.dst = id
		case errors.Is(err, corejira.ErrFieldNotFound):
			log.Printf("tam: %s has no %q custom field; that column stays empty", b.c.BaseURL(), want.name)
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
	// No expand: this is the sync path, and a changelog nobody reads here
	// would cost every row a second Jira-side lookup for nothing.
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
	var pt projectTypes
	bestTask := len(taskAliases) // lower is a better match
	for _, t := range types {
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
