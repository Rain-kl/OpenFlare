// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"log"
	"time"

	"github.com/Rain-kl/Wavelet/internal/buildinfo"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db/migrator"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/Rain-kl/Wavelet/pkg/trace"
	"github.com/spf13/cobra"
)

const traceShutdownTimeout = 10 * time.Second

var rootCmd = &cobra.Command{
	Use: "wavelet",
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		logger.Init(logger.Config{
			Level:      config.Config.Log.Level,
			Format:     config.Config.Log.Format,
			Output:     config.Config.Log.Output,
			FilePath:   config.Config.Log.FilePath,
			MaxSize:    config.Config.Log.MaxSize,
			MaxAge:     config.Config.Log.MaxAge,
			MaxBackups: config.Config.Log.MaxBackups,
			Compress:   config.Config.Log.Compress,
		})
		trace.Init(trace.Config{
			AppName:      config.Config.App.AppName,
			SamplingRate: config.Config.Otel.SamplingRate,
			TracerName:   config.Config.Otel.TracerName,
		})
	},
	PreRun: func(_ *cobra.Command, _ []string) {
		migrator.Migrate()
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		shutdownTraceProvider()
	},
	Run: func(_ *cobra.Command, args []string) {
		// 无参数时默认以融合模式启动所有服务
		if len(args) == 0 {
			allCmd.Run(allCmd, args)
			return
		}
		appMode := args[0]
		switch appMode {
		case "api":
			apiCmd.Run(apiCmd, args)
		case "scheduler":
			schedulerCmd.Run(schedulerCmd, args)
		case "worker":
			workerCmd.Run(workerCmd, args)
		case "all":
			allCmd.Run(allCmd, args)
		default:
			log.Fatal("[CMD] unknown app mode\n")
		}
	},
}

func shutdownTraceProvider() {
	ctx, cancel := context.WithTimeout(context.Background(), traceShutdownTimeout)
	defer cancel()
	trace.Shutdown(ctx)
}

func init() {
	rootCmd.Version = buildinfo.Version
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("[CMD] execute failed; %s\n", err)
	}
}
