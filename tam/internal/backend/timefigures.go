package backend

// Which of an issue's two sets of time figures a reader is shown.
//
// Jira keeps an issue's own estimate, remaining and spent apart from its
// family's, and puts the family's in the aggregate trio. An issue
// estimated through its sub-tasks therefore answers null for its own
// three and carries everything in the other set, which is why reading
// only the issue's own showed nothing at all for a parent (#142).
//
// So a row reads as its family when it has one, the way Jira's own time
// tracking panel does, and says which it is showing. A leaf has the same
// values in both sets, so it reads as itself and is marked as nothing.

// TimeFigures is one row's time as a surface should show it. The three
// seconds counts are nil where the issue carries no such figure, which
// is not the same as zero: an issue nobody estimated was not estimated
// at no work.
type TimeFigures struct {
	EstimateSeconds  *int
	RemainingSeconds *int
	SpentSeconds     *int
	// Family says the three above are the issue and its sub-tasks
	// together rather than the issue alone, so a surface can mark them.
	Family bool
	// OwnSpentSeconds and OwnEstimateSeconds are the issue's own, kept
	// beside the family's so a panel can say what belongs to the issue
	// itself. They are nil when the issue carries none, which on a
	// parent means every hour was logged on a child.
	OwnSpentSeconds    *int
	OwnEstimateSeconds *int
}

// Any reports whether there is anything to show at all.
func (t TimeFigures) Any() bool {
	return t.EstimateSeconds != nil || t.RemainingSeconds != nil || t.SpentSeconds != nil
}

// Time is the figures a row shows.
//
// The family set is taken as soon as one of its values differs from the
// issue's own, which is exactly when the issue has children carrying
// time. Jira answers a leaf with the same numbers in both sets, so this
// leaves a leaf reading as itself without anything having to ask whether
// it has sub-tasks.
//
// Where an aggregate is missing but the issue's own is not, the issue's
// own is shown: that is a row cached before the aggregates were stored,
// and showing nothing for it would lose figures TAM already has.
func (i Issue) Time() TimeFigures {
	family := differs(i.OriginalEstimateSeconds, i.AggregateEstimateSeconds) ||
		differs(i.RemainingEstimateSeconds, i.AggregateRemainingSeconds) ||
		differs(i.TimeSpentSeconds, i.AggregateTimeSpentSeconds)
	t := TimeFigures{
		EstimateSeconds:    i.OriginalEstimateSeconds,
		RemainingSeconds:   i.RemainingEstimateSeconds,
		SpentSeconds:       i.TimeSpentSeconds,
		Family:             family,
		OwnSpentSeconds:    i.TimeSpentSeconds,
		OwnEstimateSeconds: i.OriginalEstimateSeconds,
	}
	if family {
		t.EstimateSeconds = orOwn(i.AggregateEstimateSeconds, i.OriginalEstimateSeconds)
		t.RemainingSeconds = orOwn(i.AggregateRemainingSeconds, i.RemainingEstimateSeconds)
		t.SpentSeconds = orOwn(i.AggregateTimeSpentSeconds, i.TimeSpentSeconds)
	}
	return t
}

// differs says the family carries a figure the issue's own does not
// match. A family value of nil says nothing: that is an instance or a
// cached row that never reported one, not a family of no work.
func differs(own, family *int) bool {
	if family == nil {
		return false
	}
	return own == nil || *own != *family
}

// orOwn prefers the family's figure and falls back to the issue's own.
func orOwn(family, own *int) *int {
	if family != nil {
		return family
	}
	return own
}
