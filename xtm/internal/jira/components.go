package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Component is one Jira project component as the Components view shows it.
type Component struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	LeadName        string `json:"leadName"`
	LeadDisplayName string `json:"leadDisplayName"`
	AssigneeType    string `json:"assigneeType"`
}

// ComponentInput is what create and update send. Project is used on create
// only; Jira does not move a component between projects.
type ComponentInput struct {
	Project      string
	Name         string
	Description  string
	LeadUserName string
	AssigneeType string
}

// componentWire is Jira's component JSON. The lead arrives nested.
type componentWire struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	AssigneeType string `json:"assigneeType"`
	Lead         *struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"lead"`
}

func (w componentWire) component() Component {
	out := Component{ID: w.ID, Name: w.Name, Description: w.Description, AssigneeType: w.AssigneeType}
	if w.Lead != nil {
		out.LeadName, out.LeadDisplayName = w.Lead.Name, w.Lead.DisplayName
	}
	return out
}

// ProjectComponentDetails lists a project's components with their ids, which
// the write calls need, and their description, lead and assignee type.
func (c *Client) ProjectComponentDetails(ctx context.Context, projectKey string) ([]Component, error) {
	var items []componentWire
	path := fmt.Sprintf("/rest/api/2/project/%s/components", url.PathEscape(projectKey))
	if err := c.get(ctx, path, &items); err != nil {
		return nil, fmt.Errorf("project components %s: %w", projectKey, err)
	}
	out := make([]Component, 0, len(items))
	for _, it := range items {
		out = append(out, it.component())
	}
	return out, nil
}

// CreateComponent creates a component in in.Project.
func (c *Client) CreateComponent(ctx context.Context, in ComponentInput) (Component, error) {
	body := map[string]string{
		"project": in.Project, "name": in.Name, "description": in.Description,
		"leadUserName": in.LeadUserName, "assigneeType": in.AssigneeType,
	}
	var out componentWire
	if err := c.componentWrite(ctx, http.MethodPost, "/rest/api/2/component", body, &out); err != nil {
		return Component{}, fmt.Errorf("create component %q: %w", in.Name, err)
	}
	return out.component(), nil
}

// UpdateComponent replaces a component's name, description, lead and assignee
// type. An empty LeadUserName clears the lead.
func (c *Client) UpdateComponent(ctx context.Context, id string, in ComponentInput) (Component, error) {
	body := map[string]string{
		"name": in.Name, "description": in.Description,
		"leadUserName": in.LeadUserName, "assigneeType": in.AssigneeType,
	}
	var out componentWire
	path := "/rest/api/2/component/" + url.PathEscape(id)
	if err := c.componentWrite(ctx, http.MethodPut, path, body, &out); err != nil {
		return Component{}, fmt.Errorf("update component %s: %w", id, err)
	}
	return out.component(), nil
}

// DeleteComponent deletes a component. With moveIssuesTo set, Jira first moves
// the component's issues to that component.
func (c *Client) DeleteComponent(ctx context.Context, id, moveIssuesTo string) error {
	path := "/rest/api/2/component/" + url.PathEscape(id)
	if moveIssuesTo != "" {
		path += "?moveIssuesTo=" + url.QueryEscape(moveIssuesTo)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("delete component %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("delete component %s: %w", id, componentHTTPError(http.MethodDelete, path, resp.StatusCode, resp.Status, b))
	}
	return nil
}

// ComponentIssueCount is how many issues (of any type) carry the component,
// shown before a delete.
func (c *Client) ComponentIssueCount(ctx context.Context, id string) (int, error) {
	var out struct {
		IssueCount int `json:"issueCount"`
	}
	path := "/rest/api/2/component/" + url.PathEscape(id) + "/relatedIssueCounts"
	if err := c.get(ctx, path, &out); err != nil {
		return 0, fmt.Errorf("component issue count %s: %w", id, err)
	}
	return out.IssueCount, nil
}

// SearchUsers finds users by username, name or email for the lead picker.
func (c *Client) SearchUsers(ctx context.Context, query string) ([]User, error) {
	var out []User
	path := "/rest/api/2/user/search?maxResults=10&username=" + url.QueryEscape(query)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	return out, nil
}

// componentWrite sends a JSON write and turns any non-2xx answer into an
// *HTTPError with the numeric code, which core's WriteError lacks.
func (c *Client) componentWrite(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.WriteJSONRaw(ctx, method, path, body)
	if err != nil {
		return err
	}
	if resp.Code >= 300 {
		return componentHTTPError(method, path, resp.Code, resp.Status, resp.Body)
	}
	if out == nil || len(strings.TrimSpace(string(resp.Body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

// componentHTTPError builds an *HTTPError, reading Jira's errorMessages and
// errors fields into Message.
func componentHTTPError(method, path string, code int, status string, body []byte) *HTTPError {
	var parsed struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	_ = json.Unmarshal(body, &parsed)
	msgs := append([]string{}, parsed.ErrorMessages...)
	for field, m := range parsed.Errors {
		msgs = append(msgs, field+": "+m)
	}
	return &HTTPError{Method: method, Path: path, Code: code, Status: status, Message: strings.Join(msgs, "; ")}
}
