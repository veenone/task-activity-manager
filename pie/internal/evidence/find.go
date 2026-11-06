package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	corejira "agile-suite/core/jira"
)

// searchCap bounds each name or phrase search. A component shared by more
// issues than this is still found in every project it reaches, unless one
// project holds them all; the count in the detail is then a floor.
const searchCap = 100

// source is what Find reads of the case's issue.
type source struct {
	Fields struct {
		Project    struct{ Key string }    `json:"project"`
		IssueType  struct{ Name string }   `json:"issuetype"`
		Components []struct{ Name string } `json:"components"`
		Versions   []struct{ Name string } `json:"fixVersions"`
		Links      []json.RawMessage       `json:"issuelinks"`
	} `json:"fields"`
}

type link struct {
	Type struct {
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"type"`
	Outward *linked `json:"outwardIssue"`
	Inward  *linked `json:"inwardIssue"`
}

type linked struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string                `json:"summary"`
		Status  struct{ Name string } `json:"status"`
	} `json:"fields"`
}

// Find reads the case's issue and stores suggestions for every other project
// it points at: its links, projects with a component or fix version of the
// same name, and issues of its type matching the analyst's quoted phrases.
// It returns how many suggestions are new. Phrases are checked before Jira
// is asked anything.
func Find(ctx context.Context, client *corejira.Client, s *Store, profileID string, caseID int64, key string, phrases []string) (int, error) {
	var quoted []string
	for _, p := range phrases {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.ContainsAny(p, `"\`) {
			return 0, fmt.Errorf("a phrase cannot hold a quote or a backslash: %s", p)
		}
		quoted = append(quoted, p)
	}

	raw, err := client.GetIssue(ctx, key, []string{"project", "issuetype", "components", "fixVersions", "issuelinks"})
	if err != nil {
		return 0, fmt.Errorf("read %s from Jira: %s", key, readable(err))
	}
	var src source
	b, _ := json.Marshal(raw)
	if err := json.Unmarshal(b, &src); err != nil {
		return 0, fmt.Errorf("read %s from Jira: %w", key, err)
	}
	home := src.Fields.Project.Key

	var found []Item
	for _, l := range src.Fields.Links {
		var ln link
		if json.Unmarshal(l, &ln) != nil {
			continue
		}
		other, words := ln.Outward, ln.Type.Outward
		if other == nil {
			other, words = ln.Inward, ln.Type.Inward
		}
		if other == nil || projectOf(other.Key) == home || projectOf(other.Key) == "" {
			continue
		}
		found = append(found, Item{
			Project: projectOf(other.Key), Kind: Link, Ref: other.Key,
			Detail: fmt.Sprintf("%s %s: %s (%s)", words, other.Key, other.Fields.Summary, other.Fields.Status.Name),
		})
	}

	names := []struct {
		kind, field, label string
		values             []struct{ Name string }
	}{
		{Component, "component", "component", src.Fields.Components},
		{Version, "fixVersion", "fix version", src.Fields.Versions},
	}
	for _, n := range names {
		for _, v := range n.values {
			// One name per search: a name Jira does not know fails the whole
			// query, and it must not take the other names with it.
			hits, err := search(ctx, client, fmt.Sprintf("%s = %s", n.field, quote(v.Name)))
			if refused(err) {
				continue
			}
			if err != nil {
				return 0, err
			}
			count := map[string]int{}
			for _, h := range hits {
				if h.project != home {
					count[h.project]++
				}
			}
			for p, c := range count {
				issues := "issues"
				if c == 1 {
					issues = "issue"
				}
				found = append(found, Item{Project: p, Kind: n.kind, Ref: v.Name, Detail: fmt.Sprintf("%d %s with %s %s", c, issues, n.label, quote(v.Name))})
			}
		}
	}

	for _, p := range quoted {
		jql := fmt.Sprintf(`text ~ "\"%s\"" AND issuetype = %s AND key != %s`, p, quote(src.Fields.IssueType.Name), key)
		hits, err := search(ctx, client, jql)
		if refused(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		for _, h := range hits {
			if h.project != home {
				found = append(found, Item{Project: h.project, Kind: Text, Ref: h.key, Detail: fmt.Sprintf("%q: %s", p, h.summary)})
			}
		}
	}
	return s.suggest(profileID, caseID, found)
}

type hit struct{ key, project, summary string }

func search(ctx context.Context, client *corejira.Client, jql string) ([]hit, error) {
	page, err := client.SearchIssues(ctx, jql, []string{"project", "summary"}, nil, 0, searchCap)
	if err != nil {
		return nil, err
	}
	var out []hit
	for _, is := range page.Issues {
		var f struct {
			Project struct{ Key string } `json:"project"`
			Summary string               `json:"summary"`
		}
		b, _ := json.Marshal(is.Fields)
		// A hit without a project cannot be placed on a case, so it is left out.
		if json.Unmarshal(b, &f) == nil && f.Project.Key != "" {
			out = append(out, hit{is.Key, f.Project.Key, f.Summary})
		}
	}
	return out, nil
}

// refused is Jira saying no to one search, as it does for a name that exists
// in no project. Other failures, such as a rejected token, stop Find.
func refused(err error) bool {
	var he *corejira.HTTPError
	return errors.As(err, &he) && he.Code == http.StatusBadRequest
}

func readable(err error) string {
	if msg := corejira.Humanize(err); msg != "" {
		return msg
	}
	return err.Error()
}

// quote puts a value in JQL double quotes.
func quote(v string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`
}

// projectOf is everything before a key's last dash.
func projectOf(key string) string {
	if i := strings.LastIndex(key, "-"); i > 0 {
		return key[:i]
	}
	return ""
}
