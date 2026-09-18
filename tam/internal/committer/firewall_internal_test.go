package committer

import (
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestAssertNoPlaceholdersTripsOnWhatRewritingMissed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload any
		trip    bool
	}{
		{"a draft with real ids", backend.IssueDraft{Type: "story", Summary: "x", ParentKey: "PLAT-9", SprintID: "100", Labels: []string{"a"}}, false},
		{"a parent still a placeholder", backend.IssueDraft{Summary: "x", ParentKey: "TAM-NEW-2"}, true},
		{"a placeholder deep in an extra", backend.IssueDraft{Summary: "x", Extra: map[string]string{"customfield_1": "TAM-NEW-9"}}, true},
		{"a draft sprint id on a draft", backend.IssueDraft{Summary: "x", SprintID: "-1"}, true},
		{"a draft sprint id as a move target", map[string]any{"sprintId": "-3", "issues": []string{"PLAT-1"}}, true},
		{"a numeric draft sprint id", map[string]any{"sprintId": -3}, true},
		{"negative points are not a sprint", map[string]any{"storyPoints": -1}, false},
		{"a placeholder in a list of keys", map[string]any{"sprintId": "100", "issues": []string{"PLAT-1", "TAM-NEW-4"}}, true},
		{"a link to a placeholder", map[string]any{"from": "PLAT-1", "link": backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "TAM-NEW-3"}}, true},
		{"a draft board id as a string", map[string]any{"boardId": "-2"}, true},
		{"a numeric draft board id", map[string]any{"boardId": -2}, true},
		{"a real board id", map[string]any{"boardId": 5}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := assertNoPlaceholders(tc.payload)
			if tc.trip != (err != nil) {
				t.Fatalf("err = %v, want trip %v", err, tc.trip)
			}
			if err != nil && (!strings.HasPrefix(err.Error(), "internal error: ") || !strings.Contains(err.Error(), "still names a local placeholder")) {
				t.Errorf("message = %q", err)
			}
		})
	}
}
