// A Zephyr test's steps, which live behind the Zephyr API rather than on the
// Jira issue.
//
// Two adaptations apply to every call here. The endpoint takes the Jira issue
// id, not its key, so ids.go resolves it first. And Zephyr names the three
// fields step, data and result where XTM uses action, data and expected, which
// is a rename rather than a difference in meaning.
//
// Creating a step is here. Changing, deleting and reordering one are not: they
// wait for Phase 4 to run the three endpoints SmartBear documents but nobody
// has measured, because each of those reshapes a step that already exists.
// Appending one can be inspected and removed, which is the same asymmetry that
// let Phase 6 ship a step-result write while the deletes stayed refused.

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

// CreateTestStep appends one manual step to a test.
//
// The two adaptations GetTestSteps makes apply here unchanged: the endpoint
// takes the Jira issue id, and the three fields are named step, data and
// result. No orderId is sent, so the step lands at the end, which is where
// Zephyr's own editor puts a new one and the order a publish walks its steps
// in.
//
// The path and the field names are measured. The ANSWER's shape is not, which
// is why an unreadable one is not an error: see the comment on the id below.
func (a *Adapter) CreateTestStep(ctx context.Context, key, action, data, expected string) (string, error) {
	id, err := a.z.JiraIssueID(ctx, key)
	if err != nil {
		return "", err
	}
	var created struct {
		ID json.RawMessage `json:"id"`
	}
	body := map[string]any{"step": action, "data": data, "result": expected}
	if err := a.z.writeJSON(ctx, "POST", "/teststep/"+id, body, &created); err != nil {
		return "", fmt.Errorf("add a step to %s: %w", key, err)
	}
	// An id the answer does not carry is reported as empty rather than as a
	// failure. backend.Backend allows it (syncer's createStep returns the id
	// "when Jira reports one" and its caller guards on a non-empty string,
	// reconciling the placeholder on the next refresh), and the write has
	// already landed: failing here would stop a bridge publish at this step
	// and drop the rest of the test's steps for a write that succeeded.
	//
	// CreateContainer makes the opposite choice on purpose. A caller journals
	// pending work against a cycle id, so a cycle it cannot name is useless;
	// nothing keys anything by a step id.
	return jsonString(created.ID), nil
}
