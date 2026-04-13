package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	// 全局 panic handler
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("[MAIN] PANIC recovered in main: %v", r)
			log.Println(msg)
			// Write to file before exit
			home, _ := os.UserHomeDir()
			if home == "" {
				home = "/tmp"
			}
			f, _ := os.OpenFile(home+"/.xiaoqinglong/logs/panic.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if f != nil {
				f.WriteString(msg + "\n")
				f.Close()
			}
		}
	}()

	// 立即写日志确认 binary 在运行
	home, _ := os.UserHomeDir()
	logFile := home + "\\.xiaoqinglong\\logs\\main.log"
	f, _ := os.Create(logFile)
	if f != nil {
		f.WriteString("[MAIN] main() started\n")
		f.Close()
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[MAIN] xiaoqinglong.exe starting...")

	wails.Run(&options.App{
		Title:  "XiaoQinglong",
		Assets: assets,
		OnStartup: func(ctx context.Context) {
			log.Println("[MAIN] OnStartup called!")
			startup()
		},
		OnShutdown: func(ctx context.Context) {
			log.Println("[MAIN] OnShutdown called!")
			shutdown()
		},
	})
}
