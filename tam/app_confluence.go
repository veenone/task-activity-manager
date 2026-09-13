package main

import (
	"errors"
	"strings"

	"agile-suite/core/confluence"
	"agile-suite/core/profile"
)

func (a *App) GetConfluenceConfig(profileID string) (profile.ConfluenceConfig, error) {
	if err := a.requireStore(); err != nil {
		return profile.ConfluenceConfig{}, err
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return profile.ConfluenceConfig{}, err
	}
	c, err := a.profiles.ConfluenceConfig(profileID)
	if err != nil {
		return profile.ConfluenceConfig{}, err
	}
	if strings.TrimSpace(c.BaseURL) == "" && isRitualDemo(p.JiraURL) {
		return profile.ConfluenceConfig{BaseURL: "demo", SpaceKey: "DEMO", RootPageID: "demo-root"}, nil
	}
	return c, nil
}

func (a *App) SetConfluenceConfig(profileID string, config profile.ConfluenceConfig, token string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return err
	}
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	if config.BaseURL == "" && strings.TrimSpace(token) == "" {
		return a.profiles.SetConfluenceConfig(profileID, config)
	}
	if config.BaseURL == "" {
		return errors.New("Confluence URL is required when saving a token")
	}
	if err := a.profiles.SetConfluenceConfig(profileID, config); err != nil {
		return err
	}
	if strings.TrimSpace(token) != "" {
		return a.creds.Save(profile.ConfluenceCredentialID(profileID), strings.TrimSpace(token))
	}
	return nil
}

func (a *App) GetConfluencePage(profileID, pageID string) (confluence.Page, error) {
	_, client, err := a.confluenceClient(profileID)
	if err != nil {
		return confluence.Page{}, err
	}
	return client.GetPage(a.ctx, pageID)
}

func (a *App) ListConfluenceChildPages(profileID, parentID string, start, limit int) (confluence.ChildPageResult, error) {
	_, client, err := a.confluenceClient(profileID)
	if err != nil {
		return confluence.ChildPageResult{}, err
	}
	return client.ListChildPages(a.ctx, parentID, start, limit)
}

func (a *App) confluenceClient(profileID string) (profile.ConfluenceConfig, *confluence.Client, error) {
	if err := a.requireStore(); err != nil {
		return profile.ConfluenceConfig{}, nil, err
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return profile.ConfluenceConfig{}, nil, err
	}
	c, err := a.profiles.ConfluenceConfig(profileID)
	if err != nil {
		return c, nil, err
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return c, nil, errors.New("Confluence is not configured for this profile")
	}
	token, err := a.creds.Load(profile.ConfluenceCredentialID(profileID))
	if err != nil {
		return c, nil, errors.New("Confluence credentials are not configured for this profile")
	}
	return c, confluence.NewClient(c.BaseURL, token, p.CACert, p.AllowUntrustedTLS), nil
}
