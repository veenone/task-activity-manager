package jira

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// errNoDemoJQL is the demo profile's answer: its generator serves fixed
// data and has no JQL engine to run the analyst's query.
var errNoDemoJQL = errors.New("the demo profile has no Jira to run JQL against")

// KeysMatching returns the keys that also match jql, in key order. Each chunk
// of keys is one `key in (...) AND (jql)` search, so a Test Execution's
// tests can be narrowed the way Jira's own Test Execution page narrows them
// (#446). A refused query returns Jira's message, because a mistyped field
// name is the analyst's to fix.
func (c *Client) KeysMatching(ctx context.Context, keys []string, jql string) ([]string, error) {
	if isDemoURL(c.baseURL) {
		return nil, errNoDemoJQL
	}
	jql = withoutOrderBy(jql)
	out := []string{}
	for _, chunk := range chunkKeys(keys, bugSearchKeyChunk) {
		quoted := make([]string, len(chunk))
		for i, k := range chunk {
			quoted[i] = `"` + strings.TrimSpace(k) + `"`
		}
		q := "key in (" + strings.Join(quoted, ", ") + ") AND (" + jql + ")"
		for startAt := 0; ; {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			v := url.Values{}
			v.Set("jql", q)
			v.Set("startAt", strconv.Itoa(startAt))
			v.Set("maxResults", "100")
			v.Set("fields", "key")
			var resp struct {
				Total  int `json:"total"`
				Issues []struct {
					Key string `json:"key"`
				} `json:"issues"`
			}
			if err := c.get(ctx, "/rest/api/2/search?"+v.Encode(), &resp); err != nil {
				return nil, err
			}
			for _, is := range resp.Issues {
				out = append(out, is.Key)
			}
			startAt += len(resp.Issues)
			if len(resp.Issues) == 0 || startAt >= resp.Total {
				break
			}
		}
	}
	return out, nil
}

// withoutOrderBy drops an ORDER BY clause outside quotes. Jira refuses one
// inside the parentheses KeysMatching wraps the query in, a saved filter's JQL
// often ends with one, and the order means nothing to a set of keys.
func withoutOrderBy(jql string) string {
	lower := strings.ToLower(jql)
	cut := -1
	var quote byte
	for i := 0; i < len(jql); i++ {
		switch c := jql[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case strings.HasPrefix(lower[i:], "order by"):
			cut = i
		}
	}
	if cut >= 0 {
		jql = jql[:cut]
	}
	return strings.TrimSpace(jql)
}
