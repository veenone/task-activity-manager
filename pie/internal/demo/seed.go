package demo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corejira "agile-suite/core/jira"
)

// row is one invented issue. Dates are days before today, so ages stay
// realistic however long after this was written the demo runs. resolved is 0
// for an issue with no resolution date.
type row struct {
	key, typ, status, priority, summary string
	created, resolved                   int
}

// linked holds one example of every case the linked-issue views must show.
// The comment above each group names the case.
var linked = []row{
	// Out of step: a clone chain where the first fix closed and the clones did not.
	{"ALPHA-101", "Bug", "Closed", "High", "Card reset loses the PIN retry counter", 420, 200},
	{"BRAVO-201", "Bug", "Open", "High", "Card reset loses the PIN retry counter", 400, 0},
	{"CHARLIE-301", "Bug", "Postponed", "Medium", "Card reset loses the PIN retry counter", 380, 0},
	// In step: every clone closed.
	{"ALPHA-102", "Bug", "Closed", "Medium", "Log export truncates long entries", 300, 150},
	{"DELTA-401", "Bug", "Closed", "Medium", "Log export truncates long entries", 290, 120},
	// Corrected, which Jira counts as done but never dates, beside an open issue.
	{"BRAVO-202", "Bug", "Corrected", "High", "Wrong error code after a failed update", 250, 0},
	{"ECHO-501", "Bug", "Open", "Medium", "Update error codes differ between readers", 240, 0},
	// A Rejected clone beside a Closed one: both finished, so in step.
	{"ALPHA-103", "Bug", "Closed", "Low", "Timeout on slow readers", 510, 260},
	{"CHARLIE-302", "Bug", "Rejected", "Low", "Timeout on slow readers", 500, 480},
	// A set across four projects, through clones and a duplicate.
	{"ALPHA-104", "Bug", "Open", "Highest", "Memory leak in the session cache", 180, 0},
	{"BRAVO-203", "Bug", "Closed", "Highest", "Memory leak in the session cache", 170, 60},
	{"DELTA-402", "Bug", "Open", "High", "Memory leak in the session cache", 165, 0},
	{"FOXTROT-601", "Bug", "Rejected", "High", "Session cache grows without bound", 160, 150},
	// A link to an issue the query does not list (see linkOnly).
	{"DELTA-403", "Bug", "Open", "High", "Signature check skipped on retry", 90, 0},
	{"ECHO-502", "Story", "Closed", "Medium", "Rework the retry path", 700, 600},
	// A Rejected clone beside an Open one: out of step by Jira's category.
	{"BRAVO-204", "Bug", "Open", "Medium", "Status page shows a stale firmware version", 130, 0},
	{"CHARLIE-303", "Bug", "Rejected", "Medium", "Status page shows a stale firmware version", 120, 100},
}

// links run from the first key to the second: the first issue shows the
// type's outward wording, the second its inward wording.
var links = []struct{ from, to, typ string }{
	{"BRAVO-201", "ALPHA-101", "Cloners"},
	{"CHARLIE-301", "ALPHA-101", "Cloners"},
	{"DELTA-401", "ALPHA-102", "Cloners"},
	{"BRAVO-202", "ECHO-501", "Relates"},
	{"CHARLIE-302", "ALPHA-103", "Cloners"},
	{"BRAVO-203", "ALPHA-104", "Cloners"},
	{"DELTA-402", "ALPHA-104", "Cloners"},
	{"FOXTROT-601", "ALPHA-104", "Duplicate"},
	{"ECHO-502", "DELTA-403", "Defect"},
	{"CHARLIE-303", "BRAVO-204", "Cloners"},
}

// linkOnly issues are missing from a plain query and returned only by key in
// (...), as an issue outside the analyst's JQL is when the walk fetches it.
var linkOnly = map[string]bool{"ECHO-502": true}

var linkTypes = map[string][2]string{ // name: outward, inward
	"Cloners":   {"clones", "is cloned by"},
	"Relates":   {"relates to", "relates to"},
	"Duplicate": {"duplicates", "is duplicated by"},
	"Defect":    {"causes", "is caused by"},
}

var category = map[string]string{
	"Open": "new", "Postponed": "indeterminate", "Closed": "done", "Rejected": "done", "Corrected": "done",
}

var projects = []string{"ALPHA", "BRAVO", "CHARLIE", "DELTA", "ECHO", "FOXTROT"}

// unlinked fills the seed out to a realistic spread of statuses, ages and
// times to resolve. The arithmetic only spreads the values; none of it means
// anything.
func unlinked() []row {
	statuses := []string{"Open", "Closed", "Rejected", "Postponed", "Closed", "Corrected", "Open"}
	priorities := []string{"High", "Medium", "Low"}
	summaries := []string{
		"Crash when the reader is unplugged", "Wrong date format in the audit log", "Slow start with many profiles",
		"Typo in the install dialog", "Retry counter not shown", "Export fails on an empty list",
		"Help link opens the wrong page", "Firmware check ignores the patch number",
	}
	var out []row
	for i := range 47 {
		r := row{
			key:      fmt.Sprintf("%s-%d", projects[i%len(projects)], 1+i/len(projects)),
			typ:      "Bug",
			status:   statuses[i%len(statuses)],
			priority: priorities[i%len(priorities)],
			summary:  summaries[i%len(summaries)],
			created:  20 + (i*37)%1000,
		}
		if i%5 == 4 {
			r.typ = "Story"
		}
		if r.status == "Closed" || r.status == "Rejected" {
			r.resolved = max(1, r.created-(10+(i*53)%400))
		}
		out = append(out, r)
	}
	return out
}

// render builds the issues as Jira's search returns them, dated from now.
func render(now time.Time) []corejira.RawIssue {
	rows := append(append([]row{}, linked...), unlinked()...)
	byKey := map[string]row{}
	for _, r := range rows {
		byKey[r.key] = r
	}
	issueLinks := map[string][]any{}
	for _, l := range links {
		words := linkTypes[l.typ]
		typ := map[string]any{"name": l.typ, "outward": words[0], "inward": words[1]}
		issueLinks[l.from] = append(issueLinks[l.from], map[string]any{"type": typ, "outwardIssue": summary(byKey[l.to])})
		issueLinks[l.to] = append(issueLinks[l.to], map[string]any{"type": typ, "inwardIssue": summary(byKey[l.from])})
	}
	day := func(ago int) string { return now.AddDate(0, 0, -ago).Format("2006-01-02T15:04:05.000-0700") }
	out := make([]corejira.RawIssue, 0, len(rows))
	for i, r := range rows {
		f := map[string]any{
			"summary":   r.summary,
			"project":   map[string]any{"key": projectOf(r.key), "name": projectOf(r.key) + " product line"},
			"issuetype": map[string]any{"name": r.typ},
			"status":    status(r.status),
			"priority":  map[string]any{"name": r.priority},
			"created":   day(r.created),
			"updated":   day(r.created / 3),
			// Jira sends null, not an absent field, for an unresolved issue.
			"resolutiondate": nil,
			"issuelinks":     append([]any{}, issueLinks[r.key]...),
		}
		if r.resolved > 0 {
			f["resolutiondate"] = day(r.resolved)
			f["updated"] = day(r.resolved)
		}
		out = append(out, corejira.RawIssue{ID: fmt.Sprint(10000 + i), Key: r.key, Fields: raw(f)})
	}
	return out
}

// summary is the cut-down issue a link carries: Jira sends only these fields.
func summary(r row) map[string]any {
	return map[string]any{"key": r.key, "fields": map[string]any{
		"summary": r.summary, "status": status(r.status),
		"priority": map[string]any{"name": r.priority}, "issuetype": map[string]any{"name": r.typ},
	}}
}

func status(name string) map[string]any {
	return map[string]any{"name": name, "statusCategory": map[string]any{"key": category[name]}}
}

func projectOf(key string) string {
	p, _, _ := strings.Cut(key, "-")
	return p
}

func raw(f map[string]any) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(f))
	for k, v := range f {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err) // the seed is static; a value that cannot marshal is a bug here
		}
		out[k] = b
	}
	return out
}
