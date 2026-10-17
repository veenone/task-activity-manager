package demo_test

import (
	"testing"

	"agile-suite/tam/internal/demo"
)

// TestTheClosedSprintCarriesSampleTimeTracking gives the demo's reports
// something to draw: every Sprint 11 card has an estimate, the started ones
// have time spent, and the worklogs add up to that time spent.
func TestTheClosedSprintCarriesSampleTimeTracking(t *testing.T) {
	issues := demo.Issues("PLAT")
	logs := demo.SampleWorklogs(issues)
	var cards, over int
	for _, iss := range issues {
		if iss.SprintID != "11" {
			if iss.OriginalEstimateSeconds != nil || len(logs[iss.Key]) > 0 {
				t.Errorf("%s is outside Sprint 11 and should carry no sample time", iss.Key)
			}
			continue
		}
		cards++
		if iss.OriginalEstimateSeconds == nil || *iss.OriginalEstimateSeconds <= 0 {
			t.Errorf("%s has no estimate", iss.Key)
			continue
		}
		spent := 0
		for _, w := range logs[iss.Key] {
			spent += w.Seconds
		}
		if iss.TimeSpentSeconds == nil || *iss.TimeSpentSeconds != spent {
			t.Errorf("%s: time spent %v, worklogs add up to %d", iss.Key, iss.TimeSpentSeconds, spent)
		}
		if spent > *iss.OriginalEstimateSeconds {
			over++
		}
	}
	if cards == 0 || over == 0 {
		t.Errorf("Sprint 11 has %d cards and %d over their estimate; the sample needs both kinds", cards, over)
	}
	if got := demo.Issues("OPS"); got[0].Key == "PLAT-412" {
		t.Fatal("rekeying is what this test relies on not breaking")
	}
	if len(demo.SampleWorklogs(demo.Issues("OPS"))["OPS-401"]) == 0 {
		t.Error("a rekeyed profile gets the same sample under its own keys")
	}
}
