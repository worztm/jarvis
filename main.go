package main

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/wailsapp/wails/v2"
)

//go:embed all:frontend/dist
var assets embed.FS

var appInstance *App

// emit pushes an event to the frontend; safe before the context exists.
func emit(name string, data any) {
	if appInstance == nil || appInstance.ctx == nil {
		if pretty, err := json.Marshal(data); err == nil {
			fmt.Printf("[event:%s] %s\n", name, pretty)
		}
		return
	}
	wailsruntime.EventsEmit(appInstance.ctx, name, data)
}

func main() {
	app := NewApp()
	appInstance = app

	err := wails.Run(&options.App{
		Title:            "JARVIS",
		Width:            1280,
		Height:           860,
		MinWidth:         980,
		MinHeight:        640,
		DisableResize:    false,
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 10, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.Dark,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
