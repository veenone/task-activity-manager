package backend_test

import (
	"testing"

	"agile-suite/tam/internal/backend"
)

func secs(n int) *int { return &n }

// issueWith is one row's six time fields, in the order Jira reports them:
// the issue's own three, then its family's three.
func issueWith(ownEst, ownRem, ownSpent, famEst, famRem, famSpent *int) backend.Issue {
	return backend.Issue{
		Key:                       "PLAT-1",
		OriginalEstimateSeconds:   ownEst,
		RemainingEstimateSeconds:  ownRem,
		TimeSpentSeconds:          ownSpent,
		AggregateEstimateSeconds:  famEst,
		AggregateRemainingSeconds: famRem,
		AggregateTimeSpentSeconds: famSpent,
	}
}

// A leaf carries the same figures in both sets, which is what Jira
// answers for an issue with no children, so it reads as its own.
func TestALeafReadsAsItself(t *testing.T) {
	got := issueWith(secs(28800), secs(7200), secs(21600), secs(28800), secs(7200), secs(21600)).Time()
	if got.Family {
		t.Error("a leaf was read as a family")
	}
	if got.EstimateSeconds == nil || *got.EstimateSeconds != 28800 {
		t.Errorf("estimate = %v", got.EstimateSeconds)
	}
	if got.RemainingSeconds == nil || *got.RemainingSeconds != 7200 {
		t.Errorf("remaining = %v", got.RemainingSeconds)
	}
	if got.SpentSeconds == nil || *got.SpentSeconds != 21600 {
		t.Errorf("spent = %v", got.SpentSeconds)
	}
}

// The issue this was raised for: everything is estimated on the
// sub-tasks, so the parent's own fields are null and only the family's
// carry anything. Reading its own would show nothing at all.
func TestAParentEstimatedThroughItsChildrenReadsAsTheFamily(t *testing.T) {
	got := issueWith(nil, nil, nil, secs(144000), secs(100800), secs(43200)).Time()
	if !got.Family {
		t.Error("a parent with only family figures was not marked as one")
	}
	if got.EstimateSeconds == nil || *got.EstimateSeconds != 144000 {
		t.Errorf("estimate = %v, want the family's 40h", got.EstimateSeconds)
	}
	if got.SpentSeconds == nil || *got.SpentSeconds != 43200 {
		t.Errorf("spent = %v, want the family's 12h", got.SpentSeconds)
	}
}

// A parent that was worked on itself as well: the family total is what
// the row shows, and the issue's own is kept so the panel can name it.
func TestAParentWithWorkOfItsOwnKeepsBoth(t *testing.T) {
	got := issueWith(secs(7200), secs(3600), secs(3600), secs(144000), secs(100800), secs(43200)).Time()
	if !got.Family {
		t.Error("a parent whose family differs from its own was not marked")
	}
	if got.SpentSeconds == nil || *got.SpentSeconds != 43200 {
		t.Errorf("spent = %v, want the family's", got.SpentSeconds)
	}
	if got.OwnSpentSeconds == nil || *got.OwnSpentSeconds != 3600 {
		t.Errorf("own spent = %v, want the hour on the issue itself", got.OwnSpentSeconds)
	}
}

// An issue nobody has estimated or logged against still shows nothing.
// Zero is a figure it never had.
func TestAnUntrackedIssueReadsAsNothing(t *testing.T) {
	got := issueWith(nil, nil, nil, nil, nil, nil).Time()
	if got.Family || got.Any() {
		t.Errorf("an untracked issue = %+v, want nothing at all", got)
	}
}

// A row cached before the aggregates were stored carries its own figures
// and no family ones, and must read as its own rather than as a family
// of nothing.
func TestARowWithNoAggregatesYetReadsAsItsOwn(t *testing.T) {
	got := issueWith(secs(28800), secs(7200), secs(21600), nil, nil, nil).Time()
	if got.Family {
		t.Error("a row with no aggregates was read as a family")
	}
	if got.EstimateSeconds == nil || *got.EstimateSeconds != 28800 {
		t.Errorf("estimate = %v, want its own", got.EstimateSeconds)
	}
}
