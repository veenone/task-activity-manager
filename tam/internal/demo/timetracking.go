package demo

import (
	"strconv"
	"strings"

	"agile-suite/tam/internal/backend"
)

// The closed sprint's sample time tracking (#287). Sprint 11 is the one the
// demo's Reports view opens on, so its cards carry estimates, time spent and
// the worklogs behind it: without them the People tab's time bars and the
// burndown in hours would have nothing to draw on a demo profile. Only
// Sprint 11 is touched, so every other view reads the dataset as before.
const sampleSprint = "11"

// curatedTime is the hand-picked sample for the curated cards, in hours:
// estimate, spent, remaining. PLAT-401 runs over its estimate and PLAT-385
// lands on it exactly, so the bar's three states all appear.
var curatedTime = map[string][3]int{
	"PLAT-401": {8, 10, 2},
	"PLAT-347": {2, 1, 1},
	"PLAT-331": {16, 14, 0},
	"PLAT-385": {4, 4, 0},
}

// The two days the sample worklogs fall on, inside Sprint 11.
var sampleLogDays = [2]string{"2026-08-06T10:00:00.000+0000", "2026-08-12T10:00:00.000+0000"}

const hour = 3600

// withSampleTime fills in a Sprint 11 card's time tracking. canonical is the
// card's PLAT key, which curatedTime is keyed by. A filler card is estimated
// at three hours a point (four when unpointed) and has spent all of it when
// done, three quarters of it in progress, and none of it to do.
func withSampleTime(iss *backend.Issue, canonical string) {
	if iss.SprintID != sampleSprint {
		return
	}
	est, spent, rem := 0, 0, 0
	if t, ok := curatedTime[canonical]; ok {
		est, spent, rem = t[0]*hour, t[1]*hour, t[2]*hour
	} else {
		est = 4 * hour
		if iss.StoryPoints != nil {
			est = int(*iss.StoryPoints * 3 * hour)
		}
		switch iss.Status {
		case "Done":
			spent = est
		case "In Progress":
			spent = est * 3 / 4
		}
		rem = est - spent
	}
	iss.OriginalEstimateSeconds, iss.RemainingEstimateSeconds, iss.TimeSpentSeconds = &est, &rem, &spent
	iss.AggregateEstimateSeconds, iss.AggregateRemainingSeconds, iss.AggregateTimeSpentSeconds = &est, &rem, &spent
}

// SampleWorklogs is the worklogs behind the sample's time spent, keyed by
// issue: each started card's hours split across two days of the sprint,
// logged by its assignee.
func SampleWorklogs(issues []backend.Issue) map[string][]backend.Worklog {
	out := map[string][]backend.Worklog{}
	for _, iss := range issues {
		if iss.SprintID != sampleSprint || iss.TimeSpentSeconds == nil || *iss.TimeSpentSeconds <= 0 {
			continue
		}
		spent := *iss.TimeSpentSeconds
		first := spent / 2
		for i, secs := range []int{first, spent - first} {
			if secs <= 0 {
				continue
			}
			out[iss.Key] = append(out[iss.Key], backend.Worklog{
				ID:         strings.ToLower(iss.Key) + "-" + string(rune('1'+i)),
				Author:     iss.AssigneeName,
				AuthorName: iss.Assignee,
				Started:    sampleLogDays[i],
				TimeSpent:  workPhrase(secs),
				Seconds:    secs,
			})
		}
	}
	return out
}

// workPhrase is Jira's own way of writing a duration, "3h 30m".
func workPhrase(secs int) string {
	h, m := secs/hour, secs%hour/60
	switch {
	case h == 0:
		return strconv.Itoa(m) + "m"
	case m == 0:
		return strconv.Itoa(h) + "h"
	}
	return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m"
}
