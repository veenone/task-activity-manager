package ritualsync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"agile-suite/core/confluence"
	"agile-suite/tam/internal/ritualtemplate"
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

// Root outcomes. Each is an answer, not an error: forbidden and titleTaken are
// what the dialog words, and Wails delivers a value or an error, never both.
const (
	RootCreated    = "created"
	RootAdopted    = "adopted"
	RootForbidden  = "forbidden"
	RootTitleTaken = "titleTaken"
)

// Root is what an attempt to give the rituals a new root page came to.
type Root struct {
	Outcome  string `json:"outcome"`
	PageID   string `json:"pageId"`
	Title    string `json:"title"`
	SpaceKey string `json:"spaceKey"`
	// TopLevel says whether the page sits at the top of the space. For
	// titleTaken it is the page already holding the title (PageID), and only
	// a top-level one may be adopted.
	TopLevel bool `json:"topLevel"`
}

// CreateRoot creates a rituals root page at the top of the space, or, with
// adopt, takes the top-level page that already carries the title. It writes
// nothing locally: saving the new id to the profile and running the Sync are
// the caller's, under the lock.
//
// A create refused with 403 is forbidden. A create refused with 400 or 409 is
// looked up by title rather than read for its message, since which of the two
// Confluence answers, and in what words, is a probe question
// (docs/superpowers/plans/assets/2026-09-15-confluence-root-page-probe.md);
// a title that turns out to be taken is titleTaken whichever it was.
func CreateRoot(ctx context.Context, pages confluence.Pages, spaceKey, projectKey, title string, adopt bool) (Root, error) {
	out := Root{Title: strings.TrimSpace(title), SpaceKey: spaceKey}
	if out.Title == "" {
		return out, errors.New("The root page needs a title")
	}
	if adopt {
		return adoptRoot(ctx, pages, out)
	}
	created, err := pages.CreatePage(ctx, spaceKey, "", out.Title, ritualtemplate.RootBody(projectKey))
	if err == nil {
		out.Outcome, out.PageID, out.TopLevel = RootCreated, created.ID, true
		return out, nil
	}
	var h *confluence.HTTPError
	if !errors.As(err, &h) {
		return out, err
	}
	switch h.Code {
	case http.StatusForbidden:
		out.Outcome = RootForbidden
		return out, nil
	case http.StatusBadRequest, http.StatusConflict:
		if found, ok, findErr := pages.FindPageByTitle(ctx, spaceKey, out.Title); findErr == nil && ok {
			out.Outcome, out.PageID, out.TopLevel = RootTitleTaken, found.ID, len(found.AncestorIDs) == 0
			return out, nil
		}
	}
	return out, err
}

// adoptRoot takes the page already holding the title, and only from the top
// of the space: the dialog's second confirmation named that placement, and a
// page below another page belongs to somebody else's tree.
func adoptRoot(ctx context.Context, pages confluence.Pages, out Root) (Root, error) {
	found, ok, err := pages.FindPageByTitle(ctx, out.SpaceKey, out.Title)
	if err != nil {
		return out, err
	}
	if !ok {
		return out, fmt.Errorf("No page titled %q is in %s any more. Create the root page instead.", out.Title, out.SpaceKey)
	}
	out.PageID = found.ID
	if len(found.AncestorIDs) != 0 {
		out.Outcome = RootTitleTaken
		return out, nil
	}
	out.Outcome, out.TopLevel = RootAdopted, true
	return out, nil
}
