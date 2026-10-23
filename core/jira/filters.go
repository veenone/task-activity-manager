package jira

import (
	"context"
	"errors"
	"net/http"
)

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

// ErrNoOwnedFilters is a Jira with no /filter/my route. The favourites are
// still readable on such an instance, so a caller reports this by leaving
// the scope off its filters rather than by failing: a picker that lists
// the starred filters unlabelled is what this view has always done, and
// strictly better than an error where a list used to be.
var ErrNoOwnedFilters = errors.New("jira: this instance cannot list the filters you own")

// OwnedFilters is the saved filters this user owns, starred or not.
//
// It is the other half of FavouriteFilters, and the reason both are read:
// Jira does not star a filter because you created it, so the favourites
// leave out everything a person wrote and never pinned. Which list a
// filter came from is also the only scope signal needed. Being in this one
// means it is yours, which saves asking Jira who the connected user is and
// comparing owners.
//
// A 404 becomes ErrNoOwnedFilters, the same soft failure Boards makes of a
// missing Agile API. Any other error is returned as it stands.
func (c *Client) OwnedFilters(ctx context.Context) ([]RawFilter, error) {
	out := []RawFilter{}
	if err := c.Get(ctx, "/rest/api/2/filter/my", &out); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.Code == http.StatusNotFound {
			return nil, ErrNoOwnedFilters
		}
		return nil, err
	}
	return out, nil
}
