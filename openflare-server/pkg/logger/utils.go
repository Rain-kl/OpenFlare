// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// logDirPerm 日志目录权限
const logDirPerm = 0750

func getLogWriterForConfig(cfg Config) (zapcore.WriteSyncer, error) {
	if cfg.Output == "file" {
		// 初始化日志目录
		logPath := cfg.FilePath
		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, logDirPerm); err != nil {
			return nil, fmt.Errorf(errCreateLogFileDirFailed, err)
		}

		// 配置日志轮转
		logOutput := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}

		return zapcore.AddSync(logOutput), nil
	}

	return zapcore.AddSync(os.Stdout), nil
}

// getEncoderForConfig 获取日志编码器
func getEncoderForConfig(cfg Config) zapcore.Encoder {
	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if cfg.Format == "json" {
		return zapcore.NewJSONEncoder(encoderConfig)
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// getLogLevelForConfig 获取日志级别
func getLogLevelForConfig(cfg Config) zapcore.Level {
	level := cfg.Level

	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		log.Printf("[Logger] invalid log level: %s, defaulting to info\n", level)
		return zapcore.InfoLevel
	}
}

func getTraceIDFields(ctx context.Context) []zap.Field {
	span := trace.SpanFromContext(ctx)
	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return nil
	}
	return []zap.Field{
		zap.String("traceID", spanContext.TraceID().String()),
		zap.String("spanID", spanContext.SpanID().String()),
	}
}
