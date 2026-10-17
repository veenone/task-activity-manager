package reportout

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// barred is a People-style section whose rows carry a progress bar: one under
// way, one over its target, and one with no target at all.
func barred() Document {
	d := sample()
	d.Sections = append(d.Sections, Section{
		Heading: "People",
		Table: Table{
			Columns:    []string{"Person", "Actual"},
			Rows:       [][]string{{"Rita Anand", "7h"}, {"M. Ortiz", "10h"}, {"J. Park", "0m"}},
			BarHeading: "Actual against estimate",
			Bars:       []*Bar{{Done: 7, Target: 10}, {Done: 10, Target: 8}, nil},
		},
	})
	return d
}

func TestTextBarIsTenSegmentsAndThePercentage(t *testing.T) {
	for _, tc := range []struct {
		bar  *Bar
		want string
	}{
		{&Bar{Done: 7, Target: 10}, "▰▰▰▰▰▰▰▱▱▱ 70%"},
		{&Bar{Done: 10, Target: 8}, "▰▰▰▰▰▰▰▰▰▰ 125%"},
		{&Bar{Done: 0, Target: 4}, "▱▱▱▱▱▱▱▱▱▱ 0%"},
		{&Bar{Done: 3, Target: 0}, ""},
		{nil, ""},
	} {
		if got := textBar(tc.bar); got != tc.want {
			t.Errorf("textBar(%+v) = %q, want %q", tc.bar, got, tc.want)
		}
	}
}

func TestStorageDrawsTheBarColumnAsText(t *testing.T) {
	body := Storage(barred())
	for _, want := range []string{
		"<th>Person</th><th>Actual</th><th>Actual against estimate</th>",
		"<td>Rita Anand</td><td>7h</td><td>▰▰▰▰▰▰▰▱▱▱ 70%</td>",
		"<td>M. Ortiz</td><td>10h</td><td>▰▰▰▰▰▰▰▰▰▰ 125%</td>",
		"<td>J. Park</td><td>0m</td><td></td>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("storage body is missing %q:\n%s", want, body)
		}
	}
}

func TestPPTXDrawsTheBarColumnAsText(t *testing.T) {
	slide := deck(t, barred())["ppt/slides/slide3.xml"]
	for _, want := range []string{"Actual against estimate", "▰▰▰▰▰▰▰▱▱▱ 70%"} {
		if !strings.Contains(slide, want) {
			t.Errorf("the People slide is missing %q", want)
		}
	}
}

func TestXLSXDrawsTheBarColumnAsADataBar(t *testing.T) {
	f, err := excelize.OpenReader(bytes.NewReader(mustXLSX(t, barred())))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	sheet := f.GetSheetList()[0]
	rows, _ := f.GetRows(sheet)
	head := -1
	for i, r := range rows {
		if len(r) == 3 && r[2] == "Actual against estimate" {
			head = i
		}
	}
	if head < 0 {
		t.Fatalf("no bar column heading:\n%s", flat(rows))
	}
	// The cell holds the ratio as a number, shown as a percentage, so the
	// data bar has a value to scale and a reader sees the figure.
	at := func(r int) string { return "C" + strconv.Itoa(head+1+r) }
	for r, want := range map[int]string{1: "0.7", 2: "1.25"} {
		if v, _ := f.GetCellValue(sheet, at(r), excelize.Options{RawCellValue: true}); v != want {
			t.Errorf("%s = %q, want the ratio %s", at(r), v, want)
		}
	}
	if v, _ := f.GetCellValue(sheet, at(1)); v != "70%" {
		t.Errorf("%s shows %q, want 70%%", at(1), v)
	}
	if v, _ := f.GetCellValue(sheet, at(3)); v != "" {
		t.Errorf("a row with no target shows %q, want an empty cell", v)
	}
	formats, err := f.GetConditionalFormats(sheet)
	if err != nil {
		t.Fatalf("conditional formats: %v", err)
	}
	var bar bool
	for ref, opts := range formats {
		for _, o := range opts {
			if o.Type == "data_bar" && strings.HasPrefix(ref, at(1)) {
				bar = true
			}
		}
	}
	if !bar {
		t.Errorf("no data bar over the bar column; formats = %+v", formats)
	}
}

func TestCheckRefusesBarsThatDoNotMatchTheRows(t *testing.T) {
	d := barred()
	d.Sections[1].Table.Bars = d.Sections[1].Table.Bars[:2]
	if err := d.Check(); err == nil {
		t.Error("two bars for three rows was accepted")
	}
	d = barred()
	d.Sections[1].Table.Bars[0] = &Bar{Done: -1, Target: 4}
	if err := d.Check(); err == nil {
		t.Error("a negative bar was accepted")
	}
	if err := barred().Check(); err != nil {
		t.Errorf("a well-formed bar column was refused: %v", err)
	}
}
