package jira

import (
	"context"
	"net/url"
	"strconv"
)

// RawDashboard is one Jira dashboard as /rest/api/2/dashboard answers it:
// the name a reader recognises and the URL that opens it.
//
// Nothing else is kept because nothing else is reachable. Reading the
// gadgets on a dashboard needs /dashboard/{id}/gadget, which is Cloud only,
// and the property routes that would read a gadget's filter need an item id
// only that route hands out. See agents/project/out-of-scope.md.
type RawDashboard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// View is the URL of the dashboard's own page, built by Jira from the
	// base URL the instance is configured with. That is the address Jira's
	// own mail uses, so it is handed on as given rather than rebuilt from
	// the address TAM connects to.
	View string `json:"view"`
}

// dashboardPage is the envelope /dashboard answers with. Total and
// MaxResults are both read: the list pages, and Data Center clamps the page
// size to its own limit whatever is asked for.
type dashboardPage struct {
	StartAt    int            `json:"startAt"`
	MaxResults int            `json:"maxResults"`
	Total      int            `json:"total"`
	Dashboards []RawDashboard `json:"dashboards"`
}

// Dashboards is every Jira dashboard this user can see, paged to the end.
//
// The route documents startAt as having to be zero or a multiple of
// maxResults, so the loop advances by what the page actually held, which on
// a full page is the page size the server used. A short page is the last one
// the server will serve, so it ends the walk even when the total claims more:
// an instance reporting a total it will not answer for must not spin here.
func (c *Client) Dashboards(ctx context.Context) ([]RawDashboard, error) {
	// Asking for more than Jira will give costs nothing, because the loop
	// reads the page size back out of the answer.
	const size = 1000
	out := []RawDashboard{}
	for start := 0; ; {
		q := url.Values{}
		q.Set("startAt", strconv.Itoa(start))
		q.Set("maxResults", strconv.Itoa(size))
		var page dashboardPage
		if err := c.Get(ctx, "/rest/api/2/dashboard?"+q.Encode(), &page); err != nil {
			return nil, err
		}
		out = append(out, page.Dashboards...)
		n := len(page.Dashboards)
		if n == 0 || start+n >= page.Total || n < page.MaxResults {
			return out, nil
		}
		start += n
	}
}
