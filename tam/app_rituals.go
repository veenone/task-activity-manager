package main

import (
	"encoding/json"
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
	"agile-suite/tam/internal/ritualdefaults"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualsync"
	"agile-suite/tam/internal/ritualtemplate"
	"agile-suite/tam/internal/sprintdate"
	"agile-suite/tam/internal/suiteprofiles"
)

func (a *App) ListRitualAssociations(profileID string, boardID, sprintID int) ([]profile.RitualAssociation, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return nil, err
	}
	items, err := a.profiles.ListRitualAssociations(profileID, boardID, sprintID)
	if err != nil || len(items) > 0 || !isRitualDemo(p.JiraURL) {
		return items, err
	}
	_, children := confluence.DemoPages(p.ProjectKey)
	for i, child := range children {
		items = append(items, profile.RitualAssociation{BoardID: boardID, SprintID: sprintID, RitualType: []string{"planning", "standup", "review", "retro"}[i], PageID: child.ID, PageTitle: child.Title})
	}
	return items, nil
}

func (a *App) SetRitualAssociation(profileID string, assoc profile.RitualAssociation) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return err
	}
	if strings.TrimSpace(assoc.RitualType) == "" || strings.TrimSpace(assoc.PageID) == "" {
		return errors.New("ritual type and page id are required")
	}
	return a.profiles.SetRitualAssociation(profileID, assoc)
}

func (a *App) DeleteRitualAssociation(profileID string, assoc profile.RitualAssociation) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return err
	}
	return a.profiles.DeleteRitualAssociation(profileID, assoc)
}

func (a *App) GetRitualPage(profileID, pageID string) (confluence.Page, error) {
	profileRow, err := a.profiles.Get(profileID)
	if err != nil {
		return confluence.Page{}, err
	}
	if isRitualDemo(profileRow.JiraURL) {
		pages, _ := confluence.DemoPages(profileRow.ProjectKey)
		for _, page := range pages {
			if page.ID == pageID {
				return page, nil
			}
		}
		return confluence.Page{}, errors.New("demo: ritual page not found")
	}
	_, client, err := a.confluenceClient(profileID)
	if err != nil {
		return confluence.Page{}, err
	}
	p, err := client.GetPage(a.ctx, pageID)
	if err == nil {
		if payload, e := json.Marshal(p); e == nil {
			_ = a.profiles.CacheConfluencePage(profileID, pageID, string(payload), time.Now().UTC().Format(time.RFC3339))
		}
		return p, nil
	}
	payload, _, cacheErr := a.profiles.CachedConfluencePage(profileID, pageID)
	if cacheErr == nil && payload != "" {
		var cached confluence.Page
		if json.Unmarshal([]byte(payload), &cached) == nil {
			return cached, nil
		}
	}
	return confluence.Page{}, err
}

func isRitualDemo(jiraURL string) bool {
	u := strings.ToLower(strings.TrimSpace(jiraURL))
	return u == "demo" || u == "demo-pkcs" || u == "demo_pkcs"
}

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

// knownRitualType keeps an unrecognised type out of the table, because the
// composite primary key would otherwise accept any string and the view would
// grow a slot nothing can render.
func knownRitualType(ritualType string) bool {
	want := strings.TrimSpace(strings.ToLower(ritualType))
	for _, t := range ritualdefaults.Types {
		if t == want {
			return true
		}
	}
	return false
}

func (a *App) ListRitualDrafts(profileID string, boardID, sprintID int) ([]ritualrepo.Draft, error) {
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return nil, err
	}
	return a.rituals.ListDrafts(a.ctx, profileID, boardID, sprintID)
}

func (a *App) GetRitualDraft(profileID string, boardID, sprintID int, ritualType string) (ritualrepo.Draft, error) {
	if err := a.requireRituals(); err != nil {
		return ritualrepo.Draft{}, err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return ritualrepo.Draft{}, err
	}
	return a.rituals.Get(a.ctx, profileID, boardID, sprintID, ritualType)
}

// SaveRitualDraft writes the editable fields and nothing else. It always lands
// as a draft: publishing is a separate, deliberate press, and no save should
// ever reach Confluence.
func (a *App) SaveRitualDraft(profileID string, draft ritualrepo.Draft) error {
	if err := a.requireRituals(); err != nil {
		return err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return err
	}
	if !knownRitualType(draft.RitualType) {
		return errors.New("unknown ritual type")
	}
	if _, err := ritualrepo.DecodeIssues(draft.IssuesJSON); err != nil {
		return err
	}
	// The ritual type is normalised inside ritualrepo itself now (Get, Upsert,
	// and Delete all do it), so the lookup below and the write below that
	// agree with each other and with knownRitualType's own case-insensitive
	// check without this layer having to normalise a third time.

	// The publication fields belong to the publish path, so they are carried
	// from the stored row rather than trusted from the caller.
	stored, err := a.rituals.Get(a.ctx, profileID, draft.BoardID, draft.SprintID, draft.RitualType)
	if err != nil {
		return err
	}
	draft.ProfileID = profileID
	draft.ConfluencePageID = stored.ConfluencePageID
	draft.ConfluenceVersion = stored.ConfluenceVersion
	draft.Body = stored.Body
	draft.PublishedAt = stored.PublishedAt
	draft.Status = "draft"
	draft.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(draft.IssuesJSON) == "" {
		draft.IssuesJSON = "[]"
	}
	return a.rituals.Upsert(a.ctx, draft)
}

// DeleteRitualDraft removes the local document only. Any Confluence page it was
// published to is left exactly where it is, because deleting a team's meeting
// notes is not a side effect a list tidy should have.
func (a *App) DeleteRitualDraft(profileID string, boardID, sprintID int, ritualType string) error {
	if err := a.requireRituals(); err != nil {
		return err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return err
	}
	return a.rituals.Delete(a.ctx, profileID, boardID, sprintID, ritualType)
}

// ScaffoldSprintRituals creates whatever of a sprint's four rituals does not
// exist yet, each with the issues its type asks for. It never touches a ritual
// that is already there: running it twice must be safe, because the button sits
// next to documents somebody has been editing.
func (a *App) ScaffoldSprintRituals(profileID string, boardID, sprintID int) ([]ritualrepo.Draft, error) {
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return nil, err
	}

	page, err := a.repo.ListIssues(a.ctx, profileID, issuerepo.IssueQuery{
		SprintID: strconv.Itoa(sprintID),
		Limit:    500,
	})
	if err != nil {
		return nil, err
	}

	// A sprint whose name cannot be read still gets its rituals, titled by
	// ritual alone, because a scaffold must not fail on a cosmetic lookup.
	sprintName, err := a.boards.SprintName(a.ctx, profileID, strconv.Itoa(sprintID))
	if err != nil {
		sprintName = ""
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, ritualType := range ritualdefaults.Types {
		existing, err := a.rituals.Get(a.ctx, profileID, boardID, sprintID, ritualType)
		if err != nil {
			return nil, err
		}
		if existing.RitualType != "" {
			continue
		}
		issues, err := ritualrepo.EncodeIssues(ritualdefaults.Select(ritualType, page.Issues))
		if err != nil {
			return nil, err
		}
		if err := a.rituals.Upsert(a.ctx, ritualrepo.Draft{
			ProfileID:  profileID,
			BoardID:    boardID,
			SprintID:   sprintID,
			RitualType: ritualType,
			Title:      ritualdefaults.Title(ritualType, sprintName),
			IssuesJSON: issues,
			Status:     "draft",
			UpdatedAt:  now,
		}); err != nil {
			return nil, err
		}
	}
	return a.rituals.ListDrafts(a.ctx, profileID, boardID, sprintID)
}

// ListSprintIssues returns the sprint's issues for the wizard to choose from.
// It is a read of the local cache and takes no lock: the wizard opens while a
// sync may be running, and a picker that refuses to open because the profile is
// busy is worse than one showing slightly stale issues.
func (a *App) ListSprintIssues(profileID string, boardID, sprintID int) ([]backend.Issue, error) {
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return nil, err
	}
	page, err := a.repo.ListIssues(a.ctx, profileID, issuerepo.IssueQuery{
		SprintID: strconv.Itoa(sprintID),
		Limit:    500,
	})
	if err != nil {
		return nil, err
	}
	return page.Issues, nil
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
		overviews := map[[2]int]string{}
		for _, d := range docs {
			if d.RitualType == ritualtemplate.Sprint {
				overviews[[2]int{d.BoardID, d.SprintID}] = d.PageID
				space.Restore(d.PageID, c.RootPageID, d.Title, d.BaseBody, d.Version)
			}
		}
		for _, d := range docs {
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
