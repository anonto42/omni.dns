// Package logger configures the process-wide structured logger (slog).
package logger

import (
	"log/slog"
	"os"
)

// Setup installs a slog default handler matching the given format
// ("text" or "json") and level ("debug", "info", "warn", "error").
func Setup(format, levelName string) {
	opts := &slog.HandlerOptions{Level: parseLevel(levelName)}
	if format == "json" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, opts)))
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
}

func parseLevel(name string) slog.Level {
	switch name {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
