package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"agile-suite/core/confluence"
	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
	"agile-suite/tam/internal/errtext"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualsync"
	"agile-suite/tam/internal/ritualtemplate"
	"agile-suite/tam/internal/sprintdate"
	"agile-suite/tam/internal/suiteprofiles"
)

// requireRituals guards the ritual document store the way requireStore guards
// the profile store: a profile that was never opened has no local database.
func (a *App) requireRituals() error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if a.rituals == nil {
		return errors.New("local ritual store not initialised")
	}
	return nil
}

// confluencePages answers with the page transport a profile's rituals sync
// through and the configuration it was built from. It reads only local
// configuration and the credential store: nothing here makes a request.
func (a *App) confluencePages(p profile.Profile) (profile.ConfluenceConfig, confluence.Pages, error) {
	c, err := a.profiles.ConfluenceConfig(p.ID)
	if err != nil {
		return c, nil, err
	}
	if strings.TrimSpace(c.BaseURL) == "" && suiteprofiles.IsDemoURL(p.JiraURL) {
		c = profile.ConfluenceConfig{BaseURL: "demo", SpaceKey: "DEMO", RootPageID: "demo-root"}
	}
	switch {
	case strings.TrimSpace(c.BaseURL) == "":
		return c, nil, errors.New("Confluence is not configured for this profile")
	case strings.TrimSpace(c.SpaceKey) == "" || strings.TrimSpace(c.RootPageID) == "":
		return c, nil, errors.New("Confluence needs a space key and a root page id before rituals can sync")
	}
	if strings.EqualFold(strings.TrimSpace(c.BaseURL), "demo") {
		return c, a.demoSpace(p.ID, c), nil
	}
	_, client, err := a.confluenceClient(p.ID)
	if err != nil {
		return c, nil, err
	}
	return c, client, nil
}

// demoSpace is a profile's in-memory space, rebuilt from the pages the store
// knows about the first time it is asked for after the app starts, so a
// restart does not turn every demo page gone.
func (a *App) demoSpace(profileID string, c profile.ConfluenceConfig) *demo.Confluence {
	a.backendMu.Lock()
	defer a.backendMu.Unlock()
	if a.demoConfluence == nil {
		a.demoConfluence = map[string]*demo.Confluence{}
	}
	if space, ok := a.demoConfluence[profileID]; ok {
		return space
	}
	space := demo.NewConfluence(c.SpaceKey, c.RootPageID, true)
	if docs, err := a.rituals.ProfileDocuments(a.ctx, profileID); err != nil {
		log.Printf("tam: rebuild demo ritual pages for %s: %v", profileID, err)
	} else {
		// A row can carry a page id and still be gone: MarkGone only flips
		// status, so ProfileDocuments' confluence_page_id <> '' filter alone
		// cannot tell the two apart. Restoring a gone page here would put it
		// back in the space under its old title, and a later ForgetRitualPage
		// would then have Sync's place() find and adopt that resurrected page
		// by title instead of creating a genuinely new one. A gone overview
		// is skipped from the parent map too, so a gone ritual page never
		// gets rebuilt under it either.
		overviews := map[[2]int]string{}
		for _, d := range docs {
			if d.RitualType == ritualtemplate.Sprint && d.Status != ritualrepo.StatusGone {
				overviews[[2]int{d.BoardID, d.SprintID}] = d.PageID
				space.Restore(d.PageID, c.RootPageID, d.Title, d.BaseBody, d.Version)
			}
		}
		for _, d := range docs {
			if d.Status == ritualrepo.StatusGone {
				continue
			}
			if parent, ok := overviews[[2]int{d.BoardID, d.SprintID}]; ok && d.RitualType != ritualtemplate.Sprint {
				space.Restore(d.PageID, parent, d.Title, d.BaseBody, d.Version)
			}
		}
	}
	a.demoConfluence[profileID] = space
	return space
}

func (a *App) boardName(profileID string, boardID int) string {
	boards, err := a.boards.ListBoards(a.ctx, profileID)
	if err != nil {
		return ""
	}
	for _, b := range boards {
		if b.ID == boardID {
			return b.Name
		}
	}
	return ""
}

func (a *App) ritualSprint(profileID string, boardID, sprintID int) (ritualsync.Sprint, error) {
	cached, err := a.boards.ListSprints(a.ctx, profileID, boardID)
	if err != nil {
		return ritualsync.Sprint{}, err
	}
	for _, s := range cached {
		if s.ID == sprintID {
			return ritualsync.Sprint{Info: ritualsync.Info(s, a.boardName(profileID, boardID)), State: s.State}, nil
		}
	}
	return ritualsync.Sprint{}, fmt.Errorf("sprint %d is not in this board's cache; refresh the board first", sprintID)
}

func ritualKey(profileID string, boardID, sprintID int, ritualType string) (ritualrepo.Key, error) {
	if !ritualtemplate.Known(ritualType) {
		return ritualrepo.Key{}, errors.New("unknown ritual type")
	}
	return ritualrepo.Key{ProfileID: profileID, BoardID: boardID, SprintID: sprintID, RitualType: ritualType}, nil
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// EnsureSprintRituals writes whichever of a sprint's five pages are missing,
// from templates, and lists the sprint's pages. Local only, no lock: this is
// what lets a planning page be written with no network at all.
func (a *App) EnsureSprintRituals(profileID string, boardID, sprintID int) ([]ritualrepo.Document, error) {
	if _, err := a.requireProfile(profileID); err != nil {
		return nil, err
	}
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	sp, err := a.ritualSprint(profileID, boardID, sprintID)
	if err != nil {
		return nil, err
	}
	if err := ritualsync.Ensure(a.ctx, a.rituals, profileID, boardID, sp, time.Local, time.Now()); err != nil {
		return nil, err
	}
	return a.rituals.Documents(a.ctx, profileID, boardID, sprintID)
}

// ListRitualDocuments reads a sprint's pages from tam.db, and nothing else.
func (a *App) ListRitualDocuments(profileID string, boardID, sprintID int) ([]ritualrepo.Document, error) {
	if _, err := a.requireProfile(profileID); err != nil {
		return nil, err
	}
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	return a.rituals.Documents(a.ctx, profileID, boardID, sprintID)
}

// SaveRitualBody is the editor's local save. It takes no lock, the way a
// board move takes none; the sync's compare-and-set is what makes that safe.
func (a *App) SaveRitualBody(profileID string, boardID, sprintID int, ritualType, body string) (ritualrepo.Document, error) {
	if _, err := a.requireProfile(profileID); err != nil {
		return ritualrepo.Document{}, err
	}
	if err := a.requireRituals(); err != nil {
		return ritualrepo.Document{}, err
	}
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		return ritualrepo.Document{}, err
	}
	return a.rituals.SaveBody(a.ctx, k, body, nowStamp())
}

// ResolveRitualConflict keeps the local body ("mine", pushed on the next
// Sync) or takes Confluence's ("theirs"). Local, no network.
func (a *App) ResolveRitualConflict(profileID string, boardID, sprintID int, ritualType, choice string) error {
	if _, err := a.requireProfile(profileID); err != nil {
		return err
	}
	if err := a.requireRituals(); err != nil {
		return err
	}
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		return err
	}
	switch choice {
	case "mine":
		return a.rituals.ResolveMine(a.ctx, k, nowStamp())
	case "theirs":
		return a.rituals.ResolveTheirs(a.ctx, k, nowStamp())
	}
	return fmt.Errorf("unknown conflict choice %q", choice)
}

// ForgetRitualPage lets a page gone from Confluence be created again on the
// next Sync, from the local body.
func (a *App) ForgetRitualPage(profileID string, boardID, sprintID int, ritualType string) error {
	if _, err := a.requireProfile(profileID); err != nil {
		return err
	}
	if err := a.requireRituals(); err != nil {
		return err
	}
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		return err
	}
	return a.rituals.ForgetPage(a.ctx, k, nowStamp())
}

// DeleteRitualDocument removes the local copy. The Confluence page is left
// alone; the next time the sprint opens, a fresh template takes its place.
func (a *App) DeleteRitualDocument(profileID string, boardID, sprintID int, ritualType string) error {
	if _, err := a.requireProfile(profileID); err != nil {
		return err
	}
	if err := a.requireRituals(); err != nil {
		return err
	}
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		return err
	}
	return a.rituals.DeleteDocument(a.ctx, k)
}

// RitualMacroPreview is what a Jira Issues macro shows inside the editor.
type RitualMacroPreview struct {
	Supported bool            `json:"supported"`
	JQL       string          `json:"jql"`
	Issues    []backend.Issue `json:"issues"`
}

// RitualMacroIssues answers a macro's query from the issue cache, for the
// three forms the templates write. Done here is backend.IsDone, the status
// name rule, which is not Jira's statusCategory: the editor says so under
// every preview.
func (a *App) RitualMacroIssues(profileID, jql string) (RitualMacroPreview, error) {
	out := RitualMacroPreview{JQL: jql, Issues: []backend.Issue{}}
	if _, err := a.requireProfile(profileID); err != nil {
		return out, err
	}
	sprintID, filter, ok := ritualtemplate.ParseJQL(jql)
	if !ok {
		return out, nil
	}
	out.Supported = true
	page, err := a.repo.ListIssues(a.ctx, profileID, issuerepo.IssueQuery{SprintID: strconv.Itoa(sprintID), Limit: 500})
	if err != nil {
		return out, err
	}
	for _, issue := range page.Issues {
		done := backend.IsDone(issue.Status)
		if (filter == ritualtemplate.Done && !done) || (filter == ritualtemplate.NotDone && done) {
			continue
		}
		out.Issues = append(out.Issues, issue)
	}
	return out, nil
}

// StandupEntry is one day of the standup log for a YYYY-MM-DD day, the same
// fragment the template seeds, so "Add today's entry" and the template agree.
func (a *App) StandupEntry(day string) (string, error) {
	t, err := time.ParseInLocation(sprintdate.Day, strings.TrimSpace(day), time.Local)
	if err != nil {
		return "", fmt.Errorf("read standup day %q: %w", day, err)
	}
	return ritualtemplate.StandupEntry(t), nil
}

func lastRitualSyncKey(boardID int) string { return "rituals_last_sync:" + strconv.Itoa(boardID) }

// LastRitualSync is when a board's rituals last finished a Sync, "" if never.
func (a *App) LastRitualSync(profileID string, boardID int) (string, error) {
	if _, err := a.requireProfile(profileID); err != nil {
		return "", err
	}
	return a.repo.ProfileSetting(a.ctx, profileID, lastRitualSyncKey(boardID))
}

// SyncRituals is the Rituals view's Sync: one pass over a board's open
// sprints, and closed ones that already have pages. It holds the profile
// lock under "rituals". Configuration and credentials are refused before
// the lock is taken, since neither needs it; per-page trouble travels in the
// result.
func (a *App) SyncRituals(profileID string, boardID int) (ritualsync.Result, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return ritualsync.Result{}, err
	}
	if err := a.requireRituals(); err != nil {
		return ritualsync.Result{}, err
	}
	cfg, pages, err := a.confluencePages(p)
	if err != nil {
		return ritualsync.Result{}, err
	}
	if err := a.acquire(p.ID, "rituals"); err != nil {
		return ritualsync.Result{}, err
	}
	defer a.release(p.ID)
	log.Printf("tam: rituals sync for %s board %d starting", p.ID, boardID)

	cached, err := a.boards.ListSprints(a.ctx, p.ID, boardID)
	if err != nil {
		return ritualsync.Result{}, err
	}
	withRows, err := a.rituals.BoardSprintIDs(a.ctx, p.ID, boardID)
	if err != nil {
		return ritualsync.Result{}, err
	}
	res, err := ritualsync.Run(a.ctx, pages, a.rituals, ritualsync.Config{
		SpaceKey: cfg.SpaceKey, RootID: cfg.RootPageID, Location: time.Local, Now: time.Now,
	}, p.ID, boardID, ritualsync.Sprints(cached, a.boardName(p.ID, boardID), withRows))
	if err != nil {
		log.Printf("tam: rituals sync for %s board %d refused: %v", p.ID, boardID, err)
		return ritualsync.Result{}, errors.New(errtext.Line(err))
	}
	if err := a.repo.SetProfileSetting(a.ctx, p.ID, lastRitualSyncKey(boardID), res.SyncedAt); err != nil {
		log.Printf("tam: record rituals sync time for %s: %v", p.ID, err)
	}
	log.Printf("tam: rituals sync for %s board %d done: %d created, %d pulled, %d pushed, %d conflicts, %d gone, %d failed",
		p.ID, boardID, res.Created, res.Pulled, res.Pushed, res.Conflicts, res.Gone, len(res.Failed))
	return res, nil
}
