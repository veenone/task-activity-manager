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
// Center caps how many histories one request answers with, per issue,
// regardless of the issue search's own maxResults or how many pages of
// issues remain, so Total can exceed len(Histories) even on the search's
// very last page; that gap is what a caller checks to know one issue's
// history came back partial.
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
// one. A reader that wants one value per side, such as
// tam/internal/backend/jira/history.go, falls back to From/To whenever
// FromString/ToString came back empty rather than trusting the string pair
// alone.
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

// ErrFieldAmbiguous is returned by CustomFieldID when the instance has more
// than one custom field with the requested name. Data Center collects these:
// a legacy "Story Points" beside the Agile one is the common pair. A name
// that identifies two fields identifies neither, and picking one would put
// every read and every write on a field nobody chose.
var ErrFieldAmbiguous = errors.New("jira: more than one custom field has that name")

// FieldName is the name the instance gives a field id, for turning an error
// that names ids into one that names fields. It reads the same cached field
// list CustomFieldID loads, so the first of the two to be called pays for
// both. An id the instance does not list, or a list that cannot be read,
// gives "": the caller shows the id rather than inventing a name.
func (c *Client) FieldName(ctx context.Context, id string) string {
	if err := c.loadFields(ctx); err != nil {
		return ""
	}
	c.fieldMu.Lock()
	defer c.fieldMu.Unlock()
	return c.fieldNames[id]
}

// SearchIssues runs one page of /rest/api/2/search. fields names the fields
// to return, an empty list asking Jira for its default set; expand names
// what to expand beyond fields ("changelog" is the one this package uses),
// left unset by an empty list: a caller that always passes an empty expand
// never adds the "expand" parameter to the query at all, so its own query
// string can never be affected by this parameter existing.
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

// loadFields fetches the instance's field list once per client and keeps it
// both ways round: custom field ids by lowercased name, and every field's
// name by id. The name side holds every id that answers to a name, because
// Data Center lets two custom fields share one; collapsing them here is how
// the wrong "Story Points" would be picked and never questioned. Names are kept for system fields too, because an error names
// whatever id it likes and "duedate" needs a name as much as a custom one.
func (c *Client) loadFields(ctx context.Context) error {
	c.fieldMu.Lock()
	loaded := c.fieldsLoaded
	c.fieldMu.Unlock()
	if loaded {
		return nil
	}
	var fields []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Custom bool   `json:"custom"`
	}
	if err := c.Get(ctx, "/rest/api/2/field", &fields); err != nil {
		return err
	}
	ids := make(map[string][]string, len(fields))
	names := make(map[string]string, len(fields))
	for _, f := range fields {
		if f.Custom {
			key := strings.ToLower(strings.TrimSpace(f.Name))
			ids[key] = append(ids[key], f.ID)
		}
		names[f.ID] = strings.TrimSpace(f.Name)
	}
	c.fieldMu.Lock()
	c.fieldIDs = ids
	c.fieldNames = names
	c.fieldsLoaded = true
	c.fieldMu.Unlock()
	return nil
}

// CustomFieldID returns the customfield_NNNNN id for a custom field name,
// compared case-insensitively. The instance's field list is fetched once per
// client and cached, so resolving several names costs one request.
func (c *Client) CustomFieldID(ctx context.Context, name string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		return "", fmt.Errorf("jira: custom field name is empty")
	}
	if err := c.loadFields(ctx); err != nil {
		return "", err
	}
	c.fieldMu.Lock()
	found := c.fieldIDs[want]
	c.fieldMu.Unlock()
	switch len(found) {
	case 0:
		return "", fmt.Errorf("%w: %q", ErrFieldNotFound, name)
	case 1:
		return found[0], nil
	}
	return "", fmt.Errorf("%w: %q is %s", ErrFieldAmbiguous, name, strings.Join(found, " and "))
}
