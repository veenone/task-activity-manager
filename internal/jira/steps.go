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
