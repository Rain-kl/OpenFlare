package logging

import (
	"log/slog"
	"os"
	"strings"
)

func Setup() {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     parseLevel(os.Getenv("LOG_LEVEL")),
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(handler))
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
