package jira

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"agile-suite/tam/internal/backend"
)

// GetIssue reads one issue with the same fields the search asks for, so it
// parses to the same row.
func (b *Backend) GetIssue(ctx context.Context, key string) (backend.Issue, error) {
	ids := b.discover(ctx)
	fields := append(append([]string{}, baseFields...), ids.list()...)
	raw, err := b.c.GetIssue(ctx, key, fields)
	if err != nil {
		return backend.Issue{}, err
	}
	return parseIssue(raw, ids, b.requirementType), nil
}

// jiraFields turns the journal's text values into Jira's field shapes. An
// empty priority, assignee, or points clears the field with null.
func jiraFields(fields map[string]string, ids fieldIDs) (map[string]any, error) {
	out := map[string]any{}
	for name, v := range fields {
		switch name {
		case "summary":
			out["summary"] = v
		case "description":
			out["description"] = v
		case "priority":
			out["priority"] = nameOrNull(v)
		case "assignee":
			out["assignee"] = nameOrNull(v)
		case "labels":
			out["labels"] = backend.SplitLabels(v)
		case "storyPoints":
			if ids.Points == "" {
				return nil, errors.New("this Jira has no Story Points field, so points cannot be pushed")
			}
			p, err := backend.ParsePoints(v)
			if err != nil {
				return nil, err
			}
			if p == nil {
				out[ids.Points] = nil
			} else {
				out[ids.Points] = *p
			}
		default:
			return nil, fmt.Errorf("field %q cannot be sent to Jira", name)
		}
	}
	return out, nil
}

func nameOrNull(v string) any {
	if v == "" {
		return nil
	}
	return map[string]string{"name": v}
}

// UpdateIssue PUTs the edited fields. Jira answers 204 on success and a
// 400 with a per-field message otherwise; the client's error carries it.
func (b *Backend) UpdateIssue(ctx context.Context, key string, fields map[string]string) error {
	ids := b.discover(ctx)
	jf, err := jiraFields(fields, ids)
	if err != nil {
		return err
	}
	return b.c.Put(ctx, "/rest/api/2/issue/"+url.PathEscape(key), map[string]any{"fields": jf})
}

// CreateIssue POSTs the draft. Extra values are shaped from the type's
// create-meta: option fields as {"id"}, arrays as [{"id"}], numbers as
// numbers, everything else as the text entered. If the meta cannot be read
// the values go as text and Jira's own validation decides.
func (b *Backend) CreateIssue(ctx context.Context, projectKey string, d backend.IssueDraft) (string, error) {
	ids := b.discover(ctx)
	names := jiraTypeNames([]string{d.Type}, b.requirementType)
	if len(names) == 0 {
		return "", fmt.Errorf("unknown issue type %q", d.Type)
	}
	fields := map[string]any{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"name": names[0]},
		"summary":   d.Summary,
	}
	if d.Description != "" {
		fields["description"] = d.Description
	}
	if d.Priority != "" {
		fields["priority"] = map[string]string{"name": d.Priority}
	}
	if d.Assignee != "" {
		fields["assignee"] = map[string]string{"name": d.Assignee}
	}
	if len(d.Labels) > 0 {
		fields["labels"] = d.Labels
	}
	if d.StoryPoints != nil && ids.Points != "" {
		fields[ids.Points] = *d.StoryPoints
	}
	if len(d.Extra) > 0 {
		kinds := map[string]string{}
		if specs, err := b.CreateFields(ctx, projectKey, d.Type); err == nil {
			for _, s := range specs {
				kinds[s.ID] = s.Type
			}
		}
		for id, v := range d.Extra {
			if v == "" {
				continue
			}
			fields[id] = shapeExtra(kinds[id], v)
		}
	}
	var resp struct {
		Key string `json:"key"`
	}
	if err := b.c.WriteJSONReturning(ctx, http.MethodPost, "/rest/api/2/issue", map[string]any{"fields": fields}, &resp); err != nil {
		return "", err
	}
	if resp.Key == "" {
		return "", errors.New("Jira created the issue but returned no key")
	}
	return resp.Key, nil
}

func shapeExtra(kind, v string) any {
	switch kind {
	case "option":
		return map[string]string{"id": v}
	case "array":
		return []map[string]string{{"id": v}}
	case "number":
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return v
}

// createMeta is the slice of Jira's createmeta answer the form needs.
type createMeta struct {
	Projects []struct {
		IssueTypes []struct {
			Name   string `json:"name"`
			Fields map[string]struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
				Schema   struct {
					Type  string `json:"type"`
					Items string `json:"items"`
				} `json:"schema"`
				AllowedValues []struct {
					ID    string `json:"id"`
					Value string `json:"value"`
					Name  string `json:"name"`
				} `json:"allowedValues"`
			} `json:"fields"`
		} `json:"issuetypes"`
	} `json:"projects"`
}

// formFields are the create-meta ids the New issue form already carries or
// sets itself, so they never come back as extra required fields.
var formFields = map[string]bool{
	"project": true, "issuetype": true, "summary": true, "description": true,
	"priority": true, "assignee": true, "labels": true, "reporter": true,
}

// CreateFields returns the required fields of the type beyond the form's
// own, sorted by name, with their options when they have any.
func (b *Backend) CreateFields(ctx context.Context, projectKey, logicalType string) ([]backend.FieldSpec, error) {
	ids := b.discover(ctx)
	names := jiraTypeNames([]string{logicalType}, b.requirementType)
	if len(names) == 0 {
		return nil, fmt.Errorf("unknown issue type %q", logicalType)
	}
	q := url.Values{}
	q.Set("projectKeys", projectKey)
	q.Set("issuetypeNames", names[0])
	q.Set("expand", "projects.issuetypes.fields")
	var meta createMeta
	if err := b.c.Get(ctx, "/rest/api/2/issue/createmeta?"+q.Encode(), &meta); err != nil {
		return nil, err
	}
	out := []backend.FieldSpec{}
	for _, p := range meta.Projects {
		for _, t := range p.IssueTypes {
			for id, f := range t.Fields {
				if !f.Required || formFields[id] || id == ids.Points {
					continue
				}
				spec := backend.FieldSpec{ID: id, Name: f.Name, Type: fieldKind(f.Schema.Type), Required: true, AllowedValues: []backend.FieldOption{}}
				for _, av := range f.AllowedValues {
					v := av.Value
					if v == "" {
						v = av.Name
					}
					spec.AllowedValues = append(spec.AllowedValues, backend.FieldOption{ID: av.ID, Value: v})
				}
				out = append(out, spec)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func fieldKind(schemaType string) string {
	switch schemaType {
	case "option", "number", "array":
		return schemaType
	case "date", "datetime":
		return "date"
	}
	return "string"
}
