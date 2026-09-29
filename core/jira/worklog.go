package jira

import (
	"context"
	"net/url"
	"strconv"
)

// RawWorklog is one worklog entry as Jira answers it. Started is left as the
// wire's own string, offset and all: Jira dates the entry by that offset, so
// the zone is part of the value rather than noise a parse can drop. TimeSpent
// is the phrase a person typed ("2h 30m"); TimeSpentSeconds is what Jira made
// of it, which is the only one a total can be added up from.
type RawWorklog struct {
	ID     string `json:"id"`
	Author struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Started          string `json:"started"`
	TimeSpent        string `json:"timeSpent"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
	Comment          string `json:"comment"`
}

// worklogPage is the envelope /issue/{key}/worklog answers with: the search
// shape of startAt and total rather than the Agile isLast one.
type worklogPage struct {
	StartAt    int          `json:"startAt"`
	MaxResults int          `json:"maxResults"`
	Total      int          `json:"total"`
	Worklogs   []RawWorklog `json:"worklogs"`
}

// Worklogs walks an issue's worklog to its end. It is its own call rather
// than fields=worklog on a search because the search caps an issue's
// worklogs at 20 and reports nothing about the rest, so an issue worked on
// for a month would read as one worked on for a week.
//
// The loop ends when startAt plus what came back reaches the total, or on an
// empty page, which is the guard pageAgileIssues carries for the same reason:
// an instance that reports a total it will not serve would otherwise be asked
// for the missing rows for ever.
func (c *Client) Worklogs(ctx context.Context, key string) ([]RawWorklog, error) {
	const size = 200
	path := "/rest/api/2/issue/" + url.PathEscape(key) + "/worklog"
	out := []RawWorklog{}
	for start := 0; ; {
		q := url.Values{}
		q.Set("startAt", strconv.Itoa(start))
		q.Set("maxResults", strconv.Itoa(size))
		var page worklogPage
		if err := c.Get(ctx, path+"?"+q.Encode(), &page); err != nil {
			return nil, err
		}
		out = append(out, page.Worklogs...)
		n := len(page.Worklogs)
		if n == 0 || start+n >= page.Total {
			return out, nil
		}
		start += n
	}
}

// AddWorklog logs work against an issue. started carries its own UTC offset
// ("2026-09-29T01:00:00.000+0700") and is sent exactly as given: Jira files
// the entry under the day that offset makes it, and sending the same instant
// as UTC moves it to the day before for anyone east of Greenwich. timeSpent
// is Jira's own duration phrasing, which Jira parses and refuses with a 400
// when it cannot. An empty comment is left out of the body rather than sent
// as "", which would write an empty comment onto the entry.
func (c *Client) AddWorklog(ctx context.Context, key, started, timeSpent, comment string) error {
	body := map[string]any{"started": started, "timeSpent": timeSpent}
	if comment != "" {
		body["comment"] = comment
	}
	return c.Post(ctx, "/rest/api/2/issue/"+url.PathEscape(key)+"/worklog", body)
}
