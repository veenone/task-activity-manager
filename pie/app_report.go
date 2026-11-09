package main

import (
	"errors"
	"log"
	"strings"
	"time"

	"agile-suite/core/confluence"
	"agile-suite/core/profile"
	"agile-suite/pie/internal/report"
)

// The bindings for publishing cycles to Confluence. The settings live in
// core's confluence_profile table and the token in the credential store.

// ConfluenceSettings returns where the profile's reports are published.
func (a *App) ConfluenceSettings(profileID string) (profile.ConfluenceConfig, error) {
	if err := a.requireStore(); err != nil {
		return profile.ConfluenceConfig{}, err
	}
	return a.profiles.ConfluenceConfig(profileID)
}

// SetConfluenceSettings stores the settings, and the token when one is given.
// An empty token keeps the stored one.
func (a *App) SetConfluenceSettings(profileID string, c profile.ConfluenceConfig, token string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if _, err := a.profiles.Get(profileID); err != nil {
		return err
	}
	c.BaseURL, c.SpaceKey, token = strings.TrimSpace(c.BaseURL), strings.TrimSpace(c.SpaceKey), strings.TrimSpace(token)
	if c.BaseURL != "" && c.SpaceKey == "" {
		return errors.New("enter the space key the reports go to")
	}
	if token != "" && c.BaseURL == "" {
		return errors.New("enter the Confluence URL the token is for")
	}
	// A token and page ids belong to one Confluence. A new URL needs its own
	// token, so the old one is never sent there, and forgets the old pages.
	was, err := a.profiles.ConfluenceConfig(profileID)
	if err != nil {
		return err
	}
	moved := was.BaseURL != "" && strings.TrimRight(c.BaseURL, "/") != was.BaseURL
	if moved && c.BaseURL != "" && token == "" {
		return errors.New("enter the token for the new Confluence URL")
	}
	if err := a.profiles.SetConfluenceConfig(profileID, c); err != nil {
		return err
	}
	if moved {
		if err := a.cycles.ForgetPages(profileID); err != nil {
			return err
		}
		if token == "" {
			// The URL was cleared, so the token has nowhere to go.
			if err := a.creds.Delete(profile.ConfluenceCredentialID(profileID)); err != nil {
				log.Printf("pie: delete Confluence credentials for %s: %v", profileID, err)
			}
		}
	}
	if token == "" {
		return nil
	}
	return a.creds.Save(profile.ConfluenceCredentialID(profileID), token)
}

// PublishCycle writes the cycle to Confluence. An Outcome with Ask set wrote
// nothing; calling again with confirmed true goes ahead.
func (a *App) PublishCycle(profileID string, id int64, confirmed bool) (report.Outcome, error) {
	if err := a.requireStore(); err != nil {
		return report.Outcome{}, err
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return report.Outcome{}, err
	}
	c, err := a.profiles.ConfluenceConfig(profileID)
	if err != nil {
		return report.Outcome{}, err
	}
	if c.BaseURL == "" {
		return report.Outcome{}, errors.New("set up Confluence for this profile first, in Confluence settings")
	}
	token, err := a.creds.Load(profile.ConfluenceCredentialID(profileID))
	if err != nil {
		return report.Outcome{}, errors.New("this profile has no Confluence token yet. Add one in Confluence settings")
	}
	pages := confluence.NewClient(c.BaseURL, token, p.CACert, p.AllowUntrustedTLS)
	to := report.Target{Profile: profileID, Space: c.SpaceKey, Parent: c.RootPageID}
	return report.Publish(a.ctx, pages, a.cycles, to, id, confirmed, time.Now())
}
