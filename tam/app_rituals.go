package main

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"agile-suite/core/confluence"
	"agile-suite/core/profile"
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
