package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/bambutcha/localguard/internal/version"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(log)

	app := NewApp(log)

	err := wails.Run(&options.App{
		Title:  "LocalGuard " + version.Label(),
		Width:  1024,
		Height: 768,
		MinWidth:  360,
		MinHeight: 420,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// paper mist — совпадает с UI
		BackgroundColour: &options.RGBA{R: 243, G: 240, B: 232, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Error("приложение завершилось с ошибкой", "err", err)
		os.Exit(1)
	}
}
