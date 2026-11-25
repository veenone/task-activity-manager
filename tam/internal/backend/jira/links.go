package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"agile-suite/tam/internal/backend"
)

// linkTypesResponse is Jira's /rest/api/2/issueLinkType answer.
type linkTypesResponse struct {
	IssueLinkTypes []struct {
		Name    string `json:"name"`
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"issueLinkTypes"`
}

// LinkTypes reads the instance's link types once per backend.
func (b *Backend) LinkTypes(ctx context.Context) ([]backend.LinkType, error) {
	b.mu.Lock()
	if b.linkTypesLoaded {
		out := append([]backend.LinkType{}, b.linkTypes...)
		b.mu.Unlock()
		return out, nil
	}
	b.mu.Unlock()
	var resp linkTypesResponse
	if err := b.c.Get(ctx, "/rest/api/2/issueLinkType", &resp); err != nil {
		return nil, err
	}
	types := make([]backend.LinkType, 0, len(resp.IssueLinkTypes))
	for _, t := range resp.IssueLinkTypes {
		types = append(types, backend.LinkType{Name: t.Name, Inward: t.Inward, Outward: t.Outward})
	}
	b.mu.Lock()
	b.linkTypes = types
	b.linkTypesLoaded = true
	b.mu.Unlock()
	return append([]backend.LinkType{}, types...), nil
}

// CreateLink POSTs an issue link. For an outward link the source is the
// outward issue ("PLAT-1 blocks PAY-7"); for an inward one it is the inward
// issue ("PLAT-1 is blocked by PAY-7").
func (b *Backend) CreateLink(ctx context.Context, fromKey string, d backend.LinkDraft) error {
	var outward, inward string
	switch d.Direction {
	case "outward":
		outward, inward = fromKey, d.ToKey
	case "inward":
		outward, inward = d.ToKey, fromKey
	default:
		return fmt.Errorf("link direction %q is neither outward nor inward", d.Direction)
	}
	body := map[string]any{
		"type":         map[string]string{"name": d.Type},
		"inwardIssue":  map[string]string{"key": inward},
		"outwardIssue": map[string]string{"key": outward},
	}
	return b.c.Post(ctx, "/rest/api/2/issueLink", body)
}

// parseLinks flattens Jira's issuelinks array: each entry names the link
// type and carries either an inwardIssue or an outwardIssue.
func parseLinks(raw json.RawMessage) []backend.Link {
	out := []backend.Link{}
	if len(raw) == 0 {
		return out
	}
	type linked struct {
		Key    string `json:"key"`
		Fields struct {
			Summary   string `json:"summary"`
			IssueType named  `json:"issuetype"`
		} `json:"fields"`
	}
	var entries []struct {
		ID      string  `json:"id"`
		Type    named   `json:"type"`
		Inward  *linked `json:"inwardIssue"`
		Outward *linked `json:"outwardIssue"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return out
	}
	for _, e := range entries {
		if e.Inward != nil {
			out = append(out, backend.Link{ID: e.ID, Direction: "inward", Type: e.Type.Name, Key: e.Inward.Key, Summary: e.Inward.Fields.Summary, IssueType: e.Inward.Fields.IssueType.Name})
		}
		if e.Outward != nil {
			out = append(out, backend.Link{ID: e.ID, Direction: "outward", Type: e.Type.Name, Key: e.Outward.Key, Summary: e.Outward.Fields.Summary, IssueType: e.Outward.Fields.IssueType.Name})
		}
	}
	return out
}

// DeleteLink deletes the link with Jira's id (#612).
func (b *Backend) DeleteLink(ctx context.Context, id string) error {
	return b.c.Delete(ctx, "/rest/api/2/issueLink/"+url.PathEscape(id))
}
