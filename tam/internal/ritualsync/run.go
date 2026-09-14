package ritualsync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"agile-suite/core/confluence"
	"agile-suite/tam/internal/errtext"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

// PageFailure is one page a pass could not bring into step, with the reason
// in one readable line.
type PageFailure struct {
	SprintName string `json:"sprintName"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
}

// Result is what a pass did. It travels as a value, not a Go error, because
// Wails fills in a bound method's value or its error and never both: a pass
// that created four pages and failed one has to deliver all five facts.
type Result struct {
	Created   int           `json:"created"`
	Pulled    int           `json:"pulled"`
	Pushed    int           `json:"pushed"`
	Conflicts int           `json:"conflicts"`
	Gone      int           `json:"gone"`
	Failed    []PageFailure `json:"failed"`
	SyncedAt  string        `json:"syncedAt"`
}

type pass struct {
	ctx   context.Context
	pages confluence.Pages
	docs  *ritualrepo.Repository
	cfg   Config
	res   *Result
}

func (p *pass) now() string { return stamp(p.cfg.Now()) }

func (p *pass) fail(sp Sprint, d ritualrepo.Document, reason string) {
	p.res.Failed = append(p.res.Failed, PageFailure{SprintName: sp.Info.Name, Title: d.Title, Reason: reason})
}

// Run is one Sync pass over a board's sprints. Every write it makes to a
// document is either blind to body or conditional on body still holding what
// the pass read, so a save made while the pass runs is never overwritten and
// never claimed as synced.
func Run(ctx context.Context, pages confluence.Pages, docs *ritualrepo.Repository, cfg Config, profileID string, boardID int, sprints []Sprint) (Result, error) {
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	res := Result{Failed: []PageFailure{}}
	if _, err := pages.GetPageStorage(ctx, cfg.RootID); err != nil {
		return res, fmt.Errorf("The Confluence root page %s could not be read: %s", cfg.RootID, errtext.Line(err))
	}
	p := &pass{ctx: ctx, pages: pages, docs: docs, cfg: cfg, res: &res}
	for _, sp := range sprints {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if err := Ensure(ctx, docs, profileID, boardID, sp, cfg.Location, cfg.Now()); err != nil {
			return res, err
		}
		list, err := docs.Documents(ctx, profileID, boardID, sp.Info.ID)
		if err != nil {
			return res, err
		}
		byType := map[string]ritualrepo.Document{}
		for _, d := range list {
			byType[d.RitualType] = d
		}
		overview, ok := byType[ritualtemplate.Sprint]
		if !ok {
			// A closed sprint whose overview was removed locally has nothing
			// its rituals could be created under.
			continue
		}
		parentID, ok, err := p.page(sp, overview, cfg.RootID)
		if err != nil {
			return res, err
		}
		if !ok {
			continue
		}
		for _, t := range ritualtemplate.Types[1:] {
			d, ok := byType[t]
			if !ok {
				continue
			}
			if _, _, err := p.page(sp, d, parentID); err != nil {
				return res, err
			}
		}
	}
	res.SyncedAt = stamp(cfg.Now())
	return res, nil
}

// page brings one document and its Confluence page into step and answers
// with the page id children hang under. ok is false when there is no page
// to create children beneath: a refused title, a failed create, or a page
// gone from Confluence. The error is only ever the local store's; everything
// Confluence does wrong is recorded in the result instead.
func (p *pass) page(sp Sprint, d ritualrepo.Document, parentID string) (string, bool, error) {
	if d.Status == ritualrepo.StatusGone {
		return "", false, nil
	}
	if d.PageID == "" {
		return p.place(sp, d, parentID)
	}
	return p.reconcile(sp, d)
}

// place finds a home for a document with no page: adopt a page of the same
// title under parentID, refuse one anywhere else in the space, or create it.
func (p *pass) place(sp Sprint, d ritualrepo.Document, parentID string) (string, bool, error) {
	k := d.Key()
	found, exists, err := p.pages.FindPageByTitle(p.ctx, p.cfg.SpaceKey, d.Title)
	if err != nil {
		p.fail(sp, d, errtext.Line(err))
		return "", false, nil
	}
	if !exists {
		created, err := p.pages.CreatePage(p.ctx, p.cfg.SpaceKey, parentID, d.Title, d.Body)
		if err != nil {
			p.fail(sp, d, errtext.Line(err))
			return "", false, nil
		}
		if err := p.docs.ApplyCreated(p.ctx, k, created.ID, d.Body, created.Version, p.now()); err != nil {
			return "", false, err
		}
		p.res.Created++
		return created.ID, true, nil
	}
	if !slices.Contains(found.AncestorIDs, parentID) {
		p.fail(sp, d, fmt.Sprintf("A page titled %q already exists outside the rituals root. Rename one of them.", d.Title))
		return "", false, nil
	}
	// An untouched template takes the page somebody already wrote. Anything
	// else is somebody's text on both sides, and the user decides.
	if d.Body == ritualtemplate.Render(d.RitualType, sp.Info, p.cfg.Location) {
		pulled, err := p.docs.ApplyPulled(p.ctx, k, found.ID, d.Body, found.Body, found.Version, p.now())
		if err != nil {
			return "", false, err
		}
		if pulled {
			p.res.Pulled++
			return found.ID, true, nil
		}
	}
	if err := p.docs.ApplyConflict(p.ctx, k, found.ID, found.Body, found.Version); err != nil {
		return "", false, err
	}
	p.res.Conflicts++
	return found.ID, true, nil
}

// reconcile is Task 9's half: a document that already has a page. Until then
// it leaves the page alone, which is what an unchanged page wants anyway.
func (p *pass) reconcile(sp Sprint, d ritualrepo.Document) (string, bool, error) {
	_ = errors.Is
	return d.PageID, true, nil
}
