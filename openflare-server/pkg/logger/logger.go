// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"context"
	"fmt"
	"log"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config represents the logging configuration.
type Config struct {
	Level      string
	Format     string
	Output     string
	FilePath   string
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Compress   bool
}

var logger *otelzap.Logger

// ringBufferCapacity 环形缓冲区容量
const ringBufferCapacity = 5000

// GlobalRingBuffer 全局日志环形缓冲区，供 Admin 日志查询和 WebSocket 推送使用
var GlobalRingBuffer *LogRingBuffer

func doInit(cfg Config) {
	logWriter, err := getLogWriterForConfig(cfg)
	if err != nil {
		log.Fatalf("[Logger] get log writer err: %v\n", err)
	}

	// 初始化 ring buffer（保留最近 5000 行日志），如果是多次调用 Init，不需要重复创建 GlobalRingBuffer
	if GlobalRingBuffer == nil {
		GlobalRingBuffer = NewLogRingBuffer(ringBufferCapacity)
	}

	// 使用 multi writer 同时写入原始输出和 ring buffer
	multiWriter := zapcore.NewMultiWriteSyncer(
		logWriter,
		zapcore.AddSync(GlobalRingBuffer),
	)

	zapLogger := zap.New(
		zapcore.NewCore(getEncoderForConfig(cfg), multiWriter, getLogLevelForConfig(cfg)),
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)
	logger = otelzap.New(
		zapLogger,
		otelzap.WithMinLevel(zapLogger.Level()),
	)
}

func init() {
	// 默认使用 console stdout INFO 日志输出，避免在 Init 前或测试中发生空指针崩溃
	defaultCfg := Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}
	doInit(defaultCfg)
}

// Init initializes the logger with a custom configuration.
func Init(cfg Config) {
	doInit(cfg)
}

// DebugF 输出 Debug 级别日志
func DebugF(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logger.Ctx(ctx).Debug(msg, getTraceIDFields(ctx)...)
}

// InfoF 输出 Info 级别日志
func InfoF(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logger.Ctx(ctx).Info(msg, getTraceIDFields(ctx)...)
}

// WarnF 输出 Warn 级别日志
func WarnF(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logger.Ctx(ctx).Warn(msg, getTraceIDFields(ctx)...)
}

// ErrorF 输出 Error 级别日志
func ErrorF(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logger.Ctx(ctx).Error(msg, getTraceIDFields(ctx)...)
}
