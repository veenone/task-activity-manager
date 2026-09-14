// Package ritualtemplate renders the five pages a sprint's rituals start
// from, in Confluence storage format. It does no I/O and reads no clock:
// ritualsync compares a stored body against a fresh render to tell a page
// nobody has touched from one somebody wrote in, and that comparison is only
// honest if the same sprint always renders the same bytes.
package ritualtemplate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"agile-suite/tam/internal/sprintdate"
)

const (
	Sprint   = "_sprint"
	Planning = "planning"
	Standup  = "standup"
	Review   = "review"
	Retro    = "retro"
)

// Types is every page a sprint gets, the overview first because Sync has to
// create it before any ritual page can sit under it.
var Types = []string{Sprint, Planning, Standup, Review, Retro}

var labels = map[string]string{
	Sprint: "Overview", Planning: "Planning", Standup: "Standup", Review: "Review", Retro: "Retrospective",
}

func normalize(ritualType string) string { return strings.ToLower(strings.TrimSpace(ritualType)) }

// Known reports whether ritualType names one of the five pages.
func Known(ritualType string) bool { _, ok := labels[normalize(ritualType)]; return ok }

// Label is a page's name on screen and in its title.
func Label(ritualType string) string { return labels[normalize(ritualType)] }

// SprintInfo is everything a template reads, all of it from the board cache.
type SprintInfo struct {
	ID        int
	Name      string
	Goal      string
	StartDate string
	EndDate   string
	BoardName string
}

func (s SprintInfo) name() string {
	if n := strings.TrimSpace(s.Name); n != "" {
		return n
	}
	return "Sprint " + strconv.Itoa(s.ID)
}

// Title is the page title Sync creates and matches by. Confluence keeps
// titles unique within a space, which is why ritual pages carry the sprint.
func Title(ritualType string, s SprintInfo) string {
	if normalize(ritualType) == Sprint {
		return s.name()
	}
	return s.name() + " · " + Label(ritualType)
}

// Filter is which of a sprint's issues a Jira Issues macro lists.
type Filter int

const (
	All Filter = iota
	Done
	NotDone
)

// JQL is the query a macro carries. These three forms are the only ones TAM
// writes, because they are the only ones it can preview from its cache.
func JQL(sprintID int, f Filter) string {
	switch f {
	case Done:
		return fmt.Sprintf("sprint = %d AND statusCategory = Done", sprintID)
	case NotDone:
		return fmt.Sprintf("sprint = %d AND statusCategory != Done", sprintID)
	}
	return fmt.Sprintf("sprint = %d ORDER BY Rank", sprintID)
}

var jqlForm = regexp.MustCompile(`(?i)^\s*sprint\s*=\s*(\d+)\s*(?:AND\s+statusCategory\s*(!=|=)\s*Done|ORDER\s+BY\s+Rank)?\s*$`)

// ParseJQL reads a macro's query back. ok is false for anything but the
// three forms JQL writes, however reasonable the query.
func ParseJQL(jql string) (int, Filter, bool) {
	m := jqlForm.FindStringSubmatch(jql)
	if m == nil {
		return 0, All, false
	}
	id, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, All, false
	}
	switch m[2] {
	case "=":
		return id, Done, true
	case "!=":
		return id, NotDone, true
	}
	return id, All, true
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func esc(s string) string { return escaper.Replace(s) }

type page struct{ strings.Builder }

func (p *page) h2(text string)   { p.WriteString("<h2>" + esc(text) + "</h2>") }
func (p *page) para(text string) { p.WriteString("<p>" + esc(text) + "</p>") }
func (p *page) emptyList()       { p.WriteString("<ul><li></li></ul>") }
func (p *page) tasks()           { p.WriteString(taskList) }

const taskList = "<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body></ac:task-body></ac:task></ac:task-list>"

func (p *page) table(headers ...string) {
	p.WriteString("<table><tbody><tr>")
	for _, h := range headers {
		p.WriteString("<th>" + esc(h) + "</th>")
	}
	p.WriteString("</tr>")
	for row := 0; row < 3; row++ {
		p.WriteString("<tr>" + strings.Repeat("<td></td>", len(headers)) + "</tr>")
	}
	p.WriteString("</tbody></table>")
}

func (p *page) jira(jql string, storyPoints bool) {
	columns := "key,summary,type,status,assignee"
	if storyPoints {
		columns += ",story points"
	}
	p.WriteString(`<ac:structured-macro ac:name="jira">` +
		`<ac:parameter ac:name="jqlQuery">` + esc(jql) + `</ac:parameter>` +
		`<ac:parameter ac:name="columns">` + columns + `</ac:parameter>` +
		`<ac:parameter ac:name="maximumIssues">50</ac:parameter>` +
		`</ac:structured-macro>`)
}

func day(value string, loc *time.Location) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	t, err := sprintdate.Parse(value)
	if err != nil {
		return time.Time{}, false
	}
	return t.In(loc), true
}

func dates(s SprintInfo, loc *time.Location) string {
	start, okStart := day(s.StartDate, loc)
	end, okEnd := day(s.EndDate, loc)
	if !okStart && !okEnd {
		return "Dates not set"
	}
	format := func(t time.Time, ok bool) string {
		if !ok {
			return "not set"
		}
		return t.Format("2 Jan 2006")
	}
	return format(start, okStart) + " to " + format(end, okEnd)
}

// Render is the page a ritual starts as. loc decides which local day a
// sprint date falls on; nil reads as UTC.
func Render(ritualType string, s SprintInfo, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	var p page
	switch normalize(ritualType) {
	case Sprint:
		p.WriteString("<p><strong>Board:</strong> " + esc(s.BoardName) + "</p>")
		p.WriteString("<p><strong>Dates:</strong> " + esc(dates(s, loc)) + "</p>")
		p.h2("Sprint goal")
		p.para(s.Goal)
		p.h2("Rituals")
		p.WriteString(`<ac:structured-macro ac:name="children" />`)
	case Planning:
		p.h2("Sprint goal")
		p.para(s.Goal)
		p.h2("Capacity")
		p.table("Member", "Days available", "Notes")
		p.h2("Committed scope")
		p.jira(JQL(s.ID, All), true)
		p.h2("Risks and dependencies")
		p.emptyList()
		p.h2("Decisions")
		p.emptyList()
		p.h2("Action items")
		p.tasks()
	case Standup:
		p.h2("Blockers and work in flight")
		p.jira(JQL(s.ID, NotDone), false)
		p.h2("Daily log")
		if start, ok := day(s.StartDate, loc); ok {
			p.WriteString(StandupEntry(start))
		}
	case Review:
		p.h2("Sprint goal")
		p.para(s.Goal)
		p.para("Met / Partly met / Not met")
		p.h2("Completed")
		p.jira(JQL(s.ID, Done), false)
		p.h2("Not completed")
		p.jira(JQL(s.ID, NotDone), false)
		p.h2("Demo notes")
		p.emptyList()
		p.h2("Stakeholder feedback")
		p.table("Who", "Feedback", "Follow up")
		p.h2("Follow ups")
		p.tasks()
	case Retro:
		p.h2("What went well")
		p.emptyList()
		p.h2("What did not")
		p.emptyList()
		p.h2("What we will try")
		p.emptyList()
		p.h2("Carried over")
		p.jira(JQL(s.ID, NotDone), false)
		p.h2("Action items")
		p.tasks()
	}
	return p.String()
}

// StandupEntry is one day of the standup's daily log. The editor's "Add
// today's entry" inserts exactly this, so the log and the template agree.
func StandupEntry(day time.Time) string {
	return "<h3>" + day.Format("Mon 2 Jan 2006") + "</h3>" +
		"<p><strong>Yesterday</strong></p><ul><li></li></ul>" +
		"<p><strong>Today</strong></p><ul><li></li></ul>" +
		"<p><strong>Blockers</strong></p>" + taskList
}

// Note is one remark the retired wizard stored against an issue.
type Note struct {
	Key    string
	Remark string
}

// EarlierNotes carries what somebody typed into the retired wizard onto the
// page that replaces it. It answers "" when nothing was written.
func EarlierNotes(remark string, notes []Note) string {
	var kept []Note
	for _, n := range notes {
		if strings.TrimSpace(n.Remark) != "" {
			kept = append(kept, n)
		}
	}
	if strings.TrimSpace(remark) == "" && len(kept) == 0 {
		return ""
	}
	var p page
	p.h2("Earlier draft notes")
	if strings.TrimSpace(remark) != "" {
		p.para(remark)
	}
	if len(kept) > 0 {
		p.WriteString("<ul>")
		for _, n := range kept {
			p.WriteString("<li>" + esc(n.Key+": "+n.Remark) + "</li>")
		}
		p.WriteString("</ul>")
	}
	return p.String()
}
