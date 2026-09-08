package jira

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ErrNoAgile is what Boards returns when the instance answers 404 on
// /rest/agile/1.0/board: a Jira Data Center without Jira Software has no
// Agile API at all, which is a fact about the instance, not a failure of the
// sync.
var ErrNoAgile = errors.New("jira: instance has no agile api")

// ErrNoSprints is what Sprints returns when a board's first page of sprints
// comes back 400: that is Jira's way of saying a kanban board has no
// sprints to page through.
var ErrNoSprints = errors.New("jira: board has no sprints")

// RawBoard is one entry of /rest/agile/1.0/board, transport only: Type is
// whatever string Jira sent, unfiltered by board type. Filtering by type is
// the caller's job.
type RawBoard struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	// Location is the board's own home. /board?projectKeyOrId= answers with
	// every board whose filter *mentions* the project, which includes boards
	// another team owns, so this is the only thing that says whose board it
	// is. An instance that does not send it leaves ProjectKey empty.
	Location struct {
		ProjectKey string `json:"projectKey"`
	} `json:"location"`
}

// RawSprint is one entry of /rest/agile/1.0/board/{id}/sprint, transport
// only.
type RawSprint struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// RawBoardConfig is /board/{id}/configuration. Jira nests the columns under
// columnConfig and gives each column a list of status objects, not a list of
// ids; flattening that here would mean this package quietly disagreeing with
// the API it exists to speak.
type RawBoardConfig struct {
	ID           int `json:"id"`
	ColumnConfig struct {
		Columns []RawColumn `json:"columns"`
	} `json:"columnConfig"`
}

// RawColumn is one column of a board's configuration, statuses left in
// Jira's nested shape: a list of status objects, not a list of ids.
type RawColumn struct {
	Name     string      `json:"name"`
	Statuses []RawStatus `json:"statuses"`
}

// RawStatus is one status entry under a column, transport only. Name is
// ignored by the board configuration (which the id alone is enough for) but
// is needed when the same shape shows up as a transition's target status.
type RawStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// StatusIDs is the flattening every caller wants, kept beside the raw shape
// rather than inside it.
func (c RawColumn) StatusIDs() []string {
	ids := make([]string, len(c.Statuses))
	for i, s := range c.Statuses {
		ids[i] = s.ID
	}
	return ids
}

// agilePage is the envelope the board and sprint collections return. The
// board issue endpoints do not use it; see agileIssuePage below. MaxResults
// and StartAt are decoded so the envelope is captured in full, but they are
// deliberately not used for paging: the loop counts start and page size
// locally instead.
type agilePage[T any] struct {
	Values     []T  `json:"values"`
	IsLast     bool `json:"isLast"`
	MaxResults int  `json:"maxResults"`
	StartAt    int  `json:"startAt"`
}

// agileIssuePage is the search envelope /board/{id}/issue and
// /board/{id}/sprint/{sprintId}/issue answer with: issues and total, not the
// values and isLast shape the board and sprint lists use.
type agileIssuePage struct {
	Issues []struct {
		Key string `json:"key"`
	} `json:"issues"`
	StartAt    int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total      int `json:"total"`
}

// pageAgile walks an Agile 1.0 collection to its end. Jira reports the end
// with isLast; a short page ends it too, for instances that omit the flag.
//
// onFirstPageErr, when non-nil, is given the error from the very first
// request and may translate it into a different error. It never sees an
// error from a later page: by the time paging has moved past start 0 the
// collection is known to exist, and a failure there is a real one, not the
// instance saying the collection is absent.
func pageAgile[T any](ctx context.Context, c *Client, path string, query url.Values, onFirstPageErr func(error) error) ([]T, error) {
	// Board and sprint lists are short, but the loop ends on isLast or an
	// empty page and advances by what came back, so a larger ask costs
	// nothing and saves a round trip on a project with many boards.
	const size = 200
	out := []T{}
	for start := 0; ; {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		q.Set("startAt", strconv.Itoa(start))
		q.Set("maxResults", strconv.Itoa(size))
		var page agilePage[T]
		if err := c.Get(ctx, path+"?"+q.Encode(), &page); err != nil {
			if start == 0 && onFirstPageErr != nil {
				return nil, onFirstPageErr(err)
			}
			return nil, err
		}
		out = append(out, page.Values...)
		// End on isLast or on an empty page, never on a short one: a Jira
		// that clamps maxResults below what we asked for answers every
		// page short, and treating that as the end would silently drop
		// every board after the first page.
		if page.IsLast || len(page.Values) == 0 {
			return out, nil
		}
		start += len(page.Values)
	}
}

// pageAgileIssues walks the search envelope that /board/{id}/issue and
// /board/{id}/sprint/{sprintId}/issue answer with, transport only: it hands
// back the issue keys and nothing else. It cannot reuse pageAgile, which
// expects the values/isLast envelope the board and sprint lists use;
// decoding this envelope into that shape would find no values, end the loop
// on the first request, and hand back zero keys with no error anywhere. It
// pages until startAt plus the issues just read reaches total, or until a
// page comes back empty, which guards against an instance that reports a
// total it will not serve.
func pageAgileIssues(ctx context.Context, c *Client, path, jql string) ([]string, error) {
	// A board's own issue list is every issue on the board, so this is the
	// one Agile collection that is routinely thousands long: at 50 a page a
	// 5,000-issue board cost 100 serial round trips, and a sync walks this
	// once per board plus once per open sprint. Asking for more is free
	// because the loop advances by what actually came back, so an instance
	// that clamps maxResults lower is handled by the same arithmetic.
	const size = 500
	out := []string{}
	for start := 0; ; {
		q := url.Values{}
		q.Set("fields", "key")
		if jql != "" {
			q.Set("jql", jql)
		}
		q.Set("startAt", strconv.Itoa(start))
		q.Set("maxResults", strconv.Itoa(size))
		var page agileIssuePage
		if err := c.Get(ctx, path+"?"+q.Encode(), &page); err != nil {
			return nil, err
		}
		for _, iss := range page.Issues {
			out = append(out, iss.Key)
		}
		n := len(page.Issues)
		if n == 0 || start+n >= page.Total {
			return out, nil
		}
		start += n
	}
}

// Boards lists a project's boards from /rest/agile/1.0/board, transport
// only. A 404 means the instance has no Agile API at all and is reported as
// ErrNoAgile; any other error, including one on a later page, is returned as
// is.
func (c *Client) Boards(ctx context.Context, projectKey string) ([]RawBoard, error) {
	q := url.Values{}
	q.Set("projectKeyOrId", projectKey)
	return pageAgile[RawBoard](ctx, c, "/rest/agile/1.0/board", q, func(err error) error {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.Code == http.StatusNotFound {
			return ErrNoAgile
		}
		return err
	})
}

// BoardConfiguration fetches /board/{id}/configuration, transport only: the
// column and status shape is decoded as Jira sends it, not flattened. Any
// error, a 403 included, is returned as is; deciding what to do with it is
// the caller's job.
func (c *Client) BoardConfiguration(ctx context.Context, boardID int) (RawBoardConfig, error) {
	var cfg RawBoardConfig
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", boardID)
	if err := c.Get(ctx, path, &cfg); err != nil {
		return RawBoardConfig{}, err
	}
	return cfg, nil
}

// Sprints lists a board's sprints from /board/{id}/sprint, transport only. A
// kanban board answers with a 400 on the first page, which is Jira saying
// the board has no sprints; that maps to ErrNoSprints. A 400 on a later page
// is a real failure and is returned as is, not swallowed as an empty list.
func (c *Client) Sprints(ctx context.Context, boardID int) ([]RawSprint, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint", boardID)
	return pageAgile[RawSprint](ctx, c, path, nil, func(err error) error {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.Code == http.StatusBadRequest {
			return ErrNoSprints
		}
		return err
	})
}

// BoardIssueKeys lists the keys of a board's issues, transport only. With an
// empty sprintID it reads /board/{id}/issue, the board's whole issue list;
// with a sprintID it reads /board/{id}/sprint/{sprintId}/issue instead. Both
// answer with the search envelope, not the values/isLast one the board and
// sprint lists use, and both are asked for fields=key since a key is all
// this call returns.
//
// projectKey narrows the read to one project through the jql parameter, which
// both endpoints AND with the board's own filter. A board's filter is not
// bounded by a project: one seen in the field holds 8,485 cards while the
// project being synced has 38, and reading the whole board to keep those 38
// took a minute of the sync on its own. Passing "" reads the board entire.
func (c *Client) BoardIssueKeys(ctx context.Context, boardID int, sprintID, projectKey string) ([]string, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/issue", boardID)
	if sprintID != "" {
		path = fmt.Sprintf("/rest/agile/1.0/board/%d/sprint/%s/issue", boardID, url.PathEscape(sprintID))
	}
	jql := ""
	if p := strings.TrimSpace(projectKey); p != "" {
		jql = "project = " + strconv.Quote(p)
	}
	return pageAgileIssues(ctx, c, path, jql)
}

// RankIssue ranks key immediately before or after neighbourKey via PUT
// /rest/agile/1.0/issue/rank, one of the endpoint-specific write calls in
// this package. The endpoint answers 204 when the rank landed and 207
// Multi-Status when it did not, so bulkWrite fails on a 207 whatever body
// came with it.
func (c *Client) RankIssue(ctx context.Context, key, neighbourKey string, before bool) error {
	body := map[string]any{"issues": []string{key}}
	if before {
		body["rankBeforeIssue"] = neighbourKey
	} else {
		body["rankAfterIssue"] = neighbourKey
	}
	return c.bulkWrite(ctx, "rank issue", http.MethodPut, "/rest/agile/1.0/issue/rank", body, []string{key})
}

// MoveToSprint moves keys onto sprintID via POST
// /rest/agile/1.0/sprint/{sprintId}/issue, one of the endpoint-specific
// write calls in this package. The sprint id is path-escaped. Same 207
// handling as RankIssue.
func (c *Client) MoveToSprint(ctx context.Context, sprintID string, keys []string) error {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%s/issue", url.PathEscape(sprintID))
	return c.bulkWrite(ctx, "move to sprint", http.MethodPost, path, map[string]any{"issues": keys}, keys)
}

// MoveToBacklog moves keys off any sprint and onto the backlog via POST
// /rest/agile/1.0/backlog/issue, one of the endpoint-specific write calls in
// this package. Same 207 handling as RankIssue.
func (c *Client) MoveToBacklog(ctx context.Context, keys []string) error {
	return c.bulkWrite(ctx, "move to backlog", http.MethodPost, "/rest/agile/1.0/backlog/issue", map[string]any{"issues": keys}, keys)
}
