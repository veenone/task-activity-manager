package main

import (
	"embed"
	"encoding/json"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// wailsConfig is the embedded wails.json, the single source of the product
// version.
//
//go:embed wails.json
var wailsConfig []byte

// productVersion returns info.productVersion from the embedded wails.json, or
// "" if it cannot be read.
func productVersion() string {
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsConfig, &cfg); err != nil {
		return ""
	}
	return cfg.Info.ProductVersion
}

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:  "Task Activity Manager",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:         &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:                app.startup,
		OnShutdown:               app.shutdown,
		Menu:                     appMenu(app),
		EnableDefaultContextMenu: true,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

// menuViews is the View menu's list of views, in the order the menu shows
// them. It has to agree with VIEWS in frontend/src/nav.ts: the id is what the
// menu:view event carries and what the frontend routes on. Adding a view means
// adding it in both places, which is the cost of the menu bar being native.
var menuViews = []struct{ id, label, accelerator string }{
	{"backlog", "Backlog", "1"},
	{"epics", "Epics", "2"},
	{"boards", "Boards", "3"},
	{"sprints", "Sprints", "4"},
	{"reports", "Reports", "5"},
	{"rituals", "Rituals", "6"},
}

// appMenu is the native menu bar, and TAM's primary navigation: the View menu
// switches views the way XTM's does. The left nav rail is a second, optional
// way to do the same thing, toggled from the same menu. Items emit events the
// frontend listens for, so the menu and the in-app controls share one code
// path.
func appMenu(app *App) *menu.Menu {
	emit := func(event string, data ...any) func(*menu.CallbackData) {
		return func(*menu.CallbackData) {
			if app.ctx != nil {
				runtime.EventsEmit(app.ctx, event, data...)
			}
		}
	}
	m := menu.NewMenu()
	file := m.AddSubmenu("File")
	file.AddText("Profiles…", nil, emit("menu:profiles"))
	file.AddSeparator()
	file.AddText("Sync", keys.CmdOrCtrl("r"), emit("menu:sync"))
	file.AddText("Full Sync", keys.Combo("r", keys.CmdOrCtrlKey, keys.ShiftKey), emit("menu:full-sync"))
	file.AddSeparator()
	file.AddText("Quit", keys.CmdOrCtrl("q"), func(*menu.CallbackData) {
		if app.ctx != nil {
			runtime.Quit(app.ctx)
		}
	})

	view := m.AddSubmenu("View")
	for _, v := range menuViews {
		view.AddText(v.label, keys.CmdOrCtrl(v.accelerator), emit("menu:view", v.id))
	}
	view.AddSeparator()
	// The checkbox owns the rail's state: Wails renders the tick from the
	// value passed here, so the app reads the stored preference when it builds
	// the menu and writes it back on every toggle.
	view.AddCheckbox("Navigation Rail", app.showNavRail(), keys.CmdOrCtrl("b"), func(d *menu.CallbackData) {
		app.setShowNavRail(d.MenuItem.Checked)
	})

	help := m.AddSubmenu("Help")
	help.AddText("About Task Activity Manager", nil, emit("menu:about"))
	return m
}
