package jira

import (
	"context"
	"sort"
)

// Transition is one workflow move available from a Test's current status
// (FR-4.2). ID is Jira's per-workflow transition identifier — fed back to
// PostTransition. To is the destination status name; we use that name as
// the lookup key on commit so multi-step transitions and renamed workflows
// stay tolerant. Fields are what the transition's screen asks for, required
// or not; a transition with no screen has none.
type Transition struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	To     string     `json:"to"`
	Fields []FieldDef `json:"fields"`
}

// GetTransitions returns the workflow transitions available from a Test's
// current status (FR-4.2). currentStatus is required so the demo path can
// pick the right set without touching the local store; the real Jira path
// ignores it and reads the status from the issue itself.
//
// Maps to GET /rest/api/2/issue/{key}/transitions?expand=transitions.fields,
// so one request answers both what is reachable and what each move asks for.
func (c *Client) GetTransitions(ctx context.Context, key, currentStatus string) ([]Transition, error) {
	if isDemoURL(c.baseURL) {
		return demoTransitionsForStatus(currentStatus), nil
	}
	var resp struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
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
		} `json:"transitions"`
	}
	if err := c.get(ctx, "/rest/api/2/issue/"+key+"/transitions?expand=transitions.fields", &resp); err != nil {
		return nil, err
	}
	out := make([]Transition, len(resp.Transitions))
	for i, t := range resp.Transitions {
		var fields []FieldDef
		for id, f := range t.Fields {
			avs := make([]FieldOption, 0, len(f.AllowedValues))
			for _, av := range f.AllowedValues {
				v := av.Value
				if v == "" {
					v = av.Name
				}
				avs = append(avs, FieldOption{ID: av.ID, Value: v})
			}
			fields = append(fields, FieldDef{
				ID:            id,
				Name:          f.Name,
				Required:      f.Required,
				Type:          bugCreateFieldKind(f.Schema.Type, f.Schema.Items),
				AllowedValues: avs,
			})
		}
		// Jira sends the fields as a map; order them so a screen built from
		// them does not reshuffle between fetches.
		sort.Slice(fields, func(a, b int) bool { return fields[a].ID < fields[b].ID })
		out[i] = Transition{ID: t.ID, Name: t.Name, To: t.To.Name, Fields: fields}
	}
	return out, nil
}

// PostTransition executes a workflow transition on a Test (FR-4.2). Demo
// URLs short-circuit to a no-op so transitions in demo mode just clear the
// local pending status without making any HTTP calls.
//
// Maps to POST /rest/api/2/issue/{key}/transitions with the body shape
// {"transition": {"id": "..."}, "fields": {...}}. fields are the values the
// transition's screen asks for, Jira-shaped and keyed by field id; Jira takes
// them on the transition itself, not as a separate edit. Without any, the
// body carries no fields key at all.
func (c *Client) PostTransition(ctx context.Context, key, transitionID string, fields map[string]any) error {
	if isDemoURL(c.baseURL) {
		return nil
	}
	body := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	return c.post(ctx, "/rest/api/2/issue/"+key+"/transitions", body)
}
