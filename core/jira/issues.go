package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// RawIssue is one issue as Jira returns it: key, id, and the fields object
// with each value left as raw JSON for the caller to decode.
type RawIssue struct {
	ID     string                     `json:"id"`
	Key    string                     `json:"key"`
	Fields map[string]json.RawMessage `json:"fields"`
	// Changelog is set only when the search that returned this issue asked
	// to expand "changelog"; a search that did not leaves it at its zero
	// value, which is indistinguishable from a changelog with no history at
	// all, so a caller that needs to tell those apart has to know which
	// expand it asked for.
	Changelog RawChangelog `json:"changelog"`
}

// SearchPage is one page of a JQL search plus the total match count.
type SearchPage struct {
	Issues []RawIssue `json:"issues"`
	Total  int        `json:"total"`
}

// RawChangelog is the changelog Jira embeds in a search or get-issue
// response when the request expands "changelog": the page of histories
// that came back plus the total the issue's full changelog holds. Data
// Center caps how many histories one request answers with regardless of
// the issue search's own maxResults, so Total can exceed len(Histories)
// even on an issue with no more issues left to page through; that gap is
// what a caller checks to know a history is a partial one.
type RawChangelog struct {
	StartAt    int          `json:"startAt"`
	MaxResults int          `json:"maxResults"`
	Total      int          `json:"total"`
	Histories  []RawHistory `json:"histories"`
}

// RawHistory is one changelog entry: when it was made and every field it
// touched. Created is left as the wire's own string; it is not RFC 3339
// (see sprintdate's comment on the same wire quirk), so parsing it is a
// job for whoever needs the moment as a time.Time, not for this type.
type RawHistory struct {
	Created string           `json:"created"`
	Items   []RawHistoryItem `json:"items"`
}

// RawHistoryItem is one field change inside a history entry. FieldID is
// the stable id ("status", "customfield_10020") a caller should match on;
// Field is the display name Jira sends alongside it, the fallback for a
// custom field whose id discovery could not resolve. From/To and
// FromString/ToString are both kept because which pair a field populates
// depends on the field: most send the readable pair, some only the raw
// one.
type RawHistoryItem struct {
	Field      string `json:"field"`
	FieldID    string `json:"fieldId"`
	From       string `json:"from"`
	FromString string `json:"fromString"`
	To         string `json:"to"`
	ToString   string `json:"toString"`
}

// IssueType is one entry of a project's issue type list.
type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// ErrFieldNotFound is returned by CustomFieldID when the instance has no
// custom field with the requested name.
var ErrFieldNotFound = errors.New("jira: custom field not found")

// SearchIssues runs one page of /rest/api/2/search. fields names the fields
// to return, an empty list asking Jira for its default set; expand names
// what to expand beyond fields ("changelog" is the one this package uses),
// left unset by an empty list. url.Values.Encode sorts its keys, so a
// caller that always passes an empty expand can never move its own query
// string by this parameter existing: the key is simply never added.
func (c *Client) SearchIssues(ctx context.Context, jql string, fields, expand []string, startAt, maxResults int) (SearchPage, error) {
	q := url.Values{}
	q.Set("jql", jql)
	q.Set("startAt", strconv.Itoa(startAt))
	q.Set("maxResults", strconv.Itoa(maxResults))
	if len(fields) > 0 {
		q.Set("fields", strings.Join(fields, ","))
	}
	if len(expand) > 0 {
		q.Set("expand", strings.Join(expand, ","))
	}
	var page SearchPage
	if err := c.Get(ctx, "/rest/api/2/search?"+q.Encode(), &page); err != nil {
		return SearchPage{}, err
	}
	return page, nil
}

// GetIssue fetches one issue by key with the named fields.
func (c *Client) GetIssue(ctx context.Context, key string, fields []string) (RawIssue, error) {
	path := "/rest/api/2/issue/" + url.PathEscape(key)
	if len(fields) > 0 {
		path += "?fields=" + url.QueryEscape(strings.Join(fields, ","))
	}
	var iss RawIssue
	if err := c.Get(ctx, path, &iss); err != nil {
		return RawIssue{}, err
	}
	return iss, nil
}

// IssueTypes lists the issue types available in a project.
func (c *Client) IssueTypes(ctx context.Context, projectKey string) ([]IssueType, error) {
	var project struct {
		IssueTypes []IssueType `json:"issueTypes"`
	}
	if err := c.Get(ctx, "/rest/api/2/project/"+url.PathEscape(projectKey), &project); err != nil {
		return nil, err
	}
	return project.IssueTypes, nil
}

// CustomFieldID returns the customfield_NNNNN id for a custom field name,
// compared case-insensitively. The instance's field list is fetched once per
// client and cached, so resolving several names costs one request.
func (c *Client) CustomFieldID(ctx context.Context, name string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		return "", fmt.Errorf("jira: custom field name is empty")
	}
	c.fieldMu.Lock()
	loaded := c.fieldsLoaded
	c.fieldMu.Unlock()
	if !loaded {
		var fields []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Custom bool   `json:"custom"`
		}
		if err := c.Get(ctx, "/rest/api/2/field", &fields); err != nil {
			return "", err
		}
		ids := make(map[string]string, len(fields))
		for _, f := range fields {
			if f.Custom {
				ids[strings.ToLower(strings.TrimSpace(f.Name))] = f.ID
			}
		}
		c.fieldMu.Lock()
		c.fieldIDs = ids
		c.fieldsLoaded = true
		c.fieldMu.Unlock()
	}
	c.fieldMu.Lock()
	id, ok := c.fieldIDs[want]
	c.fieldMu.Unlock()
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrFieldNotFound, name)
	}
	return id, nil
}
