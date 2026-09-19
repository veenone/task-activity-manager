package jira

import (
	"context"
	"net/url"
	"sort"
)

// EditMeta reads the fields one issue's edit screen carries. Jira answers it
// as the same map of MetaField that the classic create-meta call returns,
// keyed by field id, so nothing here models the payload a second time.
//
// Unlike create metadata there is no second endpoint to fall back on and no
// version where the answer lists fields the screen does not have: editmeta
// is the edit screen. A failed read is returned as an error rather than as
// an empty screen, because an empty screen would refuse every field.
func (c *Client) EditMeta(ctx context.Context, key string) (MetaFields, error) {
	var body struct {
		Fields map[string]MetaField `json:"fields"`
	}
	if err := c.Get(ctx, "/rest/api/2/issue/"+url.PathEscape(key)+"/editmeta", &body); err != nil {
		return nil, err
	}
	out := make(MetaFields, 0, len(body.Fields))
	for id, f := range body.Fields {
		f.ID = id
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
