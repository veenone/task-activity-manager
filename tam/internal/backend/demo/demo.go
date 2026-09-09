// Package demo is the in-memory backend behind a demo profile: the curated
// dataset for reads, an overlay for writes, and one staged conflict so the
// resolution dialog can be exercised offline.
package demo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

// conflictKey is the curated story whose first version check after the app
// starts reports a later remote version. It is rekeyed to the profile's
// project.
const conflictKey = "PLAT-412"

// Backend serves the demo dataset with an overlay of writes made this run.
type Backend struct {
	project string

	mu          sync.Mutex
	over        map[string]backend.Issue
	desc        map[string]string
	nextKey     int
	conflict    map[string]bool
	links       map[string][]backend.Link
	sprintState map[int]string
	// sprintDraft holds the name and dates a start was given, beside
	// sprintState, so a demo start can show the reader what the dialog just
	// set instead of falling back to the dataset's own, unstarted values.
	sprintDraft map[int]backend.SprintDraft
}

// New returns a demo backend for the project key. An empty key uses the
// dataset's own.
func New(projectKey string) *Backend {
	if projectKey == "" {
		projectKey = demo.ProjectKey
	}
	b := &Backend{
		project:     projectKey,
		over:        map[string]backend.Issue{},
		desc:        map[string]string{},
		nextKey:     500,
		conflict:    map[string]bool{},
		links:       map[string][]backend.Link{},
		sprintState: map[int]string{},
		sprintDraft: map[int]backend.SprintDraft{},
	}
	b.conflict[b.ConflictKey()] = true
	return b
}

// ConflictKey is the key whose first Commit conflicts, for tests and docs.
func (b *Backend) ConflictKey() string {
	return b.project + conflictKey[len(demo.ProjectKey):]
}

func (b *Backend) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "demo", DisplayName: "Demo User"}, nil
}

func (b *Backend) IsDemo() bool { return true }

// issues is the dataset with the overlay applied: rewritten rows replace
// their originals, created rows follow. Every row leaves with its status id
// filled in from its status name, since the dataset stores names and the
// board joins by id. Callers hold b.mu.
func (b *Backend) issues() []backend.Issue {
	all := demo.Issues(b.project)
	seen := map[string]bool{}
	for i, iss := range all {
		if o, ok := b.over[iss.Key]; ok {
			all[i] = o
		}
		seen[iss.Key] = true
	}
	for k, o := range b.over {
		if !seen[k] {
			all = append(all, o)
		}
	}
	for i := range all {
		all[i] = statusIDFor(all[i])
	}
	return all
}

// SearchIssuesPage answers a page of the dataset, narrowed by issue type
// and by the one scope this backend understands: "sprint = N".
//
// That scope is not decoration. A sprint completion reads the sprint it is
// about with exactly that query and moves everything it comes back with, so
// a demo that answered the whole project would move the whole backlog out
// of a sprint and report it as a success. The service narrows the answer
// again by the sprint each issue reports, but it keeps an issue that
// reports none, on the grounds that the query already narrowed it, which is
// only true of a backend that honours the query. Any other scope is still
// ignored: the demo has no JQL engine, and the profile's own scope JQL is a
// filter this dataset was never built to answer.
func (b *Backend) SearchIssuesPage(_ context.Context, _, scopeJQL, _ string, types []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	want := map[string]bool{}
	for _, t := range types {
		want[t] = true
	}
	sprintID, bySprint := sprintScope(scopeJQL)
	var all []backend.Issue
	for _, iss := range b.issues() {
		if bySprint && iss.SprintID != sprintID {
			continue
		}
		if len(want) == 0 || want[iss.Type] {
			all = append(all, iss)
		}
	}
	total := len(all)
	if startAt >= total || maxResults <= 0 {
		return []backend.Issue{}, total, nil
	}
	end := startAt + maxResults
	if end > total {
		end = total
	}
	return all[startAt:end], total, nil
}

// sprintScope reads a "sprint = N" query, the one narrowing the demo
// honours, and answers with the sprint id it names. Anything else, an empty
// scope included, is not one.
func sprintScope(jql string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(jql), "sprint")
	if !ok {
		return "", false
	}
	if rest, ok = strings.CutPrefix(strings.TrimSpace(rest), "="); !ok {
		return "", false
	}
	id := strings.TrimSpace(rest)
	if id == "" {
		return "", false
	}
	if _, err := strconv.Atoi(id); err != nil {
		return "", false
	}
	return id, true
}

func (b *Backend) GetIssueDetail(_ context.Context, key string) (backend.IssueDetail, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, ok := demo.Detail(b.project, key)
	if !ok {
		if _, created := b.over[key]; created {
			d = backend.IssueDetail{Key: key, Description: "", Links: []backend.Link{}, Fields: map[string]any{}}
		} else {
			return backend.IssueDetail{}, fmt.Errorf("demo: no issue %s", key)
		}
	}
	if desc, ok := b.desc[key]; ok {
		d.Description = desc
	}
	d.Links = append(d.Links, b.links[key]...)
	return d, nil
}

func (b *Backend) IssueTypes(context.Context, string) ([]backend.IssueType, error) {
	return []backend.IssueType{
		{ID: "1", Name: "Task"},
		{ID: "2", Name: "Epic"},
		{ID: "3", Name: "Story"},
		{ID: "4", Name: "Bug"},
		{ID: "5", Name: "Requirement"},
	}, nil
}

func (b *Backend) find(key string) (backend.Issue, bool) {
	for _, iss := range b.issues() {
		if iss.Key == key {
			return iss, true
		}
	}
	for _, iss := range demo.ForeignIssues(b.project) {
		if iss.Key == key {
			return statusIDFor(iss), true
		}
	}
	return backend.Issue{}, false
}

// GetIssue returns the row, once reporting the staged conflict's later
// version so a Commit of an edit to it is held back exactly one time.
func (b *Backend) GetIssue(_ context.Context, key string) (backend.Issue, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, ok := b.find(key)
	if !ok {
		return backend.Issue{}, fmt.Errorf("demo: no issue %s", key)
	}
	if b.conflict[key] {
		delete(b.conflict, key)
		if t, err := time.Parse(time.RFC3339, iss.Updated); err == nil {
			iss.Updated = t.Add(time.Hour).Format(time.RFC3339)
		}
		iss.StoryPoints = pts(13)
		b.over[key] = iss
	}
	return iss, nil
}

func pts(v float64) *float64 { return &v }

func (b *Backend) UpdateIssue(_ context.Context, key string, fields map[string]string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, ok := b.find(key)
	if !ok {
		return fmt.Errorf("demo: no issue %s", key)
	}
	for name, v := range fields {
		switch name {
		case "summary":
			iss.Summary = v
		case "priority":
			iss.Priority = v
		case "assignee":
			iss.Assignee = v
		case "labels":
			iss.Labels = backend.SplitLabels(v)
		case "storyPoints":
			p, err := backend.ParsePoints(v)
			if err != nil {
				return err
			}
			iss.StoryPoints = p
		case "description":
			b.desc[key] = v
		case "parentKey":
			iss.ParentKey = v
		default:
			return fmt.Errorf("field %q cannot be sent to Jira", name)
		}
	}
	iss.Updated = time.Now().UTC().Format(time.RFC3339)
	b.over[key] = iss
	return nil
}

func (b *Backend) CreateIssue(_ context.Context, projectKey string, d backend.IssueDraft) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := fmt.Sprintf("%s-%d", projectKey, b.nextKey)
	b.nextKey++
	now := time.Now().UTC().Format(time.RFC3339)
	priority := d.Priority
	if priority == "" {
		priority = "Medium"
	}
	labels := d.Labels
	if labels == nil {
		labels = []string{}
	}
	parentKey := d.ParentKey
	if d.Type == backend.TypeEpic {
		parentKey = ""
	}
	b.over[key] = backend.Issue{
		Key: key, ID: fmt.Sprintf("%d", 30000+b.nextKey), Project: projectKey, Type: d.Type, Summary: d.Summary,
		Status: "To Do", Assignee: d.Assignee, Reporter: "Demo User", Priority: priority, Labels: labels,
		ParentKey: parentKey, StoryPoints: d.StoryPoints, Created: now, Updated: now,
	}
	b.desc[key] = d.Description
	return key, nil
}

// CreateFields asks for one extra field on bugs and one on requirements,
// so the New issue dialog's create-meta section can be seen offline for
// both an option field and a text field.
func (b *Backend) CreateFields(_ context.Context, _, logicalType string) ([]backend.FieldSpec, error) {
	switch logicalType {
	case backend.TypeBug:
		return []backend.FieldSpec{{
			ID: "customfield_10050", Name: "Severity", Type: "option", Required: true,
			AllowedValues: []backend.FieldOption{{ID: "1", Value: "Minor"}, {ID: "2", Value: "Major"}, {ID: "3", Value: "Critical"}},
		}}, nil
	case backend.TypeRequirement:
		return []backend.FieldSpec{{ID: "customfield_10060", Name: "Source", Type: "string", Required: true, AllowedValues: []backend.FieldOption{}}}, nil
	}
	return []backend.FieldSpec{}, nil
}

// demoLinkTypes are the three link types the demo defines.
var demoLinkTypes = []backend.LinkType{
	{Name: "Relates", Inward: "relates to", Outward: "relates to"},
	{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
	{Name: "Tested By", Inward: "is tested by", Outward: "tests"},
}

func (b *Backend) LinkTypes(context.Context) ([]backend.LinkType, error) {
	return append([]backend.LinkType{}, demoLinkTypes...), nil
}

// CreateLink records the link so the detail shows it from now on.
func (b *Backend) CreateLink(_ context.Context, fromKey string, d backend.LinkDraft) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(fromKey); !ok {
		return fmt.Errorf("demo: no issue %s", fromKey)
	}
	if _, ok := b.find(d.ToKey); !ok {
		return fmt.Errorf("demo: no issue %s", d.ToKey)
	}
	b.links[fromKey] = append(b.links[fromKey], backend.Link{
		Direction: d.Direction, Type: d.Type, Key: d.ToKey, Summary: d.ToSummary, IssueType: d.ToType,
	})
	return nil
}
