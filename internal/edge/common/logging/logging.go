// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package logging configures structured logging for edge applications.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Options holds configuration options for the structured logger.
type Options struct {
	AddSource bool
}

// Setup initialises the default slog handler using the given options and the LOG_LEVEL environment variable.
func Setup(opts Options) {
	handlerOpts := &slog.HandlerOptions{
		AddSource: opts.AddSource,
		Level:     ParseLevel(os.Getenv("LOG_LEVEL")),
	}
	handler := slog.NewTextHandler(os.Stdout, handlerOpts)
	slog.SetDefault(slog.New(handler))
}

// ParseLevel converts a log-level string (e.g. "debug", "warn") to the corresponding slog.Level.
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
