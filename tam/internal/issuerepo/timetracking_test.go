package issuerepo_test

import (
	"context"
	"testing"
	"time"
)

func secs(n int) *int { return &n }

// The estimate and the time spent against it come back off the row, the way
// the story points do: the grid and the detail panel read them from the
// store, not from a call per selection.
func TestSyncKeepsTheTimeTrackingOnTheRow(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	page := sample()[:1]
	page[0].OriginalEstimateSeconds = secs(28800)
	page[0].RemainingEstimateSeconds = secs(7200)
	page[0].TimeSpentSeconds = secs(21600)
	page[0].AggregateEstimateSeconds = secs(144000)
	page[0].AggregateRemainingSeconds = secs(100800)
	page[0].AggregateTimeSpentSeconds = secs(36000)
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	iss, err := r.GetIssue(ctx, "p1", page[0].Key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, c := range []struct {
		name string
		got  *int
		want int
	}{
		{"original estimate", iss.OriginalEstimateSeconds, 28800},
		{"remaining estimate", iss.RemainingEstimateSeconds, 7200},
		{"time spent", iss.TimeSpentSeconds, 21600},
		{"aggregate estimate", iss.AggregateEstimateSeconds, 144000},
		{"aggregate remaining", iss.AggregateRemainingSeconds, 100800},
		{"aggregate time spent", iss.AggregateTimeSpentSeconds, 36000},
	} {
		if c.got == nil {
			t.Errorf("%s is nil after a sync that carried one", c.name)
			continue
		}
		if *c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, *c.got, c.want)
		}
	}
}

// An issue nobody has estimated carries no estimate, which is not the same
// fact as an estimate of nothing. Zero would have the grid print "0h" over
// every row of a project that does not estimate in time at all.
func TestAnUnestimatedIssueReadsBackAsNoEstimateRatherThanZero(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample()[:1], time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	iss, err := r.GetIssue(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.OriginalEstimateSeconds != nil || iss.RemainingEstimateSeconds != nil ||
		iss.TimeSpentSeconds != nil || iss.AggregateEstimateSeconds != nil ||
		iss.AggregateRemainingSeconds != nil || iss.AggregateTimeSpentSeconds != nil {
		t.Errorf("unestimated issue = %+v, want all four nil", iss)
	}
}

// A parent's rolled-up spent is a different number from its own, and the two
// travel in their own columns: reporting one as the other would have an epic
// claim every hour its children burned as work logged against the epic.
func TestTheRolledUpSpentIsNotTheIssuesOwn(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	page := sample()[:1]
	page[0].TimeSpentSeconds = secs(3600)
	page[0].AggregateTimeSpentSeconds = secs(39600)
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	iss, err := r.GetIssue(ctx, "p1", page[0].Key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.TimeSpentSeconds == nil || *iss.TimeSpentSeconds != 3600 {
		t.Errorf("time spent = %v, want the hour logged on the issue itself", iss.TimeSpentSeconds)
	}
	if iss.AggregateTimeSpentSeconds == nil || *iss.AggregateTimeSpentSeconds != 39600 {
		t.Errorf("aggregate = %v, want the eleven hours the family burned", iss.AggregateTimeSpentSeconds)
	}
}
