package report

import (
	"strings"
	"testing"

	"agile-suite/pie/internal/cases"
	"agile-suite/pie/internal/cycles"
)

func TestStorageHoldsTheCountsTheGridAndEachReason(t *testing.T) {
	d := cycles.Detail{
		Cycle: cycles.Cycle{Name: "Week 45"},
		Cases: []cases.Detail{
			{Case: cases.Case{Key: "ACME-1"}, Projects: []cases.Project{
				{Key: "BETA", Verdict: cases.NotAffected, Reason: "no parser"},
				{Key: "ALPHA", Verdict: cases.Affected, Reason: "ships it", AssessedAt: "2026-11-07T10:00:00Z"},
			}},
			{Case: cases.Case{Key: "ACME-2"}, Projects: []cases.Project{}},
		},
		Changes: cycles.Changes{
			Since: "Week 44", New: []string{"ACME-2"}, Gone: []string{},
			Verdicts: []cycles.Change{{Case: "ACME-1", Project: "ALPHA", From: cases.Investigating, To: cases.Affected}},
		},
	}
	got := Storage(d)
	for _, want := range []string{
		"<p>Compared with Week 44, the last cycle published before this one.</p>",
		"<li>1 new cases: ACME-2</li>",
		"<li>0 cases gone</li>",
		"<li>1 verdicts changed</li>",
		"<tr><td>ACME-1</td><td>ALPHA</td><td>investigating</td><td>affected</td></tr>",
		"<tr><th>Case</th><th>ALPHA</th><th>BETA</th></tr>",
		"<tr><td>ACME-1</td><td>affected</td><td>not affected</td></tr>",
		"<tr><td>ACME-2</td><td></td><td></td></tr>",
		"<h3>ACME-1</h3>",
		"<tr><td>ALPHA</td><td>affected</td><td>ships it</td><td>2026-11-07</td></tr>",
		"<h3>ACME-2</h3><p>No projects on this case.</p>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("storage lacks %s\n%s", want, got)
		}
	}
	if strings.Index(got, "<td>ALPHA</td><td>affected</td><td>ships it") > strings.Index(got, "<td>BETA</td><td>not affected") {
		t.Error("the projects of a case are not in key order")
	}
}
