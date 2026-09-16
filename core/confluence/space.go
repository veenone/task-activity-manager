package confluence

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// Permission is what a probe learned about one thing a token may do in a
// space. Unknown is its own answer, not a polite no: an instance that does
// not report operations says nothing about the token.
type Permission string

const (
	PermissionYes     Permission = "yes"
	PermissionNo      Permission = "no"
	PermissionUnknown Permission = "unknown"
)

// SpaceProbe is the question the ritual sync asks before it offers to create
// a rituals root page. It is its own interface rather than a fifth method on
// Pages because an ordinary pass never needs it; only a missing root does.
type SpaceProbe interface {
	CanCreatePages(ctx context.Context, spaceKey string) Permission
}

var _ SpaceProbe = (*Client)(nil)

// CanCreatePages reads the space the way the token sees it, in two steps.
// First a one-page content listing: a space the token cannot read answers
// 401, 403 or 404, and creating in it is a no. Then the space's operations,
// where an instance that reports them lists create on page for a token that
// may. Anything else (a transport failure, an instance that leaves operations
// out) is unknown, which the caller treats as worth trying, with the create's
// own refusal as the fallback. What a real Data Center answers is recorded in
// docs/superpowers/plans/assets/2026-09-15-confluence-root-page-probe.md.
func (c *Client) CanCreatePages(ctx context.Context, spaceKey string) Permission {
	q := url.Values{}
	q.Set("spaceKey", spaceKey)
	q.Set("limit", "1")
	var listing struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := c.get(ctx, "/rest/api/content?"+q.Encode()).Decode(&listing); err != nil {
		if refused(err) {
			return PermissionNo
		}
		return PermissionUnknown
	}
	var space struct {
		Operations []struct {
			Operation  string `json:"operation"`
			TargetType string `json:"targetType"`
		} `json:"operations"`
	}
	if err := c.get(ctx, "/rest/api/space/"+url.PathEscape(spaceKey)+"?expand=operations").Decode(&space); err != nil || len(space.Operations) == 0 {
		return PermissionUnknown
	}
	for _, op := range space.Operations {
		if op.Operation == "create" && op.TargetType == "page" {
			return PermissionYes
		}
	}
	return PermissionNo
}

// refused is whether an answer says the token may not see the space at all.
func refused(err error) bool {
	var h *HTTPError
	if !errors.As(err, &h) {
		return false
	}
	switch h.Code {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	}
	return false
}
