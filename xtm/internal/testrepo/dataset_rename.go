// Renaming a dataset parameter, the half of the rename that is not the
// dataset (#277 W2).
//
// A ${placeholder} in a step or in the test's definition refers to a
// parameter by name, and Xray resolves it at run time. Rename the parameter
// and every reference to the old name resolves to nothing, which is a quiet
// failure: the test runs with the literal text instead of a value. The
// rewrite here is offered after a rename rather than forced, because a
// reference may be deliberate and only the author knows.

package testrepo

import (
	"fmt"
	"strings"
)

// datasetPlaceholderFields are the test's own fields a ${placeholder} can
// appear in. Summary is not among them: Xray does not substitute there, and
// rewriting a summary would change what the test is called.
var datasetPlaceholderFields = []string{"description", "cucumber_scenario", "generic_definition"}

// RenameDatasetPlaceholder rewrites ${oldName} to ${newName} through the
// test's steps and definition, and reports how many fields it changed.
//
// Each rewrite goes through the ordinary step and field edits, so the result
// is journalled, visible in Pending changes, and discardable one field at a
// time like anything else. A field that does not mention the parameter is
// left alone, and a longer name that merely starts with it is not touched:
// ${USER} and ${USER_ID} are different parameters.
func (r *Repository) RenameDatasetPlaceholder(profileID, testKey, oldName, newName string) (int, error) {
	if oldName == "" || newName == "" || oldName == newName {
		return 0, nil
	}
	from, to := placeholder(oldName), placeholder(newName)

	steps, err := r.ListTestSteps(profileID, testKey)
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, s := range steps {
		for field, value := range map[string]string{
			"action": s.Action, "data": s.Data, "expected": s.Expected,
		} {
			if !strings.Contains(value, from) {
				continue
			}
			if err := r.EditTestStepField(
				profileID, testKey, s.XrayID, field, strings.ReplaceAll(value, from, to),
			); err != nil {
				return changed, fmt.Errorf("rewrite step %s %s: %w", s.XrayID, field, err)
			}
			changed++
		}
	}

	tc, err := r.GetTest(profileID, testKey)
	if err != nil {
		return changed, err
	}
	for _, field := range datasetPlaceholderFields {
		value := testFieldValue(tc, field)
		if !strings.Contains(value, from) {
			continue
		}
		if err := r.EditTestField(
			profileID, testKey, field, strings.ReplaceAll(value, from, to),
		); err != nil {
			return changed, fmt.Errorf("rewrite %s: %w", field, err)
		}
		changed++
	}
	return changed, nil
}

// placeholder is how a dataset parameter is referred to in step text.
func placeholder(name string) string { return "${" + name + "}" }

// testFieldValue reads one of the fields a placeholder can appear in.
func testFieldValue(tc TestCase, field string) string {
	switch field {
	case "description":
		return tc.Description
	case "cucumber_scenario":
		return tc.CucumberScenario
	case "generic_definition":
		return tc.GenericDefinition
	}
	return ""
}
