package jira

import "context"

// Project is one project the reader can see, by key and name.
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Projects is every project the reader can see. Data Center answers the whole
// list in one response, unpaged, and an account that sees none gets an empty
// list rather than an error.
func (c *Client) Projects(ctx context.Context) ([]Project, error) {
	out := []Project{}
	if err := c.Get(ctx, "/rest/api/2/project", &out); err != nil {
		return nil, err
	}
	return out, nil
}
