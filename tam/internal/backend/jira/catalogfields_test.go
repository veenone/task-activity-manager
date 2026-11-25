package jira

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	corejira "agile-suite/core/jira"
)

func TestParseIssueReadsComponentsAndFixVersions(t *testing.T) {
	raw := corejira.RawIssue{ID: "1", Key: "PLAT-1", Fields: map[string]json.RawMessage{
		"components":  json.RawMessage(`[{"id":"10000","name":"Auth"}]`),
		"fixVersions": json.RawMessage(`[{"id":"10100"},{"id":"10101"}]`),
	}}
	iss := parseIssue(raw, fieldIDs{}, "Requirement", projectTypes{})
	if !reflect.DeepEqual(iss.Components, []int{10000}) || !reflect.DeepEqual(iss.FixVersions, []int{10100, 10101}) {
		t.Errorf("components %v fix versions %v", iss.Components, iss.FixVersions)
	}
	bare := parseIssue(corejira.RawIssue{Key: "PLAT-2", Fields: map[string]json.RawMessage{}}, fieldIDs{}, "Requirement", projectTypes{})
	if bare.Components == nil || bare.FixVersions == nil || len(bare.Components)+len(bare.FixVersions) != 0 {
		t.Errorf("absent fields gave %#v and %#v, want empty and non-nil", bare.Components, bare.FixVersions)
	}
	for _, f := range []string{"components", "fixVersions"} {
		if !slices.Contains(baseFields, f) {
			t.Errorf("baseFields does not ask for %s", f)
		}
	}
}
