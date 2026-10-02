package jira

import (
	"encoding/json"

	"agile-suite/tam/internal/backend"
)

// timeFields are the four Jira sends an issue's time tracking in, and the
// names baseFields asks for. They are seconds, every one of them.
const (
	fieldOriginalEstimate  = "timeoriginalestimate"
	fieldRemainingEstimate = "timeestimate"
	fieldTimeSpent         = "timespent"
	fieldAggregateSpent    = "aggregatetimespent"
)

// readTimeTracking copies the four onto the row. The aggregate stays apart
// from the issue's own spent: Jira counts what a family burned separately
// from what was logged against the parent, and folding the two would have an
// epic claim every hour its children spent.
func readTimeTracking(iss *backend.Issue, f map[string]json.RawMessage) {
	iss.OriginalEstimateSeconds = seconds(f[fieldOriginalEstimate])
	iss.RemainingEstimateSeconds = seconds(f[fieldRemainingEstimate])
	iss.TimeSpentSeconds = seconds(f[fieldTimeSpent])
	iss.AggregateTimeSpentSeconds = seconds(f[fieldAggregateSpent])
}

// seconds reads one of those fields. Absent and null both leave it nil,
// which is "this issue carries no such time" rather than "no time": Jira
// sends null for an issue nobody has estimated, and zero only for one
// estimated at nothing.
func seconds(raw json.RawMessage) *int {
	var n *int
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil
	}
	return n
}
