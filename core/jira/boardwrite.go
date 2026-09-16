package jira

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// CreateFilter creates a saved filter via POST /rest/api/2/filter, shared
// with the project by default so the board it goes on to back is visible to
// the whole team rather than only its creator. description is sent only
// when non-empty, the same omit-when-blank rule CreateSprint's goal follows.
//
// Template: CreateSprint (sprintwrite.go) — a map[string]any body through
// WriteJSONReturning, whose error shaping (non-2xx -> writeStatusError)
// already lives there. The id Jira assigns is asserted non-empty the way
// CreateIssue asserts its key, since a 2xx with nothing usable in the body
// is not a create TAM can build a board on.
func (c *Client) CreateFilter(ctx context.Context, name, jql, description, projectID string) (string, error) {
	body := map[string]any{
		"name": name,
		"jql":  jql,
		"sharePermissions": []map[string]any{
			{"type": "project", "project": map[string]any{"id": projectID}},
		},
	}
	if description != "" {
		body["description"] = description
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.WriteJSONReturning(ctx, http.MethodPost, "/rest/api/2/filter", body, &resp); err != nil {
		return "", err
	}
	if resp.ID == "" {
		return "", errors.New("jira: created the filter but returned no id")
	}
	return resp.ID, nil
}

// DeleteFilter deletes filterID via DELETE /rest/api/2/filter/{id}. Any 2xx
// is success; Client.Delete already shapes a non-2xx to name the method,
// path and Jira's own response body.
func (c *Client) DeleteFilter(ctx context.Context, filterID string) error {
	return c.Delete(ctx, "/rest/api/2/filter/"+url.PathEscape(filterID))
}

// CreateBoard creates a board via POST /rest/agile/1.0/board, on the filter
// and located at the project the caller names. Same template and same
// non-empty id assertion as CreateFilter.
func (c *Client) CreateBoard(ctx context.Context, name, boardType, filterID, projectKey string) (int, error) {
	body := map[string]any{
		"name": name,
		"type": boardType,
		"location": map[string]any{
			"type":           "project",
			"projectKeyOrId": projectKey,
		},
	}
	// filterId travels as a number on the wire (Jira's own agile board create
	// takes it that way); the id CreateFilter hands back is the classic
	// filter API's own string, which is always digits, so this reparses
	// rather than asking every caller to carry two representations of one id.
	if n, err := strconv.Atoi(filterID); err == nil {
		body["filterId"] = n
	} else {
		body["filterId"] = filterID
	}
	var resp struct {
		ID int `json:"id"`
	}
	if err := c.WriteJSONReturning(ctx, http.MethodPost, "/rest/agile/1.0/board", body, &resp); err != nil {
		return 0, err
	}
	if resp.ID == 0 {
		return 0, errors.New("jira: created the board but returned no id")
	}
	return resp.ID, nil
}

// backlogBatch is how many issues one add-to-backlog request carries.
// Batching keeps one bad key from taking a whole planning session's worth
// of adds down together, the same reason committer's own sprint-move pass
// batches at this width.
const backlogBatch = 20

// AddToBoardBacklog adds keys to boardID's backlog via POST
// /rest/agile/1.0/backlog/{boardId}/issue, in batches of backlogBatch. It
// goes through the shared bulkWrite, which already treats a 207
// Multi-Status as a failure of the whole batch it came from; a batch after
// the first is never sent once an earlier one has failed.
func (c *Client) AddToBoardBacklog(ctx context.Context, boardID int, keys []string) error {
	path := fmt.Sprintf("/rest/agile/1.0/backlog/%d/issue", boardID)
	for start := 0; start < len(keys); start += backlogBatch {
		end := start + backlogBatch
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[start:end]
		if err := c.bulkWrite(ctx, "add to board backlog", http.MethodPost, path, map[string]any{"issues": batch}, batch); err != nil {
			return err
		}
	}
	return nil
}
