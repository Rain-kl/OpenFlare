// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cmd 提供 CLI 命令入口
package cmd

import (
	"log"
	"sync"

	"github.com/Rain-kl/Wavelet/internal/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/router"
	"github.com/Rain-kl/Wavelet/internal/task/scheduler"
	"github.com/Rain-kl/Wavelet/internal/task/worker"
	"github.com/spf13/cobra"
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "以融合模式同时启动 API、Worker 和 Scheduler",
	Run: func(_ *cobra.Command, _ []string) {
		log.Println("[All] 融合模式启动")
		bootstrap.RegisterAll()
		runBootstrap(bootstrap.Options{API: true})

		var wg sync.WaitGroup

		// 启动 API HTTP 服务
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Println("[All] 启动 API 服务")
			router.Serve()
		}()

		// 启动 Asynq Worker 任务处理服务
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Println("[All] 启动 Worker 服务")
			if err := worker.StartWorker(); err != nil {
				log.Printf("[All] Worker 启动失败: %v\n", err)
			}
		}()

		// 启动 Asynq 定时任务调度器
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Println("[All] 启动 Scheduler 服务")
			if err := scheduler.StartScheduler(); err != nil {
				log.Printf("[All] Scheduler 启动失败: %v\n", err)
			}
		}()

		wg.Wait()
	},
}
