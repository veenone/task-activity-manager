// Package worklogweek puts together what the Worklog view draws: the issues
// down the side and the reader's own entries for the days across the top.
package worklogweek

import (
	"fmt"
	"sort"
	"time"

	"agile-suite/tam/internal/backend"
)

// MaxDays is the longest range one read may cover. The view asks for a week
// and for the fortnight behind today; anything much longer is a different
// question, and each issue in the answer can cost Jira a read of its own.
const MaxDays = 62

// categoryDone is Jira's status category for finished work.
const categoryDone = "done"

// Row is one issue in the grid. Assigned says it is the reader's and still
// open, which is the group the grid leads with. Cached says TAM holds the
// issue: work is journalled against a cached issue, so a row Jira knows and
// the cache does not can be read and not logged on.
type Row struct {
	Issue    backend.Issue `json:"issue"`
	Assigned bool          `json:"assigned"`
	Cached   bool          `json:"cached"`
}

// View is the rows and the reader's entries for the range, oldest first.
type View struct {
	Rows    []Row                `json:"rows"`
	Entries []backend.LoggedWork `json:"entries"`
}

// CheckRange refuses a range that is not two days in order, or is longer
// than MaxDays. The days arrive as text from the window (I1).
func CheckRange(fromDay, toDay string) error {
	from, err := time.Parse(backend.DayLayout, fromDay)
	if err != nil {
		return fmt.Errorf("%q is not a day", fromDay)
	}
	to, err := time.Parse(backend.DayLayout, toDay)
	if err != nil {
		return fmt.Errorf("%q is not a day", toDay)
	}
	if to.Before(from) {
		return fmt.Errorf("the range ends on %s, before it starts on %s", toDay, fromDay)
	}
	if to.Sub(from) > MaxDays*24*time.Hour {
		return fmt.Errorf("the range is longer than %d days", MaxDays)
	}
	return nil
}

// Build joins the reader's assigned issues with what they logged.
//
// The open assigned issues come first, in the order given. After them come
// the other issues an entry in the range names, in the order the entries
// were started: a finished issue, or one that is someone else's. lookup
// answers an issue from the cache; one it does not hold is drawn from the
// key and summary the entry carried.
//
// remote is what Jira holds and pending what the journal holds. Pending
// entries are for every day, so they are narrowed to the range here.
func Build(assigned []backend.Issue, lookup func(key string) (backend.Issue, bool), remote, pending []backend.LoggedWork, fromDay, toDay string) View {
	v := View{Rows: []Row{}, Entries: []backend.LoggedWork{}}
	seen := map[string]bool{}
	for _, iss := range assigned {
		if iss.StatusCategory == categoryDone || seen[iss.Key] {
			continue
		}
		seen[iss.Key] = true
		v.Rows = append(v.Rows, Row{Issue: iss, Assigned: true, Cached: true})
	}
	for _, group := range [][]backend.LoggedWork{remote, pending} {
		for _, e := range group {
			if backend.OnDays(e.Log.Started, fromDay, toDay) {
				v.Entries = append(v.Entries, e)
			}
		}
	}
	sort.SliceStable(v.Entries, func(i, j int) bool { return v.Entries[i].Log.Started < v.Entries[j].Log.Started })
	for _, e := range v.Entries {
		if seen[e.Key] {
			continue
		}
		seen[e.Key] = true
		if iss, ok := lookup(e.Key); ok {
			v.Rows = append(v.Rows, Row{Issue: iss, Cached: true})
			continue
		}
		v.Rows = append(v.Rows, Row{Issue: backend.Issue{Key: e.Key, Summary: e.Summary}})
	}
	return v
}
