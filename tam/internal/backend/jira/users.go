package jira

import (
	"context"
	"net/url"
	"strconv"

	"agile-suite/tam/internal/backend"
)

// userSearchLimit is how many people one search returns. Enough to pick from
// without turning a two-letter query into a page of hundreds.
const userSearchLimit = 30

// SearchUsers asks Jira who can be assigned an issue in this project. The
// assignable endpoint is the right one: Jira's plain user search answers with
// people who have no permission on the project, and picking one of those
// fails at Commit rather than here.
//
// A blank query is what seeds the cache, and Jira DC's assignable search
// rejects an empty username on some versions, so it goes out as the wildcard
// the endpoint documents.
func (b *Backend) SearchUsers(ctx context.Context, projectKey, query string) ([]backend.User, error) {
	if query == "" {
		query = "%"
	}
	q := url.Values{}
	q.Set("project", projectKey)
	q.Set("username", query)
	q.Set("maxResults", strconv.Itoa(userSearchLimit))
	var raw []struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Active      bool   `json:"active"`
	}
	if err := b.c.Get(ctx, "/rest/api/2/user/assignable/search?"+q.Encode(), &raw); err != nil {
		return nil, err
	}
	out := make([]backend.User, 0, len(raw))
	for _, u := range raw {
		// A deactivated account can still be searched but cannot hold an
		// issue, so offering it would only produce a failed Commit.
		if !u.Active || u.Name == "" {
			continue
		}
		name := u.DisplayName
		if name == "" {
			name = u.Name
		}
		out = append(out, backend.User{Name: u.Name, DisplayName: name})
	}
	return out, nil
}

// Priorities lists the instance's priorities in Jira's own order, which is
// highest first, so a form's first option is not a random one.
func (b *Backend) Priorities(ctx context.Context) ([]string, error) {
	var raw []struct {
		Name string `json:"name"`
	}
	if err := b.c.Get(ctx, "/rest/api/2/priority", &raw); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if p.Name != "" {
			out = append(out, p.Name)
		}
	}
	return out, nil
}
