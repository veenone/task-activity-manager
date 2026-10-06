package jira

import (
	"encoding/json"

	"agile-suite/tam/internal/backend"
)

// timeFields are the four Jira sends an issue's time tracking in, and the
// names baseFields asks for. They are seconds, every one of them.
const (
	fieldOriginalEstimate   = "timeoriginalestimate"
	fieldRemainingEstimate  = "timeestimate"
	fieldTimeSpent          = "timespent"
	fieldAggregateEstimate  = "aggregatetimeoriginalestimate"
	fieldAggregateRemaining = "aggregatetimeestimate"
	fieldAggregateSpent     = "aggregatetimespent"
)

// timeFields are the six a search asks for. An estimate read with the row
// is an estimate the grid can show without a call per issue, and the
// family's three come too, because an issue estimated through its
// sub-tasks carries nothing in its own (#142).
var timeFields = []string{
	fieldOriginalEstimate, fieldRemainingEstimate, fieldTimeSpent,
	fieldAggregateEstimate, fieldAggregateRemaining, fieldAggregateSpent,
}

// readTimeTracking copies the six onto the row: the issue's own three and
// its family's three. They stay apart because Jira counts them apart, and
// backend.Issue.Time is the one place that decides which set a reader is
// shown. Folding them here would have an epic claim every hour its
// children spent as work logged on the epic.
func readTimeTracking(iss *backend.Issue, f map[string]json.RawMessage) {
	iss.OriginalEstimateSeconds = seconds(f[fieldOriginalEstimate])
	iss.RemainingEstimateSeconds = seconds(f[fieldRemainingEstimate])
	iss.TimeSpentSeconds = seconds(f[fieldTimeSpent])
	iss.AggregateEstimateSeconds = seconds(f[fieldAggregateEstimate])
	iss.AggregateRemainingSeconds = seconds(f[fieldAggregateRemaining])
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
