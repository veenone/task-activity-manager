package jira

import "context"

// Transition is one workflow move available from a Test's current status
// (FR-4.2). ID is Jira's per-workflow transition identifier — fed back to
// PostTransition. To is the destination status name; we use that name as
// the lookup key on commit so multi-step transitions and renamed workflows
// stay tolerant.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   string `json:"to"`
}

// GetTransitions returns the workflow transitions available from a Test's
// current status (FR-4.2). currentStatus is required so the demo path can
// pick the right set without touching the local store; the real Jira path
// ignores it and reads the status from the issue itself.
//
// Maps to GET /rest/api/2/issue/{key}/transitions.
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
		} `json:"transitions"`
	}
	if err := c.get(ctx, "/rest/api/2/issue/"+key+"/transitions", &resp); err != nil {
		return nil, err
	}
	out := make([]Transition, len(resp.Transitions))
	for i, t := range resp.Transitions {
		out[i] = Transition{ID: t.ID, Name: t.Name, To: t.To.Name}
	}
	return out, nil
}

// PostTransition executes a workflow transition on a Test (FR-4.2). Demo
// URLs short-circuit to a no-op so transitions in demo mode just clear the
// local pending status without making any HTTP calls.
//
// Maps to POST /rest/api/2/issue/{key}/transitions with the body shape
// {"transition": {"id": "..."}}.
func (c *Client) PostTransition(ctx context.Context, key, transitionID string) error {
	if isDemoURL(c.baseURL) {
		return nil
	}
	body := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	return c.post(ctx, "/rest/api/2/issue/"+key+"/transitions", body)
}
