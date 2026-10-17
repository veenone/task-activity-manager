package reportout

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Bar is one row's progress against its target, for a table that shows a
// visual indicator beside its figures: the People section's actual time
// against the estimate (#287). Target zero is a row with nothing to measure
// against, and it draws no bar.
type Bar struct {
	Done   float64 `json:"done"`
	Target float64 `json:"target"`
}

// barSegments is how wide a text bar is. Ten makes each segment a tenth,
// which a reader can count.
const barSegments = 10

// textBar is a bar drawn in text, for the outputs that cannot draw a shape
// in a table cell: the Confluence page and the deck. A no-break space joins
// it to its percentage so a narrow cell keeps them on one line, and a full
// bar past the target keeps its real percentage, so an overrun reads as one.
func textBar(b *Bar) string {
	if b == nil || b.Target <= 0 {
		return ""
	}
	ratio := b.Done / b.Target
	filled := int(math.Round(math.Min(ratio, 1) * barSegments))
	return strings.Repeat("▰", filled) + strings.Repeat("▱", barSegments-filled) +
		fmt.Sprintf(" %d%%", int(math.Round(ratio*100)))
}

// ratio is a bar's done over its target, and false for a row with no bar.
func (b *Bar) ratio() (float64, bool) {
	if b == nil || b.Target <= 0 {
		return 0, false
	}
	return b.Done / b.Target, true
}

// withBars is the table as a text-only output lays it out: the bar column
// last, its cells drawn by textBar. A table with no bars is itself.
func withBars(t Table) Table {
	if len(t.Bars) == 0 {
		return t
	}
	out := Table{Columns: append(append([]string{}, t.Columns...), t.BarHeading)}
	for i, row := range t.Rows {
		out.Rows = append(out.Rows, append(append([]string{}, row...), textBar(t.Bars[i])))
	}
	return out
}

// checkBars refuses a bar column that does not fit its table (I1: it crosses
// the binding): one bar per row, each finite and not negative.
func checkBars(heading string, t Table) error {
	if len(t.Bars) == 0 {
		return nil
	}
	if len(t.Bars) != len(t.Rows) {
		return fmt.Errorf("the %s table has %d rows and %d bars; each row needs one, even if empty", heading, len(t.Rows), len(t.Bars))
	}
	if t.BarHeading == "" {
		return fmt.Errorf("the %s table's bar column has no heading", heading)
	}
	for _, b := range t.Bars {
		if b == nil {
			continue
		}
		for _, v := range []float64{b.Done, b.Target} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return errors.New("a progress bar in the " + heading + " table holds a value that is not a count")
			}
		}
	}
	return nil
}
