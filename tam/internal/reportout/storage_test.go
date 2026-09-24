package reportout

import (
	"strings"
	"testing"
)

func sample() Document {
	return Document{
		Title: "Sprint 11 · Report",
		Sections: []Section{{
			Heading: "Sprint outcome",
			Lines:   []string{"Closed sprint · final results"},
			Table: Table{
				Columns: []string{"Figure", "Amount"},
				Rows:    [][]string{{"Committed", "34 points"}, {"Completed", "29 points"}},
			},
			Notes: []string{"Committed is a minimum estimate."},
		}},
	}
}

func TestCheckRefusesADocumentWithNothingInIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  Document
	}{
		{"no title", Document{Sections: sample().Sections}},
		{"no sections", Document{Title: "Sprint 11 · Report"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.doc.Check(); err == nil {
				t.Fatal("want a refusal, got nil")
			}
		})
	}
	if err := sample().Check(); err != nil {
		t.Fatalf("want a whole document accepted, got %v", err)
	}
}

func TestStorageCarriesTheHeadingTheLinesTheTableAndTheNotes(t *testing.T) {
	body := Storage(sample())
	for _, want := range []string{
		"<h2>Sprint outcome</h2>",
		"<p>Closed sprint · final results</p>",
		"<th>Figure</th><th>Amount</th>",
		"<td>Committed</td><td>34 points</td>",
		"<p>Committed is a minimum estimate.</p>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("storage body is missing %q:\n%s", want, body)
		}
	}
	if i, j := strings.Index(body, "34 points"), strings.Index(body, "Committed is a minimum"); i > j {
		t.Error("the caveat is above the figures it qualifies; it belongs under them")
	}
}

func TestStorageEscapesWhatWouldBeMarkup(t *testing.T) {
	d := sample()
	d.Sections[0].Lines = []string{`a <b> & "quoted" sprint`}
	body := Storage(d)
	if strings.Contains(body, "<b>") {
		t.Errorf("an angle bracket reached the page as markup:\n%s", body)
	}
	if !strings.Contains(body, "&lt;b&gt; &amp; &#34;quoted&#34;") {
		t.Errorf("the escaped text is missing:\n%s", body)
	}
}

func TestStorageLeavesOutATableWithNoColumns(t *testing.T) {
	d := sample()
	d.Sections[0].Table = Table{}
	if strings.Contains(Storage(d), "<table>") {
		t.Error("an empty table was written; a section with no rows says why in its lines instead")
	}
}
