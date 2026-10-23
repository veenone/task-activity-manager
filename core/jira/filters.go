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
	// Owner is who wrote the filter, and only their username is kept: it
	// is the same value /rest/api/2/myself answers with, so comparing the
	// two is how a caller tells a filter this user owns from one somebody
	// else shared with them.
	//
	// An instance that names its users another way, Cloud by account id
	// among them, leaves this empty. Empty is a real answer meaning the
	// owner could not be read, and it must never be compared against
	// another empty one as though they matched.
	Owner struct {
		Name string `json:"name"`
	} `json:"owner"`
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
