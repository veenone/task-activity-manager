package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// transitionsEnvelope is the body /rest/api/2/issue/{key}/transitions
// answers with.
type transitionsEnvelope struct {
	Transitions []RawTransition `json:"transitions"`
}

// RawTransition is one entry of /rest/api/2/issue/{key}/transitions,
// transport only. Fields is keyed by field id exactly as Jira sends it, so a
// caller checking what a transition demands has both the id to fill in and
// the name to put in an error message: without the name, a caller can only
// say "customfield_11400 is required".
type RawTransition struct {
	ID     string                        `json:"id"`
	Name   string                        `json:"name"`
	To     RawStatus                     `json:"to"`
	Fields map[string]RawTransitionField `json:"fields"`
}

// RawTransitionField is one entry of a transition's Fields map, transport
// only.
type RawTransitionField struct {
	Name          string            `json:"name"`
	Required      bool              `json:"required"`
	AllowedValues []RawAllowedValue `json:"allowedValues"`
}

// RawAllowedValue is one entry of a transition field's allowedValues,
// transport only: an id to send back on the transition and a name to show.
type RawAllowedValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// transitionRequest is the body DoTransition sends: the transition id
// always, fields only when the caller supplies any.
type transitionRequest struct {
	Transition transitionRef  `json:"transition"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// transitionRef names the transition a transitionRequest fires.
type transitionRef struct {
	ID string `json:"id"`
}

// Transitions lists the transitions available on key from
// /rest/api/2/issue/{key}/transitions, expanded with transitions.fields so a
// caller can see what each transition demands before pushing it. The key is
// path-escaped.
func (c *Client) Transitions(ctx context.Context, key string) ([]RawTransition, error) {
	path := fmt.Sprintf("/rest/api/2/issue/%s/transitions?expand=transitions.fields", url.PathEscape(key))
	var env transitionsEnvelope
	if err := c.Get(ctx, path, &env); err != nil {
		return nil, err
	}
	return env.Transitions, nil
}

// DoTransition fires transitionID on key via POST
// /rest/api/2/issue/{key}/transitions, one of the endpoint-specific write
// calls in this package. fields fills the transition's resolution screen
// and any other fields Transitions reported as required, keyed by field id;
// a nil or empty map is left out of the request body entirely. Most Data
// Center workflows put a resolution screen on the transition into Done, and
// some of them reject a request that always sends "fields" as an empty
// object, so a caller with nothing to fill has to be able to send no fields
// key at all. The key is path-escaped.
func (c *Client) DoTransition(ctx context.Context, key, transitionID string, fields map[string]any) error {
	path := fmt.Sprintf("/rest/api/2/issue/%s/transitions", url.PathEscape(key))
	body := transitionRequest{Transition: transitionRef{ID: transitionID}, Fields: fields}
	return c.WriteJSON(ctx, http.MethodPost, path, body)
}
