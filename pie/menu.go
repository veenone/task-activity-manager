package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// menuViews is the View menu's list of tabs, in tab order. It has to agree
// with VIEWS in frontend/src/views.ts, and the instruction gate checks that it
// does. The accelerator is the tab's place in the list.
var menuViews = []struct{ id, label, accelerator string }{
	{"results", "Results", "1"},
	{"trace", "Trace", "2"},
	{"map", "Project map", "3"},
	{"links", "Link map", "4"},
	{"stats", "Statistics", "5"},
	{"cases", "Cases", "6"},
	{"cycles", "Cycles", "7"},
}

var themes = []struct{ id, label string }{{"light", "Light"}, {"dark", "Dark"}, {"system", "System"}}

// appMenu is the native menu bar, shaped like TAM's. Items emit events the
// frontend already handles for its own buttons, so both share one code path.
// A theme picked here is stored before the frontend hears of it.
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
	file.AddText("Manage profiles…", nil, emit("menu:profiles"))
	file.AddText("Jira fields…", nil, emit("menu:fields"))
	file.AddSeparator()
	file.AddText("Run query", keys.CmdOrCtrl("r"), emit("menu:run"))
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
	theme := view.AddSubmenu("Theme")
	current := app.Theme()
	for _, t := range themes {
		theme.AddRadio(t.label, current == t.id, nil, func(*menu.CallbackData) {
			if err := app.SetTheme(t.id); err == nil && app.ctx != nil {
				runtime.EventsEmit(app.ctx, "menu:theme", t.id)
			}
		})
	}

	help := m.AddSubmenu("Help")
	help.AddText("About Project Issue Explorer", nil, emit("menu:about"))
	return m
}
