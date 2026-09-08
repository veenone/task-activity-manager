package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// bulkWrite sends one Agile bulk write and decides whether it landed. Jira
// answers 204 when every issue in the request was moved or ranked and 207
// Multi-Status when at least one was not, so a 207 is a failure here
// whatever its body says: success is decided by the status, never by
// recognising a body schema. The body is read only to say why, and op and
// path go into the message because the failure ends up in a journal row
// someone has to act on.
//
// keys are the issues the call was made with. They are named in the message
// because the rank endpoint reports its rejections by numeric issue id,
// which a caller holding issue keys cannot match on its own.
func (c *Client) bulkWrite(ctx context.Context, op, method, path string, body any, keys []string) error {
	resp, err := c.WriteJSONRaw(ctx, method, path, body)
	if err != nil {
		return fmt.Errorf("jira: %s: %w", op, err)
	}
	switch {
	case resp.Code == http.StatusMultiStatus:
		return fmt.Errorf(
			"jira: %s refused: %s %s -> %s for %s: %s",
			op, method, path, resp.Status, strings.Join(keys, ", "), multiStatusReason(resp.Body),
		)
	case resp.Code >= 300:
		return fmt.Errorf("jira: %s: %w", op, writeStatusError(method, path, resp))
	}
	return nil
}

// multiStatusEntries is the shape Atlassian documents for a partial failure
// on the rank endpoint: an entries array, each entry carrying the numeric
// issue id, the status Jira gave that issue, and its reasons.
type multiStatusEntries struct {
	Entries []struct {
		IssueID       json.Number `json:"issueId"`
		Status        int         `json:"status"`
		Errors        []string    `json:"errors"`
		ErrorMessages []string    `json:"errorMessages"`
	} `json:"entries"`
}

// multiStatusReason reads a 207 body for something worth putting in the
// error message: the documented entries array, then the errors object and
// errorMessages shapes jiraErrorMessage already knows, then a slice of the
// raw body when none of them parses. It never returns an empty string, so a
// body nobody anticipated still leaves the journal row something a person
// can act on.
func multiStatusReason(body []byte) string {
	if s := entriesReason(body); s != "" {
		return s
	}
	if s := jiraErrorMessage(body); s != "" {
		return s
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return "no response body"
	}
	return "unrecognised response body: " + snippet(body, 512)
}

// entriesReason renders the documented entries array, keeping the entries
// that report a failure and falling back to all of them when none does.
func entriesReason(body []byte) string {
	var decoded multiStatusEntries
	if err := json.Unmarshal(body, &decoded); err != nil || len(decoded.Entries) == 0 {
		return ""
	}
	parts := []string{}
	for _, e := range decoded.Entries {
		reasons := append(append([]string{}, e.Errors...), e.ErrorMessages...)
		if e.Status < 300 && len(reasons) == 0 {
			continue
		}
		part := fmt.Sprintf("issue id %s", e.IssueID.String())
		if e.Status != 0 {
			part += fmt.Sprintf(" (status %d)", e.Status)
		}
		if len(reasons) > 0 {
			part += ": " + strings.Join(reasons, "; ")
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d entries, none of them reporting a failure", len(decoded.Entries))
	}
	return strings.Join(parts, "; ")
}
