package jira

import (
	"context"
	"fmt"
)

// Step is one ordered step in an Xray Test (FR-2.5). Xray stores step
// content under "raw" / "rendered" subkeys so unicode and wiki markup
// round-trip; we keep just "raw" here — it's what the editor reads and
// writes, and rendering is the UI's job.
type Step struct {
	ID       string `json:"id"`
	Index    int    `json:"index"`
	Action   string `json:"action"`
	Data     string `json:"data"`
	Expected string `json:"expected"`
}

// UpdateTestStep applies field changes to a single Test Step (FR-2.5). The
// fields map is keyed by the local domain names ("action", "data",
// "expected"); only the keys present are sent — Xray leaves any field
// absent from the body untouched. Demo URLs short-circuit to a no-op so
// step edits in demo just clear local pending rows.
//
// Maps to PUT /rest/raven/2.0/api/test/{key}/steps/{stepId}. Xray's body
// uses "step" (= our "action"), "data", and "result" (= our "expected").
func (c *Client) UpdateTestStep(ctx context.Context, key, stepID string, fields map[string]string) error {
	if isDemoURL(c.baseURL) {
		return nil
	}
	body := map[string]any{}
	if v, ok := fields["action"]; ok {
		body["step"] = map[string]string{"raw": v}
	}
	if v, ok := fields["data"]; ok {
		body["data"] = map[string]string{"raw": v}
	}
	if v, ok := fields["expected"]; ok {
		body["result"] = map[string]string{"raw": v}
	}
	if len(body) == 0 {
		return nil
	}
	return c.put(ctx, fmt.Sprintf("/rest/raven/2.0/api/test/%s/steps/%s", key, stepID), body)
}

// GetTestSteps returns the ordered list of Steps for a Test (FR-2.5). Demo
// URLs fall through to a deterministic generator so the steps panel renders
// without a real Xray.
//
// Maps to GET /rest/raven/2.0/api/test/{key}/steps.
func (c *Client) GetTestSteps(ctx context.Context, key string) ([]Step, error) {
	if isDemoURL(c.baseURL) {
		return demoStepsForKey(key), nil
	}
	var resp []struct {
		ID    string `json:"id"`
		Index int    `json:"index"`
		Step  struct {
			Raw string `json:"raw"`
		} `json:"step"`
		Data struct {
			Raw string `json:"raw"`
		} `json:"data"`
		Result struct {
			Raw string `json:"raw"`
		} `json:"result"`
	}
	if err := c.get(ctx, fmt.Sprintf("/rest/raven/2.0/api/test/%s/steps", key), &resp); err != nil {
		return nil, err
	}
	out := make([]Step, len(resp))
	for i, s := range resp {
		out[i] = Step{
			ID:       s.ID,
			Index:    s.Index,
			Action:   s.Step.Raw,
			Data:     s.Data.Raw,
			Expected: s.Result.Raw,
		}
	}
	return out, nil
}
