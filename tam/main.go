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

// appMenu is the native menu bar. Items emit events the frontend listens for,
// so the menu and the in-app buttons share one code path.
func appMenu(app *App) *menu.Menu {
	emit := func(event string) func(*menu.CallbackData) {
		return func(*menu.CallbackData) {
			if app.ctx != nil {
				runtime.EventsEmit(app.ctx, event)
			}
		}
	}
	m := menu.NewMenu()
	file := m.AddSubmenu("File")
	file.AddText("Profiles…", nil, emit("menu:profiles"))
	file.AddSeparator()
	file.AddText("Quit", keys.CmdOrCtrl("q"), func(*menu.CallbackData) {
		if app.ctx != nil {
			runtime.Quit(app.ctx)
		}
	})
	help := m.AddSubmenu("Help")
	help.AddText("About Task Activity Manager", nil, emit("menu:about"))
	return m
}
