package jira

import (
	"context"
	"fmt"
	"net/http"
)

// CreateSprint creates a sprint on boardID via POST /rest/agile/1.0/sprint,
// sending originBoardId, name, startDate and endDate unconditionally and
// goal only when it is not empty. start and end must already be in the
// Agile API's own datetime format; see StartSprint's comment for the same
// requirement on those two calls.
//
// The response decodes into RawSprint. An empty response body, or one that
// decodes with a zero id, is not treated as an error: Jira DC's own answer
// shape for this endpoint is not worth failing a create over when the
// create itself landed, so the zero value comes back and the caller is
// expected to refresh its sprint list rather than trust this call's own
// echo of what it just created. A body that is not JSON at all, which is
// what a proxy answering 200 with an HTML page produces, is still an error:
// that is not a shape difference, it is evidence the request did not reach
// Jira, and the caller should hear about it.
//
// Creating a sprint is one of the writes in TAM that reach Jira outside a
// Commit, alongside StartSprint and CompleteSprint above. See the comment on
// UpdateSprint for the reason, which is the same for all of them.
func (c *Client) CreateSprint(ctx context.Context, boardID int, name, goal, start, end string) (RawSprint, error) {
	body := map[string]any{
		"originBoardId": boardID,
		"name":          name,
		"startDate":     start,
		"endDate":       end,
	}
	if goal != "" {
		body["goal"] = goal
	}
	var raw RawSprint
	if err := c.WriteJSONReturning(ctx, http.MethodPost, "/rest/agile/1.0/sprint", body, &raw); err != nil {
		return RawSprint{}, err
	}
	return raw, nil
}

// UpdateSprint edits sprintID via POST /rest/agile/1.0/sprint/{sprintId},
// touching only the keys the caller supplies. name, start and end are
// included when non-empty for the same partial-update reason StartSprint's
// comment gives: this endpoint overwrites a key that is present and leaves
// alone a key that is absent, so sending "" for a field the caller left
// untouched would erase it. state is never sent; a rename or a date change
// silently closing or reopening the sprint is not something any caller of
// this method is asking for.
//
// clearGoal is the deliberate exception to that omit-when-empty rule: when
// true, goal: "" goes into the body on purpose, because otherwise a goal
// set from TAM could never be cleared, only ever overwritten by another
// non-empty one, and TAM has no other way to tell Jira "take the goal away".
//
// Editing a sprint is one of the writes in TAM that reach Jira outside a
// Commit, and this is where the reason for all of them is written down,
// because it is easy to reach for a wrong one that sounds better.
//
// The wrong reason is that a sprint is somehow more of a shared Jira object
// than an issue is. It is not. A new issue is every bit as much a thing a
// whole team plans around, and TAM journals it behind a TAM-NEW-n placeholder
// and pushes it on Commit like everything else.
//
// The real reason is that TAM's journal is issue machinery. A pending change
// is keyed by issue key, a conflict is decided by comparing an issue's
// updated stamp, and Commit walks issues. A sprint has none of that: no
// cached version to rebase on, no conflict card, no rekey path for an id
// Jira has not handed out yet. Journaling these three writes would mean
// building a second journal for a second kind of entity, with its own
// placeholder ids for create, its own conflict story for edit, and a queue
// holding a destructive intent for delete. That is a cost, not a principle,
// and the cost is not worth paying for three calls a user makes a handful of
// times per sprint.
func (c *Client) UpdateSprint(ctx context.Context, sprintID int, name, goal, start, end string, clearGoal bool) error {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID)
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if start != "" {
		body["startDate"] = start
	}
	if end != "" {
		body["endDate"] = end
	}
	switch {
	case goal != "":
		body["goal"] = goal
	case clearGoal:
		body["goal"] = ""
	}
	return c.WriteJSON(ctx, http.MethodPost, path, body)
}

// DeleteSprint deletes sprintID via DELETE /rest/agile/1.0/sprint/{sprintId}.
// The delete carries no body, which Client.Delete already covers without
// this package adding anything new: any 2xx is success, and a non-2xx comes
// back naming the method, the path with the sprint id in it, and Jira's own
// response body, so a 404 on an already-gone sprint names exactly which one.
//
// Deleting a sprint is one of the writes in TAM that reach Jira outside
// a Commit, for the reason written on UpdateSprint above.
func (c *Client) DeleteSprint(ctx context.Context, sprintID int) error {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID)
	return c.Delete(ctx, path)
}
