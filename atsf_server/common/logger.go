package common

import (
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type logLevel int

const (
	logLevelDebug logLevel = iota
	logLevelInfo
	logLevelWarn
	logLevelError
)

var currentLogLevel = logLevelInfo
var currentLogLevelName = "info"
var commonLogWriter io.Writer = os.Stdout
var errorLogWriter io.Writer = os.Stderr
var defaultLogger *slog.Logger

type levelRouterHandler struct {
	commonHandler slog.Handler
	errorHandler  slog.Handler
}

func (h *levelRouterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.commonHandler.Enabled(ctx, level) || h.errorHandler.Enabled(ctx, level)
}

func (h *levelRouterHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level >= slog.LevelError {
		return h.errorHandler.Handle(ctx, record)
	}
	return h.commonHandler.Handle(ctx, record)
}

func (h *levelRouterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelRouterHandler{
		commonHandler: h.commonHandler.WithAttrs(attrs),
		errorHandler:  h.errorHandler.WithAttrs(attrs),
	}
}

func (h *levelRouterHandler) WithGroup(name string) slog.Handler {
	return &levelRouterHandler{
		commonHandler: h.commonHandler.WithGroup(name),
		errorHandler:  h.errorHandler.WithGroup(name),
	}
}

func configureGinWriters() {
	if shouldLog(logLevelDebug) {
		gin.DefaultWriter = commonLogWriter
	} else {
		gin.DefaultWriter = io.Discard
	}
	gin.DefaultErrorWriter = errorLogWriter
}

func slogLevel() slog.Level {
	switch currentLogLevel {
	case logLevelDebug:
		return slog.LevelDebug
	case logLevelWarn:
		return slog.LevelWarn
	case logLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func ensureLogger() *slog.Logger {
	if defaultLogger != nil {
		return defaultLogger
	}
	handlerOptions := &slog.HandlerOptions{Level: slogLevel()}
	defaultLogger = slog.New(&levelRouterHandler{
		commonHandler: slog.NewTextHandler(commonLogWriter, handlerOptions),
		errorHandler:  slog.NewTextHandler(errorLogWriter, handlerOptions),
	})
	slog.SetDefault(defaultLogger)
	return defaultLogger
}

func SetLogLevel(level string) {
	normalized := strings.TrimSpace(strings.ToLower(level))
	switch normalized {
	case "debug":
		currentLogLevel = logLevelDebug
		currentLogLevelName = "debug"
	case "warn", "warning":
		currentLogLevel = logLevelWarn
		currentLogLevelName = "warn"
	case "error":
		currentLogLevel = logLevelError
		currentLogLevelName = "error"
	default:
		currentLogLevel = logLevelInfo
		currentLogLevelName = "info"
	}
	configureGinWriters()
}

func GetLogLevel() string {
	return currentLogLevelName
}

func shouldLog(level logLevel) bool {
	return level >= currentLogLevel
}

func SetupGinLog() {
	if *LogDir != "" {
		commonLogPath := filepath.Join(*LogDir, "common.log")
		errorLogPath := filepath.Join(*LogDir, "error.log")
		commonFd, err := os.OpenFile(commonLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			_, _ = io.WriteString(os.Stderr, "failed to open common log file\n")
			os.Exit(1)
		}
		errorFd, err := os.OpenFile(errorLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			_, _ = io.WriteString(os.Stderr, "failed to open error log file\n")
			os.Exit(1)
		}
		commonLogWriter = io.MultiWriter(os.Stdout, commonFd)
		errorLogWriter = io.MultiWriter(os.Stderr, errorFd)
	}
	configureGinWriters()
	defaultLogger = nil
	ensureLogger()
}

func SysLog(s string) {
	if !shouldLog(logLevelInfo) {
		return
	}
	ensureLogger().Info(s)
}

func SysError(s string) {
	if !shouldLog(logLevelError) {
		return
	}
	ensureLogger().Error(s)
}

func FatalLog(v ...any) {
	ensureLogger().Error("fatal error", "details", v)
	os.Exit(1)
}
