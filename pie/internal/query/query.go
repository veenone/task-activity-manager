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

	corejira "agile-suite/core/jira"
)

// MaxRows is where a run stops fetching. Without a project clause nothing else
// bounds a query, and a mistyped one can match the whole instance.
const MaxRows = 5000

// AskAbove is the match count past which Explore asks before fetching. The
// team's normal query matches about 900.
const AskAbove = 2000

const pageSize = 100

var fields = []string{"summary", "project", "issuetype", "status", "priority", "updated", "created", "resolutiondate", "issuelinks"}

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
	res := Result{Issues: []Issue{}}
	for {
		size := min(pageSize, MaxRows-len(res.Issues))
		page, err := client.SearchIssues(ctx, jql, want, nil, len(res.Issues), size)
		if err != nil {
			return Result{}, readable(err)
		}
		res.Total = page.Total
		for _, raw := range page.Issues {
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
			res.Issues = append(res.Issues, is)
		}
		// An empty page ends the walk even if Total claims more: advancing by
		// what the page held is what makes a server's own page limit harmless.
		if len(page.Issues) == 0 || len(res.Issues) >= page.Total || len(res.Issues) >= MaxRows {
			break
		}
	}
	res.Truncated = res.Total > len(res.Issues)
	return res, nil
}

// readable keeps Jira's own sentence for a refused query, because it is the
// only thing that says which clause or filter was wrong. Other failures get
// core's wording, and anything else stays as it is.
func readable(err error) error {
	var he *corejira.HTTPError
	if errors.As(err, &he) && he.Code == http.StatusBadRequest && he.Message != "" {
		return errors.New(he.Message)
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
		switch {
		case l.Outward != nil && l.Outward.Key != "":
			out = append(out, Link{Key: l.Outward.Key, Type: l.Type.Name, Words: l.Type.Outward, Outward: true})
		case l.Inward != nil && l.Inward.Key != "":
			out = append(out, Link{Key: l.Inward.Key, Type: l.Type.Name, Words: l.Type.Inward})
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
