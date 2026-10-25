// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cmd 提供 CLI 命令入口
package cmd

import (
	"log"
	"sync"

	"github.com/Rain-kl/Wavelet/internal/infra/task/scheduler"
	"github.com/Rain-kl/Wavelet/internal/infra/task/worker"
	"github.com/Rain-kl/Wavelet/internal/platform/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/router"
	"github.com/spf13/cobra"
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Start API, Worker and Scheduler together",
	Run: func(_ *cobra.Command, _ []string) {
		log.Println("[All] Starting API, Worker and Scheduler")
		bootstrap.RegisterAll()
		runBootstrap(bootstrap.Options{API: true})

		var wg sync.WaitGroup

		// 启动 API HTTP 服务
		wg.Go(func() {
			log.Println("[All] Starting API service")
			router.Serve(func() {
				printStartupBanner(startupState{mode: "API + Worker + Scheduler", relationalDB: latestMigrationState.relationalDB, clickHouseDB: latestMigrationState.clickHouseDB, listensForHTTP: true})
			})
		})

		// 启动 Asynq Worker 任务处理服务
		wg.Go(func() {
			log.Println("[All] Starting Worker service")
			if err := worker.StartWorker(); err != nil {
				log.Printf("[All] Failed to start Worker: %v\n", err)
			}
		})

		// 启动 Asynq 定时任务调度器
		wg.Go(func() {
			log.Println("[All] Starting Scheduler service")
			if err := scheduler.StartScheduler(); err != nil {
				log.Printf("[All] Failed to start Scheduler: %v\n", err)
			}
		})

		wg.Wait()
	},
}
