package jira

import "context"

// RawFilter is one saved filter as Jira answers it: the id a dashboard is
// pinned to, the name its owner gave it, and the JQL it stands for.
//
// The JQL is kept beside the id rather than resolved through
// /filter/{id}/search on every read: a dashboard has to say what it counts
// even while Jira is unreachable, and a filter's own words are the only
// honest answer to that.
type RawFilter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	JQL  string `json:"jql"`
}

// FavouriteFilters is the saved filters this user has starred. A person
// with none gets an empty list, which is a fact about their account rather
// than a failure to read it.
func (c *Client) FavouriteFilters(ctx context.Context) ([]RawFilter, error) {
	out := []RawFilter{}
	if err := c.Get(ctx, "/rest/api/2/filter/favourite", &out); err != nil {
		return nil, err
	}
	return out, nil
}
