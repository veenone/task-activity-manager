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
}

// SearchPage is one page of a JQL search plus the total match count.
type SearchPage struct {
	Issues []RawIssue `json:"issues"`
	Total  int        `json:"total"`
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
// to return; an empty list asks Jira for its default set.
func (c *Client) SearchIssues(ctx context.Context, jql string, fields []string, startAt, maxResults int) (SearchPage, error) {
	q := url.Values{}
	q.Set("jql", jql)
	q.Set("startAt", strconv.Itoa(startAt))
	q.Set("maxResults", strconv.Itoa(maxResults))
	if len(fields) > 0 {
		q.Set("fields", strings.Join(fields, ","))
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
