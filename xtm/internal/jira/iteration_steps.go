// Steps of one iteration of a parameterized run (#334).
//
// The run resource lists the iterations but carries no steps. Each
// iteration's steps come from its own resource, one call per iteration.

package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// IterationStep is one step of one iteration, as Xray recorded it.
//
// There is no id and no status: the per-iteration resource reports neither,
// so a caller has only the iteration's own status to show. Rank is Xray's
// own `index`, kept rather than recomputed so a step that Xray numbered
// unusually still reads the way Xray numbers it.
type IterationStep struct {
	Rank     int
	Action   string
	Data     string
	Expected string
}

// iterationStepField is Xray's wrapper around one field's value. The rendered
// form is HTML; raw is the text the grid shows.
type iterationStepField struct {
	Type  string `json:"type"`
	Value struct {
		Raw      string `json:"raw"`
		Rendered string `json:"rendered"`
	} `json:"value"`
}

// GetIterationSteps reads the steps of one iteration of a run (#334).
//
// Xray has already substituted the iteration's parameter values into the
// fields it answers with, so the text comes back resolved and must not be
// substituted again. The demo path builds its steps from the test definition
// and substitutes itself, which is the difference between the two.
//
// The field names come from steps.go. They are instance configuration rather
// than part of the API, so a step whose keys match none of them still takes
// its place in the list: dropping it would renumber every step after it.
func (c *Client) GetIterationSteps(ctx context.Context, runID string, iterationID int) ([]IterationStep, error) {
	path := testRunV2Path + url.PathEscape(runID) + "/iteration/" + strconv.Itoa(iterationID)
	body, err := c.getBytes(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("read iteration %d of run %s: %w", iterationID, runID, err)
	}
	var iteration struct {
		Steps []struct {
			Index  int                           `json:"index"`
			Fields map[string]iterationStepField `json:"fields"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(body, &iteration); err != nil {
		return nil, fmt.Errorf("decode iteration %d of run %s: %w", iterationID, runID, err)
	}

	steps := make([]IterationStep, 0, len(iteration.Steps))
	for _, s := range iteration.Steps {
		steps = append(steps, IterationStep{
			Rank:     s.Index,
			Action:   s.Fields[stepFieldAction].Value.Raw,
			Data:     s.Fields[stepFieldData].Value.Raw,
			Expected: s.Fields[stepFieldResult].Value.Raw,
		})
	}
	return steps, nil
}
