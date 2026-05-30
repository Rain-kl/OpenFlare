package common

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var (
	Port         = flag.Int("port", 3000, "the listening port")
	PrintVersion = flag.Bool("version", false, "print version and exit")
	PrintHelp    = flag.Bool("help", false, "print help and exit")
	LogDir       = flag.String("log-dir", "", "specify the log directory")
)

// UploadPath Maybe override by ENV_VAR
var UploadPath = "upload"

func printHelp() {
	fmt.Println("OpenFlare " + Version + " - Internal OpenResty Control Plane.")
	fmt.Println("Copyright (C) 2023 JustSong. All rights reserved.")
	fmt.Println("GitHub: https://github.com/Rain-kl/OpenFlare")
	fmt.Println("Usage: openflare [--port <port>] [--log-dir <log directory>] [--version] [--help]")
}

func init() {
	executableName := strings.ToLower(filepath.Base(os.Args[0]))
	if !strings.Contains(executableName, ".test") {
		flag.Parse()
	}

	if *PrintVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	if *PrintHelp {
		printHelp()
		os.Exit(0)
	}

	if os.Getenv("SESSION_SECRET") != "" {
		SessionSecret = os.Getenv("SESSION_SECRET")
	}
	if os.Getenv("SQLITE_PATH") != "" {
		SQLitePath = os.Getenv("SQLITE_PATH")
	}
	if os.Getenv("SQL_DSN") != "" {
		SQLDSN = os.Getenv("SQL_DSN")
	}
	if os.Getenv("DSN") != "" {
		SQLDSN = os.Getenv("DSN")
	}
	if os.Getenv("UPLOAD_PATH") != "" {
		UploadPath = os.Getenv("UPLOAD_PATH")
	}
	if os.Getenv("AGENT_TOKEN") != "" {
		AgentToken = os.Getenv("AGENT_TOKEN")
	}
	SetLogLevel(os.Getenv("LOG_LEVEL"))
	if *LogDir != "" {
		var err error
		*LogDir, err = filepath.Abs(*LogDir)
		if err != nil {
			slog.Error("resolve log directory failed", "error", err)
			os.Exit(1)
		}
		if _, err := os.Stat(*LogDir); os.IsNotExist(err) {
			err = os.Mkdir(*LogDir, 0777)
			if err != nil {
				slog.Error("create log directory failed", "error", err)
				os.Exit(1)
			}
		}
	}
	if _, err := os.Stat(UploadPath); os.IsNotExist(err) {
		_ = os.Mkdir(UploadPath, 0777)
	}
}
