package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := &App{}
	err := wails.Run(&options.App{
		Title:  "Project Issue Explorer",
		Width:  1280,
		Height: 800,
		// Below this the Link map's side panes squeeze the graph canvas (#552);
		// App.css narrows them under 1280px and layout.test.ts checks the sum.
		MinWidth:  1024,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Menu:             appMenu(app),
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
