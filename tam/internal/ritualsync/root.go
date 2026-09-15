package ritualsync

import (
	"context"
	"strings"

	"agile-suite/core/confluence"
)

// RootMissing is a pass that stopped because the rituals root page answered
// 404. It travels in the result, not as an error, for the reason Result does:
// the view needs every field to offer to create a root, and Wails would
// deliver only the sentence. CanCreate is the permission probe's answer with
// unknown read as true, so an instance that reports nothing still gets the
// offer and the create's own 403 speaks for it.
type RootMissing struct {
	PageID         string `json:"pageId"`
	SpaceKey       string `json:"spaceKey"`
	CanCreate      bool   `json:"canCreate"`
	SuggestedTitle string `json:"suggestedTitle"`
}

// SuggestedRootTitle is the title the missing-root dialog starts from.
func SuggestedRootTitle(projectKey string) string {
	return strings.TrimSpace(strings.TrimSpace(projectKey) + " Rituals")
}

// canCreate reads the transport's permission probe. Unknown, and a transport
// with no probe at all, both read as worth trying.
func canCreate(ctx context.Context, pages confluence.Pages, spaceKey string) bool {
	probe, ok := pages.(confluence.SpaceProbe)
	if !ok {
		return true
	}
	return probe.CanCreatePages(ctx, spaceKey) != confluence.PermissionNo
}
