package main

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"agile-suite/core/confluence"
	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/ritualdefaults"
	"agile-suite/tam/internal/ritualrepo"
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
	ritualType = strings.TrimSpace(strings.ToLower(ritualType))
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
	// Normalised once, before any repository call: the lookup below, the
	// write, and every other method's lookup must all agree with
	// knownRitualType's own case-insensitive check, or a caller passing
	// "Review" against a stored "review" row misses the existing document
	// entirely and silently wipes its publication fields.
	draft.RitualType = strings.TrimSpace(strings.ToLower(draft.RitualType))

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
	ritualType = strings.TrimSpace(strings.ToLower(ritualType))
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
