package sprints

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"agile-suite/tam/internal/sprintdate"
)

// DefaultLength is how long a sprint is assumed to run when the board has no
// closed sprints to measure: a fortnight, which is what the field shows and
// what the dialog focuses so it is the first thing the user can correct.
const DefaultLength = 14

// Suggestion is what the start dialog opens with: the dates it fills in, the
// name the board's own numbering implies, and whether any of it came from
// the board's history at all.
type Suggestion struct {
	// Name is the name the board's last sprint suggests for the next one,
	// empty when its name carries no number to follow.
	Name string `json:"name"`
	// Start and End are bare dates, the shape a date input takes.
	Start string `json:"start"`
	End   string `json:"end"`
	// Length is the whole days between them.
	Length int `json:"length"`
	// FromHistory is false when the board had no closed sprint to measure,
	// so the dialog can focus the end date rather than let a default it
	// invented pass unread.
	FromHistory bool `json:"fromHistory"`
}

// Suggest builds those defaults: today, today plus the board's own sprint
// length, and the name that follows the board's last one. length is what
// boardrepo.SprintLength answered with, and zero means the board has no
// history, which is where the fortnight comes from.
func Suggest(now time.Time, length int, lastSprintName string) Suggestion {
	s := Suggestion{Length: length, FromHistory: length > 0}
	if !s.FromHistory {
		s.Length = DefaultLength
	}
	s.Start = sprintdate.FormatDay(now)
	s.End = sprintdate.FormatDay(now.AddDate(0, 0, s.Length))
	s.Name = NextName(lastSprintName)
	return s
}

// NextName is the name that follows the board's last sprint: the same words
// with the trailing number moved on by one, keeping the width a padded
// number was written with. A name with no trailing number suggests nothing,
// and the dialog falls back to the sprint's own name rather than offer a
// duplicate of the one before it.
func NextName(last string) string {
	name := strings.TrimRight(last, " ")
	end := len(name)
	start := end
	for start > 0 && name[start-1] >= '0' && name[start-1] <= '9' {
		start--
	}
	if start == end {
		return ""
	}
	digits := name[start:end]
	n, err := strconv.Atoi(digits)
	if err != nil {
		return ""
	}
	return name[:start] + fmt.Sprintf("%0*d", len(digits), n+1)
}
