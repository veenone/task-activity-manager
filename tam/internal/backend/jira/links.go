package jira

import (
	"context"
	"fmt"

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
