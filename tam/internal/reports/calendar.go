package reports

import "time"

// The calendar the walk counts in: the guide line's arithmetic and the day
// stepping underneath it. It is split out of series.go, which the walk
// itself fills.

// ideal is the guide line's value after elapsed working days of a sprint
// that has working of them, which is the committed total run down to zero
// in equal steps. A sprint with no working day at all, a weekend hackathon
// for instance, keeps its guide flat rather than dividing by zero.
//
// An overdue active sprint continues past its planned end. Its ideal line
// stays at zero rather than turning negative while actual work continues.
func ideal(committed float64, elapsed, working int) float64 {
	if working <= 0 {
		return committed
	}
	if elapsed >= working {
		return 0
	}
	return committed * (1 - float64(elapsed)/float64(working))
}

// civil is the calendar date t falls on in loc, carried as a UTC midnight.
//
// The arithmetic below counts and steps days, and it does that in UTC on
// purpose: UTC has no daylight saving, so adding a day can never land on
// the same date twice or skip one, which stepping a local midnight through
// a spring forward can. The one place a real local moment is needed, the
// midnight a day's changes are cut off at, builds it from these fields in
// loc rather than reusing this value.
func civil(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// daysBetween is how many whole days separate two civil dates, negative
// when to is the earlier one.
func daysBetween(from, to time.Time) int {
	return int(to.Sub(from).Hours() / 24)
}

// workingDays counts the Mondays to Fridays from one civil date to another
// inclusive.
func workingDays(from, to time.Time) int {
	n := 0
	for i := 0; i <= daysBetween(from, to); i++ {
		if isWorkingDay(from.AddDate(0, 0, i)) {
			n++
		}
	}
	return n
}

// isWorkingDay is the guide line's definition of a day the team works,
// which is Monday to Friday. It is not configurable: a per-profile working
// week and a holiday calendar are a feature with their own settings and
// their own tests, and nothing in this phase's design asks for one.
func isWorkingDay(d time.Time) bool {
	switch d.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return true
}
