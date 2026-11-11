// Package query runs an analyst's JQL against Jira. Nothing here scopes the
// search to a project: PIE starts from the filter, not from a project.
package query

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"

	corejira "agile-suite/core/jira"
)

// MaxRows is where a run stops fetching. Without a project clause nothing else
// bounds a query, and a mistyped one can match the whole instance.
const MaxRows = 5000

// AskAbove is the match count past which Explore asks before fetching. The
// team's normal query matches about 900.
const AskAbove = 2000

const pageSize = 100

// workers is how many searches PIE sends Jira at once. The client's rate
// limiter still paces them, so this bounds waiting, not load.
const workers = 4

var fields = []string{"summary", "project", "issuetype", "status", "priority", "updated", "created", "resolutiondate", "issuelinks", "parent", "subtasks"}

// Issue is one row of a query's result.
type Issue struct {
	Key     string `json:"key"`
	Project string `json:"project"`
	// ProjectName is what analysts call the project; Project is its key.
	ProjectName string `json:"projectName"`
	Type        string `json:"type"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	Updated     string `json:"updated"`
	// StatusCategory is Jira's own new, indeterminate or done, so PIE keeps no
	// list of which statuses count as finished.
	StatusCategory string `json:"statusCategory"`
	Created        string `json:"created"`
	Resolved       string `json:"resolved"`
	Links          []Link `json:"links"`
	// Parent is a sub-task's parent key and Subtasks an issue's sub-task keys,
	// for the issue graph's parent edges (#464).
	Parent   string   `json:"parent"`
	Subtasks []string `json:"subtasks"`
	// Bases are the values of the profile's product base field. An option field
	// with several options gives several.
	Bases []string `json:"bases"`
	// ClientVisibility and ClientImpact are the profile's client fields as
	// text, "" when unset or not named.
	ClientVisibility string `json:"clientVisibility"`
	ClientImpact     string `json:"clientImpact"`
	// Matched is false for an issue fetched only because another one links to it.
	Matched bool `json:"matched"`
}

// Link is one of an issue's links, worded from this issue's side.
type Link struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Words string `json:"words"`
	// Outward is true when this issue is the link's outward end, so Words is
	// the type's outward wording and the link points from here to Key.
	Outward bool `json:"outward"`
	// Symmetric is true for a type worded the same both ways, such as
	// "relates to", which has no direction to draw.
	Symmetric bool `json:"symmetric"`
}

// Result is what a run fetched. Truncated means Total is more than the query's
// own rows; Total counts only those, never the issues reached through links.
type Result struct {
	Total     int     `json:"total"`
	Issues    []Issue `json:"issues"`
	Truncated bool    `json:"truncated"`
	Sets      []Set   `json:"sets"`
	// LinkError is why following the links stopped. The rows stay.
	LinkError      string `json:"linkError"`
	LinksTruncated bool   `json:"linksTruncated"`
	// NeedsConfirm means nothing was fetched: Total is above AskAbove and the
	// analyst has not yet said to go ahead.
	NeedsConfirm bool `json:"needsConfirm"`
}

// Run pages through every issue the JQL matches, up to MaxRows.
func Run(ctx context.Context, client *corejira.Client, jql string) (Result, error) {
	return run(ctx, client, jql, Fields{})
}

// Fields are the custom field ids a profile reads, "" for one it does not.
type Fields struct {
	Base, Visibility, Impact string
}

// run is Run that also reads the custom fields in extra.
func run(ctx context.Context, client *corejira.Client, jql string, extra Fields) (Result, error) {
	jql = strings.TrimSpace(jql)
	if jql == "" {
		return Result{}, errors.New("enter a JQL query to run")
	}
	want := slices.Clip(fields)
	for _, id := range []string{extra.Base, extra.Visibility, extra.Impact} {
		if id != "" {
			want = append(want, id)
		}
	}
	page := func(start, size int) ([]Issue, int, error) {
		got, err := client.SearchIssues(ctx, jql, want, nil, start, size)
		if err != nil {
			return nil, 0, readable(err)
		}
		out := make([]Issue, 0, len(got.Issues))
		for _, raw := range got.Issues {
			is := toIssue(raw)
			if extra.Base != "" {
				is.Bases = values(raw.Fields[extra.Base])
			}
			if extra.Visibility != "" {
				is.ClientVisibility = strings.Join(values(raw.Fields[extra.Visibility]), ", ")
			}
			if extra.Impact != "" {
				is.ClientImpact = strings.Join(values(raw.Fields[extra.Impact]), ", ")
			}
			out = append(out, is)
		}
		return out, got.Total, nil
	}

	// The first page says how many match and how many a page holds, which can
	// be fewer than asked: Jira imposes its own limit. The rest are fetched
	// at that size, several at once, and put back in order.
	first, total, err := page(0, min(pageSize, MaxRows))
	if err != nil {
		return Result{}, err
	}
	res := Result{Total: total, Issues: first}
	step := len(first)
	end := min(total, MaxRows)
	done := &counter{ctx: ctx, stage: StageIssues, total: end}
	done.add(step)
	if step > 0 && step < end {
		starts := []int{}
		for s := step; s < end; s += step {
			starts = append(starts, s)
		}
		pages := make([][]Issue, len(starts))
		err := parallel(len(starts), func(i int) error {
			got, _, err := page(starts[i], min(step, end-starts[i]))
			pages[i] = got
			done.add(len(got))
			return err
		})
		if err != nil {
			return Result{}, err
		}
		for _, p := range pages {
			res.Issues = append(res.Issues, p...)
		}
	}
	res.Truncated = res.Total > len(res.Issues)
	return res, nil
}

// parallel calls fn for 0 to n-1, at most workers at a time, and returns the
// error of the lowest i that failed, so a failure reads the same on every run.
func parallel(n int, fn func(i int) error) error {
	errs := make([]error, n)
	slots := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := range n {
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-slots; wg.Done() }()
			errs[i] = fn(i)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// readable keeps Jira's own sentence for a refused query, because it is the
// only thing that says which clause or filter was wrong. Other failures get
// core's wording, and anything else stays as it is.
// worded is an error shown in Jira's own words that still unwraps to the
// original, so a caller can tell a refusal from a failure (#464).
type worded struct {
	msg string
	err error
}

func (w worded) Error() string { return w.msg }
func (w worded) Unwrap() error { return w.err }

func readable(err error) error {
	var he *corejira.HTTPError
	if errors.As(err, &he) && he.Code == http.StatusBadRequest && he.Message != "" {
		return worded{he.Message, err}
	}
	if msg := corejira.Humanize(err); msg != "" {
		return errors.New(msg)
	}
	return err
}

func toIssue(raw corejira.RawIssue) Issue {
	return Issue{
		Key:     raw.Key,
		Project: named(raw.Fields["project"], "key"),

		ProjectName: named(raw.Fields["project"], "name"),
		Type:        named(raw.Fields["issuetype"], "name"),
		Summary:     text(raw.Fields["summary"]),
		Status:      named(raw.Fields["status"], "name"),
		Priority:    named(raw.Fields["priority"], "name"),
		Updated:     text(raw.Fields["updated"]),

		StatusCategory: text(object(object(raw.Fields["status"])["statusCategory"])["key"]),
		Created:        text(raw.Fields["created"]),
		Resolved:       text(raw.Fields["resolutiondate"]),
		Links:          links(raw.Fields["issuelinks"]),
		Parent:         named(raw.Fields["parent"], "key"),
		Subtasks:       keys(raw.Fields["subtasks"]),
		Matched:        true,
	}
}

func links(raw json.RawMessage) []Link {
	var ls []struct {
		Type struct {
			Name    string `json:"name"`
			Inward  string `json:"inward"`
			Outward string `json:"outward"`
		} `json:"type"`
		Outward *struct{ Key string } `json:"outwardIssue"`
		Inward  *struct{ Key string } `json:"inwardIssue"`
	}
	if json.Unmarshal(raw, &ls) != nil {
		return nil
	}
	var out []Link
	for _, l := range ls {
		symmetric := strings.EqualFold(strings.TrimSpace(l.Type.Inward), strings.TrimSpace(l.Type.Outward))
		switch {
		case l.Outward != nil && l.Outward.Key != "":
			out = append(out, Link{Key: l.Outward.Key, Type: l.Type.Name, Words: l.Type.Outward, Outward: true, Symmetric: symmetric})
		case l.Inward != nil && l.Inward.Key != "":
			out = append(out, Link{Key: l.Inward.Key, Type: l.Type.Name, Words: l.Type.Inward, Symmetric: symmetric})
		}
	}
	return out
}

// keys reads a list of issues, such as subtasks, as their keys.
func keys(raw json.RawMessage) []string {
	var is []struct{ Key string }
	if json.Unmarshal(raw, &is) != nil {
		return nil
	}
	out := make([]string, 0, len(is))
	for _, i := range is {
		if i.Key != "" {
			out = append(out, i.Key)
		}
	}
	return out
}

// text and named read a field Jira may omit, null or shape differently. Each
// gives "" rather than losing the row (I1).
func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

// values reads a custom field as text: a string, an option's value, or each
// option of a multi-select. Blank values are dropped.
func values(raw json.RawMessage) []string {
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		list = []json.RawMessage{raw}
	}
	var out []string
	for _, v := range list {
		s := text(v)
		if s == "" {
			s = named(v, "value")
		}
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func named(raw json.RawMessage, key string) string {
	return text(object(raw)[key])
}

func object(raw json.RawMessage) map[string]json.RawMessage {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	return obj
}
