package logging

import (
	"log/slog"
	"os"
	"strings"
)

type Options struct {
	AddSource bool
}

func Setup(opts Options) {
	handlerOpts := &slog.HandlerOptions{
		AddSource: opts.AddSource,
		Level:     ParseLevel(os.Getenv("LOG_LEVEL")),
	}
	handler := slog.NewTextHandler(os.Stdout, handlerOpts)
	slog.SetDefault(slog.New(handler))
}

func ParseLevel(value string) slog.Level {
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