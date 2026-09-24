package reportout

import (
	"context"
	"fmt"

	"agile-suite/core/confluence"
	"agile-suite/tam/internal/errtext"
)

// Published is the page a publish wrote, so the user is told which one it
// was rather than that something happened somewhere.
type Published struct {
	Title  string `json:"title"`
	PageID string `json:"pageId"`
}

// Publish writes a report to its own page in the space the rituals sync
// uses, under parentID, through the same confluence.Pages transport.
//
// It is a page of its own and not the sprint's overview page, which the
// rituals sync owns: writing the report into a page that sync tracks would
// bump the version behind the sync's back and leave that page in conflict
// on the next pass, every sprint, forever.
//
// It takes the page the title already names when there is one, so
// publishing the same sprint twice replaces the report rather than
// refusing on Confluence's unique title rule or leaving two pages of
// figures that disagree. A failure names the page and the reason, because
// "publishing failed" tells a user nothing they can act on.
func Publish(ctx context.Context, pages confluence.Pages, spaceKey, parentID string, d Document) (Published, error) {
	if err := d.Check(); err != nil {
		return Published{}, err
	}
	found, exists, err := pages.FindPageByTitle(ctx, spaceKey, d.Title)
	if err != nil {
		return Published{}, fmt.Errorf("Confluence could not be asked whether the page %q is in %s: %s", d.Title, spaceKey, errtext.Line(err))
	}
	body := Storage(d)
	if exists {
		updated, err := pages.UpdatePage(ctx, found.ID, d.Title, body, found.Version+1)
		if err != nil {
			return Published{}, fmt.Errorf("the Confluence page %q (%s) could not be updated: %s", d.Title, found.ID, errtext.Line(err))
		}
		return Published{Title: d.Title, PageID: updated.ID}, nil
	}
	created, err := pages.CreatePage(ctx, spaceKey, parentID, d.Title, body)
	if err != nil {
		return Published{}, fmt.Errorf("the Confluence page %q could not be created in %s: %s", d.Title, spaceKey, errtext.Line(err))
	}
	return Published{Title: d.Title, PageID: created.ID}, nil
}
