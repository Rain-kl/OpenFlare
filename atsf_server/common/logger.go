package common

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
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
			log.Fatal("failed to open log file")
		}
		errorFd, err := os.OpenFile(errorLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal("failed to open log file")
		}
		gin.DefaultWriter = io.MultiWriter(os.Stdout, commonFd)
		gin.DefaultErrorWriter = io.MultiWriter(os.Stderr, errorFd)
	}
}

func SysLog(s string) {
	if !shouldLog(logLevelInfo) {
		return
	}
	t := time.Now()
	_, _ = fmt.Fprintf(gin.DefaultWriter, "[SYS] %v | %s \n", t.Format("2006/01/02 - 15:04:05"), s)
}

func SysError(s string) {
	if !shouldLog(logLevelError) {
		return
	}
	t := time.Now()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[SYS] %v | %s \n", t.Format("2006/01/02 - 15:04:05"), s)
}

func FatalLog(v ...any) {
	t := time.Now()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[FATAL] %v | %v \n", t.Format("2006/01/02 - 15:04:05"), v)
	os.Exit(1)
}
