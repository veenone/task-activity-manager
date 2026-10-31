// Step results on a plain execution, which is the one thing Essential does
// better than Xray.
//
// A Zephyr step result is its own entity: it has an id, a status drawn from a
// vocabulary of its own, and a comment, and it exists on any execution rather
// than only inside a parameterized iteration. #371 Phase 6 is where XTM gained
// somewhere to put that; backend/run_step_result.go holds the model.
//
// Two reads make one answer. /stepResult says what was recorded and carries no
// step text and no order; /teststep says what the step is and where it sits on
// the test. Neither alone is showable, so they are joined here.

package zephyr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"agile-suite/xtm/internal/backend"
)

// GetRunStepResults reads one test's step results in one cycle.
//
// Ordering comes from the step definitions, because /stepResult returns the
// results in no documented order and carries no rank of its own. A result
// whose step has since been deleted from the test keeps its status and sorts
// last: it is a result somebody recorded, and dropping it would quietly hide
// an executed step (I1).
func (a *Adapter) GetRunStepResults(ctx context.Context, execKey, testKey string) ([]backend.RunStepResult, error) {
	vocabulary, err := a.z.StepStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("read this instance's step result vocabulary: %w", err)
	}
	executionID, err := a.executionID(ctx, execKey, testKey)
	if err != nil {
		return nil, err
	}
	definitions, err := a.GetTestSteps(ctx, testKey)
	if err != nil {
		return nil, err
	}
	byDefinition := make(map[string]backend.Step, len(definitions))
	for _, step := range definitions {
		byDefinition[step.ID] = step
	}

	raw, err := a.z.stepResults(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("read the step results of %s in cycle %s: %w", testKey, execKey, err)
	}
	results := make([]backend.RunStepResult, 0, len(raw))
	for _, r := range raw {
		id := jsonString(r.ID)
		if id == "" {
			// A result with no id cannot be written and cannot be told apart
			// from another, so there is nothing useful to show for it.
			continue
		}
		definitionID := jsonString(r.StepID)
		result := backend.RunStepResult{
			ID:           id,
			DefinitionID: definitionID,
			Status:       vocabulary.NameFor(statusID(r.Status)),
			Comment:      strings.TrimSpace(r.Comment),
			// An orphan sorts last rather than first, which rank 0 would do.
			Rank: len(raw) + 1,
		}
		if step, ok := byDefinition[definitionID]; ok {
			result.Rank = step.Index
			result.Action = step.Action
			result.Data = step.Data
			result.Expected = step.Expected
		}
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Rank < results[j].Rank })
	return results, nil
}

// SetRunStepStatus records a result against one step result.
//
// execKey and testKey are not sent. A step result id addresses the result on
// its own, and the pair is in the signature because the journal row and the
// commit path are keyed by them: asking the instance to confirm the result
// belongs to that execution would be a round trip to re-establish what the id
// already determines.
func (a *Adapter) SetRunStepStatus(ctx context.Context, execKey, testKey, stepResultID, status string) error {
	vocabulary, err := a.z.StepStatuses(ctx)
	if err != nil {
		return fmt.Errorf("read this instance's step result vocabulary: %w", err)
	}
	statusID, err := vocabulary.IDFor(status)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(stepResultID)
	if id == "" {
		return fmt.Errorf("a step result id is required to set a step's result")
	}
	// As with the three writes in write.go, the path is documented and the
	// body is not: the field name mirrors the one the read returns. A wrong
	// guess is refused by the instance and reported as a failed commit with
	// the row left pending, and writeJSON reads the body as well as the status
	// because an authentication failure arrives as 200 with an errorId (I1).
	body := map[string]any{"status": statusID}
	if err := a.z.writeJSON(ctx, "PUT", "/stepResult/"+url.PathEscape(id), body, nil); err != nil {
		return fmt.Errorf("set the result of step %s on %s in cycle %s: %w", id, testKey, execKey, err)
	}
	return nil
}

// stepResultRow is one row of /stepResult. The status arrives as a quoted
// number on the probed instance, which is why it is read as a raw message and
// not an int.
type stepResultRow struct {
	ID      json.RawMessage `json:"id"`
	StepID  json.RawMessage `json:"stepId"`
	Status  json.RawMessage `json:"status"`
	Comment string          `json:"comment"`
}

// stepResults reads the results of one execution. The answer is a bare array,
// unlike the step definitions, which arrive wrapped in stepBeanCollection.
func (c *Client) stepResults(ctx context.Context, executionID string) ([]stepResultRow, error) {
	q := url.Values{}
	q.Set("executionId", executionID)
	var rows []stepResultRow
	if err := c.getJSON(ctx, "/stepResult?"+q.Encode(), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// statusID reads a status that can arrive as 2 or as "2".
func statusID(raw json.RawMessage) int {
	id, err := strconv.Atoi(jsonString(raw))
	if err != nil {
		return 0
	}
	return id
}
