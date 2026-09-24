package jira

import (
	"context"
	"log"
	"strings"

	"agile-suite/tam/internal/backend"
)

// projectTypes are the Jira names one project gives the two levels TAM cannot
// assume. Either is "" when the project defines no such type. ids maps each
// type's lowercased name to its id, which the per-type create-meta endpoint
// is asked with.
type projectTypes struct {
	task    string
	subtask string
	ids     map[string]string
	// xray is the names of the project's Xray types, the ones a sync keeps
	// out of its scope because they belong to XTM. Recognised by icon, not
	// by name; see isXrayType.
	xray []string
}

// taskAliases are the names an instance gives the plain task level, in the
// order they are preferred. "Task" is Jira's default; "Todo" is what an
// instance seen in the field calls it.
var taskAliases = []string{"task", "todo", "to do"}

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
		// An Xray type is XTM's: not synced, not offered, not creatable from
		// here, so it never reaches the id map the create path reads.
		if isXrayType(t.IconURL) {
			pt.xray = append(pt.xray, t.Name)
			continue
		}
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

// IssueTypes is the project's own types, in the order Jira lists them, each
// carrying the logical type TAM maps it onto. The mapping needs the
// project's level names, which resolveTypes has already worked out, so this
// is one read and not two.
func (b *Backend) IssueTypes(ctx context.Context, projectKey string) ([]backend.IssueType, error) {
	pt, err := b.resolveTypes(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	types, err := b.c.IssueTypes(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	out := make([]backend.IssueType, 0, len(types))
	for _, t := range types {
		if isXrayType(t.IconURL) {
			continue
		}
		out = append(out, backend.IssueType{
			ID:      t.ID,
			Name:    t.Name,
			Subtask: t.Subtask,
			Logical: logicalType(t.Name, b.requirementType, pt),
		})
	}
	return out, nil
}
