package backend

import "context"

// DayLayout is a calendar day as the Worklog view names one: no clock and no
// zone, because the day a worklog belongs to is the one its own offset puts
// it on and never an instant.
const DayLayout = "2006-01-02"

// LoggedWork is one worklog with the issue it was logged on. Summary is what
// Jira calls the issue, carried so a row can be drawn for an issue the cache
// does not hold.
type LoggedWork struct {
	Key     string  `json:"key"`
	Summary string  `json:"summary"`
	Log     Worklog `json:"log"`
}

// OwnWorklogBackend reads one person's worklogs across issues. It is its own
// optional interface because only the Worklog view asks: the panel's section
// reads one issue, and this reads one person.
type OwnWorklogBackend interface {
	// WorklogsBy is every entry author logged on a day from fromDay to toDay,
	// both included and both in DayLayout.
	WorklogsBy(ctx context.Context, author, fromDay, toDay string) ([]LoggedWork, error)
}

// WorklogDay is the calendar day an entry was logged on: the date its started
// stamp reads in its own offset. It is "" for a stamp too short to hold one.
func WorklogDay(started string) string {
	if len(started) < len(DayLayout) {
		return ""
	}
	return started[:len(DayLayout)]
}

// OnDays reports whether an entry started on a day from fromDay to toDay.
// Days in DayLayout order as text the way they order as dates, so the
// comparison needs no parse.
func OnDays(started, fromDay, toDay string) bool {
	day := WorklogDay(started)
	return day != "" && day >= fromDay && day <= toDay
}
