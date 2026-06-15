package main

import (
	"log/slog"
	"os"

	"github.com/sohidul/dns-server/internal/server"
	"github.com/sohidul/dns-server/internal/shared/config"
	"github.com/sohidul/dns-server/internal/shared/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// The logger is not configured yet; use the slog default.
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	logger.Setup(cfg.LogFormat, cfg.LogLevel)

	srv, err := server.New(cfg, server.StaticFiles{
		Embedded:   isEmbedded(),
		FileSystem: getFileSystem(),
	})
	if err != nil {
		slog.Error("initialize server", "error", err)
		os.Exit(1)
	}
	defer srv.Close()

	if err := srv.Run(); err != nil {
		slog.Error("run server", "error", err)
		os.Exit(1)
	}
}
