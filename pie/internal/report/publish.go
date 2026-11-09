// Package report publishes a reporting cycle to Confluence. It never takes
// over a page PIE did not write, or writes over an edit made in Confluence,
// without the analyst saying so.
package report

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agile-suite/core/confluence"
	"agile-suite/pie/internal/cycles"
)

// Target is where a profile's reports go: a space, and the page they sit
// under, or "" for the top of the space.
type Target struct {
	Profile string
	Space   string
	Parent  string
}

// Outcome is a publish that wrote PageID, or one that stopped to Ask the
// analyst first. Nothing was written when Ask is set.
type Outcome struct {
	Title  string `json:"title"`
	PageID string `json:"pageId"`
	Ask    string `json:"ask"`
}

// Publish writes the cycle to its page and marks it published. The first
// publish creates the page; later ones update the page the cycle recorded.
// confirmed is the analyst's yes to the question an earlier call asked.
func Publish(ctx context.Context, pages confluence.Pages, cy *cycles.Store, to Target, id int64, confirmed bool, at time.Time) (Outcome, error) {
	d, err := cy.Get(to.Profile, id)
	if err != nil {
		return Outcome{}, err
	}
	out := Outcome{Title: "Impact report: " + d.Name}
	body := Storage(d)

	page, err := ownPage(ctx, pages, cy, to.Profile, id)
	if err != nil {
		return Outcome{}, err
	}
	if page == nil {
		found, exists, err := pages.FindPageByTitle(ctx, to.Space, out.Title)
		if err != nil {
			return Outcome{}, fmt.Errorf("Confluence did not say whether %q is in %s: %w", out.Title, to.Space, err)
		}
		if exists && !confirmed {
			out.Ask = fmt.Sprintf("A page named %q already exists in %s, and PIE did not write it. Publish over it?", out.Title, to.Space)
			return out, nil
		}
		if exists {
			page = &recorded{StoredPage: found, wrote: found.Version}
		}
	} else if page.Version != page.wrote && !confirmed {
		out.Ask = fmt.Sprintf("Someone edited %q in Confluence since PIE published it. Publish anyway and replace their edit?", out.Title)
		return out, nil
	}

	var written confluence.StoredPage
	if page == nil {
		written, err = pages.CreatePage(ctx, to.Space, to.Parent, out.Title, body)
	} else {
		written, err = pages.UpdatePage(ctx, page.ID, out.Title, body, page.Version+1)
	}
	if err != nil {
		return Outcome{}, fmt.Errorf("Confluence did not take %q: %w", out.Title, err)
	}
	out.PageID = written.ID
	return out, cy.MarkPublished(to.Profile, id, cycles.Page{ID: written.ID, Version: written.Version}, at)
}

// recorded is the page as Confluence has it now, with the version PIE wrote.
type recorded struct {
	confluence.StoredPage
	wrote int
}

// ownPage reads the page the cycle was published to. A page deleted in
// Confluence counts as none, so the next publish writes it again.
func ownPage(ctx context.Context, pages confluence.Pages, cy *cycles.Store, profile string, id int64) (*recorded, error) {
	rec, ok, err := cy.Page(profile, id)
	if err != nil || !ok {
		return nil, err
	}
	now, err := pages.GetPageStorage(ctx, rec.ID)
	if errors.Is(err, confluence.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Confluence did not return the cycle's page %s: %w", rec.ID, err)
	}
	return &recorded{StoredPage: now, wrote: rec.Version}, nil
}
