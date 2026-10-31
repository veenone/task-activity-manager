// A Zephyr test's steps, which live behind the Zephyr API rather than on the
// Jira issue.
//
// Two adaptations apply to every call here. The endpoint takes the Jira issue
// id, not its key, so ids.go resolves it first. And Zephyr names the three
// fields step, data and result where XTM uses action, data and expected, which
// is a rename rather than a difference in meaning.
//
// Writing a step waits for #371 Phase 5, which waits for Phase 4 to run the
// three endpoints SmartBear documents but nobody has measured.

package zephyr

import (
	"context"
	"encoding/json"
	"fmt"

	"agile-suite/xtm/internal/backend"
)

// GetTestSteps reads one test's manual steps.
//
// The array arrives wrapped in stepBeanCollection, and every field arrives
// twice: step, data and result carry the text, htmlStep, htmlData and
// htmlResult carry the same thing marked up. XTM reads the plain fields,
// because the HTML copies would reach the step editor and then every diff a
// commit produces.
func (a *Adapter) GetTestSteps(ctx context.Context, key string) ([]backend.Step, error) {
	id, err := a.z.JiraIssueID(ctx, key)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Steps []struct {
			ID      json.RawMessage `json:"id"`
			OrderID int             `json:"orderId"`
			Step    string          `json:"step"`
			Data    string          `json:"data"`
			Result  string          `json:"result"`
		} `json:"stepBeanCollection"`
	}
	if err := a.z.getJSON(ctx, "/teststep/"+id, &resp); err != nil {
		return nil, fmt.Errorf("read the steps of %s: %w", key, err)
	}
	steps := make([]backend.Step, 0, len(resp.Steps))
	for _, s := range resp.Steps {
		steps = append(steps, backend.Step{
			ID: jsonString(s.ID),
			// Zephyr's orderId starts at 1, which is the index XTM shows and
			// stores, so this is a copy rather than a conversion.
			Index:    s.OrderID,
			Action:   s.Step,
			Data:     s.Data,
			Expected: s.Result,
			// Essential has no "call to test" step, so a step here is always
			// manual content. CreateCalledTestStep is unsupported for the
			// same reason; see absent.go.
		})
	}
	return steps, nil
}
