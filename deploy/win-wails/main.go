package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	wails.Run(&options.App{
		Title:  "XiaoQinglong",
		Assets: assets,
		OnStartup: func(ctx context.Context) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Wails] Panic recovered: %v", r)
				}
			}()
			startup()
		},
		OnShutdown: func(ctx context.Context) {
			shutdown()
		},
	})
}
