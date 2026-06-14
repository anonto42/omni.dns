package main

import (
	"log/slog"
	"os"

	"github.com/sohidul/dns-server/internal/app"
)

func main() {
	cfg := app.LoadConfig()
	app.SetupLogger(cfg.LogFormat, cfg.LogLevel)

	server, err := app.New(cfg, app.StaticFiles{
		Embedded:   isEmbedded(),
		FileSystem: getFileSystem(),
	})
	if err != nil {
		slog.Error("initialize app", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	if err := server.Run(); err != nil {
		slog.Error("run app", "error", err)
		os.Exit(1)
	}
}
