// Package query runs an analyst's JQL against Jira. Nothing here scopes the
// search to a project: PIE starts from the filter, not from a project.
package query

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	corejira "agile-suite/core/jira"
)

// MaxRows is where a run stops fetching. Without a project clause nothing else
// bounds a query, and a mistyped one can match the whole instance.
//
// ponytail: a fixed ceiling reported in the result. The plan's confirm-before-a-
// large-fetch step replaces it once results are cached locally.
const MaxRows = 5000

const pageSize = 100

var fields = []string{"summary", "project", "issuetype", "status", "priority", "updated"}

// Issue is one row of a query's result.
type Issue struct {
	Key      string `json:"key"`
	Project  string `json:"project"`
	Type     string `json:"type"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Updated  string `json:"updated"`
}

// Result is what a run fetched. Truncated means Total is more than Issues holds.
type Result struct {
	Total     int     `json:"total"`
	Issues    []Issue `json:"issues"`
	Truncated bool    `json:"truncated"`
}

// Run pages through every issue the JQL matches, up to MaxRows.
func Run(ctx context.Context, client *corejira.Client, jql string) (Result, error) {
	jql = strings.TrimSpace(jql)
	if jql == "" {
		return Result{}, errors.New("enter a JQL query to run")
	}
	res := Result{Issues: []Issue{}}
	for {
		size := min(pageSize, MaxRows-len(res.Issues))
		page, err := client.SearchIssues(ctx, jql, fields, nil, len(res.Issues), size)
		if err != nil {
			return Result{}, readable(err)
		}
		res.Total = page.Total
		for _, raw := range page.Issues {
			res.Issues = append(res.Issues, toIssue(raw))
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
		Key:      raw.Key,
		Project:  named(raw.Fields["project"], "key"),
		Type:     named(raw.Fields["issuetype"], "name"),
		Summary:  text(raw.Fields["summary"]),
		Status:   named(raw.Fields["status"], "name"),
		Priority: named(raw.Fields["priority"], "name"),
		Updated:  text(raw.Fields["updated"]),
	}
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

func named(raw json.RawMessage, key string) string {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	return text(obj[key])
}
