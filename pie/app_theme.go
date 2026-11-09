package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed wails.json
var wailsJSON []byte

// productVersion reads info.productVersion, the one place PIE's version is
// written; Wails stamps the same value on the exe.
func productVersion(raw []byte) string {
	var w struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	_ = json.Unmarshal(raw, &w)
	return w.Info.ProductVersion
}

// Version is PIE's version, for Help > About.
func (a *App) Version() string { return productVersion(wailsJSON) }

// Theme is the stored theme: light, dark, or system to follow the OS. A
// failed read falls back to system, which is what a new install shows.
func (a *App) Theme() string {
	if a.settings == nil {
		return "system"
	}
	s, err := a.settings.Get()
	if err != nil || s.Theme == "" {
		return "system"
	}
	return s.Theme
}

// SetTheme stores the theme and redraws the menu, so its Theme radio follows
// a change made from the top bar.
func (a *App) SetTheme(theme string) error {
	if !slices.ContainsFunc(themes, func(t struct{ id, label string }) bool { return t.id == theme }) {
		return fmt.Errorf("unknown theme %q", theme)
	}
	if a.settings == nil {
		return errors.New("the database is not available: " + a.startupErr)
	}
	if err := a.settings.SetTheme(theme); err != nil {
		return err
	}
	if a.ctx != nil {
		runtime.MenuSetApplicationMenu(a.ctx, appMenu(a))
		runtime.MenuUpdateApplicationMenu(a.ctx)
	}
	return nil
}
