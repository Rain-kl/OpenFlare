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

func printHelp() {
	fmt.Println("OpenFlare " + Version + " - Internal OpenResty Control Plane.")
	fmt.Println("Copyright (C) 2023 JustSong. All rights reserved.")
	fmt.Println("GitHub: https://github.com/Rain-kl/OpenFlare")
	fmt.Println("Usage: openflare [--port <port>] [--log-dir <log directory>] [--version] [--help]")
}

// ParseFlags 在命令行参数被任何 import 链上的 init() 误解析之前，
// 由各 binary 的 main() 显式调用一次。openflare-server 与 openflare-relay
// 共用 flag.CommandLine，必须先注册各自的 flag 再调用本函数。
// 测试场景（go test）下不会执行本函数，单元测试可直接跳过命令行解析。
func ParseFlags() {
	executableName := strings.ToLower(filepath.Base(os.Args[0]))
	isTest := strings.Contains(executableName, ".test") || flag.Lookup("test.v") != nil
	if isTest {
		return
	}
	flag.Parse()

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
	if os.Getenv("JWT_SECRET") != "" {
		JWTSecret = os.Getenv("JWT_SECRET")
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

	if os.Getenv("AGENT_TOKEN") != "" {
		AccessToken = os.Getenv("AGENT_TOKEN")
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

}
